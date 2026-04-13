<template>
  <div class="workspace-tab-view">
    <!-- 顶部Tab栏 -->
    <div class="tab-header">
      <div class="tab-list">
        <button
          type="button"
          class="tab-item"
          :class="{ active: activeTab === 'terminal' }"
          @click="activateTab('terminal')"
        >
          <n-icon size="16">
            <TerminalOutline />
          </n-icon>
          <span class="tab-label">{{ t('nav.terminal') }}</span>
          <span v-if="terminalCount > 0" class="tab-badge">{{ terminalCount }}</span>
        </button>
        <button
          type="button"
          class="tab-item"
          :class="{ active: activeTab === 'web' }"
          @click="activateTab('web')"
        >
          <n-icon size="16">
            <ChatbubblesOutline />
          </n-icon>
          <span class="tab-label">{{ t('nav.webSession') }}</span>
          <span class="tab-badge session-summary-badge">
            {{ webSessionSummaryText }}
          </span>
        </button>
        <button
          type="button"
          class="tab-item"
          :class="{ active: activeTab === 'files' }"
          @click="activateTab('files')"
        >
          <n-icon size="16">
            <FolderOpenOutline />
          </n-icon>
          <span class="tab-label">{{ t('nav.files') }}</span>
        </button>
        <button
          type="button"
          class="tab-item"
          :class="{ active: activeTab === 'kanban' }"
          @click="activateTab('kanban')"
        >
          <n-icon size="16">
            <GridOutline />
          </n-icon>
          <span class="tab-label">{{ t('nav.kanban') }}</span>
        </button>
      </div>
      <div v-if="activeTab === 'terminal' || activeTab === 'web'" class="tab-actions">
        <n-tooltip placement="bottom" :delay="250">
          <template #trigger>
            <button
              type="button"
              class="header-action-btn"
              :aria-label="rightSidebarToggleLabel"
              :aria-pressed="isRightSidebarVisible"
              @click="toggleRightSidebar"
            >
              <svg
                v-if="isRightSidebarVisible"
                class="sidebar-toggle-icon"
                viewBox="0 0 20 20"
                fill="none"
                aria-hidden="true"
              >
                <rect
                  x="2.75"
                  y="3.25"
                  width="14.5"
                  height="13.5"
                  rx="2.25"
                  stroke="currentColor"
                  stroke-width="1.5"
                />
                <path
                  d="M12.25 4v12"
                  stroke="currentColor"
                  stroke-width="1.5"
                  stroke-linecap="round"
                />
                <path
                  d="M14 8.25L15.75 10L14 11.75"
                  stroke="currentColor"
                  stroke-width="1.5"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              <svg
                v-else
                class="sidebar-toggle-icon"
                viewBox="0 0 20 20"
                fill="none"
                aria-hidden="true"
              >
                <rect
                  x="2.75"
                  y="3.25"
                  width="14.5"
                  height="13.5"
                  rx="2.25"
                  stroke="currentColor"
                  stroke-width="1.5"
                />
                <path
                  d="M12.25 4v12"
                  stroke="currentColor"
                  stroke-width="1.5"
                  stroke-linecap="round"
                  stroke-dasharray="1.5 2"
                  opacity="0.5"
                />
                <path
                  d="M15.75 8.25L14 10L15.75 11.75"
                  stroke="currentColor"
                  stroke-width="1.5"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
            </button>
          </template>
          {{ rightSidebarToggleLabel }}
        </n-tooltip>
      </div>
    </div>

    <!-- Tab内容 -->
    <div class="tab-content">
      <div v-show="activeTab === 'kanban'" class="tab-pane kanban-pane">
        <KanbanBoard :project-id="projectId" />
      </div>
      <div v-show="activeTab === 'terminal'" class="tab-pane terminal-pane">
        <div class="terminal-split">
          <div class="terminal-main">
            <TerminalPanel :project-id="projectId" />
          </div>
          <DockedNotificationSidebar v-if="isRightSidebarVisible" />
        </div>
      </div>
      <div v-show="activeTab === 'web'" class="tab-pane web-pane">
        <div class="terminal-split">
          <div class="terminal-main web-main">
            <WebSessionPanel
              :project-id="projectId"
              :show-sidebar="isRightSidebarVisible"
              :is-active="activeTab === 'web'"
            />
          </div>
        </div>
      </div>
      <div v-show="activeTab === 'files'" class="tab-pane files-pane">
        <FileManagerPanel :project-id="projectId" :is-active="activeTab === 'files'" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { useEventListener, useStorage } from '@vueuse/core';
