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
      <n-space :size="4" align="center">
        <n-button
          quaternary
          circle
          size="small"
          :title="t('terminal.specifyDirectory')"
          @click="showDirectoryPicker = true"
        >
          <template #icon>
            <n-icon><FolderOpenOutline /></n-icon>
          </template>
        </n-button>
        <n-button quaternary circle size="small" :loading="loading" @click="loadSessions(false)">
          <template #icon>
            <n-icon><RefreshOutline /></n-icon>
          </template>
        </n-button>
      </n-space>
    </template>
    <n-spin :show="loading">
      <div class="session-tabs">
        <!-- Current directory indicator -->
        <div v-if="currentViewPath" class="current-path-bar">
          <n-icon :size="14"><FolderOutline /></n-icon>
          <span class="current-path-text" :title="currentViewPath">{{
            truncatePath(currentViewPath, 60)
          }}</span>
          <n-button v-if="isCustomPath" quaternary size="tiny" @click="resetToProjectPath">
            <template #icon>
              <n-icon :size="12"><CloseOutline /></n-icon>
            </template>
          </n-button>
        </div>
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
                      <n-tag
                        size="small"
                        type="info"
                        class="message-count-tag"
                        @click.stop="viewConversation(session)"
                      >
                        {{
                          t('terminal.messageCount', {
                            count: session.messageCount,
                            replyCount: session.assistantMessageCount,
                          })
                        }}
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
                      <n-tag
                        size="small"
                        type="success"
                        class="message-count-tag"
                        @click.stop="viewConversation(session)"
                      >
                        {{
                          t('terminal.messageCount', {
                            count: session.messageCount,
                            replyCount: session.assistantMessageCount,
                          })
                        }}
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
    <ConversationViewer
      :messages="currentConversation?.messages ?? []"
      :loading="conversationLoading"
      :refreshing="conversationRefreshing"
      :session-info="currentSessionInfo"
      @load-tool-result="loadToolResult"
      @refresh="refreshConversation"
    />
  </n-modal>

  <!-- Directory Picker Dialog -->
  <DirectoryPickerDialog
    v-model:show="showDirectoryPicker"
    :initial-path="currentProjectPath"
    @confirm="handleDirectorySelected"
  />
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useMessage } from 'naive-ui';
import {
  ChevronForwardOutline,
  ChatboxOutline,
  PlayOutline,
  CopyOutline,
  RefreshOutline,
  SearchOutline,
  FolderOpenOutline,
  FolderOutline,
  CloseOutline,
} from '@vicons/ionicons5';
import { useLocale } from '@/composables/useLocale';
import { http } from '@/api/http';
import { useTimeAgo } from '@vueuse/core';
import ConversationViewer, { type SessionInfo } from '@/components/common/ConversationViewer.vue';
import DirectoryPickerDialog from '@/components/common/DirectoryPickerDialog.vue';
import { useProjectStore } from '@/stores/project';

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
  assistantMessageCount: number;
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
const projectStore = useProjectStore();

// 通过 projectId 获取项目路径
const currentProjectPath = computed(() => {
  const project = projectStore.projects.find(p => p.id === props.projectId);
  return project?.path || '';
});

const loading = ref(false);
const activeType = ref<'claude_code' | 'codex'>('claude_code');
const expandedSessionId = ref<string | null>(null);
const claudeSessions = ref<AISessionSummary[]>([]);
const codexSessions = ref<AISessionSummary[]>([]);
const claudeScanPhase = ref<ScanPhase>('complete');
const codexScanPhase = ref<ScanPhase>('complete');
const searchQuery = ref('');
const customPath = ref('');
const showDirectoryPicker = ref(false);
const currentViewPath = ref('');
const isCustomPath = ref(false);
let refreshTimer: ReturnType<typeof setTimeout> | null = null;

// Filtered sessions based on search query
const filteredClaudeSessions = computed(() => {
  if (!searchQuery.value.trim()) return claudeSessions.value;
  const query = searchQuery.value.toLowerCase();
  return claudeSessions.value.filter(
    s =>
      (s.title && s.title.toLowerCase().includes(query)) ||
      s.sessionId.toLowerCase().includes(query) ||
      s.model?.toLowerCase().includes(query)
  );
});

