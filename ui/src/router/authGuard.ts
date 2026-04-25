export interface AuthGuardRoute {
  name?: string | symbol | null;
  fullPath?: string;
  query?: Record<string, unknown>;
}

export interface AuthGuardStore {
  ensureLoaded(force?: boolean): Promise<void> | void;
  enabled: boolean;
  authenticated: boolean;
  bypassed: boolean;
  canAccessProtectedContent: boolean;
}

export async function resolveAuthNavigation(to: AuthGuardRoute, authStore: AuthGuardStore) {
  await authStore.ensureLoaded();

  if (!authStore.enabled) {
    if (to.name === 'login') {
      return { name: 'projects' };
    }
    return true;
  }

  if (!authStore.canAccessProtectedContent && to.name !== 'login') {
    const redirect = to.fullPath && to.fullPath !== '/login' ? to.fullPath : '/';
    return {
      name: 'login',
      query: { redirect },
    };
  }

  if (authStore.canAccessProtectedContent && to.name === 'login') {
    const redirect =
      typeof to.query?.redirect === 'string' && to.query.redirect.startsWith('/')
        ? to.query.redirect
        : '/';
    return redirect;
  }

  return true;
}