import { NIcon } from 'naive-ui';
import {
  ChatbubblesOutline,
  FolderOpenOutline,
  GridOutline,
  TerminalOutline,
} from '@vicons/ionicons5';
import { storeToRefs } from 'pinia';
import { useRoute, useRouter } from 'vue-router';
import {
  formatAiStatusTripletWithTotal,
  summarizeWebSessions,
} from '@/composables/useAiStatusSummary';
import { useLocale } from '@/composables/useLocale';
import { useSettingsStore } from '@/stores/settings';
import { useTerminalStore } from '@/stores/terminal';
import { useWebSessionStore } from '@/stores/webSession';
import FileManagerPanel from '@/components/files/FileManagerPanel.vue';
import KanbanBoard from '@/components/kanban/KanbanBoard.vue';
import TerminalPanel from '@/components/terminal/TerminalPanel.vue';
import DockedNotificationSidebar from '@/components/workspace/DockedNotificationSidebar.vue';
import WebSessionPanel from '@/components/web-session/WebSessionPanel.vue';
import {
  buildWorkspaceRouteQuery,
  isWorkspaceRouteTabQuerySynced,
  resolveDesktopWorkspaceRouteTab,
  type DesktopWorkspaceRouteTab,
} from '@/utils/workspaceRoute';

const props = defineProps<{
  projectId: string;
}>();

type WorkspaceTab = DesktopWorkspaceRouteTab;

const WORKSPACE_ACTIVE_TAB_STORAGE_KEY = 'workspace-active-tab';

const { t } = useLocale();
const route = useRoute();
const router = useRouter();
const settingsStore = useSettingsStore();
const terminalStore = useTerminalStore();
const webSessionStore = useWebSessionStore();
const { terminalShortcut } = storeToRefs(settingsStore);

function normalizeWorkspaceTab(value: unknown): WorkspaceTab {
  return resolveDesktopWorkspaceRouteTab(null, value);
}

const storedActiveTab = useStorage<WorkspaceTab>(WORKSPACE_ACTIVE_TAB_STORAGE_KEY, 'terminal');
const activeTab = ref<WorkspaceTab>(normalizeWorkspaceTab(storedActiveTab.value));
const previousTab = ref<WorkspaceTab | null>(null);
const isRightSidebarVisible = useStorage('workspace-right-sidebar-visible', true);

function syncWorkspaceRouteTab(tab: WorkspaceTab) {
  if (isWorkspaceRouteTabQuerySynced(route.query, tab)) {
    return;
  }
  void router.replace({
    query: buildWorkspaceRouteQuery(route.query, tab),
  });
}

watch(
  [() => route.query, storedActiveTab],
  ([query, storedTab]) => {
    const nextTab = resolveDesktopWorkspaceRouteTab(query, storedTab);
    if (storedActiveTab.value !== nextTab) {
      storedActiveTab.value = nextTab;
    }
    if (activeTab.value !== nextTab) {
      previousTab.value = activeTab.value;
      activeTab.value = nextTab;
    }
    syncWorkspaceRouteTab(nextTab);
  },
  { immediate: true }
);

function activateTab(nextTab: WorkspaceTab) {
  const normalized = normalizeWorkspaceTab(nextTab);
  if (storedActiveTab.value !== normalized) {
    storedActiveTab.value = normalized;
  }
  if (normalized === activeTab.value) {
    syncWorkspaceRouteTab(normalized);
    return;
  }
  previousTab.value = activeTab.value;
  activeTab.value = normalized;
  syncWorkspaceRouteTab(normalized);
}

function togglePreviousWorkspaceTab() {
  const previous = previousTab.value ? normalizeWorkspaceTab(previousTab.value) : null;
  if (!previous || previous === activeTab.value) {
    return;
  }
  previousTab.value = activeTab.value;
  activeTab.value = previous;
  storedActiveTab.value = previous;
  syncWorkspaceRouteTab(previous);
}

// 终端数量
const terminalCount = computed(() => {
  return terminalStore.getTabs(props.projectId).length;
});

const webSessionSummary = computed(() =>
  summarizeWebSessions(webSessionStore.getSessions(props.projectId), sessionId =>
    webSessionStore.getLiveState(sessionId)
  )
);
const webSessionSummaryText = computed(() =>
  formatAiStatusTripletWithTotal(
    webSessionSummary.value,
    webSessionStore.getSessions(props.projectId).length
  )
);

const rightSidebarToggleLabel = computed(() =>
  t(isRightSidebarVisible.value ? 'webSession.hideSidebar' : 'webSession.showSidebar')
);

function isToggleShortcut(event: KeyboardEvent) {
  if (event.metaKey || event.ctrlKey || event.altKey) {
    return false;
  }
  return event.code === terminalShortcut.value.code;
}

function isTerminalElement(element: HTMLElement | null) {
  if (!element) {
    return false;
  }
  return Boolean(element.closest('.terminal-shell'));
}

