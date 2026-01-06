<template>
  <div class="conversation-viewer">
    <n-spin :show="loading">
      <div ref="containerRef" class="conversation-container">
        <template v-if="filteredMessages.length > 0">
          <div
            v-for="(msg, index) in filteredMessages"
            :key="index"
            :ref="el => setMessageRef(el, index)"
            class="message-item"
            :class="msg.role"
          >
            <div class="message-header">
              <span class="message-role">{{ msg.role === 'user' ? t('terminal.user') : t('terminal.assistant') }}</span>
              <span v-if="msg.timestamp" class="message-time">{{ formatTime(msg.timestamp) }}</span>
            </div>
            <div
              class="message-content"
              v-html="renderMarkdown(isToolResult(msg) && msg.toolUseId && isExpanded(msg.toolUseId) && msg.full ? msg.full : msg.content)"
            ></div>
            <div
              v-if="isToolResult(msg) && msg.toolUseId && (msg.hasMore || msg.full)"
              class="tool-result-controls"
            >
              <n-button
                size="tiny"
                quaternary
                :loading="!!toolResultLoading[msg.toolUseId]"
                @click.stop="toggleToolResult(msg)"
              >
                {{ isExpanded(msg.toolUseId) ? t('terminal.collapseToolResult') : t('terminal.expandToolResult') }}
              </n-button>
            </div>
            <!-- 用户消息导航按钮 -->
            <div v-if="msg.role === 'user'" class="message-nav">
              <n-tooltip>
                <template #trigger>
                  <n-button
                    size="tiny"
                    quaternary
                    :disabled="!hasPrevUserMessage(index)"
                    @click="goToPrevUserMessage(index)"
                  >
                    <template #icon>
                      <n-icon size="14"><ChevronUpOutline /></n-icon>
                    </template>
                  </n-button>
                </template>
                {{ t('terminal.prevUserMessage') }}
              </n-tooltip>
              <n-tooltip>
                <template #trigger>
                  <n-button
                    size="tiny"
                    quaternary
                    :disabled="!hasNextUserMessage(index)"
                    @click="goToNextUserMessage(index)"
                  >
                    <template #icon>
                      <n-icon size="14"><ChevronDownOutline /></n-icon>
                    </template>
                  </n-button>
                </template>
                {{ t('terminal.nextUserMessage') }}
              </n-tooltip>
            </div>
          </div>
        </template>
        <n-empty v-else-if="!loading" :description="emptyText" />
      </div>
    </n-spin>

    <!-- 底部工具栏 -->
    <div class="conversation-toolbar">
      <div v-if="sessionInfo" class="session-info">
        <n-tag v-if="sessionInfo.type" size="small" :type="sessionInfo.type === 'claude_code' ? 'info' : 'success'">
          <template #icon>
            <n-icon size="12">
              <svg v-if="sessionInfo.type === 'claude_code'" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
              </svg>
              <LogoGithub v-else />
            </n-icon>
          </template>
          {{ sessionInfo.type === 'claude_code' ? 'Claude Code' : 'Codex' }}
        </n-tag>
        <code class="session-id-code">{{ sessionInfo.sessionId }}</code>
        <n-button size="tiny" quaternary @click="copySessionId">
          <template #icon>
            <n-icon size="12"><CopyOutline /></n-icon>
          </template>
        </n-button>
      </div>
      <div class="toolbar-right">
        <n-tooltip>
          <template #trigger>
            <n-button size="tiny" quaternary :loading="refreshing" @click="emit('refresh')">
              <template #icon>
                <n-icon size="14"><RefreshOutline /></n-icon>
              </template>
            </n-button>
          </template>
          {{ t('terminal.refreshConversation') }}
        </n-tooltip>
        <n-checkbox v-model:checked="showUserOnly" size="small">
          {{ t('terminal.showUserMessagesOnly') }}
        </n-checkbox>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useMessage } from 'naive-ui';
import { CopyOutline, LogoGithub, ChevronUpOutline, ChevronDownOutline, RefreshOutline } from '@vicons/ionicons5';
import { useLocale } from '@/composables/useLocale';
import { useTimeAgo } from '@vueuse/core';
import { marked } from 'marked';

export interface ConversationMessage {
  role: 'user' | 'assistant';
  content: string;
  timestamp?: string;
  kind?: string;
  toolUseId?: string;
  hasMore?: boolean;
  full?: string;
}

export interface SessionInfo {
  sessionId: string;
  type?: 'claude_code' | 'codex' | string;
}

