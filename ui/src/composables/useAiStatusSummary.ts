import { computed } from 'vue';
import { useProjectStore } from '@/stores/project';
import { useTerminalReminderStore } from '@/stores/terminalReminder';
import { useWebSessionStore } from '@/stores/webSession';

export interface AiStatusSummary {
  working: number;
  blocking: number;
  unreadCompleted: number;
}

type StatusBucket = keyof AiStatusSummary;

const EMPTY_AI_STATUS_SUMMARY: Readonly<AiStatusSummary> = Object.freeze({
  working: 0,
  blocking: 0,
  unreadCompleted: 0,
});

const BUCKET_PRIORITY: Record<StatusBucket, number> = {
  unreadCompleted: 0,
  working: 1,
  blocking: 2,
};

function createSummary(): AiStatusSummary {
  return {
    working: 0,
    blocking: 0,
    unreadCompleted: 0,
  };
}

function ensureProjectBuckets(
  projectBuckets: Map<string, Map<string, StatusBucket>>,
  projectId: string
) {
  let sessionBuckets = projectBuckets.get(projectId);
  if (!sessionBuckets) {
    sessionBuckets = new Map<string, StatusBucket>();
    projectBuckets.set(projectId, sessionBuckets);
  }
  return sessionBuckets;
}

function rememberSessionBucket(
  projectBuckets: Map<string, Map<string, StatusBucket>>,
  projectId: string | undefined,
  sessionKey: string,
  bucket: StatusBucket
) {
  if (!projectId || !sessionKey) {
    return;
  }

  const sessionBuckets = ensureProjectBuckets(projectBuckets, projectId);
  const currentBucket = sessionBuckets.get(sessionKey);
  if (currentBucket && BUCKET_PRIORITY[currentBucket] >= BUCKET_PRIORITY[bucket]) {
    return;
  }
  sessionBuckets.set(sessionKey, bucket);
}

function summarizeProjectBuckets(sessionBuckets?: Map<string, StatusBucket>): AiStatusSummary {
  if (!sessionBuckets) {
    return createSummary();
  }

  const summary = createSummary();
  sessionBuckets.forEach(bucket => {
    summary[bucket] += 1;
  });
  return summary;
}

export function formatAiStatusTitle(summary: AiStatusSummary, appName: string) {
  const total = summary.working + summary.blocking + summary.unreadCompleted;
  if (total === 0) {
    return appName;
  }
  return `[${summary.working}/${summary.blocking}/${summary.unreadCompleted}] ${appName}`;
}

export function useAiStatusSummary() {
  const projectStore = useProjectStore();
  const reminderStore = useTerminalReminderStore();
  const webSessionStore = useWebSessionStore();

  const projectBucketMap = computed(() => {
    const buckets = new Map<string, Map<string, StatusBucket>>();

    reminderStore.approvalRecords.forEach(record => {
      if (!record.sessionId) {
        return;
      }
      rememberSessionBucket(buckets, record.projectId, `terminal:${record.sessionId}`, 'blocking');
    });

    reminderStore.completionRecords.forEach(record => {
      if (!record.sessionId) {
        return;
      }
      if (record.state === 'working') {
        rememberSessionBucket(buckets, record.projectId, `terminal:${record.sessionId}`, 'working');
        return;
      }
      if (!record.readAt) {
        rememberSessionBucket(
          buckets,
          record.projectId,
          `terminal:${record.sessionId}`,
          'unreadCompleted'
        );
      }
    });

    projectStore.projects.forEach(project => {
      webSessionStore.getSessions(project.id).forEach(session => {
        const liveState = webSessionStore.getLiveState(session.id);
        const sessionKey = `web:${session.id}`;

        if (liveState.phase === 'waiting_approval' || liveState.phase === 'waiting_input') {
          rememberSessionBucket(buckets, project.id, sessionKey, 'blocking');
          return;
        }
        if (liveState.running) {
          rememberSessionBucket(buckets, project.id, sessionKey, 'working');
          return;
        }
        if (session.hasUnread && liveState.phase === 'done') {
          rememberSessionBucket(buckets, project.id, sessionKey, 'unreadCompleted');
        }
      });
    });

    return buckets;
  });

  const projectSummaries = computed<Record<string, AiStatusSummary>>(() => {
    const summaries: Record<string, AiStatusSummary> = {};
    projectBucketMap.value.forEach((sessionBuckets, projectId) => {
      summaries[projectId] = summarizeProjectBuckets(sessionBuckets);
    });
    return summaries;
  });

  const totalSummary = computed(() => {
    const summary = createSummary();
    projectBucketMap.value.forEach(sessionBuckets => {
      const projectSummary = summarizeProjectBuckets(sessionBuckets);
      summary.working += projectSummary.working;
      summary.blocking += projectSummary.blocking;
      summary.unreadCompleted += projectSummary.unreadCompleted;
    });
    return summary;
  });

  function getProjectSummary(projectId: string) {
    return projectSummaries.value[projectId] ?? EMPTY_AI_STATUS_SUMMARY;
  }

  return {
    projectSummaries,
    totalSummary,
    getProjectSummary,
  };
}
