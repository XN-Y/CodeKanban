<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue';
import { useNotification, type NotificationReactive } from 'naive-ui';
import { useI18n } from 'vue-i18n';
import { useProjectStore } from '@/stores/project';
import { useWebSessionStore, type WebSessionAIEvent } from '@/stores/webSession';

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

function playCompletionSound() {
  try {
    const audioContext = new (window.AudioContext ||
      (
        window as Window &
          typeof globalThis & {
            webkitAudioContext?: typeof AudioContext;
          }
      ).webkitAudioContext)();
    const oscillator = audioContext.createOscillator();
    const gainNode = audioContext.createGain();

    oscillator.connect(gainNode);
    gainNode.connect(audioContext.destination);

    oscillator.frequency.value = 523.25;
    oscillator.type = 'sine';

    gainNode.gain.setValueAtTime(0.1, audioContext.currentTime);
    gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.5);

    oscillator.start(audioContext.currentTime);
    oscillator.stop(audioContext.currentTime + 0.5);
  } catch (error) {
    console.warn('[Web Session Completion Notifier] Failed to play sound', error);
  }
}

function handleCompletion(event: WebSessionAIEvent) {
  const sessionId = String(event?.sessionId || '').trim();
  if (!sessionId || notifiedSessions.has(sessionId)) {
    return;
  }

  notifiedSessions.add(sessionId);
  window.setTimeout(() => {
    notifiedSessions.delete(sessionId);
  }, NOTIFICATION_COOLDOWN);

  const projectName = resolveProjectName(event.projectId);
  const title = projectName
    ? `${t('terminal.aiCompleted')} - ${projectName}`
    : t('terminal.aiCompleted');
  const content = `${event.assistant.displayName} - ${event.sessionTitle}`;

  const instance = notification.success({
    title,
    content,
    duration: 4000,
    closable: true,
    onClose: () => {
      activeNotifications.delete(sessionId);
    },
    onAfterLeave: () => {
      activeNotifications.delete(sessionId);
    },
  });

  activeNotifications.set(sessionId, instance);
  playCompletionSound();
}

function handleWorking(event: WebSessionAIEvent) {
  clearNotification(event?.sessionId);
}

function handleViewed(event: { sessionId?: string }) {
  clearNotification(event?.sessionId);
}

onMounted(() => {
  webSessionStore.emitter.on('ai:completed', handleCompletion);
  webSessionStore.emitter.on('ai:working', handleWorking);
  webSessionStore.emitter.on('ai:closed', handleWorking);
  webSessionStore.emitter.on('web-session:viewed', handleViewed);
});

onUnmounted(() => {
  webSessionStore.emitter.off('ai:completed', handleCompletion);
  webSessionStore.emitter.off('ai:working', handleWorking);
  webSessionStore.emitter.off('ai:closed', handleWorking);
  webSessionStore.emitter.off('web-session:viewed', handleViewed);

  activeNotifications.forEach(instance => instance.destroy());
  activeNotifications.clear();
});
</script>

<template></template>
