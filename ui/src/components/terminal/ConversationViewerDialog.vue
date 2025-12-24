<template>
  <n-modal
    v-model:show="showModal"
    preset="card"
    :title="title"
    style="width: 800px; max-width: 90vw; max-height: 85vh"
    :mask-closable="true"
    :closable="true"
    @close="handleClose"
  >
    <n-spin :show="loading">
      <div class="conversation-container">
        <template v-if="conversation && conversation.messages.length > 0">
          <div
            v-for="(msg, index) in conversation.messages"
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
        <n-empty v-else-if="!loading" :description="t('terminal.noMessages')" />
      </div>
    </n-spin>
    <template #footer>
      <div v-if="sessionId" class="conversation-footer">
        <div class="conversation-footer__info">
          <code class="session-id-code">{{ sessionId }}</code>
          <n-button size="tiny" quaternary @click="copySessionId">
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
import { ref, watch, computed } from 'vue';
import { useMessage } from 'naive-ui';
import { CopyOutline } from '@vicons/ionicons5';
import { useLocale } from '@/composables/useLocale';
import { http } from '@/api/http';
import { useTimeAgo } from '@vueuse/core';

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

const props = defineProps<{
  sessionId: string | null;
}>();

const showModal = defineModel<boolean>('show', { default: false });

const { t } = useLocale();
const message = useMessage();

const loading = ref(false);
const conversation = ref<ConversationResponse | null>(null);

const title = computed(() => {
  if (conversation.value?.title) {
    return conversation.value.title;
  }
  return t('terminal.viewConversation');
});

function getTimeAgo(timestamp: string) {
  return useTimeAgo(new Date(timestamp)).value;
}

watch(
  () => [showModal.value, props.sessionId],
  async ([show, sessionId]) => {
    if (show && sessionId) {
      await loadConversation(sessionId as string);
    }
  },
  { immediate: true }
);

async function loadConversation(sessionId: string) {
  loading.value = true;
  conversation.value = null;

  try {
    const response = await http
      .Get<{ item?: ConversationResponse }>(`/ai-sessions/by-session-id/${sessionId}/conversation`, {
        cacheFor: 0,
      })
      .send();

    if (response?.item) {
      conversation.value = response.item;
    }
  } catch (error) {
    console.error('Failed to load conversation:', error);
    message.error(t('terminal.loadConversationFailed'));
  } finally {
    loading.value = false;
  }
}

async function copySessionId() {
  if (!props.sessionId) return;
  try {
    await navigator.clipboard.writeText(props.sessionId);
    message.success(t('terminal.aiSessionIdCopied'));
  } catch {
    message.error(t('terminal.copyFailed'));
  }
}

function handleClose() {
  showModal.value = false;
  conversation.value = null;
}
</script>

<style scoped>
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
