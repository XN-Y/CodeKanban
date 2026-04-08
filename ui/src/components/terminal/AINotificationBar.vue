<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { storeToRefs } from 'pinia';
import type { DropdownOption } from 'naive-ui';
import { useTerminalStore } from '@/stores/terminal';
import {
  useTerminalReminderStore,
  type TerminalApprovalRecord,
  type TerminalCompletionRecord,
} from '@/stores/terminalReminder';
import { useTerminalSessionSnapshotStore } from '@/stores/terminalSessionSnapshot';
import { useProjectStore } from '@/stores/project';
import { useSettingsStore } from '@/stores/settings';
import { getAssistantIconByType, getAssistantColorByType } from '@/utils/assistantIcon';
import type { TerminalSession } from '@/types/models';

// Props
const props = withDefaults(
  defineProps<{
    isMobile?: boolean;
    layout?: 'overlay' | 'sidebar' | 'docked-sidebar';
    compactMode?: 'auto' | 'force-compact' | 'force-comfortable';
    dockedCollapsed?: boolean;
  }>(),
  {
    isMobile: false,
    layout: 'overlay',
    compactMode: 'auto',
    dockedCollapsed: false,
  }
);

const { t } = useI18n();
const terminalStore = useTerminalStore();
const reminderStore = useTerminalReminderStore();
const sessionSnapshotStore = useTerminalSessionSnapshotStore();
const projectStore = useProjectStore();
const settingsStore = useSettingsStore();
const { terminalDisplayMode } = storeToRefs(settingsStore);
const { completionRecords, approvalRecords } = storeToRefs(reminderStore);
const { sessionsByProject, sessionsById } = storeToRefs(sessionSnapshotStore);
const isSidebar = computed(() => props.layout === 'sidebar' || props.layout === 'docked-sidebar');
const isDockedSidebar = computed(() => props.layout === 'docked-sidebar');

// 通知开关状态
const NOTIFICATIONS_STORAGE_KEY = 'kanban-ai-notifications-enabled';
const CLICKED_NOTIFICATIONS_STORAGE_KEY = 'kanban-ai-notifications-clicked';
const COMPACT_MODE_STORAGE_KEY = 'kanban-ai-notifications-compact';
const DISPLAY_MODE_STORAGE_KEY = 'kanban-ai-notifications-mode';
const CURRENT_PROJECT_ONLY_STORAGE_KEY = 'kanban-ai-notifications-current-project-only';
const notificationsEnabled = ref(true);
const clickedNotifications = ref<Set<string>>(new Set());
const compactModeEnabledStored = ref(true);
const compactModeEnabled = computed({
  get: () => {
    if (props.compactMode === 'force-compact') {
      return true;
    }
    if (props.compactMode === 'force-comfortable') {
      return false;
    }
    return compactModeEnabledStored.value;
  },
  set: value => {
    if (props.compactMode !== 'auto') {
      return;
    }
    compactModeEnabledStored.value = value;
  },
});
const canToggleCompactMode = computed(() => props.compactMode === 'auto');
const currentProjectOnly = ref(false);

// 项目序号颜色表
const PROJECT_INDEX_COLORS = [
  '#10b981', // 绿色
  '#3b82f6', // 蓝色
  '#f59e0b', // 橙色
  '#8b5cf6', // 紫色
  '#ec4899', // 粉色
  '#14b8a6', // 青色
  '#ef4444', // 红色
  '#6366f1', // 靛蓝色
];

const sessionSnapshotScopeId = `terminal-notification-bar-${Math.random().toString(36).slice(2, 8)}`;

type NotificationDisplayMode = 'standard' | 'idle-only' | 'exclude-idle';
const DISPLAY_MODE_SEQUENCE: NotificationDisplayMode[] = ['standard', 'idle-only', 'exclude-idle'];
const notificationDisplayMode = ref<NotificationDisplayMode>('standard');

// 从 localStorage 加载设置
function loadNotificationSettings() {
  try {
    const stored = localStorage.getItem(NOTIFICATIONS_STORAGE_KEY);
    if (stored !== null) {
      notificationsEnabled.value = stored === 'true';
    }
  } catch (error) {
    console.warn('[AI Notification] Failed to load notification settings', error);
  }
}

// 加载已点击的通知记录
function loadClickedNotifications() {
  try {
    const stored = localStorage.getItem(CLICKED_NOTIFICATIONS_STORAGE_KEY);
    if (stored) {
      const parsed = JSON.parse(stored);
      clickedNotifications.value = new Set(Array.isArray(parsed) ? parsed : []);
    }
  } catch (error) {
    console.warn('[AI Notification] Failed to load clicked notifications', error);
  }
}

function loadCompactModeSetting() {
  try {
    const stored = localStorage.getItem(COMPACT_MODE_STORAGE_KEY);
    if (stored !== null) {
      compactModeEnabledStored.value = stored === 'true';
    }
  } catch (error) {
    console.warn('[AI Notification] Failed to load compact mode setting', error);
  }
}

function loadDisplayModeSetting() {
  try {
    const stored = localStorage.getItem(DISPLAY_MODE_STORAGE_KEY) as NotificationDisplayMode | null;
    if (stored && DISPLAY_MODE_SEQUENCE.includes(stored)) {
      notificationDisplayMode.value = stored;
    }
  } catch (error) {
    console.warn('[AI Notification] Failed to load display mode setting', error);
  }
}

// 保存设置到 localStorage
function saveNotificationSettings() {
  try {
    localStorage.setItem(NOTIFICATIONS_STORAGE_KEY, String(notificationsEnabled.value));
  } catch (error) {
    console.warn('[AI Notification] Failed to save notification settings', error);
  }
}

// 保存已点击的通知记录
function saveClickedNotifications() {
  try {
    localStorage.setItem(
      CLICKED_NOTIFICATIONS_STORAGE_KEY,
      JSON.stringify(Array.from(clickedNotifications.value))
    );
  } catch (error) {
    console.warn('[AI Notification] Failed to save clicked notifications', error);
  }
}

function saveCompactModeSetting() {
  if (!canToggleCompactMode.value) {
    return;
  }
  try {
    localStorage.setItem(COMPACT_MODE_STORAGE_KEY, String(compactModeEnabled.value));
  } catch (error) {
    console.warn('[AI Notification] Failed to save compact mode setting', error);
  }
}

function saveDisplayModeSetting() {
  try {
    localStorage.setItem(DISPLAY_MODE_STORAGE_KEY, notificationDisplayMode.value);
  } catch (error) {
    console.warn('[AI Notification] Failed to save display mode setting', error);
  }
}

function loadCurrentProjectOnlySetting() {
  try {
    const stored = localStorage.getItem(CURRENT_PROJECT_ONLY_STORAGE_KEY);
    if (stored !== null) {
      currentProjectOnly.value = stored === 'true';
    }
  } catch (error) {
    console.warn('[AI Notification] Failed to load current project only setting', error);
  }
}

function saveCurrentProjectOnlySetting() {
  try {
    localStorage.setItem(CURRENT_PROJECT_ONLY_STORAGE_KEY, String(currentProjectOnly.value));
  } catch (error) {
    console.warn('[AI Notification] Failed to save current project only setting', error);
  }
}

function markNotificationsAsRead(notificationIds: string[]) {
  const next = new Set(clickedNotifications.value);
  let changed = false;
  notificationIds.forEach(id => {
    if (id && !next.has(id)) {
      next.add(id);
      changed = true;
    }
  });
  if (!changed) {
    return;
  }
  clickedNotifications.value = next;
  saveClickedNotifications();
}

function clearReadStateForNotifications(notificationIds: string[]) {
  const next = new Set(clickedNotifications.value);
  let changed = false;
  notificationIds.forEach(id => {
    if (id && next.delete(id)) {
      changed = true;
    }
  });
  if (!changed) {
    return;
  }
  clickedNotifications.value = next;
  saveClickedNotifications();
}

async function submitCompletionRecordsRead(recordIds: string[]) {
  await reminderStore.markCompletionRecordsRead(recordIds);
}

function markSessionCompletionNotificationsAsRead(sessionId: string) {
  if (!sessionId) {
    return;
  }
  const completionItems = notifications.value.filter(
    notification =>
      notification.type === 'completion' &&
      notification.sessionId === sessionId &&
      notification.state !== 'working'
  );
  const ids = completionItems.map(notification => notification.id);
  if (ids.length) {
    markNotificationsAsRead(ids);
  }

  const recordIdsToRead = completionItems.filter(item => !item.readAt).map(item => item.recordId);
  if (recordIdsToRead.length) {
    void submitCompletionRecordsRead(recordIdsToRead);
  }
}

// 切换通知开关
function toggleNotifications() {
  notificationsEnabled.value = !notificationsEnabled.value;
  saveNotificationSettings();
}

function toggleCompactMode() {
  if (!canToggleCompactMode.value) {
    return;
  }
  compactModeEnabled.value = !compactModeEnabled.value;
  saveCompactModeSetting();
}

