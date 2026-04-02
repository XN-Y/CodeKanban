<template>
  <n-modal
    v-model:show="visible"
    preset="dialog"
    :title="t('project.createProject')"
    :positive-text="t('common.create')"
    :negative-text="t('common.cancel')"
    :loading="loading"
    @positive-click="handleCreate"
  >
    <n-form ref="formRef" :model="formData" :rules="rules" label-placement="top">
      <n-form-item :label="t('project.projectName')" path="name">
        <n-input v-model:value="formData.name" :placeholder="t('project.namePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('project.projectDirectory')" path="path">
        <n-input-group>
          <n-input v-model:value="formData.path" :placeholder="t('project.pathPlaceholder')" />
          <n-button @click="showDirectoryPicker = true">
            <template #icon>
              <n-icon><FolderOpenOutline /></n-icon>
            </template>
          </n-button>
        </n-input-group>
        <template #feedback>
          <n-text depth="3">
            {{ t('project.pathHint') }}
          </n-text>
        </template>
      </n-form-item>
      <n-form-item :label="t('project.projectDescription')" path="description">
        <n-input
          v-model:value="formData.description"
          type="textarea"
          :rows="3"
          :placeholder="t('project.descriptionPlaceholder')"
        />
      </n-form-item>
      <n-form-item :label="t('project.hidePath')" path="hidePath">
        <n-space align="center">
          <n-switch v-model:value="formData.hidePath" />
          <n-text depth="3">{{ t('project.hidePathHint') }}</n-text>
        </n-space>
      </n-form-item>
    </n-form>
  </n-modal>

  <DirectoryPickerDialog
    v-model:show="showDirectoryPicker"
    :initial-path="pickerInitialPath"
    @confirm="handleDirectorySelected"
  />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useMessage, type FormInst, type FormRules } from 'naive-ui';
import { FolderOpenOutline } from '@vicons/ionicons5';
import { useProjectStore } from '@/stores/project';
import type { Project } from '@/types/models';
import { useLocale } from '@/composables/useLocale';
import { http } from '@/api/http';
import DirectoryPickerDialog from '@/components/common/DirectoryPickerDialog.vue';

const { t } = useLocale();

const props = defineProps<{
  show: boolean;
}>();

const emit = defineEmits<{
  'update:show': [value: boolean];
  success: [project: Project];
}>();

const projectStore = useProjectStore();
const message = useMessage();

const visible = computed({
  get: () => props.show,
  set: value => emit('update:show', value),
});

const formRef = ref<FormInst | null>(null);
const loading = ref(false);
const showDirectoryPicker = ref(false);
const homeDir = ref('');
const formData = ref({
  name: '',
  path: '',
  description: '',
  hidePath: false,
});

// 目录选择器的初始路径：优先用已填写的路径，否则用 HOME 目录
const pickerInitialPath = computed(() => formData.value.path || homeDir.value);

function handleDirectorySelected(path: string) {
  formData.value.path = path;
}

async function fetchHomeDir() {
  try {
    const res = await http.Get<{ item?: { path: string } }>('/fs/home').send();
    if (res?.item?.path) {
      homeDir.value = res.item.path;
    }
  } catch (e) {
    console.error('Failed to get home directory:', e);
  }
}

// 获取 HOME 目录
fetchHomeDir();

const rules: FormRules = {
  name: [
    { required: true, message: t('validation.projectNameRequired'), trigger: ['blur', 'input'] },
  ],
  path: [
    { required: true, message: t('validation.projectPathRequired'), trigger: ['blur', 'input'] },
  ],
};

watch(visible, newVal => {
  if (!newVal) {
    formData.value = { name: '', path: '', description: '', hidePath: false };
  }
});

async function handleCreate() {
  try {
    await formRef.value?.validate();
    loading.value = true;
    const project = await projectStore.createProject(formData.value);
    message.success(t('message.projectCreated'));
    visible.value = false;
    emit('success', project);
  } catch (error: any) {
    if (error?.message) {
      message.error(error.message);
    }
    return false;
  } finally {
    loading.value = false;
  }

  return true;
}
</script>
