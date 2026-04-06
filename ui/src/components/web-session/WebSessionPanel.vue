<template>
  <div class="web-session-panel">
    <WebSessionCompletionNotifier />
    <WebSessionApprovalNotifier />

    <div class="panel-main">
      <div class="panel-body">
        <div class="panel-content">
          <div class="panel-header">
            <div v-if="isMobile && sessions.length > 0" class="mobile-tab-selector">
              <button
                type="button"
                class="mobile-nav-btn"
                :disabled="!hasPrevSession"
                @click="goToPrevSession"
              >
                <n-icon size="18">
                  <ChevronBackOutline />
                </n-icon>
              </button>
              <n-dropdown
                trigger="manual"
                placement="bottom-start"
                :show="showMobileTabSelector"
                :options="mobileTabOptions"
                @select="handleMobileTabSelect"
                @clickoutside="showMobileTabSelector = false"
              >
                <button
                  type="button"
                  class="mobile-tab-trigger"
                  @click="showMobileTabSelector = !showMobileTabSelector"
                >
                  <span class="mobile-tab-title">{{ activeSessionTitle }}</span>
                  <n-icon class="mobile-tab-arrow" :class="{ 'is-open': showMobileTabSelector }">
                    <ChevronDownOutline />
                  </n-icon>
                </button>
              </n-dropdown>
              <button
                type="button"
                class="mobile-nav-btn"
                :disabled="!hasNextSession"
                @click="goToNextSession"
              >
                <n-icon size="18">
                  <ChevronForwardOutline />
                </n-icon>
              </button>
            </div>

            <div v-else-if="sessions.length > 0" ref="tabsContainerRef" class="tabs-container">
              <n-tabs
                :value="activeSessionId"
                type="card"
                closable
                size="small"
                :theme-overrides="tabsThemeOverrides"
                @update:value="handleSessionSelect"
                @close="handleDeleteSession"
              >
                <n-tab-pane
                  v-for="session in sessions"
                  :key="session.id"
                  :name="session.id"
                  display-directive="show:lazy"
                  :tab-props="createTabProps(session)"
                >
                  <template #tab>
                    <span class="tab-label" :title="session.title">
                      <span
                        v-if="shouldShowSessionStatusDot(session)"
                        class="status-dot"
                        :class="session.status"
                      ></span>
                      <span class="tab-title" :style="tabTitleStyle">{{ session.title }}</span>
                      <span
                        class="ai-status-pill"
                        :class="[
                          `state-${getSessionAssistantStateClass(session)}`,
                          getSessionPillSizeClass(),
                        ]"
                        :title="getSessionStatusTooltip(session)"
                      >
                        <span
                          class="ai-status-icon"
                          v-html="getSessionAssistantIcon(session)"
                        ></span>
                        <span class="ai-status-text">{{ getSessionStatusLabel(session) }}</span>
                        <span class="ai-status-emoji">{{ getSessionStatusEmoji(session) }}</span>
                      </span>
                    </span>
                  </template>
                </n-tab-pane>
              </n-tabs>
              <div class="active-tab-indicator" :style="activeTabIndicatorStyle"></div>
            </div>

            <div v-else class="empty-tabs-label">{{ t('webSession.emptyTitle') }}</div>

            <n-dropdown
              trigger="manual"
              placement="bottom-start"
              :show="!!contextMenuSession"
              :options="contextMenuOptions"
              :x="contextMenuX"
              :y="contextMenuY"
              @select="handleContextMenuSelect"
              @clickoutside="contextMenuSession = null"
            />

            <div class="header-actions">
              <n-button
                secondary
                size="small"
                class="new-session-button"
                :title="t('webSession.newSession')"
                :aria-label="t('webSession.newSession')"
                @click="handleCreateSession()"
              >
                <template #icon>
                  <n-icon><AddOutline /></n-icon>
                </template>
              </n-button>
            </div>
          </div>

          <div v-if="currentSession" class="timeline-shell">
            <div ref="timelineScrollRef" class="timeline-scroll" @scroll="handleTimelineScroll">
              <div class="timeline-list">
                <div v-if="historyMeta.loading" class="history-loading">
                  {{ t('common.loading') }}
                </div>

                <div v-if="blocks.length === 0" class="timeline-intro">
                  <span class="timeline-intro-badge">
                    {{ currentSession.agent === 'codex' ? 'Codex' : 'Claude' }}
                  </span>
                  <div class="timeline-intro-title">{{ t('webSession.readyTitle') }}</div>
                  <div class="timeline-intro-text">{{ t('webSession.readyDescription') }}</div>
                </div>

                <div
                  v-for="item in blocks"
                  :key="item.key"
                  class="timeline-item"
                  :class="`kind-${item.kind}`"
                >
                  <div class="item-meta">
                    <span class="item-role">
                      {{
                        item.kind === 'user'
                          ? t('terminal.user')
                          : item.kind === 'assistant'
                            ? t('terminal.assistant')
                            : t('common.info')
                      }}
                    </span>
                    <span class="item-time">{{ formatTime(item.timestamp) }}</span>
                  </div>

                  <div class="item-bubble" :class="item.level ? `level-${item.level}` : undefined">
                    <div
                      v-if="item.text"
                      class="item-text chat-markdown"
                      v-html="renderMarkdown(item.text)"
                    ></div>
                    <div v-if="item.attachments.length > 0" class="attachment-row">
                      <span
                        v-for="attachment in item.attachments"
                        :key="attachment.id"
                        class="attachment-pill"
                      >
                        <n-popover
                          v-if="canPreviewAttachment(attachment)"
                          trigger="hover"
                          placement="top-start"
                          :delay="120"
                        >
                          <template #trigger>
                            <button
                              type="button"
                              class="attachment-preview-trigger"
                              :title="attachment.name"
                              @click="openAttachmentPreview(attachment)"
                            >
                              <span class="attachment-preview-trigger-text">{{
                                attachment.name
                              }}</span>
                            </button>
                          </template>
                          <div class="attachment-hover-preview">
                            <img
                              :src="getAttachmentPreviewUrl(attachment.id)"
                              :alt="attachment.name"
                              class="attachment-hover-image"
                              loading="lazy"
                            />
                          </div>
                        </n-popover>
                        <button
                          v-else
                          type="button"
                          class="attachment-preview-trigger is-static"
                          :title="attachment.name"
                        >
                          <span class="attachment-preview-trigger-text">{{ attachment.name }}</span>
                        </button>
                      </span>
                    </div>

                    <div v-if="item.tools.length > 0" class="tool-list">
                      <div v-for="tool in item.tools" :key="tool.id" class="tool-card">
                        <button
                          type="button"
                          class="tool-header"
                          @click="toggleToolExpanded(tool.id)"
                        >
                          <span class="tool-header-main">
                            <span class="tool-header-leading">
                              <span class="tool-kind">{{ toolKindLabel(tool) }}</span>
                              <span class="tool-name">{{ tool.name }}</span>
                            </span>
                            <span class="tool-state-badge" :class="`state-${tool.status}`">
                              <span class="tool-state-dot"></span>
                              {{ toolStateLabel(tool) }}
                            </span>
                          </span>
                          <span v-if="toolPreview(tool)" class="tool-preview">{{
                            toolPreview(tool)
                          }}</span>
                        </button>
                        <div v-if="isToolExpanded(tool.id)" class="tool-body">
                          <div v-if="tool.input" class="tool-section">
                            <div class="tool-section-label">{{ t('webSession.toolInput') }}</div>
                            <pre class="tool-code">{{ stringifyValue(tool.input) }}</pre>
                          </div>
                          <div v-if="tool.output" class="tool-section">
                            <div class="tool-section-label">{{ t('webSession.toolOutput') }}</div>
                            <pre class="tool-code">{{ tool.output }}</pre>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div
            v-if="currentSession && (liveState.phase !== 'idle' || pendingApproval)"
            class="runtime-strip"
          >
            <button
              type="button"
              class="live-card"
              :class="[
                `phase-${liveState.phase}`,
                {
                  'show-jump-hint': showJumpToBottom,
                },
              ]"
              :title="t('webSession.jumpToBottom')"
              @click="handleLiveCardClick"
            >
              <div class="live-card-main">
                <span class="live-orb"></span>
                <div class="live-copy">
                  <div class="live-title">{{ liveStateLabel }}</div>
                  <div v-if="liveStateDetail" class="live-detail">{{ liveStateDetail }}</div>
                </div>
              </div>
              <div class="live-meta">
                <span v-if="liveStateWorking" class="live-activity" aria-hidden="true">
                  <span class="live-activity-bar"></span>
                  <span class="live-activity-bar"></span>
                  <span class="live-activity-bar"></span>
                </span>
                <span v-if="showJumpToBottom" class="live-jump-hint">
                  {{ t('webSession.jumpToBottom') }}
                </span>
                <span class="live-time">{{ formatTime(liveState.updatedAt) }}</span>
              </div>
            </button>

            <div v-if="pendingApproval" class="approval-card">
              <div class="approval-card-header">
                <span class="approval-badge">{{ t('webSession.approvalTitle') }}</span>
                <span class="approval-time">{{ formatTime(pendingApproval.requestedAt) }}</span>
              </div>
              <div class="approval-prompt">
                {{ pendingApproval.prompt || t('webSession.approvalPromptFallback') }}
              </div>
              <div class="approval-actions">
                <n-button size="small" type="primary" @click="handleApproval('approve')">
                  {{ t('webSession.approvalApprove') }}
                </n-button>
                <n-button size="small" secondary @click="handleApproval('reject')">
                  {{ t('webSession.approvalReject') }}
                </n-button>
                <n-button size="small" tertiary @click="handleAbortCurrent">
                  {{ t('webSession.stop') }}
                </n-button>
              </div>
            </div>
          </div>

          <div v-else-if="!currentSession" class="empty-state">
            <n-empty :description="t('webSession.emptyDescription')">
              <template #extra>
                <n-button type="primary" @click="handleCreateSession()">
                  {{ t('webSession.newSession') }}
                </n-button>
              </template>
            </n-empty>
          </div>

          <div class="composer">
            <input
              ref="fileInputRef"
              type="file"
              accept="image/*"
              multiple
              class="hidden-file-input"
              @change="handleFileChange"
            />

            <div
              class="composer-shell"
              :class="{
                'is-running': liveState.running,
                'is-drag-over': isComposerDragOver,
              }"
              @paste.capture="handleComposerPaste"
              @dragenter="handleComposerDragEnter"
              @dragover="handleComposerDragOver"
              @dragleave="handleComposerDragLeave"
              @drop="handleComposerDrop"
            >
              <div class="composer-config">
                <div class="composer-config-row">
                  <n-select
                    v-model:value="selectedAgent"
                    class="composer-select agent-select"
                    size="small"
                    :options="agentOptions"
                    :disabled="Boolean(currentSession?.nativeSessionId)"
                  />
                  <n-select
                    v-model:value="selectedModel"
                    class="composer-select model-select"
                    size="small"
                    :options="modelOptions"
                  />
                  <n-select
                    v-if="selectedAgent === 'codex'"
                    v-model:value="selectedReasoningEffort"
                    class="composer-select reasoning-select"
                    size="small"
                    :options="reasoningEffortOptions"
                  />
                  <div class="composer-mode-row">
                    <n-button-group class="composer-mode-switch">
                      <n-button
                        size="small"
                        :type="selectedBaseMode === 'default' ? 'primary' : 'default'"
                        @click="setBaseMode('default')"
                      >
                        Default
                      </n-button>
                      <n-button
                        size="small"
                        :type="selectedBaseMode === 'plan' ? 'primary' : 'default'"
                        @click="setBaseMode('plan')"
                      >
                        Plan
                      </n-button>
                    </n-button-group>
                    <n-checkbox
                      v-model:checked="autoApproveEnabled"
                      class="composer-auto-approve"
                      size="small"
                    >
                      {{ t('webSession.autoApproveLabel') }}
                    </n-checkbox>
                  </div>
                  <div v-if="currentSession" class="composer-path" :title="currentSession.cwd">
                    {{ currentSession.cwd }}
                  </div>
                </div>
              </div>

              <div v-if="draftAttachments.length > 0" class="draft-attachments">
                <span
                  v-for="attachment in draftAttachments"
                  :key="attachment.id"
                  class="draft-attachment-pill"
                >
                  <n-popover
                    v-if="canPreviewAttachment(attachment)"
                    trigger="hover"
                    placement="top-start"
                    :delay="120"
                  >
                    <template #trigger>
                      <button
                        type="button"
                        class="attachment-preview-trigger"
                        :title="attachment.name"
                        @click="openAttachmentPreview(attachment)"
                      >
                        <span class="attachment-preview-trigger-text">{{ attachment.name }}</span>
                      </button>
                    </template>
                    <div class="attachment-hover-preview">
                      <img
                        :src="getAttachmentPreviewUrl(attachment.id)"
                        :alt="attachment.name"
                        class="attachment-hover-image"
                        loading="lazy"
                      />
                    </div>
                  </n-popover>
                  <button
                    v-else
                    type="button"
                    class="attachment-preview-trigger is-static"
                    :title="attachment.name"
                  >
                    <span class="attachment-preview-trigger-text">{{ attachment.name }}</span>
                  </button>
                  <button
                    type="button"
                    class="draft-attachment-remove"
                    @click="removeAttachment(attachment.id)"
                  >
                    ×
                  </button>
                </span>
              </div>

              <div v-if="pendingInputs.length > 0" class="pending-inputs">
                <div v-for="item in pendingInputs" :key="item.id" class="pending-input-item">
                  <span class="pending-input-badge" :class="`mode-${item.mode}`">
                    {{ pendingModeLabel(item.mode) }}
                  </span>
                  <span class="pending-input-preview">{{ pendingInputPreview(item) }}</span>
                  <button
                    type="button"
                    class="pending-input-remove"
                    @click="handleRemovePendingInput(item.id)"
                  >
                    ×
                  </button>
                </div>
              </div>

              <n-input
                v-model:value="composerText"
                type="textarea"
                class="composer-input"
                :autosize="{ minRows: 2, maxRows: 7 }"
                :placeholder="composerPlaceholder"
                @keydown.enter.exact="handleComposerEnter"
              />

              <div class="composer-footer">
                <div class="composer-footer-left">
                  <button type="button" class="composer-icon-btn" @click="openFilePicker">
                    <n-icon size="14"><ImageOutline /></n-icon>
                  </button>
                  <span class="composer-hint">{{ composerHint }}</span>
                </div>

                <div class="composer-footer-right">
                  <n-button
                    v-if="currentSession?.status === 'running'"
                    secondary
                    type="warning"
                    class="composer-stop-btn"
                    @click="handleAbortCurrent"
                  >
                    {{ t('webSession.stop') }}
                  </n-button>
                  <template v-if="canStageDuringRun">
                    <n-button secondary class="composer-queue-btn" @click="handlePreinput('queue')">
                      {{ t('webSession.preinputQueue') }}
                    </n-button>
                    <n-button
                      type="primary"
                      class="composer-send-btn"
                      @click="handlePreinput('redirect')"
                    >
                      {{ t('webSession.preinputRedirect') }}
                    </n-button>
                  </template>
                  <n-button
                    v-else
                    type="primary"
                    class="composer-send-btn"
                    :disabled="!canSend"
                    @click="handleSubmit"
                  >
                    {{ t('webSession.send') }}
                  </n-button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="showCrossProjectSidebar" ref="sidebarRootRef" class="session-sidebar-shell">
          <div
            class="terminal-resizer"
            :class="{ 'is-dragging': isSidebarResizing }"
            @mousedown="startSidebarResize"
          >
            <div class="resizer-handle"></div>
          </div>

          <aside class="session-sidebar" :style="{ width: effectiveSidebarWidthPx + 'px' }">
            <div class="session-sidebar-header">
              <div class="session-sidebar-title-wrap">
                <div class="session-sidebar-title">{{ t('webSession.allSessions') }}</div>
                <div class="session-sidebar-subtitle">
                  {{ t('webSession.crossProjectSessions') }}
                </div>
              </div>
              <span class="session-sidebar-count">{{ crossProjectSessions.length }}</span>
            </div>

            <div v-if="crossProjectSessions.length === 0" class="session-sidebar-empty">
              {{ t('webSession.emptyTitle') }}
            </div>

            <div v-else class="session-sidebar-list">
              <button
                v-for="item in crossProjectSessions"
                :key="`${item.projectId}:${item.session.id}`"
                type="button"
                class="session-sidebar-item"
                :class="[
                  'session-sidebar-row',
                  ...getSidebarSessionClasses(item),
                  { 'is-active': item.isCurrent },
                ]"
                :style="{ '--session-sidebar-accent': getSidebarSessionAccentColor(item) }"
                :title="`${item.projectName} · ${item.session.title}${getSidebarSessionSubtitle(item) ? ` · ${getSidebarSessionSubtitle(item)}` : ''}`"
                @click="handleSidebarSessionSelect(item)"
              >
                <div class="session-sidebar-main">
                  <div class="session-sidebar-title-line">
                    <span
                      class="session-sidebar-agent-icon"
                      v-html="getSessionAssistantIcon(item.session)"
                    ></span>
                    <span class="session-sidebar-item-title">{{ item.session.title }}</span>
                    <span v-if="getSidebarSessionSubtitle(item)" class="session-sidebar-state-text">
                      · {{ getSidebarSessionSubtitle(item) }}
                    </span>
                  </div>
                </div>

                <div class="session-sidebar-actions">
                  <span
                    v-if="item.projectIndex"
                    class="project-index-badge session-project-badge"
                    :class="{ 'is-single-project': isSingleSidebarProject }"
                    :style="{ '--badge-color': item.projectIndex.color }"
                  >
                    {{ item.projectIndex.index }}
                  </span>
                  <span
                    class="session-current-indicator"
                    :class="{ 'is-hidden': !item.isCurrent }"
                    :title="t('terminal.currentActiveSession')"
                  >
                    <svg
                      width="14"
                      height="14"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2.5"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    >
                      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
                      <circle cx="12" cy="12" r="3"></circle>
                    </svg>
                  </span>
                </div>
              </button>
            </div>
          </aside>
        </div>
      </div>
    </div>

    <n-modal
      :show="showAttachmentPreview"
      preset="card"
      class="attachment-preview-modal"
      :title="activeAttachmentPreview?.name"
      :bordered="false"
      :segmented="{ content: false, footer: false }"
      :mask-closable="true"
      closable
      style="width: min(92vw, 960px)"
      @update:show="handleAttachmentPreviewVisibilityChange"
    >
      <div v-if="activeAttachmentPreview" class="attachment-preview-modal-body">
        <img
          :src="activeAttachmentPreview.url"
          :alt="activeAttachmentPreview.name"
          class="attachment-preview-modal-image"
        />
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import {
  computed,
  h,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  shallowRef,
  watch,
  type HTMLAttributes,
} from 'vue';
import { useRouter } from 'vue-router';
import { useDebounceFn, useResizeObserver, useStorage } from '@vueuse/core';
import { storeToRefs } from 'pinia';
import { NCheckbox, NInput, useDialog, useMessage, type DropdownOption } from 'naive-ui';
import {
  AddOutline,
  ChevronBackOutline,
  ChevronDownOutline,
  ChevronForwardOutline,
  ImageOutline,
} from '@vicons/ionicons5';
import Sortable, { type SortableEvent } from 'sortablejs';
import { getPresetById } from '@/constants/themes';
import { useLocale } from '@/composables/useLocale';
import { useResponsive } from '@/composables/useResponsive';
import { useProjectStore } from '@/stores/project';
import { useSettingsStore } from '@/stores/settings';
import {
  useWebSessionStore,
  type WebSessionLiveState,
  type WebSessionPendingInput,
} from '@/stores/webSession';
import type { WebSessionSummary } from '@/types/models';
import {
  calculateCardTabIndicatorStyle,
  hiddenCardTabIndicatorStyle,
} from '@/utils/cardTabIndicator';
import { getAssistantIconByType } from '@/utils/assistantIcon';
import { renderMarkdown } from '@/utils/markdown';
import { urlBase } from '@/api';
import WebSessionApprovalNotifier from '@/components/web-session/WebSessionApprovalNotifier.vue';
import WebSessionCompletionNotifier from '@/components/web-session/WebSessionCompletionNotifier.vue';

