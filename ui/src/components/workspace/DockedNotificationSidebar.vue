<template>
  <div ref="rootRef" class="docked-notification-sidebar">
    <!-- 可拖动分隔条 -->
    <div class="terminal-resizer" :class="{ 'is-dragging': isResizing }" @mousedown="startResize">
      <div class="resizer-handle"></div>
    </div>
    <div
      class="terminal-notifications"
      :style="{ width: effectiveSidebarWidthPx + 'px', flex: 'none' }"
    >
      <AINotificationBar layout="docked-sidebar" compact-mode="force-compact">
        <template #toolbar-extra>
          <n-tooltip placement="bottom" :delay="250">
            <template #trigger>
              <button type="button" class="docked-reset-btn" @click="resetWidth">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M21 12a9 9 0 1 1-3-6.7"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                  />
                  <path
                    d="M21 3v6h-6"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  />
                </svg>
              </button>
            </template>
            {{ t('terminal.resetNotificationWidth') }}
          </n-tooltip>
        </template>
      </AINotificationBar>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useStorage } from '@vueuse/core';
import { useLocale } from '@/composables/useLocale';
import AINotificationBar from '@/components/terminal/AINotificationBar.vue';

const MIN_NOTIFICATION_WIDTH = 220;
const MAX_NOTIFICATION_WIDTH = 400;
const DEFAULT_NOTIFICATION_WIDTH = 240;
const MIN_TERMINAL_MAIN_WIDTH = 420;

const { t } = useLocale();

const LEGACY_PX_STORAGE_KEY = 'workspace-notification-width';
const notificationWidthPx = useStorage<number>(LEGACY_PX_STORAGE_KEY, DEFAULT_NOTIFICATION_WIDTH);

const rootRef = ref<HTMLElement | null>(null);
const containerWidth = ref(0);
const isResizing = ref(false);

function clamp(min: number, value: number, max: number) {
  return Math.max(min, Math.min(max, value));
}

function updateContainerWidth() {
  const parent = rootRef.value?.parentElement;
  if (!parent) {
    containerWidth.value = 0;
    return;
  }
  containerWidth.value = parent.getBoundingClientRect().width;
}

let resizeObserver: ResizeObserver | null = null;
onMounted(() => {
  const parent = rootRef.value?.parentElement;
  if (parent && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => updateContainerWidth());
    resizeObserver.observe(parent);
  }
  updateContainerWidth();
});

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  resizeObserver = null;
});

const maxWidthByContainer = computed(() => {
  if (!containerWidth.value) {
    return MAX_NOTIFICATION_WIDTH;
  }
  const reservedForMain = MIN_TERMINAL_MAIN_WIDTH;
  const maxAllowed = Math.max(MIN_NOTIFICATION_WIDTH, containerWidth.value - reservedForMain);
  return Math.min(MAX_NOTIFICATION_WIDTH, maxAllowed);
});

const effectiveSidebarWidthPx = computed(() => {
  if (!containerWidth.value) {
    return MIN_NOTIFICATION_WIDTH;
  }
  const maxWidth = maxWidthByContainer.value;
  return clamp(MIN_NOTIFICATION_WIDTH, Math.round(notificationWidthPx.value), Math.round(maxWidth));
});

function resetWidth() {
  notificationWidthPx.value = DEFAULT_NOTIFICATION_WIDTH;
}

function startResize(e: MouseEvent) {
  if (!containerWidth.value) {
    return;
  }
  e.preventDefault();
  isResizing.value = true;
  const startX = e.clientX;
  const startWidth = effectiveSidebarWidthPx.value;

  function onMouseMove(moveEvent: MouseEvent) {
    const delta = startX - moveEvent.clientX;
    const maxWidth = maxWidthByContainer.value;
    const newWidthPx = clamp(MIN_NOTIFICATION_WIDTH, startWidth + delta, maxWidth);
    notificationWidthPx.value = Math.round(newWidthPx);
  }

  function onMouseUp() {
    isResizing.value = false;
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

watch(
  () => containerWidth.value,
  () => {
    const maxWidth = maxWidthByContainer.value;
    notificationWidthPx.value = clamp(MIN_NOTIFICATION_WIDTH, notificationWidthPx.value, maxWidth);
  }
);
</script>

<style scoped>
.docked-notification-sidebar {
  display: flex;
  min-height: 0;
}

.terminal-notifications {
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  background: var(--app-surface-color, var(--n-card-color, #fff));
  padding: 8px;
  display: flex;
  flex-direction: column;
}

/* 可拖动分隔条样式 - 极简隐蔽设计 */
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

.docked-reset-btn {
  width: 36px;
  height: 32px;
  border-radius: 6px;
  border: 1px solid var(--kanban-notification-button-border, rgba(0, 0, 0, 0.2));
  background: var(--app-surface-color, var(--body-color, #ffffff));
  box-shadow: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--kanban-notification-button-fg, var(--text-color, #000000));
  transition: all 0.2s ease;
  opacity: 0.85;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  padding: 0;
}

.docked-reset-btn:hover {
  opacity: 1;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.docked-reset-btn:active {
  transform: scale(0.96);
}

.docked-reset-btn svg {
  display: block;
}
</style>
