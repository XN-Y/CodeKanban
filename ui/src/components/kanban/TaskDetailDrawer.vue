<template>
  <n-drawer
    width="520"
    placement="right"
    :show="show"
    :on-after-enter="handleAfterEnter"
    @update:show="emit('update:show', $event as boolean)"
    @after-leave="emit('closed')"
  >
    <n-drawer-content :title="t('task.taskDetail')">
      <n-spin :show="detailLoading">
        <n-empty v-if="!task" :description="t('task.pleaseSelectTask')" />
        <div v-else class="task-detail">
          <n-form label-placement="top" :model="form">
            <n-form-item :label="t('task.fieldTitle')">
              <n-input v-model:value="form.title" />
            </n-form-item>

            <n-form-item :label="t('task.fieldDescription')">
              <n-input
                v-model:value="form.description"
                type="textarea"
                rows="5"
                :placeholder="t('task.useMarkdown')"
              />
            </n-form-item>

            <n-form-item :label="t('task.fieldPriority')">
              <n-select v-model:value="form.priority" :options="priorityOptions" />
            </n-form-item>

            <n-form-item :label="t('task.relatedBranch')">
              <n-select
                v-model:value="form.worktreeId"
                :options="worktreeOptions"
                :placeholder="t('task.optional')"
                clearable
              />
            </n-form-item>

            <n-form-item :label="t('task.dueDate')">
              <n-date-picker
                v-model:formatted-value="form.dueDate"
                type="date"
                value-format="yyyy-MM-dd"
                clearable
              />
            </n-form-item>

            <n-form-item :label="t('task.tags')">
              <n-dynamic-tags v-model:value="form.tags" />
            </n-form-item>
          </n-form>

          <n-divider />

          <!-- 关联的 AI Session -->
          <section>
            <div class="task-detail__section-header">
              <h3>{{ t('task.linkedAiSessions') }}</h3>
              <n-button size="tiny" quaternary type="primary" @click="openLinkSessionModal">
                <template #icon>
                  <n-icon size="14"><AddOutline /></n-icon>
                </template>
                {{ t('task.linkAiSession') }}
              </n-button>
            </div>

            <div v-if="linkedSessionsLoading" class="ai-session__loading">
              <n-spin size="small" />
            </div>
            <n-list v-else-if="linkedAiSessions.length > 0" bordered>
              <n-list-item v-for="session in linkedAiSessions" :key="session.id">
                <div class="linked-session-item">
                  <div class="linked-session-info">
                    <div class="linked-session-title">{{ session.title || t('terminal.untitledSession') }}</div>
                    <div class="linked-session-meta">
                      <n-tag size="tiny" :type="session.type === 'claude_code' ? 'info' : 'success'">
                        {{ session.type === 'claude_code' ? 'Claude' : 'Codex' }}
                      </n-tag>
                      <span class="linked-session-time">{{ formatDate(session.sessionStartedAt) }}</span>
                      <span class="linked-session-count">{{ session.messageCount }} msgs</span>
                    </div>
                  </div>
                  <n-space :size="4">
                    <n-tooltip>
                      <template #trigger>
                        <n-button size="tiny" quaternary type="default" @click.stop="copySessionId(session)">
                          <template #icon>
                            <n-icon size="14"><CopyOutline /></n-icon>
                          </template>
                        </n-button>
                      </template>
                      {{ t('task.copySessionId') }}
                    </n-tooltip>
                    <n-button size="tiny" quaternary type="info" @click="viewLinkedConversation(session)">
                      {{ t('task.viewConversation') }}
                    </n-button>
                    <n-tooltip>
                      <template #trigger>
                        <n-button size="tiny" quaternary type="default" @click="confirmUnlinkSession(session)">
                          <template #icon>
                            <n-icon size="14"><CloseOutline /></n-icon>
                          </template>
                        </n-button>
                      </template>
                      {{ t('task.unlinkAiSession') }}
                    </n-tooltip>
                  </n-space>
                </div>
              </n-list-item>
            </n-list>
            <n-empty v-else :description="t('task.noAiMessages')" size="small" />
          </section>

          <n-divider />

          <section>
            <div class="task-detail__section-header">
              <h3>{{ t('task.comments') }}</h3>
            </div>

            <n-space vertical size="small">
              <n-input
                v-model:value="newComment"
                type="textarea"
                rows="3"
                :placeholder="t('task.commentPlaceholder')"
              />
              <n-button
                type="primary"
                size="small"
                :loading="commentLoading"
                @click="handleCreateComment"
              >
                {{ t('task.publishComment') }}
              </n-button>
            </n-space>

            <n-list v-if="comments.length" bordered style="margin-top: 12px">
              <n-list-item v-for="comment in comments" :key="comment.id">
                <n-space justify="space-between" align="center">
                  <div class="task-detail__comment">
                    <div class="content">{{ comment.content }}</div>
                    <n-text depth="3">{{ formatDate(comment.createdAt) }}</n-text>
                  </div>
                  <n-button
                    quaternary
                    type="error"
                    size="tiny"
                    @click="handleDeleteComment(comment.id)"
                  >
                    {{ t('task.deleteComment') }}
                  </n-button>
                </n-space>
              </n-list-item>
            </n-list>
            <n-empty v-else :description="t('task.noComments')" />
          </section>
        </div>
      </n-spin>

      <template #footer>
        <n-space justify="space-between" style="width: 100%">
          <n-button tertiary @click="emit('update:show', false)">{{ t('common.close') }}</n-button>
          <n-space>
            <n-button type="error" tertiary :loading="deleteLoading" @click="confirmDelete">{{
              t('task.deleteTask')
            }}</n-button>
            <n-button type="primary" :loading="saveLoading" @click="handleSave">{{
              t('task.saveChanges')
            }}</n-button>
          </n-space>
        </n-space>
      </template>
    </n-drawer-content>
  </n-drawer>

  <!-- 查看对话模态框 -->
  <n-modal
    v-model:show="showConversationModal"
    preset="card"
    :title="currentConversationTitle"
    style="width: 800px; max-width: 90vw; max-height: 85vh"
  >
    <n-spin :show="conversationLoading">
      <div class="conversation-container" style="max-height: 60vh; overflow-y: auto">
        <template v-if="currentConversation && currentConversation.messages.length > 0">
          <div
            v-for="(msg, index) in currentConversation.messages"
            :key="index"
            class="message-item"
            :class="msg.role"
          >
            <div class="message-header">
              <span class="message-role">{{ msg.role === 'user' ? t('terminal.user') : t('terminal.assistant') }}</span>
              <span v-if="msg.timestamp" class="message-time">{{ formatDate(msg.timestamp) }}</span>
            </div>
            <div class="message-content">{{ msg.content }}</div>
          </div>
        </template>
        <n-empty v-else-if="!conversationLoading" :description="t('terminal.noMessages')" />
      </div>
    </n-spin>
    <template #footer>
      <div v-if="currentConversationSession" class="conversation-footer">
        <div class="conversation-footer__info">
          <n-tag size="small" :type="currentConversationSession.type === 'claude_code' ? 'info' : 'success'">
            <template #icon>
              <n-icon size="12">
                <svg v-if="currentConversationSession.type === 'claude_code'" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
                </svg>
                <LogoGithub v-else />
              </n-icon>
            </template>
            {{ currentConversationSession.type === 'claude_code' ? 'Claude Code' : 'Codex' }}
          </n-tag>
          <code class="session-id-code">{{ currentConversationSession.sessionId }}</code>
          <n-button size="tiny" quaternary @click="copySessionId(currentConversationSession)">
            <template #icon>
              <n-icon size="12"><CopyOutline /></n-icon>
            </template>
          </n-button>
        </div>
      </div>
    </template>
  </n-modal>

  <!-- 关联 AI Session 模态框 -->
  <n-modal
    v-model:show="showLinkSessionModal"
    preset="card"
    :title="t('task.selectAiSession')"
    style="width: 600px; max-width: 90vw; max-height: 80vh"
  >
    <n-spin :show="availableSessionsLoading">
      <div v-if="availableSessions.length > 0" class="available-sessions-list">
        <n-radio-group v-model:value="selectedSessionId" class="session-radio-group">
          <div
            v-for="session in availableSessions"
            :key="session.id"
            class="available-session-item"
            :class="{ selected: selectedSessionId === session.id }"
            @click="selectedSessionId = session.id"
          >
            <n-radio :value="session.id" />
            <div class="available-session-info">
              <div class="available-session-title">{{ session.title || t('terminal.untitledSession') }}</div>
              <div class="available-session-meta">
                <n-tag size="tiny" :type="session.type === 'claude_code' ? 'info' : 'success'">
                  {{ session.type === 'claude_code' ? 'Claude' : 'Codex' }}
                </n-tag>
                <span>{{ session.model || '-' }}</span>
                <span>{{ formatDate(session.sessionStartedAt) }}</span>
                <span>{{ session.messageCount }} msgs</span>
              </div>
            </div>
          </div>
        </n-radio-group>
      </div>
      <n-empty v-else-if="!availableSessionsLoading" :description="t('task.noAvailableAiSessions')" />
    </n-spin>
    <template #footer>
      <n-space justify="end">
        <n-button @click="showLinkSessionModal = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" :disabled="!selectedSessionId" @click="handleLinkSession">
          {{ t('task.linkAiSession') }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useDialog, useMessage } from 'naive-ui';