const MAX_TAB_TITLE_WIDTH = 160;
const TAB_LABEL_EXTRA_SPACE = 40;
const TABS_CONTAINER_STATIC_OFFSET = 220;
const TABS_CONTAINER_MIN_OFFSET = 140;
const SHARED_WIDTH_HIDE_THRESHOLD = 860;
const SIDEBAR_STATUS_TEXT_THRESHOLD = 280;
const MIN_SESSION_SIDEBAR_WIDTH = 200;
const MAX_SESSION_SIDEBAR_WIDTH = 400;
const DEFAULT_SESSION_SIDEBAR_WIDTH = 240;
const MIN_SESSION_MAIN_WIDTH = 420;
const PROJECT_INDEX_COLORS = [
  '#10b981',
  '#3b82f6',
  '#f59e0b',
  '#8b5cf6',
  '#ec4899',
  '#14b8a6',
  '#ef4444',
  '#6366f1',
];

const props = withDefaults(
  defineProps<{
    projectId: string;
    showSidebar?: boolean;
    isActive?: boolean;
  }>(),
  {
    showSidebar: true,
    isActive: true,
  }
);

const webSessionStore = useWebSessionStore();
const projectStore = useProjectStore();
const settingsStore = useSettingsStore();
const router = useRouter();
const dialog = useDialog();
const message = useMessage();
const { t } = useLocale();
const { isMobile } = useResponsive();
const { activeTheme, currentPresetId, confirmBeforeTerminalClose } = storeToRefs(settingsStore);

const tabsContainerRef = ref<HTMLElement | null>(null);
const timelineScrollRef = ref<HTMLDivElement | null>(null);
const fileInputRef = ref<HTMLInputElement | null>(null);
const sidebarRootRef = ref<HTMLElement | null>(null);
const composerText = ref('');
const autoFollowBottom = ref(true);
const showJumpToBottom = ref(false);
const expandedTools = ref<Record<string, boolean>>({});
const lastNonYoloModeBySession = ref<Record<string, 'default' | 'plan'>>({});
const showMobileTabSelector = ref(false);
const contextMenuSession = ref<WebSessionSummary | null>(null);
const contextMenuX = ref(0);
const contextMenuY = ref(0);
const activeTabIndicatorStyle = ref(hiddenCardTabIndicatorStyle());
const tabsContainerWidth = ref(0);
const tabTitleMaxWidth = ref(MAX_TAB_TITLE_WIDTH);
const isComposerDragOver = ref(false);
const showAttachmentPreview = ref(false);
const activeAttachmentPreview = ref<{
  id: string;
  name: string;
  url: string;
} | null>(null);
const viewedEventSeqBySession = ref<Record<string, number>>({});
const pendingHistoryAnchor = ref<{
  sessionId: string;
  previousHeight: number;
  previousTop: number;
} | null>(null);
const tabDragSortable = shallowRef<Sortable | null>(null);
let composerDragDepth = 0;
const loadedSidebarProjectIds = new Set<string>();
const sidebarContainerWidth = ref(0);
const isSidebarResizing = ref(false);
const sidebarWidthPx = useStorage<number>(
  'workspace-web-session-sidebar-width',
  DEFAULT_SESSION_SIDEBAR_WIDTH
);
let sidebarResizeObserver: ResizeObserver | null = null;

const IMAGE_ATTACHMENT_NAME_PATTERN = /\.(png|jpe?g|gif|webp|bmp|svg|tiff?)$/i;

const draftAgent = ref<'claude' | 'codex'>('codex');
const draftModel = ref('gpt-5.4');
const draftReasoningEffort = ref<'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh'>('xhigh');
const draftBaseMode = ref<'default' | 'plan'>('default');
const draftYoloEnabled = ref(true);