function setDisplayMode(mode: NotificationDisplayMode) {
  if (!DISPLAY_MODE_SEQUENCE.includes(mode)) {
    return;
  }
  notificationDisplayMode.value = mode;
  saveDisplayModeSetting();
}

function cycleDisplayMode() {
  const currentIndex = DISPLAY_MODE_SEQUENCE.indexOf(notificationDisplayMode.value);
  const next = DISPLAY_MODE_SEQUENCE[(currentIndex + 1) % DISPLAY_MODE_SEQUENCE.length];
  setDisplayMode(next);
}

function handleNotificationModeSelect(key: string | number) {
  if (typeof key !== 'string') {
    return;
  }
  // 处理"仅当前项目" checkbox
  if (key === 'current-project-only') {
    currentProjectOnly.value = !currentProjectOnly.value;
    saveCurrentProjectOnlySetting();
    return;
  }
  setDisplayMode(key as NotificationDisplayMode);
}

// 检查通知是否被点击过
function isNotificationClicked(notificationId: string): boolean {
  return clickedNotifications.value.has(notificationId);
}

interface NotificationItem {
  id: string;
  recordId: string;
  type: 'completion' | 'approval' | 'idle';
  sessionId: string;
  projectId: string;
  projectName?: string;
  worktreeId?: string;
  branchName?: string;
  title: string;
  assistantName: string;
  assistantType?: string;
  assistantIcon?: string;
  assistantColor?: string;
  timestamp: Date;
  readAt?: Date;
  state?: 'completed' | 'working';
  lastAgentCommand?: string;
  lastUserInput?: string;
  assistantState?: string;
  processStatus?: 'idle' | 'busy' | 'unknown';
  interrupted?: boolean;
}

function isNotificationRead(notification: NotificationItem): boolean {
  if (isNotificationClicked(notification.id)) {
    return true;
  }
  return notification.type === 'completion' && Boolean(notification.readAt);
}

function isActiveCompletionNotification(notification: NotificationItem): boolean {
  return (
    notification.type === 'completion' &&
    notification.state === 'completed' &&
    notification.sessionId === currentActiveSessionId.value
  );
}

function isNotificationVisuallyRead(notification: NotificationItem): boolean {
  return isNotificationRead(notification) || isActiveCompletionNotification(notification);
}

type NotificationType = 'completion' | 'approval' | 'idle';

interface AssistantInfo {
  type?: string;
  name?: string;
  displayName?: string;
}

const defaultAssistantIcon = getAssistantIconByType();
const defaultAssistantColor = getAssistantColorByType();
const worktreeBranchCache = new Map<string, { branchName: string; projectId?: string }>();
const router = useRouter();
const currentRoute = useRoute();
const notifications = ref<NotificationItem[]>([]);
const pendingCompletionSessions = computed(() => {
  const sessionIds = new Set<string>();
  notifications.value.forEach(notification => {
    if (
      notification.type === 'completion' &&
      notification.sessionId &&
      notification.state !== 'working' &&
      !isNotificationRead(notification)
    ) {
      sessionIds.add(notification.sessionId);
    }
  });
  return sessionIds;
});

// 跟踪已有的完成通知 ID，用于检测新通知并播放声音
const existingCompletionIds = ref<Set<string>>(new Set());

watch(
  completionRecords,
  records => {
    const approvalSessionIds = new Set(approvalRecords.value.map(item => item.sessionId));
    const items = records
      .filter(record => !approvalSessionIds.has(record.sessionId))
      .map(mapCompletionRecord);

    // 检测是否有新通知，播放提示音
    const hasNewNotification = items.some(item => !existingCompletionIds.value.has(item.recordId));
    if (hasNewNotification && existingCompletionIds.value.size > 0) {
      playCompletionSound();
    }

    // 更新已知 ID 集合
    existingCompletionIds.value = new Set(items.map(item => item.recordId));

    setNotificationsForType('completion', items);
  },
  { immediate: true }
);

watch(
  approvalRecords,
  records => {
    const items = records.map(mapApprovalRecord);
    const approvalSessionIds = new Set(items.map(item => item.sessionId));
    notifications.value = notifications.value.filter(
      item => !(item.type === 'completion' && approvalSessionIds.has(item.sessionId))
    );
    setNotificationsForType('approval', items);
  },
  { immediate: true }
);

function getDisplayModeLabel(mode: NotificationDisplayMode) {
  if (mode === 'idle-only') {
    return t('terminal.notificationModeIdleOnly');
  }
  if (mode === 'exclude-idle') {
    return t('terminal.notificationModeExcludeIdle');
  }
  return t('terminal.notificationModeAll');
}

const notificationModeOptions = computed<DropdownOption[]>(() => [
  ...DISPLAY_MODE_SEQUENCE.map(mode => ({
    label: getDisplayModeLabel(mode),
    key: mode,
  })),
  { type: 'divider', key: 'd1' },
  {
    label: `${currentProjectOnly.value ? '✓ ' : ''}${t('terminal.notificationModeCurrentProjectOnly')}`,
    key: 'current-project-only',
  },
]);

const currentDisplayModeLabel = computed(() => getDisplayModeLabel(notificationDisplayMode.value));

// 计算项目序号映射（基于 projectId 分配序号和颜色）
const projectIndexMap = computed(() => {
  const map = new Map<string, { index: number; color: string }>();
  const seenProjects: string[] = [];

  // 按时间顺序遍历通知，收集唯一的 projectId
  for (const notification of notifications.value) {
    if (notification.projectId && !seenProjects.includes(notification.projectId)) {
      seenProjects.push(notification.projectId);
    }
  }

  // 为每个项目分配序号和颜色
  seenProjects.forEach((projectId, idx) => {
    map.set(projectId, {
      index: idx + 1,
      color: PROJECT_INDEX_COLORS[idx % PROJECT_INDEX_COLORS.length],
    });
  });

  return map;
});

// 获取通知的项目序号信息
function getProjectIndex(notification: NotificationItem) {
  return projectIndexMap.value.get(notification.projectId);
}

const currentProjectId = computed(() => {
  const id = currentRoute.params.id;
  return typeof id === 'string' ? id : '';
});

type DockedSessionItem = {
  sessionId: string;
  projectId: string;
  projectName?: string;
  title: string;
  branchName?: string;
  assistantDisplayName?: string;
  assistantType?: string;
  assistantIcon?: string;
  assistantColor?: string;
  assistantState?: string;
  interrupted?: boolean;
  processStatus?: 'idle' | 'busy' | 'unknown';
  activityTs: number;
  isCurrentSession: boolean;
  projectIndex?: { index: number; color: string };
};

function isAgentTerminalSession(session: TerminalSession) {
  return session.aiAssistant?.detected === true;
}

function parseTimestamp(value?: string) {
  if (!value) {
    return 0;
  }
  const ts = Date.parse(value);
  return Number.isFinite(ts) ? ts : 0;
}

const dockedProjectIdsToLoad = computed(() => {
  if (!isDockedSidebar.value) {
    return [];
  }
  const ids = new Set<string>();
  if (currentProjectId.value) {
    ids.add(currentProjectId.value);
  }
  projectStore.recentProjects.forEach(project => ids.add(project.id));
  projectStore.projects.forEach(project => {
    if (project.id) {
      ids.add(project.id);
    }
  });
  return Array.from(ids);
});

// 当前激活的终端 session ID
const currentActiveSessionId = computed(() => {
  if (!currentProjectId.value) {
    return '';
  }
  return terminalStore.getActiveTabId(currentProjectId.value) || '';
});

const activeCompletionAutoReadKey = computed(() => {
  const sessionId = currentActiveSessionId.value;
  if (!sessionId) {
    return '';
  }
  return notifications.value
    .filter(
      notification =>
        notification.type === 'completion' &&
        notification.sessionId === sessionId &&
        notification.state === 'completed'
    )
    .map(
      notification =>
        `${notification.recordId}:${notification.readAt ? 1 : 0}:${isNotificationClicked(notification.id) ? 1 : 0}`
    )
    .join(',');
});

function autoReadCurrentActiveCompletionNotifications() {
  const sessionId = currentActiveSessionId.value;
  if (!sessionId) {
    return;
  }

  const items = notifications.value.filter(
    notification =>
      notification.type === 'completion' &&
      notification.sessionId === sessionId &&
      notification.state === 'completed'
  );
  if (!items.length) {
    return;
  }

  markNotificationsAsRead(items.map(item => item.id));
  const recordIdsToRead = items.filter(item => !item.readAt).map(item => item.recordId);
  if (recordIdsToRead.length) {
    void submitCompletionRecordsRead(recordIdsToRead);
  }
}

watch(
  activeCompletionAutoReadKey,
  () => {
    autoReadCurrentActiveCompletionNotifications();
  },
  { immediate: true }
);

