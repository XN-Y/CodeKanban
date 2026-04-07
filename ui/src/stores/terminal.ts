import { defineStore } from 'pinia';
import { reactive, watch } from 'vue';
import EventEmitter from 'eventemitter3';
import Apis, { alovaInstance, urlBase } from '@/api';
import { extractItem } from '@/api/response';
import type { TerminalCreateInputBody } from '@/api/globals';
import type { Task, TerminalModesSnapshot, TerminalSession } from '@/types/models';
import {
  DEFAULT_TERMINAL_RENDER_MODE,
  DEFAULT_TERMINAL_SNAPSHOT_INTERVAL_MS,
  sanitizeTerminalRenderMode,
  sanitizeTerminalSnapshotIntervalMs,
  type TerminalRenderMode,
} from '@/constants/terminalRenderMode';
import { resolveWsUrl } from '@/utils/ws';
import { useProjectStore } from '@/stores/project';
import { useSettingsStore } from '@/stores/settings';
import { useTaskStore } from '@/stores/task';
import { taskActions } from '@/composables/useTaskActions';

export type { TerminalModesSnapshot } from '@/types/models';

export type ClientStatus = 'connecting' | 'ready' | 'closed' | 'error';

export type TerminalRemoteSnapshot = {
  kind: 'full';
  content: string;
  rows: number;
  cols: number;
  sequence: number;
  baseSequence: number;
  altScreen: boolean;
  cursorVisible: boolean;
  modeFlags: number;
  capturedAt?: string;
  lines?: string[];
  cursor?: string;
};

type TerminalRemoteSnapshotDelta = {
  kind: 'delta';
  rows: number;
  cols: number;
  sequence: number;
  baseSequence: number;
  altScreen: boolean;
  cursorVisible: boolean;
  modeFlags: number;
  capturedAt?: string;
  changedLines: Array<{
    index: number;
    content: string;
  }>;
  cursor: string;
};

type TerminalRemoteSnapshotFrame = TerminalRemoteSnapshot | TerminalRemoteSnapshotDelta;

export interface TerminalTabState extends TerminalSession {
  clientStatus: ClientStatus;
  lastAgentCommand?: string;
  renderMode: TerminalRenderMode;
  snapshotIntervalMs: number;
  useGlobalRenderMode: boolean;
  useGlobalSnapshotInterval: boolean;
}

export type TerminalSerializedSnapshot = {
  content: string;
  updatedAt: number;
  rows: number;
  cols: number;
};

export type ServerMessage = {
  type:
    | 'ready'
    | 'data'
    | 'exit'
    | 'error'
    | 'metadata'
    | 'modes'
    | 'snapshot'
    | 'replay-complete'
    | 'render-mode';
  data?: string;
  cols?: number;
  rows?: number;
  mode?: TerminalRenderMode;
  snapshotIntervalMs?: number;
  snapshotCompressionEnabled?: boolean;
  snapshotIncrementalEnabled?: boolean;
  snapshot?: TerminalRemoteSnapshot;
  modes?: TerminalModesSnapshot;
  metadata?: {
    title?: string;
    processPid?: number;
    processStatus?: string;
    processHasChildren?: boolean;
    runningCommand?: string;
    aiAssistantRecentInput?: string;
    taskId?: string;
    aiSessionId?: string;
    aiAssistant?: {
      type: string;
      name: string;
      displayName: string;
      detected: boolean;
      command?: string;
      state?: string;
      stateUpdatedAt?: string;
      interrupted?: boolean;
    };
  };
};

const TERMINAL_SNAPSHOT_PREFIX = '\x1b[0m\x1b[2J\x1b[3J\x1b[H';

export type TerminalCreateOptions = {
  worktreeId?: string;
  workingDir?: string;
  title?: string;
  rows?: number;
  cols?: number;
  taskId?: string;
  /** 插入到指定 sessionId 之后，用于复制标签时保持位置 */
  insertAfterSessionId?: string;
};

type SessionRecord = {
  projectId: string;
  tab: TerminalTabState;
};

type TerminalRenderPreference = {
  useGlobalRenderMode: boolean;
  renderMode?: TerminalRenderMode;
  useGlobalSnapshotInterval: boolean;
  snapshotIntervalMs?: number;
};

const TAB_ORDER_STORAGE_KEY = 'kanban-terminal-tab-order';
const LAST_ACTIVE_TAB_STORAGE_KEY = 'kanban-terminal-last-active';
const TAB_RENDER_PREFERENCE_STORAGE_KEY = 'kanban-terminal-render-preferences';

const storedTabOrders = loadStoredTabOrders();
const storedActiveTabs = loadStoredActiveTabs();
const storedRenderPreferences = loadStoredRenderPreferences();

function cloneTerminalModesSnapshot(
  modes?: TerminalModesSnapshot | null
): TerminalModesSnapshot | undefined {
  if (!modes) {
    return undefined;
  }
  return {
    mouseTracking: modes.mouseTracking,
    mouseSgr: modes.mouseSgr,
    focusReporting: modes.focusReporting,
    bracketedPaste: modes.bracketedPaste,
    alternateScreen: modes.alternateScreen,
  };
}

function loadStoredTabOrders() {
  if (typeof window === 'undefined' || !window.localStorage) {
    return new Map<string, string[]>();
  }
  try {
    const raw = window.localStorage.getItem(TAB_ORDER_STORAGE_KEY);
    if (!raw) {
      return new Map<string, string[]>();
    }
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    const result = new Map<string, string[]>();
    Object.entries(parsed).forEach(([projectId, value]) => {
      if (!projectId || !Array.isArray(value)) {
        return;
      }
      const ids = value
        .map(id => (typeof id === 'string' ? id.trim() : ''))
        .filter((id): id is string => Boolean(id));
      if (ids.length) {
        result.set(projectId, ids);
      }
    });
    return result;
  } catch (error) {
    console.warn('[Terminal Store] Failed to parse stored tab order', error);
    return new Map<string, string[]>();
  }
}

function persistStoredTabOrders() {
  if (typeof window === 'undefined' || !window.localStorage) {
    return;
  }
  if (!storedTabOrders.size) {
    window.localStorage.removeItem(TAB_ORDER_STORAGE_KEY);
    return;
  }
  const payload: Record<string, string[]> = {};
  storedTabOrders.forEach((order, projectId) => {
    if (order.length) {
      payload[projectId] = order;
    }
  });
  if (Object.keys(payload).length === 0) {
    window.localStorage.removeItem(TAB_ORDER_STORAGE_KEY);
    return;
  }
  window.localStorage.setItem(TAB_ORDER_STORAGE_KEY, JSON.stringify(payload));
}

function captureProjectOrder(projectId: string, bucket?: TerminalTabState[]) {
  if (!projectId) {
    return;
  }
  const nextOrder = bucket?.map(tab => tab.id).filter(Boolean) ?? [];
  if (!nextOrder.length) {
    if (storedTabOrders.delete(projectId)) {
      persistStoredTabOrders();
    }
    return;
  }
  const currentOrder = storedTabOrders.get(projectId);
  if (ordersEqual(currentOrder, nextOrder)) {
    return;
  }
  storedTabOrders.set(projectId, nextOrder);
  persistStoredTabOrders();
}

function ordersEqual(current: string[] | undefined, next: string[]) {
  if (!current || current.length !== next.length) {
    return false;
  }
  for (let index = 0; index < current.length; index += 1) {
    if (current[index] !== next[index]) {
      return false;
    }
  }
  return true;
}

function loadStoredActiveTabs() {
  if (typeof window === 'undefined' || !window.localStorage) {
    return new Map<string, string>();
  }
  try {
    const raw = window.localStorage.getItem(LAST_ACTIVE_TAB_STORAGE_KEY);
    if (!raw) {
      return new Map<string, string>();
    }
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    const result = new Map<string, string>();
    Object.entries(parsed).forEach(([projectId, value]) => {
      if (!projectId || typeof value !== 'string') {
        return;
      }
      const normalized = value.trim();
      if (normalized) {
        result.set(projectId, normalized);
      }
    });
    return result;
  } catch (error) {
    console.warn('[Terminal Store] Failed to parse stored active tabs', error);
    return new Map<string, string>();
  }
}

