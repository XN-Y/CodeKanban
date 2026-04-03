<template>
  <div class="terminal-viewport">
    <div ref="containerRef" class="terminal-shell"></div>
    <div v-if="statusOverlayMessage" class="terminal-overlay" :style="terminalOverlayStyle">
      <span>{{ statusOverlayMessage }}</span>
    </div>
    <div
      v-if="transferCardMessage"
      class="terminal-transfer-card"
      :class="{ 'is-error': transferCardTone === 'error' }"
      :style="transferCardStyle"
    >
      <span class="terminal-transfer-message">{{ transferCardMessage }}</span>
      <div v-if="transferProgress !== null" class="terminal-transfer-progress">
        <div class="terminal-transfer-progress-fill" :style="transferProgressStyle"></div>
      </div>
      <span v-if="transferProgress !== null" class="terminal-transfer-percent">
        {{ transferProgress }}%
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useDebounceFn } from '@vueuse/core';
import { storeToRefs } from 'pinia';
import type EventEmitter from 'eventemitter3';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { SearchAddon } from '@xterm/addon-search';
import { SerializeAddon } from '@xterm/addon-serialize';
import { useMessage } from 'naive-ui';
import '@/styles/terminal.css';
import type { TerminalTabState, ServerMessage } from '@/composables/useTerminalClient';
import {
  getTerminalSnapshot,
  saveTerminalSnapshot,
  clearTerminalSnapshot,
} from '@/utils/terminalSnapshotCache';
import { useSettingsStore, DEFAULT_TERMINAL_FONT_FAMILY } from '@/stores/settings';
import { useTerminalStore } from '@/stores/terminal';
import { getTerminalThemeById, getDefaultTerminalTheme } from '@/constants/terminalThemes';
import { hexToRgba } from '@/utils/color';
import {
  formatTerminalPathInput,
  uploadTerminalImage,
  type TerminalImageUploadSource,
} from '@/utils/terminalImageUpload';
import { useLocale } from '@/composables/useLocale';

const props = defineProps<{
  tab: TerminalTabState;
  emitter: EventEmitter;
  send: (sessionId: string, payload: any) => void;
  shouldAutoFocus?: boolean;
  isMobile?: boolean;
}>();

// 移动端使用较小的字体
// 移动端固定使用 11px，桌面端使用用户设置
const MOBILE_FONT_SIZE = 11;

function getTerminalFontSize(baseFontSize: number): number {
  if (props.isMobile) {
    return MOBILE_FONT_SIZE;
  }
  return baseFontSize;
}

const settingsStore = useSettingsStore();
const terminalStore = useTerminalStore();
const message = useMessage();
const { t } = useLocale();
const { effectiveTerminalThemeId, terminalFont, terminalWebGLRenderer } =
  storeToRefs(settingsStore);

const activeTerminalTheme = computed(() => {
  return getTerminalThemeById(effectiveTerminalThemeId.value) || getDefaultTerminalTheme();
});

const terminalOverlayStyle = computed(() => {
  const theme = activeTerminalTheme.value.theme;
  return {
    '--terminal-overlay-bg': hexToRgba(theme.background || '#0f111a', 0.7),
    '--terminal-overlay-color': theme.foreground ?? '#f6f8ff',
  };
});

const transferCardStyle = computed(() => {
  const theme = activeTerminalTheme.value.theme;
  const background = theme.background || '#0f111a';
  const foreground = theme.foreground || '#f6f8ff';

  return {
    '--terminal-transfer-card-bg': hexToRgba(background, 0.94),
    '--terminal-transfer-card-fg': foreground,
    '--terminal-transfer-card-border': hexToRgba(foreground, 0.18),
    '--terminal-transfer-card-track': hexToRgba(foreground, 0.14),
  };
});

const containerRef = ref<HTMLDivElement>();
let terminal: Terminal | null = null;
let fitAddon: FitAddon | null = null;
let webglAddon: WebglAddon | null = null;
let serializeAddon: SerializeAddon | null = null;
let pasteHandler: ((event: ClipboardEvent) => void) | null = null;
let keydownCaptureHandler: ((event: KeyboardEvent) => void) | null = null;
let dragOverHandler: ((event: DragEvent) => void) | null = null;
let dropHandler: ((event: DragEvent) => void) | null = null;
let transferOverlayTimer: number | null = null;
const textDecoder = typeof TextDecoder !== 'undefined' ? new TextDecoder('utf-8') : null;
const SNAPSHOT_SCROLLBACK = 1200;
const transferCardMessage = ref('');
const transferCardTone = ref<'progress' | 'error'>('progress');
const transferProgress = ref<number | null>(null);

