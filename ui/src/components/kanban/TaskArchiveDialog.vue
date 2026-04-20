<template>
  <n-modal
    preset="card"
    class="task-archive-dialog"
    :title="t('task.archiveDialogTitle')"
    :show="show"
    @update:show="emit('update:show', $event as boolean)"
    :style="dialogStyle"
    :card-style="dialogCardStyle"
  >
    <n-space vertical size="large">
      <n-text depth="3">{{ t('task.archiveDialogHint') }}</n-text>

      <n-space justify="space-between" align="center">
        <n-checkbox
          :checked="allSelected"
          :indeterminate="indeterminate"
          :disabled="tasks.length === 0"
          @update:checked="toggleSelectAll"
        >
          {{ t('task.archiveSelectAll') }}
        </n-checkbox>

        <n-button size="small" :disabled="!clipboardSupported" @click="handleCopy">
          <template #icon>
            <n-icon>
              <CopyOutline />
            </n-icon>
          </template>
          {{ t('task.archiveCopy') }}
        </n-button>
      </n-space>

      <n-input
        type="textarea"
        :value="summaryText"
        readonly
        :autosize="{ minRows: 4, maxRows: 10 }"
      />

      <n-empty v-if="tasks.length === 0" :description="t('task.archiveEmptyHint')" />

      <n-checkbox-group v-else v-model:value="selectedIds">
        <n-space vertical size="small">
          <n-checkbox v-for="task in tasks" :key="task.id" :value="task.id">
            <n-space size="small" align="center">
              <span>{{ task.title }}</span>
              <n-tag v-if="task.branchName" size="tiny" :bordered="false">{{
                task.branchName
              }}</n-tag>
            </n-space>
          </n-checkbox>
        </n-space>
      </n-checkbox-group>
    </n-space>

    <template #footer>
      <n-space justify="end">
        <n-button @click="emit('update:show', false)">{{ t('common.cancel') }}</n-button>
        <n-button
          type="primary"
          :loading="archiving"
          :disabled="selectedIds.length === 0"
          @click="handleArchive"
        >
          {{ t('task.archiveAction') }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, ref, watch, type CSSProperties } from 'vue';
import { CopyOutline } from '@vicons/ionicons5';
import { useMessage } from 'naive-ui';
import { useAppClipboard } from '@/composables/useAppClipboard';
import { taskActions } from '@/composables/useTaskActions';
import { extractItem } from '@/api/response';
import type { Task } from '@/types/models';
import { useLocale } from '@/composables/useLocale';

const { t } = useLocale();
const message = useMessage();
const { moveTask } = taskActions;

const props = defineProps<{
  show: boolean;
  tasks: Task[];
}>();

const emit = defineEmits<{
  'update:show': [boolean];
  archived: [Task[]];
}>();

const selectedIds = ref<string[]>([]);
const archiving = ref(false);

watch(
  () => props.show,
  value => {
    if (value) {
      selectedIds.value = [];
    }
  }
);

watch(
  () => props.tasks,
  tasks => {
    const available = new Set(tasks.map(task => task.id));
    selectedIds.value = selectedIds.value.filter(id => available.has(id));
  },
  { deep: true }
);

const summaryText = computed(() => {
  if (!props.tasks.length) {
    return '';
  }
  const lines = props.tasks.map((task, index) => {
    const suffix = task.branchName ? ` (${task.branchName})` : '';
    return `${index + 1}. ${task.title}${suffix}`;
  });
  return lines.join('\n');
});

const { copyText, isSupported: clipboardSupported } = useAppClipboard();

const allSelected = computed(
  () => props.tasks.length > 0 && selectedIds.value.length === props.tasks.length
);
const indeterminate = computed(
  () => selectedIds.value.length > 0 && selectedIds.value.length < props.tasks.length
);

function toggleSelectAll(checked: boolean) {
  selectedIds.value = checked ? props.tasks.map(task => task.id) : [];
}

async function handleCopy() {
  await copyText(summaryText.value, {
    failureMessage: t('task.archiveCopyFailed'),
    successMessage: t('task.archiveCopied'),
    unsupportedMessage: t('task.copyNotSupported'),
  });
}

async function handleArchive() {
  if (archiving.value) {
    return;
  }
  if (selectedIds.value.length === 0) {
    message.warning(t('task.archiveSelectRequired'));
    return;
  }

  archiving.value = true;

  const succeeded: Task[] = [];
  const failedIds: string[] = [];

  try {
    for (const id of selectedIds.value) {
      try {
        const response = await moveTask.send(id, { status: 'archived' });
        const updated = extractItem(response) as unknown as Task | undefined;
        if (updated) {
          succeeded.push(updated);
        }
      } catch {
        failedIds.push(id);
      }
    }
  } finally {
    archiving.value = false;
  }

  if (succeeded.length > 0) {
    emit('archived', succeeded);
    message.success(t('task.archiveSuccess', { count: succeeded.length }));
  }

  if (failedIds.length > 0) {
    selectedIds.value = [...failedIds];
    message.error(t('task.archivePartialFailed', { count: failedIds.length }));
    return;
  }

  emit('update:show', false);
}

const dialogStyle: CSSProperties = {
  width: 'min(90vw, 720px)',
  maxWidth: '720px',
};

const dialogCardStyle: CSSProperties = {
  backgroundColor: 'transparent',
  boxShadow: 'none',
};
</script>
