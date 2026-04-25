import { describe, expect, it, vi } from 'vitest';

import { resolveAuthNavigation, type AuthGuardStore } from '@/router/authGuard';

function createStore(overrides: Partial<AuthGuardStore> = {}): AuthGuardStore {
  return {
    ensureLoaded: vi.fn().mockResolvedValue(undefined),
    enabled: true,
    authenticated: false,
    bypassed: false,
    canAccessProtectedContent: false,
    ...overrides,
  };
}

describe('resolveAuthNavigation', () => {
  it('redirects unauthenticated protected navigation to login', async () => {
    const store = createStore();

    const result = await resolveAuthNavigation(
      {
        name: 'settings',
        fullPath: '/settings?section=security',
        query: {},
      },
      store
    );

    expect(result).toEqual({
      name: 'login',
      query: { redirect: '/settings?section=security' },
    });
  });

  it('allows bypassed requests to protected routes', async () => {
    const store = createStore({
      bypassed: true,
      canAccessProtectedContent: true,
    });

    const result = await resolveAuthNavigation(
      {
        name: 'settings',
        fullPath: '/settings',
        query: {},
      },
      store
    );

    expect(result).toBe(true);
  });

  it('redirects protected users away from the login page', async () => {
    const store = createStore({
      authenticated: true,
      canAccessProtectedContent: true,
    });

    const result = await resolveAuthNavigation(
      {
        name: 'login',
        fullPath: '/login',
        query: { redirect: '/project/abc' },
      },
      store
    );

    expect(result).toBe('/project/abc');
  });
});
