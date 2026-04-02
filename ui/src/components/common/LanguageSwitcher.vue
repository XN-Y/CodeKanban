<template>
  <n-dropdown :options="languageOptions" @select="handleSelect">
    <n-button quaternary>
      <template #icon>
        <n-icon><LanguageOutline /></n-icon>
      </template>
      {{ currentLanguageLabel }}
    </n-button>
  </n-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { LanguageOutline } from '@vicons/ionicons5';
import { useLocale, type LocaleType } from '@/composables/useLocale';

const { locale, setLocale } = useLocale();

const languageOptions = [
  {
    label: '简体中文',
    key: 'zh-CN',
  },
  {
    label: 'English',
    key: 'en-US',
  },
];

const currentLanguageLabel = computed(() => {
  return languageOptions.find(item => item.key === locale.value)?.label || '简体中文';
});

const handleSelect = (key: string) => {
  setLocale(key as LocaleType);
};
</script>
