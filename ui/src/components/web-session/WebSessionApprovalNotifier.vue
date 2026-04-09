<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue';
import { useNotification, type NotificationReactive } from 'naive-ui';
import { useI18n } from 'vue-i18n';
import { useProjectStore } from '@/stores/project';
import {
  useWebSessionStore,
  type WebSessionAIEvent,
  type WebSessionApprovalEvent,
} from '@/stores/webSession';

const notification = useNotification();
const { t } = useI18n();
const projectStore = useProjectStore();
const webSessionStore = useWebSessionStore();

const notifiedSessions = new Set<string>();
const activeNotifications = new Map<string, NotificationReactive>();
const NOTIFICATION_COOLDOWN = 3000;

function resolveProjectName(projectId?: string) {
  const normalizedProjectId = String(projectId || '').trim();
  if (!normalizedProjectId) {
    return '';
  }
  if (projectStore.currentProject?.id === normalizedProjectId && projectStore.currentProject.name) {
    return projectStore.currentProject.name;
  }
  return (
    projectStore.projects.find(project => project.id === normalizedProjectId)?.name ||
    projectStore.recentProjects.find(project => project.id === normalizedProjectId)?.name ||
    ''
  );
}

function clearNotification(sessionId?: string) {
  const normalizedSessionId = String(sessionId || '').trim();
  if (!normalizedSessionId) {
    return;
  }
  const instance = activeNotifications.get(normalizedSessionId);
  if (!instance) {
    return;
  }
  instance.destroy();
  activeNotifications.delete(normalizedSessionId);
}

function handleApproval(event: WebSessionApprovalEvent) {
  const sessionId = String(event?.sessionId || '').trim();
  if (!sessionId || notifiedSessions.has(sessionId)) {
    return;
  }

  notifiedSessions.add(sessionId);
  window.setTimeout(() => {
    notifiedSessions.delete(sessionId);
  }, NOTIFICATION_COOLDOWN);

  const projectName = resolveProjectName(event.projectId);
  const content = projectName
    ? `[${projectName}] ${event.assistant.displayName} ${t('terminal.isWaitingForApproval')} - ${event.sessionTitle}`
    : `${event.assistant.displayName} ${t('terminal.isWaitingForApproval')} - ${event.sessionTitle}`;

  const instance = notification.warning({
    title: t('terminal.aiNeedsApproval'),
    content,
    duration: 6000,
    closable: true,
    onClose: () => {
      activeNotifications.delete(sessionId);
    },
    onAfterLeave: () => {
      activeNotifications.delete(sessionId);
    },
  });

  activeNotifications.set(sessionId, instance);
}

function handleReset(event: WebSessionAIEvent | { sessionId?: string }) {
  clearNotification(event?.sessionId);
}

onMounted(() => {
  webSessionStore.emitter.on('ai:approval-needed', handleApproval);
  webSessionStore.emitter.on('ai:working', handleReset);
  webSessionStore.emitter.on('ai:completed', handleReset);
  webSessionStore.emitter.on('ai:closed', handleReset);
  webSessionStore.emitter.on('web-session:viewed', handleReset);
});

onUnmounted(() => {
  webSessionStore.emitter.off('ai:approval-needed', handleApproval);
  webSessionStore.emitter.off('ai:working', handleReset);
  webSessionStore.emitter.off('ai:completed', handleReset);
  webSessionStore.emitter.off('ai:closed', handleReset);
  webSessionStore.emitter.off('web-session:viewed', handleReset);

  activeNotifications.forEach(instance => instance.destroy());
  activeNotifications.clear();
});
</script>

<template></template>
