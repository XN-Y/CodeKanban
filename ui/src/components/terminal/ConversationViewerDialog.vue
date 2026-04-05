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
    <template #header-extra>
      <n-space :size="6" align="center">
        <n-tooltip>
          <template #trigger>
            <n-button
              quaternary
              circle
              size="small"
              :disabled="!navState.hasPrev"
              @click="viewerRef?.goToPrevUserMessage()"
            >
              <template #icon>
                <n-icon><ChevronUpOutline /></n-icon>
              </template>
            </n-button>
          </template>
          {{ t('terminal.prevUserMessage') }}
        </n-tooltip>
        <span class="conversation-nav-indicator">
          {{
            t('terminal.userMessagePosition', {
              current: navState.currentUserPosition,
              total: navState.totalUserMessages,
            })
          }}
        </span>
        <n-tooltip>
          <template #trigger>
            <n-button
              quaternary
              circle
              size="small"
              :disabled="!navState.hasNext"
              @click="viewerRef?.goToNextUserMessage()"
            >
              <template #icon>
                <n-icon><ChevronDownOutline /></n-icon>
              </template>
            </n-button>
          </template>
          {{ t('terminal.nextUserMessage') }}
        </n-tooltip>
      </n-space>
    </template>
    <ConversationViewer
      ref="viewerRef"
      :messages="conversation?.messages ?? []"
      :loading="loading"
      :session-info="sessionInfo"
      :load-tool-result="loadToolResult"
      @nav-state-change="updateNavState"
    />
  </n-modal>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import { useMessage } from 'naive-ui';
import { ChevronDownOutline, ChevronUpOutline } from '@vicons/ionicons5';
import { useLocale } from '@/composables/useLocale';
import { http } from '@/api/http';
import ConversationViewer, {
  type ConversationMessage,
  type ConversationViewerNavState,
  type SessionInfo,
} from '@/components/common/ConversationViewer.vue';

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
const viewerRef = ref<{
  goToPrevUserMessage: () => void;
  goToNextUserMessage: () => void;
  syncNavigationState?: () => void;
} | null>(null);
const navState = ref<ConversationViewerNavState>({
  currentUserPosition: 0,
  totalUserMessages: 0,
  hasPrev: false,
  hasNext: false,
});

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
      await nextTick();
      viewerRef.value?.syncNavigationState?.();
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
  if (!sessionId || !toolUseId) return null;

  try {
    const response = await http
      .Get<{
        item?: {
          toolUseId: string;
          content: string;
        };
      }>(
        `/ai-sessions/by-session-id/${sessionId}/conversation/tool-results/${encodeURIComponent(toolUseId)}`,
        { cacheFor: 0 }
      )
      .send();

    const content = response?.item?.content;
    if (!content || !conversation.value) return null;

    const msg = conversation.value.messages.find(m => m.toolUseId === toolUseId);
    if (msg) {
      msg.full = content;
    }
    return content;
  } catch (error) {
    console.error('Failed to load tool result:', error);
    message.error(t('terminal.loadConversationFailed'));
    return null;
  }
}

function updateNavState(value: ConversationViewerNavState) {
  navState.value = value;
}

function handleClose() {
  showModal.value = false;
  conversation.value = null;
  navState.value = {
    currentUserPosition: 0,
    totalUserMessages: 0,
    hasPrev: false,
    hasNext: false,
  };
}
</script>

<style scoped>
.conversation-nav-indicator {
  min-width: 52px;
  text-align: center;
  font-size: 12px;
  color: var(--n-text-color-3);
}
</style>
