<template>
  <n-modal
    :show="show"
    preset="card"
    class="daily-tip-dialog-modal"
    :title="t('dailyTip.title')"
    :mask-closable="false"
    :auto-focus="false"
    style="width: min(560px, calc(100vw - 32px))"
    @update:show="handleUpdateShow"
  >
    <div class="daily-tip-dialog">
      <div class="daily-tip-dialog__body">
        <span class="daily-tip-dialog__counter">
          {{ t('dailyTip.tipCounter', { current: tipIndex + 1, total: totalTips }) }}
        </span>
        <h3 class="daily-tip-dialog__headline">{{ tip.title }}</h3>
        <div class="daily-tip-dialog__description-wrap">
          <p class="daily-tip-dialog__description">{{ tip.description }}</p>
        </div>
        <p class="daily-tip-dialog__hint">{{ t('dailyTip.oncePerDayHint') }}</p>
      </div>
      <div class="daily-tip-dialog__actions">
        <n-button quaternary :disabled="totalTips <= 1" @click="emit('next')">
          {{ t('dailyTip.showAnother') }}
        </n-button>
        <n-space justify="end">
          <n-button quaternary @click="emit('disable')">
            {{ t('dailyTip.disableForever') }}
          </n-button>
          <n-button type="primary" @click="emit('acknowledge')">
            {{ t('dailyTip.acknowledge') }}
          </n-button>
        </n-space>
      </div>
    </div>
  </n-modal>
</template>

<script setup lang="ts">
import { useLocale } from '@/composables/useLocale';
import type { DailyTipDefinition } from '@/utils/dailyTips';

defineProps<{
  show: boolean;
  tip: DailyTipDefinition;
  tipIndex: number;
  totalTips: number;
}>();

const emit = defineEmits<{
  'update:show': [value: boolean];
  next: [];
  acknowledge: [];
  disable: [];
}>();

const { t } = useLocale();

function handleUpdateShow(value: boolean) {
  emit('update:show', value);
}
</script>

<style scoped>
.daily-tip-dialog {
  display: flex;
  flex-direction: column;
  gap: 16px;
  height: min(320px, calc(100vh - 220px));
  min-height: 0;
}

.daily-tip-dialog__body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
}

.daily-tip-dialog__counter {
  display: inline-flex;
  align-self: flex-start;
  padding: 4px 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--n-primary-color, #18a058) 14%, transparent);
  color: var(--n-primary-color, #18a058);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.2;
}

.daily-tip-dialog__headline {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  line-height: 1.35;
  color: var(--app-text-color, var(--n-text-color-1, #1f1f1f));
}

.daily-tip-dialog__description {
  margin: 0;
  font-size: 14px;
  line-height: 1.75;
  color: var(--app-text-color, var(--n-text-color-1, #1f1f1f));
}

.daily-tip-dialog__description-wrap {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 4px;
}

.daily-tip-dialog__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--n-text-color-3, #8c8c8c);
}

.daily-tip-dialog__actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
}
</style>