const sessions = computed(() => webSessionStore.getSessions(props.projectId));
const currentSession = computed(() => webSessionStore.getActiveSession(props.projectId));
const blocks = computed(() =>
  currentSession.value ? webSessionStore.getBlocks(currentSession.value.id) : []
);
const liveState = computed(() =>
  currentSession.value
    ? webSessionStore.getLiveState(currentSession.value.id)
    : ({ phase: 'idle', running: false, updatedAt: Date.now() } as WebSessionLiveState)
);
const pendingApproval = computed(() =>
  currentSession.value ? webSessionStore.getPendingApproval(currentSession.value.id) : null
);
const historyMeta = computed(() =>
  currentSession.value
    ? webSessionStore.getHistoryMeta(currentSession.value.id)
    : { hasMore: false, beforeCursor: '', total: 0, loading: false }
);
const draftAttachments = computed(() => webSessionStore.getDraftAttachments(props.projectId));
const pendingInputs = computed(() =>
  currentSession.value ? webSessionStore.getPendingInputs(currentSession.value.id) : []
);
const currentSessionLatestEventSeq = computed(() =>
  currentSession.value ? webSessionStore.getLatestEventSeq(currentSession.value.id) : 0
);
const isRunActive = computed(() => Boolean(currentSession.value?.status === 'running'));
const hasDraftContent = computed(
  () =>
    composerText.value.trim().length > 0 ||
    webSessionStore.getDraftAttachments(props.projectId).length > 0
);
const canSend = computed(() => !isRunActive.value && hasDraftContent.value);
const canStageDuringRun = computed(() => isRunActive.value && hasDraftContent.value);
const composerPlaceholder = computed(() => t('webSession.inputPlaceholder'));
const composerHint = computed(() => {
  if (pendingApproval.value) {
    return t('webSession.composerHintApproval');
  }
  if (liveState.value.running) {
    return t('webSession.composerHintRunning');
  }
  return t('webSession.composerHintIdle');
});
const liveStateLabel = computed(() => {
  switch (liveState.value.phase) {
    case 'starting':
      return t('webSession.liveStarting');
    case 'thinking':
      return t('webSession.liveThinking');
    case 'tool':
      return t('webSession.liveTool', { tool: liveState.value.tool?.name || 'Tool' });
    case 'waiting_approval':
      return t('webSession.liveWaitingApproval');
    case 'done':
      return t('webSession.liveDone');
    case 'error':
      return t('webSession.liveError');
    default:
      return t('webSession.liveIdle');
  }
});
const liveStateDetail = computed(() => {
  if (pendingApproval.value?.prompt) {
    return pendingApproval.value.prompt;
  }
  if (liveState.value.phase === 'tool' && liveState.value.tool?.kind) {
    return liveState.value.tool.kind;
  }
  if (liveState.value.phase === 'error' && liveState.value.errorMessage) {
    return liveState.value.errorMessage;
  }
  return '';
});
const liveStateWorking = computed(() =>
  ['starting', 'thinking', 'tool'].includes(liveState.value.phase)
);
const activeSessionId = computed(() => currentSession.value?.id ?? '');
const activeSessionTitle = computed(
  () => currentSession.value?.title ?? t('webSession.emptyTitle')
);
const showCrossProjectSidebar = computed(() => !isMobile.value && props.showSidebar);
const currentSessionIndex = computed(() =>
  sessions.value.findIndex(session => session.id === activeSessionId.value)
);
const hasPrevSession = computed(() => currentSessionIndex.value > 0);
const hasNextSession = computed(
  () => currentSessionIndex.value >= 0 && currentSessionIndex.value < sessions.value.length - 1
);
const mobileTabOptions = computed<DropdownOption[]>(() =>
  sessions.value.map(session => ({
    label: session.title,
    key: session.id,
  }))
);
const contextMenuOptions = computed<DropdownOption[]>(() => [
  {
    label: t('webSession.newSession'),
    key: 'new',
  },
  {
    label: t('common.edit'),
    key: 'rename',
    disabled: !contextMenuSession.value,
  },
  {
    label: t('common.delete'),
    key: 'delete',
    disabled: !contextMenuSession.value,
  },
]);
const tabsThemeOverrides = computed(() => {
  const theme = activeTheme.value;
  const preset = getPresetById(currentPresetId.value);
  const tabBg = theme.terminalTabBg || preset?.colors.terminalTabBg || theme.bodyColor;
  const tabActiveBg =
    theme.terminalTabActiveBg || preset?.colors.terminalTabActiveBg || theme.surfaceColor;
  return {
    tabColor: tabBg,
    tabColorSegment: tabActiveBg,
  };
});
const completionColors = computed(() => {
  const theme = activeTheme.value;
  const preset = getPresetById(currentPresetId.value);
  return {
    bg:
      theme.terminalTabCompletionBg ||
      preset?.colors.terminalTabCompletionBg ||
      'rgba(16, 185, 129, 0.25)',
    border:
      theme.terminalTabCompletionBorder ||
      preset?.colors.terminalTabCompletionBorder ||
      'rgba(16, 185, 129, 0.5)',
  };
});
const approvalColors = computed(() => {
  const theme = activeTheme.value;
  const preset = getPresetById(currentPresetId.value);
  return {
    bg:
      theme.terminalTabApprovalBg ||
      preset?.colors.terminalTabApprovalBg ||
      'rgba(247, 144, 9, 0.25)',
    border:
      theme.terminalTabApprovalBorder ||
      preset?.colors.terminalTabApprovalBorder ||
      'rgba(247, 144, 9, 0.5)',
  };
});
const tabTitleStyle = computed(() => ({
  maxWidth: `${tabTitleMaxWidth.value}px`,
}));
const timelineContentVersion = computed(() =>
  blocks.value
    .map(block => {
      const toolVersion = block.tools
        .map(tool => `${tool.id}:${tool.status}:${String(tool.output ?? '').length}`)
        .join(',');
      return `${block.key}:${block.text.length}:${block.attachments.length}:${toolVersion}:${block.done ? 1 : 0}`;
    })
    .join('|')
);
const sidebarProjectIdsToLoad = computed(() => {
  const ids = new Set<string>();
  if (props.projectId) {
    ids.add(props.projectId);
  }
  projectStore.recentProjects.forEach(project => {
    if (project.id) {
      ids.add(project.id);
    }
  });
  projectStore.projects.forEach(project => {
    if (project.id) {
      ids.add(project.id);
    }
  });
  return Array.from(ids);
});

function parseTimestamp(value?: string | null) {
  if (!value) {
    return 0;
  }
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function getSessionActivityTimestamp(session: WebSessionSummary) {
  return parseTimestamp(session.lastMessageAt || session.updatedAt || session.createdAt);
}

function markSessionViewed(sessionId?: string) {
  const normalizedSessionId = String(sessionId || '').trim();
  if (!props.isActive || !normalizedSessionId) {
    return;
  }

  const latestSeq = webSessionStore.getLatestEventSeq(normalizedSessionId);
  const previousViewedSeq = viewedEventSeqBySession.value[normalizedSessionId] ?? -1;
  if (latestSeq <= previousViewedSeq) {
    return;
  }

  viewedEventSeqBySession.value = {
    ...viewedEventSeqBySession.value,
    [normalizedSessionId]: latestSeq,
  };
  webSessionStore.emitter.emit('web-session:viewed', {
    sessionId: normalizedSessionId,
  });
}

function hasSessionUnread(session: (typeof sessions.value)[number]) {
  const latestSeq = webSessionStore.getLatestEventSeq(session.id);
  const viewedSeq = viewedEventSeqBySession.value[session.id] ?? -1;
  if (latestSeq > 0) {
    return latestSeq > viewedSeq;
  }
  return session.hasUnread && (!props.isActive || activeSessionId.value !== session.id);
}

function getProjectName(projectId: string) {
  if (!projectId) {
    return '';
  }
  if (projectStore.currentProject?.id === projectId && projectStore.currentProject.name) {
    return projectStore.currentProject.name;
  }
  return (
    projectStore.projects.find(project => project.id === projectId)?.name ||
    projectStore.recentProjects.find(project => project.id === projectId)?.name ||
    projectId
  );
}

type CrossProjectSessionItem = {
  session: WebSessionSummary;
  projectId: string;
  projectName: string;
  activityAt: number;
  isCurrent: boolean;
  projectIndex?: { index: number; color: string };
};

const crossProjectSessions = computed<CrossProjectSessionItem[]>(() => {
  const rawItems: Omit<CrossProjectSessionItem, 'projectIndex'>[] = [];
  sidebarProjectIdsToLoad.value.forEach(projectId => {
    webSessionStore.getSessions(projectId).forEach(session => {
      rawItems.push({
        session,
        projectId,
        projectName: getProjectName(projectId),
        activityAt: getSessionActivityTimestamp(session),
        isCurrent: projectId === props.projectId && session.id === activeSessionId.value,
      });
    });
  });
  const sorted = rawItems.sort((left, right) => {
    if (right.activityAt !== left.activityAt) {
      return right.activityAt - left.activityAt;
    }
    if (left.isCurrent !== right.isCurrent) {
      return left.isCurrent ? -1 : 1;
    }
    const leftHasUnread = hasSessionUnread(left.session);
    const rightHasUnread = hasSessionUnread(right.session);
    if (leftHasUnread !== rightHasUnread) {
      return leftHasUnread ? -1 : 1;
    }
    const projectNameCompare = left.projectName.localeCompare(right.projectName);
    if (projectNameCompare !== 0) {
      return projectNameCompare;
    }
    if (left.session.orderIndex !== right.session.orderIndex) {
      return left.session.orderIndex - right.session.orderIndex;
    }
    return left.session.id.localeCompare(right.session.id);
  });

  const presentProjectIds = new Set(sorted.map(item => item.projectId).filter(Boolean));
  const projectIds: string[] = [];
  projectStore.projects.forEach(project => {
    if (project.id && presentProjectIds.has(project.id)) {
      projectIds.push(project.id);
    }
  });
  sorted.forEach(item => {
    if (item.projectId && !projectIds.includes(item.projectId)) {
      projectIds.push(item.projectId);
    }
  });

  const projectIndex = new Map<string, { index: number; color: string }>();
  projectIds.forEach((projectId, idx) => {
    projectIndex.set(projectId, {
      index: idx + 1,
      color: PROJECT_INDEX_COLORS[idx % PROJECT_INDEX_COLORS.length],
    });
  });

  return sorted.map(item => ({
    ...item,
    projectIndex: projectIndex.get(item.projectId),
  }));
});

const isSingleSidebarProject = computed(() => {
  const ids = new Set(crossProjectSessions.value.map(item => item.projectId).filter(Boolean));
  return ids.size <= 1;
});

function clamp(min: number, value: number, max: number) {
  return Math.max(min, Math.min(max, value));
}

const maxSidebarWidthByContainer = computed(() => {
  if (!sidebarContainerWidth.value) {
    return MAX_SESSION_SIDEBAR_WIDTH;
  }
  const maxAllowed = Math.max(
    MIN_SESSION_SIDEBAR_WIDTH,
    sidebarContainerWidth.value - MIN_SESSION_MAIN_WIDTH
  );
  return Math.min(MAX_SESSION_SIDEBAR_WIDTH, maxAllowed);
});

const effectiveSidebarWidthPx = computed(() => {
  if (!sidebarContainerWidth.value) {
    return DEFAULT_SESSION_SIDEBAR_WIDTH;
  }
  return clamp(
    MIN_SESSION_SIDEBAR_WIDTH,
    Math.round(sidebarWidthPx.value),
    Math.round(maxSidebarWidthByContainer.value)
  );
});

const showSidebarStatusText = computed(
  () => effectiveSidebarWidthPx.value >= SIDEBAR_STATUS_TEXT_THRESHOLD
);

const agentOptions = [
  { label: 'Codex', value: 'codex' },
  { label: 'Claude', value: 'claude' },
];

const CLAUDE_MODEL_OPTIONS = [
  { label: 'Opus', value: 'opus' },
  { label: 'Sonnet', value: 'sonnet' },
  { label: 'Haiku', value: 'haiku' },
];

const CODEX_MODEL_OPTIONS = [
  { label: 'GPT-5.3 Codex', value: 'gpt-5.3-codex' },
  { label: 'GPT-5.3 Codex Spark', value: 'gpt-5.3-codex-spark' },
  { label: 'GPT-5.4', value: 'gpt-5.4' },
  { label: 'GPT-5.4 mini', value: 'gpt-5.4-mini' },
  { label: 'GPT-5.4 nano', value: 'gpt-5.4-nano' },
  { label: 'GPT-5.4 Pro', value: 'gpt-5.4-pro' },
];

const CUSTOM_MODEL_VALUE = '__custom_model__';

function withCurrentModelOption(
  options: Array<{ label: string; value: string }>,
  currentModel?: string | null
) {
  const normalizedModel = String(currentModel || '').trim();
  if (!normalizedModel) {
    return options;
  }
  if (options.some(option => option.value === normalizedModel)) {
    return options;
  }
  return [
    ...options,
    {
      label: `${normalizedModel} (Current)`,
      value: normalizedModel,
    },
  ];
}

function defaultReasoningEffortForAgent(agent: 'claude' | 'codex') {
  return agent === 'codex' ? 'xhigh' : 'default';
}

function withCurrentReasoningEffortOption(
  options: Array<{ label: string; value: string }>,
  currentEffort?: string | null
) {
  const normalizedEffort = String(currentEffort || '')
    .trim()
    .toLowerCase();
  if (!normalizedEffort) {
    return options;
  }
  if (options.some(option => option.value === normalizedEffort)) {
    return options;
  }
  return [
    ...options,
    {
      label: `${normalizedEffort} (Current)`,
      value: normalizedEffort,
    },
  ];
}

const modelOptions = computed(() => {
  const activeModel = currentSession.value?.model ?? draftModel.value;
  if (selectedAgent.value === 'claude') {
    return [
      ...withCurrentModelOption(CLAUDE_MODEL_OPTIONS, activeModel),
      { label: t('webSession.customModel'), value: CUSTOM_MODEL_VALUE },
    ];
  }
  return [
    ...withCurrentModelOption(CODEX_MODEL_OPTIONS, activeModel),
    { label: t('webSession.customModel'), value: CUSTOM_MODEL_VALUE },
  ];
});

const reasoningEffortOptions = computed(() => {
  const options = [
    { label: t('common.default'), value: 'default' },
    { label: 'None', value: 'none' },
    { label: 'Low', value: 'low' },
    { label: 'Medium', value: 'medium' },
    { label: 'High', value: 'high' },
    { label: 'Xhigh', value: 'xhigh' },
  ];
  const activeEffort = currentSession.value?.reasoningEffort ?? draftReasoningEffort.value;
  return withCurrentReasoningEffortOption(options, activeEffort);
});

const selectedAgent = computed({
  get: () => currentSession.value?.agent ?? draftAgent.value,
  set: value => {
    const next = value as 'claude' | 'codex';
    draftAgent.value = next;
    if (next === 'claude' && draftModel.value.startsWith('gpt-')) {
      draftModel.value = 'opus';
    }
    if (next === 'codex' && !draftModel.value.startsWith('gpt-')) {
      draftModel.value = 'gpt-5.4';
    }
    draftReasoningEffort.value = defaultReasoningEffortForAgent(next);
    if (currentSession.value) {
      void webSessionStore.updateAgent(currentSession.value.id, next).catch(error => {
        message.error(error instanceof Error ? error.message : t('common.error'));
      });
    }
  },
});

const selectedModel = computed({
  get: () => currentSession.value?.model ?? draftModel.value,
  set: value => {
    const next = String(value);
    if (next === CUSTOM_MODEL_VALUE) {
      openCustomModelDialog();
      return;
    }
    draftModel.value = next;
    if (currentSession.value) {
      void webSessionStore.updateModel(currentSession.value.id, next).catch(error => {
        message.error(error instanceof Error ? error.message : t('common.error'));
      });
    }
  },
});

const selectedReasoningEffort = computed<'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh'>({
  get: () => currentSession.value?.reasoningEffort ?? draftReasoningEffort.value,
  set: value => {
    const next = value as 'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh';
    draftReasoningEffort.value = next;
    if (currentSession.value) {
      void webSessionStore.updateReasoningEffort(currentSession.value.id, next).catch(error => {
        message.error(error instanceof Error ? error.message : t('common.error'));
      });
    }
  },
});

const selectedBaseMode = computed<'default' | 'plan'>(() => {
  const session = currentSession.value;
  if (!session) {
    return draftBaseMode.value;
  }
  if (session.permissionMode === 'plan') {
    return 'plan';
  }
  if (session.permissionMode === 'default') {
    return 'default';
  }
  return lastNonYoloModeBySession.value[session.id] ?? 'default';
});

const isYoloMode = computed(() => {
  const session = currentSession.value;
  if (!session) {
    return draftYoloEnabled.value;
  }
  return session.permissionMode === 'yolo';
});

const autoApproveEnabled = computed({
  get: () => isYoloMode.value,
  set: value => {
    const next = Boolean(value);
    const session = currentSession.value;
    if (!session) {
      draftYoloEnabled.value = next;
      return;
    }
    if (next) {
      lastNonYoloModeBySession.value = {
        ...lastNonYoloModeBySession.value,
        [session.id]: selectedBaseMode.value,
      };
      void webSessionStore.updateMode(session.id, 'yolo').catch(error => {
        message.error(error instanceof Error ? error.message : t('common.error'));
      });
      return;
    }
    const fallbackMode = lastNonYoloModeBySession.value[session.id] ?? selectedBaseMode.value;
    void webSessionStore.updateMode(session.id, fallbackMode).catch(error => {
      message.error(error instanceof Error ? error.message : t('common.error'));
    });
  },
});

const refreshTabSortable = useDebounceFn(() => {
  nextTick(() => {
    setupTabSorting();
  });
}, 100);

let tabScrollContainer: HTMLElement | null = null;

function setBaseMode(mode: 'default' | 'plan') {
  draftBaseMode.value = mode;
  const session = currentSession.value;
  if (!session) {
    return;
  }
  lastNonYoloModeBySession.value = {
    ...lastNonYoloModeBySession.value,
    [session.id]: mode,
  };
  if (session.permissionMode === 'yolo') {
    return;
  }
  void webSessionStore.updateMode(session.id, mode).catch(error => {
    message.error(error instanceof Error ? error.message : t('common.error'));
  });
}

function openCustomModelDialog() {
  const inputValue = ref((currentSession.value?.model ?? draftModel.value).trim());
  dialog.create({
    title: t('webSession.customModelTitle'),
    content: () =>
      h(NInput, {
        value: inputValue.value,
        'onUpdate:value': (value: string) => {
          inputValue.value = value;
        },
        maxlength: 128,
        autofocus: true,
        placeholder: t('webSession.customModelPlaceholder'),
      }),
    positiveText: t('common.save'),
    negativeText: t('common.cancel'),
    showIcon: false,
    maskClosable: false,
    closeOnEsc: true,
    onPositiveClick: async () => {
      const nextModel = inputValue.value.trim();
      if (!nextModel) {
        message.warning(t('webSession.customModelEmpty'));
        return false;
      }
      draftModel.value = nextModel;
      if (!currentSession.value) {
        return true;
      }
      try {
        await webSessionStore.updateModel(currentSession.value.id, nextModel);
        return true;
      } catch (error) {
        message.error(error instanceof Error ? error.message : t('common.error'));
        return false;
      }
    },
  });
}

function defaultModelForAgent(agent: 'claude' | 'codex') {
  return agent === 'claude' ? 'opus' : 'gpt-5.4';
}

function formatTime(timestamp: number) {
  return new Date(timestamp).toLocaleTimeString();
}

function stringifyValue(value: unknown) {
  if (typeof value === 'string') {
    return value;
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value ?? '');
  }
}

