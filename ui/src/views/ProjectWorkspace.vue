<template>
  <div class="project-workspace" :class="{ 'is-mobile': isMobileLayout }">
    <!-- 桌面端布局 -->
    <template v-if="!isMobileLayout">
      <n-layout has-sider>
        <!-- 左侧最近项目侧边栏 -->
        <n-layout-sider bordered :width="240" :min-width="200" :max-width="400" resizable>
          <RecentProjects
            :current-project-id="currentProjectId"
            @edit-current="openProjectEditDialog"
            @toggle-terminal="toggleTerminalPanel"
          />
        </n-layout-sider>

        <n-layout has-sider>
          <!-- 右侧工作树侧边栏 -->
          <n-layout-sider
            v-model:collapsed="worktreeSiderCollapsed"
            bordered
            :width="320"
            :collapsed-width="0"
            show-trigger="arrow-circle"
          >
            <WorktreeList @open-terminal="handleOpenTerminal" />
          </n-layout-sider>

          <n-layout-content content-style="height: 100vh;">
            <!-- 主内容区 -->
            <!-- Dock 模式：使用 Tab 视图切换看板和终端 -->
            <WorkspaceTabView v-if="isDockMode" :project-id="currentProjectId" />
            <!-- 浮动模式：只显示看板 -->
            <div v-else class="workspace-content">
              <KanbanBoard :project-id="currentProjectId" />
            </div>
          </n-layout-content>
        </n-layout>
      </n-layout>
    </template>

    <!-- 移动端布局 -->
    <template v-else>
      <div class="mobile-workspace">
        <!-- 看板视图 -->
        <div v-show="mobileActiveView === 'kanban'" class="mobile-view mobile-kanban-view">
          <KanbanBoard :project-id="currentProjectId" />
        </div>

        <!-- 终端视图占位（实际终端由 TerminalPanel 控制） -->
        <div v-show="mobileActiveView === 'terminal'" class="mobile-view mobile-terminal-view">
          <!-- 终端面板会覆盖此区域 -->
        </div>

        <!-- 项目视图 -->
        <div v-show="mobileActiveView === 'projects'" class="mobile-view mobile-projects-view">
          <RecentProjects
            :current-project-id="currentProjectId"
            :is-mobile="true"
            @edit-current="openProjectEditDialog"
            @toggle-terminal="() => setMobileView('terminal')"
          />
        </div>

        <!-- 提醒视图 -->
        <div
          v-show="mobileActiveView === 'notifications'"
          class="mobile-view mobile-notifications-view"
        >
          <AINotificationBar :is-mobile="true" />
        </div>

        <!-- 移动端底部导航 -->
        <div class="mobile-bottom-nav safe-area-bottom">
          <button
            type="button"
            class="nav-item"
            :class="{ active: mobileActiveView === 'projects' }"
            @click="setMobileView('projects')"
          >
            <n-icon size="20">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
                <path
                  fill="currentColor"
                  d="M10 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"
                />
              </svg>
            </n-icon>
            <span>{{ t('nav.projects') }}</span>
          </button>
          <button
            type="button"
            class="nav-item"
            :class="{ active: mobileActiveView === 'kanban' }"
            @click="setMobileView('kanban')"
          >
            <n-icon size="20">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
                <path
                  fill="currentColor"
                  d="M4 4h4v4H4V4zm0 6h4v4H4v-4zm0 6h4v4H4v-4zm6-12h4v4h-4V4zm0 6h4v4h-4v-4zm0 6h4v4h-4v-4zm6-12h4v4h-4V4zm0 6h4v4h-4v-4z"
                />
              </svg>
            </n-icon>
            <span>{{ t('nav.kanban') }}</span>
          </button>
          <button
            type="button"
            class="nav-item"
            :class="{ active: mobileActiveView === 'terminal' }"
            @click="setMobileView('terminal')"
          >
            <n-icon size="20">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
                <path
                  fill="currentColor"
                  d="M20 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2zM7.293 15.707L5.586 14l1.707-1.707 1.414 1.414L7.293 15.707zm6.121-4.293l-1.414 1.414-1.414-1.414L11.879 10l1.535 1.414z"
                />
              </svg>
            </n-icon>
            <span>{{ t('nav.terminal') }}</span>
          </button>
          <button
            type="button"
            class="nav-item"
            :class="{ active: mobileActiveView === 'notifications' }"
            @click="setMobileView('notifications')"
          >
            <n-icon size="20">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
                <path
                  fill="currentColor"
                  d="M12 22c1.1 0 2-.9 2-2h-4c0 1.1.9 2 2 2zm6-6v-5c0-3.07-1.63-5.64-4.5-6.32V4c0-.83-.67-1.5-1.5-1.5s-1.5.67-1.5 1.5v.68C7.64 5.36 6 7.92 6 11v5l-2 2v1h16v-1l-2-2z"
                />
              </svg>
            </n-icon>
            <span>{{ t('nav.notifications') }}</span>
          </button>
        </div>
      </div>
    </template>

    <!-- 悬浮终端面板：仅在浮动模式下显示 -->
    <TerminalPanel
      v-if="!isDockMode"
      ref="terminalPanelRef"
      :project-id="currentProjectId"
      :is-mobile="isMobileLayout"
      :hidden="isMobileLayout && mobileActiveView !== 'terminal'"
    />
    <ProjectEditDialog
      v-model:show="showEditDialog"
      :project="projectStore.currentProject"
      @success="handleProjectUpdated"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, provide, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useTitle } from '@vueuse/core';
