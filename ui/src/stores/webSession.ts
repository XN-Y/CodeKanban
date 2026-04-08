import EventEmitter from 'eventemitter3';
import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { webSessionApi } from '@/api/webSession';
import type { WebSessionAttachment, WebSessionSummary } from '@/types/models';
import { resolveWsUrl } from '@/utils/ws';

type WireFrameKind = 'ack' | 'snap' | 'evt' | 'err';
type SessionStatus = WebSessionSummary['status'];

type WireSession = {
  id: string;
  pid: string;
  wid?: string | null;
  oi?: number;
  ag: 'claude' | 'codex';
  md: string;
  re?: 'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh';
  wm: 'default' | 'plan';
  pl: 'default' | 'elevated' | 'yolo';
  ttl: string;
  cwd: string;
  nsid?: string | null;
  st: SessionStatus;
  unr: boolean;
  lu: number;
  lma?: number | null;
  usa?: {
    in?: number;
    cin?: number;
    out?: number;
  };
  cost?: number;
};

type WireEvent = {
  id: string;
  sq: number;
  tp: string;
  rid2?: string;
  pid2?: string;
  ts: number;
  p?: Record<string, unknown>;
};

type WireFrame = {
  v: number;
  k: WireFrameKind;
  rid?: string;
  sid?: string;
  ts: number;
  op?: string;
  p?: unknown;
  ok?: number;
  s?: WireSession;
  h?: {
    evs: WireEvent[];
    hm: boolean;
    bc?: string;
    tot: number;
  };
  e?: WireEvent;
  code?: string;
  msg?: string;
  retry?: boolean;
};

export interface WebSessionToolBlock {
  id: string;
  name: string;
  kind?: string;
  input?: unknown;
  output?: string;
  status: 'running' | 'done' | 'error';
  meta?: Record<string, unknown>;
  commandGroup?: {
    id: string;
    count: number;
    firstSeq?: number;
    lastSeq?: number;
    latestToolId?: string;
    compacted?: boolean;
  };
}

export interface WebSessionHistoryAnswerEntry {
  id: string;
  label: string;
  values: string[];
  masked?: boolean;
}

export interface WebSessionHistoryDetail {
  type: 'approval_request' | 'approval_response' | 'user_input_request' | 'user_input_response';
  prompt?: string;
  questions?: WebSessionUserInputQuestion[];
  answers?: WebSessionHistoryAnswerEntry[];
  action?: 'approve' | 'reject' | string;
}

export interface WebSessionBlock {
  key: string;
  id: string;
  kind: 'user' | 'assistant' | 'system' | 'tool';
  text: string;
  timestamp: number;
  attachments: Array<{
    id: string;
    name: string;
    mime?: string;
    size?: number;
  }>;
  tool?: WebSessionToolBlock;
  level?: 'info' | 'warn' | 'error';
  done?: boolean;
  detail?: WebSessionHistoryDetail;
}

export interface WebSessionApprovalState {
  id: string;
  prompt: string;
  requestedAt: number;
  stale: boolean;
  recoveryReason?: string;
  recoveryMessage?: string;
}

export interface WebSessionUserInputOption {
  label: string;
  description: string;
}

export interface WebSessionUserInputQuestion {
  id: string;
  header: string;
  question: string;
  isOther: boolean;
  isSecret: boolean;
  options: WebSessionUserInputOption[];
}

export interface WebSessionUserInputState {
  id: string;
  itemId: string;
  prompt: string;
  questions: WebSessionUserInputQuestion[];
  requestedAt: number;
  stale: boolean;
  recoveryReason?: string;
  recoveryMessage?: string;
}

export interface WebSessionLiveState {
  phase:
    | 'idle'
    | 'starting'
    | 'thinking'
    | 'tool'
    | 'waiting_approval'
    | 'waiting_input'
    | 'done'
    | 'error';
  running: boolean;
  updatedAt: number;
  tool?: {
    id: string;
    name: string;
    kind?: string;
  };
  approval?: WebSessionApprovalState | null;
  userInput?: WebSessionUserInputState | null;
  errorMessage?: string;
}

export interface WebSessionPendingInput {
  id: string;
  mode: 'redirect' | 'queue';
  text: string;
  attachmentIds: string[];
  createdAt: number;
}

type WebSessionAssistantDescriptor = {
  type: 'claude-code' | 'codex';
  name: 'Claude Code' | 'Codex';
  displayName: 'Claude Code' | 'Codex';
};

export interface WebSessionAIEvent {
  sessionId: string;
  sessionTitle: string;
  projectId: string;
  assistant: WebSessionAssistantDescriptor;
}

export interface WebSessionApprovalEvent extends WebSessionAIEvent {
  approval: WebSessionApprovalState;
}

type HistoryMeta = {
  hasMore: boolean;
  beforeCursor: string;
  total: number;
  loading: boolean;
};

type SessionSnapshotWaiter = {
  id: string;
  resolve: (frame: WireFrame) => void;
  reject: (reason?: unknown) => void;
  timer: number | null;
};

const ACTIVE_SESSION_STORAGE_KEY = 'kanban-web-active-session';
const WS_PATH = '/api/v1/web-sessions/ws';
const PROCESS_RESTART_REASON = 'process_restart';
const DEFAULT_RECOVERY_MESSAGE =
  'The previous run was interrupted because the app restarted. Send a new message to continue.';

function loadStoredActiveSessions() {
  try {
    const raw = localStorage.getItem(ACTIVE_SESSION_STORAGE_KEY);
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw) as Record<string, string>;
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function persistActiveSessions(value: Record<string, string>) {
  try {
    const persisted = Object.fromEntries(
      Object.entries(value).filter(([, sessionId]) => typeof sessionId === 'string' && sessionId)
    );
    localStorage.setItem(ACTIVE_SESSION_STORAGE_KEY, JSON.stringify(persisted));
  } catch (error) {
    console.warn('[Web Session] Failed to persist active sessions', error);
  }
}

function compareSessions(left: WebSessionSummary, right: WebSessionSummary) {
  if (left.orderIndex !== right.orderIndex) {
    return left.orderIndex - right.orderIndex;
  }
  if (left.updatedAt !== right.updatedAt) {
    return right.updatedAt.localeCompare(left.updatedAt);
  }
  return left.id.localeCompare(right.id);
}

function sortSessions(sessions: WebSessionSummary[]) {
  return [...sessions].sort(compareSessions);
}

function isWorkingPhase(phase: WebSessionLiveState['phase']) {
  return phase === 'starting' || phase === 'thinking' || phase === 'tool';
}

function isProcessRestartPayload(payload?: Record<string, unknown>) {
  return String(payload?.reason ?? '') === PROCESS_RESTART_REASON;
}

function getRecoveryMessage(payload?: Record<string, unknown>) {
  const message = typeof payload?.msg === 'string' ? payload.msg.trim() : '';
  return message || DEFAULT_RECOVERY_MESSAGE;
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return undefined;
  }
  return value as Record<string, unknown>;
}

