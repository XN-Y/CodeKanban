import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { extractItem } from '@/api/response';
import { http } from '@/api/http';
import { deriveClientHash } from '@/utils/auth';

type AuthStatus = {
  enabled: boolean;
  authenticated: boolean;
  frontendSalt: string;
  frontendPBKDF2Rounds: number;
  sessionTtlSeconds: number;
};

type ItemResponse<T> = {
  item?: T;
};

type MessageResponse = {
  message?: string;
};

export const useAuthStore = defineStore('auth', () => {
  const ready = ref(false);
  const loading = ref(false);
  const enabled = ref(false);
  const authenticated = ref(false);
  const frontendSalt = ref('');
  const frontendPBKDF2Rounds = ref(20000);
  const sessionTtlSeconds = ref(30 * 24 * 60 * 60);
  const pendingLoad = ref<Promise<void> | null>(null);

  const canAccessProtectedContent = computed(() => !enabled.value || authenticated.value);

  function applyStatus(status?: AuthStatus) {
    enabled.value = status?.enabled ?? false;
    authenticated.value = status?.authenticated ?? false;
    frontendSalt.value = status?.frontendSalt ?? '';
    frontendPBKDF2Rounds.value = status?.frontendPBKDF2Rounds ?? 20000;
    sessionTtlSeconds.value = status?.sessionTtlSeconds ?? 30 * 24 * 60 * 60;
    ready.value = true;
  }

  async function fetchStatus() {
    const response = await http.Get<ItemResponse<AuthStatus>>('/auth/status').send(true);
    const status = extractItem<AuthStatus>(response);
    applyStatus(status);
  }

  async function ensureLoaded(force = false) {
    if (ready.value && !force) {
      return;
    }
    if (pendingLoad.value && !force) {
      return pendingLoad.value;
    }

    loading.value = true;
    const task = fetchStatus().finally(() => {
      loading.value = false;
      pendingLoad.value = null;
    });
    pendingLoad.value = task;
    return task;
  }

  async function refreshStatus() {
    await ensureLoaded(true);
  }

  async function loginWithPassword(passwordText: string) {
    const clientHash = await deriveClientHash(
      passwordText,
      frontendSalt.value,
      frontendPBKDF2Rounds.value
    );
    await http.Post<MessageResponse>('/auth/login', { clientHash }).send();
    await refreshStatus();
  }

  async function enablePasswordProtection(passwordText: string) {
    const clientHash = await deriveClientHash(
      passwordText,
      frontendSalt.value,
      frontendPBKDF2Rounds.value
    );
    await http.Post<MessageResponse>('/auth/password/enable', { clientHash }).send();
    await refreshStatus();
  }

  async function changePasswordProtection(currentPasswordText: string, newPasswordText: string) {
    const [currentClientHash, newClientHash] = await Promise.all([
      deriveClientHash(currentPasswordText, frontendSalt.value, frontendPBKDF2Rounds.value),
      deriveClientHash(newPasswordText, frontendSalt.value, frontendPBKDF2Rounds.value),
    ]);
    await http
      .Post<MessageResponse>('/auth/password/change', {
        currentClientHash,
        newClientHash,
      })
      .send();
    await refreshStatus();
  }

  async function disablePasswordProtection(passwordText: string) {
    const clientHash = await deriveClientHash(
      passwordText,
      frontendSalt.value,
      frontendPBKDF2Rounds.value
    );
    await http.Post<MessageResponse>('/auth/password/disable', { clientHash }).send();
    await refreshStatus();
  }

  async function logout() {
    await http.Post<MessageResponse>('/auth/logout').send();
    authenticated.value = false;
    await refreshStatus();
  }

  function markUnauthorized() {
    if (!enabled.value) {
      return;
    }
    authenticated.value = false;
    ready.value = true;
  }

  if (typeof window !== 'undefined') {
    window.localStorage.removeItem('token');
  }

  return {
    ready,
    loading,
    enabled,
    authenticated,
    frontendSalt,
    frontendPBKDF2Rounds,
    sessionTtlSeconds,
    canAccessProtectedContent,
    applyStatus,
    ensureLoaded,
    refreshStatus,
    loginWithPassword,
    enablePasswordProtection,
    changePasswordProtection,
    disablePasswordProtection,
    logout,
    markUnauthorized,
  };
});
