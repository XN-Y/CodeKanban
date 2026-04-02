<template>
  <n-modal
    v-model:show="visible"
    preset="dialog"
    :title="t('project.editProject')"
    :positive-text="t('common.save')"
    :negative-text="t('common.cancel')"
    :loading="loading"
    @positive-click="handleUpdate"
  >
    <n-form ref="formRef" :model="formData" :rules="rules" label-placement="top">
      <n-form-item :label="t('project.projectName')" path="name">
        <n-input v-model:value="formData.name" :placeholder="t('project.namePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('project.projectDescription')" path="description">
        <n-input
          v-model:value="formData.description"
          type="textarea"
          :rows="3"
          :placeholder="t('project.descriptionPlaceholder')"
        />
      </n-form-item>
      <n-form-item :label="t('project.projectPath')">
        <n-input :value="props.project?.path ?? ''" disabled />
      </n-form-item>
      <n-form-item :label="t('project.hidePath')" path="hidePath">
        <n-space align="center">
          <n-switch v-model:value="formData.hidePath" />
          <n-text depth="3">{{ t('project.hidePathHint') }}</n-text>
        </n-space>
      </n-form-item>
    </n-form>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useMessage, type FormInst, type FormRules } from 'naive-ui';
import { useProjectStore } from '@/stores/project';
import type { Project } from '@/types/models';
import { useLocale } from '@/composables/useLocale';

const { t } = useLocale();

const props = defineProps<{
  show: boolean;
  project: Project | null;
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
const formData = ref({
  name: '',
  description: '',
  hidePath: false,
});

const rules: FormRules = {
  name: [
    { required: true, message: t('validation.projectNameRequired'), trigger: ['blur', 'input'] },
  ],
};

function syncFormWithProject(project: Project | null) {
  if (!project) {
    formData.value = { name: '', description: '', hidePath: false };
    return;
  }
  formData.value = {
    name: project.name,
    description: project.description ?? '',
    hidePath: project.hidePath ?? false,
  };
}

watch(
  () => props.project,
  project => {
    if (visible.value) {
      syncFormWithProject(project);
    }
  },
  { immediate: true }
);

watch(visible, value => {
  if (value) {
    syncFormWithProject(props.project);
  }
});

async function handleUpdate() {
  if (!props.project) {
    message.warning(t('project.selectProjectToEdit'));
    return false;
  }
  try {
    await formRef.value?.validate();
    loading.value = true;
    const project = await projectStore.updateProject(props.project.id, {
      name: formData.value.name.trim(),
      description: formData.value.description.trim(),
      hidePath: formData.value.hidePath,
    });
    message.success(t('message.projectUpdated'));
    emit('success', project);
    visible.value = false;
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