const dockedSessionItems = computed<DockedSessionItem[]>(() => {
  if (!isDockedSidebar.value || !notificationsEnabled.value) {
    return [];
  }

  const dockedCurrentProjectId = currentProjectId.value;
  const dockedCurrentSessionId = dockedCurrentProjectId
    ? terminalStore.getActiveTabId(dockedCurrentProjectId) || ''
    : '';

  const items: Omit<DockedSessionItem, 'projectIndex'>[] = [];
  dockedProjectIdsToLoad.value.forEach(projectId => {
    const sessions = sessionsByProject.value.get(projectId) ?? [];
    sessions.forEach(session => {
      if (!isAgentTerminalSession(session)) {
        return;
      }
      const worktreeId = session.worktreeId;
      const branchName = resolveBranchName(projectId, worktreeId);
      const assistantType = session.aiAssistant?.type;
      const activityTs =
        parseTimestamp(session.aiAssistant?.stateUpdatedAt) || parseTimestamp(session.createdAt);
      items.push({
        sessionId: session.id,
        projectId,
        projectName: getProjectNameById(projectId),
        title: session.title || 'Terminal',
        branchName,
        assistantDisplayName: session.aiAssistant?.displayName || session.aiAssistant?.name || '',
        assistantType,
        assistantIcon: getAssistantIconByType(assistantType),
        assistantColor: getAssistantColorByType(assistantType),
        assistantState: session.aiAssistant?.state,
        interrupted: session.aiAssistant?.interrupted === true,
        processStatus: session.processStatus,
        activityTs,
        isCurrentSession: Boolean(
          dockedCurrentProjectId &&
            dockedCurrentSessionId &&
            projectId === dockedCurrentProjectId &&
            session.id === dockedCurrentSessionId
        ),
      });
    });
  });

  const sorted = [...items].sort((a, b) => b.activityTs - a.activityTs);

  const presentProjectIds = new Set(sorted.map(item => item.projectId).filter(Boolean));
  const projectIds: string[] = [];
  projectStore.projects.forEach(project => {
    if (project.id && presentProjectIds.has(project.id)) {
      projectIds.push(project.id);
    }
  });
  sorted.forEach(item => {
    if (item.projectId && !projectIds.includes(item.projectId)) {
      projectIds.push(item.projectId);
    }
  });

  const projectIndex = new Map<string, { index: number; color: string }>();
  projectIds.forEach((projectId, idx) => {
    projectIndex.set(projectId, {
      index: idx + 1,
      color: PROJECT_INDEX_COLORS[idx % PROJECT_INDEX_COLORS.length],
    });
  });

  return sorted.map(item => ({
    ...item,
    projectIndex: projectIndex.get(item.projectId),
  }));
});

const isSingleDockedProject = computed(() => {
  if (!isDockedSidebar.value) {
    return false;
  }
  const ids = new Set<string>();
  dockedProjectIdsToLoad.value.forEach(projectId => {
    const sessions = sessionsByProject.value.get(projectId) ?? [];
    if (sessions.some(session => isAgentTerminalSession(session))) {
      ids.add(projectId);
    }
  });
  return ids.size <= 1;
});

const filteredNotifications = computed(() => {
  if (!notificationsEnabled.value) {
    return [];
  }
  return notifications.value.filter(notification => {
    // 先检查显示模式过滤
    if (!matchesDisplayMode(notification)) {
      return false;
    }
    // 如果启用了"仅当前项目"，过滤非当前项目的通知
    if (currentProjectOnly.value && currentProjectId.value) {
      if (notification.projectId !== currentProjectId.value) {
        return false;
      }
    }
    return true;
  });
});

function matchesDisplayMode(notification: NotificationItem) {
  if (notificationDisplayMode.value === 'idle-only') {
    return isIdleNotification(notification);
  }
  if (notificationDisplayMode.value === 'exclude-idle') {
    return !isIdleNotification(notification);
  }
  return true;
}

function isIdleNotification(notification: NotificationItem) {
  // idle 类型通知始终是空闲状态
  if (notification.type === 'idle') {
    return true;
  }

  if (notification.type === 'approval') {
    return true;
  }

  const assistantState = notification.assistantState;
  if (
    assistantState &&
    ['waiting_input', 'waiting_approval', 'idle', 'completed'].includes(assistantState)
  ) {
    return true;
  }

  if (notification.type === 'completion') {
    if (!notification.state || notification.state === 'completed') {
      return true;
    }
  }

  if (notification.processStatus === 'idle') {
    return true;
  }

  if (!notification.processStatus && notification.state && notification.state !== 'working') {
    return true;
  }

  return false;
}

function getDockedSessionSubtitle(item: DockedSessionItem) {
  return item.branchName?.trim() || '';
}

function getDockedSessionAccentColor(item: DockedSessionItem) {
  const assistantState = item.assistantState?.trim();
  switch (assistantState) {
    case 'working':
      return '#8b5cf6';
    case 'waiting_approval':
      return '#f79009';
    case 'completed':
      return '#10b981';
    case 'idle':
    case 'waiting_input':
      return '#9ca3af';
    default:
      break;
  }

  switch (item.processStatus) {
    case 'busy':
      return '#8b5cf6';
    case 'idle':
      return '#9ca3af';
    case 'unknown':
      return '#94a3b8';
    default:
      return 'rgba(15, 23, 42, 0.08)';
  }
}

function getDockedSessionClasses(item: DockedSessionItem): string[] {
  const shouldTreatCompletionAsRead = item.isCurrentSession;
  if (item.sessionId && pendingCompletionSessions.value.has(item.sessionId)) {
    return shouldTreatCompletionAsRead ? ['notification-idle'] : ['notification-completion'];
  }
  const assistantState = item.assistantState?.trim();
  switch (assistantState) {
    case 'working':
      return ['notification-working'];
    case 'waiting_approval':
      return ['notification-approval'];
    case 'waiting_input':
    case 'idle':
      return ['notification-idle'];
    case 'completed':
      return shouldTreatCompletionAsRead ? ['notification-idle'] : ['notification-completion'];
    default:
      break;
  }

  switch (item.processStatus) {
    case 'busy':
      return ['notification-working'];
    case 'idle':
      return ['notification-idle'];
    default:
      return [];
  }
}

async function handleDockedSessionClick(item: DockedSessionItem) {
  if (!item.projectId) {
    return;
  }
  if (item.sessionId) {
    markSessionCompletionNotificationsAsRead(item.sessionId);
  }
  const currentId = typeof currentRoute.params.id === 'string' ? currentRoute.params.id : '';
  if (currentId !== item.projectId) {
    try {
      await router.push({ name: 'project', params: { id: item.projectId } });
      await nextTick();
    } catch (error) {
      console.error('[AI Notification] Failed to switch project for docked session', error);
    }
  }

  await terminalStore.loadSessions(item.projectId);
  terminalStore.focusSession(item.projectId, item.sessionId);
}

watch(
  () =>
    projectStore.worktrees.map(worktree => ({
      id: worktree.id,
      branchName: worktree.branchName,
      projectId: worktree.projectId,
    })),
  entries => {
    entries.forEach(({ id, branchName, projectId }) => {
      if (id && branchName) {
        worktreeBranchCache.set(id, { branchName, projectId });
      }
    });
  },
  { deep: true, immediate: true }
);

function resolveBranchName(projectId?: string, worktreeId?: string) {
  if (!worktreeId) {
    return undefined;
  }
  const cached = worktreeBranchCache.get(worktreeId);
  if (cached && (!projectId || !cached.projectId || cached.projectId === projectId)) {
    return cached.branchName;
  }
  const match = projectStore.worktrees.find(worktree => worktree.id === worktreeId);
  if (match?.branchName) {
    worktreeBranchCache.set(worktreeId, {
      branchName: match.branchName,
      projectId: match.projectId,
    });
    return match.branchName;
  }
  return undefined;
}

function getLocationLabel(notification: NotificationItem) {
  return notification.branchName || notification.projectName || '';
}

function getProjectBranchLabel(notification: NotificationItem) {
  const project = (notification.projectName || '').trim();
  const branch = (notification.branchName || '').trim();
  if (project && branch) {
    return `${project} [${branch}]`;
  }
  if (project) {
    return project;
  }
  if (branch) {
    return `[${branch}]`;
  }
  return '';
}

function getCompletionHeader(notification: NotificationItem) {
  const projectLabel = getProjectBranchLabel(notification);
  let titleKey: string;
  if (notification.state === 'working') {
    titleKey = 'terminal.aiWorking';
  } else if (notification.interrupted || isNotificationVisuallyRead(notification)) {
    // 被中断或用户已看过的完成通知显示"空闲"
    titleKey = 'terminal.aiIdle';
  } else {
    titleKey = 'terminal.aiCompleted';
  }
  const baseTitle = t(titleKey);
  return projectLabel ? `${baseTitle} - ${projectLabel}` : baseTitle;
}

