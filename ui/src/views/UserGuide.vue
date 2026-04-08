<template>
  <div class="user-guide-page">
    <n-page-header @back="goBack">
      <template #title>
        <div class="title-wrapper">
          <n-icon size="24">
            <BookOutline />
          </n-icon>
          <span>{{ t('nav.guide') }}</span>
        </div>
      </template>
      <template #extra>
        <n-space align="center">
          <LanguageSwitcher />
          <n-button quaternary size="small" @click="goBack">
            {{ t('common.backToList') }}
          </n-button>
        </n-space>
      </template>
    </n-page-header>

    <component :is="guideContentComponent" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';
import { BookOutline } from '@vicons/ionicons5';
import LanguageSwitcher from '@/components/common/LanguageSwitcher.vue';
import GuideContentZhCN from '@/components/guide/GuideContentZhCN.vue';
import GuideContentEnUS from '@/components/guide/GuideContentEnUS.vue';
import { useLocale } from '@/composables/useLocale';

const { t, locale } = useLocale();
const router = useRouter();

const guideContentComponent = computed(() => {
  return locale.value === 'zh-CN' ? GuideContentZhCN : GuideContentEnUS;
});

function goBack() {
  router.push({ name: 'projects' });
}
</script>

<style scoped>
.user-guide-page {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.title-wrapper {
  display: flex;
  align-items: center;
  gap: 8px;
}

:deep(.guide-content) {
  display: flex;
  flex-direction: column;
  gap: 24px;
  margin-top: 24px;
}

:deep(.n-card__content) {
  font-size: 14px;
  line-height: 1.8;
}

:deep(.n-h3) {
  margin-top: 16px;
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

:deep(.n-h3:first-child) {
  margin-top: 0;
}

:deep(.n-ul),
:deep(.n-ol) {
  margin: 8px 0;
  padding-left: 24px;
}

:deep(.n-li) {
  margin: 4px 0;
}

:deep(.n-alert) {
  margin: 12px 0;
}

:deep(.n-steps) {
  margin-top: 16px;
}

:deep(.n-step) {
  padding-bottom: 24px;
}

:deep(.n-collapse) {
  margin-top: 16px;
}

:deep(.n-collapse-item__header) {
  font-size: 16px;
  font-weight: 500;
}
</style>