import { CloseOutline, AddOutline, CopyOutline, LogoGithub } from '@vicons/ionicons5';
import dayjs from 'dayjs';
import { useTaskStore } from '@/stores/task';
import { useProjectStore } from '@/stores/project';
import { taskActions } from '@/composables/useTaskActions';
import { extractItem, extractItems } from '@/api/response';
import type {
  Task,
  TaskComment,
  TaskAISessionWithDetails,
  ConversationResponse,
  AISessionSummary,
  ProjectAISessions,
} from '@/types/models';
import { useLocale } from '@/composables/useLocale';
import { http } from '@/api/http';

const { t } = useLocale();

const props = defineProps<{
  show: boolean;
  taskId?: string | null;
  projectId?: string;
}>();

const emit = defineEmits<{
  'update:show': [boolean];
  closed: [];
}>();

const taskStore = useTaskStore();
const projectStore = useProjectStore();
const {
  getTask,
  updateTask,
  bindWorktree,
  deleteTask,
  listComments,
  createComment,
  deleteCommentReq,
  invalidateTaskCache,
} = taskActions;
const message = useMessage();
const dialog = useDialog();

const form = ref({
  title: '',
  description: '',
  priority: 0,
  worktreeId: null as string | null,
  dueDate: null as string | null,
  tags: [] as string[],
});
const originalWorktreeId = ref<string | null>(null);
const newComment = ref('');