/**
 * 替换 True Color #000000 为可见颜色
 * xterm.js 的 extendedAnsi 只对 256 色索引模式生效，
 * 但很多程序使用 True Color (24-bit RGB) 直接输出颜色，绕过了映射表。
 * 这里对 True Color 前景色和背景色 RGB(0,0,0) 进行替换，避免在深色背景上不可见。
 * 这对 oh-my-zsh 主题（如 agnoster）尤为重要，因为它们大量使用背景色。
 *
 * True Color 前景色格式: \x1b[38;2;R;G;Bm 或 \x1b[38;2;R;G;B;...m
 * True Color 背景色格式: \x1b[48;2;R;G;Bm 或 \x1b[48;2;R;G;B;...m
 */
const TRUE_COLOR_FG_BLACK_REGEX = /\x1b\[38;2;0;0;0([;m])/g;
const TRUE_COLOR_BG_BLACK_REGEX = /\x1b\[48;2;0;0;0([;m])/g;
const TRUE_COLOR_FG_BLACK_REPLACEMENT = '\x1b[38;2;74;74;74$1'; // #4a4a4a
const TRUE_COLOR_BG_BLACK_REPLACEMENT = '\x1b[48;2;40;40;40$1'; // #282828 - 背景用更深的灰

function remapInvisibleColors(data: string): string {
  return data
    .replace(TRUE_COLOR_FG_BLACK_REGEX, TRUE_COLOR_FG_BLACK_REPLACEMENT)
    .replace(TRUE_COLOR_BG_BLACK_REGEX, TRUE_COLOR_BG_BLACK_REPLACEMENT);
}

// 内存中记录已经访问过的终端（刷新后清空）
// 用于检测刷新后首次切换到终端时滚动到底部
const visitedTerminals = new Set<string>();

// 监听 clientStatus 变化
watch(
  () => props.tab.clientStatus,
  (newStatus, oldStatus) => {
    console.log('[Terminal Watch] ClientStatus changed:', {
      sessionId: props.tab.id,
      from: oldStatus,
      to: newStatus,
    });
  }
);

// 监听终端主题变化，动态更新终端主题
watch(activeTerminalTheme, newTheme => {
  if (terminal) {
    terminal.options.theme = newTheme.theme;
  }
});

// 监听终端字体设置变化，动态更新终端字体
watch(
  terminalFont,
  newFont => {
    if (terminal) {
      const actualFontSize = getTerminalFontSize(newFont.fontSize);
      terminal.options.fontFamily = newFont.fontFamily || DEFAULT_TERMINAL_FONT_FAMILY;
      terminal.options.fontSize = actualFontSize;
      terminal.options.fontWeight = newFont.fontWeight;
      terminal.options.fontWeightBold = newFont.fontWeightBold;
      terminal.options.lineHeight = newFont.lineHeight;
      terminal.options.letterSpacing = newFont.letterSpacing;
      // 字体变化后需要重新 fit 以适应新的尺寸
      if (fitAddon) {
        setTimeout(() => {
          handleResize();
        }, 50);
      }
    }
  },
  { deep: true }
);

const shouldAutoFocus = computed(() => props.shouldAutoFocus !== false);

const statusOverlayMessage = computed(() => {
  const status = props.tab.clientStatus;
  // Removed debug log to avoid confusion with AI completion detection
  switch (status) {
    case 'connecting':
      return '正在连接终端…';
    case 'error':
      return '连接异常，稍后重试';
    case 'closed':
      return '会话已结束';
    default:
      return '';
  }
});

const transferProgressStyle = computed(() => {
  return {
    width: `${transferProgress.value ?? 0}%`,
  };
});

function clearTransferOverlay() {
  if (transferOverlayTimer != null) {
    window.clearTimeout(transferOverlayTimer);
    transferOverlayTimer = null;
  }
  transferCardMessage.value = '';
  transferCardTone.value = 'progress';
  transferProgress.value = null;
}

function showTransferOverlay(
  messageText: string,
  options?: {
    duration?: number;
    tone?: 'progress' | 'error';
    progress?: number | null;
  }
) {
  if (transferOverlayTimer != null) {
    window.clearTimeout(transferOverlayTimer);
    transferOverlayTimer = null;
  }

  transferCardMessage.value = messageText;
  transferCardTone.value = options?.tone ?? 'progress';
  transferProgress.value =
    typeof options?.progress === 'number'
      ? Math.max(0, Math.min(100, Math.round(options.progress)))
      : options?.progress === null
        ? null
        : transferProgress.value;

  if ((options?.duration ?? 0) > 0) {
    transferOverlayTimer = window.setTimeout(() => {
      transferOverlayTimer = null;
      clearTransferOverlay();
    }, options?.duration ?? 0);
  }
}

function handleMessage(payload: ServerMessage) {
  if (!terminal) {
    return;
  }
  switch (payload.type) {
    case 'data':
      if (payload.data) {
        terminal.write(remapInvisibleColors(decodeChunk(payload.data)));
      }
      break;
    case 'exit':
      if (payload.data) {
        terminal.writeln(`\r\n${payload.data}`);
      }
      break;
    case 'error':
      if (payload.data) {
        terminal.writeln(`\r\n错误: ${payload.data}`);
      }
      break;
    case 'metadata':
      // Forward metadata to parent component via emitter
      if (payload.metadata) {
        props.emitter.emit('metadata', props.tab.id, payload.metadata);
      }
      break;
    default:
      break;
  }
}

function decodeChunk(chunk: string) {
  if (!chunk) {
    return '';
  }
  if (textDecoder) {
    try {
      const bytes = base64ToUint8Array(chunk);
      return textDecoder.decode(bytes);
    } catch {
      // fall through to legacy atob for unexpected errors
    }
  }
  try {
    return window.atob(chunk);
  } catch {
    return chunk;
  }
}

function base64ToUint8Array(value: string) {
  const binary = window.atob(value);
  const len = binary.length;
  const bytes = new Uint8Array(len);
  for (let i = 0; i < len; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function sendTerminalInput(data: string) {
  if (!data) {
    return;
  }

  props.send(props.tab.id, { type: 'input', data });
}

function shouldUseBrowserPasteShortcut(event: KeyboardEvent) {
  if (event.type !== 'keydown') {
    return false;
  }

  return (event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'v';
}

function shouldBlockCodexClipboardShortcut(event: KeyboardEvent) {
  if (event.type !== 'keydown') {
    return false;
  }

  if (!(props.tab.aiAssistant?.detected && props.tab.aiAssistant?.type === 'codex')) {
    return false;
  }

  return (
    event.altKey &&
    !event.ctrlKey &&
    !event.metaKey &&
    !event.shiftKey &&
    event.key.toLowerCase() === 'v'
  );
}

function getClipboardImage(clipboardData: DataTransfer | null) {
  if (!clipboardData) {
    return null;
  }

  for (const item of Array.from(clipboardData.items || [])) {
    if (!item.type.startsWith('image/')) {
      continue;
    }

    const file = item.getAsFile();
    if (file) {
      return file;
    }
  }

  for (const file of Array.from(clipboardData.files || [])) {
    if (file.type.startsWith('image/')) {
      return file;
    }
  }

  return null;
}

async function uploadImageAndInsert(
  blob: Blob | File,
  source: TerminalImageUploadSource,
  explicitFileName?: string
) {
  showTransferOverlay(t('terminal.imageUploading'), {
    tone: 'progress',
    progress: 0,
  });

  try {
    const result = await uploadTerminalImage({
      blob,
      fileName: explicitFileName,
      source,
      onProgress: progress => {
        showTransferOverlay(t('terminal.imageUploading'), {
          tone: 'progress',
          progress: progress.percent ?? transferProgress.value ?? 0,
        });
      },
    });

    sendTerminalInput(formatTerminalPathInput(result.path));
    clearTransferOverlay();
  } catch (error) {
    console.warn('[Terminal] Failed to upload image:', error);
    showTransferOverlay(t('terminal.imageUploadFailed'), {
      tone: 'error',
      progress: null,
      duration: 900,
    });
    message.error(t('terminal.imageUploadFailed'));
  }
}

function handlePaste(event: ClipboardEvent) {
  const clipboardData = event.clipboardData;
  if (!clipboardData) {
    return;
  }

  const image = getClipboardImage(clipboardData);
  if (image) {
    event.preventDefault();
    void uploadImageAndInsert(image, 'paste', image.name);
    return;
  }
  // Let xterm/browser handle text paste natively so it is only inserted once.
}

function restoreSnapshotIfAvailable() {
  if (!terminal) {
    return false;
  }
  const snapshot = getTerminalSnapshot(props.tab.id);
  if (!snapshot) {
    return false;
  }
  try {
    terminal.reset();
    terminal.write(remapInvisibleColors(snapshot.serialized));
    console.log('[Terminal Snapshot] Restored cache for session:', props.tab.id);
    return true;
  } catch (error) {
    console.warn('[Terminal Snapshot] Failed to restore cache', error);
    clearTerminalSnapshot(props.tab.id);
    return false;
  }
}

function persistSnapshot() {
  if (!terminal || !serializeAddon) {
    return;
  }
  try {
    const serialized = serializeAddon.serialize({
      scrollback: SNAPSHOT_SCROLLBACK,
    });
    if (!serialized) {
      clearTerminalSnapshot(props.tab.id);
      return;
    }
    saveTerminalSnapshot(props.tab.id, {
      serialized,
      cols: terminal.cols,
      rows: terminal.rows,
    });
    console.log('[Terminal Snapshot] Saved cache for session:', props.tab.id);
  } catch (error) {
    console.warn('[Terminal Snapshot] Failed to serialize terminal contents', error);
  }
}

function isContainerVisible() {
  return Boolean(
    containerRef.value &&
      containerRef.value.offsetWidth > 0 &&
      containerRef.value.offsetHeight > 0
  );
}

function refreshTerminalViewport(
  reason: string,
  options: {
    clearTextureAtlas?: boolean;
    retry?: boolean;
  } = {}
) {
  if (!terminal) {
    return;
  }

  const runRefresh = () => {
    if (!terminal || !isContainerVisible()) {
      return;
    }

    handleResize();

    if (options.clearTextureAtlas !== false) {
      try {
        terminal.clearTextureAtlas();
      } catch (error) {
        console.warn('[Terminal Refresh] Failed to clear texture atlas', {
          sessionId: props.tab.id,
          reason,
          error,
        });
      }
    }

    try {
      terminal.refresh(0, Math.max(terminal.rows - 1, 0));
      console.log('[Terminal Refresh]', {
        sessionId: props.tab.id,
        title: props.tab.title,
        reason,
        cols: terminal.cols,
        rows: terminal.rows,
      });
    } catch (error) {
      console.warn('[Terminal Refresh] Failed to refresh terminal viewport', {
        sessionId: props.tab.id,
        reason,
        error,
      });
    }
  };

  runRefresh();

  if (options.retry !== false) {
    window.setTimeout(runRefresh, 160);
  }
}

function handleResize() {
  if (!terminal || !fitAddon) {
    console.log('[Terminal Resize] Skipped: terminal or fitAddon not ready');
    return;
  }

  // 检查容器是否可见（v-show="false" 时容器尺寸为 0）
  if (
    !containerRef.value ||
    containerRef.value.offsetWidth === 0 ||
    containerRef.value.offsetHeight === 0
  ) {
    console.log('[Terminal Resize] Skipped: container not visible', {
      sessionId: props.tab.id,
      title: props.tab.title,
      containerSize: containerRef.value
        ? {
            width: containerRef.value.offsetWidth,
            height: containerRef.value.offsetHeight,
          }
        : null,
    });
    return;
  }

  try {
    fitAddon.fit();

    props.tab.cols = terminal.cols;
    props.tab.rows = terminal.rows;
    console.log('[Terminal Resize]', {
      sessionId: props.tab.id,
      title: props.tab.title,
      cols: terminal.cols,
      rows: terminal.rows,
      containerSize: containerRef.value
        ? {
            width: containerRef.value.offsetWidth,
            height: containerRef.value.offsetHeight,
          }
        : null,
    });
    props.send(props.tab.id, {
      type: 'resize',
      cols: terminal.cols,
      rows: terminal.rows,
    });
  } catch (error) {
    // 忽略 fit 可能出现的错误
    console.warn('Terminal resize failed:', error);
  }
}

// 防抖版本的 resize 处理，避免窗口调整时发送大量 resize 消息阻塞输入
const debouncedResize = useDebounceFn(handleResize, 100);

function handleTerminalResizeAll() {
  console.log('[Terminal Resize Event]', {
    sessionId: props.tab.id,
    title: props.tab.title,
  });
  // 延迟一下确保 DOM 更新完成，使用防抖版本避免阻塞输入
  setTimeout(() => {
    refreshTerminalViewport('terminal-resize-event');
  }, 10);
}

onMounted(() => {
  // 获取当前选择的终端主题
  const selectedTheme = activeTerminalTheme.value;
  // 获取当前的字体设置
  const fontSettings = terminalFont.value;

  // 移动端使用固定较小字体
  const actualFontSize = getTerminalFontSize(fontSettings.fontSize);

  terminal = new Terminal({
    allowProposedApi: true,
    convertEol: true,
    rows: props.tab.rows || 24,
    cols: props.tab.cols || 80,
    cursorBlink: true,
    scrollOnUserInput: true,
    fontFamily: fontSettings.fontFamily || DEFAULT_TERMINAL_FONT_FAMILY,
    fontSize: actualFontSize,
    fontWeight: fontSettings.fontWeight,
    fontWeightBold: fontSettings.fontWeightBold,
    lineHeight: fontSettings.lineHeight,
    letterSpacing: fontSettings.letterSpacing,
    theme: selectedTheme.theme,
  });
  console.log('[Terminal] Created:', {
    isMobile: props.isMobile,
    fontSize: actualFontSize,
    baseFontSize: fontSettings.fontSize,
    dpr: window.devicePixelRatio,
    screenWidth: window.screen.width,
    innerWidth: window.innerWidth,
    visualViewportWidth: window.visualViewport?.width,
  });

  fitAddon = new FitAddon();
  const webLinksAddon = new WebLinksAddon();
  const searchAddon = new SearchAddon();
  serializeAddon = new SerializeAddon();

  terminal.loadAddon(fitAddon);
  terminal.loadAddon(webLinksAddon);
  terminal.loadAddon(searchAddon);
  terminal.loadAddon(serializeAddon);

  // 根据设置决定是否使用 WebGL 渲染器
  // - auto: 桌面端使用 WebGL，移动端使用 Canvas（避免 DPR 缩放问题）
  // - force: 强制使用 WebGL
  // - disable: 强制禁用 WebGL
  const webglMode = terminalWebGLRenderer.value;
  const shouldUseWebGL = webglMode === 'force' || (webglMode === 'auto' && !props.isMobile);

  if (shouldUseWebGL) {
    try {
      webglAddon = new WebglAddon();
      webglAddon.onContextLoss(() => {
        console.warn('[Terminal] WebGL context lost, falling back to canvas renderer', {
          sessionId: props.tab.id,
          title: props.tab.title,
        });
        webglAddon?.dispose();
        webglAddon = null;
        window.setTimeout(() => {
          refreshTerminalViewport('webgl-context-loss', {
            clearTextureAtlas: false,
          });
        }, 0);
      });
      terminal.loadAddon(webglAddon);
      console.log('[Terminal] WebGL renderer loaded successfully', {
        webglMode,
        isMobile: props.isMobile,
      });
    } catch (error) {
      console.warn('[Terminal] WebGL renderer failed to load, using Canvas fallback', error);
    }
  } else {
    console.log('[Terminal] Using Canvas renderer', { webglMode, isMobile: props.isMobile });
  }

  const restoredFromCache = restoreSnapshotIfAvailable();

  const container = containerRef.value;
  if (container) {
    terminal.open(container);
    if (restoredFromCache) {
      setTimeout(() => {
        terminal?.scrollToBottom();
      }, 0);
    }
    // 延迟执行 fit，确保 DOM 完全渲染且面板动画完成
    // 面板展开动画 200ms + 额外缓冲 150ms = 350ms
    const performFit = (retryIfSmall = true) => {
      if (!fitAddon || !terminal) return;

      // 检查容器是否可见
      if (
        !containerRef.value ||
        containerRef.value.offsetWidth === 0 ||
        containerRef.value.offsetHeight === 0
      ) {
        console.log('[Terminal Init Fit] Skipped: container not visible', {
          sessionId: props.tab.id,
          title: props.tab.title,
          retryIfSmall,
          containerSize: containerRef.value
            ? {
                width: containerRef.value.offsetWidth,
                height: containerRef.value.offsetHeight,
              }
            : null,
        });
        // 容器不可见，稍后重试
        if (retryIfSmall) {
          setTimeout(() => performFit(false), 200);
        }
        return;
      }

      fitAddon.fit();

      // 等待数据写入完成后滚动到底部
      let lastLength = terminal.buffer.active.length;
      let stableCount = 0;
      let totalChecks = 0;
      const maxChecks = 50;

      const checkStableAndScroll = () => {
        if (!terminal) return;
        totalChecks++;
        if (totalChecks >= maxChecks) {
          return;
        }

        const currentLength = terminal.buffer.active.length;
        if (currentLength === lastLength) {
          stableCount++;
          if (stableCount >= 3) {
            terminal.scrollToBottom();
            return;
          }
        } else {
          stableCount = 0;
          lastLength = currentLength;
        }
        setTimeout(checkStableAndScroll, 20);
      };
      setTimeout(checkStableAndScroll, 20);

      const cols = terminal.cols;
      const rows = terminal.rows;

      console.log('[Terminal Init Fit]', {
        sessionId: props.tab.id,
        title: props.tab.title,
        cols,
        rows,
        retryIfSmall,
        containerSize: containerRef.value
          ? {
              width: containerRef.value.offsetWidth,
              height: containerRef.value.offsetHeight,
            }
          : null,
      });

      // 检查计算出的尺寸是否合理
      if ((cols < 20 || rows < 5) && retryIfSmall) {
        console.warn('[Terminal Init] Size too small, will retry:', { cols, rows });
        // 容器可能还没准备好，延迟再试一次
        setTimeout(() => performFit(false), 200);
        return;
      }

      // 标记为已访问（初始化时就可见的终端）
      visitedTerminals.add(props.tab.id);

      // 更新状态并通知服务器
      props.tab.cols = cols;
      props.tab.rows = rows;
      props.send(props.tab.id, {
        type: 'resize',
        cols,
        rows,
      });
      if (shouldAutoFocus.value) {
        terminal.focus();
      }
    };

    setTimeout(() => performFit(), 350);
  }

  terminal.onData(data => {
    sendTerminalInput(data);
  });

  keydownCaptureHandler = (event: KeyboardEvent) => {
    if (shouldUseBrowserPasteShortcut(event)) {
      // Keep browser paste enabled, but stop xterm/Codex from consuming Ctrl/Cmd+V first.
      event.stopImmediatePropagation();
      event.stopPropagation();
      return;
    }
  };

  terminal.attachCustomKeyEventHandler(event => {
    if (shouldBlockCodexClipboardShortcut(event)) {
      event.preventDefault();
      message.warning(t('terminal.nativePasteUnavailable'));
      return false;
    }

    return true;
  });

  pasteHandler = (event: ClipboardEvent) => {
    handlePaste(event);
  };

  dragOverHandler = (event: DragEvent) => {
    event.preventDefault();
    event.stopPropagation();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'copy';
    }
  };

  dropHandler = async (event: DragEvent) => {
    event.preventDefault();
    event.stopPropagation();

    const files = event.dataTransfer?.files;
    if (!files || files.length === 0) {
      return;
    }

    for (const file of Array.from(files)) {
      if (!file.type.startsWith('image/')) {
        continue;
      }

      await uploadImageAndInsert(file, 'drop', file.name);
    }
  };

  container?.addEventListener('keydown', keydownCaptureHandler, true);
  container?.addEventListener('paste', pasteHandler, true);
  container?.addEventListener('dragover', dragOverHandler);
  container?.addEventListener('drop', dropHandler);

  props.emitter.on(props.tab.id, handleMessage);
  props.emitter.on('terminal-resize-all', handleTerminalResizeAll);
  props.emitter.on(`terminal-resize-${props.tab.id}`, handleTerminalResizeAll);
  props.emitter.on(`terminal-activated-${props.tab.id}`, handleTerminalActivated);
  props.emitter.on('terminal-blur-all', handleTerminalBlurEvent);
  window.addEventListener('resize', debouncedResize);

  // Replay any buffered messages that were received while this component was unmounted
  // This ensures no data is lost when switching between projects
  terminalStore.replayBufferedMessages(props.tab.id);
});

function handleTerminalBlurEvent() {
  terminal?.blur();
}

// 处理终端激活事件，首次访问时滚动到底部
function handleTerminalActivated() {
  if (!terminal || !fitAddon) return;

  refreshTerminalViewport('terminal-activated');

  const isFirstVisit = !visitedTerminals.has(props.tab.id);
  if (isFirstVisit) {
    visitedTerminals.add(props.tab.id);
    // 先 fit 确保终端尺寸正确，然后滚动到底部
    try {
      fitAddon.fit();
    } catch (e) {
      // ignore
    }
    // 延迟执行滚动，确保 fit 完成
    setTimeout(() => {
      if (!terminal) return;
      // 先滚动到顶部，再滚动到底部，确保滚动生效
      terminal.scrollToTop();
      setTimeout(() => {
        terminal?.scrollToBottom();
        console.log('[Terminal] First visit after refresh, scrolled to bottom:', props.tab.id);
      }, 100);
    }, 50);
  }
}

onBeforeUnmount(() => {
  persistSnapshot();
  props.emitter.off(props.tab.id, handleMessage);
  props.emitter.off('terminal-resize-all', handleTerminalResizeAll);
  props.emitter.off(`terminal-resize-${props.tab.id}`, handleTerminalResizeAll);
  props.emitter.off(`terminal-activated-${props.tab.id}`, handleTerminalActivated);
  props.emitter.off('terminal-blur-all', handleTerminalBlurEvent);
  window.removeEventListener('resize', debouncedResize);
  if (containerRef.value) {
    if (keydownCaptureHandler) {
      containerRef.value.removeEventListener('keydown', keydownCaptureHandler, true);
    }
    if (pasteHandler) {
      containerRef.value.removeEventListener('paste', pasteHandler, true);
    }
    if (dragOverHandler) {
      containerRef.value.removeEventListener('dragover', dragOverHandler);
    }
    if (dropHandler) {
      containerRef.value.removeEventListener('drop', dropHandler);
    }
  }
  webglAddon?.dispose();
  webglAddon = null;
  terminal?.dispose();
  terminal = null;
  fitAddon?.dispose();
  fitAddon = null;
  serializeAddon?.dispose();
  serializeAddon = null;
  keydownCaptureHandler = null;
  pasteHandler = null;
  dragOverHandler = null;
  dropHandler = null;
  clearTransferOverlay();
});
</script>

<style scoped>
.terminal-viewport {
  position: relative;
  height: 100%;
  width: 100%;
  background-color: var(--kanban-terminal-bg, #0f111a);
}

.terminal-shell {
  height: 100%;
  width: 100%;
}

.terminal-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  background: var(--terminal-overlay-bg, rgba(0, 0, 0, 0.35));
  color: var(--terminal-overlay-color, var(--kanban-terminal-fg, #f6f8ff));
  font-size: 13px;
}

.terminal-transfer-card {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  min-width: 220px;
  max-width: min(320px, calc(100% - 32px));
  padding: 12px 14px;
  border-radius: 12px;
  border: 1px solid var(--terminal-transfer-card-border, rgba(255, 255, 255, 0.14));
  background: var(--terminal-transfer-card-bg, rgba(15, 17, 26, 0.92));
  color: var(--terminal-transfer-card-fg, var(--kanban-terminal-fg, #f6f8ff));
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(10px);
  pointer-events: none;
}

.terminal-transfer-card.is-error {
  border-color: rgba(255, 117, 117, 0.35);
}

.terminal-transfer-message {
  font-size: 13px;
  line-height: 1.4;
}

.terminal-transfer-progress {
  width: 100%;
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--terminal-transfer-card-track, rgba(255, 255, 255, 0.12));
}

.terminal-transfer-progress-fill {
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, rgba(112, 211, 255, 0.95), rgba(116, 170, 156, 0.95));
  transition: width 120ms ease-out;
}

.terminal-transfer-card.is-error .terminal-transfer-progress-fill {
  background: linear-gradient(90deg, rgba(255, 131, 131, 0.95), rgba(255, 180, 117, 0.95));
}

.terminal-transfer-percent {
  font-size: 12px;
  opacity: 0.8;
}
</style>

<style>
.terminal.xterm {
  height: 100%;
}
</style>