function getApprovalHeader(notification: NotificationItem) {
  const projectLabel = getProjectBranchLabel(notification);
  return projectLabel
    ? `${t('terminal.aiNeedsApproval')} - ${projectLabel}`
    : t('terminal.aiNeedsApproval');
}

function getIdleHeader(notification: NotificationItem) {
  const projectLabel = getProjectBranchLabel(notification);
  const baseTitle = t('terminal.aiIdle');
  return projectLabel ? `${baseTitle} - ${projectLabel}` : baseTitle;
}

function getNotificationHeader(notification: NotificationItem) {
  if (notification.type === 'completion') {
    return getCompletionHeader(notification);
  }
  if (notification.type === 'idle') {
    return getIdleHeader(notification);
  }
  return getApprovalHeader(notification);
}

function formatCompletionBody(notification: NotificationItem) {
  return notification.title;
}

function getNotificationDescription(notification: NotificationItem) {
  // 工作中、任务完成和空闲的卡片第二行不显示分支名
  if (notification.type === 'completion' || notification.type === 'idle') {
    return notification.title;
  }
  const body = `${t('terminal.isWaitingForApproval')} - ${notification.title}`;
  const location = getLocationLabel(notification);
  return location ? `[${location}] ${body}` : body;
}

function getTabLabel(notification: NotificationItem) {
  return notification.title?.trim() || 'AI Session';
}

function getLatestAgentCommand(notification: NotificationItem) {
  return notification.lastAgentCommand?.trim() || '';
}

function getLastUserInput(notification: NotificationItem) {
  return notification.lastUserInput?.trim() || '';
}

// 紧凑模式下的完整显示内容: {项目名}[{终端标题}] {用户上次输入的信息}
function getCompactDisplayText(notification: NotificationItem) {
  const projectName = (notification.projectName || '').trim();
  const title = (notification.title || '').trim();
  const userInput = getLastUserInput(notification);

  const parts: string[] = [];
  if (projectName) {
    parts.push(projectName);
  }
  if (title) {
    parts.push(`[${title}]`);
  }
  if (userInput) {
    parts.push(userInput);
  }

  return parts.join(' ');
}

// 格式化通知时间
function formatNotificationTime(timestamp: Date): string {
  const now = new Date();
  const diff = now.getTime() - timestamp.getTime();
  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (seconds < 60) {
    return t('terminal.timeJustNow');
  } else if (minutes < 60) {
    return t('terminal.timeMinutesAgo', { n: minutes });
  } else if (hours < 24) {
    return t('terminal.timeHoursAgo', { n: hours });
  } else {
    // 显示具体时间
    return timestamp.toLocaleString();
  }
}

// 判断是否在项目列表页
const isOnProjectListPage = computed(() => {
  return currentRoute.name === 'projects';
});

function getAssistantName(info?: AssistantInfo) {
  return info?.displayName || info?.name || 'AI';
}

function getProjectNameById(projectId?: string, fallback?: string) {
  if (!projectId) {
    return fallback;
  }
  const project = projectStore.projects.find(p => p.id === projectId);
  return project?.name || fallback;
}

function mapCompletionRecord(record: TerminalCompletionRecord): NotificationItem {
  const session = sessionsById.value.get(record.sessionId);
  const worktreeId = session?.worktreeId;
  const branchName = resolveBranchName(record.projectId, worktreeId);
  const assistantType = record.assistant?.type;
  const processStatus = session?.processStatus as 'idle' | 'busy' | 'unknown' | undefined;
  const assistantState = session?.aiAssistant?.state;
  const interrupted = session?.aiAssistant?.interrupted === true;
  // 直接使用后端返回的 lastUserInput，不回退到前端数据
  const lastUserInput = record.lastUserInput?.trim() || '';

  return {
    id: record.id,
    recordId: record.id,
    type: 'completion',
    sessionId: record.sessionId,
    projectId: record.projectId,
    projectName: record.projectName || getProjectNameById(record.projectId),
    worktreeId,
    branchName,
    title: record.title || session?.title || 'AI Session',
    assistantName: getAssistantName(record.assistant),
    assistantType,
    assistantIcon: getAssistantIconByType(assistantType),
    assistantColor: getAssistantColorByType(assistantType),
    timestamp: record.completedAt ? new Date(record.completedAt) : new Date(),
    readAt: record.readAt ? new Date(record.readAt) : undefined,
    state: record.state === 'working' ? 'working' : 'completed',
    assistantState,
    processStatus,
    lastUserInput: lastUserInput || undefined,
    interrupted,
  };
}

function mapApprovalRecord(record: TerminalApprovalRecord): NotificationItem {
  const session = sessionsById.value.get(record.sessionId);
  const worktreeId = session?.worktreeId;
  const branchName = resolveBranchName(record.projectId, worktreeId);
  const assistantType = record.assistant?.type;
  const processStatus = session?.processStatus as 'idle' | 'busy' | 'unknown' | undefined;
  const assistantState = session?.aiAssistant?.state;

  return {
    id: record.id,
    recordId: record.id,
    type: 'approval',
    sessionId: record.sessionId,
    projectId: record.projectId,
    projectName: record.projectName || getProjectNameById(record.projectId),
    worktreeId,
    branchName,
    title: record.title || session?.title || 'AI Session',
    assistantName: getAssistantName(record.assistant),
    assistantType,
    assistantIcon: getAssistantIconByType(assistantType),
    assistantColor: getAssistantColorByType(assistantType),
    timestamp: record.requestedAt ? new Date(record.requestedAt) : new Date(),
    assistantState,
    processStatus,
  };
}

function sortNotifications(list: NotificationItem[]) {
  return [...list].sort((a, b) => b.timestamp.getTime() - a.timestamp.getTime());
}

function setNotificationsForType(type: NotificationType, items: NotificationItem[]) {
  // 移除与新通知相同 session 的 idle 通知
  const sessionIdsWithNewNotifications = new Set(items.map(item => item.sessionId));
  const others = notifications.value.filter(item => {
    if (item.type === type) {
      return false; // 移除旧的同类型通知
    }
    // 移除与新通知相同 session 的 idle 通知
    if (item.type === 'idle' && sessionIdsWithNewNotifications.has(item.sessionId)) {
      return false;
    }
    return true;
  });
  notifications.value = sortNotifications([...others, ...items]);
  // 注意：不再自动标记通知为已读
  // 通知只应在用户真正点击或查看终端时才被标记，这样"已完成"状态才能正确显示
}

function removeNotificationLocally(recordId: string) {
  notifications.value = notifications.value.filter(item => item.recordId !== recordId);
}

function handleTerminalViewedEvent(event: any) {
  const sessionId = event?.sessionId;
  if (!sessionId) {
    return;
  }
  markSessionCompletionNotificationsAsRead(sessionId);
}

function getNotificationClass(notification: NotificationItem) {
  if (notification.type === 'completion') {
    return notification.state === 'working' ? 'notification-working' : 'notification-completion';
  }
  if (notification.type === 'idle') {
    return 'notification-idle';
  }
  return 'notification-approval';
}

// 播放完成提示音
function playCompletionSound() {
  try {
    const audioContext = new (window.AudioContext || (window as any).webkitAudioContext)();
    const oscillator = audioContext.createOscillator();
    const gainNode = audioContext.createGain();

    oscillator.connect(gainNode);
    gainNode.connect(audioContext.destination);

    oscillator.frequency.value = 523.25; // C5
    oscillator.type = 'sine';

    gainNode.gain.setValueAtTime(0.1, audioContext.currentTime);
    gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.5);

    oscillator.start(audioContext.currentTime);
    oscillator.stop(audioContext.currentTime + 0.5);
  } catch (error) {
    console.warn('Failed to play completion sound:', error);
  }
}

// 点击通知，切换到对应的终端
async function handleNotificationClick(notification: NotificationItem) {
  // 记录该通知已被点击
  markNotificationsAsRead([notification.id]);
  if (
    notification.type === 'completion' &&
    notification.state === 'completed' &&
    !notification.readAt
  ) {
    void submitCompletionRecordsRead([notification.recordId]);
  }

  const targetProjectId = notification.projectId;
  if (!targetProjectId) {
    return;
  }

  const currentProjectId = typeof currentRoute.params.id === 'string' ? currentRoute.params.id : '';

  if (currentProjectId !== targetProjectId) {
    try {
      await router.push({ name: 'project', params: { id: targetProjectId } });
      await nextTick();
    } catch (error) {
      console.error('[AI Notification] Failed to switch project for notification', error);
    }
  }

  // Ensure the terminal panel is visible when jumping from notifications
  terminalStore.emitter.emit('terminal:ensure-expanded', {
    projectId: targetProjectId,
  });

  // 切换到对应的终端标签
  terminalStore.setActiveTab(targetProjectId, notification.sessionId);
}

