<template>
  <div class="project-status-popover" :class="{ 'is-compact': compact }">
    <div class="project-status-popover-title">{{ t('project.aiStatusSummary') }}</div>
    <div class="project-status-row">
      <span>{{ t('project.aiStatusWorking') }}</span>
      <n-tag :size="tagSize" :bordered="false">{{ summary.working }}</n-tag>
    </div>
    <div class="project-status-row">
      <span>{{ t('project.aiStatusBlocking') }}</span>
      <n-tag :size="tagSize" :bordered="false" type="warning">{{ summary.blocking }}</n-tag>
    </div>
    <div class="project-status-row">
      <span>{{ t('project.aiStatusUnreadCompleted') }}</span>
      <n-tag :size="tagSize" :bordered="false" type="success">{{ summary.unreadCompleted }}</n-tag>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useAiStatusSummary } from '@/composables/useAiStatusSummary';
import { useLocale } from '@/composables/useLocale';

const props = defineProps<{
  projectId: string;
  compact?: boolean;
}>();

const { t } = useLocale();
const { getProjectSummary } = useAiStatusSummary();

const summary = computed(() => getProjectSummary(props.projectId));
const compact = computed(() => props.compact === true);
const tagSize = computed(() => (compact.value ? 'tiny' : 'small'));
</script>

<style scoped>
.project-status-popover {
  min-width: 180px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.project-status-popover-title {
  font-size: 13px;
  font-weight: 600;
}

.project-status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  font-size: 13px;
}

.project-status-popover.is-compact {
  min-width: 148px;
  gap: 8px;
}

.project-status-popover.is-compact .project-status-popover-title {
  font-size: 12px;
}

.project-status-popover.is-compact .project-status-row {
  gap: 12px;
  font-size: 12px;
}
</style>
