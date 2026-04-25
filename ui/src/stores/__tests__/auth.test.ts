import { createPinia, setActivePinia, storeToRefs } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { getMethodMock, getSendMock, postMethodMock, postSendMock } = vi.hoisted(() => {
  const getSendMock = vi.fn();
  const postSendMock = vi.fn();
  return {
    getMethodMock: vi.fn(() => ({ send: getSendMock })),
    getSendMock,
    postMethodMock: vi.fn(() => ({ send: postSendMock })),
    postSendMock,
  };
});

vi.mock('@/api/http', () => ({
  http: {
    Get: getMethodMock,
    Post: postMethodMock,
  },
}));

import { useAuthStore } from '@/stores/auth';

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getMethodMock.mockClear();
    getSendMock.mockReset();
    postMethodMock.mockClear();
    postSendMock.mockReset();
  });

  it('treats bypassed status as protected-content access but not security management', async () => {
    getSendMock.mockResolvedValue({
      item: {
        enabled: true,
        authenticated: false,
        bypassed: true,
        frontendSalt: 'salt',
        frontendPBKDF2Rounds: 20000,
        sessionTtlSeconds: 3600,
      },
    });

    const store = useAuthStore();
    const { bypassed, canAccessProtectedContent, canManageSecurity } = storeToRefs(store);

    await store.ensureLoaded();

    expect(bypassed.value).toBe(true);
    expect(canAccessProtectedContent.value).toBe(true);
    expect(canManageSecurity.value).toBe(false);
  });

  it('allows authenticated sessions to manage security settings', () => {
    const store = useAuthStore();
    store.applyStatus({
      enabled: true,
      authenticated: true,
      bypassed: false,
      frontendSalt: 'salt',
      frontendPBKDF2Rounds: 20000,
      sessionTtlSeconds: 3600,
    });

    expect(store.canManageSecurity).toBe(true);
  });

  it('clears bypass state when unauthorized is reported', () => {
    const store = useAuthStore();
    store.applyStatus({
      enabled: true,
      authenticated: false,
      bypassed: true,
      frontendSalt: 'salt',
      frontendPBKDF2Rounds: 20000,
      sessionTtlSeconds: 3600,
    });

    store.markUnauthorized();

    expect(store.bypassed).toBe(false);
    expect(store.canAccessProtectedContent).toBe(false);
  });

  it('loads and saves auth access config through the auth endpoints', async () => {
    const store = useAuthStore();
    getSendMock.mockResolvedValueOnce({
      item: {
        accessRules: {
          bypassIPs: ['127.0.0.1'],
          bypassDomains: ['localhost'],
          forceAuthIPs: [],
          forceAuthDomains: ['admin.example.com'],
        },
        proxyHeader: 'X-Forwarded-For',
        trustedProxies: ['10.0.0.0/24'],
      },
    });
    postSendMock.mockResolvedValueOnce({
      item: {
        accessRules: {
          bypassIPs: ['127.0.0.1'],
          bypassDomains: ['localhost'],
          forceAuthIPs: ['203.0.113.9'],
          forceAuthDomains: ['admin.example.com'],
        },
        proxyHeader: 'X-Forwarded-For',
        trustedProxies: ['10.0.0.0/24'],
      },
    });

    const loaded = await store.fetchAccessConfig();
    const saved = await store.updateAccessConfig({
      accessRules: {
        ...loaded.accessRules,
        forceAuthIPs: ['203.0.113.9'],
      },
      proxyHeader: loaded.proxyHeader,
      trustedProxies: loaded.trustedProxies,
    });

    expect(getMethodMock).toHaveBeenCalledWith('/auth/access-config');
    expect(postMethodMock).toHaveBeenCalledWith('/auth/access-config', {
      accessRules: {
        bypassIPs: ['127.0.0.1'],
        bypassDomains: ['localhost'],
        forceAuthIPs: ['203.0.113.9'],
        forceAuthDomains: ['admin.example.com'],
      },
      proxyHeader: 'X-Forwarded-For',
      trustedProxies: ['10.0.0.0/24'],
    });
    expect(saved.accessRules.forceAuthIPs).toEqual(['203.0.113.9']);
  });
});