const detailLoading = ref(false);
const saveLoading = ref(false);
const deleteLoading = ref(false);
const commentLoading = ref(false);

// 关联的 AI Session 状态
const linkedAiSessions = ref<TaskAISessionWithDetails[]>([]);
const linkedSessionsLoading = ref(false);

// 对话查看模态框状态
const showConversationModal = ref(false);
const conversationLoading = ref(false);
const currentConversation = ref<ConversationResponse | null>(null);
const currentConversationTitle = ref('');
const currentConversationSession = ref<TaskAISessionWithDetails | null>(null);

// 关联 AI Session 模态框状态
const showLinkSessionModal = ref(false);
const availableSessionsLoading = ref(false);
const availableSessions = ref<AISessionSummary[]>([]);
const selectedSessionId = ref<string | null>(null);

const task = computed<Task | null>(() => {
  if (!props.taskId) {
    return null;
  }
  return taskStore.tasks.find(item => item.id === props.taskId) ?? null;
});

const comments = computed<TaskComment[]>(() => {
  if (!props.taskId) {
    return [];
  }
  return taskStore.commentsMap[props.taskId] ?? [];
});

const worktreeOptions = computed(() =>
  (projectStore.worktrees ?? []).map(worktree => ({
    label: worktree.branchName,
    value: worktree.id,
  }))
);

const priorityOptions = computed(() => [
  { label: t('task.priority.normal'), value: 0 },
  { label: t('task.priority.low'), value: 1 },
  { label: t('task.priority.medium'), value: 2 },
  { label: t('task.priority.high'), value: 3 },
]);

watch(
  () => task.value,
  value => {
    if (!value) {
      return;
    }
    form.value = {
      title: value.title,
      description: value.description ?? '',
      priority: value.priority,
      worktreeId: value.worktreeId ?? null,
      dueDate: value.dueDate ? dayjs(value.dueDate).format('YYYY-MM-DD') : null,
      tags: [...(value.tags ?? [])],
    };
    originalWorktreeId.value = value.worktreeId ?? null;
  },
  { immediate: true }
);

const handleAfterEnter = () => {
  void loadData();
};