// 关闭通知
async function dismissNotification(notification: NotificationItem) {
  try {
    if (notification.type === 'completion') {
      await reminderStore.dismissCompletionRecord(notification.recordId);
    } else if (notification.type === 'approval') {
      await reminderStore.dismissApprovalRecord(notification.recordId);
    }
    removeNotificationLocally(notification.recordId);
  } catch (error) {
    console.error('[AI Notification] Failed to dismiss record', error);
  }
}

onMounted(() => {
  // 加载通知设置
  loadNotificationSettings();
  loadClickedNotifications();
  loadCompactModeSetting();
  loadDisplayModeSetting();
  loadCurrentProjectOnlySetting();
  reminderStore.retain();

  terminalStore.emitter.on('terminal:viewed', handleTerminalViewedEvent);
});

onUnmounted(() => {
  terminalStore.emitter.off('terminal:viewed', handleTerminalViewedEvent);
  reminderStore.release();
  sessionSnapshotStore.releaseScope(sessionSnapshotScopeId);
});

watch(
  [dockedProjectIdsToLoad, notificationsEnabled],
  () => {
    if (!notificationsEnabled.value) {
      sessionSnapshotStore.releaseScope(sessionSnapshotScopeId);
      return;
    }
    sessionSnapshotStore.retainScope(sessionSnapshotScopeId, dockedProjectIdsToLoad.value);
  },
  { immediate: true }
);

// 注：提醒记录轮询由 terminalReminderStore 统一负责
</script>

<template>
  <!-- 移动端：全屏列表布局 -->
  <div v-if="props.isMobile" class="mobile-notification-container">
    <div class="mobile-notification-header">
      <h3 class="mobile-notification-title">{{ t('terminal.notifications') }}</h3>
      <div class="mobile-notification-actions">
        <button
          type="button"
          class="mobile-action-btn"
          :class="{ 'is-active': compactModeEnabled }"
          @click="toggleCompactMode"
          :title="
            compactModeEnabled ? t('terminal.disableCompactMode') : t('terminal.enableCompactMode')
          "
        >
          <svg v-if="!compactModeEnabled" width="18" height="18" viewBox="0 0 24 24" fill="none">
            <path
              d="M4 7h16M4 12h16M4 17h16"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
            />
          </svg>
          <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none">
            <path
              d="M4 8h16M7 12h10M9 16h6"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
            />
          </svg>
        </button>
        <n-dropdown
          trigger="click"
          placement="bottom-end"
          :options="notificationModeOptions"
          @select="handleNotificationModeSelect"
        >
          <button
            type="button"
            class="mobile-action-btn"
            :class="{ 'is-active': notificationDisplayMode !== 'standard' }"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none">
              <path
                d="M4 7h16M4 12h10M4 17h8"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </button>
        </n-dropdown>
        <button
          type="button"
          class="mobile-action-btn"
          @click="toggleNotifications"
          :title="
            notificationsEnabled
              ? t('terminal.disableNotifications')
              : t('terminal.enableNotifications')
          "
        >
          <svg
            v-if="notificationsEnabled"
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
            <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
          </svg>
          <svg
            v-else
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M6.3 5.3a1 1 0 0 0-1.4 1.4l1.5 1.5A6 6 0 0 0 6 10c0 7-3 9-3 9h14"></path>
            <path d="m21.7 18.7-1.6-1.6"></path>
            <path d="M2 2l20 20"></path>
            <path d="M8.7 3a6 6 0 0 1 10.3 5c0 1-.1 1.9-.4 2.7"></path>
          </svg>
        </button>
      </div>
    </div>

    <div v-if="filteredNotifications.length === 0" class="mobile-empty-state">
      <svg
        width="48"
        height="48"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
        <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
      </svg>
      <p>{{ t('terminal.noNotifications') }}</p>
    </div>

    <div v-else class="mobile-notification-list" :class="{ 'is-compact': compactModeEnabled }">
      <div
        v-for="notification in filteredNotifications"
        :key="notification.id"
        class="mobile-notification-row"
        :class="{ 'is-current-session': notification.sessionId === currentActiveSessionId }"
      >
        <span
          v-if="getProjectIndex(notification)"
          class="mobile-project-badge"
          :class="{
            'is-single-project': projectIndexMap.size <= 1,
          }"
          :style="{ '--badge-color': getProjectIndex(notification)?.color }"
        >
          {{ getProjectIndex(notification)?.index }}
        </span>
        <div
          :class="[
            'mobile-notification-item',
            getNotificationClass(notification),
            { 'notification-clicked': isNotificationVisuallyRead(notification) },
          ]"
          @click="handleNotificationClick(notification)"
        >
          <div class="mobile-notification-content">
            <div v-if="!compactModeEnabled" class="mobile-notification-header-row">
              <span
                class="notification-icon"
                :style="{ color: notification.assistantColor || defaultAssistantColor }"
                v-html="notification.assistantIcon || defaultAssistantIcon"
              ></span>
              <span class="mobile-notification-header-title">{{
                getNotificationHeader(notification)
              }}</span>
            </div>
            <div class="mobile-notification-body" :class="{ 'compact-body': compactModeEnabled }">
              <div class="mobile-notification-description" :class="{ compact: compactModeEnabled }">
                <template v-if="compactModeEnabled">
                  <span class="notification-text compact-text">{{
                    getCompactDisplayText(notification)
                  }}</span>
                </template>
                <template v-else>
                  <span class="notification-text">
                    <span class="notification-tab-label">{{ getTabLabel(notification) }}</span>
                    <template v-if="getLatestAgentCommand(notification)">
                      <span class="notification-text-separator">·</span>
                      <span class="notification-command-text">{{
                        getLatestAgentCommand(notification)
                      }}</span>
                    </template>
                  </span>
                </template>
              </div>
              <div v-if="!compactModeEnabled" class="mobile-notification-footer">
                <span class="notification-action-hint">{{
                  t('terminal.clickToJumpTerminal')
                }}</span>
                <span class="notification-time" :title="notification.timestamp.toLocaleString()">{{
                  formatNotificationTime(notification.timestamp)
                }}</span>
              </div>
            </div>
          </div>
          <button
            class="mobile-notification-close"
            @click.stop="dismissNotification(notification)"
            :title="t('common.close')"
          >
            ×
          </button>
        </div>
      </div>
    </div>
  </div>

  <!-- 桌面端：右上角固定定位通知栏 -->
  <div
    v-else-if="!isOnProjectListPage"
    class="notification-bar-container"
    :class="{
      'compact-mode': compactModeEnabled,
      'is-sidebar': isSidebar,
      'is-docked-sidebar': isDockedSidebar,
      'is-docked-collapsed': isDockedSidebar && props.dockedCollapsed,
    }"
  >
    <div class="notification-toolbar">
      <button
        v-if="canToggleCompactMode"
        type="button"
        class="notification-action-btn"
        :class="{ 'is-active': compactModeEnabled }"
        @click="toggleCompactMode"
        :title="
          compactModeEnabled ? t('terminal.disableCompactMode') : t('terminal.enableCompactMode')
        "
      >
        <span class="action-btn-icon" aria-hidden="true">
          <svg v-if="!compactModeEnabled" width="16" height="16" viewBox="0 0 24 24" fill="none">
            <path
              d="M4 7h16M4 12h16M4 17h16"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
            />
          </svg>
          <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none">
            <path
              d="M4 8h16M7 12h10M9 16h6"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
            />
          </svg>
        </span>
        <span class="action-btn-label">
          {{
            compactModeEnabled
              ? t('terminal.compactModeCompact')
              : t('terminal.compactModeComfortable')
          }}
        </span>
      </button>

      <div
        v-if="!isDockedSidebar"
        class="notification-mode-control notification-action-btn"
        :class="{ 'is-active': notificationDisplayMode !== 'standard' }"
      >
        <button
          type="button"
          class="mode-control-btn"
          @click="cycleDisplayMode"
          :title="t('terminal.notificationModeCycleTooltip')"
        >
          <span class="action-btn-icon" aria-hidden="true">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
              <path
                d="M4 7h16M4 12h10M4 17h8"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </span>
          <span class="action-btn-label">{{ currentDisplayModeLabel }}</span>
        </button>
        <n-dropdown
          trigger="click"
          placement="bottom-end"
          :options="notificationModeOptions"
          @select="handleNotificationModeSelect"
        >
          <button
            type="button"
            class="mode-dropdown-btn"
            :title="t('terminal.notificationModeMenuTooltip')"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
              <path
                d="M6 9l6 6 6-6"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </button>
        </n-dropdown>
      </div>

      <!-- 通知开关按钮 -->
      <n-tooltip placement="bottom" :delay="250">
        <template #trigger>
          <button class="notification-toggle-btn" @click="toggleNotifications">
            <svg
              v-if="notificationsEnabled"
              xmlns="http://www.w3.org/2000/svg"
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
              <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
            </svg>
            <svg
              v-else
              xmlns="http://www.w3.org/2000/svg"
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <path d="M6.3 5.3a1 1 0 0 0-1.4 1.4l1.5 1.5A6 6 0 0 0 6 10c0 7-3 9-3 9h14"></path>
              <path d="m21.7 18.7-1.6-1.6"></path>
              <path d="M2 2l20 20"></path>
              <path d="M8.7 3a6 6 0 0 1 10.3 5c0 1-.1 1.9-.4 2.7"></path>
            </svg>
          </button>
        </template>
        {{
          notificationsEnabled
            ? t('terminal.disableNotifications')
            : t('terminal.enableNotifications')
        }}
      </n-tooltip>
      <slot name="toolbar-extra" />
    </div>

    <transition-group
      v-if="isDockedSidebar"
      name="notification-slide"
      tag="div"
      class="notification-list docked-session-list"
    >
      <div
        v-for="item in dockedSessionItems"
        :key="`${item.projectId}:${item.sessionId}`"
        class="notification-row docked-session-row"
        :class="{ 'is-current-session': item.isCurrentSession }"
        @click="handleDockedSessionClick(item)"
      >
        <div
          :class="['notification-item', 'docked-session-item', ...getDockedSessionClasses(item)]"
          :style="{ '--docked-session-accent': getDockedSessionAccentColor(item) }"
        >
          <div class="docked-session-main">
            <div class="docked-session-title">
              <span
                class="notification-icon"
                :style="{ color: item.assistantColor || defaultAssistantColor }"
                v-html="item.assistantIcon || defaultAssistantIcon"
              ></span>
              <span class="docked-session-title-text">{{ item.title }}</span>
              <span v-if="getDockedSessionSubtitle(item)" class="docked-session-state">
                · {{ getDockedSessionSubtitle(item) }}
              </span>
            </div>
          </div>

          <div class="docked-session-actions">
            <span
              v-if="item.projectIndex"
              class="project-index-badge docked-project-badge"
              :class="{
                'is-single-project': isSingleDockedProject,
              }"
              :style="{ '--badge-color': item.projectIndex.color }"
            >
              {{ item.projectIndex.index }}
            </span>
            <span
              class="docked-current-indicator"
              :class="{ 'is-hidden': !item.isCurrentSession }"
              :title="t('terminal.currentActiveSession')"
            >
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2.5"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
                <circle cx="12" cy="12" r="3"></circle>
              </svg>
            </span>
          </div>
        </div>
      </div>
    </transition-group>

    <transition-group
      v-else
      name="notification-slide"
      tag="div"
      class="notification-list"
      :class="{ 'is-compact': compactModeEnabled }"
    >
      <div
        v-for="notification in filteredNotifications"
        :key="notification.id"
        class="notification-row"
        :class="{ 'is-current-session': notification.sessionId === currentActiveSessionId }"
      >
        <!-- 当前激活 tab 指示器（眼睛图标） -->
        <span
          v-if="notification.sessionId === currentActiveSessionId"
          class="current-session-indicator"
          :title="t('terminal.currentActiveSession')"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
            <circle cx="12" cy="12" r="3"></circle>
          </svg>
        </span>
        <!-- 项目序号标签（在卡片外面，始终占位） -->
        <span
          v-if="getProjectIndex(notification)"
          class="project-index-badge"
          :class="{
            'is-single-project': projectIndexMap.size <= 1,
          }"
          :style="{
            '--badge-color': getProjectIndex(notification)?.color,
          }"
        >
          {{ getProjectIndex(notification)?.index }}
        </span>
        <div
          :class="[
            'notification-item',
            getNotificationClass(notification),
            { 'notification-clicked': isNotificationVisuallyRead(notification) },
          ]"
          @click="handleNotificationClick(notification)"
        >
          <div class="notification-content">
            <div v-if="!compactModeEnabled" class="notification-header">
              <span
                class="notification-icon"
                :style="{ color: notification.assistantColor || defaultAssistantColor }"
                v-html="notification.assistantIcon || defaultAssistantIcon"
              ></span>
              <span class="notification-title">
                {{ getNotificationHeader(notification) }}
              </span>
            </div>
            <div class="notification-body" :class="{ 'compact-body': compactModeEnabled }">
              <n-popover
                trigger="hover"
                :delay="1500"
                placement="bottom-end"
                :show-arrow="false"
                class="notification-popover"
              >
                <template #trigger>
                  <div class="notification-description" :class="{ compact: compactModeEnabled }">
                    <!-- 紧凑模式：显示 {项目名}[{终端标题}] {用户上次输入的信息} -->
                    <template v-if="compactModeEnabled">
                      <span class="notification-text compact-text">
                        {{ getCompactDisplayText(notification) }}
                      </span>
                    </template>
                    <!-- 普通模式：保持原有显示逻辑 -->
                    <template v-else>
                      <span class="notification-text">
                        <span class="notification-tab-label">
                          {{ getTabLabel(notification) }}
                        </span>
                        <template v-if="getLatestAgentCommand(notification)">
                          <span class="notification-text-separator">·</span>
                          <span class="notification-command-text">
                            {{ getLatestAgentCommand(notification) }}
                          </span>
                        </template>
                      </span>
                    </template>
                  </div>
                </template>
                <div class="notification-detail-text">
                  {{ getNotificationDescription(notification) }}
                </div>
              </n-popover>
              <div class="notification-footer">
                <span class="notification-action-hint">
                  {{ t('terminal.clickToJumpTerminal') }}
                </span>
                <span class="notification-time" :title="notification.timestamp.toLocaleString()">
                  {{ formatNotificationTime(notification.timestamp) }}
                </span>
              </div>
            </div>
          </div>
          <button
            class="notification-close"
            @click.stop="dismissNotification(notification)"
            :title="t('common.close')"
          >
            ×
          </button>
        </div>
      </div>
    </transition-group>
  </div>