function parseToolCommandGroup(value: unknown) {
  const record = asRecord(value);
  if (!record) {
    return undefined;
  }
  const id = String(record.id ?? '').trim();
  if (!id) {
    return undefined;
  }
  return {
    id,
    count: Math.max(1, Number(record.count ?? 1) || 1),
    firstSeq:
      typeof record.firstSeq === 'number' && Number.isFinite(record.firstSeq)
        ? record.firstSeq
        : undefined,
    lastSeq:
      typeof record.lastSeq === 'number' && Number.isFinite(record.lastSeq)
        ? record.lastSeq
        : undefined,
    latestToolId: String(record.latestToolId ?? '').trim() || undefined,
    compacted: record.compacted === true,
  };
}

function parseUserInputQuestions(value: unknown): WebSessionUserInputQuestion[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map(item => {
      const record = asRecord(item);
      if (!record) {
        return null;
      }
      return {
        id: String(record.id ?? ''),
        header: String(record.header ?? ''),
        question: String(record.question ?? ''),
        isOther: record.isOther === true,
        isSecret: record.isSecret === true,
        options: Array.isArray(record.options)
          ? record.options
              .map(option => {
                const optionRecord = asRecord(option);
                if (!optionRecord) {
                  return null;
                }
                return {
                  label: String(optionRecord.label ?? ''),
                  description: String(optionRecord.description ?? ''),
                };
              })
              .filter((option): option is WebSessionUserInputOption => Boolean(option))
          : [],
      };
    })
    .filter((question): question is WebSessionUserInputQuestion => Boolean(question));
}

function summarizeUserInputPrompt(payload: Record<string, unknown>) {
  const explicit = String(payload.txt ?? '').trim();
  if (explicit) {
    return explicit;
  }
  const questions = parseUserInputQuestions(payload.qs);
  const lines = questions
    .map(question => question.question.trim() || question.header.trim())
    .filter(Boolean);
  return lines.length > 0 ? lines.join('\n') : 'Additional input is required.';
}

function summarizeUserInputAnswer(payload: Record<string, unknown>) {
  const answers = asRecord(payload.ans);
  if (!answers) {
    return 'Submitted requested input';
  }
  const parts = Object.values(answers)
    .flatMap(value => (Array.isArray(value) ? value : []))
    .map(value => String(value).trim())
    .filter(Boolean);
  if (parts.length === 0) {
    return 'Submitted requested input';
  }
  return parts.join(', ');
}

function buildUserInputAnswerEntries(
  payload: Record<string, unknown>,
  questions: WebSessionUserInputQuestion[]
): WebSessionHistoryAnswerEntry[] {
  const answers = asRecord(payload.ans);
  if (!answers) {
    return [];
  }

  const questionMap = new Map(questions.map(question => [question.id, question]));
  const result: WebSessionHistoryAnswerEntry[] = [];
  Object.entries(answers).forEach(([questionId, value]) => {
    const question = questionMap.get(questionId);
    const values = (Array.isArray(value) ? value : [])
      .map(item => String(item).trim())
      .filter(Boolean);
    if (values.length === 0) {
      return;
    }
    result.push({
      id: questionId,
      label:
        question?.header?.trim() || question?.question?.trim() || questionId || 'Submitted answer',
      values,
      masked: question?.isSecret === true,
    });
  });
  return result;
}