async function loadData() {
  if (!props.show || !props.taskId) {
    return;
  }
  detailLoading.value = true;
  linkedAiSessions.value = [];
  try {
    const [taskResponse, commentsResponse] = await Promise.all([
      getTask.send(props.taskId),
      listComments.send(props.taskId),
    ]);
    const freshTask = extractItem(taskResponse) as unknown as Task | undefined;
    if (freshTask) {
      taskStore.upsertTask(freshTask);
    }
    const items = extractItems(commentsResponse) as unknown as TaskComment[];
    taskStore.setComments(props.taskId, items);

    // 加载已关联的 AI Session
    void loadLinkedAISessions(props.taskId);
  } catch (error: any) {
    message.error(error?.message ?? t('task.loadCommentsFailed'));
  } finally {
    detailLoading.value = false;
  }
}

// 加载已关联的 AI Session
async function loadLinkedAISessions(taskId: string) {
  linkedSessionsLoading.value = true;
  try {
    const response = await http.Get<{ items: TaskAISessionWithDetails[] }>(
      `/tasks/${taskId}/ai-sessions`
    ).send();
    linkedAiSessions.value = response?.items ?? [];
  } catch (error: any) {
    console.error('Failed to load linked AI sessions:', error);
  } finally {
    linkedSessionsLoading.value = false;
  }
}

// 确认解除关联
function confirmUnlinkSession(session: TaskAISessionWithDetails) {
  dialog.warning({
    title: t('task.unlinkAiSession'),
    content: t('task.confirmUnlinkAiSession'),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: () => handleUnlinkSession(session),
  });
}

// 解除关联 AI Session
async function handleUnlinkSession(session: TaskAISessionWithDetails) {
  if (!props.taskId) return;

  try {
    await http.Post(`/tasks/${props.taskId}/ai-sessions/unlink`, {
      aiSessionId: session.aiSessionDbId,
    }).send();
    message.success(t('task.aiSessionUnlinked'));
    // 从列表中移除
    linkedAiSessions.value = linkedAiSessions.value.filter(s => s.id !== session.id);
  } catch (error: any) {
    message.error(error?.message ?? t('task.aiSessionUnlinkFailed'));
  }
}

// 查看关联的会话对话
async function viewLinkedConversation(session: TaskAISessionWithDetails) {
  currentConversationTitle.value = session.title || t('terminal.untitledSession');
  currentConversationSession.value = session;
  showConversationModal.value = true;
  conversationLoading.value = true;
  currentConversation.value = null;

  try {
    const response = await http.Get<{ item: ConversationResponse }>(
      `/ai-sessions/${session.aiSessionDbId}/conversation`
    ).send();
    if (response?.item) {
      currentConversation.value = response.item;
    }
  } catch (error: any) {
    message.error(t('terminal.loadConversationFailed'));
  } finally {
    conversationLoading.value = false;
  }
}

// 复制 Session ID
async function copySessionId(session: TaskAISessionWithDetails) {
  try {
    await navigator.clipboard.writeText(session.sessionId);
    message.success(t('task.sessionIdCopied'));
  } catch {
    message.error(t('terminal.copyFailed'));
  }
}

// 打开关联 AI Session 模态框
async function openLinkSessionModal() {
  if (!props.projectId) return;

  showLinkSessionModal.value = true;
  availableSessionsLoading.value = true;
  selectedSessionId.value = null;
  availableSessions.value = [];

  try {
    const response = await http.Get<{ item: ProjectAISessions }>(
      `/projects/${props.projectId}/ai-sessions`
    ).send();

    if (response?.item) {
      // 合并 Claude 和 Codex sessions，排除已关联的
      const linkedIds = new Set(linkedAiSessions.value.map(s => s.aiSessionDbId));
      const allSessions = [
        ...(response.item.claudeSessions || []),
        ...(response.item.codexSessions || []),
      ].filter(s => !linkedIds.has(s.id));

      // 按时间排序，最新的在前
      allSessions.sort((a, b) => {
        const timeA = new Date(a.lastMessageAt || a.sessionStartedAt).getTime();
        const timeB = new Date(b.lastMessageAt || b.sessionStartedAt).getTime();
        return timeB - timeA;
      });

      availableSessions.value = allSessions;
    }
  } catch (error: any) {
    message.error(t('task.loadAiSessionsFailed'));
  } finally {
    availableSessionsLoading.value = false;
  }
}

// 关联选中的 AI Session
async function handleLinkSession() {
  if (!props.taskId || !selectedSessionId.value) return;

  try {
    await http.Post(`/tasks/${props.taskId}/ai-sessions/link`, {
      aiSessionId: selectedSessionId.value,
    }).send();
    message.success(t('task.aiSessionLinked'));
    showLinkSessionModal.value = false;
    // 重新加载已关联的 sessions
    void loadLinkedAISessions(props.taskId);
  } catch (error: any) {
    message.error(error?.message ?? t('task.aiSessionLinkFailed'));
  }
}

