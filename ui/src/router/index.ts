import { createRouter, createWebHashHistory } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { pinia } from '@/stores/pinia';

const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
    },
    {
      path: '/',
      name: 'projects',
      component: () => import('@/views/ProjectList.vue'),
    },
    {
      path: '/project/:id',
      name: 'project',
      component: () => import('@/views/ProjectWorkspace.vue'),
    },
    {
      path: '/project/:id/branches',
      name: 'project-branches',
      component: () => import('@/views/BranchManagement.vue'),
    },
    {
      path: '/guide',
      name: 'guide',
      component: () => import('@/views/UserGuide.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/GeneralSettings.vue'),
    },
  ],
});

router.beforeEach(async to => {
  const authStore = useAuthStore(pinia);
  await authStore.ensureLoaded();

  if (!authStore.enabled) {
    if (to.name === 'login') {
      return { name: 'projects' };
    }
    return true;
  }

  if (!authStore.authenticated && to.name !== 'login') {
    const redirect = to.fullPath && to.fullPath !== '/login' ? to.fullPath : '/';
    return {
      name: 'login',
      query: { redirect },
    };
  }

  if (authStore.authenticated && to.name === 'login') {
    const redirect =
      typeof to.query.redirect === 'string' && to.query.redirect.startsWith('/')
        ? to.query.redirect
        : '/';
    return redirect;
  }

  return true;
});

export default router;
