<template>
  <n-dropdown :options="languageOptions" @select="handleSelect">
    <n-button quaternary :title="currentLanguageLabel" :aria-label="currentLanguageLabel">
      <template #icon>
        <n-icon><LanguageOutline /></n-icon>
      </template>
      <span v-if="!compact">{{ currentLanguageLabel }}</span>
    </n-button>
  </n-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { LanguageOutline } from '@vicons/ionicons5';
import { useLocale, type LocaleType } from '@/composables/useLocale';

withDefaults(
  defineProps<{
    compact?: boolean;
  }>(),
  {
    compact: false,
  }
);

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