</template>

<style scoped>
.notification-bar-container {
  position: fixed;
  top: 6px;
  right: 8px;
  z-index: 5;
  pointer-events: none;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.notification-bar-container.is-sidebar {
  position: relative;
  top: auto;
  right: auto;
  z-index: auto;
  height: 100%;
  width: min(360px, 32vw);
  max-width: 420px;
  pointer-events: auto;
  align-items: stretch;
}

.notification-bar-container.is-docked-sidebar {
  width: 100%;
  max-width: none;
  min-width: 0;
  gap: 6px;
}

.notification-bar-container.is-docked-sidebar.is-docked-collapsed .notification-toolbar {
  gap: 6px;
}

.notification-bar-container.is-docked-sidebar .notification-toolbar {
  padding-bottom: 6px;
  border-bottom: 1px solid rgba(15, 23, 42, 0.08);
}

.notification-bar-container.is-docked-sidebar .docked-session-list {
  width: 100%;
  max-width: none;
  flex: 1;
  overflow: auto;
  min-height: 0;
  padding: 0;
  box-sizing: border-box;
}

.notification-row.docked-session-row {
  gap: 0;
}

.notification-item.docked-session-item {
  width: 100%;
  min-width: 0;
  padding: 6px 10px;
  border-radius: 8px;
  box-shadow: none;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
  align-items: center;
  gap: 10px;
  border-left-color: var(--docked-session-accent, rgba(15, 23, 42, 0.08));
  box-sizing: border-box;
}

.notification-bar-container.is-docked-sidebar.is-docked-collapsed
  .notification-item.docked-session-item {
  padding: 6px;
  gap: 6px;
}

.docked-session-main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
}

.notification-bar-container.is-docked-sidebar.is-docked-collapsed .docked-session-main {
  flex: 0 0 auto;
}

.docked-session-title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.notification-bar-container.is-docked-sidebar.is-docked-collapsed .docked-session-title {
  gap: 0;
}

.docked-session-title .notification-icon {
  flex-shrink: 0;
}

.docked-session-title-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
  font-size: 12px;
}

.docked-session-state {
  flex-shrink: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  font-weight: 500;
  color: var(--text-color-secondary, #666666);
}

.notification-bar-container.is-docked-sidebar.is-docked-collapsed .docked-session-title-text,
.notification-bar-container.is-docked-sidebar.is-docked-collapsed .docked-session-state {
  display: none;
}

.docked-session-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  align-self: center;
}

.notification-bar-container.is-docked-sidebar.is-docked-collapsed .docked-session-actions {
  margin-left: auto;
  gap: 3px;
}

.docked-session-actions > * {
  display: flex;
  align-items: center;
  justify-content: center;
}

.notification-bar-container.is-docked-sidebar.is-docked-collapsed
  .project-index-badge.docked-project-badge,
.notification-bar-container.is-docked-sidebar.is-docked-collapsed .docked-current-indicator {
  width: 14px;
  height: 14px;
}

.notification-bar-container.is-docked-sidebar.is-docked-collapsed
  .project-index-badge.docked-project-badge {
  font-size: 7px;
  color: rgba(255, 255, 255, 0.92);
  border: 1px solid rgba(255, 255, 255, 0.55);
  box-shadow: none;
  opacity: 0.7;
}