function canPreviewAttachment(attachment: { name: string; mime?: string }) {
  const normalizedMime = attachment.mime?.trim().toLowerCase();
  if (normalizedMime) {
    return normalizedMime.startsWith('image/');
  }
  return IMAGE_ATTACHMENT_NAME_PATTERN.test(attachment.name);
}

function getAttachmentPreviewUrl(attachmentID: string) {
  const normalizedID = String(attachmentID || '').trim();
  if (!normalizedID) {
    return '';
  }
  const path = `/api/v1/web-sessions/attachments/${encodeURIComponent(normalizedID)}`;
  return urlBase ? new URL(path, urlBase).toString() : path;
}

function openAttachmentPreview(attachment: { id: string; name: string; mime?: string }) {
  if (!canPreviewAttachment(attachment)) {
    return;
  }
  activeAttachmentPreview.value = {
    id: attachment.id,
    name: attachment.name,
    url: getAttachmentPreviewUrl(attachment.id),
  };
  showAttachmentPreview.value = true;
}

function handleAttachmentPreviewVisibilityChange(show: boolean) {
  showAttachmentPreview.value = show;
  if (!show) {
    activeAttachmentPreview.value = null;
  }
}

function isToolExpanded(toolId: string) {
  return Boolean(expandedTools.value[toolId]);
}

function toggleToolExpanded(toolId: string) {
  expandedTools.value = {
    ...expandedTools.value,
    [toolId]: !expandedTools.value[toolId],
  };
}

function toolKindLabel(tool: { name: string; kind?: string }) {
  const kind = (tool.kind || '').trim();
  if (!kind) {
    return t('webSession.toolKindDefault');
  }
  if (kind === 'tool_use') {
    return t('webSession.toolKindTool');
  }
  if (kind === 'shell') {
    return 'Shell';
  }
  return kind;
}

function toolPreview(tool: { input?: unknown; output?: string }) {
  const source =
    typeof tool.output === 'string' && tool.output.trim()
      ? tool.output
      : stringifyValue(tool.input);
  return source.replace(/\s+/g, ' ').trim().slice(0, 120);
}

function toolStateLabel(tool: { status: 'running' | 'done' | 'error' }) {
  if (tool.status === 'done') {
    return t('webSession.toolDone');
  }
  if (tool.status === 'error') {
    return t('webSession.toolError');
  }
  return t('webSession.toolRunning');
}

async function initializeProjectSessions(projectId: string) {
  if (!projectId) {
    return;
  }
  const loadedSessions = await webSessionStore.loadSessions(projectId);
  await webSessionStore.openSocket();
  const targetSessionId = webSessionStore.getActiveSessionId(projectId) || loadedSessions[0]?.id;
  if (targetSessionId) {
    await webSessionStore.ensureSessionConnected(projectId, targetSessionId);
  }
}

async function handleSessionSelect(sessionId: string) {
  if (!sessionId) {
    return;
  }
  showMobileTabSelector.value = false;
  await webSessionStore.ensureSessionConnected(props.projectId, sessionId);
  scrollToBottom(true);
}