function persistStoredActiveTabs() {
  if (typeof window === 'undefined' || !window.localStorage) {
    return;
  }
  if (!storedActiveTabs.size) {
    window.localStorage.removeItem(LAST_ACTIVE_TAB_STORAGE_KEY);
    return;
  }
  const payload: Record<string, string> = {};
  storedActiveTabs.forEach((tabId, projectId) => {
    if (tabId) {
      payload[projectId] = tabId;
    }
  });
  if (Object.keys(payload).length === 0) {
    window.localStorage.removeItem(LAST_ACTIVE_TAB_STORAGE_KEY);
    return;
  }
  window.localStorage.setItem(LAST_ACTIVE_TAB_STORAGE_KEY, JSON.stringify(payload));
}

function rememberStoredActiveTab(projectId: string, tabId: string) {
  if (!projectId) {
    return;
  }
  const normalized = tabId.trim();
  if (!normalized) {
    forgetStoredActiveTab(projectId);
    return;
  }
  const current = storedActiveTabs.get(projectId);
  if (current === normalized) {
    return;
  }
  storedActiveTabs.set(projectId, normalized);
  persistStoredActiveTabs();
}

function forgetStoredActiveTab(projectId: string) {
  if (!projectId) {
    return;
  }
  if (!storedActiveTabs.has(projectId)) {
    return;
  }
  storedActiveTabs.delete(projectId);
  persistStoredActiveTabs();
}

function loadStoredRenderPreferences() {
  if (typeof window === 'undefined' || !window.localStorage) {
    return new Map<string, TerminalRenderPreference>();
  }
  try {
    const raw = window.localStorage.getItem(TAB_RENDER_PREFERENCE_STORAGE_KEY);
    if (!raw) {
      return new Map<string, TerminalRenderPreference>();
    }
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    const result = new Map<string, TerminalRenderPreference>();
    Object.entries(parsed).forEach(([key, value]) => {
      if (!key || typeof value !== 'object' || value == null) {
        return;
      }
      const preference = sanitizeRenderPreference(value as Partial<TerminalRenderPreference>);
      result.set(key, preference);
    });
    return result;
  } catch (error) {
    console.warn('[Terminal Store] Failed to parse stored render preferences', error);
    return new Map<string, TerminalRenderPreference>();
  }
}

function persistStoredRenderPreferences() {
  if (typeof window === 'undefined' || !window.localStorage) {
    return;
  }
  if (!storedRenderPreferences.size) {
    window.localStorage.removeItem(TAB_RENDER_PREFERENCE_STORAGE_KEY);
    return;
  }
  const payload: Record<string, TerminalRenderPreference> = {};
  storedRenderPreferences.forEach((preference, key) => {
    payload[key] = {
      useGlobalRenderMode: preference.useGlobalRenderMode,
      renderMode: preference.renderMode,
      useGlobalSnapshotInterval: preference.useGlobalSnapshotInterval,
      snapshotIntervalMs: preference.snapshotIntervalMs,
    };
  });
  window.localStorage.setItem(TAB_RENDER_PREFERENCE_STORAGE_KEY, JSON.stringify(payload));
}

function sanitizeRenderPreference(
  value?: Partial<TerminalRenderPreference>
): TerminalRenderPreference {
  return {
    useGlobalRenderMode: value?.useGlobalRenderMode !== false,
    renderMode: sanitizeTerminalRenderMode(value?.renderMode),
    useGlobalSnapshotInterval: value?.useGlobalSnapshotInterval !== false,
    snapshotIntervalMs: sanitizeTerminalSnapshotIntervalMs(value?.snapshotIntervalMs),
  };
}

function buildRenderPreferenceKey(projectId: string, sessionId: string) {
  return `${projectId}:${sessionId}`;
}

function sortSessionsWithStoredOrder(projectId: string, sessions: TerminalSession[]) {
  if (!sessions.length) {
    return sessions;
  }
  const storedOrder = storedTabOrders.get(projectId);
  const ordered = [...sessions];
  if (!storedOrder || storedOrder.length === 0) {
    ordered.sort((a, b) => a.createdAt.localeCompare(b.createdAt) || a.id.localeCompare(b.id));
    return ordered;
  }
  const orderIndex = new Map<string, number>();
  storedOrder.forEach((id, index) => {
    if (id) {
      orderIndex.set(id, index);
    }
  });
  ordered.sort((a, b) => {
    const indexA = orderIndex.get(a.id);
    const indexB = orderIndex.get(b.id);
    if (indexA != null && indexB != null) {
      if (indexA !== indexB) {
        return indexA - indexB;
      }
      return a.createdAt.localeCompare(b.createdAt) || a.id.localeCompare(b.id);
    }
    if (indexA != null) {
      return -1;
    }
    if (indexB != null) {
      return 1;
    }
    return a.createdAt.localeCompare(b.createdAt) || a.id.localeCompare(b.id);
  });
  return ordered;
}

function supportsSnapshotZlibCompression() {
  return typeof DecompressionStream !== 'undefined';
}

async function inflateSnapshotPayload(payload: Uint8Array) {
  if (!supportsSnapshotZlibCompression()) {
    throw new Error('zlib snapshot compression is not supported in this browser');
  }

  const stream = new Blob([payload]).stream().pipeThrough(new DecompressionStream('deflate'));
  const buffer = await new Response(stream).arrayBuffer();
  return new Uint8Array(buffer);
}

function decodeSnapshotText(decoder: TextDecoder, bytes: Uint8Array) {
  if (bytes.byteLength === 0) {
    return '';
  }
  return decoder.decode(bytes);
}

function readSnapshotString(
  view: DataView,
  bytes: Uint8Array,
  offset: number,
  decoder: TextDecoder
) {
  if (offset + 4 > bytes.byteLength) {
    return null;
  }

  const length = view.getUint32(offset, false);
  offset += 4;
  if (offset + length > bytes.byteLength) {
    return null;
  }

  const value = decodeSnapshotText(decoder, bytes.subarray(offset, offset + length));
  return {
    value,
    nextOffset: offset + length,
  };
}

function buildSnapshotContent(lines?: string[], cursor = '', fallbackContent = '') {
  if (!Array.isArray(lines)) {
    return fallbackContent;
  }
  return `${TERMINAL_SNAPSHOT_PREFIX}${lines.join('\r\n')}${cursor}`;
}

function parseVersion6SnapshotFrame(
  rows: number,
  cols: number,
  frameKind: number,
  sequence: number,
  baseSequence: number,
  altScreen: boolean,
  cursorVisible: boolean,
  modeFlags: number,
  capturedAt: string | undefined,
  encodedContent: Uint8Array
): TerminalRemoteSnapshotFrame | null {
  const view = new DataView(
    encodedContent.buffer,
    encodedContent.byteOffset,
    encodedContent.byteLength
  );
  const decoder = new TextDecoder('utf-8');
  let offset = 0;

  if (frameKind === 1) {
    if (offset + 2 > encodedContent.byteLength) {
      return null;
    }

    const changeCount = view.getUint16(offset, false);
    offset += 2;
    const changedLines: TerminalRemoteSnapshotDelta['changedLines'] = [];
    for (let index = 0; index < changeCount; index += 1) {
      if (offset + 2 > encodedContent.byteLength) {
        return null;
      }
      const rowIndex = view.getUint16(offset, false);
      offset += 2;
      const rowValue = readSnapshotString(view, encodedContent, offset, decoder);
      if (!rowValue) {
        return null;
      }
      offset = rowValue.nextOffset;
      changedLines.push({
        index: rowIndex,
        content: rowValue.value,
      });
    }

    const cursorValue = readSnapshotString(view, encodedContent, offset, decoder);
    if (!cursorValue) {
      return null;
    }

    return {
      kind: 'delta',
      rows,
      cols,
      sequence,
      baseSequence,
      altScreen,
      cursorVisible,
      modeFlags,
      capturedAt,
      changedLines,
      cursor: cursorValue.value,
    };
  }

  const lines: string[] = [];
  for (let row = 0; row < rows; row += 1) {
    const rowValue = readSnapshotString(view, encodedContent, offset, decoder);
    if (!rowValue) {
      return null;
    }
    offset = rowValue.nextOffset;
    lines.push(rowValue.value);
  }

  const cursorValue = readSnapshotString(view, encodedContent, offset, decoder);
  if (!cursorValue) {
    return null;
  }

  return {
    kind: 'full',
    rows,
    cols,
    sequence,
    baseSequence,
    altScreen,
    cursorVisible,
    modeFlags,
    capturedAt,
    lines,
    cursor: cursorValue.value,
    content: buildSnapshotContent(lines, cursorValue.value),
  };
}

