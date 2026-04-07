import { computed, ref } from 'vue';
import { defineStore } from 'pinia';
import Apis from '@/api';

const REMINDER_POLL_INTERVAL_MS = 5000;

export interface TerminalReminderAssistantInfo {
  type?: string;
  name?: string;
  displayName?: string;
}

export interface TerminalCompletionRecord {
  id: string;
  sessionId: string;
  projectId: string;
  projectName?: string;
  title: string;
  assistant?: TerminalReminderAssistantInfo;
  completedAt?: string;
  readAt?: string;
  dismissed?: boolean;
  state?: 'completed' | 'working';
  lastUserInput?: string;
}

export interface TerminalApprovalRecord {
  id: string;
  sessionId: string;
  projectId: string;
  projectName?: string;
  title: string;
  assistant?: TerminalReminderAssistantInfo;
  requestedAt?: string;
  dismissed?: boolean;
}

function normalizeCompletionRecords(payload: unknown): TerminalCompletionRecord[] {
  if (!Array.isArray(payload)) {
    return [];
  }
  return payload.filter(
    (item): item is TerminalCompletionRecord =>
      Boolean(item) && typeof item === 'object' && typeof (item as TerminalCompletionRecord).id === 'string'
  );
}

function normalizeApprovalRecords(payload: unknown): TerminalApprovalRecord[] {
  if (!Array.isArray(payload)) {
    return [];
  }
  return payload.filter(
    (item): item is TerminalApprovalRecord =>
      Boolean(item) && typeof item === 'object' && typeof (item as TerminalApprovalRecord).id === 'string'
  );
}

function uniqueIds(ids: string[]) {
  return Array.from(new Set(ids.map(id => id.trim()).filter(Boolean)));
}

