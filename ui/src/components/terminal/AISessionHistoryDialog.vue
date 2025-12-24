<template>
  <n-modal
    v-model:show="showModal"
    preset="card"
    :title="t('terminal.aiSessionHistory')"
    style="width: 680px; max-width: 90vw; max-height: 80vh"
    :mask-closable="true"
    :closable="true"
    @close="handleClose"
  >
    <template #header-extra>
      <n-button
        quaternary
        circle
        size="small"
        :loading="loading"
        @click="loadSessions(false)"
      >
        <template #icon>
          <n-icon><RefreshOutline /></n-icon>
        </template>
      </n-button>
    </template>
    <n-spin :show="loading">
      <div class="session-tabs">
        <div class="search-bar">
          <n-input
            v-model:value="searchQuery"
            :placeholder="t('terminal.searchSessions')"
            clearable
            size="small"
          >
            <template #prefix>
              <n-icon><SearchOutline /></n-icon>
            </template>
          </n-input>
        </div>
        <n-tabs v-model:value="activeType" type="line" animated>
          <n-tab-pane name="claude_code" :tab="claudeCodeTabLabel">
            <div class="session-list">
              <template v-if="filteredClaudeSessions.length > 0">
                <div
                  v-for="session in filteredClaudeSessions"
                  :key="session.id"
                  class="session-item"
                  :class="{ expanded: expandedSessionId === session.id }"
                  @click="toggleSession(session)"
                >
                  <div class="session-header">
                    <div class="session-info">
                      <span class="session-title" :title="session.title || undefined">
                        {{ getDisplayTitle(session) }}
                      </span>
                      <div class="session-meta-row">
                        <span class="session-model">{{ session.model || 'Claude' }}</span>
                        <span class="session-time">
                          {{ getTimeAgo(session.lastMessageAt || session.sessionStartedAt) }}
                        </span>
                      </div>
                    </div>
                    <div class="session-meta">
                      <n-tag size="small" type="info">
                        {{ t('terminal.messageCount', { count: session.messageCount }) }}
                      </n-tag>
                      <n-icon
                        :size="16"
                        class="expand-icon"
                        :class="{ rotated: expandedSessionId === session.id }"
                      >
                        <ChevronForwardOutline />
                      </n-icon>
                    </div>
                  </div>
                  <div v-if="expandedSessionId === session.id" class="session-detail">
                    <div class="detail-row">
                      <span class="detail-label">{{ t('terminal.logFile') }}:</span>
                      <code class="detail-value file-path" :title="session.filePath">
                        {{ truncatePath(session.filePath) }}
                      </code>
                    </div>
                    <div class="detail-row resume-command-row">
                      <code class="resume-command" :title="getResumeCommand(session)">
                        {{ getResumeCommand(session) }}
                      </code>
                      <n-button
                        size="tiny"
                        quaternary
                        class="copy-command-btn"
                        @click.stop="copyResumeCommand(session)"
                      >
                        <template #icon>
                          <n-icon size="14"><CopyOutline /></n-icon>
                        </template>
                      </n-button>
                    </div>
                    <div class="detail-actions">
                      <n-button size="small" @click.stop="copyPath(session.filePath)">
                        {{ t('terminal.copyPath') }}
                      </n-button>
                      <n-button size="small" type="info" @click.stop="viewConversation(session)">
                        <template #icon>
                          <n-icon><ChatboxOutline /></n-icon>
                        </template>
                        {{ t('terminal.viewConversation') }}
                      </n-button>
                      <n-button size="small" type="primary" @click.stop="resumeSession(session)">
                        <template #icon>
                          <n-icon><PlayOutline /></n-icon>
                        </template>
                        {{ t('terminal.resumeSession') }}
                      </n-button>
                    </div>
                  </div>
                </div>
              </template>
              <n-empty v-else :description="t('terminal.noClaudeSessions')" />
            </div>
          </n-tab-pane>
          <n-tab-pane name="codex" :tab="codexTabLabel">
            <div class="session-list">
              <template v-if="filteredCodexSessions.length > 0">
                <div
                  v-for="session in filteredCodexSessions"
                  :key="session.id"
                  class="session-item"
                  :class="{ expanded: expandedSessionId === session.id }"
                  @click="toggleSession(session)"
                >
                  <div class="session-header">
                    <div class="session-info">
                      <span class="session-title" :title="session.title || undefined">
                        {{ getDisplayTitle(session) }}
                      </span>
                      <div class="session-meta-row">
                        <span class="session-model">{{ session.model || 'Codex' }}</span>
                        <span class="session-time">
                          {{ getTimeAgo(session.lastMessageAt || session.sessionStartedAt) }}
                        </span>
                      </div>
                    </div>
                    <div class="session-meta">
                      <n-tag size="small" type="success">
                        {{ t('terminal.messageCount', { count: session.messageCount }) }}
                      </n-tag>
                      <n-icon
                        :size="16"
                        class="expand-icon"
                        :class="{ rotated: expandedSessionId === session.id }"
                      >
                        <ChevronForwardOutline />
                      </n-icon>
                    </div>
                  </div>
                  <div v-if="expandedSessionId === session.id" class="session-detail">
                    <div class="detail-row">
                      <span class="detail-label">{{ t('terminal.logFile') }}:</span>
                      <code class="detail-value file-path" :title="session.filePath">
                        {{ truncatePath(session.filePath) }}
                      </code>
                    </div>
                    <div class="detail-actions">
                      <n-button size="small" @click.stop="copyPath(session.filePath)">
                        {{ t('terminal.copyPath') }}
                      </n-button>
                      <n-button size="small" type="info" @click.stop="viewConversation(session)">
                        <template #icon>
                          <n-icon><ChatboxOutline /></n-icon>
                        </template>
                        {{ t('terminal.viewConversation') }}
                      </n-button>
                    </div>
                  </div>
                </div>
              </template>
              <n-empty v-else :description="t('terminal.noCodexSessions')" />
            </div>
          </n-tab-pane>
        </n-tabs>
      </div>
    </n-spin>
  </n-modal>

  <!-- Conversation Viewer Modal -->
  <n-modal
    v-model:show="showConversationModal"
    preset="card"
    :title="currentSessionTitle"
    style="width: 800px; max-width: 90vw; max-height: 85vh"
    :mask-closable="true"
    :closable="true"
    @close="closeConversationModal"
  >
    <n-spin :show="conversationLoading">
      <div class="conversation-container">
        <template v-if="currentConversation && currentConversation.messages.length > 0">
          <div
            v-for="(msg, index) in currentConversation.messages"
            :key="index"
            class="message-item"
            :class="msg.role"
          >
            <div class="message-header">
              <span class="message-role">{{ msg.role === 'user' ? t('terminal.user') : t('terminal.assistant') }}</span>
              <span v-if="msg.timestamp" class="message-time">{{ getTimeAgo(msg.timestamp) }}</span>
            </div>
            <div class="message-content">{{ msg.content }}</div>
          </div>
        </template>
        <n-empty v-else-if="!conversationLoading" :description="t('terminal.noMessages')" />
      </div>
    </n-spin>
    <template #footer>
      <div v-if="currentSession" class="conversation-footer">
        <div class="conversation-footer__info">
          <n-tag size="small" :type="currentSession.type === 'claude_code' ? 'info' : 'success'">
            <template #icon>
              <n-icon size="12">
                <svg v-if="currentSession.type === 'claude_code'" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
                </svg>
                <LogoGithub v-else />
              </n-icon>
            </template>
            {{ currentSession.type === 'claude_code' ? 'Claude Code' : 'Codex' }}
          </n-tag>
          <code class="session-id-code">{{ currentSession.sessionId }}</code>
          <n-button size="tiny" quaternary @click="copySessionId(currentSession)">
            <template #icon>
              <n-icon size="12"><CopyOutline /></n-icon>
            </template>
          </n-button>
        </div>
      </div>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useMessage } from 'naive-ui';