.notification-bar-container.is-docked-sidebar.is-docked-collapsed .docked-current-indicator {
  border-width: 1px;
  box-shadow: 0 1px 4px rgba(59, 130, 246, 0.28);
}

.notification-bar-container.is-docked-sidebar.is-docked-collapsed .docked-current-indicator svg {
  width: 10px;
  height: 10px;
}

.project-index-badge.docked-project-badge {
  width: 18px;
  height: 18px;
  font-size: 10px;
  border-width: 1px;
}

.notification-bar-container.is-docked-sidebar .docked-current-indicator {
  position: static;
  left: auto;
  top: auto;
  transform: none;
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 0;
  border-radius: 50%;
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
  color: #ffffff;
  border: 1px solid rgba(59, 130, 246, 0.9);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.4);
  animation: none;
}

.notification-bar-container.is-docked-sidebar .docked-current-indicator.is-hidden {
  opacity: 0;
  pointer-events: none;
}

.notification-bar-container.is-docked-sidebar .docked-current-indicator svg {
  display: block;
}

.notification-toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
  pointer-events: none;
}

.notification-toolbar > * {
  pointer-events: auto;
}

.notification-action-btn,
.notification-mode-btn,
.notification-mode-dropdown-btn {
  height: 32px;
  border-radius: 6px;
  border: 1px solid var(--kanban-notification-button-border, rgba(0, 0, 0, 0.2));
  background: var(--app-surface-color, var(--body-color, #ffffff));
  box-shadow: none;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--kanban-notification-button-fg, var(--text-color, #000000));
  transition: all 0.2s ease;
  padding: 0 10px;
  font-size: 12px;
  font-weight: 500;
  gap: 6px;
  opacity: 0.9;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.notification-action-btn:hover,
.notification-mode-btn:hover,
.notification-mode-dropdown-btn:hover {
  opacity: 1;
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.15);
}

.notification-action-btn.is-active,
.notification-mode-btn.is-active,
.notification-mode-dropdown-btn.is-active {
  box-shadow: none;
}

.notification-action-btn.is-active:hover,
.notification-mode-btn.is-active:hover,
.notification-mode-dropdown-btn.is-active:hover {
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.15);
}

.notification-mode-control {
  display: inline-flex;
  border-radius: 6px;
  gap: 0;
  padding: 0;
  border: 1px solid var(--kanban-notification-button-border, rgba(0, 0, 0, 0.2));
  background: var(--app-surface-color, var(--body-color, #ffffff));
}

.mode-control-btn,
.mode-dropdown-btn {
  border: none;
  background: transparent;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font: inherit;
  color: inherit;
  padding: 0 10px;
  height: 100%;
}

.mode-control-btn {
  flex: 1;
  justify-content: flex-start;
  background: transparent;
}

.mode-dropdown-btn {
  padding: 0 8px;
  border-left: 1px solid rgba(0, 0, 0, 0.08);
  justify-content: center;
  background: transparent;
}
.action-btn-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.notification-toggle-btn {
  width: 36px;
  height: 32px;
  border-radius: 6px;
  border: 1px solid var(--kanban-notification-button-border, rgba(0, 0, 0, 0.2));
  background: var(--app-surface-color, var(--body-color, #ffffff));
  box-shadow: none;
  cursor: pointer;
  pointer-events: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--kanban-notification-button-fg, var(--text-color, #000000));
  transition: all 0.2s ease;
  opacity: 0.85;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.notification-toggle-btn:hover {
  opacity: 1;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.notification-toggle-btn:active {
  transform: scale(0.96);
}

.notification-toggle-btn svg {
  display: block;
}

:slotted(.docked-reset-btn) {
  width: 36px;
  height: 32px;
  border-radius: 6px;
  border: 1px solid var(--kanban-notification-button-border, rgba(0, 0, 0, 0.2));
  background: var(--app-surface-color, var(--body-color, #ffffff));
  box-shadow: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--kanban-notification-button-fg, var(--text-color, #000000));
  transition: all 0.2s ease;
  opacity: 0.85;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  padding: 0;
}

:slotted(.docked-reset-btn:hover) {
  opacity: 1;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

:slotted(.docked-reset-btn:active) {
  transform: scale(0.96);
}

:slotted(.docked-reset-btn svg) {
  display: block;
}

.notification-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: min(345px, calc(100vw - 32px));
  max-width: 380px;
  pointer-events: auto;
}

.notification-bar-container.is-sidebar .notification-list {
  width: 100%;
  max-width: none;
  flex: 1;
  overflow: auto;
  min-height: 0;
}

.notification-list.is-compact {
  gap: 4px;
}

.notification-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 14px;
  background: var(--app-surface-color, var(--body-color, #ffffff));
  border-radius: 12px;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.18);
  cursor: pointer;
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-left: 4px solid transparent;
  min-width: 280px;
  flex: 1;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

.notification-list.is-compact .notification-item {
  padding: 6px 10px;
  border-radius: 6px;
  min-width: 240px;
  gap: 6px;
  align-items: center;
}

.notification-bar-container.is-sidebar .notification-item {
  min-width: 0;
}

.notification-bar-container.is-docked-sidebar .notification-item {
  box-shadow: none;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

.notification-bar-container.is-docked-sidebar .notification-list.is-compact .notification-item {
  padding: 6px 8px;
  border-radius: 8px;
}

.notification-item:hover {
  transform: translateX(-4px);
  box-shadow: 0 16px 32px rgba(15, 23, 42, 0.22);
}

.notification-bar-container.is-docked-sidebar .notification-item:hover {
  transform: none;
  box-shadow: 0 6px 16px rgba(15, 23, 42, 0.12);
}

.notification-completion {
  --notification-completion-fill: var(--kanban-terminal-tab-completion-bg, rgba(16, 185, 129, 0.3));
  --notification-completion-accent: var(
    --kanban-terminal-tab-completion-border,
    rgba(16, 185, 129, 0.6)
  );
  background: #d1fae5;
  border-color: rgba(16, 185, 129, 0.3);
  border-left-color: #10b981;
  box-shadow: 0 12px 28px rgba(16, 185, 129, 0.15);
}

.notification-bar-container.is-docked-sidebar .notification-completion {
  box-shadow: none;
}

/* 已点击过的完成通知样式 - 左侧提示条变黑灰色，背景变白色 */
.notification-completion.notification-clicked {
  border-left-color: #9ca3af !important;
  background: #ffffff !important;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.12) !important;
}

.notification-bar-container.is-docked-sidebar .notification-completion.notification-clicked {
  box-shadow: none !important;
}

/* 空闲通知样式 - 灰色边框，白色背景 */
.notification-idle {
  --notification-idle-fill: rgba(156, 163, 175, 0.15);
  --notification-idle-accent: rgba(156, 163, 175, 0.8);
  background: #ffffff;
  border-color: rgba(156, 163, 175, 0.3);
  border-left-color: #9ca3af;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.12);
}

.notification-bar-container.is-docked-sidebar .notification-idle {
  box-shadow: none;
}

.notification-idle .notification-icon {
  color: var(--notification-idle-accent, #9ca3af);
}

/* 工作中 / 审批通知在已读后保持原样 */
.notification-approval {
  --notification-approval-fill: var(--kanban-terminal-tab-approval-bg, rgba(247, 144, 9, 0.25));
  --notification-approval-accent: var(
    --kanban-terminal-tab-approval-border,
    rgba(247, 144, 9, 0.55)
  );
  background: #fed7aa;
  border-color: rgba(247, 144, 9, 0.3);
  border-left-color: #f79009;
  box-shadow: 0 12px 28px rgba(247, 144, 9, 0.15);
}

.notification-bar-container.is-docked-sidebar .notification-approval {
  box-shadow: none;
}

.notification-working {
  --notification-working-fill: var(--kanban-terminal-tab-working-bg, rgba(237, 233, 254, 1));
  --notification-working-accent: var(--kanban-terminal-tab-working-border, rgba(139, 92, 246, 1));
  background: var(--notification-working-fill);
  border-color: rgba(139, 92, 246, 0.3);
  border-left-color: var(--notification-working-accent);
  box-shadow: 0 12px 28px rgba(139, 92, 246, 0.15);
}

.notification-bar-container.is-docked-sidebar .notification-working {
  box-shadow: none;
}

.notification-content {
  flex: 1;
  min-width: 0;
}

.notification-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
  font-weight: 600;
  font-size: 14px;
}

.notification-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--text-color, #000000);
}

.notification-icon :deep(svg) {
  display: block;
  width: 16px;
  height: 16px;
}

.notification-completion .notification-icon {
  color: var(--notification-completion-accent, rgba(16, 185, 129, 1));
}

.notification-approval .notification-icon {
  color: var(--notification-approval-accent, rgba(247, 144, 9, 1));
}

.notification-working .notification-icon {
  color: var(--notification-working-accent, rgba(139, 92, 246, 1));
}

.notification-title {
  color: var(--text-color, #000000);
}

.notification-body {
  font-size: 13px;
  color: var(--text-color-secondary, #666666);
  line-height: 1.3;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.notification-list.is-compact .notification-body {
  font-size: 12px;
  gap: 0;
}

.notification-body.compact-body {
  flex-direction: row;
  align-items: center;
}

.notification-description {
  display: flex;
  align-items: baseline;
  flex-wrap: nowrap;
  width: 100%;
  min-width: 0;
  gap: 4px;
}

.notification-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.notification-action-hint {
  font-size: 12px;
  color: var(--n-color-primary, #3b82f6);
  font-weight: 500;
}

.notification-time {
  font-size: 11px;
  color: var(--text-color-secondary, #999);
  flex-shrink: 0;
}

.notification-list.is-compact .notification-footer {
  display: none;
}

.project-badge {
  font-weight: 500;
  color: var(--text-color, #000000);
  flex-shrink: 0;
}
.notification-text {
  display: inline-block;
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.notification-tab-label {
  color: var(--n-color-primary, #3b82f6);
  font-weight: 600;
}

.notification-command-text {
  color: var(--text-color, #111);
}

.notification-text-separator {
  color: var(--text-color-secondary, #6b7280);
}

.notification-list.is-compact .notification-description {
  white-space: nowrap;
}

.notification-description.compact {
  gap: 10px;
  align-items: center;
}

.notification-text.compact-text {
  color: var(--text-color, #111);
  font-weight: 500;
}

.project-badge.compact {
  font-weight: 600;
  color: var(--n-color-primary, #3b82f6);
  padding: 2px 8px;
  border-right: 1px solid rgba(15, 23, 42, 0.12);
  margin-right: 4px;
  display: inline-flex;
  align-items: center;
  border-radius: 4px;
  background: rgba(59, 130, 246, 0.08);
  line-height: 1.2;
}

/* 通知行容器（包含序号和卡片） */
.notification-row {
  display: flex;
  align-items: center;
  gap: 6px;
  pointer-events: auto;
  position: relative;
}

/* 项目序号标签尺寸调整 */
.notification-list.is-compact .project-index-badge {
  width: 18px;
  height: 18px;
  font-size: 10px;
}

/* 当前激活 tab 指示器样式 - 绝对定位不占宽度 */
.current-session-indicator {
  position: absolute;
  left: 1px;
  top: 50%;
  transform: translateY(-50%);
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.4);
  animation: pulse-glow 2s ease-in-out infinite;
  z-index: 10;
}

.current-session-indicator svg {
  width: 10px;
  height: 10px;
}

.notification-list.is-compact .current-session-indicator {
  width: 18px;
  height: 18px;
  left: -23px;
}

/* 单项目时（无序号）紧凑模式调整 left */
.notification-list.is-compact
  .notification-row:has(.project-index-badge.is-single-project)
  .current-session-indicator {
  left: 0;
}

.notification-list.is-compact .current-session-indicator svg {
  width: 10px;
  height: 10px;
}

/* 当前激活 tab 行高亮效果 */
.notification-row.is-current-session .notification-item {
  box-shadow:
    0 0 0 2px rgba(59, 130, 246, 0.35),
    0 12px 28px rgba(15, 23, 42, 0.18);
}

.notification-bar-container.is-docked-sidebar
  .notification-row.docked-session-row.is-current-session
  .notification-item.docked-session-item {
  box-shadow: none;
}

@keyframes pulse-glow {
  0%,
  100% {
    box-shadow: 0 2px 8px rgba(59, 130, 246, 0.4);
  }
  50% {
    box-shadow: 0 2px 12px rgba(59, 130, 246, 0.6);
  }
}

.notification-close {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  border: none;
  background: transparent;
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
  color: var(--text-color-secondary, #666666);
  opacity: 0.6;
  transition: opacity 0.2s ease;
  padding: 0;
}

.notification-close:hover {
  opacity: 1;
}

.notification-detail-text {
  max-width: 420px;
  font-size: 13px;
  line-height: 1.4;
  color: var(--text-color, #000);
  word-break: break-word;
}

.notification-popover :deep(.n-popover__content) {
  padding: 10px 12px;
}

/* 动画 */
.notification-slide-enter-active {
  animation: slide-in 0.3s ease;
}

.notification-slide-leave-active {
  animation: slide-out 0.3s ease;
}

@keyframes slide-in {
  from {
    opacity: 0;
    transform: translateX(100%);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@keyframes slide-out {
  from {
    opacity: 1;
    transform: translateX(0);
  }
  to {
    opacity: 0;
    transform: translateX(100%);
  }
}

/* ==================== 移动端样式 ==================== */
.mobile-notification-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--app-surface-color, #ffffff);
}

.mobile-notification-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid var(--n-border-color, #e0e0e0);
  margin-bottom: 12px;
}

.mobile-notification-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-color, #000);
  margin: 0;
}

.mobile-notification-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.mobile-action-btn {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  border: 1px solid var(--n-border-color, #e0e0e0);
  background: var(--app-surface-color, #ffffff);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--text-color-secondary, #666);
  transition: all 0.2s ease;
}

.mobile-action-btn:active {
  background: var(--n-hover-color, #f5f5f5);
}

.mobile-action-btn.is-active {
  background: var(--n-primary-color, #18a058);
  border-color: var(--n-primary-color, #18a058);
  color: #fff;
}

.mobile-empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--text-color-secondary, #999);
  padding: 40px 20px;
}

.mobile-empty-state svg {
  opacity: 0.5;
}

.mobile-empty-state p {
  margin: 0;
  font-size: 14px;
}

.mobile-notification-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

.mobile-notification-list.is-compact {
  gap: 6px;
}

.mobile-notification-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.mobile-project-badge {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background-color: var(--badge-color, #3b82f6);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: default;
  border: 1px solid rgba(255, 255, 255, 0.9);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.14);
  margin-top: 12px;
}

.mobile-project-badge.is-single-project {
  visibility: hidden;
  pointer-events: none;
}

.mobile-notification-list.is-compact .mobile-project-badge {
  width: 20px;
  height: 20px;
  font-size: 10px;
  margin-top: 6px;
}

.mobile-notification-item {
  flex: 1;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  background: var(--app-surface-color, #ffffff);
  border-radius: 12px;
  border: 1px solid rgba(15, 23, 42, 0.1);
  border-left: 4px solid transparent;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.08);
  cursor: pointer;
  transition: all 0.2s ease;
}

.mobile-notification-item:active {
  transform: scale(0.98);
}

.mobile-notification-list.is-compact .mobile-notification-item {
  padding: 8px 12px;
  border-radius: 8px;
  gap: 8px;
  align-items: center;
}

/* 移动端通知类型样式 */
.mobile-notification-item.notification-completion {
  background: #d1fae5;
  border-color: rgba(16, 185, 129, 0.3);
  border-left-color: #10b981;
}

.mobile-notification-item.notification-completion.notification-clicked {
  border-left-color: #9ca3af !important;
  background: #ffffff !important;
}

.mobile-notification-item.notification-idle {
  background: #ffffff;
  border-color: rgba(156, 163, 175, 0.3);
  border-left-color: #9ca3af;
}

.mobile-notification-item.notification-approval {
  background: #fed7aa;
  border-color: rgba(247, 144, 9, 0.3);
  border-left-color: #f79009;
}

.mobile-notification-item.notification-working {
  background: var(--kanban-terminal-tab-working-bg, rgba(237, 233, 254, 1));
  border-color: rgba(139, 92, 246, 0.3);
  border-left-color: var(--kanban-terminal-tab-working-border, rgba(139, 92, 246, 1));
}

.mobile-notification-content {
  flex: 1;
  min-width: 0;
}

.mobile-notification-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.mobile-notification-header-title {
  font-weight: 600;
  font-size: 14px;
  color: var(--text-color, #000);
}

.mobile-notification-body {
  font-size: 13px;
  color: var(--text-color-secondary, #666);
  line-height: 1.4;
}

.mobile-notification-body.compact-body {
  display: flex;
  align-items: center;
}

.mobile-notification-description {
  display: flex;
  align-items: baseline;
  flex-wrap: nowrap;
  width: 100%;
  min-width: 0;
  gap: 4px;
}

.mobile-notification-description.compact {
  gap: 8px;
  align-items: center;
}

.mobile-notification-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 6px;
}

.mobile-notification-close {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
  color: var(--text-color-secondary, #666);
  opacity: 0.6;
  transition: opacity 0.2s ease;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.mobile-notification-close:active {
  opacity: 1;
}

/* 移动端当前激活行高亮 */
.mobile-notification-row.is-current-session .mobile-notification-item {
  box-shadow:
    0 0 0 2px rgba(59, 130, 246, 0.35),
    0 2px 8px rgba(15, 23, 42, 0.08);
}
</style>