export const useWebSessionStore = defineStore('web-session', () => {
  const sessionsByProject = ref<Record<string, WebSessionSummary[]>>({});
  const eventsBySession = ref<Record<string, WireEvent[]>>({});
  const historyBySession = ref<Record<string, HistoryMeta>>({});
  const draftAttachmentsByProject = ref<Record<string, WebSessionAttachment[]>>({});
  const pendingInputsBySession = ref<Record<string, WebSessionPendingInput[]>>({});
  const activeSessionIdByProject = ref<Record<string, string>>(loadStoredActiveSessions());
  const loadedProjects = ref<Record<string, boolean>>({});
  const emitter = new EventEmitter();

  const connectionState = ref<'idle' | 'connecting' | 'open' | 'closed'>('idle');
  const lastError = ref<string | null>(null);

  let socket: WebSocket | null = null;
  let connectPromise: Promise<void> | null = null;
  let reconnectTimer: number | null = null;
  const pending = new Map<
    string,
    {
      resolve: (value: WireFrame) => void;
      reject: (reason?: unknown) => void;
    }
  >();
  const snapshotWaitersBySession = new Map<string, SessionSnapshotWaiter[]>();
  const seenSeqBySession = new Map<string, Set<number>>();
  const redirectAbortSessions = new Set<string>();
  const pendingFlushTimers = new Map<string, number>();
  const flushingSessions = new Set<string>();

  const allSessionIds = computed(() => {
    const ids = new Set<string>();
    Object.values(sessionsByProject.value).forEach(items => {
      items.forEach(item => ids.add(item.id));
    });
    return ids;
  });

  function getSessions(projectId: string) {
    return sessionsByProject.value[projectId] ?? [];
  }

  function getActiveSessionId(projectId: string) {
    return activeSessionIdByProject.value[projectId] ?? '';
  }

  function hasStoredActiveSession(projectId: string) {
    return Object.prototype.hasOwnProperty.call(activeSessionIdByProject.value, projectId);
  }

  function getActiveSession(projectId: string) {
    const activeId = getActiveSessionId(projectId);
    return getSessions(projectId).find(item => item.id === activeId) ?? null;
  }

  function findSessionById(sessionId: string) {
    for (const sessions of Object.values(sessionsByProject.value)) {
      const matched = sessions.find(item => item.id === sessionId);
      if (matched) {
        return matched;
      }
    }
    return null;
  }

  function getLatestEventSeq(sessionId: string) {
    const events = eventsBySession.value[sessionId] ?? [];
    return events.length > 0 ? (events[events.length - 1]?.sq ?? 0) : 0;
  }

  function getDraftAttachments(projectId: string) {
    return draftAttachmentsByProject.value[projectId] ?? [];
  }

  function getPendingInputs(sessionId: string) {
    return pendingInputsBySession.value[sessionId] ?? [];
  }

  function getHistoryMeta(sessionId: string): HistoryMeta {
    return (
      historyBySession.value[sessionId] ?? {
        hasMore: false,
        beforeCursor: '',
        total: 0,
        loading: false,
      }
    );
  }

  function rememberActiveSession(projectId: string, sessionId: string) {
    activeSessionIdByProject.value = {
      ...activeSessionIdByProject.value,
      [projectId]: sessionId,
    };
    persistActiveSessions(activeSessionIdByProject.value);
  }

  function setActiveSession(projectId: string, sessionId: string) {
    if (!projectId) {
      return;
    }
    if (!sessionId) {
      activeSessionIdByProject.value = {
        ...activeSessionIdByProject.value,
        [projectId]: '',
      };
      return;
    }
    rememberActiveSession(projectId, sessionId);
  }

  function ensureSeenSet(sessionId: string) {
    let seen = seenSeqBySession.get(sessionId);
    if (!seen) {
      seen = new Set<number>();
      seenSeqBySession.set(sessionId, seen);
    }
    return seen;
  }

  function removeSnapshotWaiter(sessionId: string, waiterId: string) {
    const current = snapshotWaitersBySession.get(sessionId);
    if (!current?.length) {
      return;
    }
    const next = current.filter(waiter => waiter.id !== waiterId);
    if (next.length > 0) {
      snapshotWaitersBySession.set(sessionId, next);
    } else {
      snapshotWaitersBySession.delete(sessionId);
    }
  }

  function resolveSnapshotWaiters(sessionId: string, frame: WireFrame) {
    const waiters = snapshotWaitersBySession.get(sessionId);
    if (!waiters?.length) {
      return;
    }
    snapshotWaitersBySession.delete(sessionId);
    waiters.forEach(waiter => {
      if (waiter.timer != null) {
        window.clearTimeout(waiter.timer);
      }
      waiter.resolve(frame);
    });
  }

  function waitForNextSessionSnapshot(sessionId: string, timeoutMs = 4000) {
    const waiterId = `snap_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    let active = true;
    let rejectWaiter: ((reason?: unknown) => void) | null = null;

    const promise = new Promise<WireFrame>((resolve, reject) => {
      rejectWaiter = reason => {
        active = false;
        reject(reason);
      };
      const waiter: SessionSnapshotWaiter = {
        id: waiterId,
        resolve: frame => {
          active = false;
          resolve(frame);
        },
        reject: reason => rejectWaiter?.(reason),
        timer:
          timeoutMs > 0
            ? window.setTimeout(() => {
                removeSnapshotWaiter(sessionId, waiterId);
                waiter.reject(new Error(`Timed out waiting for session snapshot: ${sessionId}`));
              }, timeoutMs)
            : null,
      };
      const current = snapshotWaitersBySession.get(sessionId) ?? [];
      snapshotWaitersBySession.set(sessionId, [...current, waiter]);
    });

    return {
      promise,
      cancel(reason?: unknown) {
        if (!active) {
          return;
        }
        removeSnapshotWaiter(sessionId, waiterId);
        if (reason instanceof Error) {
          rejectWaiter?.(reason);
          return;
        }
        rejectWaiter?.(new Error(String(reason ?? `Cancelled snapshot wait for ${sessionId}`)));
      },
    };
  }

  function normalizeSession(session: WireSession): WebSessionSummary {
    return {
      id: session.id,
      projectId: session.pid,
      worktreeId: session.wid ?? null,
      orderIndex: Number(session.oi ?? 0),
      agent: session.ag,
      title: session.ttl,
      model: session.md,
      reasoningEffort: session.re ?? 'default',
      workflowMode: session.wm ?? 'default',
      permissionLevel: session.pl ?? 'elevated',
      cwd: session.cwd,
      nativeSessionId: session.nsid ?? null,
      status: session.st,
      hasUnread: session.unr,
      lastMessageAt: session.lma ? new Date(session.lma).toISOString() : null,
      createdAt: new Date(session.lu).toISOString(),
      updatedAt: new Date(session.lu).toISOString(),
      usage: {
        inputTokens: session.usa?.in ?? 0,
        cachedInputTokens: session.usa?.cin ?? 0,
        outputTokens: session.usa?.out ?? 0,
        cost: session.cost ?? 0,
      },
    };
  }

  function upsertSession(summary: WebSessionSummary) {
    const current = sessionsByProject.value[summary.projectId] ?? [];
    const next = [...current];
    const index = next.findIndex(item => item.id === summary.id);
    if (index >= 0) {
      next.splice(index, 1, {
        ...next[index],
        ...summary,
      });
    } else {
      next.unshift(summary);
    }
    sessionsByProject.value = {
      ...sessionsByProject.value,
      [summary.projectId]: sortSessions(next),
    };
  }

  function removeSession(projectId: string, sessionId: string) {
    const current = sessionsByProject.value[projectId] ?? [];
    const removed = current.find(item => item.id === sessionId) ?? null;
    const next = current.filter(item => item.id !== sessionId);
    sessionsByProject.value = {
      ...sessionsByProject.value,
      [projectId]: next,
    };
    const currentActive = activeSessionIdByProject.value[projectId];
    if (currentActive === sessionId) {
      const nextActive = next[0]?.id ?? '';
      activeSessionIdByProject.value = {
        ...activeSessionIdByProject.value,
        [projectId]: nextActive,
      };
      persistActiveSessions(activeSessionIdByProject.value);
    }
    const nextEvents = { ...eventsBySession.value };
    delete nextEvents[sessionId];
    eventsBySession.value = nextEvents;
    const nextHistory = { ...historyBySession.value };
    delete nextHistory[sessionId];
    historyBySession.value = nextHistory;
    const nextPendingInputs = { ...pendingInputsBySession.value };
    delete nextPendingInputs[sessionId];
    pendingInputsBySession.value = nextPendingInputs;
    seenSeqBySession.delete(sessionId);
    redirectAbortSessions.delete(sessionId);
    flushingSessions.delete(sessionId);
    const timer = pendingFlushTimers.get(sessionId);
    if (timer != null) {
      window.clearTimeout(timer);
      pendingFlushTimers.delete(sessionId);
    }
    if (removed) {
      emitter.emit('ai:closed', {
        sessionId: removed.id,
        sessionTitle: removed.title,
        projectId: removed.projectId,
        assistant: getAssistantDescriptor(removed),
      } satisfies WebSessionAIEvent);
    }
    const waiters = snapshotWaitersBySession.get(sessionId);
    if (waiters?.length) {
      snapshotWaitersBySession.delete(sessionId);
      waiters.forEach(waiter => {
        if (waiter.timer != null) {
          window.clearTimeout(waiter.timer);
        }
        waiter.reject(new Error(`Session removed before snapshot refresh completed: ${sessionId}`));
      });
    }
  }

  function setPendingInputs(sessionId: string, items: WebSessionPendingInput[]) {
    pendingInputsBySession.value = {
      ...pendingInputsBySession.value,
      [sessionId]: items,
    };
  }

  function enqueuePendingInput(
    sessionId: string,
    text: string,
    attachmentIds: string[],
    mode: 'redirect' | 'queue'
  ) {
    const item: WebSessionPendingInput = {
      id: `pending_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
      mode,
      text,
      attachmentIds: [...attachmentIds],
      createdAt: Date.now(),
    };
    setPendingInputs(sessionId, [...getPendingInputs(sessionId), item]);
    return item;
  }

  function removePendingInput(sessionId: string, pendingId: string) {
    setPendingInputs(
      sessionId,
      getPendingInputs(sessionId).filter(item => item.id !== pendingId)
    );
  }

  function schedulePendingFlush(sessionId: string, delay = 80) {
    const previous = pendingFlushTimers.get(sessionId);
    if (previous != null) {
      window.clearTimeout(previous);
    }
    const timer = window.setTimeout(() => {
      pendingFlushTimers.delete(sessionId);
      void flushPendingInput(sessionId);
    }, delay);
    pendingFlushTimers.set(sessionId, timer);
  }

  async function flushPendingInput(sessionId: string) {
    if (flushingSessions.has(sessionId)) {
      return;
    }
    const session = findSessionById(sessionId);
    if (!session || session.status === 'running') {
      return;
    }
    const items = getPendingInputs(sessionId);
    const next = items[0];
    if (!next) {
      return;
    }
    flushingSessions.add(sessionId);
    setPendingInputs(sessionId, items.slice(1));
    try {
      await sendCommand('send', sessionId, { txt: next.text, atts: next.attachmentIds });
    } catch (error) {
      setPendingInputs(sessionId, [next, ...getPendingInputs(sessionId)]);
      schedulePendingFlush(sessionId, 240);
      throw error;
    } finally {
      flushingSessions.delete(sessionId);
    }
  }

  function maybeAbortForRedirect(sessionId: string) {
    const session = findSessionById(sessionId);
    if (!session || session.status !== 'running' || redirectAbortSessions.has(sessionId)) {
      return;
    }
    const next = getPendingInputs(sessionId)[0];
    if (!next || next.mode !== 'redirect') {
      return;
    }
    redirectAbortSessions.add(sessionId);
    void abortSession(sessionId).catch(() => {
      redirectAbortSessions.delete(sessionId);
    });
  }

  function mergeEvents(sessionId: string, incoming: WireEvent[]) {
    const seen = ensureSeenSet(sessionId);
    const merged = [...(eventsBySession.value[sessionId] ?? [])];
    incoming.forEach(event => {
      if (!event || typeof event.sq !== 'number') {
        return;
      }
      if (seen.has(event.sq)) {
        return;
      }
      seen.add(event.sq);
      merged.push(event);
    });
    merged.sort((left, right) => left.sq - right.sq);
    eventsBySession.value = {
      ...eventsBySession.value,
      [sessionId]: merged,
    };
  }

  function resetSessionEvents(sessionId: string, events: WireEvent[]) {
    seenSeqBySession.set(
      sessionId,
      new Set(events.filter(event => typeof event.sq === 'number').map(event => event.sq))
    );
    eventsBySession.value = {
      ...eventsBySession.value,
      [sessionId]: [...events].sort((left, right) => left.sq - right.sq),
    };
  }

  function updateSessionStatus(
    sessionId: string,
    updater: (current: WebSessionSummary) => WebSessionSummary
  ) {
    const entries = Object.entries(sessionsByProject.value);
    let changed = false;
    const nextSessions: Record<string, WebSessionSummary[]> = {};
    entries.forEach(([projectId, sessions]) => {
      const nextProjectSessions = sessions.map(item => {
        if (item.id !== sessionId) {
          return item;
        }
        changed = true;
        return updater(item);
      });
      nextSessions[projectId] = sortSessions(nextProjectSessions);
    });
    if (changed) {
      sessionsByProject.value = nextSessions;
    }
  }

  function getAssistantDescriptor(session: WebSessionSummary): WebSessionAssistantDescriptor {
    return session.agent === 'claude'
      ? {
          type: 'claude-code',
          name: 'Claude Code',
          displayName: 'Claude Code',
        }
      : {
          type: 'codex',
          name: 'Codex',
          displayName: 'Codex',
        };
  }

  function emitStateTransition(
    sessionId: string,
    previousState: WebSessionLiveState,
    previousApproval: WebSessionApprovalState | null
  ) {
    const session = findSessionById(sessionId);
    if (!session) {
      return;
    }

    const nextState = getLiveState(sessionId);
    const nextApproval = getPendingApproval(sessionId);
    const baseEvent: WebSessionAIEvent = {
      sessionId,
      sessionTitle: session.title,
      projectId: session.projectId,
      assistant: getAssistantDescriptor(session),
    };

    if (isWorkingPhase(nextState.phase) && !isWorkingPhase(previousState.phase)) {
      emitter.emit('ai:working', baseEvent);
    }

    if (
      nextApproval &&
      (!previousApproval ||
        previousApproval.id !== nextApproval.id ||
        previousApproval.requestedAt !== nextApproval.requestedAt)
    ) {
      emitter.emit('ai:approval-needed', {
        ...baseEvent,
        approval: nextApproval,
      } satisfies WebSessionApprovalEvent);
    }

    if (nextState.phase === 'done' && previousState.phase !== 'done') {
      emitter.emit('ai:completed', baseEvent);
    }

    if (
      (nextState.phase === 'idle' || nextState.phase === 'error') &&
      nextState.phase !== previousState.phase
    ) {
      emitter.emit('ai:closed', baseEvent);
    }
  }

  function applyFrame(frame: WireFrame) {
    if (frame.k === 'err') {
      lastError.value = frame.msg ?? 'Unknown websocket error';
      if (frame.rid && pending.has(frame.rid)) {
        pending.get(frame.rid)?.reject(new Error(frame.msg ?? frame.code ?? 'unknown error'));
        pending.delete(frame.rid);
      }
      return;
    }

    if (frame.k === 'ack') {
      if (frame.rid && pending.has(frame.rid)) {
        pending.get(frame.rid)?.resolve(frame);
        pending.delete(frame.rid);
      }
      return;
    }

    if (frame.k === 'snap' && frame.sid && frame.s) {
      const summary = normalizeSession(frame.s);
      upsertSession(summary);
      resetSessionEvents(frame.sid, frame.h?.evs ?? []);
      historyBySession.value = {
        ...historyBySession.value,
        [frame.sid]: {
          hasMore: frame.h?.hm ?? false,
          beforeCursor: frame.h?.bc ?? '',
          total: frame.h?.tot ?? frame.h?.evs?.length ?? 0,
          loading: false,
        },
      };
      resolveSnapshotWaiters(frame.sid, frame);
      return;
    }

    if (frame.k === 'evt' && frame.sid && frame.e) {
      const previousState = getLiveState(frame.sid);
      const previousApproval = getPendingApproval(frame.sid);

      if (frame.e.tp === 'hist_ch') {
        const historicalEvents = Array.isArray(frame.e.p?.evs)
          ? (frame.e.p?.evs as WireEvent[])
          : [];
        mergeEvents(frame.sid, historicalEvents);
        historyBySession.value = {
          ...historyBySession.value,
          [frame.sid]: {
            ...getHistoryMeta(frame.sid),
            hasMore: Boolean(frame.e.p?.hm),
            beforeCursor: String(frame.e.p?.bc ?? ''),
            loading: false,
          },
        };
        return;
      }

      mergeEvents(frame.sid, [frame.e]);
      updateSessionStatus(frame.sid, current => {
        const next = { ...current };
        next.updatedAt = new Date(frame.e?.ts ?? Date.now()).toISOString();
        if (frame.e?.tp === 'run_st') {
          next.status = 'running';
        } else if (frame.e?.tp === 'run_done') {
          next.status = 'done';
        } else if (frame.e?.tp === 'run_fail') {
          next.status = 'err';
        } else if (frame.e?.tp === 'run_abort') {
          next.status = 'idle';
        } else if (frame.e?.tp === 'msg_u') {
          next.lastMessageAt = new Date(frame.e?.ts ?? Date.now()).toISOString();
        } else if (frame.e?.tp === 'usage') {
          next.usage = {
            inputTokens: Number(frame.e?.p?.in ?? next.usage.inputTokens),
            cachedInputTokens: Number(frame.e?.p?.cin ?? next.usage.cachedInputTokens),
            outputTokens: Number(frame.e?.p?.out ?? next.usage.outputTokens),
            cost: Number(frame.e?.p?.cost ?? next.usage.cost),
          };
        }
        return next;
      });

      if (frame.e.tp === 'tool_end') {
        maybeAbortForRedirect(frame.sid);
      }

      if (frame.e.tp === 'run_done' || frame.e.tp === 'run_fail' || frame.e.tp === 'run_abort') {
        redirectAbortSessions.delete(frame.sid);
        schedulePendingFlush(frame.sid);
      }

      emitStateTransition(frame.sid, previousState, previousApproval);
    }
  }

  function openSocket(): Promise<void> {
    if (socket && socket.readyState === WebSocket.OPEN) {
      connectionState.value = 'open';
      return Promise.resolve();
    }
    if (connectPromise) {
      return connectPromise;
    }
    connectionState.value = 'connecting';
    connectPromise = new Promise((resolve, reject) => {
      const ws = new WebSocket(resolveWsUrl(WS_PATH));
      ws.onopen = () => {
        socket = ws;
        connectionState.value = 'open';
        connectPromise = null;
        reconnectActiveSessions();
        resolve();
      };
      ws.onmessage = event => {
        try {
          const frame = JSON.parse(event.data) as WireFrame;
          applyFrame(frame);
        } catch (error) {
          console.error('[Web Session] Failed to parse websocket frame', error);
        }
      };
      ws.onerror = event => {
        console.error('[Web Session] websocket error', event);
      };
      ws.onclose = () => {
        socket = null;
        connectionState.value = 'closed';
        connectPromise = null;
        if (reconnectTimer != null) {
          window.clearTimeout(reconnectTimer);
        }
        reconnectTimer = window.setTimeout(() => {
          reconnectTimer = null;
          if (allSessionIds.value.size > 0) {
            void openSocket();
          }
        }, 1200);
      };
    });
    return connectPromise.catch(error => {
      connectPromise = null;
      connectionState.value = 'closed';
      throw error;
    });
  }

  async function sendCommand(op: string, sessionId: string, payload: Record<string, unknown> = {}) {
    await openSocket();
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      throw new Error('websocket is not connected');
    }
    const requestId = `ws_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    const frame = {
      v: 1,
      k: 'cmd',
      rid: requestId,
      sid: sessionId || undefined,
      op,
      p: payload,
    };
    const promise = new Promise<WireFrame>((resolve, reject) => {
      pending.set(requestId, { resolve, reject });
    });
    socket.send(JSON.stringify(frame));
    return promise;
  }

  function reconnectActiveSessions() {
    Object.entries(activeSessionIdByProject.value).forEach(([projectId, sessionId]) => {
      if (!projectId || !sessionId) {
        return;
      }
      void sendCommand('connect', sessionId, {}).catch(error => {
        console.warn('[Web Session] Failed to reconnect session', sessionId, error);
      });
    });
  }

  async function loadSessions(projectId: string, force = false) {
    if (!projectId) {
      return [];
    }
    if (!force && loadedProjects.value[projectId]) {
      return sessionsByProject.value[projectId] ?? [];
    }
    const sessions = await webSessionApi.list(projectId);
    sessionsByProject.value = {
      ...sessionsByProject.value,
      [projectId]: sortSessions(sessions),
    };
    loadedProjects.value = {
      ...loadedProjects.value,
      [projectId]: true,
    };
    if (!hasStoredActiveSession(projectId) && sessions[0]?.id) {
      rememberActiveSession(projectId, sessions[0].id);
    }
    return sessions;
  }

  async function ensureSessionConnected(projectId: string, sessionId: string) {
    if (!projectId || !sessionId) {
      return;
    }
    rememberActiveSession(projectId, sessionId);
    await sendCommand('connect', sessionId, {});
  }

  async function refreshSessionSnapshot(sessionId: string, timeoutMs = 4000) {
    if (!sessionId) {
      return null;
    }
    const waiter = waitForNextSessionSnapshot(sessionId, timeoutMs);
    try {
      await sendCommand('connect', sessionId, {});
      return await waiter.promise;
    } catch (error) {
      try {
        waiter.cancel(error);
      } catch {
        // waiter cancel rethrows for a single-path caller API; swallow here and
        // preserve the original failure below.
      }
      throw error;
    }
  }

  async function renameSession(projectId: string, sessionId: string, title: string) {
    await sendCommand('rename', sessionId, { ttl: title });
    rememberActiveSession(projectId, sessionId);
  }

  async function deleteSession(projectId: string, sessionId: string) {
    await sendCommand('del', sessionId, {});
    removeSession(projectId, sessionId);
  }

  async function sendMessage(
    sessionId: string,
    text: string,
    attachmentIds: string[],
    mode?: 'redirect' | 'queue'
  ) {
    const session = findSessionById(sessionId);
    if (session?.status === 'running' && mode) {
      enqueuePendingInput(sessionId, text, attachmentIds, mode);
      return;
    }
    await sendCommand('send', sessionId, { txt: text, atts: attachmentIds });
  }

  async function abortSession(sessionId: string) {
    await sendCommand('abort', sessionId, {});
  }

  async function approveSession(sessionId: string) {
    await sendCommand('approve', sessionId, {});
  }

  async function rejectSession(sessionId: string) {
    await sendCommand('reject', sessionId, {});
  }

  async function answerUserInput(
    sessionId: string,
    itemId: string,
    answers: Record<string, string[]>
  ) {
    await sendCommand('user_input', sessionId, { iid: itemId, ans: answers });
  }

  async function loadMoreHistory(sessionId: string, limit = 80) {
    const meta = getHistoryMeta(sessionId);
    if (meta.loading || !meta.hasMore || !meta.beforeCursor) {
      return;
    }
    historyBySession.value = {
      ...historyBySession.value,
      [sessionId]: {
        ...meta,
        loading: true,
      },
    };
    try {
      await sendCommand('hist', sessionId, {
        bc: meta.beforeCursor,
        lim: limit,
      });
    } catch (error) {
      historyBySession.value = {
        ...historyBySession.value,
        [sessionId]: {
          ...meta,
          loading: false,
        },
      };
      throw error;
    }
  }

  async function updateModel(sessionId: string, model: string) {
    await sendCommand('set_md', sessionId, { md: model });
  }

  async function updateReasoningEffort(
    sessionId: string,
    reasoningEffort: 'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh'
  ) {
    await sendCommand('set_re', sessionId, { re: reasoningEffort });
  }

  async function updateWorkflowMode(sessionId: string, workflowMode: 'default' | 'plan') {
    const session = findSessionById(sessionId);
    const previousWorkflowMode = session?.workflowMode;
    const shouldOptimisticallyUpdate = Boolean(session) && previousWorkflowMode !== workflowMode;

    if (shouldOptimisticallyUpdate) {
      updateSessionStatus(sessionId, current => ({
        ...current,
        workflowMode,
      }));
    }

    try {
      await sendCommand('set_wm', sessionId, { wm: workflowMode });
    } catch (error) {
      if (shouldOptimisticallyUpdate && previousWorkflowMode) {
        updateSessionStatus(sessionId, current => ({
          ...current,
          workflowMode: previousWorkflowMode,
        }));
      }
      throw error;
    }
  }

  async function updatePermissionLevel(
    sessionId: string,
    permissionLevel: 'default' | 'elevated' | 'yolo'
  ) {
    await sendCommand('set_pl', sessionId, { pl: permissionLevel });
  }

  async function updateAgent(sessionId: string, agent: 'claude' | 'codex') {
    await sendCommand('set_ag', sessionId, { ag: agent });
  }

  async function moveSession(projectId: string, fromIndex: number, toIndex: number) {
    const current = getSessions(projectId);
    if (
      !projectId ||
      fromIndex < 0 ||
      toIndex < 0 ||
      fromIndex >= current.length ||
      toIndex >= current.length ||
      fromIndex === toIndex
    ) {
      return;
    }

    const original = [...current];
    const reordered = [...current];
    const [moving] = reordered.splice(fromIndex, 1);
    if (!moving) {
      return;
    }
    reordered.splice(toIndex, 0, moving);
    const reorderedWithOrder = reordered.map((session, index) => ({
      ...session,
      orderIndex: (index + 1) * 1000,
    }));
    sessionsByProject.value = {
      ...sessionsByProject.value,
      [projectId]: reorderedWithOrder,
    };

    const prevSessionId = reorderedWithOrder[toIndex - 1]?.id ?? '';
    const nextSessionId = reorderedWithOrder[toIndex + 1]?.id ?? '';

    try {
      await sendCommand('move', moving.id, {
        prv: prevSessionId,
        nxt: nextSessionId,
      });
    } catch (error) {
      sessionsByProject.value = {
        ...sessionsByProject.value,
        [projectId]: sortSessions(original),
      };
      await loadSessions(projectId, true);
      throw error;
    }
  }

  async function uploadAttachment(projectId: string, file: File) {
    const attachment = await webSessionApi.uploadAttachment(projectId, file);
    const current = getDraftAttachments(projectId);
    draftAttachmentsByProject.value = {
      ...draftAttachmentsByProject.value,
      [projectId]: [...current, attachment],
    };
    return attachment;
  }

  function removeDraftAttachment(projectId: string, attachmentId: string) {
    draftAttachmentsByProject.value = {
      ...draftAttachmentsByProject.value,
      [projectId]: getDraftAttachments(projectId).filter(item => item.id !== attachmentId),
    };
  }

  function clearDraftAttachments(projectId: string) {
    draftAttachmentsByProject.value = {
      ...draftAttachmentsByProject.value,
      [projectId]: [],
    };
  }

  function buildBlocks(sessionId: string): WebSessionBlock[] {
    const blocks: WebSessionBlock[] = [];
    const toolIndex = new Map<string, number>();
    const lastAssistantBlockIndexByMessageId = new Map<string, number>();
    const userInputQuestionsByItemId = new Map<string, WebSessionUserInputQuestion[]>();
    let openAssistantMessageId = '';
    let openAssistantBlockIndex = -1;

    const closeAssistantTextSegment = () => {
      openAssistantMessageId = '';
      openAssistantBlockIndex = -1;
    };

    const appendBlock = (block: WebSessionBlock) => {
      blocks.push(block);
      return block;
    };

    const createAssistantTextBlock = (messageId: string, timestamp: number) => {
      const block = appendBlock({
        key: `assistant:${messageId}:${blocks.length}`,
        id: messageId,
        kind: 'assistant',
        text: '',
        timestamp,
        attachments: [],
        done: false,
      });
      openAssistantMessageId = messageId;
      openAssistantBlockIndex = blocks.length - 1;
      lastAssistantBlockIndexByMessageId.set(messageId, openAssistantBlockIndex);
      return block;
    };

    const resolveToolBlockId = (payload: Record<string, unknown>, fallback: string) =>
      parseToolCommandGroup(asRecord(payload.meta)?.commandGroup)?.id ?? fallback;

    const applyToolPayload = (
      tool: WebSessionToolBlock,
      payload: Record<string, unknown>,
      status: WebSessionToolBlock['status']
    ) => {
      tool.name = String(payload.name ?? tool.name ?? 'Tool');
      if (typeof payload.kind === 'string') {
        tool.kind = payload.kind;
      }
      if (Object.prototype.hasOwnProperty.call(payload, 'in')) {
        tool.input = payload.in;
      }
      if (Object.prototype.hasOwnProperty.call(payload, 'out')) {
        tool.output = String(payload.out ?? '');
      }
      tool.status = status;
      const meta = asRecord(payload.meta);
      if (meta) {
        tool.meta = meta;
      }
      tool.commandGroup = parseToolCommandGroup(meta?.commandGroup);
    };

    const ensureToolBlock = (
      toolId: string,
      timestamp: number,
      payload: Record<string, unknown>,
      initialStatus: WebSessionToolBlock['status']
    ) => {
      const blockId = resolveToolBlockId(payload, toolId);
      const existingIndex = toolIndex.get(blockId);
      if (existingIndex != null) {
        blocks[existingIndex].timestamp = timestamp;
        return blocks[existingIndex];
      }

      const block = appendBlock({
        key: `tool:${blockId}`,
        id: blockId,
        kind: 'tool',
        text: '',
        timestamp,
        attachments: [],
        tool: {
          id: blockId,
          name: String(payload.name ?? 'Tool'),
          kind: typeof payload.kind === 'string' ? payload.kind : undefined,
          input: payload.in,
          output: typeof payload.out === 'string' ? payload.out : undefined,
          meta: asRecord(payload.meta),
          status: initialStatus,
          commandGroup: parseToolCommandGroup(asRecord(payload.meta)?.commandGroup),
        },
      });
      toolIndex.set(blockId, blocks.length - 1);
      return block;
    };

    (eventsBySession.value[sessionId] ?? []).forEach(event => {
      const payload = event.p ?? {};
      if (event.tp !== 'txt_d') {
        closeAssistantTextSegment();
      }

      switch (event.tp) {
        case 'msg_u': {
          const mid = String(payload.mid ?? event.id);
          appendBlock({
            key: `user:${mid}`,
            id: mid,
            kind: 'user',
            text: String(payload.txt ?? ''),
            timestamp: event.ts,
            attachments: Array.isArray(payload.atts)
              ? payload.atts.map((item: Record<string, unknown>) => ({
                  id: String(item.id ?? ''),
                  name: String(item.name ?? ''),
                  mime: typeof item.mime === 'string' ? item.mime : undefined,
                  size: typeof item.sz === 'number' ? item.sz : undefined,
                }))
              : [],
          });
          break;
        }
        case 'msg_a_st': {
          break;
        }
        case 'txt_d': {
          const mid = String(payload.mid ?? event.pid2 ?? event.id);
          const block =
            openAssistantMessageId === mid && openAssistantBlockIndex >= 0
              ? blocks[openAssistantBlockIndex]
              : createAssistantTextBlock(mid, event.ts);
          block.text += String(payload.txt ?? '');
          break;
        }
        case 'txt_end': {
          const mid = String(payload.mid ?? event.pid2 ?? event.id);
          const blockIndex = lastAssistantBlockIndexByMessageId.get(mid);
          if (blockIndex != null) {
            blocks[blockIndex].done = true;
          }
          break;
        }
        case 'tool_st': {
          const toolId = String(payload.tid ?? event.id);
          const block = ensureToolBlock(toolId, event.ts, payload, 'running');
          if (block.tool) {
            applyToolPayload(block.tool, payload, 'running');
          }
          break;
        }
        case 'tool_end': {
          const toolId = String(payload.tid ?? event.id);
          const block = ensureToolBlock(
            toolId,
            event.ts,
            payload,
            payload.ok === false ? 'error' : 'done'
          );
          const tool = block.tool;
          if (!tool) {
            break;
          }
          applyToolPayload(tool, payload, payload.ok === false ? 'error' : 'done');
          break;
        }
        case 'approval_req': {
          const prompt = String(payload.prompt ?? 'Approval required');
          blocks.push({
            key: `approval:${event.id}`,
            id: event.id,
            kind: 'system',
            text: prompt,
            timestamp: event.ts,
            attachments: [],
            level: 'warn',
            detail: {
              type: 'approval_request',
              prompt,
            },
          });
          break;
        }
        case 'approval_res': {
          const action = String(payload.act ?? 'approve');
          const prompt = String(payload.prompt ?? '');
          blocks.push({
            key: `approval-res:${event.id}`,
            id: event.id,
            kind: 'system',
            text: action === 'reject' ? 'Approval rejected' : 'Approval granted',
            timestamp: event.ts,
            attachments: [],
            level: action === 'reject' ? 'warn' : 'info',
            detail: {
              type: 'approval_response',
              prompt,
              action,
            },
          });
          break;
        }
        case 'user_input_req': {
          const itemId = String(payload.iid ?? '');
          const questions = parseUserInputQuestions(payload.qs);
          if (itemId) {
            userInputQuestionsByItemId.set(itemId, questions);
          }
          blocks.push({
            key: `user-input:${event.id}`,
            id: event.id,
            kind: 'system',
            text: summarizeUserInputPrompt(payload),
            timestamp: event.ts,
            attachments: [],
            level: 'warn',
            detail: {
              type: 'user_input_request',
              prompt: summarizeUserInputPrompt(payload),
              questions,
            },
          });
          break;
        }
        case 'user_input_res': {
          const itemId = String(payload.iid ?? '');
          const questions = itemId ? (userInputQuestionsByItemId.get(itemId) ?? []) : [];
          blocks.push({
            key: `user-input-res:${event.id}`,
            id: event.id,
            kind: 'system',
            text: summarizeUserInputAnswer(payload),
            timestamp: event.ts,
            attachments: [],
            level: 'info',
            detail: {
              type: 'user_input_response',
              answers: buildUserInputAnswerEntries(payload, questions),
            },
          });
          break;
        }
        case 'note': {
          blocks.push({
            key: `note:${event.id}`,
            id: event.id,
            kind: 'system',
            text: String(payload.txt ?? ''),
            timestamp: event.ts,
            attachments: [],
            level: payload.lvl === 'warn' ? 'warn' : payload.lvl === 'error' ? 'error' : 'info',
          });
          break;
        }
        case 'run_fail': {
          blocks.push({
            key: `fail:${event.id}`,
            id: event.id,
            kind: 'system',
            text: String(payload.msg ?? 'Run failed'),
            timestamp: event.ts,
            attachments: [],
            level: 'error',
          });
          break;
        }
        case 'run_abort': {
          const abortedByRestart = isProcessRestartPayload(payload);
          blocks.push({
            key: `abort:${event.id}`,
            id: event.id,
            kind: 'system',
            text: abortedByRestart ? getRecoveryMessage(payload) : 'Run aborted',
            timestamp: event.ts,
            attachments: [],
            level: abortedByRestart ? 'warn' : 'info',
          });
          break;
        }
      }
    });

    return blocks;
  }

  const getBlocks = (sessionId: string) => buildBlocks(sessionId);

  function getPendingApproval(sessionId: string): WebSessionApprovalState | null {
    let pending: WebSessionApprovalState | null = null;
    for (const event of eventsBySession.value[sessionId] ?? []) {
      const payload = event.p ?? {};
      switch (event.tp) {
        case 'approval_req':
          pending = {
            id: event.id,
            prompt: String(payload.prompt ?? ''),
            requestedAt: event.ts,
            stale: false,
          };
          break;
        case 'msg_u':
        case 'run_st':
        case 'approval_res':
        case 'run_done':
        case 'run_fail':
          pending = null;
          break;
        case 'run_abort':
          if (pending && isProcessRestartPayload(payload)) {
            const activeRequest: WebSessionApprovalState = pending;
            pending = {
              ...activeRequest,
              stale: true,
              recoveryReason: String(payload.reason ?? ''),
              recoveryMessage: getRecoveryMessage(payload),
            };
            break;
          }
          pending = null;
          break;
      }
    }
    return pending;
  }

  function getPendingUserInput(sessionId: string): WebSessionUserInputState | null {
    let pending: WebSessionUserInputState | null = null;
    for (const event of eventsBySession.value[sessionId] ?? []) {
      const payload = event.p ?? {};
      switch (event.tp) {
        case 'user_input_req':
          pending = {
            id: event.id,
            itemId: String(payload.iid ?? ''),
            prompt: summarizeUserInputPrompt(payload),
            questions: parseUserInputQuestions(payload.qs),
            requestedAt: event.ts,
            stale: false,
          };
          break;
        case 'msg_u':
        case 'run_st':
        case 'user_input_res':
        case 'run_done':
        case 'run_fail':
          pending = null;
          break;
        case 'run_abort':
          if (pending && isProcessRestartPayload(payload)) {
            const activeRequest: WebSessionUserInputState = pending;
            pending = {
              ...activeRequest,
              stale: true,
              recoveryReason: String(payload.reason ?? ''),
              recoveryMessage: getRecoveryMessage(payload),
            };
            break;
          }
          pending = null;
          break;
      }
    }
    return pending;
  }

  function getLiveState(sessionId: string): WebSessionLiveState {
    const session = findSessionById(sessionId);
    const approval = getPendingApproval(sessionId);
    const userInput = getPendingUserInput(sessionId);
    let activeTool:
      | {
          id: string;
          name: string;
          kind?: string;
        }
      | undefined;
    let sawAssistantOutput = false;
    let assistantDone = false;
    let errorMessage = '';
    let updatedAt = session ? Date.parse(session.updatedAt) || Date.now() : Date.now();

    for (const event of eventsBySession.value[sessionId] ?? []) {
      const payload = event.p ?? {};
      updatedAt = event.ts;
      switch (event.tp) {
        case 'msg_a_st':
        case 'txt_d':
          sawAssistantOutput = true;
          assistantDone = false;
          break;
        case 'txt_end':
          assistantDone = true;
          break;
        case 'tool_st':
          if (payload.kind === 'reasoning') {
            break;
          }
          activeTool = {
            id: String(payload.tid ?? event.id),
            name: String(payload.name ?? 'Tool'),
            kind: typeof payload.kind === 'string' ? payload.kind : undefined,
          };
          break;
        case 'tool_end': {
          if (payload.kind === 'reasoning') {
            break;
          }
          const toolId = String(payload.tid ?? event.id);
          if (activeTool?.id === toolId) {
            activeTool = undefined;
          }
          break;
        }
        case 'run_fail':
          errorMessage = String(payload.msg ?? 'Run failed');
          break;
      }
    }

    if (approval && !approval.stale && session?.status === 'running') {
      return {
        phase: 'waiting_approval',
        running: true,
        updatedAt: approval.requestedAt,
        approval,
        tool: activeTool,
      };
    }

    if (userInput && !userInput.stale && session?.status === 'running') {
      return {
        phase: 'waiting_input',
        running: true,
        updatedAt: userInput.requestedAt,
        tool: activeTool,
        userInput,
      };
    }

    if (session?.status === 'running') {
      if (activeTool) {
        return {
          phase: 'tool',
          running: true,
          updatedAt,
          tool: activeTool,
        };
      }
      if (sawAssistantOutput && !assistantDone) {
        return {
          phase: 'thinking',
          running: true,
          updatedAt,
        };
      }
      return {
        phase: 'starting',
        running: true,
        updatedAt,
      };
    }

    if (session?.status === 'done') {
      return {
        phase: 'done',
        running: false,
        updatedAt,
      };
    }

    if (session?.status === 'err') {
      return {
        phase: 'error',
        running: false,
        updatedAt,
        errorMessage,
      };
    }

    return {
      phase: 'idle',
      running: false,
      updatedAt,
    };
  }

  async function createSessionViaHttp(
    projectId: string,
    payload: {
      worktreeId?: string;
      agent: 'claude' | 'codex';
      model?: string;
      reasoningEffort?: 'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh';
      workflowMode?: 'default' | 'plan';
      permissionLevel?: 'default' | 'elevated' | 'yolo';
      title?: string;
    }
  ) {
    const session = await webSessionApi.create(projectId, payload);
    upsertSession(session);
    rememberActiveSession(projectId, session.id);
    emitter.emit('web-session:created', {
      projectId,
      sessionId: session.id,
    });
    try {
      await ensureSessionConnected(projectId, session.id);
    } catch (error) {
      console.warn('[Web Session] Failed to connect new session after creation', error);
    }
    return session;
  }

  return {
    connectionState,
    lastError,
    getSessions,
    getActiveSessionId,
    hasStoredActiveSession,
    getActiveSession,
    getDraftAttachments,
    getPendingInputs,
    getHistoryMeta,
    getBlocks,
    getLatestEventSeq,
    loadSessions,
    setActiveSession,
    ensureSessionConnected,
    refreshSessionSnapshot,
    createSession: createSessionViaHttp,
    renameSession,
    deleteSession,
    sendMessage,
    abortSession,
    approveSession,
    rejectSession,
    answerUserInput,
    loadMoreHistory,
    updateModel,
    updateReasoningEffort,
    updateWorkflowMode,
    updatePermissionLevel,
    updateAgent,
    moveSession,
    getPendingApproval,
    getPendingUserInput,
    getLiveState,
    uploadAttachment,
    removeDraftAttachment,
    removePendingInput,
    clearDraftAttachments,
    openSocket,
    emitter,
  };
});