const props = withDefaults(defineProps<{
  messages: ConversationMessage[];
  loading?: boolean;
  refreshing?: boolean;
  sessionInfo?: SessionInfo | null;
  emptyText?: string;
  useRelativeTime?: boolean;
}>(), {
  loading: false,
  refreshing: false,
  sessionInfo: null,
  emptyText: '',
  useRelativeTime: true,
});

const emit = defineEmits<{
  (e: 'load-tool-result', toolUseId: string): void;
  (e: 'refresh'): void;
}>();

const { t } = useLocale();
const message = useMessage();

const showUserOnly = ref(false);
const containerRef = ref<HTMLElement | null>(null);
const messageRefs = ref<Map<number, HTMLElement>>(new Map());
const expandedToolResults = ref<Record<string, boolean>>({});
const toolResultLoading = ref<Record<string, boolean>>({});

// 设置消息元素的 ref
function setMessageRef(el: unknown, index: number) {
  if (el) {
    messageRefs.value.set(index, el as HTMLElement);
  } else {
    messageRefs.value.delete(index);
  }
}

// 清理 refs 当消息列表变化时
watch(() => props.messages, () => {
  messageRefs.value.clear();
});

// 过滤消息
const filteredMessages = computed(() => {
  if (!showUserOnly.value) {
    return props.messages;
  }
  return props.messages.filter(msg => msg.role === 'user');
});

// 空文本
const emptyText = computed(() => {
  return props.emptyText || t('terminal.noMessages');
});

// 获取过滤后列表中所有用户消息的索引
function getUserMessageIndices(): number[] {
  const indices: number[] = [];
  filteredMessages.value.forEach((msg, index) => {
    if (msg.role === 'user') {
      indices.push(index);
    }
  });
  return indices;
}

// 检查是否有上一条用户消息
function hasPrevUserMessage(currentIndex: number): boolean {
  const indices = getUserMessageIndices();
  const currentPos = indices.indexOf(currentIndex);
  return currentPos > 0;
}

// 检查是否有下一条用户消息
function hasNextUserMessage(currentIndex: number): boolean {
  const indices = getUserMessageIndices();
  const currentPos = indices.indexOf(currentIndex);
  return currentPos >= 0 && currentPos < indices.length - 1;
}

// 跳转到上一条用户消息
function goToPrevUserMessage(currentIndex: number) {
  const indices = getUserMessageIndices();
  const currentPos = indices.indexOf(currentIndex);
  if (currentPos > 0) {
    const prevIndex = indices[currentPos - 1];
    scrollToMessage(prevIndex);
  }
}

// 跳转到下一条用户消息
function goToNextUserMessage(currentIndex: number) {
  const indices = getUserMessageIndices();
  const currentPos = indices.indexOf(currentIndex);
  if (currentPos >= 0 && currentPos < indices.length - 1) {
    const nextIndex = indices[currentPos + 1];
    scrollToMessage(nextIndex);
  }
}

// 滚动到指定消息
function scrollToMessage(index: number) {
  const el = messageRefs.value.get(index);
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }
}

// 格式化时间
function formatTime(timestamp: string) {
  if (props.useRelativeTime) {
    return useTimeAgo(new Date(timestamp)).value;
  }
  return new Date(timestamp).toLocaleString();
}

// 配置 marked
marked.setOptions({
  breaks: true,
  gfm: true,
});

// 渲染 Markdown
function renderMarkdown(content: string): string {
  try {
    return marked.parse(content) as string;
  } catch {
    // 如果解析失败，返回转义后的原始内容
    return content
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/\n/g, '<br>');
  }
}

function isToolResult(msg: ConversationMessage) {
  return msg.kind === 'tool_result' && !!msg.toolUseId;
}

function isExpanded(toolUseId: string) {
  return !!expandedToolResults.value[toolUseId];
}

function toggleToolResult(msg: ConversationMessage) {
  const toolUseId = msg.toolUseId;
  if (!toolUseId) return;

  const next = !isExpanded(toolUseId);
  expandedToolResults.value = { ...expandedToolResults.value, [toolUseId]: next };
  if (!next) return;

  if (msg.full) return;
  if (toolResultLoading.value[toolUseId]) return;

  toolResultLoading.value = { ...toolResultLoading.value, [toolUseId]: true };
  emit('load-tool-result', toolUseId);
}