async function handleSave() {
  if (!task.value) {
    return;
  }
  saveLoading.value = true;
  try {
    const payload = {
      title: form.value.title,
      description: form.value.description,
      priority: form.value.priority,
      tags: form.value.tags,
      dueDate: form.value.dueDate,
    };
    const response = await updateTask.send(task.value.id, payload);
    let updated = extractItem(response) as unknown as Task | undefined;

    if (form.value.worktreeId !== originalWorktreeId.value) {
      const bindResponse = await bindWorktree.send(task.value.id, form.value.worktreeId);
      updated = extractItem(bindResponse) as unknown as Task | undefined;
    }

    if (updated) {
      taskStore.upsertTask(updated);
      originalWorktreeId.value = updated.worktreeId ?? null;
    }
    // 更新后使缓存失效，确保其他地方获取最新数据
    invalidateTaskCache();
    message.success(t('task.taskUpdated'));
  } catch (error: any) {
    message.error(error?.message ?? t('task.saveFailed'));
  } finally {
    saveLoading.value = false;
  }
}

function confirmDelete() {
  if (!task.value) {
    return;
  }
  dialog.warning({
    title: t('task.deleteTask'),
    content: t('task.deleteConfirm'),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: handleDelete,
  });
}

async function handleDelete() {
  if (!task.value) {
    return;
  }
  deleteLoading.value = true;
  try {
    await deleteTask.send(task.value.id);
    taskStore.removeTask(task.value.id);
    emit('update:show', false);
    message.success(t('task.taskDeleted'));
  } catch (error: any) {
    message.error(error?.message ?? t('task.deleteTaskFailed'));
  } finally {
    deleteLoading.value = false;
  }
}

async function handleCreateComment() {
  if (!task.value || !newComment.value.trim()) {
    return;
  }
  commentLoading.value = true;
  try {
    const response = await createComment.send(task.value.id, newComment.value.trim());
    const comment = extractItem<TaskComment>(response);
    if (comment) {
      taskStore.appendComment(task.value.id, comment);
      newComment.value = '';
    }
  } catch (error: any) {
    message.error(error?.message ?? t('task.publishCommentFailed'));
  } finally {
    commentLoading.value = false;
  }
}

async function handleDeleteComment(commentId: string) {
  if (!task.value) {
    return;
  }
  try {
    await deleteCommentReq.send(commentId);
    taskStore.removeComment(task.value.id, commentId);
  } catch (error: any) {
    message.error(error?.message ?? t('task.deleteCommentFailed'));
  }
}

const formatDate = (value: string) => dayjs(value).format('YYYY-MM-DD HH:mm');
</script>

<style scoped>
.task-detail {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-detail__section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.task-detail__section-header h3 {
  margin: 0;
  font-size: 16px;
}

.task-detail__comment {
  display: flex;
  flex-direction: column;
}

.task-detail__comment .content {
  margin-bottom: 4px;
  white-space: pre-wrap;
  word-break: break-word;
}

.ai-session__loading {
  display: flex;
  justify-content: center;
  padding: 16px;
}

/* 关联的 AI Session 列表 */
.linked-session-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.linked-session-info {
  flex: 1;
  min-width: 0;
}

.linked-session-title {
  font-weight: 500;
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.linked-session-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.linked-session-time,
.linked-session-count {
  color: var(--n-text-color-3);
}

/* 对话查看器样式 */
.conversation-container {
  padding: 8px 0;
}

.message-item {
  margin-bottom: 16px;
  padding: 12px 16px;
  border-radius: 8px;
  background: var(--n-color-embedded);
}

.message-item.user {
  border-left: 3px solid var(--n-primary-color);
}

.message-item.assistant {
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

/* 可用 AI Session 列表 */
.available-sessions-list {
  max-height: 400px;
  overflow-y: auto;
}

.session-radio-group {
  width: 100%;
}

.available-session-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  margin-bottom: 8px;
  background: var(--n-color-embedded);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.available-session-item:hover {
  border-color: var(--n-border-color);
}

.available-session-item.selected {
  border-color: var(--n-primary-color);
  background: var(--n-color-embedded);
}

.available-session-info {
  flex: 1;
  min-width: 0;
}

.available-session-title {
  font-weight: 500;
  font-size: 14px;
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.available-session-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--n-text-color-3);
}
</style>
