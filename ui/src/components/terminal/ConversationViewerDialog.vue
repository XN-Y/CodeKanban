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
    <ConversationViewer
      :messages="conversation?.messages ?? []"
      :loading="loading"
      :session-info="sessionInfo"
      @load-tool-result="loadToolResult"
    />
  </n-modal>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue';
import { useMessage } from 'naive-ui';
import { useLocale } from '@/composables/useLocale';
import { http } from '@/api/http';
import ConversationViewer, { type SessionInfo } from '@/components/common/ConversationViewer.vue';

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

const sessionInfo = computed<SessionInfo | null>(() => {
  if (!props.sessionId) return null;
  return {
    sessionId: props.sessionId,
  };
});

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
      .Get<{ item?: ConversationResponse }>(
        `/ai-sessions/by-session-id/${sessionId}/conversation`,
        {
          cacheFor: 0,
        }
      )
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

async function loadToolResult(toolUseId: string) {
  const sessionId = props.sessionId;
  if (!sessionId || !toolUseId) return;

  try {
    const response = await http
      .Get<{
        item?: ToolResultResponse;
      }>(`/ai-sessions/by-session-id/${sessionId}/conversation/tool-results/${encodeURIComponent(toolUseId)}`, { cacheFor: 0 })
      .send();

    const content = response?.item?.content;
    if (!content || !conversation.value) return;

    const msg = conversation.value.messages.find(m => m.toolUseId === toolUseId);
    if (msg) {
      msg.full = content;
    }
  } catch (error) {
    console.error('Failed to load tool result:', error);
    message.error(t('terminal.loadConversationFailed'));
  }
}

function handleClose() {
  showModal.value = false;
  conversation.value = null;
}
</script>