function isEditableElement(element: HTMLElement | null) {
  if (!element) {
    return false;
  }
  if (element.isContentEditable) {
    return true;
  }
  const tagName = element.tagName;
  if (tagName === 'INPUT' || tagName === 'TEXTAREA') {
    const input = element as HTMLInputElement | HTMLTextAreaElement;
    return !input.readOnly && !input.disabled;
  }
  return false;
}

function handleDockedTerminalToggleShortcut(event: KeyboardEvent) {
  if (event.defaultPrevented) {
    return;
  }
  if (event.repeat || !isToggleShortcut(event)) {
    return;
  }
  const activeElement = (
    typeof document !== 'undefined' ? document.activeElement : null
  ) as HTMLElement | null;
  if (isTerminalElement(activeElement) || isEditableElement(activeElement)) {
    return;
  }
  event.preventDefault();
  togglePreviousWorkspaceTab();
}

function toggleRightSidebar() {
  isRightSidebarVisible.value = !isRightSidebarVisible.value;
}

const handleEnsureExpandedEvent = (payload?: { projectId?: string }) => {
  if (payload?.projectId && payload.projectId !== props.projectId) {
    return;
  }
  activateTab('terminal');
};

const handleTerminalCreatedEvent = (payload?: { projectId?: string }) => {
  if (payload?.projectId && payload.projectId !== props.projectId) {
    return;
  }
  activateTab('terminal');
};

const handleWebSessionCreatedEvent = (payload?: { projectId?: string }) => {
  if (payload?.projectId && payload.projectId !== props.projectId) {
    return;
  }
  activateTab('web');
};

terminalStore.emitter.on('terminal:ensure-expanded', handleEnsureExpandedEvent);
terminalStore.emitter.on('terminal:created', handleTerminalCreatedEvent);
webSessionStore.emitter.on('web-session:created', handleWebSessionCreatedEvent);
onBeforeUnmount(() => {
  terminalStore.emitter.off('terminal:ensure-expanded', handleEnsureExpandedEvent);
  terminalStore.emitter.off('terminal:created', handleTerminalCreatedEvent);
  webSessionStore.emitter.off('web-session:created', handleWebSessionCreatedEvent);
});

if (typeof window !== 'undefined') {
  useEventListener(window, 'keydown', handleDockedTerminalToggleShortcut);
}
</script>

<style scoped>
.workspace-tab-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.tab-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  height: 40px;
  border-bottom: 1px solid var(--n-border-color, var(--app-input-border-color, rgba(0, 0, 0, 0.12)));
  background-color: var(--app-surface-color, var(--n-card-color, #ffffff));
  flex-shrink: 0;
}

.tab-list {
  display: flex;
  gap: 4px;
}

.tab-actions {
  display: flex;
  align-items: center;
}

.tab-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--n-text-color-2);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.tab-item:hover {
  background-color: var(--n-color-hover);
  color: var(--n-text-color);
}

.tab-item.active {
  background-color: var(--n-color-target);
  color: var(--n-primary-color);
  font-weight: 500;
}

.tab-item:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px var(--n-primary-color);
}

.tab-label {
  white-space: nowrap;
}

.tab-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 9px;
  background-color: var(--n-primary-color);
  color: #fff;
  font-size: 11px;
  font-weight: 500;
}

.session-summary-badge {
  min-width: auto;
  padding: 0 7px;
  font-variant-numeric: tabular-nums;
}

.header-action-btn {
  width: 30px;
  height: 30px;
  border: none;
  border-radius: 8px;
  background-color: transparent;
  color: var(--n-text-color-2);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  opacity: 0.82;
  transition:
    color 0.2s ease,
    background-color 0.2s ease,
    opacity 0.2s ease,
    box-shadow 0.2s ease;
}

.header-action-btn:hover {
  color: var(--n-text-color);
  background-color: var(--n-color-hover);
  opacity: 1;
}

.header-action-btn[aria-pressed='true'] {
  color: var(--n-text-color);
  background-color: transparent;
  opacity: 0.94;
}

.header-action-btn[aria-pressed='true']:hover {
  color: var(--n-primary-color);
}

.sidebar-toggle-icon {
  width: 18px;
  height: 18px;
  display: block;
}

.header-action-btn:focus-visible {
  outline: 2px solid var(--n-primary-color);
  outline-offset: 2px;
}

.tab-content {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
}

.tab-pane {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.kanban-pane {
  padding: 24px;
  overflow-y: auto;
  background-color: var(--app-surface-color, #ffffff);
}

.terminal-pane {
  display: flex;
  flex-direction: column;
}

.web-pane {
  display: flex;
  flex-direction: column;
}

.files-pane {
  background-color: var(--app-surface-color, #ffffff);
}

.web-main {
  background: linear-gradient(180deg, rgba(246, 241, 232, 0.78), rgba(255, 255, 255, 0.94));
}

.terminal-split {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: 12px;
  padding: 12px;
}

.terminal-main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
}
</style>