async function handleSidebarSessionSelect(item: CrossProjectSessionItem) {
  const sessionId = item.session.id;
  if (!sessionId) {
    return;
  }
  try {
    if (item.projectId === props.projectId && sessionId === activeSessionId.value) {
      scrollToBottom(true);
      return;
    }
    await webSessionStore.ensureSessionConnected(item.projectId, sessionId);
    if (item.projectId !== props.projectId) {
      projectStore.addRecentProject(item.projectId);
      await router.push({
        name: 'project',
        params: { id: item.projectId },
      });
      return;
    }
    scrollToBottom(true);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

function updateSidebarContainerWidth() {
  const parent = sidebarRootRef.value?.parentElement;
  if (!parent) {
    sidebarContainerWidth.value = 0;
    return;
  }
  sidebarContainerWidth.value = parent.getBoundingClientRect().width;
}

function setupSidebarResizeObserver() {
  sidebarResizeObserver?.disconnect();
  sidebarResizeObserver = null;
  const parent = sidebarRootRef.value?.parentElement;
  if (!parent || typeof ResizeObserver === 'undefined') {
    updateSidebarContainerWidth();
    return;
  }
  sidebarResizeObserver = new ResizeObserver(() => updateSidebarContainerWidth());
  sidebarResizeObserver.observe(parent);
  updateSidebarContainerWidth();
}

function startSidebarResize(event: MouseEvent) {
  if (!sidebarContainerWidth.value) {
    return;
  }
  event.preventDefault();
  isSidebarResizing.value = true;
  const startX = event.clientX;
  const startWidth = effectiveSidebarWidthPx.value;

  function onMouseMove(moveEvent: MouseEvent) {
    const delta = startX - moveEvent.clientX;
    sidebarWidthPx.value = Math.round(
      clamp(MIN_SESSION_SIDEBAR_WIDTH, startWidth + delta, maxSidebarWidthByContainer.value)
    );
  }

  function onMouseUp() {
    isSidebarResizing.value = false;
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }

  document.addEventListener('mousemove', onMouseMove);
  document.addEventListener('mouseup', onMouseUp);
  document.body.style.cursor = 'col-resize';
  document.body.style.userSelect = 'none';
}

async function handleCreateSession(forceAgent?: 'claude' | 'codex') {
  try {
    const agent = forceAgent ?? selectedAgent.value;
    const session = await webSessionStore.createSession(props.projectId, {
      worktreeId: projectStore.selectedWorktreeId ?? undefined,
      agent,
      model: draftModel.value || defaultModelForAgent(agent),
      reasoningEffort:
        agent === 'codex' ? selectedReasoningEffort.value : defaultReasoningEffortForAgent(agent),
      permissionMode: draftYoloEnabled.value ? 'yolo' : selectedBaseMode.value,
    });
    draftAgent.value = session.agent;
    draftModel.value = session.model;
    draftReasoningEffort.value =
      session.reasoningEffort || defaultReasoningEffortForAgent(session.agent);
    if (session.permissionMode !== 'yolo') {
      draftBaseMode.value = session.permissionMode;
    }
    draftYoloEnabled.value = session.permissionMode === 'yolo';
    scrollToBottom(true);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function handleRenameSession(sessionId: string) {
  const session = sessions.value.find(item => item.id === sessionId);
  if (!session) {
    return;
  }

  const inputValue = ref(session.title);
  dialog.create({
    title: t('webSession.renameTitle'),
    content: () =>
      h(NInput, {
        value: inputValue.value,
        'onUpdate:value': (value: string) => {
          inputValue.value = value;
        },
        maxlength: 64,
        autofocus: true,
        placeholder: t('webSession.renamePlaceholder'),
      }),
    positiveText: t('common.save'),
    negativeText: t('common.cancel'),
    showIcon: false,
    maskClosable: false,
    closeOnEsc: true,
    onPositiveClick: async () => {
      const nextTitle = inputValue.value.trim();
      if (!nextTitle) {
        message.warning(t('webSession.emptyName'));
        return false;
      }
      if (nextTitle === session.title) {
        return true;
      }
      try {
        await webSessionStore.renameSession(props.projectId, sessionId, nextTitle);
        message.success(t('webSession.renameSuccess'));
        return true;
      } catch (error) {
        message.error(error instanceof Error ? error.message : t('webSession.renameFailed'));
        return false;
      }
    },
  });
}

function handleDeleteSession(sessionId: string) {
  const session = sessions.value.find(item => item.id === sessionId);
  if (!session) {
    return;
  }

  if (confirmBeforeTerminalClose.value) {
    dialog.warning({
      title: t('webSession.confirmCloseTitle'),
      content: () =>
        h('div', { class: 'web-session-close-confirm' }, [
          h('div', { class: 'web-session-close-confirm__message' }, [
            t('webSession.confirmCloseContent', { title: session.title }),
          ]),
        ]),
      positiveText: t('webSession.confirmCloseButton'),
      negativeText: t('common.cancel'),
      onPositiveClick: async () => performDeleteSession(sessionId),
    });
    return;
  }

  void performDeleteSession(sessionId);
}

async function performDeleteSession(sessionId: string): Promise<boolean> {
  try {
    await webSessionStore.deleteSession(props.projectId, sessionId);
    const nextSession = webSessionStore.getActiveSession(props.projectId);
    if (nextSession?.id) {
      await webSessionStore.ensureSessionConnected(props.projectId, nextSession.id);
    }
    return true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
    return false;
  }
}

function openFilePicker() {
  fileInputRef.value?.click();
}

function getImageFilesFromTransfer(dataTransfer: DataTransfer | null) {
  if (!dataTransfer) {
    return [];
  }

  const imageFiles: File[] = [];
  const seen = new Set<string>();
  const register = (file: File | null) => {
    if (!file || !file.type.startsWith('image/')) {
      return;
    }
    const key = [file.name, file.type, file.size, file.lastModified].join(':');
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    imageFiles.push(file);
  };

  for (const item of Array.from(dataTransfer.items || [])) {
    if (!item.type.startsWith('image/')) {
      continue;
    }
    register(item.getAsFile());
  }

  for (const file of Array.from(dataTransfer.files || [])) {
    register(file);
  }

  return imageFiles;
}

function hasFileTransfer(dataTransfer: DataTransfer | null) {
  if (!dataTransfer) {
    return false;
  }

  if (Array.from(dataTransfer.items || []).some(item => item.kind === 'file')) {
    return true;
  }

  return (
    Array.from(dataTransfer.files || []).length > 0 ||
    Array.from(dataTransfer.types || []).includes('Files')
  );
}

function resetComposerDragState() {
  composerDragDepth = 0;
  isComposerDragOver.value = false;
}

async function uploadComposerImages(files: File[]) {
  for (const file of files) {
    try {
      await webSessionStore.uploadAttachment(props.projectId, file);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('common.error'));
    }
  }
}

async function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement | null;
  const files = Array.from(target?.files ?? []).filter(file => file.type.startsWith('image/'));
  if (files.length === 0) {
    return;
  }
  await uploadComposerImages(files);
  if (target) {
    target.value = '';
  }
}

function handleComposerPaste(event: ClipboardEvent) {
  const files = getImageFilesFromTransfer(event.clipboardData);
  if (files.length === 0) {
    return;
  }

  event.preventDefault();
  void uploadComposerImages(files);
}

function handleComposerDragEnter(event: DragEvent) {
  if (!hasFileTransfer(event.dataTransfer)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
  composerDragDepth += 1;
  isComposerDragOver.value = true;
}

function handleComposerDragOver(event: DragEvent) {
  if (!hasFileTransfer(event.dataTransfer)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy';
  }
  isComposerDragOver.value = true;
}

function handleComposerDragLeave(event: DragEvent) {
  if (!isComposerDragOver.value) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
  composerDragDepth = Math.max(0, composerDragDepth - 1);
  if (composerDragDepth === 0) {
    isComposerDragOver.value = false;
  }
}

async function handleComposerDrop(event: DragEvent) {
  if (!hasFileTransfer(event.dataTransfer)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
  const files = getImageFilesFromTransfer(event.dataTransfer);
  resetComposerDragState();
  if (files.length === 0) {
    return;
  }

  await uploadComposerImages(files);
}

function removeAttachment(attachmentId: string) {
  webSessionStore.removeDraftAttachment(props.projectId, attachmentId);
}

async function handleSubmit() {
  if (isRunActive.value || !hasDraftContent.value) {
    return;
  }
  try {
    let session = currentSession.value;
    if (!session) {
      await handleCreateSession();
      session = webSessionStore.getActiveSession(props.projectId);
    }
    if (!session) {
      return;
    }
    const attachments = draftAttachments.value;
    await webSessionStore.sendMessage(
      session.id,
      composerText.value,
      attachments.map(item => item.id)
    );
    composerText.value = '';
    webSessionStore.clearDraftAttachments(props.projectId);
    autoFollowBottom.value = true;
    scrollToBottom(true);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function handlePreinput(mode: 'redirect' | 'queue') {
  if (!currentSession.value || !hasDraftContent.value) {
    return;
  }
  try {
    const attachments = draftAttachments.value;
    await webSessionStore.sendMessage(
      currentSession.value.id,
      composerText.value,
      attachments.map(item => item.id),
      mode
    );
    composerText.value = '';
    webSessionStore.clearDraftAttachments(props.projectId);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

function handleComposerEnter(event: KeyboardEvent) {
  if (isRunActive.value) {
    if (hasDraftContent.value) {
      event.preventDefault();
      void handlePreinput('redirect');
    }
    return;
  }
  if (!hasDraftContent.value) {
    return;
  }
  event.preventDefault();
  void handleSubmit();
}

function pendingModeLabel(mode: WebSessionPendingInput['mode']) {
  return mode === 'redirect' ? t('webSession.pendingRedirect') : t('webSession.pendingQueue');
}

function pendingInputPreview(item: WebSessionPendingInput) {
  const text = item.text.trim();
  if (text) {
    return text.length > 72 ? `${text.slice(0, 72)}...` : text;
  }
  return t('webSession.pendingAttachments', { count: item.attachmentIds.length });
}

function handleRemovePendingInput(pendingId: string) {
  if (!currentSession.value) {
    return;
  }
  webSessionStore.removePendingInput(currentSession.value.id, pendingId);
}

async function handleApproval(action: 'approve' | 'reject') {
  if (!currentSession.value) {
    return;
  }
  try {
    if (action === 'approve') {
      await webSessionStore.approveSession(currentSession.value.id);
      return;
    }
    await webSessionStore.rejectSession(currentSession.value.id);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

async function handleAbortCurrent() {
  if (!currentSession.value) {
    return;
  }
  try {
    await webSessionStore.abortSession(currentSession.value.id);
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('common.error'));
  }
}

function syncScrollToBottom() {
  const container = timelineScrollRef.value;
  if (!container) {
    return;
  }
  container.scrollTop = container.scrollHeight;
  autoFollowBottom.value = true;
  showJumpToBottom.value = false;
}

function scrollToBottom(force = false) {
  if (!force && !autoFollowBottom.value) {
    return;
  }
  nextTick(() => {
    syncScrollToBottom();
  });
}

function handleLiveCardClick() {
  scrollToBottom(true);
}

function updateBottomState(container: HTMLDivElement) {
  const nearBottom = container.scrollHeight - (container.scrollTop + container.clientHeight) < 160;
  autoFollowBottom.value = nearBottom;
  showJumpToBottom.value = !nearBottom;
}

function restoreHistoryAnchor() {
  const anchor = pendingHistoryAnchor.value;
  const container = timelineScrollRef.value;
  if (!anchor || !container || currentSession.value?.id !== anchor.sessionId) {
    return false;
  }
  container.scrollTop = anchor.previousTop + (container.scrollHeight - anchor.previousHeight);
  pendingHistoryAnchor.value = null;
  updateBottomState(container);
  return true;
}

function handleTimelineScroll(event: Event) {
  const container = event.currentTarget as HTMLDivElement | null;
  if (!container) {
    return;
  }
  const nearTop = container.scrollTop < 120;
  updateBottomState(container);
  if (
    nearTop &&
    !pendingHistoryAnchor.value &&
    currentSession.value &&
    historyMeta.value.hasMore &&
    !historyMeta.value.loading
  ) {
    pendingHistoryAnchor.value = {
      sessionId: currentSession.value.id,
      previousHeight: container.scrollHeight,
      previousTop: container.scrollTop,
    };
    void webSessionStore.loadMoreHistory(currentSession.value.id).catch(error => {
      pendingHistoryAnchor.value = null;
      console.error('[Web Session] Failed to load more history', error);
    });
  }
}

function recalcTabTitleWidth(explicitWidth?: number) {
  if (typeof explicitWidth === 'number') {
    tabsContainerWidth.value = explicitWidth;
  }
  const containerWidth =
    typeof explicitWidth === 'number' ? explicitWidth : tabsContainerWidth.value;
  if (!containerWidth) {
    tabTitleMaxWidth.value = MAX_TAB_TITLE_WIDTH;
    return;
  }
  const sessionCount = Math.max(sessions.value.length, 1);
  let activeOffset = TABS_CONTAINER_STATIC_OFFSET;
  if (containerWidth - activeOffset < SHARED_WIDTH_HIDE_THRESHOLD) {
    activeOffset = TABS_CONTAINER_MIN_OFFSET;
  }
  const availableWidth = Math.max(containerWidth - activeOffset, 0);
  const rawWidth = availableWidth / sessionCount - TAB_LABEL_EXTRA_SPACE;
  tabTitleMaxWidth.value = Math.round(Math.min(MAX_TAB_TITLE_WIDTH, Math.max(56, rawWidth)));
}

function updateActiveTabIndicator() {
  nextTick(() => {
    activeTabIndicatorStyle.value =
      !isMobile.value && activeSessionId.value
        ? calculateCardTabIndicatorStyle(tabsContainerRef.value)
        : hiddenCardTabIndicatorStyle();
  });
}

function setupTabScrollListener() {
  cleanupTabScrollListener();
  nextTick(() => {
    if (isMobile.value) {
      return;
    }
    const container = tabsContainerRef.value;
    if (!container) {
      return;
    }
    const scrollContainer = container.querySelector('.v-x-scroll') as HTMLElement | null;
    if (scrollContainer) {
      tabScrollContainer = scrollContainer;
      scrollContainer.addEventListener('scroll', updateActiveTabIndicator);
    }
  });
}

function cleanupTabScrollListener() {
  if (tabScrollContainer) {
    tabScrollContainer.removeEventListener('scroll', updateActiveTabIndicator);
    tabScrollContainer = null;
  }
}

function createTabProps(session: (typeof sessions.value)[number]): HTMLAttributes {
  const isActive = activeSessionId.value === session.id;
  const theme = activeTheme.value;
  const preset = getPresetById(currentPresetId.value);
  const hideHeaderBorder = theme.terminalHeaderBorder === false;
  const props: HTMLAttributes = {
    onContextmenu: (event: MouseEvent) => handleTabContextMenu(event, session),
  };
  const classes: string[] = [];

  if (hasSessionUnviewedApproval(session)) {
    classes.push('has-unviewed-approval');
    props.style = {
      backgroundColor: approvalColors.value.bg,
      borderColor: approvalColors.value.border,
      ...(isActive && hideHeaderBorder ? { borderBottom: 'none' } : {}),
    };
  } else if (hasSessionUnviewedCompletion(session)) {
    classes.push('has-unviewed-completion');
    props.style = {
      backgroundColor: completionColors.value.bg,
      borderColor: completionColors.value.border,
      ...(isActive && hideHeaderBorder ? { borderBottom: 'none' } : {}),
    };
  } else if (isActive) {
    props.style = {
      backgroundColor:
        theme.terminalTabActiveBg || preset?.colors.terminalTabActiveBg || theme.surfaceColor,
      ...(hideHeaderBorder ? { borderBottom: 'none' } : {}),
    };
  } else {
    props.style = {
      backgroundColor: theme.terminalTabBg || preset?.colors.terminalTabBg || theme.bodyColor,
    };
  }

  if (classes.length > 0) {
    props.class = classes.join(' ');
  }
  return props;
}

function getSessionAssistantStateClass(session: (typeof sessions.value)[number]) {
  const live = webSessionStore.getLiveState(session.id);
  switch (live.phase) {
    case 'starting':
    case 'thinking':
    case 'tool':
      return 'working';
    case 'waiting_approval':
      return 'waiting_approval';
    case 'done':
    case 'idle':
      return 'waiting_input';
    default:
      return 'unknown';
  }
}

function getSessionStatusLabel(session: (typeof sessions.value)[number]) {
  switch (getSessionAssistantStateClass(session)) {
    case 'working':
      return t('terminal.aiStatusWorking');
    case 'waiting_approval':
      return t('terminal.aiStatusWaitingApproval');
    case 'waiting_input':
      return t('terminal.aiStatusWaitingInput');
    default:
      return '';
  }
}

function getSessionStatusEmoji(session: (typeof sessions.value)[number]) {
  switch (getSessionAssistantStateClass(session)) {
    case 'working':
      return '🤔';
    case 'waiting_approval':
      return '✋';
    case 'waiting_input':
      return '✓';
    default:
      return '';
  }
}

function getSessionAssistantIcon(session: (typeof sessions.value)[number]) {
  return getAssistantIconByType(session.agent === 'claude' ? 'claude-code' : 'codex');
}

function getSessionStatusTooltip(session: (typeof sessions.value)[number]) {
  const label = getSessionStatusLabel(session);
  const agentName = session.agent === 'claude' ? 'Claude Code' : 'Codex';
  return label ? `${agentName} · ${label}` : agentName;
}

function getSidebarSessionSubtitle(item: CrossProjectSessionItem) {
  if (!showSidebarStatusText.value) {
    return '';
  }
  return getSessionStatusLabel(item.session);
}

function getSidebarSessionAccentColor(item: CrossProjectSessionItem) {
  const assistantState = getSessionAssistantStateClass(item.session);
  if (hasSessionUnread(item.session) && assistantState === 'waiting_input') {
    return '#10b981';
  }
  switch (assistantState) {
    case 'working':
      return '#8b5cf6';
    case 'waiting_approval':
      return '#f79009';
    case 'waiting_input':
      return '#9ca3af';
    default:
      if (item.session.status === 'err') {
        return '#f04438';
      }
      return 'rgba(15, 23, 42, 0.08)';
  }
}

function getSidebarSessionClasses(item: CrossProjectSessionItem): string[] {
  const assistantState = getSessionAssistantStateClass(item.session);
  if (hasSessionUnread(item.session) && assistantState === 'waiting_input') {
    return ['session-sidebar-completion'];
  }
  switch (assistantState) {
    case 'working':
      return ['session-sidebar-working'];
    case 'waiting_approval':
      return ['session-sidebar-approval'];
    case 'waiting_input':
      return ['session-sidebar-idle'];
    default:
      if (item.session.status === 'err') {
        return ['session-sidebar-error'];
      }
      return [];
  }
}

function getSessionPillSizeClass() {
  const width = tabTitleMaxWidth.value;
  if (width < 60) {
    return 'pill-size-icon-only';
  }
  if (width < 90) {
    return 'pill-size-icon-emoji';
  }
  return 'pill-size-full';
}

function shouldShowSessionStatusDot(session: (typeof sessions.value)[number]) {
  return session.status === 'err';
}

function hasSessionUnviewedApproval(session: (typeof sessions.value)[number]) {
  return hasSessionUnread(session) && getSessionAssistantStateClass(session) === 'waiting_approval';
}

function hasSessionUnviewedCompletion(session: (typeof sessions.value)[number]) {
  if (!hasSessionUnread(session) || hasSessionUnviewedApproval(session)) {
    return false;
  }
  return getSessionAssistantStateClass(session) === 'waiting_input' && session.status !== 'err';
}

function handleTabContextMenu(event: MouseEvent, session: (typeof sessions.value)[number]) {
  event.preventDefault();
  event.stopPropagation();
  contextMenuSession.value = session;
  contextMenuX.value = event.clientX;
  contextMenuY.value = event.clientY;
}

async function handleContextMenuSelect(key: string | number) {
  const action = String(key);
  const session = contextMenuSession.value;
  contextMenuSession.value = null;
  if (action === 'new') {
    await handleCreateSession();
    return;
  }
  if (!session) {
    return;
  }
  if (action === 'rename') {
    await handleRenameSession(session.id);
    return;
  }
  if (action === 'delete') {
    await handleDeleteSession(session.id);
  }
}

function handleMobileTabSelect(key: string | number) {
  void handleSessionSelect(String(key));
}

function goToPrevSession() {
  if (!hasPrevSession.value) {
    return;
  }
  const session = sessions.value[currentSessionIndex.value - 1];
  if (session) {
    void handleSessionSelect(session.id);
  }
}

function goToNextSession() {
  if (!hasNextSession.value) {
    return;
  }
  const session = sessions.value[currentSessionIndex.value + 1];
  if (session) {
    void handleSessionSelect(session.id);
  }
}

function setupTabSorting() {
  if (isMobile.value) {
    destroyTabSorting();
    return;
  }
  const container = tabsContainerRef.value;
  if (!container || sessions.value.length <= 1) {
    destroyTabSorting();
    return;
  }
  const wrapper = container.querySelector('.n-tabs-wrapper') as HTMLElement | null;
  if (!wrapper) {
    destroyTabSorting();
    return;
  }
  if (tabDragSortable.value) {
    if (tabDragSortable.value.el === wrapper) {
      tabDragSortable.value.option('disabled', sessions.value.length <= 1);
      return;
    }
    destroyTabSorting();
  }
  tabDragSortable.value = Sortable.create(wrapper, {
    animation: 150,
    direction: 'horizontal',
    draggable: '.n-tabs-tab-wrapper',
    handle: '.n-tabs-tab',
    filter: '.n-tabs-tab__close',
    preventOnFilter: false,
    ghostClass: 'web-session-tab-ghost',
    chosenClass: 'web-session-tab-chosen',
    dragClass: 'web-session-tab-dragging',
    onEnd: handleTabDragEnd,
  });
  tabDragSortable.value.option('disabled', sessions.value.length <= 1);
}

function destroyTabSorting() {
  if (tabDragSortable.value) {
    tabDragSortable.value.destroy();
    tabDragSortable.value = null;
  }
}

function handleTabDragEnd(event: SortableEvent) {
  const fromIndex = event.oldDraggableIndex ?? event.oldIndex ?? -1;
  const toIndex = event.newDraggableIndex ?? event.newIndex ?? -1;
  if (fromIndex === -1 || toIndex === -1 || fromIndex === toIndex) {
    return;
  }
  void webSessionStore.moveSession(props.projectId, fromIndex, toIndex).catch(error => {
    message.error(error instanceof Error ? error.message : t('common.error'));
  });
  nextTick(() => {
    updateActiveTabIndicator();
  });
}

watch(
  () => props.projectId,
  projectId => {
    if (projectId) {
      void initializeProjectSessions(projectId);
    }
  },
  { immediate: true }
);

watch(
  sidebarProjectIdsToLoad,
  projectIds => {
    projectIds.forEach(projectId => {
      if (!projectId || loadedSidebarProjectIds.has(projectId)) {
        return;
      }
      loadedSidebarProjectIds.add(projectId);
      void webSessionStore.loadSessions(projectId).catch(error => {
        loadedSidebarProjectIds.delete(projectId);
        console.error('[Web Session] Failed to preload sidebar sessions', projectId, error);
      });
    });
  },
  { immediate: true }
);

watch(
  () => sidebarContainerWidth.value,
  () => {
    if (!showCrossProjectSidebar.value) {
      return;
    }
    sidebarWidthPx.value = clamp(
      MIN_SESSION_SIDEBAR_WIDTH,
      sidebarWidthPx.value,
      maxSidebarWidthByContainer.value
    );
  }
);

watch(
  showCrossProjectSidebar,
  visible => {
    if (!visible) {
      sidebarResizeObserver?.disconnect();
      sidebarResizeObserver = null;
      sidebarContainerWidth.value = 0;
      return;
    }
    nextTick(() => {
      setupSidebarResizeObserver();
    });
  },
  { immediate: true }
);

watch(
  () => currentSession.value?.id,
  sessionId => {
    pendingHistoryAnchor.value = null;
    if (!sessionId) {
      showMobileTabSelector.value = false;
      return;
    }
    const session = currentSession.value;
    if (!session) {
      return;
    }
    draftAgent.value = session.agent;
    draftModel.value = session.model || defaultModelForAgent(session.agent);
    draftReasoningEffort.value =
      session.reasoningEffort || defaultReasoningEffortForAgent(session.agent);
    if (session.permissionMode !== 'yolo') {
      draftBaseMode.value = session.permissionMode;
      lastNonYoloModeBySession.value = {
        ...lastNonYoloModeBySession.value,
        [session.id]: session.permissionMode,
      };
    }
    draftYoloEnabled.value = session.permissionMode === 'yolo';
    expandedTools.value = {};
    autoFollowBottom.value = true;
    scrollToBottom(true);
    updateActiveTabIndicator();
    markSessionViewed(session.id);
  },
  { immediate: true }
);

watch(
  () => props.isActive,
  active => {
    if (!active) {
      return;
    }
    markSessionViewed(currentSession.value?.id);
  },
  { immediate: true }
);

watch(currentSessionLatestEventSeq, () => {
  markSessionViewed(currentSession.value?.id);
});

watch(
  () =>
    sessions.value
      .map(
        session =>
          `${session.id}:${session.orderIndex}:${session.status}:${session.hasUnread}:${getSessionAssistantStateClass(session)}`
      )
      .join('|'),
  () => {
    nextTick(() => {
      recalcTabTitleWidth();
      updateActiveTabIndicator();
      setupTabScrollListener();
      if (isMobile.value) {
        destroyTabSorting();
      } else {
        refreshTabSortable();
      }
    });
  },
  { immediate: true }
);

watch(
  () => isMobile.value,
  mobile => {
    if (mobile) {
      showMobileTabSelector.value = false;
      cleanupTabScrollListener();
      destroyTabSorting();
      activeTabIndicatorStyle.value = hiddenCardTabIndicatorStyle();
      return;
    }
    nextTick(() => {
      setupTabScrollListener();
      refreshTabSortable();
      updateActiveTabIndicator();
    });
  },
  { immediate: true }
);

watch(timelineContentVersion, async () => {
  await nextTick();
  if (restoreHistoryAnchor()) {
    markSessionViewed(currentSession.value?.id);
    return;
  }
  const container = timelineScrollRef.value;
  if (!container) {
    return;
  }
  if (autoFollowBottom.value) {
    syncScrollToBottom();
  } else {
    updateBottomState(container);
  }
  markSessionViewed(currentSession.value?.id);
});

watch(
  () => selectedAgent.value,
  value => {
    if (!draftModel.value || (value === 'claude' && draftModel.value.startsWith('gpt-'))) {
      draftModel.value = defaultModelForAgent(value);
    }
    if (value === 'codex' && !draftModel.value.startsWith('gpt-')) {
      draftModel.value = defaultModelForAgent(value);
    }
  }
);

useResizeObserver(tabsContainerRef, entries => {
  const entry = entries[0];
  if (!entry) {
    return;
  }
  const width = entry.contentRect.width;
  if (width !== tabsContainerWidth.value) {
    recalcTabTitleWidth(width);
    updateActiveTabIndicator();
  }
});

onMounted(() => {
  if (projectStore.projects.length === 0) {
    void projectStore.fetchProjects().catch(error => {
      console.error('[Web Session] Failed to preload projects', error);
    });
  }
  nextTick(() => {
    setupSidebarResizeObserver();
    recalcTabTitleWidth();
    setupTabScrollListener();
    updateActiveTabIndicator();
    if (currentSession.value) {
      syncScrollToBottom();
    }
  });
});

onBeforeUnmount(() => {
  resetComposerDragState();
  cleanupTabScrollListener();
  destroyTabSorting();
  sidebarResizeObserver?.disconnect();
  sidebarResizeObserver = null;
});
</script>

<style scoped>
.web-session-panel {
  height: 100%;
  overflow: hidden;
}

.panel-main {
  height: 100%;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: var(--app-surface-color, var(--n-card-color, #fff));
}

.panel-body {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  overflow: hidden;
}

.panel-content {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.panel-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 12px 0;
  flex-shrink: 0;
  background-color: var(--app-surface-color, var(--n-card-color, #fff));
  color: var(--app-text-color, var(--n-text-color-1, #1f1f1f));
  border-bottom: var(--kanban-terminal-header-border, 1px solid var(--n-border-color));
  position: relative;
  z-index: 1;
}

.tabs-container {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  padding-right: 8px;
  position: relative;
}

.tabs-container :deep(.n-tabs) {
  width: 100%;
}

.tabs-container :deep(.n-tabs-pane-wrapper) {
  display: none;
}

.tabs-container :deep(.n-tab-pane) {
  padding: 0 !important;
}

.tabs-container :deep(.n-tabs-tab) {
  cursor: grab;
  user-select: none;
}

.tabs-container :deep(.n-tabs-tab:active) {
  cursor: grabbing;
}

.active-tab-indicator {
  position: absolute;
  bottom: 8px;
  left: 0;
  height: 2px;
  background-color: var(--n-primary-color);
  border-radius: 1px;
  transition:
    transform 0.3s cubic-bezier(0.4, 0, 0.2, 1),
    width 0.3s cubic-bezier(0.4, 0, 0.2, 1),
    opacity 0.3s ease;
  z-index: 2;
}

.panel-header :deep(.n-tabs) {
  --n-tab-border-color: var(--n-border-color, rgba(0, 0, 0, 0.1));
  --n-tab-text-color: var(--app-text-color, var(--n-text-color-2, #666));
  --n-tab-text-color-hover: var(--app-text-color, var(--n-text-color-1, #333));
  --n-tab-text-color-active: var(--app-text-color, var(--n-text-color-1, #333));
}

.panel-header :deep(.n-tabs .n-tabs-card-tabs) {
  background-color: transparent;
}

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab) {
  background-color: var(--kanban-terminal-tab-bg, #ffffff) !important;
  color: var(--n-tab-text-color);
  border-color: var(--n-tab-border-color);
  transition:
    background-color 0.2s ease,
    color 0.2s ease;
}

.panel-header :deep(.n-tabs .n-tabs-nav--card-type .n-tabs-tab.n-tabs-tab--active) {
  background-color: var(--kanban-terminal-tab-active-bg, #e8e8e8) !important;
  color: var(--n-tab-text-color-active);
}

.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
}

.tab-title {
  display: inline-block;
  max-width: min(160px, 20vw);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;
  background-color: var(--n-text-color-disabled, #c0c4d8);
  box-shadow: 0 0 0 1px var(--n-box-shadow-color, rgba(15, 17, 26, 0.08));
}

.status-dot.running {
  background-color: var(--kanban-terminal-status-connecting, var(--n-color-warning, #f79009));
  box-shadow: 0 0 0 1px rgba(247, 144, 9, 0.25);
}

.status-dot.done {
  background-color: var(--kanban-terminal-status-ready, var(--n-color-success, #12b76a));
  box-shadow: 0 0 0 1px rgba(18, 183, 106, 0.25);
}

.status-dot.err {
  background-color: var(--kanban-terminal-status-error, var(--n-color-error, #f04438));
  box-shadow: 0 0 0 1px rgba(240, 68, 56, 0.25);
}

.status-dot.aborting {
  background-color: var(--n-warning-color, #f59e0b);
  box-shadow: 0 0 0 1px rgba(245, 158, 11, 0.25);
}

.ai-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0 6px;
  margin-bottom: 2px;
  border-radius: 999px;
  font-size: 10px;
  line-height: 16px;
  background-color: #eef2ff;
  color: #6366f1;
  transition: all 0.2s ease;
}

.ai-status-pill.pill-size-full .ai-status-emoji {
  display: none;
}

.ai-status-pill.pill-size-icon-emoji .ai-status-text {
  display: none;
}

.ai-status-pill.pill-size-icon-emoji .ai-status-emoji {
  display: inline;
  font-size: 10px;
  line-height: 1;
}

.ai-status-pill.pill-size-icon-only .ai-status-text,
.ai-status-pill.pill-size-icon-only .ai-status-emoji {
  display: none;
}

.ai-status-pill.pill-size-icon-only {
  padding: 0 4px;
}

.ai-status-pill.state-working {
  background-color: #eadffc;
  color: #7c3aed;
}

.ai-status-pill.state-waiting_approval {
  background-color: #fed7aa;
  color: #f79009;
}

.ai-status-pill.state-waiting_input {
  background-color: #eceef2;
  color: #475467;
}

.ai-status-pill.state-unknown {
  background-color: #f1f5f9;
  color: #94a3b8;
  padding: 0 4px;
}

.ai-status-pill.state-unknown .ai-status-text,
.ai-status-pill.state-unknown .ai-status-emoji {
  display: none;
}

.ai-status-icon {
  display: inline-flex;
  align-items: center;
  line-height: 1;
}

.ai-status-icon :deep(svg) {
  display: block;
}

.ai-status-emoji {
  font-size: 10px;
  line-height: 1;
}

:deep(.n-tabs-tab.has-unviewed-completion) {
  background-color: var(--kanban-terminal-tab-completion-bg, rgba(16, 185, 129, 0.2)) !important;
  border-color: var(--kanban-terminal-tab-completion-border, rgba(16, 185, 129, 0.5)) !important;
}

:deep(.n-tabs-tab.has-unviewed-completion.n-tabs-tab--active) {
  background-color: var(
    --kanban-terminal-tab-completion-active-bg,
    rgba(16, 185, 129, 0.25)
  ) !important;
  border-color: var(
    --kanban-terminal-tab-completion-active-border,
    rgba(16, 185, 129, 0.6)
  ) !important;
}

:deep(.n-tabs-tab.has-unviewed-approval) {
  background-color: var(--kanban-terminal-tab-approval-bg, rgba(247, 144, 9, 0.2)) !important;
  border-color: var(--kanban-terminal-tab-approval-border, rgba(247, 144, 9, 0.5)) !important;
}

:deep(.n-tabs-tab.has-unviewed-approval.n-tabs-tab--active) {
  background-color: var(
    --kanban-terminal-tab-approval-active-bg,
    rgba(247, 144, 9, 0.25)
  ) !important;
  border-color: var(
    --kanban-terminal-tab-approval-active-border,
    rgba(247, 144, 9, 0.6)
  ) !important;
}

.empty-tabs-label {
  font-size: 13px;
  color: var(--n-text-color-3);
  padding-bottom: 6px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  padding-right: 4px;
  margin-left: auto;
}

.new-session-button {
  min-width: 32px;
  width: 32px;
  padding-left: 0 !important;
  padding-right: 0 !important;
}

.mobile-tab-selector {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  padding-bottom: 6px;
}

.mobile-nav-btn,
.mobile-tab-trigger {
  border: 1px solid var(--n-border-color);
  background: var(--app-surface-color, #fff);
  color: var(--app-text-color, var(--n-text-color-2, #666));
  height: 30px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition:
    background-color 0.2s ease,
    color 0.2s ease,
    border-color 0.2s ease,
    transform 0.18s ease;
}

.mobile-nav-btn {
  width: 30px;
  padding: 0;
}

.mobile-nav-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
  transform: none;
}

.mobile-tab-trigger {
  flex: 1;
  min-width: 0;
  justify-content: space-between;
  gap: 8px;
  padding: 0 12px;
}

.mobile-tab-title {
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.mobile-tab-arrow {
  transition: transform 0.2s ease;
}

.mobile-tab-arrow.is-open {
  transform: rotate(180deg);
}

.agent-select {
  width: 112px;
}

.session-sidebar-shell {
  display: flex;
  min-height: 0;
}

.session-sidebar {
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  background: var(--app-surface-color, var(--n-card-color, #fff));
  padding: 8px;
  display: flex;
  flex-direction: column;
}

.session-sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 4px 4px 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--n-primary-color) 8%, var(--n-border-color));
}

.session-sidebar-title-wrap {
  min-width: 0;
}

.session-sidebar-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.session-sidebar-subtitle {
  margin-top: 1px;
  font-size: 10px;
  color: var(--n-text-color-3);
}

.session-sidebar-count {
  min-width: 24px;
  height: 24px;
  padding: 0 6px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
  color: var(--n-primary-color);
  font-size: 11px;
  font-weight: 700;
}

.session-sidebar-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 2px 2px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.session-sidebar-empty {
  padding: 20px 12px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.session-sidebar-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 12%, var(--n-border-color));
  border-left: 4px solid var(--session-sidebar-accent, rgba(15, 23, 42, 0.08));
  background: var(--app-surface-color, #fff);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background-color 0.18s ease,
    transform 0.18s ease,
    box-shadow 0.18s ease;
}

.session-sidebar-item:hover {
  transform: none;
  box-shadow: 0 6px 16px rgba(15, 23, 42, 0.12);
}

.session-sidebar-item.is-active {
  border-color: color-mix(in srgb, var(--n-primary-color) 34%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-primary-color) 8%, var(--app-surface-color, #fff));
  box-shadow: 0 6px 16px rgba(59, 130, 246, 0.12);
}

.session-sidebar-main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
}

.session-sidebar-title-line {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.session-sidebar-agent-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 999px;
  background: transparent;
  color: var(--n-primary-color);
  flex-shrink: 0;
}

.session-sidebar-agent-icon :deep(svg) {
  display: block;
}

.session-sidebar-item-title {
  min-width: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.session-sidebar-state-text {
  flex-shrink: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  font-weight: 500;
  color: var(--n-text-color-3);
}

.session-sidebar-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.project-index-badge.session-project-badge {
  width: 18px;
  height: 18px;
  font-size: 10px;
  border-width: 1px;
}

.project-index-badge.session-project-badge.is-single-project {
  visibility: hidden;
  pointer-events: none;
}

.session-current-indicator {
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 0;
  border-radius: 50%;
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
  color: #ffffff;
  border: 1px solid rgba(59, 130, 246, 0.9);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.4);
}

.session-current-indicator.is-hidden {
  opacity: 0;
  pointer-events: none;
}

.session-current-indicator svg {
  display: block;
}

.session-sidebar-working {
  background: color-mix(in srgb, #8b5cf6 8%, var(--app-surface-color, #fff));
}

.session-sidebar-approval {
  background: color-mix(in srgb, #f79009 10%, var(--app-surface-color, #fff));
}

.session-sidebar-completion {
  background: color-mix(in srgb, #10b981 10%, var(--app-surface-color, #fff));
}

.session-sidebar-idle {
  background: color-mix(in srgb, #9ca3af 4%, var(--app-surface-color, #fff));
}

.session-sidebar-error {
  background: color-mix(in srgb, #f04438 8%, var(--app-surface-color, #fff));
}

.terminal-resizer {
  flex-shrink: 0;
  width: 6px;
  margin: 0 -3px;
  cursor: col-resize;
  position: relative;
  z-index: 1;
}

.resizer-handle {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 2px;
  height: 24px;
  border-radius: 1px;
  background-color: transparent;
  transition:
    background-color 0.15s ease,
    height 0.15s ease,
    opacity 0.15s ease;
  opacity: 0;
}

.terminal-resizer:hover .resizer-handle {
  background-color: var(--n-border-color, #d0d0d0);
  height: 40px;
  opacity: 1;
}

.terminal-resizer.is-dragging .resizer-handle {
  background-color: var(--n-primary-color, #3b82f6);
  height: 60px;
  opacity: 1;
}

.timeline-shell {
  position: relative;
  flex: 1;
  min-height: 0;
}

.timeline-scroll {
  height: 100%;
  overflow-y: auto;
  overscroll-behavior: contain;
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--n-primary-color) 10%, transparent),
      transparent 26%
    ),
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--n-primary-color) 2%, var(--app-body-color, #f7f8fa)),
      var(--app-surface-color, #fff)
    );
}

.timeline-list {
  min-height: 100%;
  padding: 24px 24px 28px;
}

.history-loading {
  display: flex;
  justify-content: center;
  padding: 4px 0 16px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.timeline-intro {
  max-width: 640px;
  padding: 16px 16px 18px;
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 14%, var(--n-border-color));
  border-radius: 12px;
  background: var(--app-surface-color, #fff);
}

.timeline-intro-badge {
  display: inline-flex;
  align-items: center;
  padding: 5px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  color: var(--n-primary-color);
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
}

.timeline-intro-title {
  margin-top: 14px;
  font-size: 18px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.timeline-intro-text {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--n-text-color-3);
}

.timeline-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 20px;
}

.timeline-item.kind-user {
  align-items: flex-end;
}

.timeline-item.kind-system {
  align-items: flex-start;
}

.item-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.timeline-item.kind-user .item-meta {
  justify-content: flex-end;
}

.item-bubble {
  max-width: min(860px, 84%);
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 10%, var(--n-border-color));
  border-radius: 12px;
  background: var(--app-surface-color, #fff);
  padding: 15px 16px;
}

.timeline-item.kind-user .item-bubble {
  background: color-mix(in srgb, var(--n-primary-color) 10%, rgba(255, 255, 255, 0.92));
  border-color: color-mix(in srgb, var(--n-primary-color) 22%, var(--n-border-color));
  border-top-right-radius: 8px;
}

.timeline-item.kind-system .item-bubble {
  max-width: min(780px, 100%);
  background: color-mix(in srgb, var(--app-surface-color, #fff) 92%, var(--n-primary-color) 8%);
  border-style: dashed;
}

.item-bubble.level-error {
  border-color: color-mix(in srgb, var(--n-error-color) 35%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-error-color) 7%, rgba(255, 255, 255, 0.9));
}

.item-bubble.level-warn {
  border-color: color-mix(in srgb, var(--n-warning-color) 35%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-warning-color) 10%, rgba(255, 255, 255, 0.92));
}

.item-text {
  min-width: 0;
}

.attachment-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.attachment-pill,
.draft-attachment-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
  font-size: 12px;
}

.attachment-preview-trigger {
  min-width: 0;
  max-width: 100%;
  display: inline-flex;
  align-items: center;
  padding: 0;
  border: none;
  background: transparent;
  color: inherit;
  font: inherit;
  cursor: zoom-in;
  transition: color 0.2s ease;
}

.attachment-preview-trigger:hover {
  color: var(--n-primary-color);
}

.attachment-preview-trigger.is-static {
  cursor: default;
}

.attachment-preview-trigger.is-static:hover {
  color: inherit;
}

.attachment-preview-trigger-text {
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.attachment-hover-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  width: min(40vw, 320px);
  min-width: 160px;
  min-height: 120px;
}

.attachment-hover-image {
  display: block;
  max-width: 100%;
  max-height: min(36vh, 240px);
  border-radius: 10px;
  object-fit: contain;
}

.attachment-preview-modal-body {
  display: flex;
  align-items: center;
  justify-content: center;
  max-height: calc(88vh - 96px);
}

.attachment-preview-modal-image {
  display: block;
  max-width: 100%;
  max-height: calc(88vh - 96px);
  border-radius: 12px;
  object-fit: contain;
}

.tool-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 14px;
}

.tool-card {
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 14%, var(--n-border-color));
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-surface-color, #fff) 94%, var(--n-primary-color) 6%);
  overflow: hidden;
}

.tool-header {
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
  padding: 12px 14px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
}

.tool-header-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.tool-header-leading {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.tool-kind {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--n-primary-color) 9%, transparent);
  color: var(--n-primary-color);
  font-size: 11px;
  font-weight: 600;
  flex-shrink: 0;
}

.tool-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tool-state-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  flex-shrink: 0;
}

.tool-state-badge.state-running {
  background: rgba(139, 92, 246, 0.12);
  color: #7c3aed;
}

.tool-state-badge.state-done {
  background: rgba(16, 185, 129, 0.12);
  color: #059669;
}

.tool-state-badge.state-error {
  background: rgba(239, 68, 68, 0.12);
  color: #dc2626;
}

.tool-state-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.tool-state-badge.state-running .tool-state-dot {
  animation: livePulse 1.4s ease-in-out infinite;
}

.tool-preview {
  font-size: 12px;
  line-height: 1.5;
  color: var(--n-text-color-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tool-body {
  padding: 0 14px 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.tool-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.tool-section-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--n-text-color-3);
  text-transform: uppercase;
}

.tool-code {
  margin: 0;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.5;
  background: color-mix(in srgb, var(--n-primary-color) 8%, transparent);
  border-radius: 8px;
  padding: 10px;
}

.runtime-strip {
  padding: 0 12px 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.live-card,
.approval-card {
  border: 1px solid var(--n-border-color);
  border-radius: 10px;
  background: var(--app-surface-color, #fff);
}

.live-card {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 12px;
  width: 100%;
  appearance: none;
  text-align: left;
  color: inherit;
  font: inherit;
  cursor: pointer;
  overflow: hidden;
  isolation: isolate;
  transition:
    border-color 0.2s ease,
    background-color 0.2s ease,
    transform 0.18s ease,
    box-shadow 0.18s ease;
}

.live-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(
    120deg,
    transparent 0%,
    rgba(255, 255, 255, 0.02) 32%,
    rgba(255, 255, 255, 0.34) 50%,
    rgba(255, 255, 255, 0.02) 68%,
    transparent 100%
  );
  transform: translateX(-130%);
  opacity: 0;
  pointer-events: none;
}

.live-card::after {
  content: '';
  position: absolute;
  left: -36%;
  bottom: 0;
  width: 36%;
  height: 3px;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(167, 139, 250, 0.18) 8%,
    rgba(139, 92, 246, 0.95) 48%,
    rgba(167, 139, 250, 0.4) 82%,
    transparent 100%
  );
  opacity: 0;
  pointer-events: none;
}

.live-card:hover {
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.12);
}

.live-card:active {
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.1);
}

.live-card:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--n-primary-color) 72%, white);
  outline-offset: 2px;
}

.live-card.phase-starting,
.live-card.phase-thinking,
.live-card.phase-tool {
  border-color: rgba(139, 92, 246, 0.24);
  background:
    linear-gradient(
      135deg,
      rgba(139, 92, 246, 0.11) 0%,
      rgba(139, 92, 246, 0.03) 52%,
      transparent 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 8px 20px rgba(139, 92, 246, 0.08);
}

.live-card.phase-starting::before,
.live-card.phase-thinking::before,
.live-card.phase-tool::before {
  opacity: 0.82;
  animation: liveSweep 1.9s linear infinite;
}

.live-card.phase-starting::after,
.live-card.phase-thinking::after,
.live-card.phase-tool::after {
  opacity: 1;
  animation: liveTrack 1.45s linear infinite;
}

.live-card.phase-waiting_approval,
.approval-card {
  border-color: rgba(247, 144, 9, 0.28);
}

.live-card.phase-waiting_approval {
  background:
    linear-gradient(
      135deg,
      rgba(247, 144, 9, 0.14) 0%,
      rgba(247, 144, 9, 0.04) 50%,
      transparent 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 8px 20px rgba(247, 144, 9, 0.08);
}

.live-card.phase-waiting_approval::before {
  opacity: 0.48;
  animation: liveSweep 3.2s ease-in-out infinite;
}

.live-card.phase-done {
  border-color: rgba(16, 185, 129, 0.24);
  background:
    linear-gradient(
      135deg,
      rgba(16, 185, 129, 0.12) 0%,
      rgba(16, 185, 129, 0.035) 48%,
      transparent 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 8px 20px rgba(16, 185, 129, 0.08);
}

.live-card.phase-done::before {
  opacity: 0.38;
  animation: liveSweep 4.2s ease-in-out infinite;
}

.live-card.phase-error {
  border-color: rgba(239, 68, 68, 0.24);
  background:
    linear-gradient(
      135deg,
      rgba(239, 68, 68, 0.11) 0%,
      rgba(239, 68, 68, 0.03) 48%,
      transparent 100%
    ),
    var(--app-surface-color, #fff);
  box-shadow: 0 8px 20px rgba(239, 68, 68, 0.08);
}

.live-card-main {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  position: relative;
  z-index: 1;
}

.live-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  position: relative;
  z-index: 1;
}

.live-activity {
  display: inline-flex;
  align-items: flex-end;
  gap: 3px;
  height: 16px;
  padding: 0 2px;
}

.live-activity-bar {
  width: 3px;
  height: 6px;
  border-radius: 999px;
  background: currentColor;
  transform-origin: center bottom;
  animation: liveBars 0.95s ease-in-out infinite;
  opacity: 0.9;
}

.live-activity-bar:nth-child(2) {
  animation-delay: 0.14s;
}

.live-activity-bar:nth-child(3) {
  animation-delay: 0.28s;
}

.live-jump-hint {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--n-primary-color) 12%, transparent);
  color: var(--n-primary-color);
  font-size: 10px;
  font-weight: 600;
  white-space: nowrap;
  opacity: 0;
  transform: translateX(6px);
  transition:
    opacity 0.18s ease,
    transform 0.18s ease,
    background-color 0.18s ease;
}

.live-jump-hint::before {
  content: '↓';
  font-size: 11px;
  line-height: 1;
}

.live-card:hover .live-jump-hint,
.live-card:focus-visible .live-jump-hint,
.live-card.show-jump-hint .live-jump-hint {
  opacity: 1;
  transform: translateX(0);
}

.live-orb {
  position: relative;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #8b5cf6;
  box-shadow: 0 0 0 5px rgba(139, 92, 246, 0.16);
  animation: livePulse 1.05s ease-in-out infinite;
  flex-shrink: 0;
}

.live-orb::after {
  content: '';
  position: absolute;
  inset: -9px;
  border-radius: 50%;
  background: rgba(139, 92, 246, 0.22);
  opacity: 0;
  animation: liveRipple 1.35s ease-out infinite;
}

.live-card.phase-waiting_approval .live-orb,
.approval-badge {
  background: #f79009;
}

.live-card.phase-waiting_approval .live-orb::after {
  background: rgba(247, 144, 9, 0.2);
}

.live-card.phase-done .live-orb {
  background: #10b981;
  box-shadow: 0 0 0 4px rgba(16, 185, 129, 0.12);
  animation: livePulse 2.8s ease-in-out infinite;
}

.live-card.phase-done .live-orb::after {
  background: rgba(16, 185, 129, 0.18);
  animation-duration: 2.6s;
}

.live-card.phase-error .live-orb {
  background: #ef4444;
  box-shadow: 0 0 0 4px rgba(239, 68, 68, 0.12);
  animation: none;
}

.live-card.phase-error .live-orb::after {
  background: rgba(239, 68, 68, 0.18);
  opacity: 0.35;
  animation: none;
}

.live-copy {
  min-width: 0;
}

.live-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
}

.live-detail {
  margin-top: 2px;
  font-size: 11px;
  line-height: 1.45;
  color: var(--n-text-color-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.live-time,
.approval-time {
  font-size: 11px;
  color: var(--n-text-color-3);
  flex-shrink: 0;
}

.approval-card {
  padding: 11px 12px;
}

.approval-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.approval-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  color: #fff;
  font-size: 11px;
  font-weight: 600;
}

.approval-prompt {
  margin-top: 8px;
  font-size: 12px;
  line-height: 1.55;
  color: var(--app-text-color, var(--n-text-color-1, #111827));
  white-space: pre-wrap;
}

.approval-actions {
  margin-top: 10px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.composer {
  border-top: 1px solid var(--n-border-color);
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.composer-shell {
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 12%, var(--n-border-color));
  border-radius: 12px;
  padding: 8px 10px 6px;
  background: var(--app-surface-color, #fff);
  transition:
    border-color 0.2s ease,
    background-color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
}

.composer-shell.is-running {
  border-color: rgba(139, 92, 246, 0.28);
}

.composer-shell.is-drag-over {
  border-color: color-mix(in srgb, var(--n-primary-color) 58%, var(--n-border-color));
  background: color-mix(in srgb, var(--n-primary-color) 5%, var(--app-surface-color, #fff));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--n-primary-color) 16%, transparent);
}

.composer-config {
  display: flex;
  align-items: center;
  width: 100%;
  margin-bottom: 4px;
  padding-bottom: 4px;
  border-bottom: 1px solid color-mix(in srgb, var(--n-border-color) 72%, transparent);
}

.composer-config-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0;
}

.composer-mode-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.composer-select {
  width: 138px;
  flex-shrink: 0;
}

.reasoning-select {
  width: 106px;
}

.composer-mode-switch {
  flex-shrink: 0;
}

.composer-mode-switch :deep(.n-button) {
  min-width: 54px;
}

.composer-yolo-btn {
  width: 64px;
  flex-shrink: 0;
}

.composer-auto-approve {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  white-space: nowrap;
  flex-shrink: 0;
}

.composer-auto-approve :deep(.n-checkbox__label) {
  font-size: 12px;
  padding-left: 4px;
}

.composer-path {
  min-width: 0;
  flex: 1;
  font-size: 10px;
  color: var(--n-text-color-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: right;
}

.composer-input {
  flex: 1;
}

.composer-input :deep(.n-input-wrapper) {
  background: transparent !important;
  box-shadow: none !important;
  padding-left: 0 !important;
  padding-right: 0 !important;
}

.composer-input :deep(.n-input__border),
.composer-input :deep(.n-input__state-border) {
  display: none !important;
}

.composer-input :deep(.n-input__textarea-el) {
  min-height: 52px !important;
  font-size: 14px;
  line-height: 1.55;
}

.composer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  margin-top: 2px;
}

.composer-footer-left,
.composer-footer-right {
  display: flex;
  align-items: center;
  gap: 6px;
}

.composer-footer-left {
  min-width: 0;
  margin-left: -2px;
}

.composer-icon-btn {
  width: 24px;
  height: 24px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--n-text-color-3);
  cursor: pointer;
  appearance: none;
  -webkit-appearance: none;
  box-shadow: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition:
    background-color 0.2s ease,
    color 0.2s ease;
}

.composer-icon-btn:hover {
  background: color-mix(in srgb, var(--n-primary-color) 10%, transparent);
  color: var(--n-primary-color);
}

.composer-hint {
  min-width: 0;
  font-size: 10px;
  line-height: 1.15;
  color: var(--n-text-color-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.composer-send-btn,
.composer-stop-btn,
.composer-queue-btn {
  min-width: 84px;
}

.draft-attachments {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 4px;
}

.pending-inputs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 4px;
}

.pending-input-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  max-width: 100%;
  padding: 4px 6px;
  border: 1px solid color-mix(in srgb, var(--n-border-color) 82%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-surface-color, #fff) 98%, var(--n-primary-color) 2%);
}

.pending-input-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 600;
  flex-shrink: 0;
}

.pending-input-badge.mode-redirect {
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
}

.pending-input-badge.mode-queue {
  background: rgba(99, 102, 241, 0.12);
  color: #4f46e5;
}

.pending-input-preview {
  min-width: 0;
  flex: 1;
  font-size: 11px;
  color: var(--n-text-color-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pending-input-remove {
  border: none;
  background: transparent;
  color: var(--n-text-color-3);
  cursor: pointer;
  font-size: 13px;
  line-height: 1;
  flex-shrink: 0;
}

.draft-attachment-remove {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
}

.hidden-file-input {
  display: none;
}

:global(.web-session-tab-ghost) {
  opacity: 0.4;
}

:global(.web-session-tab-chosen .n-tabs-tab) {
  box-shadow: 0 0 0 1px var(--n-color-primary);
}

:global(.web-session-tab-dragging .n-tabs-tab) {
  cursor: grabbing !important;
}

@media (max-width: 900px) {
  .panel-header {
    gap: 8px;
    padding-right: 8px;
  }

  .header-actions {
    gap: 6px;
  }

  .runtime-strip {
    padding: 0 10px 10px;
  }

  .item-bubble {
    max-width: 100%;
  }

  .composer-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .composer-footer-left,
  .composer-footer-right {
    width: 100%;
    justify-content: space-between;
  }

  .composer-config-row {
    flex-wrap: wrap;
  }

  .composer-path {
    width: 100%;
    text-align: left;
  }
}

@media (max-width: 640px) {
  .panel-header {
    padding: 6px 8px 0;
  }

  .header-actions {
    gap: 4px;
  }

  .header-actions :deep(.n-button) {
    padding-left: 10px;
    padding-right: 10px;
  }

  .composer-select {
    width: calc(50% - 4px);
  }

  .composer-mode-switch {
    width: auto;
  }

  .composer-mode-switch :deep(.n-button) {
    flex: 1;
  }

  .composer-yolo-btn {
    width: 64px;
  }

  .composer-mode-row {
    width: 100%;
    justify-content: space-between;
  }

  .pending-inputs {
    gap: 5px;
  }

  .composer-path {
    width: 100%;
  }

  .timeline-list {
    padding: 14px 12px 20px;
  }

  .composer {
    padding: 10px;
  }

  .runtime-strip {
    padding: 0 10px 10px;
  }

  .live-card,
  .approval-card,
  .composer-shell {
    border-radius: 10px;
  }
}

@keyframes livePulse {
  0% {
    transform: scale(1);
    opacity: 1;
  }

  50% {
    transform: scale(1.18);
    opacity: 0.72;
  }

  100% {
    transform: scale(1);
    opacity: 1;
  }
}

@keyframes liveRipple {
  0% {
    transform: scale(0.5);
    opacity: 0.56;
  }

  70% {
    opacity: 0;
  }

  100% {
    transform: scale(1.9);
    opacity: 0;
  }
}

@keyframes liveBars {
  0%,
  100% {
    transform: scaleY(0.55);
    opacity: 0.45;
  }

  50% {
    transform: scaleY(1.85);
    opacity: 1;
  }
}

@keyframes liveSweep {
  0% {
    transform: translateX(-130%);
  }

  55% {
    transform: translateX(130%);
  }

  100% {
    transform: translateX(130%);
  }
}

@keyframes liveTrack {
  0% {
    transform: translateX(0);
  }

  100% {
    transform: translateX(380%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .live-card,
  .live-jump-hint,
  .live-activity-bar,
  .live-orb,
  .live-orb::after,
  .live-card::before,
  .live-card::after,
  .tool-state-badge.state-running .tool-state-dot {
    animation: none !important;
    transition: none !important;
  }
}
</style>
