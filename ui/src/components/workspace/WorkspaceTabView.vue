<template>
  <div class="workspace-tab-view">
    <!-- 顶部Tab栏 -->
    <div class="tab-header">
      <div class="tab-list">
        <button
          type="button"
          class="tab-item"
          :class="{ active: activeTab === 'kanban' }"
          @click="activeTab = 'kanban'"
        >
          <n-icon size="16">
            <GridOutline />
          </n-icon>
          <span class="tab-label">{{ t('nav.kanban') }}</span>
        </button>
        <button
          type="button"
          class="tab-item"
          :class="{ active: activeTab === 'terminal' }"
          @click="activeTab = 'terminal'"
        >
          <n-icon size="16">
            <TerminalOutline />
          </n-icon>
          <span class="tab-label">{{ t('nav.terminal') }}</span>
          <span v-if="terminalCount > 0" class="terminal-badge">{{ terminalCount }}</span>
        </button>
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
            <TerminalPanel :project-id="projectId" mode="docked" />
          </div>
          <DockedNotificationSidebar />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue';
import { useEventListener, useStorage } from '@vueuse/core';
import { NIcon } from 'naive-ui';
import { GridOutline, TerminalOutline } from '@vicons/ionicons5';
import { storeToRefs } from 'pinia';
import { useLocale } from '@/composables/useLocale';
import { useSettingsStore } from '@/stores/settings';
import { useTerminalStore } from '@/stores/terminal';
import KanbanBoard from '@/components/kanban/KanbanBoard.vue';
import TerminalPanel from '@/components/terminal/TerminalPanel.vue';
import DockedNotificationSidebar from '@/components/workspace/DockedNotificationSidebar.vue';

const props = defineProps<{
  projectId: string;
}>();

const { t } = useLocale();
const settingsStore = useSettingsStore();
const terminalStore = useTerminalStore();
const { terminalShortcut } = storeToRefs(settingsStore);

// 当前活跃的Tab，持久化存储
const activeTab = useStorage<'kanban' | 'terminal'>('workspace-active-tab', 'kanban');

// 终端数量
const terminalCount = computed(() => {
  return terminalStore.getTabs(props.projectId).length;
});

// 监听终端事件，如果有新终端创建或需要关注的事件，自动切换到终端Tab
watch(
  () => terminalStore.getTabs(props.projectId),
  (newTabs, oldTabs) => {
    // 如果新建了终端，自动切换到终端Tab
    if (newTabs.length > (oldTabs?.length || 0)) {
      activeTab.value = 'terminal';
    }
  }
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
  activeTab.value = activeTab.value === 'terminal' ? 'kanban' : 'terminal';
}

const handleEnsureExpandedEvent = (payload?: { projectId?: string }) => {
  if (payload?.projectId && payload.projectId !== props.projectId) {
    return;
  }
  activeTab.value = 'terminal';
};

terminalStore.emitter.on('terminal:ensure-expanded', handleEnsureExpandedEvent);
onBeforeUnmount(() => {
  terminalStore.emitter.off('terminal:ensure-expanded', handleEnsureExpandedEvent);
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

.tab-label {
  white-space: nowrap;
}

.terminal-badge {
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

.tab-content {
  flex: 1;
  overflow: hidden;
  position: relative;
}

.tab-pane {
  position: absolute;
  inset: 0;
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