import { ChevronForwardOutline, ChatboxOutline, PlayOutline, CopyOutline, RefreshOutline, SearchOutline, LogoGithub } from '@vicons/ionicons5';
import { useLocale } from '@/composables/useLocale';
import { http } from '@/api/http';
import { useTimeAgo } from '@vueuse/core';

type ScanPhase = 'recent' | 'extended' | 'complete';

interface AISessionSummary {
  id: string;
  sessionId: string;
  type: string;
  model: string;
  title: string | null;
  sessionStartedAt: string;
  lastMessageAt: string | null;
  messageCount: number;
  filePath: string;
}

interface ProjectAISessions {
  hasClaudeCode: boolean;
  hasCodex: boolean;
  claudeSessions: AISessionSummary[];
  codexSessions: AISessionSummary[];
  claudeScanPhase?: ScanPhase;
  codexScanPhase?: ScanPhase;
}

interface ItemResponse<T> {
  item?: T;
}

const props = defineProps<{
  projectId: string;
}>();

const showModal = defineModel<boolean>('show', { default: false });

const emit = defineEmits<{
  (e: 'resume', sessionId: string, sessionType: string): void;
}>();

const { t } = useLocale();
const message = useMessage();

const loading = ref(false);
const activeType = ref<'claude_code' | 'codex'>('claude_code');
const expandedSessionId = ref<string | null>(null);
const claudeSessions = ref<AISessionSummary[]>([]);
const codexSessions = ref<AISessionSummary[]>([]);
const claudeScanPhase = ref<ScanPhase>('complete');
const codexScanPhase = ref<ScanPhase>('complete');
const searchQuery = ref('');
let refreshTimer: ReturnType<typeof setTimeout> | null = null;

