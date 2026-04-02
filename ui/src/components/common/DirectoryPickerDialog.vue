<template>
  <n-modal
    v-model:show="showModal"
    preset="card"
    :title="t('common.selectDirectory')"
    style="width: 500px; max-width: 90vw"
    :mask-closable="true"
    :closable="true"
    @close="handleClose"
  >
    <DirectoryPicker ref="pickerRef" :initial-path="initialPath" @select="handleSelect" />
    <template #footer>
      <n-space justify="end">
        <n-button @click="handleClose">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" :disabled="!selectedPath" @click="handleConfirm">
          {{ t('common.confirm') }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { useLocale } from '@/composables/useLocale';
import DirectoryPicker from './DirectoryPicker.vue';

const props = defineProps<{
  initialPath?: string;
}>();

const showModal = defineModel<boolean>('show', { default: false });

const emit = defineEmits<{
  (e: 'confirm', path: string): void;
}>();

const { t } = useLocale();

const pickerRef = ref<InstanceType<typeof DirectoryPicker> | null>(null);
const selectedPath = ref('');

function handleSelect(path: string) {
  console.log('[DirectoryPickerDialog] handleSelect:', path);
  selectedPath.value = path;
}

function handleConfirm() {
  console.log('[DirectoryPickerDialog] handleConfirm:', selectedPath.value);
  if (selectedPath.value) {
    emit('confirm', selectedPath.value);
    showModal.value = false;
  }
}

function handleClose() {
  showModal.value = false;
}

watch(showModal, show => {
  if (show) {
    // 打开时重新加载目录
    if (props.initialPath) {
      selectedPath.value = props.initialPath;
      pickerRef.value?.loadDirectory(props.initialPath);
    }
  } else {
    selectedPath.value = '';
  }
});
</script>