const filteredCodexSessions = computed(() => {
  if (!searchQuery.value.trim()) return codexSessions.value;
  const query = searchQuery.value.toLowerCase();
  return codexSessions.value.filter(
    s =>
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
  kind?: string;
  toolUseId?: string;
  hasMore?: boolean;
  full?: string;
}

interface ConversationResponse {
  sessionId: string;
  title: string;
  messages: ConversationMessage[];
}

interface ToolResultResponse {
  toolUseId: string;
  content: string;
}

const showConversationModal = ref(false);
const conversationLoading = ref(false);
const conversationRefreshing = ref(false);
const currentConversation = ref<ConversationResponse | null>(null);
const currentSessionTitle = ref('');
const currentSession = ref<AISessionSummary | null>(null);

const currentSessionInfo = computed<SessionInfo | null>(() => {
  if (!currentSession.value) return null;
  return {
    sessionId: currentSession.value.sessionId,
    type: currentSession.value.type as 'claude_code' | 'codex',
  };
});

const isScanning = computed(
  () => claudeScanPhase.value !== 'complete' || codexScanPhase.value !== 'complete'
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

watch(showModal, async show => {
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
      if (
        showModal.value &&
        (claudeScanPhase.value !== 'complete' || codexScanPhase.value !== 'complete')
      ) {
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

function handleDirectorySelected(path: string) {
  customPath.value = path;
  loadSessionsByPath(path);
}

async function loadSessionsByPath(path?: string) {
  const targetPath = path || customPath.value.trim();
  console.log('[AISessionHistoryDialog] loadSessionsByPath:', targetPath);
  if (!targetPath) {
    message.warning(t('terminal.pleaseEnterPath'));
    return;
  }

  loading.value = true;
  expandedSessionId.value = null;

  try {
    const response = await http
      .Post<ItemResponse<ProjectAISessions>>('/ai-sessions/by-path', {
        path: targetPath,
      })
      .send();

    console.log('[AISessionHistoryDialog] response:', response);
    const data = response?.item;
    console.log('[AISessionHistoryDialog] data:', data);
    console.log('[AISessionHistoryDialog] claudeSessions:', data?.claudeSessions);
    console.log('[AISessionHistoryDialog] codexSessions:', data?.codexSessions);
    if (data) {
      claudeSessions.value = data.claudeSessions || [];
      codexSessions.value = data.codexSessions || [];
      claudeScanPhase.value = data.claudeScanPhase || 'complete';
      codexScanPhase.value = data.codexScanPhase || 'complete';
      currentViewPath.value = targetPath;
      isCustomPath.value = true;

      // Auto-select tab with sessions
      if (data.hasCodex && !data.hasClaudeCode) {
        activeType.value = 'codex';
      } else {
        activeType.value = 'claude_code';
      }

      // Schedule refresh if still scanning
      if (
        showModal.value &&
        (claudeScanPhase.value !== 'complete' || codexScanPhase.value !== 'complete')
      ) {
        if (refreshTimer) {
          clearTimeout(refreshTimer);
        }
        refreshTimer = setTimeout(() => loadSessionsByPath(targetPath), 2000);
      }
    }
  } catch (error) {
    console.error('Failed to load AI sessions by path:', error);
    message.error(t('common.loadFailed'));
  } finally {
    loading.value = false;
  }
}

function resetToProjectPath() {
  customPath.value = '';
  isCustomPath.value = false;
  currentViewPath.value = '';
  loadSessions(false);
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

async function loadToolResult(toolUseId: string) {
  const session = currentSession.value;
  if (!session || !toolUseId) return;

  try {
    const response = await http
      .Get<{
        item?: ToolResultResponse;
      }>(`/ai-sessions/${session.id}/conversation/tool-results/${encodeURIComponent(toolUseId)}`, { cacheFor: 0 })
      .send();

    const content = response?.item?.content;
    if (!content || !currentConversation.value) return;

    const msg = currentConversation.value.messages.find(m => m.toolUseId === toolUseId);
    if (msg) {
      msg.full = content;
    }
  } catch (error) {
    console.error('Failed to load tool result:', error);
    message.error(t('terminal.loadConversationFailed'));
  }
}

async function refreshConversation() {
  const session = currentSession.value;
  if (!session) return;

  conversationRefreshing.value = true;

  try {
    // Call API to clear cache and reload
    const response = await http
      .Post<{ item?: ConversationResponse }>(`/ai-sessions/${session.id}/refresh`)
      .send();

    if (response?.item) {
      currentConversation.value = response.item;
      message.success(t('terminal.conversationRefreshed'));
    }
  } catch (error) {
    console.error('Failed to refresh conversation:', error);
    message.error(t('terminal.refreshConversationFailed'));
  } finally {
    conversationRefreshing.value = false;
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

.current-path-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  margin-bottom: 10px;
  background: var(--n-color-embedded);
  border-radius: 6px;
  font-size: 12px;
  color: var(--n-text-color-2);
}

.current-path-text {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: monospace;
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

.message-count-tag {
  cursor: pointer;
  transition: opacity 0.2s;
}

.message-count-tag:hover {
  opacity: 0.8;
}
</style>