import { storeToRefs } from 'pinia';
import { useMessage } from 'naive-ui';
import { useProjectStore } from '@/stores/project';
import { useSettingsStore } from '@/stores/settings';
import { useTerminalStore } from '@/stores/terminal';
import { useResponsive } from '@/composables/useResponsive';
import { useLocale } from '@/composables/useLocale';
import WorktreeList from '@/components/worktree/WorktreeList.vue';
import KanbanBoard from '@/components/kanban/KanbanBoard.vue';
import RecentProjects from '@/components/project/RecentProjects.vue';
import TerminalPanel from '@/components/terminal/TerminalPanel.vue';
import WorkspaceTabView from '@/components/workspace/WorkspaceTabView.vue';
import ProjectEditDialog from '@/components/project/ProjectEditDialog.vue';
import AINotificationBar from '@/components/terminal/AINotificationBar.vue';
import type { Worktree } from '@/types/models';
import { APP_NAME } from '@/constants/app';

const WORKSPACE_MOBILE_MAX_WIDTH = 900;

const route = useRoute();
const router = useRouter();
const message = useMessage();
const projectStore = useProjectStore();
const settingsStore = useSettingsStore();
const terminalStore = useTerminalStore();
const { terminalDisplayMode } = storeToRefs(settingsStore);
const { windowWidth } = useResponsive();
const { t } = useLocale();
const terminalPanelRef = ref<InstanceType<typeof TerminalPanel> | null>(null);
const showEditDialog = ref(false);

const isMobileLayout = computed(() => windowWidth.value <= WORKSPACE_MOBILE_MAX_WIDTH);

const WORKTREE_SIDER_COLLAPSED_KEY = 'worktree-sider-collapsed';
const getInitialWorktreeSiderCollapsedState = (): boolean => {
  const stored = localStorage.getItem(WORKTREE_SIDER_COLLAPSED_KEY);
  return stored ? Boolean(JSON.parse(stored)) : true;
};
const worktreeSiderCollapsed = ref(getInitialWorktreeSiderCollapsedState());
watch(worktreeSiderCollapsed, collapsed => {
  localStorage.setItem(WORKTREE_SIDER_COLLAPSED_KEY, JSON.stringify(collapsed));
});

// Dock 模式：终端固定在中央区域，与看板形成 Tab 切换
const isDockMode = computed(
  () => !isMobileLayout.value && terminalDisplayMode.value === 'docked'
);

// 移动端视图切换
type MobileView = 'kanban' | 'terminal' | 'projects' | 'notifications';
const mobileActiveView = ref<MobileView>('kanban');

// 提供终端面板引用给子组件
provide('terminalPanelRef', terminalPanelRef);