function markToolResultLoaded(toolUseId: string) {
  if (!toolUseId) return;
  if (!toolResultLoading.value[toolUseId]) return;
  const next = { ...toolResultLoading.value };
  delete next[toolUseId];
  toolResultLoading.value = next;
}

watch(
  () => props.messages,
  () => {
    // Cleanup loading state when parent updates messages (e.g., full tool result arrives).
    for (const msg of props.messages) {
      if (isToolResult(msg) && msg.toolUseId && msg.full) {
        markToolResultLoaded(msg.toolUseId);
      }
    }
  },
  { deep: true }
);

// 复制 Session ID
async function copySessionId() {
  if (!props.sessionInfo?.sessionId) return;
  try {
    await navigator.clipboard.writeText(props.sessionInfo.sessionId);
    message.success(t('terminal.aiSessionIdCopied'));
  } catch {
    message.error(t('terminal.copyFailed'));
  }
}
</script>

<style scoped>
.conversation-viewer {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.conversation-container {
  flex: 1;
  max-height: 60vh;
  overflow-y: auto;
  padding: 8px 0;
}

.tool-result-controls {
  margin-top: 8px;
}

.message-item {
  position: relative;
  margin-bottom: 16px;
  padding: 12px 16px;
  border-radius: 8px;
  background: var(--n-color-embedded);
}

.message-item.user {
  background: var(--n-color-embedded);
  border-left: 3px solid var(--n-primary-color);
}

.message-item.assistant {
  background: var(--n-color-embedded);
  border-left: 3px solid var(--n-success-color);
}

.message-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.message-role {
  font-weight: 600;
  font-size: 13px;
}

.message-item.user .message-role {
  color: var(--n-primary-color);
}

.message-item.assistant .message-role {
  color: var(--n-success-color);
}

.message-time {
  font-size: 12px;
  color: var(--n-text-color-3);
}

.message-content {
  font-size: 14px;
  line-height: 1.6;
  word-break: break-word;
}

/* 用户消息导航按钮 */
.message-nav {
  position: absolute;
  right: 8px;
  bottom: 8px;
  display: flex;
  gap: 2px;
  opacity: 0.5;
  transition: opacity 0.2s;
}

.message-item:hover .message-nav {
  opacity: 1;
}

/* Markdown 样式 */
.message-content :deep(p) {
  margin: 0 0 8px 0;
}

.message-content :deep(p:last-child) {
  margin-bottom: 0;
}

.message-content :deep(pre) {
  background: var(--n-code-color);
  padding: 12px;
  border-radius: 6px;
  overflow-x: auto;
  margin: 8px 0;
}

.message-content :deep(code) {
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
}

.message-content :deep(:not(pre) > code) {
  background: var(--n-code-color);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.9em;
}

.message-content :deep(ul),
.message-content :deep(ol) {
  padding-left: 20px;
  margin: 8px 0;
}

.message-content :deep(li) {
  margin: 4px 0;
}

.message-content :deep(blockquote) {
  border-left: 3px solid var(--n-border-color);
  padding-left: 12px;
  margin: 8px 0;
  color: var(--n-text-color-3);
}

.message-content :deep(h1),
.message-content :deep(h2),
.message-content :deep(h3),
.message-content :deep(h4),
.message-content :deep(h5),
.message-content :deep(h6) {
  margin: 16px 0 8px 0;
  font-weight: 600;
}

.message-content :deep(h1) { font-size: 1.5em; }
.message-content :deep(h2) { font-size: 1.3em; }
.message-content :deep(h3) { font-size: 1.1em; }

.message-content :deep(a) {
  color: var(--n-primary-color);
  text-decoration: none;
}

.message-content :deep(a:hover) {
  text-decoration: underline;
}

.message-content :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 8px 0;
}

.message-content :deep(th),
.message-content :deep(td) {
  border: 1px solid var(--n-border-color);
  padding: 8px 12px;
  text-align: left;
}

.message-content :deep(th) {
  background: var(--n-color-embedded);
  font-weight: 600;
}

.message-content :deep(hr) {
  border: none;
  border-top: 1px solid var(--n-border-color);
  margin: 16px 0;
}

/* 底部工具栏 */
.conversation-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 12px;
  border-top: 1px solid var(--n-border-color);
  margin-top: 8px;
}

.session-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.session-id-code {
  font-size: 12px;
  font-family: monospace;
  background: var(--n-color-embedded);
  padding: 2px 8px;
  border-radius: 4px;
  color: var(--n-text-color-2);
  user-select: all;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