async function parseBinarySnapshotFrame(
  payload: ArrayBuffer
): Promise<TerminalRemoteSnapshotFrame | null> {
  if (!(payload instanceof ArrayBuffer) || payload.byteLength < 27) {
    return null;
  }

  const view = new DataView(payload);
  const version = view.getUint8(0);
  if (version !== 6) {
    return null;
  }

  const rows = view.getUint16(1, false);
  const cols = view.getUint16(3, false);
  const capturedAtMs = Number(view.getBigUint64(5, false));
  const headerSize = 27;
  if (payload.byteLength < headerSize) {
    return null;
  }
  const flags = view.getUint8(13);
  const altScreen = (flags & (1 << 0)) !== 0;
  const cursorVisible = (flags & (1 << 1)) !== 0;
  const modeFlags = view.getUint32(14, false);
  const compressed = (flags & (1 << 2)) !== 0;
  const sequence = view.getUint32(18, false);
  const baseSequence = view.getUint32(22, false);
  const frameKind = view.getUint8(26);
  const encodedContent = new Uint8Array(payload, headerSize);
  const contentBytes = compressed ? await inflateSnapshotPayload(encodedContent) : encodedContent;
  const capturedAt =
    capturedAtMs > 0 && Number.isFinite(capturedAtMs)
      ? new Date(capturedAtMs).toISOString()
      : undefined;

  return parseVersion6SnapshotFrame(
    rows,
    cols,
    frameKind,
    sequence,
    baseSequence,
    altScreen,
    cursorVisible,
    modeFlags,
    capturedAt,
    contentBytes
  );
}

function assembleServerSnapshotFrame(
  previous: TerminalRemoteSnapshot | undefined,
  frame: TerminalRemoteSnapshotFrame
): TerminalRemoteSnapshot | null {
  if (frame.kind === 'full') {
    return {
      ...frame,
      kind: 'full',
      content: buildSnapshotContent(frame.lines, frame.cursor, frame.content),
    };
  }

  if (!previous || !Array.isArray(previous.lines)) {
    return null;
  }
  if (previous.sequence !== frame.baseSequence) {
    return null;
  }
  if (
    previous.rows !== frame.rows ||
    previous.cols !== frame.cols ||
    previous.altScreen !== frame.altScreen ||
    previous.modeFlags !== frame.modeFlags
  ) {
    return null;
  }

  const lines = [...previous.lines];
  for (const patch of frame.changedLines) {
    if (patch.index < 0 || patch.index >= lines.length) {
      return null;
    }
    lines[patch.index] = patch.content;
  }

  const cursor = frame.cursor ?? previous.cursor ?? '';
  return {
    kind: 'full',
    rows: frame.rows,
    cols: frame.cols,
    sequence: frame.sequence,
    baseSequence: frame.baseSequence,
    altScreen: frame.altScreen,
    cursorVisible: frame.cursorVisible,
    modeFlags: frame.modeFlags,
    capturedAt: frame.capturedAt,
    lines,
    cursor,
    content: buildSnapshotContent(lines, cursor),
  };
}

