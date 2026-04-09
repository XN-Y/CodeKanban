<template>
  <div class="login-page">
    <div class="login-page__glow login-page__glow--a"></div>
    <div class="login-page__glow login-page__glow--b"></div>
    <n-card class="login-card" :bordered="false">
      <template #header>
        <div class="login-card__header">
          <div class="login-card__eyebrow">{{ t('auth.loginEyebrow') }}</div>
          <h1 class="login-card__title">{{ t('auth.loginTitle') }}</h1>
          <p class="login-card__subtitle">{{ t('auth.loginSubtitle') }}</p>
        </div>
      </template>

      <n-form @submit.prevent="handleSubmit">
        <n-form-item :label="t('auth.passwordLabel')">
          <n-input
            v-model:value="password"
            type="password"
            show-password-on="click"
            :placeholder="t('auth.passwordPlaceholder')"
            :disabled="submitting"
            @keydown.enter.prevent="handleSubmit"
          />
        </n-form-item>

        <n-space vertical size="small">
          <n-button
            block
            type="primary"
            size="large"
            :loading="submitting"
            :disabled="!password.trim()"
            @click="handleSubmit"
          >
            {{ t('auth.loginAction') }}
          </n-button>
          <div class="login-card__hint">{{ t('auth.loginHashHint') }}</div>
        </n-space>
      </n-form>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useMessage } from 'naive-ui';
import { useLocale } from '@/composables/useLocale';
import { useAuthStore } from '@/stores/auth';

const router = useRouter();
const route = useRoute();
const message = useMessage();
const authStore = useAuthStore();
const { t } = useLocale();

const password = ref('');
const submitting = ref(false);

const redirectTarget = computed(() => {
  const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/';
  return redirect.startsWith('/') ? redirect : '/';
});

async function handleSubmit() {
  if (submitting.value || !password.value.trim()) {
    return;
  }

  submitting.value = true;
  try {
    await authStore.loginWithPassword(password.value);
    password.value = '';
    await router.replace(redirectTarget.value);
  } catch (error) {
    const detail = error instanceof Error ? error.message : t('auth.loginFailed');
    message.error(detail);
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped>
.login-page {
  position: relative;
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 32px 20px;
  overflow: hidden;
  background:
    radial-gradient(circle at top left, rgba(15, 118, 110, 0.16), transparent 34%),
    radial-gradient(circle at bottom right, rgba(194, 65, 12, 0.18), transparent 30%),
    linear-gradient(
      145deg,
      color-mix(in srgb, var(--app-body-color, #f3f4f6) 82%, #ffffff) 0%,
      var(--app-body-color, #f3f4f6) 100%
    );
}

.login-page__glow {
  position: absolute;
  width: 320px;
  height: 320px;
  border-radius: 999px;
  filter: blur(56px);
  opacity: 0.34;
  pointer-events: none;
}

.login-page__glow--a {
  top: -72px;
  left: -64px;
  background: rgba(13, 148, 136, 0.24);
}

.login-page__glow--b {
  right: -48px;
  bottom: -88px;
  background: rgba(249, 115, 22, 0.22);
}

.login-card {
  position: relative;
  z-index: 1;
  width: min(100%, 440px);
  border-radius: 24px;
  background: color-mix(in srgb, var(--app-surface-color, #ffffff) 88%, rgba(255, 255, 255, 0.96));
  box-shadow: 0 24px 72px rgba(15, 23, 42, 0.16);
}

.login-card__header {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.login-card__eyebrow {
  font-size: 12px;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: color-mix(in srgb, var(--n-text-color-3) 78%, #0f766e);
}

.login-card__title {
  margin: 0;
  font-size: clamp(26px, 5vw, 34px);
  line-height: 1.05;
}

.login-card__subtitle {
  margin: 0;
  color: var(--n-text-color-2);
  line-height: 1.6;
}

.login-card__hint {
  font-size: 12px;
  line-height: 1.6;
  color: var(--n-text-color-3);
}
</style>