const currentProjectId = computed(() =>
  typeof route.params.id === 'string' ? route.params.id : ''
);

const pageTitle = computed(() => {
  const projectName = projectStore.currentProject?.name;
  return projectName ? `${projectName} - ${APP_NAME}` : APP_NAME;
});

useTitle(pageTitle);

const loadProject = (id: string) => {
  if (!id) {
    return;
  }
  projectStore.fetchProject(id);
  projectStore.addRecentProject(id);
};

onMounted(() => {
  if (currentProjectId.value) {
    loadProject(currentProjectId.value);
  }
});

watch(
  () => route.params.id,
  newId => {
    if (typeof newId === 'string') {
      loadProject(newId);
    }
  }
);

// 监听路由变化，当从分支管理等页面返回到项目工作区时刷新 worktrees
watch(
  () => route.name,
  (newName, oldName) => {
    // 当从分支管理页面返回到项目工作区时，重新加载 worktrees
    // 这样可以确保在分支管理页面创建的新 worktree 能够立即显示
    if (newName === 'project' && oldName === 'project-branches' && currentProjectId.value) {
      projectStore.fetchWorktrees(currentProjectId.value);
    }
  }
);

watch(
  currentProjectId,
  newId => {
    if (!newId) {
      return;
    }
    nextTick(() => {
      void terminalPanelRef.value?.reloadSessions();
    });
  },
  { immediate: true }
);

function handleOpenTerminal(worktree: Worktree) {
  // Floating mode: delegate to TerminalPanel for expand/focus behavior.
  if (terminalPanelRef.value) {
    terminalPanelRef.value.createTerminal({
      worktreeId: worktree.id,
      workingDir: worktree.path,
      title: worktree.branchName,
    });
    return;
  }

  // Dock mode: TerminalPanel isn't mounted, so create the session via the store.
  // WorkspaceTabView will auto-switch to the terminal tab when a new session appears.
  if (!currentProjectId.value) {
    return;
  }
  terminalStore
    .createSession(currentProjectId.value, {
      worktreeId: worktree.id,
      workingDir: worktree.path,
      title: worktree.branchName,
    })
    .catch((error: any) => {
      message.error(error?.message ?? t('terminal.createFailed'));
    });
}

function openProjectEditDialog() {
  showEditDialog.value = true;
}

async function handleProjectUpdated() {
  if (currentProjectId.value) {
    await projectStore.fetchProject(currentProjectId.value);
  }
}

function toggleTerminalPanel() {
  terminalPanelRef.value?.toggleExpanded();
}

// 移动端视图切换
function setMobileView(view: MobileView) {
  mobileActiveView.value = view;
}

function goToProjectList() {
  router.push('/');
}
</script>

<style scoped>
.project-workspace {
  height: 100vh;
}

.workspace-content {
  padding: 24px;
  height: 100vh;
  overflow-y: auto;
  background-color: var(--app-surface-color, #ffffff);
}

/* 移动端布局 */
.project-workspace.is-mobile {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.mobile-workspace {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.mobile-view {
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

.mobile-kanban-view {
  padding-bottom: 60px; /* 为底部导航留出空间 */
}

.mobile-projects-view {
  padding: 16px;
  padding-bottom: 76px;
}

.mobile-notifications-view {
  padding: 16px;
  padding-bottom: 76px;
}

/* 移动端底部导航 */
.mobile-bottom-nav {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-around;
  background-color: var(--app-surface-color, #ffffff);
  border-top: 1px solid var(--n-border-color, #e0e0e0);
  z-index: 200;
}

.mobile-bottom-nav .nav-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 8px 16px;
  border: none;
  background: transparent;
  color: var(--n-text-color-3, #999);
  font-size: 12px;
  cursor: pointer;
  transition: color 0.2s;
  min-width: 64px;
}

.mobile-bottom-nav .nav-item.active {
  color: var(--n-primary-color, #18a058);
}

.mobile-bottom-nav .nav-item:active {
  opacity: 0.7;
}

/* 平板端适配 */
@media (min-width: 768px) and (max-width: 1023px) {
  .workspace-content {
    padding: 16px;
  }
}
</style>