// Filtered sessions based on search query
const filteredClaudeSessions = computed(() => {
  if (!searchQuery.value.trim()) return claudeSessions.value;
  const query = searchQuery.value.toLowerCase();
  return claudeSessions.value.filter(
    (s) =>
      (s.title && s.title.toLowerCase().includes(query)) ||
      s.sessionId.toLowerCase().includes(query) ||
      s.model?.toLowerCase().includes(query)
  );
});

const filteredCodexSessions = computed(() => {
  if (!searchQuery.value.trim()) return codexSessions.value;
  const query = searchQuery.value.toLowerCase();
  return codexSessions.value.filter(
    (s) =>
      (s.title && s.title.toLowerCase().includes(query)) ||
      s.sessionId.toLowerCase().includes(query) ||
      s.model?.toLowerCase().includes(query)
  );
});

// Conversation viewer state
interface ConversationMessage {
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
}

interface ConversationResponse {
  sessionId: string;
  title: string;
  messages: ConversationMessage[];
}

const showConversationModal = ref(false);
const conversationLoading = ref(false);
const currentConversation = ref<ConversationResponse | null>(null);
const currentSessionTitle = ref('');
const currentSession = ref<AISessionSummary | null>(null);

const isScanning = computed(() =>
  claudeScanPhase.value !== 'complete' || codexScanPhase.value !== 'complete'
);

const claudeCodeTabLabel = computed(() => {
  const count = claudeSessions.value.length;
  const scanIndicator = claudeScanPhase.value !== 'complete' ? ' ...' : '';
  return `Claude Code${count > 0 ? ` (${count}${scanIndicator})` : scanIndicator}`;
});

const codexTabLabel = computed(() => {
  const count = codexSessions.value.length;
  const scanIndicator = codexScanPhase.value !== 'complete' ? ' ...' : '';
  return `Codex${count > 0 ? ` (${count}${scanIndicator})` : scanIndicator}`;
});

watch(showModal, async (show) => {
  if (show && props.projectId) {
    await loadSessions();
  } else {
    // Clear refresh timer when modal closes
    if (refreshTimer) {
      clearTimeout(refreshTimer);
      refreshTimer = null;
    }
  }
});

async function loadSessions(isRefresh = false) {
  if (!props.projectId) return;

  // Only show loading spinner on first load, not on background refreshes
  if (!isRefresh) {
    loading.value = true;
    expandedSessionId.value = null;
  }

  try {
    const response = await http
      .Get<ItemResponse<ProjectAISessions>>(`/projects/${props.projectId}/ai-sessions`, {
        cacheFor: 0, // No cache - we want fresh data for scan progress
      })
      .send();

    const data = response?.item;
    if (data) {
      claudeSessions.value = data.claudeSessions || [];
      codexSessions.value = data.codexSessions || [];
      claudeScanPhase.value = data.claudeScanPhase || 'complete';
      codexScanPhase.value = data.codexScanPhase || 'complete';

      // Auto-select tab with sessions (only on first load)
      if (!isRefresh) {
        if (data.hasCodex && !data.hasClaudeCode) {
          activeType.value = 'codex';
        } else {
          activeType.value = 'claude_code';
        }
      }

      // Schedule refresh if still scanning
      if (showModal.value && (claudeScanPhase.value !== 'complete' || codexScanPhase.value !== 'complete')) {
        if (refreshTimer) {
          clearTimeout(refreshTimer);
        }
        // Poll every 2 seconds while scanning
        refreshTimer = setTimeout(() => loadSessions(true), 2000);
      }
    }
  } catch (error) {
    console.error('Failed to load AI sessions:', error);
    if (!isRefresh) {
      message.error(t('common.loadFailed'));
    }
  } finally {
    if (!isRefresh) {
      loading.value = false;
    }
  }
}

function toggleSession(session: AISessionSummary) {
  if (expandedSessionId.value === session.id) {
    expandedSessionId.value = null;
  } else {
    expandedSessionId.value = session.id;
  }
}

function formatTime(dateStr: string | null) {
  if (!dateStr) return '-';
  const date = new Date(dateStr);
  return date.toLocaleString();
}

function getTimeAgo(dateStr: string | null) {
  if (!dateStr) return '';
  return useTimeAgo(new Date(dateStr)).value;
}

function getDisplayTitle(session: AISessionSummary) {
  // Use title if available, otherwise show "Untitled Session"
  return session.title || t('terminal.untitledSession');
}