export const useTerminalStore = defineStore('terminal', () => {
  const tabStore = reactive(new Map<string, TerminalTabState[]>());
  const sessionIndex = new Map<string, SessionRecord>();
  const activeTabByProject = reactive(new Map<string, string>());
  const sockets = new Map<string, WebSocket>();
  const manualCloseIds = new Set<string>();
  const pausedSocketIds = new Set<string>();
  let globalLoadToken = 0;
  const projectLoadTokens = new Map<string, number>();
  const emitter = new EventEmitter();
  const cachedCounts = reactive(new Map<string, number>());
  const projectConnectionRefCounts = reactive(new Map<string, number>());
  // Track AI assistant state for each session to detect state changes
  const aiPreviousStates = new Map<string, string>();
  // Track whether AI agent has been detected for each session (to emit ai:detected only once)
  const aiDetectedSessions = new Set<string>();
  // Get project store for looking up project names
  const projectStore = useProjectStore();
  const settingsStore = useSettingsStore();
  const taskStore = useTaskStore();
  const sessionToTaskMap = reactive(new Map<string, string>());
  const taskToSessionMap = reactive(new Map<string, string>());
  const pendingTaskFetch = new Set<string>();
  // Buffer for WebSocket messages when no listener is attached
  // This prevents data loss when TerminalViewport is unmounted but WebSocket is still active
  const messageBuffers = new Map<string, ServerMessage[]>();
  const MESSAGE_BUFFER_MAX_SIZE = 5000; // Limit buffer size to prevent memory issues
  const latestServerSnapshots = new Map<string, TerminalRemoteSnapshot>();
  const latestServerSnapshotSequence = new Map<string, number>();
  const serializedSnapshots = new Map<string, TerminalSerializedSnapshot>();

  function getGlobalRenderMode() {
    return sanitizeTerminalRenderMode(
      settingsStore.defaultTerminalRenderMode ?? DEFAULT_TERMINAL_RENDER_MODE
    );
  }

  function getGlobalSnapshotIntervalMs() {
    return sanitizeTerminalSnapshotIntervalMs(
      settingsStore.defaultTerminalSnapshotIntervalMs ?? DEFAULT_TERMINAL_SNAPSHOT_INTERVAL_MS
    );
  }

  function getGlobalSnapshotCompressionEnabled() {
    return (
      settingsStore.defaultTerminalSnapshotZlibCompression !== false &&
      supportsSnapshotZlibCompression()
    );
  }

  function getStoredRenderPreference(projectId: string, sessionId: string) {
    return storedRenderPreferences.get(buildRenderPreferenceKey(projectId, sessionId));
  }

  function getEffectiveRenderMode(projectId: string, sessionId: string) {
    const preference = getStoredRenderPreference(projectId, sessionId);
    if (!preference || preference.useGlobalRenderMode) {
      return getGlobalRenderMode();
    }
    return sanitizeTerminalRenderMode(preference.renderMode);
  }

  function getEffectiveSnapshotIntervalMs(projectId: string, sessionId: string) {
    const preference = getStoredRenderPreference(projectId, sessionId);
    if (!preference || preference.useGlobalSnapshotInterval) {
      return getGlobalSnapshotIntervalMs();
    }
    return sanitizeTerminalSnapshotIntervalMs(preference.snapshotIntervalMs);
  }

  function applyRenderPreferenceToTab(tab: TerminalTabState) {
    const preference = getStoredRenderPreference(tab.projectId, tab.id);
    tab.useGlobalRenderMode = preference?.useGlobalRenderMode !== false;
    tab.useGlobalSnapshotInterval = preference?.useGlobalSnapshotInterval !== false;
    tab.renderMode = getEffectiveRenderMode(tab.projectId, tab.id);
    tab.snapshotIntervalMs = getEffectiveSnapshotIntervalMs(tab.projectId, tab.id);
    return tab;
  }

  function persistRenderPreference(
    projectId: string,
    sessionId: string,
    partial: Partial<TerminalRenderPreference>
  ) {
    const storageKey = buildRenderPreferenceKey(projectId, sessionId);
    const current = storedRenderPreferences.get(storageKey);
    const next = sanitizeRenderPreference({
      ...current,
      ...partial,
    });
    storedRenderPreferences.set(storageKey, next);
    persistStoredRenderPreferences();
    return next;
  }

  function clearRenderPreference(projectId: string, sessionId: string) {
    const storageKey = buildRenderPreferenceKey(projectId, sessionId);
    if (storedRenderPreferences.delete(storageKey)) {
      persistStoredRenderPreferences();
    }
  }

  function buildRenderModeMessage(
    sessionId: string
  ): Pick<
    ServerMessage,
    | 'type'
    | 'mode'
    | 'snapshotIntervalMs'
    | 'snapshotCompressionEnabled'
    | 'snapshotIncrementalEnabled'
  > | null {
    const record = sessionIndex.get(sessionId);
    if (!record) {
      return null;
    }
    return {
      type: 'render-mode',
      mode: getEffectiveRenderMode(record.projectId, sessionId),
      snapshotIntervalMs: getEffectiveSnapshotIntervalMs(record.projectId, sessionId),
      snapshotCompressionEnabled: getGlobalSnapshotCompressionEnabled(),
      snapshotIncrementalEnabled: true,
    };
  }

  function sendRenderModePreference(sessionId: string) {
    const message = buildRenderModeMessage(sessionId);
    if (!message) {
      return false;
    }
    return send(sessionId, message);
  }

  function updateRenderModeAck(
    sessionId: string,
    mode: TerminalRenderMode | undefined,
    snapshotIntervalMs: number | undefined,
    _snapshotCompressionEnabled: boolean | undefined,
    _snapshotIncrementalEnabled: boolean | undefined
  ) {
    const record = sessionIndex.get(sessionId);
    if (!record) {
      return;
    }
    const bucket = tabStore.get(record.projectId);
    if (!bucket) {
      return;
    }
    const index = bucket.findIndex(tab => tab.id === sessionId);
    if (index === -1) {
      return;
    }
    bucket[index] = {
      ...bucket[index],
      renderMode: sanitizeTerminalRenderMode(mode ?? bucket[index].renderMode),
      snapshotIntervalMs: sanitizeTerminalSnapshotIntervalMs(
        snapshotIntervalMs ?? bucket[index].snapshotIntervalMs
      ),
    };
    record.tab = bucket[index];
  }

  function applyGlobalRenderDefaultsToTabs() {
    sessionIndex.forEach(({ projectId }, sessionId) => {
      const record = sessionIndex.get(sessionId);
      if (!record) {
        return;
      }
      const bucket = tabStore.get(projectId);
      if (!bucket) {
        return;
      }
      const index = bucket.findIndex(tab => tab.id === sessionId);
      if (index === -1) {
        return;
      }
      const current = bucket[index];
      const nextRenderMode = current.useGlobalRenderMode
        ? getGlobalRenderMode()
        : current.renderMode;
      const nextSnapshotIntervalMs = current.useGlobalSnapshotInterval
        ? getGlobalSnapshotIntervalMs()
        : current.snapshotIntervalMs;
      bucket[index] = {
        ...current,
        renderMode: sanitizeTerminalRenderMode(nextRenderMode),
        snapshotIntervalMs: sanitizeTerminalSnapshotIntervalMs(nextSnapshotIntervalMs),
      };
      record.tab = bucket[index];
      void sendRenderModePreference(sessionId);
    });
  }

  watch(
    () => [
      settingsStore.defaultTerminalRenderMode,
      settingsStore.defaultTerminalSnapshotIntervalMs,
      settingsStore.defaultTerminalSnapshotZlibCompression,
    ],
    () => {
      applyGlobalRenderDefaultsToTabs();
    }
  );

  // Helper function to get project name by ID
  function getProjectName(projectId: string): string | undefined {
    const project = projectStore.projects.find(p => p.id === projectId);
    return project?.name;
  }

  function updateSessionTaskMapping(sessionId: string, nextTaskId?: string | null) {
    const previousTaskId = sessionToTaskMap.get(sessionId);
    const record = sessionIndex.get(sessionId);
    if (previousTaskId && previousTaskId !== nextTaskId) {
      sessionToTaskMap.delete(sessionId);
      if (taskToSessionMap.get(previousTaskId) === sessionId) {
        taskToSessionMap.delete(previousTaskId);
      }
    }
    if (nextTaskId) {
      sessionToTaskMap.set(sessionId, nextTaskId);
      taskToSessionMap.set(nextTaskId, sessionId);
    } else if (!nextTaskId) {
      sessionToTaskMap.delete(sessionId);
    }

    if (nextTaskId && nextTaskId !== previousTaskId) {
      void ensureTaskLoaded(nextTaskId);
    }
  }

  async function ensureTaskLoaded(taskId: string) {
    if (!taskId || pendingTaskFetch.has(taskId)) {
      return;
    }
    if (taskStore.tasks.some(task => task.id === taskId)) {
      return;
    }
    pendingTaskFetch.add(taskId);
    try {
      const response = await taskActions.getTask.send(taskId);
      const task = response?.item as unknown as Task | undefined;
      if (task) {
        taskStore.upsertTask(task);
      }
    } catch (error) {
      console.error(`[Terminal] Failed to fetch linked task ${taskId}`, error);
    } finally {
      pendingTaskFetch.delete(taskId);
    }
  }

  function getTabs(projectId?: string) {
    if (!projectId) {
      return [];
    }
    return tabStore.get(projectId) ?? [];
  }

  function getActiveTabId(projectId?: string) {
    if (!projectId) {
      return '';
    }
    const bucket = tabStore.get(projectId);
    if (!bucket || bucket.length === 0) {
      return '';
    }
    const current = activeTabByProject.get(projectId);
    if (current && bucket.some(tab => tab.id === current)) {
      rememberStoredActiveTab(projectId, current);
      return current;
    }
    if (tryRestoreActiveTabFromStorage(projectId)) {
      const restored = activeTabByProject.get(projectId) ?? '';
      if (restored) {
        rememberStoredActiveTab(projectId, restored);
        return restored;
      }
    }
    const fallback = bucket[0]?.id ?? '';
    if (fallback) {
      setActiveTab(projectId, fallback);
      return fallback;
    }
    return '';
  }

  function setActiveTab(projectId: string | undefined, tabId?: string) {
    if (!projectId) {
      return;
    }
    const normalized = typeof tabId === 'string' ? tabId.trim() : '';
    if (normalized) {
      const current = activeTabByProject.get(projectId);
      if (current !== normalized) {
        activeTabByProject.set(projectId, normalized);
      }
      rememberStoredActiveTab(projectId, normalized);
      return;
    }
    if (activeTabByProject.has(projectId)) {
      activeTabByProject.delete(projectId);
    }
    forgetStoredActiveTab(projectId);
  }

  function prepareProject(projectId: string) {
    ensureBucket(projectId);
    ensureActiveTab(projectId);
  }

  async function loadSessions(projectId?: string) {
    const resolved = ensureProjectSelected(projectId);
    const token = ++globalLoadToken;
    projectLoadTokens.set(resolved, token);
    try {
      const response = await Apis.terminalSession
        .list({
          pathParams: { projectId: resolved },
          cacheFor: 0,
        })
        .send();
      if (projectLoadTokens.get(resolved) !== token) {
        return;
      }
      const items = response?.items ?? [];
      reconcileSessions(resolved, items as unknown as TerminalSession[]);
      // 更新终端计数缓存
      cachedCounts.set(resolved, items.length);
    } catch (error) {
      console.error('Failed to load terminal sessions', error);
    }
  }

  async function createSession(
    projectId: string | undefined,
    options: TerminalCreateOptions
  ): Promise<string> {
    const resolved = ensureProjectSelected(projectId);

    // 如果没有提供 worktreeId，自动选择
    let worktreeId = options.worktreeId;
    if (!worktreeId) {
      const projectStore = useProjectStore();
      const worktrees = projectStore.worktrees;

      if (worktrees.length === 0) {
        throw new Error('当前项目没有可用的分支');
      }

      // 优先选择主分支，否则选择第一个
      const mainWorktree = worktrees.find(w => w.isMain);
      worktreeId = mainWorktree ? mainWorktree.id : worktrees[0].id;
    }

    const payload: TerminalCreateInputBody = {
      workingDir: options.workingDir ?? '',
      title: options.title ?? '',
      rows: options.rows ?? 0,
      cols: options.cols ?? 0,
    };
    if (options.taskId) {
      payload.taskId = options.taskId;
    }
    const response = await Apis.terminalSession
      .create({
        pathParams: {
          projectId: resolved,
          worktreeId: worktreeId,
        },
        data: payload,
        cacheFor: 0,
      })
      .send();
    if (!response?.item) {
      throw new Error('�����ն�ʧ��');
    }
    const session = response.item as unknown as TerminalSession;
    attachOrUpdateSession(session, {
      activate: true,
      projectIdOverride: resolved,
      insertAfterSessionId: options.insertAfterSessionId,
    });
    emitter.emit('terminal:created', {
      projectId: resolved,
      sessionId: session.id,
    });
    // 更新终端计数缓存
    const currentCount = cachedCounts.get(resolved) ?? 0;
    cachedCounts.set(resolved, currentCount + 1);
    return session.id;
  }

  async function renameSession(projectId: string | undefined, sessionId: string, title: string) {
    const resolved = ensureProjectSelected(projectId);
    const normalized = title.trim();
    if (!normalized) {
      throw new Error('��������µ��ն˱��⡣');
    }
    const response = await Apis.terminalSession
      .rename({
        pathParams: {
          projectId: resolved,
          sessionId,
        },
        data: {
          title: normalized,
        },
        cacheFor: 0,
      })
      .send();
    if (!response?.item) {
      return;
    }
    attachOrUpdateSession(response.item as unknown as TerminalSession, {
      projectIdOverride: resolved,
    });
  }

  async function closeSession(projectId: string | undefined, sessionId: string) {
    const resolved = ensureProjectSelected(projectId);
    await Apis.terminalSession
      .close({
        pathParams: { projectId: resolved, sessionId },
        cacheFor: 0,
      })
      .send();
    disconnectTab(sessionId, true);
  }

  async function linkSessionTask(projectId: string | undefined, sessionId: string, taskId: string) {
    const resolved = ensureProjectSelected(projectId);
    const response = await alovaInstance
      .Post(
        `/api/v1/projects/${resolved}/terminals/${sessionId}/tasks/link`,
        { taskId },
        { cacheFor: 0 }
      )
      .send();
    const session = extractItem(response) as unknown as TerminalSession | undefined;
    if (session) {
      attachOrUpdateSession(session, { projectIdOverride: resolved });
    }
    updateSessionTaskMapping(sessionId, taskId);
    return session;
  }

  async function unlinkSessionTask(projectId: string | undefined, sessionId: string) {
    const resolved = ensureProjectSelected(projectId);
    const response = await alovaInstance
      .Post(`/api/v1/projects/${resolved}/terminals/${sessionId}/tasks/unlink`, {}, { cacheFor: 0 })
      .send();
    const session = extractItem(response) as unknown as TerminalSession | undefined;
    if (session) {
      attachOrUpdateSession(session, { projectIdOverride: resolved });
    }
    updateSessionTaskMapping(sessionId);
    return session;
  }

  function setSessionRenderMode(
    projectId: string | undefined,
    sessionId: string,
    mode: TerminalRenderMode | null
  ) {
    const resolved = ensureProjectSelected(projectId);
    const record = sessionIndex.get(sessionId);
    if (!record || record.projectId !== resolved) {
      return false;
    }

    const nextPreference =
      mode == null
        ? { useGlobalRenderMode: true }
        : { useGlobalRenderMode: false, renderMode: sanitizeTerminalRenderMode(mode) };
    persistRenderPreference(resolved, sessionId, nextPreference);

    const bucket = tabStore.get(resolved);
    if (!bucket) {
      return false;
    }
    const index = bucket.findIndex(tab => tab.id === sessionId);
    if (index === -1) {
      return false;
    }
    bucket[index] = applyRenderPreferenceToTab({
      ...bucket[index],
    });
    record.tab = bucket[index];
    return sendRenderModePreference(sessionId);
  }

  function setSessionSnapshotInterval(
    projectId: string | undefined,
    sessionId: string,
    snapshotIntervalMs: number | null
  ) {
    const resolved = ensureProjectSelected(projectId);
    const record = sessionIndex.get(sessionId);
    if (!record || record.projectId !== resolved) {
      return false;
    }

    const nextPreference =
      snapshotIntervalMs == null
        ? { useGlobalSnapshotInterval: true }
        : {
            useGlobalSnapshotInterval: false,
            snapshotIntervalMs: sanitizeTerminalSnapshotIntervalMs(snapshotIntervalMs),
          };
    persistRenderPreference(resolved, sessionId, nextPreference);

    const bucket = tabStore.get(resolved);
    if (!bucket) {
      return false;
    }
    const index = bucket.findIndex(tab => tab.id === sessionId);
    if (index === -1) {
      return false;
    }
    bucket[index] = applyRenderPreferenceToTab({
      ...bucket[index],
    });
    record.tab = bucket[index];
    return sendRenderModePreference(sessionId);
  }

  function send(sessionId: string, message: any): boolean {
    const socket = sockets.get(sessionId);
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(message));
      return true;
    }
    return false;
  }

  function isProjectConnectionActive(projectId: string | undefined) {
    if (!projectId) {
      return false;
    }
    return (projectConnectionRefCounts.get(projectId) ?? 0) > 0;
  }

  function pauseSocket(sessionId: string) {
    const socket = sockets.get(sessionId);
    if (!socket) {
      return;
    }
    pausedSocketIds.add(sessionId);
    socket.close();
    sockets.delete(sessionId);
  }

  function disconnectTab(sessionId: string, remove = true) {
    const socket = sockets.get(sessionId);
    if (socket) {
      manualCloseIds.add(sessionId);
      socket.close();
      sockets.delete(sessionId);
    }
    pausedSocketIds.delete(sessionId);
    if (remove) {
      const record = sessionIndex.get(sessionId);
      if (!record) {
        return;
      }
      const bucket = tabStore.get(record.projectId);
      if (bucket) {
        const index = bucket.findIndex(tab => tab.id === sessionId);
        if (index !== -1) {
          bucket.splice(index, 1);
          // 更新终端计数缓存
          const currentCount = cachedCounts.get(record.projectId) ?? 0;
          cachedCounts.set(record.projectId, Math.max(0, currentCount - 1));
        }
        captureProjectOrder(record.projectId, bucket);
        if (bucket.length === 0) {
          tabStore.delete(record.projectId);
        }
      }
      sessionIndex.delete(sessionId);
      aiPreviousStates.delete(sessionId); // Clean up AI state tracking
      aiDetectedSessions.delete(sessionId); // Clean up AI detected tracking
      updateSessionTaskMapping(sessionId);
      if (activeTabByProject.get(record.projectId) === sessionId) {
        const nextId = tabStore.get(record.projectId)?.[0]?.id;
        setActiveTab(record.projectId, nextId);
      }
      messageBuffers.delete(sessionId); // Clean up message buffer
      latestServerSnapshots.delete(sessionId);
      latestServerSnapshotSequence.delete(sessionId);
      serializedSnapshots.delete(sessionId);
      clearRenderPreference(record.projectId, sessionId);
    }
  }

  /**
   * Replay buffered messages for a session and clear the buffer.
   * Called when TerminalViewport remounts after being unmounted (e.g., project switch).
   */
  function replayBufferedMessages(sessionId: string) {
    const buffer = messageBuffers.get(sessionId);
    if (!buffer || buffer.length === 0) {
      return;
    }
    // Emit all buffered messages
    for (const message of buffer) {
      emitter.emit(sessionId, message);
    }
    // Clear the buffer after replay
    messageBuffers.delete(sessionId);
    console.log(`[Terminal] Replayed ${buffer.length} buffered messages for session:`, sessionId);
  }

  function saveSerializedSnapshot(sessionId: string, snapshot?: TerminalSerializedSnapshot | null) {
    if (!sessionId) {
      return;
    }
    if (!snapshot) {
      serializedSnapshots.delete(sessionId);
      return;
    }
    serializedSnapshots.set(sessionId, snapshot);
  }

  function getSerializedSnapshot(sessionId: string) {
    if (!sessionId) {
      return undefined;
    }
    return serializedSnapshots.get(sessionId);
  }

  function getLatestServerSnapshot(sessionId: string) {
    if (!sessionId) {
      return undefined;
    }
    return latestServerSnapshots.get(sessionId);
  }

  function applyServerSnapshotFrame(sessionId: string, frame: TerminalRemoteSnapshotFrame) {
    if (!sessionId) {
      return null;
    }

    const assembled = assembleServerSnapshotFrame(latestServerSnapshots.get(sessionId), frame);
    if (!assembled) {
      send(sessionId, {
        type: 'snapshot-request',
        reason: 'delta-baseline-miss',
      });
      return null;
    }

    latestServerSnapshots.set(sessionId, assembled);
    return assembled;
  }

  function ensureBucket(projectId: string) {
    if (!projectId) {
      return [];
    }
    let bucket = tabStore.get(projectId);
    if (!bucket) {
      bucket = reactive<TerminalTabState[]>([]);
      tabStore.set(projectId, bucket);
    }
    return bucket;
  }

  function tryRestoreActiveTabFromStorage(projectId: string) {
    if (!projectId) {
      return false;
    }
    const storedId = storedActiveTabs.get(projectId);
    if (!storedId) {
      return false;
    }
    const bucket = tabStore.get(projectId);
    if (!bucket || bucket.length === 0) {
      return false;
    }
    const exists = bucket.some(tab => tab.id === storedId);
    if (!exists) {
      forgetStoredActiveTab(projectId);
      return false;
    }
    setActiveTab(projectId, storedId);
    return true;
  }

  function ensureActiveTab(projectId: string) {
    if (!projectId) {
      return;
    }
    const bucket = tabStore.get(projectId);
    if (!bucket || bucket.length === 0) {
      return;
    }
    const current = activeTabByProject.get(projectId);
    if (current && bucket.some(tab => tab.id === current)) {
      rememberStoredActiveTab(projectId, current);
      return;
    }
    if (tryRestoreActiveTabFromStorage(projectId)) {
      return;
    }
    setActiveTab(projectId, bucket[0].id);
  }

  function reorderTabs(projectId: string | undefined, fromIndex: number, toIndex: number) {
    if (!projectId) {
      return;
    }
    const bucket = tabStore.get(projectId);
    if (!bucket || bucket.length < 2) {
      return;
    }
    if (fromIndex === toIndex) {
      return;
    }
    if (fromIndex < 0 || fromIndex >= bucket.length) {
      return;
    }
    const clampedToIndex = Math.max(0, Math.min(bucket.length - 1, toIndex));
    const [tab] = bucket.splice(fromIndex, 1);
    if (!tab) {
      return;
    }
    bucket.splice(clampedToIndex, 0, tab);
    captureProjectOrder(projectId, bucket);
  }

  function attachOrUpdateSession(
    session: TerminalSession,
    options?: { activate?: boolean; projectIdOverride?: string; insertAfterSessionId?: string }
  ) {
    const existing = sessionIndex.get(session.id);
    if (existing) {
      const immutableProjectId = existing.projectId;
      const payloadProjectId = normalizeProjectId(session.projectId);
      if (payloadProjectId && payloadProjectId !== immutableProjectId) {
        console.warn(
          '[Terminal Store] Received mismatched project for terminal session',
          session.id,
          'payload project:',
          payloadProjectId,
          'tracked as:',
          immutableProjectId
        );
      }
      // 直接使用 session.taskId，如果是 null/undefined 则清除关联
      const updatedTaskId = session.taskId;
      const updatedTab: TerminalTabState = {
        ...existing.tab,
        ...session,
        projectId: immutableProjectId,
        taskId: updatedTaskId ?? undefined,
        terminalModes: cloneTerminalModesSnapshot(session.terminalModes ?? existing.tab.terminalModes),
        renderMode: existing.tab.renderMode,
        snapshotIntervalMs: existing.tab.snapshotIntervalMs,
        useGlobalRenderMode: existing.tab.useGlobalRenderMode,
        useGlobalSnapshotInterval: existing.tab.useGlobalSnapshotInterval,
      };
      // 用 splice 替换以触发 Vue 响应式更新
      const bucket = tabStore.get(immutableProjectId);
      if (bucket) {
        const index = bucket.findIndex(t => t.id === session.id);
        if (index !== -1) {
          bucket.splice(index, 1, updatedTab);
        }
      }
      existing.tab = updatedTab;
      updateSessionTaskMapping(session.id, updatedTaskId ?? undefined);
      if (options?.activate) {
        setActiveTab(immutableProjectId, session.id);
      }
      if (!sockets.has(session.id) && isProjectConnectionActive(immutableProjectId)) {
        connect(updatedTab);
      }
      return updatedTab;
    }

    const resolvedProjectId = resolveSessionProjectId(session, options?.projectIdOverride);
    if (!resolvedProjectId) {
      console.warn('[Terminal Store] Skip terminal session with unknown projectId', session.id);
      return;
    }

    const bucket = ensureBucket(resolvedProjectId);
    const tab = applyRenderPreferenceToTab({
      ...session,
      projectId: resolvedProjectId,
      clientStatus: 'connecting',
      terminalModes: cloneTerminalModesSnapshot(session.terminalModes),
      renderMode: getEffectiveRenderMode(resolvedProjectId, session.id),
      snapshotIntervalMs: getEffectiveSnapshotIntervalMs(resolvedProjectId, session.id),
      useGlobalRenderMode: true,
      useGlobalSnapshotInterval: true,
    });
    // 如果指定了 insertAfterSessionId，在其后插入；否则添加到末尾
    if (options?.insertAfterSessionId) {
      const insertIndex = bucket.findIndex(t => t.id === options.insertAfterSessionId);
      if (insertIndex !== -1) {
        bucket.splice(insertIndex + 1, 0, tab);
      } else {
        bucket.push(tab);
      }
    } else {
      bucket.push(tab);
    }
    sessionIndex.set(tab.id, { projectId: resolvedProjectId, tab });
    updateSessionTaskMapping(tab.id, tab.taskId ?? undefined);
    captureProjectOrder(resolvedProjectId, bucket);
    if (options?.activate) {
      setActiveTab(resolvedProjectId, tab.id);
    } else if (!activeTabByProject.get(resolvedProjectId)) {
      const storedId = storedActiveTabs.get(resolvedProjectId);
      if (!storedId) {
        setActiveTab(resolvedProjectId, tab.id);
      } else if (storedId === tab.id) {
        setActiveTab(resolvedProjectId, tab.id);
      }
    }
    if (isProjectConnectionActive(resolvedProjectId)) {
      connect(tab);
    }
    return tab;
  }

  function updateTabStatus(sessionId: string, status: ClientStatus) {
    const record = sessionIndex.get(sessionId);
    if (!record) return;

    const bucket = tabStore.get(record.projectId);
    if (!bucket) return;

    const index = bucket.findIndex(t => t.id === sessionId);
    if (index === -1) return;

    bucket[index] = { ...bucket[index], clientStatus: status };
    record.tab = bucket[index];
  }

  function updateTabMetadata(sessionId: string, metadata: ServerMessage['metadata']) {
    const record = sessionIndex.get(sessionId);
    if (!record || !metadata) return;

    const bucket = tabStore.get(record.projectId);
    if (!bucket) return;

    const index = bucket.findIndex(t => t.id === sessionId);
    if (index === -1) return;

    const nextTaskId = metadata.taskId ?? bucket[index].taskId;
    const nextTitle = metadata.title;
    const latestCommand =
      typeof metadata.aiAssistantRecentInput === 'string' && metadata.aiAssistantRecentInput.trim()
        ? metadata.aiAssistantRecentInput.trim()
        : '';

    bucket[index] = {
      ...bucket[index],
      processPid: metadata.processPid,
      processStatus: metadata.processStatus as 'idle' | 'busy' | 'unknown' | undefined,
      processHasChildren: metadata.processHasChildren,
      runningCommand: metadata.runningCommand,
      aiAssistant: metadata.aiAssistant,
      taskId: nextTaskId,
      aiSessionId: metadata.aiSessionId || bucket[index].aiSessionId,
      title: typeof nextTitle === 'string' ? nextTitle : bucket[index].title,
      lastAgentCommand: latestCommand || bucket[index].lastAgentCommand,
    };
    record.tab = bucket[index];
    updateSessionTaskMapping(sessionId, nextTaskId ?? undefined);
  }

  function updateTabTerminalModes(sessionId: string, modes?: TerminalModesSnapshot) {
    const record = sessionIndex.get(sessionId);
    if (!record) return;

    const bucket = tabStore.get(record.projectId);
    if (!bucket) return;

    const index = bucket.findIndex(t => t.id === sessionId);
    if (index === -1) return;

    bucket[index] = {
      ...bucket[index],
      terminalModes: cloneTerminalModesSnapshot(modes),
    };
    record.tab = bucket[index];
  }

  function connect(tab: TerminalTabState) {
    if (!isProjectConnectionActive(tab.projectId)) {
      return;
    }
    const existingSocket = sockets.get(tab.id);
    if (
      existingSocket &&
      (existingSocket.readyState === WebSocket.OPEN ||
        existingSocket.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }
    pausedSocketIds.delete(tab.id);
    const resolvedWsURL = resolveWsUrl(tab.wsUrl || tab.wsPath, urlBase);
    const wsURL = new URL(resolvedWsURL);
    wsURL.searchParams.set(
      'snapshotCompression',
      getGlobalSnapshotCompressionEnabled() ? 'zlib' : 'none'
    );
    const socket = new WebSocket(wsURL.toString());
    socket.binaryType = 'arraybuffer';
    sockets.set(tab.id, socket);

    socket.addEventListener('open', () => {
      updateTabStatus(tab.id, 'ready');
      latestServerSnapshotSequence.delete(tab.id);
      const renderModeMessage = buildRenderModeMessage(tab.id);
      if (renderModeMessage) {
        socket.send(JSON.stringify(renderModeMessage));
      }
      socket.send(
        JSON.stringify({
          type: 'resize',
          cols: tab.cols,
          rows: tab.rows,
        })
      );
    });

    socket.addEventListener('message', event => {
      void (async () => {
        try {
          if (sockets.get(tab.id) !== socket) {
            return;
          }

          let payload: ServerMessage;
          if (typeof event.data === 'string') {
            payload = JSON.parse(event.data) as ServerMessage;
          } else {
            const frame = await parseBinarySnapshotFrame(event.data as ArrayBuffer);
            if (!frame || sockets.get(tab.id) !== socket) {
              return;
            }
            const previousSequence = latestServerSnapshotSequence.get(tab.id) ?? -1;
            if (frame.sequence <= previousSequence) {
              return;
            }
            latestServerSnapshotSequence.set(tab.id, frame.sequence);
            const snapshot = applyServerSnapshotFrame(tab.id, frame);
            if (!snapshot || sockets.get(tab.id) !== socket) {
              return;
            }
            payload = {
              type: 'snapshot',
              snapshot,
            };
          }
          if (payload.type === 'render-mode') {
            updateRenderModeAck(
              tab.id,
              payload.mode,
              payload.snapshotIntervalMs,
              payload.snapshotCompressionEnabled,
              payload.snapshotIncrementalEnabled
            );
          }
          if (payload.type === 'ready') {
            updateTabStatus(tab.id, 'ready');
          } else if (payload.type === 'exit') {
            updateTabStatus(tab.id, 'closed');
          } else if (payload.type === 'error') {
            updateTabStatus(tab.id, 'error');
          } else if (payload.type === 'modes') {
            updateTabTerminalModes(tab.id, payload.modes);
          } else if (payload.type === 'metadata' && payload.metadata) {
            // Update tab metadata in realtime
            updateTabMetadata(tab.id, payload.metadata);

            const trimmedAssistantInput =
              typeof payload.metadata.aiAssistantRecentInput === 'string'
                ? payload.metadata.aiAssistantRecentInput.trim()
                : '';

            if (trimmedAssistantInput) {
              console.log(
                `[Terminal] AI Input Captured: ${payload.metadata.aiAssistantRecentInput}`,
                {
                  sessionId: tab.id,
                  sessionTitle: tab.title,
                  assistant: payload.metadata.aiAssistant,
                }
              );

              taskActions.invalidateTaskCache();
            }

            // 🎯 Detect AI assistant completion
            // Only trigger notification when transitioning from working state to waiting_input
            const assistant = payload.metadata.aiAssistant;
            const currentState = assistant?.state;
            const previousState = aiPreviousStates.get(tab.id);

            // 🔍 Detect AI assistant detection/closure
            const isAgentDetected = assistant?.detected === true;

            // Detect AI agent first appearance - emit ai:detected only once per session
            if (isAgentDetected && !aiDetectedSessions.has(tab.id)) {
              aiDetectedSessions.add(tab.id);
              console.log(
                `[Terminal] AI Agent Detected: ${assistant?.displayName || 'AI'} detected in session ${tab.id}`,
                {
                  sessionId: tab.id,
                  sessionTitle: tab.title,
                  assistant,
                }
              );
              emitter.emit('ai:detected', {
                sessionId: tab.id,
                sessionTitle: tab.title,
                projectId: tab.projectId,
                projectName: getProjectName(tab.projectId),
                worktreeId: tab.worktreeId,
                detectedAt: new Date(),
                assistantName: assistant?.displayName || assistant?.name,
                assistantType: assistant?.type,
              });
            }

            // When agent is closed, clear any existing notifications
            if (!isAgentDetected) {
              // If assistant info is missing or marked as not detected, treat it as closed/detached.
              // This prevents stale state from triggering false completion notifications when the agent restarts.
              if (aiDetectedSessions.has(tab.id)) {
                aiDetectedSessions.delete(tab.id);
                console.log(
                  `[Terminal] AI Agent Closed: Clearing notifications for session ${tab.id}`,
                  {
                    sessionId: tab.id,
                    sessionTitle: tab.title,
                    assistant,
                  }
                );
                emitter.emit('ai:closed', {
                  sessionId: tab.id,
                });
              }
              aiPreviousStates.delete(tab.id);
            } else {
              // Detect approval requests
              if (currentState === 'waiting_approval') {
                console.log(
                  `[Terminal] AI Approval Needed: ${assistant?.displayName || 'AI'} is waiting for approval`,
                  {
                    sessionId: tab.id,
                    sessionTitle: tab.title,
                    previousState,
                    currentState,
                    assistant,
                  }
                );
                emitter.emit('ai:approval-needed', {
                  sessionId: tab.id,
                  sessionTitle: tab.title,
                  projectId: tab.projectId,
                  projectName: getProjectName(tab.projectId),
                  assistant,
                });
              }

              // Detect AI starting to work again (after being idle/completed)
              if (currentState === 'working' && previousState && previousState !== 'working') {
                console.log(
                  `[Terminal] AI Started Working: ${assistant?.displayName || 'AI'} resumed work`,
                  {
                    sessionId: tab.id,
                    sessionTitle: tab.title,
                    previousState,
                    currentState,
                    assistant,
                  }
                );
                emitter.emit('ai:working', {
                  sessionId: tab.id,
                  sessionTitle: tab.title,
                  projectId: tab.projectId,
                  projectName: getProjectName(tab.projectId),
                  assistant,
                  latestCommand: trimmedAssistantInput,
                });
              }

              if (currentState === 'waiting_input' && previousState) {
                // Check if transitioning from working state
                const isFromWorkingState = previousState === 'working';
                const wasInterrupted = assistant?.interrupted === true;

                if (isFromWorkingState && !wasInterrupted) {
                  // Valid completion: working state → waiting input (NOT interrupted)
                  console.log(
                    `[Terminal] AI Completion Detected: ${assistant?.displayName || 'AI'} completed execution`,
                    {
                      sessionId: tab.id,
                      sessionTitle: tab.title,
                      previousState,
                      currentState,
                      assistant,
                    }
                  );
                  emitter.emit('ai:completed', {
                    sessionId: tab.id,
                    sessionTitle: tab.title,
                    projectId: tab.projectId,
                    projectName: getProjectName(tab.projectId),
                    assistant,
                  });
                } else if (isFromWorkingState && wasInterrupted) {
                  // User interrupted the execution
                  console.log(
                    `[Terminal] AI Interrupted: ${assistant?.displayName || 'AI'} was interrupted by user`,
                    {
                      sessionId: tab.id,
                      sessionTitle: tab.title,
                      previousState,
                      currentState,
                      assistant,
                    }
                  );
                  // Don't emit ai:completed for interrupted executions
                }
              }

              // Update previous state for next comparison
              if (currentState) {
                aiPreviousStates.set(tab.id, currentState);
              }
            }
          }
          // Check if there are any listeners for this session
          // If not, buffer the message for later replay when component remounts
          if (emitter.listenerCount(tab.id) > 0) {
            emitter.emit(tab.id, payload);
          } else if (
            payload.type === 'data' ||
            payload.type === 'exit' ||
            payload.type === 'error'
          ) {
            // Buffer terminal content events while the viewport is unmounted.
            let buffer = messageBuffers.get(tab.id);
            if (!buffer) {
              buffer = [];
              messageBuffers.set(tab.id, buffer);
            }
            buffer.push(payload);
            // Limit buffer size to prevent memory issues
            if (buffer.length > MESSAGE_BUFFER_MAX_SIZE) {
              buffer.shift();
            }
          }
        } catch (error) {
          console.error('[Terminal] Failed to process websocket message', error);
          // ignore malformed payloads
        }
      })();
    });

    socket.addEventListener('close', () => {
      sockets.delete(tab.id);
      if (pausedSocketIds.has(tab.id)) {
        pausedSocketIds.delete(tab.id);
        return;
      }
      if (manualCloseIds.has(tab.id)) {
        manualCloseIds.delete(tab.id);
        updateTabStatus(tab.id, 'closed');
        return;
      }
      if (sessionIndex.has(tab.id) && isProjectConnectionActive(tab.projectId)) {
        updateTabStatus(tab.id, 'connecting');
        setTimeout(() => {
          if (sessionIndex.has(tab.id) && isProjectConnectionActive(tab.projectId)) {
            connect(tab);
          }
        }, 1000);
      } else {
        const record = sessionIndex.get(tab.id);
        if (record && !isProjectConnectionActive(record.projectId)) {
          return;
        }
        updateTabStatus(tab.id, 'closed');
      }
    });

    socket.addEventListener('error', () => {
      updateTabStatus(tab.id, 'error');
    });
  }

  function reconcileSessions(projectId: string, sessions: TerminalSession[]) {
    const bucket = ensureBucket(projectId);
    const incomingIds = new Set(sessions.map(session => session.id));
    for (const tab of [...bucket]) {
      if (!incomingIds.has(tab.id)) {
        disconnectTab(tab.id, true);
      }
    }
    const orderedSessions = sortSessionsWithStoredOrder(projectId, sessions);
    for (const session of orderedSessions) {
      attachOrUpdateSession(session, { projectIdOverride: projectId });
    }
    const finalBucket = tabStore.get(projectId);
    if (!finalBucket || finalBucket.length === 0) {
      setActiveTab(projectId, undefined);
    } else {
      ensureActiveTab(projectId);
    }
    captureProjectOrder(projectId, finalBucket);
  }

  function ensureProjectSelected(projectId?: string) {
    if (!projectId) {
      throw new Error('����ѡ����Ŀ');
    }
    return projectId;
  }

  function normalizeProjectId(value?: string) {
    return typeof value === 'string' ? value.trim() : '';
  }

  function resolveSessionProjectId(session: TerminalSession, requested?: string) {
    const fromPayload = normalizeProjectId(session.projectId);
    const requestedProjectId = normalizeProjectId(requested);
    if (fromPayload && requestedProjectId && fromPayload !== requestedProjectId) {
      console.warn(
        '[Terminal Store] Server response project mismatch for terminal session',
        session.id,
        'payload:',
        fromPayload,
        'requested:',
        requestedProjectId
      );
    }
    return fromPayload || requestedProjectId;
  }

  function getTerminalCount(projectId?: string) {
    if (!projectId) {
      return 0;
    }
    const bucket = tabStore.get(projectId);
    return bucket?.length ?? 0;
  }

  async function loadTerminalCounts() {
    try {
      const response = await Apis.terminalSession.terminalCounts({ cacheFor: 0 }).send();
      const counts = response?.counts ?? {};

      // 更新缓存的终端数量
      cachedCounts.clear();
      Object.entries(counts).forEach(([projectId, count]) => {
        cachedCounts.set(projectId, count);
      });

      return counts;
    } catch (error) {
      console.error('Failed to load terminal counts', error);
      return {};
    }
  }

  async function closeAllSessions(projectId: string | undefined) {
    const resolved = ensureProjectSelected(projectId);
    const tabs = getTabs(resolved);

    // 关闭所有终端
    const closePromises = tabs.map(tab => closeSession(resolved, tab.id));
    await Promise.allSettled(closePromises);
  }

  function getSessionById(sessionId: string) {
    return sessionIndex.get(sessionId)?.tab;
  }

  function getLinkedTaskId(sessionId: string) {
    return sessionToTaskMap.get(sessionId) ?? undefined;
  }

  function focusSession(projectId: string | undefined, sessionId?: string) {
    if (!projectId || !sessionId) {
      return false;
    }
    const bucket = tabStore.get(projectId);
    if (!bucket || !bucket.some(tab => tab.id === sessionId)) {
      return false;
    }
    setActiveTab(projectId, sessionId);
    emitter.emit('terminal:ensure-expanded', { projectId });
    return true;
  }

  function retainProjectConnections(projectId: string | undefined) {
    if (!projectId) {
      return;
    }
    const nextCount = (projectConnectionRefCounts.get(projectId) ?? 0) + 1;
    projectConnectionRefCounts.set(projectId, nextCount);
    if (nextCount !== 1) {
      return;
    }
    const bucket = tabStore.get(projectId);
    if (!bucket) {
      return;
    }
    bucket.forEach(tab => {
      connect(tab);
    });
  }

  function releaseProjectConnections(projectId: string | undefined) {
    if (!projectId) {
      return;
    }
    const currentCount = projectConnectionRefCounts.get(projectId) ?? 0;
    if (currentCount <= 1) {
      projectConnectionRefCounts.delete(projectId);
      const bucket = tabStore.get(projectId);
      if (!bucket) {
        return;
      }
      bucket.forEach(tab => {
        pauseSocket(tab.id);
      });
      return;
    }
    projectConnectionRefCounts.set(projectId, currentCount - 1);
  }

  function getSessionByTask(taskId: string | undefined, projectId?: string) {
    if (!taskId) {
      return undefined;
    }
    const sessionId = taskToSessionMap.get(taskId);
    if (!sessionId) {
      return undefined;
    }
    const record = sessionIndex.get(sessionId);
    if (!record) {
      return undefined;
    }
    if (projectId && record.projectId !== projectId) {
      return undefined;
    }
    return record.tab;
  }

  return {
    emitter,
    getTabs,
    getActiveTabId,
    setActiveTab,
    prepareProject,
    loadSessions,
    createSession,
    renameSession,
    closeSession,
    closeAllSessions,
    send,
    disconnectTab,
    reorderTabs,
    getTerminalCount,
    terminalCounts: cachedCounts,
    loadTerminalCounts,
    getSessionById,
    linkSessionTask,
    unlinkSessionTask,
    setSessionRenderMode,
    setSessionSnapshotInterval,
    focusSession,
    retainProjectConnections,
    releaseProjectConnections,
    getSessionByTask,
    getLinkedTaskId,
    replayBufferedMessages,
    saveSerializedSnapshot,
    getSerializedSnapshot,
    getLatestServerSnapshot,
  };
});