export const useTerminalReminderStore = defineStore('terminal-reminder', () => {
  const completionRecords = ref<TerminalCompletionRecord[]>([]);
  const approvalRecords = ref<TerminalApprovalRecord[]>([]);
  const retainCount = ref(0);

  let pollTimer: number | null = null;
  let refreshPromise: Promise<void> | null = null;
  let listenersBound = false;

  const completionReadInFlight = new Set<string>();
  const completionDismissInFlight = new Set<string>();
  const approvalDismissInFlight = new Set<string>();

  const approvalSessionMap = computed(() => {
    const map: Record<string, boolean> = {};
    approvalRecords.value.forEach(record => {
      if (record.sessionId) {
        map[record.sessionId] = true;
      }
    });
    return map;
  });

  const unreadCompletionSessionMap = computed(() => {
    const approvals = approvalSessionMap.value;
    const map: Record<string, boolean> = {};
    completionRecords.value.forEach(record => {
      if (!record.sessionId || approvals[record.sessionId]) {
        return;
      }
      if (record.state === 'working' || record.readAt) {
        return;
      }
      map[record.sessionId] = true;
    });
    return map;
  });

  const projectNotificationCountMap = computed(() => {
    const sessionKeysByProject = new Map<string, Set<string>>();

    const remember = (projectId: string | undefined, key: string) => {
      if (!projectId || !key) {
        return;
      }
      let bucket = sessionKeysByProject.get(projectId);
      if (!bucket) {
        bucket = new Set<string>();
        sessionKeysByProject.set(projectId, bucket);
      }
      bucket.add(key);
    };

    approvalRecords.value.forEach(record => {
      if (record.sessionId) {
        remember(record.projectId, `approval:${record.sessionId}`);
      }
    });

    const approvals = approvalSessionMap.value;
    completionRecords.value.forEach(record => {
      if (!record.sessionId || approvals[record.sessionId]) {
        return;
      }
      if (record.state === 'working' || record.readAt) {
        return;
      }
      remember(record.projectId, `completion:${record.sessionId}`);
    });

    const result: Record<string, number> = {};
    sessionKeysByProject.forEach((bucket, projectId) => {
      result[projectId] = bucket.size;
    });
    return result;
  });

  function handleVisibilityRefresh() {
    if (retainCount.value > 0) {
      void refresh();
    }
  }

  function bindWindowListeners() {
    if (listenersBound || typeof window === 'undefined') {
      return;
    }
    listenersBound = true;
    window.addEventListener('focus', handleVisibilityRefresh);
    window.addEventListener('online', handleVisibilityRefresh);
    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', handleVisibilityRefresh);
    }
  }

  function unbindWindowListeners() {
    if (!listenersBound || typeof window === 'undefined') {
      return;
    }
    listenersBound = false;
    window.removeEventListener('focus', handleVisibilityRefresh);
    window.removeEventListener('online', handleVisibilityRefresh);
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', handleVisibilityRefresh);
    }
  }

  async function refresh() {
    if (refreshPromise) {
      return refreshPromise;
    }

    refreshPromise = (async () => {
      try {
        const [completionResponse, approvalResponse] = await Promise.all([
          Apis.terminalSession.terminalCompletionRecordsList({ cacheFor: 0 }).send(),
          Apis.terminalSession.terminalApprovalRecordsList({ cacheFor: 0 }).send(),
        ]);

        completionRecords.value = normalizeCompletionRecords(completionResponse?.items).filter(
          record => !record.dismissed
        );
        approvalRecords.value = normalizeApprovalRecords(approvalResponse?.items).filter(
          record => !record.dismissed
        );
      } catch (error) {
        console.error('[Terminal Reminder] Failed to refresh reminder records', error);
      } finally {
        refreshPromise = null;
      }
    })();

    return refreshPromise;
  }

  function startPolling() {
    bindWindowListeners();
    if (typeof window !== 'undefined' && pollTimer == null) {
      pollTimer = window.setInterval(() => {
        void refresh();
      }, REMINDER_POLL_INTERVAL_MS);
    }
    void refresh();
  }

  function stopPolling() {
    if (pollTimer != null && typeof window !== 'undefined') {
      window.clearInterval(pollTimer);
      pollTimer = null;
    }
    unbindWindowListeners();
  }

  function retain() {
    retainCount.value += 1;
    if (retainCount.value === 1) {
      startPolling();
    }
  }

  function release() {
    if (retainCount.value <= 0) {
      return;
    }
    retainCount.value -= 1;
    if (retainCount.value === 0) {
      stopPolling();
    }
  }

  async function markCompletionRecordsRead(recordIds: string[]) {
    const ids = uniqueIds(recordIds).filter(id => !completionReadInFlight.has(id));
    if (!ids.length) {
      return;
    }

    const readAt = new Date().toISOString();
    completionRecords.value = completionRecords.value.map(record =>
      ids.includes(record.id) && !record.readAt ? { ...record, readAt } : record
    );

    try {
      await Promise.all(
        ids.map(async recordId => {
          completionReadInFlight.add(recordId);
          try {
            await Apis.terminalSession
              .terminalCompletionRecordRead({
                pathParams: { recordId },
                cacheFor: 0,
              })
              .send();
          } finally {
            completionReadInFlight.delete(recordId);
          }
        })
      );
    } catch (error) {
      console.error('[Terminal Reminder] Failed to mark completion records read', error);
      void refresh();
    }
  }

  async function markSessionCompletionsRead(sessionId: string | undefined) {
    if (!sessionId) {
      return;
    }
    const ids = completionRecords.value
      .filter(record => record.sessionId === sessionId && !record.readAt && record.state !== 'working')
      .map(record => record.id);
    await markCompletionRecordsRead(ids);
  }

  async function dismissCompletionRecord(recordId: string) {
    const normalized = recordId.trim();
    if (!normalized || completionDismissInFlight.has(normalized)) {
      return;
    }

    completionDismissInFlight.add(normalized);
    const previous = completionRecords.value;
    completionRecords.value = completionRecords.value.filter(record => record.id !== normalized);

    try {
      await Apis.terminalSession
        .terminalCompletionRecordDismiss({
          pathParams: { recordId: normalized },
          cacheFor: 0,
        })
        .send();
    } catch (error) {
      console.error('[Terminal Reminder] Failed to dismiss completion record', normalized, error);
      completionRecords.value = previous;
      void refresh();
    } finally {
      completionDismissInFlight.delete(normalized);
    }
  }

  async function dismissApprovalRecord(recordId: string) {
    const normalized = recordId.trim();
    if (!normalized || approvalDismissInFlight.has(normalized)) {
      return;
    }

    approvalDismissInFlight.add(normalized);
    const previous = approvalRecords.value;
    approvalRecords.value = approvalRecords.value.filter(record => record.id !== normalized);

    try {
      await Apis.terminalSession
        .terminalApprovalRecordDismiss({
          pathParams: { recordId: normalized },
          cacheFor: 0,
        })
        .send();
    } catch (error) {
      console.error('[Terminal Reminder] Failed to dismiss approval record', normalized, error);
      approvalRecords.value = previous;
      void refresh();
    } finally {
      approvalDismissInFlight.delete(normalized);
    }
  }

  return {
    completionRecords,
    approvalRecords,
    approvalSessionMap,
    unreadCompletionSessionMap,
    projectNotificationCountMap,
    retain,
    release,
    refresh,
    markCompletionRecordsRead,
    markSessionCompletionsRead,
    dismissCompletionRecord,
    dismissApprovalRecord,
  };
});