function truncatePath(path: string, maxLen = 50) {
  if (path.length <= maxLen) return path;
  const filename = path.split(/[\\/]/).pop() || '';
  if (filename.length >= maxLen - 3) {
    return '...' + filename.slice(-maxLen + 3);
  }
  return '...' + path.slice(-maxLen + 3);
}

async function copyPath(path: string) {
  try {
    await navigator.clipboard.writeText(path);
    message.success(t('terminal.pathCopied'));
  } catch {
    message.error(t('terminal.copyFailed'));
  }
}

function getResumeCommand(session: AISessionSummary) {
  return `claude --resume ${session.sessionId}`;
}

async function copyResumeCommand(session: AISessionSummary) {
  try {
    await navigator.clipboard.writeText(getResumeCommand(session));
    message.success(t('terminal.commandCopied'));
  } catch {
    message.error(t('terminal.copyFailed'));
  }
}

async function resumeSession(session: AISessionSummary) {
  // Claude Code supports resume, Codex does not
  if (session.type !== 'claude_code') {
    message.warning(t('terminal.resumeNotSupported'));
    return;
  }

  // Copy resume command to clipboard
  try {
    await navigator.clipboard.writeText(getResumeCommand(session));
    message.success(t('terminal.resumeCommandCopied'));
  } catch {
    message.error(t('terminal.copyFailed'));
  }

  // Create new terminal
  emit('resume', session.sessionId, session.type);
  showModal.value = false;
}

function handleClose() {
  showModal.value = false;
}

async function viewConversation(session: AISessionSummary) {
  currentSessionTitle.value = session.title || t('terminal.untitledSession');
  currentSession.value = session;
  showConversationModal.value = true;
  conversationLoading.value = true;
  currentConversation.value = null;

  try {
    const response = await http
      .Get<{ item?: ConversationResponse }>(`/ai-sessions/${session.id}/conversation`, {
        cacheFor: 0,
      })
      .send();

    if (response?.item) {
      currentConversation.value = response.item;
    }
  } catch (error) {
    console.error('Failed to load conversation:', error);
    message.error(t('terminal.loadConversationFailed'));
  } finally {
    conversationLoading.value = false;
  }
}

async function copySessionId(session: AISessionSummary) {
  try {
    await navigator.clipboard.writeText(session.sessionId);
    message.success(t('task.sessionIdCopied'));
  } catch {
    message.error(t('terminal.copyFailed'));
  }
}

function closeConversationModal() {
  showConversationModal.value = false;
  currentConversation.value = null;
  currentSession.value = null;
}
</script>

<style scoped>
.session-tabs {
  min-height: 300px;
}

.search-bar {
  margin-bottom: 12px;
}

.session-list {
  max-height: 400px;
  overflow-y: auto;
  padding: 8px 0;
}

.session-item {
  padding: 12px 16px;
  margin-bottom: 8px;
  background: var(--n-color-embedded);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.session-item:hover {
  border-color: var(--n-border-color);
}

.session-item.expanded {
  border-color: var(--n-primary-color);
  background: var(--n-color-embedded);
  box-shadow: inset 0 0 0 1px var(--n-primary-color);
}

.session-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.session-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  min-width: 0;
}

.session-title {
  font-weight: 500;
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
}

.session-meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.session-model {
  font-weight: 400;
}

.session-time {
  &::before {
    content: '·';
    margin-right: 8px;
  }
}

.session-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.expand-icon {
  transition: transform 0.2s ease;
}

.expand-icon.rotated {
  transform: rotate(90deg);
}

.session-detail {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed var(--n-border-color);
}

.detail-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 13px;
}

.detail-label {
  color: var(--n-text-color-3);
  flex-shrink: 0;
}

.detail-value {
  word-break: break-all;
}

.detail-value.file-path {
  font-size: 12px;
  background: var(--n-color-embedded);
  padding: 2px 6px;
  border-radius: 4px;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}

.resume-command-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

.resume-command {
  font-size: 12px;
  font-family: monospace;
  background: var(--n-color-embedded);
  padding: 4px 8px;
  border-radius: 4px;
  color: var(--n-primary-color);
  user-select: all;
}

.copy-command-btn {
  flex-shrink: 0;
  opacity: 0.6;
  transition: opacity 0.2s;
}

.copy-command-btn:hover {
  opacity: 1;
}

/* Conversation viewer styles */
.conversation-container {
  max-height: 60vh;
  overflow-y: auto;
  padding: 8px 0;
}

.message-item {
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
  white-space: pre-wrap;
  word-break: break-word;
}

/* 对话模态框底部 */
.conversation-footer {
  padding-top: 8px;
}

.conversation-footer__info {
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
</style>
