import EventEmitter from 'eventemitter3';
import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { webSessionApi, type WebSessionAttachmentUploadProgress } from '@/api/webSession';
import type {
  WebSessionAttachment,
  WebSessionContextWindowSource,
  WebSessionSummary,
} from '@/types/models';
import {
  buildWebSessionSnapshotVersion,
  compareWebSessionSnapshotVersion,
  selectLatestWebSessionSnapshotVersion,
  shouldApplyIncomingWebSessionSnapshot,
  type WebSessionSnapshotVersion,
  type WebSessionSnapshotVersionInput,
} from '@/stores/webSessionSnapshotVersion';
import { buildUploadImageFileName } from '@/utils/webSessionImages';
import { resolveWsUrl } from '@/utils/ws';

type WireFrameKind = 'ack' | 'snap' | 'evt' | 'err';
type SessionStatus = WebSessionSummary['status'];
type SessionAssistantState =
  | 'working'
  | 'waiting_approval'
  | 'waiting_input'
  | 'waiting_plan_approval';

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
  ast?: SessionAssistantState | null;
  unr: boolean;
  aa?: number | null;
  act?: number | null;
  ca?: number | null;
  lu: number;
  lma?: number | null;
  asu?: number | null;
  sk: string;
  ss: 'fresh' | 'stale' | 'missing' | 'syncing' | 'error';
  lsm?: 'fast' | 'deep';
  sca?: number | null;
  sua?: number | null;
  lsa?: number | null;
  tp?: string | null;
  tpv?: string | null;
  tc?: number;
  ic?: number;
  se?: string | null;
  usa?: {
    in?: number;
    cin?: number;
    out?: number;
  };
  cea?: {
    in?: number;
    cin?: number;
    out?: number;
    usd?: number;
  };
  cem?: 'cumulative_total' | 'since_compaction';
  lcca?: number | null;
  cost?: number;
  cwt?: number | null;
  cws?: WebSessionContextWindowSource;
};

type WireHistoryItem = {
  id: string;
  stid?: string | null;
  siid?: string | null;
  oi: number;
  kd: 'user' | 'assistant' | 'system' | 'tool';
  tp: string;
  txt?: string;
  ts2?: number | null;
  obs?: number | null;
  atts?: Array<{
    id: string;
    name: string;
    mime?: string;
    sz?: number;
    path?: string;
  }>;
  tl?: {
    id: string;
    name: string;
    kind?: string;
    in?: unknown;
    out?: string;
    st: 'running' | 'done' | 'error' | string;
    meta?: Record<string, unknown>;
    cg?: {
      id: string;
      count: number;
      firstSeq?: number;
      lastSeq?: number;
      latestToolId?: string;
      compacted?: boolean;
    };
  } | null;
  lvl?: 'info' | 'warn' | 'error' | string;
  dn?: boolean;
  dt?: {
    type: 'approval_request' | 'approval_response' | 'user_input_request' | 'user_input_response';
    prompt?: string;
    questions?: WebSessionUserInputQuestion[];
    answers?: WebSessionHistoryAnswerEntry[];
    action?: 'approve' | 'reject' | string;
  } | null;
  pl?: Record<string, unknown>;
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
    its: WireHistoryItem[];
    hm: boolean;
    bc?: string;
    tot: number;
  };
  i?: WireHistoryItem;
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
  startedAt?: number;
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
  sourceTurnId?: string | null;
  sourceItemId?: string | null;
  orderIndex: number;
  kind: 'user' | 'assistant' | 'system' | 'tool';
  itemType: string;
  text: string;
  timestamp: number;
  observedAt?: number | null;
  attachments: Array<{
    id: string;
    name: string;
    mime?: string;
    size?: number;
    path?: string;
  }>;
  tool?: WebSessionToolBlock;
  level?: 'info' | 'warn' | 'error';
  done?: boolean;
  detail?: WebSessionHistoryDetail;
  payload?: Record<string, unknown>;
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
    | 'retrying'
    | 'tool'
    | 'waiting_approval'
    | 'waiting_plan_approval'
    | 'waiting_input'
    | 'done'
    | 'error';
  running: boolean;
  updatedAt: number;
  startedAt?: number;
  tool?: {
    id: string;
    name: string;
    kind?: string;
    summary?: string;
    count?: number;
    groupId?: string;
    startedAt?: number;
  };
  approval?: WebSessionApprovalState | null;
  userInput?: WebSessionUserInputState | null;
  errorMessage?: string;
  retry?: {
    code: string;
    message: string;
    attempt?: number;
    maxAttempts?: number;
  };
}

export interface WebSessionPendingInput {
  id: string;
  mode: 'redirect' | 'queue';
  text: string;
  attachmentIds: string[];
  createdAt: number;
}

export interface WebSessionDraftState {
  text: string;
  attachments: WebSessionAttachment[];
  updatedAt: number;
}

export interface WebSessionDraftAttachmentUploadState {
  id: string;
  fileName: string;
  currentFileIndex: number;
  totalFiles: number;
  loaded: number;
  total?: number;
  percent: number | null;
}

export interface WebSessionDraftAttachmentUploadError {
  fileName: string;
  message: string;
}

export interface WebSessionDraftAttachmentUploadBatchResult {
  attachments: WebSessionAttachment[];
  errors: WebSessionDraftAttachmentUploadError[];
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

type ArchivedListMeta = {
  scopeKey: string;
  total: number;
  offset: number;
  hasMore: boolean;
  loading: boolean;
};

type SyncSessionOptions = {
  rememberActive?: boolean;
};

type LoadSessionSnapshotOptions = {
  rememberActive?: boolean;
};

const ACTIVE_SESSION_STORAGE_KEY = 'kanban-web-active-session';
const SESSION_DRAFT_STORAGE_KEY = 'kanban-web-session-drafts';
const COMMAND_WS_PATH = '/api/v1/web-sessions/ws';
const EVENTS_WS_PATH = '/api/v1/web-sessions/events';
const PROCESS_RESTART_REASON = 'process_restart';
const DEFAULT_RECOVERY_MESSAGE =
  'The previous run was interrupted because the app restarted. Send a new message to continue.';

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value));
}

function normalizeStoredAttachment(value: unknown): WebSessionAttachment | null {
  if (!isRecord(value)) {
    return null;
  }
  const id = typeof value.id === 'string' ? value.id.trim() : '';
  const name = typeof value.name === 'string' ? value.name.trim() : '';
  if (!id || !name) {
    return null;
  }
  return {
    id,
    name,
    mime: typeof value.mime === 'string' ? value.mime : '',
    size: typeof value.size === 'number' && Number.isFinite(value.size) ? value.size : 0,
    path: typeof value.path === 'string' ? value.path : '',
    createdAt: typeof value.createdAt === 'string' ? value.createdAt : '',
  };
}

function normalizeStoredDrafts(
  value: unknown
): Record<string, Record<string, WebSessionDraftState>> {
  if (!isRecord(value)) {
    return {};
  }
  const result: Record<string, Record<string, WebSessionDraftState>> = {};
  Object.entries(value).forEach(([projectId, projectValue]) => {
    if (!projectId.trim() || !isRecord(projectValue)) {
      return;
    }
    const projectDrafts: Record<string, WebSessionDraftState> = {};
    Object.entries(projectValue).forEach(([sessionId, draftValue]) => {
      if (!sessionId.trim() || !isRecord(draftValue)) {
        return;
      }
      const text = typeof draftValue.text === 'string' ? draftValue.text : '';
      const attachments = Array.isArray(draftValue.attachments)
        ? draftValue.attachments
            .map(item => normalizeStoredAttachment(item))
            .filter((item): item is WebSessionAttachment => Boolean(item))
        : [];
      if (!text.trim() && attachments.length === 0) {
        return;
      }
      projectDrafts[sessionId] = {
        text,
        attachments,
        updatedAt:
          typeof draftValue.updatedAt === 'number' && Number.isFinite(draftValue.updatedAt)
            ? draftValue.updatedAt
            : Date.now(),
      };
    });
    if (Object.keys(projectDrafts).length > 0) {
      result[projectId] = projectDrafts;
    }
  });
  return result;
}

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

function loadStoredSessionDrafts() {
  try {
    const raw = localStorage.getItem(SESSION_DRAFT_STORAGE_KEY);
    if (!raw) {
      return {};
    }
    return normalizeStoredDrafts(JSON.parse(raw));
  } catch {
    return {};
  }
}

function persistSessionDrafts(value: Record<string, Record<string, WebSessionDraftState>>) {
  try {
    const persisted = normalizeStoredDrafts(value);
    if (Object.keys(persisted).length === 0) {
      localStorage.removeItem(SESSION_DRAFT_STORAGE_KEY);
      return;
    }
    localStorage.setItem(SESSION_DRAFT_STORAGE_KEY, JSON.stringify(persisted));
  } catch (error) {
    console.warn('[Web Session] Failed to persist session drafts', error);
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

function normalizeAssistantStateValue(value: unknown): SessionAssistantState | '' {
  switch (String(value ?? '').trim()) {
    case 'working':
    case 'waiting_approval':
    case 'waiting_input':
    case 'waiting_plan_approval':
      return String(value).trim() as SessionAssistantState;
    default:
      return '';
  }
}

function getSessionAssistantStateValue(
  session?: WebSessionSummary | null
): SessionAssistantState | '' {
  if (!session) {
    return '';
  }
  return normalizeAssistantStateValue(session.assistantState);
}

function getAssistantStateUpdatedAt(session?: WebSessionSummary | null) {
  if (!session) {
    return undefined;
  }
  if (session.assistantStateUpdatedAt) {
    const parsed = Date.parse(session.assistantStateUpdatedAt);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return undefined;
}

function isWorkingPhase(phase: WebSessionLiveState['phase']) {
  return phase === 'starting' || phase === 'thinking' || phase === 'retrying' || phase === 'tool';
}

function isProcessRestartPayload(payload?: Record<string, unknown>) {
  return String(payload?.reason ?? '') === PROCESS_RESTART_REASON;
}

function getRecoveryMessage(payload?: Record<string, unknown>) {
  const message = typeof payload?.msg === 'string' ? payload.msg.trim() : '';
  return message || DEFAULT_RECOVERY_MESSAGE;
}

function normalizeHistorySourceItemId(
  record: Record<string, unknown>,
  payload?: Record<string, unknown>
) {
  if (typeof record.siid === 'string' && record.siid.trim()) {
    return record.siid;
  }
  if (typeof record.sourceItemId === 'string' && record.sourceItemId.trim()) {
    return record.sourceItemId;
  }
  if (typeof payload?.iid === 'string' && payload.iid.trim()) {
    return payload.iid;
  }
  return null;
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return undefined;
  }
  return value as Record<string, unknown>;
}

function parseHistoryTimeValue(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string') {
    const parsed = Date.parse(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
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

function normalizeToolKindValue(value: unknown) {
  const normalized = String(value ?? '').trim();
  if (normalized === 'commandExecution') {
    return 'command_execution';
  }
  if (normalized === 'mcpToolCall') {
    return 'mcp_tool_call';
  }
  if (normalized === 'fileChange') {
    return 'file_change';
  }
  if (normalized === 'webSearch') {
    return 'web_search';
  }
  return normalized;
}

function extractToolSummary(payload: Record<string, unknown>) {
  const kind = normalizeToolKindValue(payload.kind ?? asRecord(payload.meta)?.kind);
  const input = asRecord(payload.in);
  const meta = asRecord(payload.meta);
  const subtitle = String(meta?.subtitle ?? '').trim();

  if (kind === 'command_execution') {
    const command = String(input?.command ?? '').trim();
    return command || subtitle;
  }

  if (kind === 'file_change') {
    const path =
      String(input?.path ?? input?.file_path ?? input?.new_path ?? input?.old_path ?? '').trim() ||
      subtitle;
    if (path) {
      return path;
    }
    const changes = Array.isArray(input?.changes) ? input.changes.length : 0;
    return changes > 0 ? `${changes} change${changes > 1 ? 's' : ''}` : '';
  }

  if (kind === 'mcp_tool_call') {
    const toolName = String(input?.tool_name ?? input?.name ?? '').trim();
    const args = asRecord(input?.arguments);
    const target =
      String(
        args?.url ??
          args?.query ??
          args?.path ??
          args?.file ??
          args?.name ??
          args?.id ??
          input?.server ??
          input?.path ??
          ''
      ).trim() || subtitle;
    if (toolName && target && toolName !== target) {
      return `${toolName} · ${target}`;
    }
    return toolName || target;
  }

  if (kind === 'web_search') {
    const query = String(input?.query ?? '').trim();
    if (query) {
      return query;
    }
    const action = asRecord(input?.action);
    const queries = Array.isArray(action?.queries)
      ? action?.queries
          .map(value => String(value ?? '').trim())
          .filter((value): value is string => Boolean(value))
      : [];
    return queries[0] ?? subtitle;
  }

  return subtitle;
}

function getTransportRetryPayload(payload?: Record<string, unknown>) {
  if (!payload || String(payload.code ?? '').trim() !== 'transport_retrying') {
    return null;
  }
  const attempt =
    typeof payload.attempt === 'number' && Number.isFinite(payload.attempt) && payload.attempt > 0
      ? Math.trunc(payload.attempt)
      : undefined;
  const maxAttempts =
    typeof payload.maxAttempts === 'number' &&
    Number.isFinite(payload.maxAttempts) &&
    payload.maxAttempts > 0
      ? Math.trunc(payload.maxAttempts)
      : undefined;
  return {
    code: 'transport_retrying',
    message: String(payload.txt ?? '').trim(),
    attempt,
    maxAttempts,
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

function normalizeProjectScope(projectIds: string[]) {
  const ids = Array.from(
    new Set(projectIds.map(projectId => String(projectId || '').trim()).filter(Boolean))
  ).sort((left, right) => left.localeCompare(right));
  return {
    ids,
    key: ids.join('::'),
  };
}

function defaultArchivedListMeta(): ArchivedListMeta {
  return {
    scopeKey: '',
    total: 0,
    offset: 0,
    hasMore: false,
    loading: false,
  };
}

export const useWebSessionStore = defineStore('web-session', () => {
  const sessionsByProject = ref<Record<string, WebSessionSummary[]>>({});
  const archivedSessionsById = ref<Record<string, WebSessionSummary>>({});
  const archivedSessionIds = ref<string[]>([]);
  const archivedListMeta = ref<ArchivedListMeta>(defaultArchivedListMeta());
  const eventsBySession = ref<Record<string, WebSessionBlock[]>>({});
  const historyBySession = ref<Record<string, HistoryMeta>>({});
  const draftStateByProject =
    ref<Record<string, Record<string, WebSessionDraftState>>>(loadStoredSessionDrafts());
  const draftAttachmentUploadsByProject = ref<
    Record<string, Record<string, WebSessionDraftAttachmentUploadState>>
  >({});
  const pendingInputsBySession = ref<Record<string, WebSessionPendingInput[]>>({});
  const activeSessionIdByProject = ref<Record<string, string>>(loadStoredActiveSessions());
  const loadedProjects = ref<Record<string, boolean>>({});
  const emitter = new EventEmitter();

  const connectionState = ref<'idle' | 'connecting' | 'open' | 'closed'>('idle');
  const lastError = ref<string | null>(null);

  let eventSocket: WebSocket | null = null;
  let eventConnectPromise: Promise<void> | null = null;
  let eventReconnectTimer: number | null = null;
  let commandSocket: WebSocket | null = null;
  let commandConnectPromise: Promise<void> | null = null;
  const pending = new Map<
    string,
    {
      resolve: (value: WireFrame) => void;
      reject: (reason?: unknown) => void;
    }
  >();
  const redirectAbortSessions = new Set<string>();
  const pendingFlushTimers = new Map<string, number>();
  const flushingSessions = new Set<string>();
  const draftAttachmentUploadQueues = new Map<string, Promise<unknown>>();
  const appliedSnapshotVersionBySession = new Map<string, WebSessionSnapshotVersion>();
  const completedTransitionVersionBySession = new Map<string, number>();
  let draftAttachmentUploadSeed = 0;

  const allSessionIds = computed(() => {
    const ids = new Set<string>();
    Object.values(sessionsByProject.value).forEach(items => {
      items.forEach(item => ids.add(item.id));
    });
    archivedSessionIds.value.forEach(sessionId => ids.add(sessionId));
    return ids;
  });

  function getSessions(projectId: string) {
    return sessionsByProject.value[projectId] ?? [];
  }

  function getArchivedSessions(projectIds: string[]) {
    const scope = normalizeProjectScope(projectIds);
    if (!scope.key || archivedListMeta.value.scopeKey !== scope.key) {
      return [];
    }
    return archivedSessionIds.value
      .map(sessionId => archivedSessionsById.value[sessionId])
      .filter((session): session is WebSessionSummary => Boolean(session));
  }

  function getArchivedMeta(projectIds: string[]): ArchivedListMeta {
    const scope = normalizeProjectScope(projectIds);
    if (!scope.key || archivedListMeta.value.scopeKey !== scope.key) {
      return defaultArchivedListMeta();
    }
    return archivedListMeta.value;
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
    const archived = archivedSessionsById.value[sessionId];
    if (archived) {
      return archived;
    }
    return null;
  }

  function getLatestEventSeq(sessionId: string) {
    const events = eventsBySession.value[sessionId] ?? [];
    return events.length > 0 ? (events[events.length - 1]?.orderIndex ?? 0) : 0;
  }

  function getDraft(projectId: string, sessionId: string): WebSessionDraftState {
    const normalizedProjectId = String(projectId || '').trim();
    const normalizedSessionId = String(sessionId || '').trim();
    if (!normalizedProjectId || !normalizedSessionId) {
      return {
        text: '',
        attachments: [],
        updatedAt: 0,
      };
    }
    return (
      draftStateByProject.value[normalizedProjectId]?.[normalizedSessionId] ?? {
        text: '',
        attachments: [],
        updatedAt: 0,
      }
    );
  }

  function getDraftAttachments(projectId: string, sessionId: string) {
    return getDraft(projectId, sessionId).attachments;
  }

  function getDraftAttachmentUpload(projectId: string, sessionId: string) {
    const normalizedProjectId = String(projectId || '').trim();
    const normalizedSessionId = String(sessionId || '').trim();
    if (!normalizedProjectId || !normalizedSessionId) {
      return null;
    }
    return (
      draftAttachmentUploadsByProject.value[normalizedProjectId]?.[normalizedSessionId] ?? null
    );
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

  function setHistoryLoading(sessionId: string, loading: boolean) {
    historyBySession.value = {
      ...historyBySession.value,
      [sessionId]: {
        ...getHistoryMeta(sessionId),
        loading,
      },
    };
  }

  function currentSnapshotVersionInput(sessionId: string): WebSessionSnapshotVersionInput | null {
    const session = findSessionById(sessionId);
    if (!session) {
      return null;
    }
    return {
      session,
      historyTotal: getHistoryMeta(sessionId).total,
    };
  }

  function rememberAppliedSnapshotVersion(
    sessionId: string,
    snapshot: WebSessionSnapshotVersionInput
  ) {
    const nextVersion = buildWebSessionSnapshotVersion(snapshot);
    const currentVersion = appliedSnapshotVersionBySession.get(sessionId) ?? null;
    if (!currentVersion || compareWebSessionSnapshotVersion(nextVersion, currentVersion) >= 0) {
      appliedSnapshotVersionBySession.set(sessionId, nextVersion);
    }
  }

  function applySessionSnapshot(
    sessionId: string,
    summary: WebSessionSummary,
    items: WebSessionBlock[],
    history: {
      hasMore: boolean;
      beforeCursor?: string;
      total: number;
    }
  ) {
    upsertSession(summary);
    resetSessionEvents(sessionId, items);
    historyBySession.value = {
      ...historyBySession.value,
      [sessionId]: {
        hasMore: Boolean(history.hasMore),
        beforeCursor: String(history.beforeCursor ?? ''),
        total: Number(history.total ?? 0),
        loading: false,
      },
    };
    rememberAppliedSnapshotVersion(sessionId, {
      session: summary,
      historyTotal: history.total,
    });
    if (summary.status === 'done') {
      completedTransitionVersionBySession.set(
        sessionId,
        Math.max(
          Date.parse(summary.updatedAt || '') || 0,
          Date.parse(summary.lastMessageAt || '') || 0
        )
      );
    }
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

  function commitProjectDrafts(projectId: string, drafts: Record<string, WebSessionDraftState>) {
    const normalizedProjectId = String(projectId || '').trim();
    if (!normalizedProjectId) {
      return;
    }
    const nextDraftState = { ...draftStateByProject.value };
    if (Object.keys(drafts).length > 0) {
      nextDraftState[normalizedProjectId] = drafts;
    } else {
      delete nextDraftState[normalizedProjectId];
    }
    draftStateByProject.value = nextDraftState;
    persistSessionDrafts(nextDraftState);
  }

  function commitDraftAttachmentUploads(
    projectId: string,
    uploads: Record<string, WebSessionDraftAttachmentUploadState>
  ) {
    const normalizedProjectId = String(projectId || '').trim();
    if (!normalizedProjectId) {
      return;
    }
    const nextUploads = { ...draftAttachmentUploadsByProject.value };
    if (Object.keys(uploads).length > 0) {
      nextUploads[normalizedProjectId] = uploads;
    } else {
      delete nextUploads[normalizedProjectId];
    }
    draftAttachmentUploadsByProject.value = nextUploads;
  }

  function setDraftAttachmentUploadState(
    projectId: string,
    sessionId: string,
    upload: WebSessionDraftAttachmentUploadState | null
  ) {
    const normalizedProjectId = String(projectId || '').trim();
    const normalizedSessionId = String(sessionId || '').trim();
    if (!normalizedProjectId || !normalizedSessionId) {
      return;
    }
    const projectUploads = draftAttachmentUploadsByProject.value[normalizedProjectId] ?? {};
    const nextProjectUploads = { ...projectUploads };
    if (upload) {
      nextProjectUploads[normalizedSessionId] = upload;
    } else {
      delete nextProjectUploads[normalizedSessionId];
    }
    commitDraftAttachmentUploads(normalizedProjectId, nextProjectUploads);
  }

  function createDraftAttachmentUploadID() {
    draftAttachmentUploadSeed += 1;
    return `upload-${Date.now()}-${draftAttachmentUploadSeed}`;
  }

  function draftAttachmentUploadQueueKey(projectId: string, sessionId: string) {
    return `${projectId}:${sessionId}`;
  }

  function normalizeDraftAttachmentFileName(file: File, index: number) {
    return buildUploadImageFileName(file.name, index, file.type);
  }

  function normalizeDraftAttachmentFile(file: File, index: number) {
    const fileName = normalizeDraftAttachmentFileName(file, index);
    if (fileName === file.name) {
      return {
        file,
        fileName,
      };
    }
    return {
      file: new File([file], fileName, {
        type: file.type,
        lastModified: file.lastModified,
      }),
      fileName,
    };
  }

  async function uploadAttachments(
    projectId: string,
    sessionId: string,
    files: File[]
  ): Promise<WebSessionDraftAttachmentUploadBatchResult> {
    const normalizedProjectId = String(projectId || '').trim();
    const normalizedSessionId = String(sessionId || '').trim();
    const imageFiles = Array.from(files).filter(file => file.type.startsWith('image/'));
    if (!normalizedProjectId || !normalizedSessionId || imageFiles.length === 0) {
      return {
        attachments: [],
        errors: [],
      };
    }

    const queueKey = draftAttachmentUploadQueueKey(normalizedProjectId, normalizedSessionId);
    const previousTask = draftAttachmentUploadQueues.get(queueKey) ?? Promise.resolve();
    const task = previousTask
      .catch(() => undefined)
      .then(async () => {
        const attachments: WebSessionAttachment[] = [];
        const errors: WebSessionDraftAttachmentUploadError[] = [];
        const batchID = createDraftAttachmentUploadID();
        const existingAttachmentCount = getDraft(normalizedProjectId, normalizedSessionId)
          .attachments.length;

        for (const [index, file] of imageFiles.entries()) {
          const nextAttachmentIndex = existingAttachmentCount + attachments.length + 1;
          const normalizedFile = normalizeDraftAttachmentFile(file, nextAttachmentIndex);
          const fileName = normalizedFile.fileName;
          const applyProgress = (progress: WebSessionAttachmentUploadProgress) => {
            setDraftAttachmentUploadState(normalizedProjectId, normalizedSessionId, {
              id: batchID,
              fileName,
              currentFileIndex: index + 1,
              totalFiles: imageFiles.length,
              loaded: progress.loaded,
              total: progress.total,
              percent: progress.percent ?? 0,
            });
          };

          applyProgress({
            loaded: 0,
            total: file.size > 0 ? file.size : undefined,
            percent: 0,
          });

          try {
            const attachment = await webSessionApi.uploadAttachment(
              normalizedProjectId,
              normalizedFile.file,
              {
                onProgress: applyProgress,
              }
            );
            attachments.push(attachment);
            updateDraft(normalizedProjectId, normalizedSessionId, draft => ({
              ...draft,
              attachments: [...draft.attachments, attachment],
              updatedAt: Date.now(),
            }));
          } catch (error) {
            errors.push({
              fileName,
              message: error instanceof Error ? error.message : 'failed to upload attachment',
            });
          }
        }

        setDraftAttachmentUploadState(normalizedProjectId, normalizedSessionId, null);
        return {
          attachments,
          errors,
        };
      });

    draftAttachmentUploadQueues.set(queueKey, task);

    try {
      return await task;
    } finally {
      if (draftAttachmentUploadQueues.get(queueKey) === task) {
        draftAttachmentUploadQueues.delete(queueKey);
      }
    }
  }

  function updateDraft(
    projectId: string,
    sessionId: string,
    updater: (draft: WebSessionDraftState) => WebSessionDraftState | null
  ) {
    const normalizedProjectId = String(projectId || '').trim();
    const normalizedSessionId = String(sessionId || '').trim();
    if (!normalizedProjectId || !normalizedSessionId) {
      return;
    }
    const projectDrafts = draftStateByProject.value[normalizedProjectId] ?? {};
    const currentDraft = projectDrafts[normalizedSessionId] ?? {
      text: '',
      attachments: [],
      updatedAt: 0,
    };
    const nextDraft = updater({
      text: currentDraft.text,
      attachments: [...currentDraft.attachments],
      updatedAt: currentDraft.updatedAt,
    });
    const nextProjectDrafts = { ...projectDrafts };
    if (!nextDraft || (!nextDraft.text.trim() && nextDraft.attachments.length === 0)) {
      delete nextProjectDrafts[normalizedSessionId];
    } else {
      nextProjectDrafts[normalizedSessionId] = {
        text: nextDraft.text,
        attachments: [...nextDraft.attachments],
        updatedAt: nextDraft.updatedAt || Date.now(),
      };
    }
    commitProjectDrafts(normalizedProjectId, nextProjectDrafts);
  }

  function setDraftText(projectId: string, sessionId: string, text: string) {
    updateDraft(projectId, sessionId, draft => ({
      ...draft,
      text,
      updatedAt: Date.now(),
    }));
  }

  function clearDraft(projectId: string, sessionId: string) {
    updateDraft(projectId, sessionId, () => null);
  }

  function moveDraft(projectId: string, fromSessionId: string, toSessionId: string) {
    const normalizedProjectId = String(projectId || '').trim();
    const normalizedFromSessionId = String(fromSessionId || '').trim();
    const normalizedToSessionId = String(toSessionId || '').trim();
    if (!normalizedProjectId || !normalizedFromSessionId || !normalizedToSessionId) {
      return;
    }
    if (normalizedFromSessionId === normalizedToSessionId) {
      return;
    }
    const projectDrafts = draftStateByProject.value[normalizedProjectId] ?? {};
    const sourceDraft = projectDrafts[normalizedFromSessionId];
    if (!sourceDraft) {
      return;
    }
    const targetDraft = projectDrafts[normalizedToSessionId] ?? {
      text: '',
      attachments: [],
      updatedAt: 0,
    };
    const mergedAttachments = [
      ...sourceDraft.attachments,
      ...targetDraft.attachments.filter(
        attachment =>
          !sourceDraft.attachments.some(sourceAttachment => sourceAttachment.id === attachment.id)
      ),
    ];
    const nextProjectDrafts = { ...projectDrafts };
    delete nextProjectDrafts[normalizedFromSessionId];
    nextProjectDrafts[normalizedToSessionId] = {
      text: sourceDraft.text.trim() ? sourceDraft.text : targetDraft.text,
      attachments: mergedAttachments,
      updatedAt: Date.now(),
    };
    commitProjectDrafts(normalizedProjectId, nextProjectDrafts);
  }

  function normalizeSession(session: WireSession): WebSessionSummary {
    const archivedAt =
      typeof session.aa === 'number' && Number.isFinite(session.aa)
        ? new Date(session.aa).toISOString()
        : null;
    const activityAt =
      typeof session.act === 'number' && Number.isFinite(session.act)
        ? new Date(session.act).toISOString()
        : new Date(session.lu).toISOString();
    const createdAt =
      typeof session.ca === 'number' && Number.isFinite(session.ca)
        ? new Date(session.ca).toISOString()
        : new Date(session.lu).toISOString();
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
      assistantState: normalizeAssistantStateValue(session.ast) || null,
      hasUnread: session.unr,
      archivedAt,
      activityAt,
      lastMessageAt: session.lma ? new Date(session.lma).toISOString() : null,
      assistantStateUpdatedAt: session.asu ? new Date(session.asu).toISOString() : null,
      sourceKind: session.sk ?? 'codex_app_server',
      syncState: session.ss ?? 'missing',
      lastSyncMode: session.lsm === 'deep' || session.lsm === 'fast' ? session.lsm : null,
      sourceCreatedAt: session.sca ? new Date(session.sca).toISOString() : null,
      sourceUpdatedAt: session.sua ? new Date(session.sua).toISOString() : null,
      lastSyncedAt: session.lsa ? new Date(session.lsa).toISOString() : null,
      threadPath: session.tp ?? null,
      threadPreview: session.tpv ?? null,
      turnCount: Number(session.tc ?? 0),
      itemCount: Number(session.ic ?? 0),
      syncError: session.se ?? null,
      createdAt,
      updatedAt: new Date(session.lu).toISOString(),
      usage: {
        inputTokens: session.usa?.in ?? 0,
        cachedInputTokens: session.usa?.cin ?? 0,
        outputTokens: session.usa?.out ?? 0,
        cost: session.cost ?? 0,
      },
      contextEstimate: {
        inputTokens: session.cea?.in ?? session.usa?.in ?? 0,
        cachedInputTokens: session.cea?.cin ?? session.usa?.cin ?? 0,
        outputTokens: session.cea?.out ?? session.usa?.out ?? 0,
        usedTokens:
          session.cea?.usd ??
          Math.max(
            0,
            Number(session.cea?.in ?? session.usa?.in ?? 0) +
              Number(session.cea?.cin ?? session.usa?.cin ?? 0) +
              Number(session.cea?.out ?? session.usa?.out ?? 0)
          ),
      },
      contextEstimateMode:
        session.cem === 'since_compaction' ? 'since_compaction' : 'cumulative_total',
      lastContextCompactionAt: session.lcca ? new Date(session.lcca).toISOString() : null,
      contextWindowTokens:
        typeof session.cwt === 'number' && Number.isFinite(session.cwt) ? session.cwt : null,
      contextWindowSource:
        session.cws === 'config' || session.cws === 'default' || session.cws === 'unavailable'
          ? session.cws
          : 'unavailable',
    };
  }

  function normalizeHistoryItem(item: WireHistoryItem | Record<string, unknown>): WebSessionBlock {
    const record = asRecord(item) ?? {};
    const rawTimestamp = parseHistoryTimeValue(record.ts2 ?? record.timestamp);
    const rawObservedAt = parseHistoryTimeValue(record.obs ?? record.observedAt);
    const rawAttachments = Array.isArray(record.atts)
      ? record.atts
      : Array.isArray(record.attachments)
        ? record.attachments
        : [];
    const rawTool = asRecord(record.tl ?? record.tool);
    const rawDetail = asRecord(record.dt ?? record.detail);
    const rawPayload = asRecord(record.pl ?? record.payload);
    const kind = String(record.kd ?? record.kind ?? '').trim();
    const itemType = String(record.tp ?? record.itemType ?? '').trim();
    const detailType = String(rawDetail?.type ?? '').trim();

    return {
      key: `${String(record.id ?? '')}:${Number(record.oi ?? record.orderIndex ?? 0)}`,
      id: String(record.id ?? ''),
      sourceTurnId:
        typeof record.stid === 'string'
          ? record.stid
          : typeof record.sourceTurnId === 'string'
            ? record.sourceTurnId
            : null,
      sourceItemId: normalizeHistorySourceItemId(record, rawPayload),
      orderIndex: Number(record.oi ?? record.orderIndex ?? 0),
      kind:
        kind === 'user' || kind === 'assistant' || kind === 'system' || kind === 'tool'
          ? kind
          : 'system',
      itemType,
      text:
        typeof record.txt === 'string'
          ? record.txt
          : typeof record.text === 'string'
            ? record.text
            : '',
      timestamp: rawTimestamp ?? rawObservedAt ?? 0,
      observedAt: rawObservedAt ?? rawTimestamp ?? null,
      attachments: rawAttachments
        .map(attachment => asRecord(attachment))
        .filter((attachment): attachment is Record<string, unknown> => Boolean(attachment))
        .map(attachment => ({
          id: String(attachment.id ?? ''),
          name: String(attachment.name ?? ''),
          mime: typeof attachment.mime === 'string' ? attachment.mime : undefined,
          size:
            typeof attachment.sz === 'number'
              ? attachment.sz
              : typeof attachment.size === 'number'
                ? attachment.size
                : undefined,
          path: typeof attachment.path === 'string' ? attachment.path : undefined,
        }))
        .filter(attachment => Boolean(attachment.id || attachment.name)),
      tool: rawTool
        ? {
            id: String(rawTool.id ?? ''),
            name: String(rawTool.name ?? ''),
            kind: typeof rawTool.kind === 'string' ? rawTool.kind : undefined,
            input: rawTool.in ?? rawTool.input,
            output:
              typeof rawTool.out === 'string'
                ? rawTool.out
                : typeof rawTool.output === 'string'
                  ? rawTool.output
                  : undefined,
            status:
              rawTool.st === 'error' || rawTool.st === 'running' || rawTool.st === 'done'
                ? rawTool.st
                : rawTool.status === 'error' ||
                    rawTool.status === 'running' ||
                    rawTool.status === 'done'
                  ? rawTool.status
                  : rawTool.st === 'completed' || rawTool.status === 'completed'
                    ? 'done'
                    : 'running',
            startedAt:
              rawTimestamp != null
                ? rawTimestamp
                : rawObservedAt != null
                  ? rawObservedAt
                  : undefined,
            meta: asRecord(rawTool.meta),
            commandGroup: parseToolCommandGroup(rawTool.cg ?? rawTool.commandGroup),
          }
        : undefined,
      level:
        record.lvl === 'warn' || record.lvl === 'error' || record.lvl === 'info'
          ? record.lvl
          : record.level === 'warn' || record.level === 'error' || record.level === 'info'
            ? record.level
            : undefined,
      done: record.dn === true || record.done === true,
      detail: rawDetail
        ? {
            type:
              detailType === 'approval_request' ||
              detailType === 'approval_response' ||
              detailType === 'user_input_request' ||
              detailType === 'user_input_response'
                ? detailType
                : 'approval_request',
            prompt: typeof rawDetail.prompt === 'string' ? rawDetail.prompt : undefined,
            questions: Array.isArray(rawDetail.questions)
              ? (rawDetail.questions as WebSessionUserInputQuestion[])
              : undefined,
            answers: Array.isArray(rawDetail.answers)
              ? (rawDetail.answers as WebSessionHistoryAnswerEntry[])
              : undefined,
            action: typeof rawDetail.action === 'string' ? rawDetail.action : undefined,
          }
        : undefined,
      payload: rawPayload,
    };
  }

  function compareArchivedSessions(left: WebSessionSummary, right: WebSessionSummary) {
    const leftActivity = Date.parse(left.activityAt || left.updatedAt || left.createdAt);
    const rightActivity = Date.parse(right.activityAt || right.updatedAt || right.createdAt);
    if (
      Number.isFinite(leftActivity) &&
      Number.isFinite(rightActivity) &&
      leftActivity !== rightActivity
    ) {
      return rightActivity - leftActivity;
    }
    return right.id.localeCompare(left.id);
  }

  function isArchivedScopeProject(projectId: string) {
    const scopeKey = archivedListMeta.value.scopeKey;
    if (!scopeKey || !projectId) {
      return false;
    }
    return scopeKey.split('::').includes(projectId);
  }

  function sortArchivedSessionIds(ids: string[]) {
    return [...ids].sort((leftId, rightId) => {
      const left = archivedSessionsById.value[leftId];
      const right = archivedSessionsById.value[rightId];
      if (!left && !right) {
        return leftId.localeCompare(rightId);
      }
      if (!left) {
        return 1;
      }
      if (!right) {
        return -1;
      }
      return compareArchivedSessions(left, right);
    });
  }

  function upsertArchivedSession(
    summary: WebSessionSummary,
    options?: { includeInList?: boolean }
  ) {
    const previous = archivedSessionsById.value[summary.id];
    archivedSessionsById.value = {
      ...archivedSessionsById.value,
      [summary.id]: {
        ...previous,
        ...summary,
      },
    };

    const includeInList = options?.includeInList ?? isArchivedScopeProject(summary.projectId);
    if (!includeInList) {
      return;
    }

    const nextIds = archivedSessionIds.value.includes(summary.id)
      ? archivedSessionIds.value
      : [...archivedSessionIds.value, summary.id];
    archivedSessionIds.value = sortArchivedSessionIds(nextIds);
  }

  function removeArchivedSessionRecord(sessionId: string, options?: { clearSummary?: boolean }) {
    archivedSessionIds.value = archivedSessionIds.value.filter(id => id !== sessionId);
    if (options?.clearSummary !== false) {
      const next = { ...archivedSessionsById.value };
      delete next[sessionId];
      archivedSessionsById.value = next;
    }
  }

  function removeCurrentSessionRecord(projectId: string, sessionId: string) {
    const current = sessionsByProject.value[projectId] ?? [];
    const removed = current.find(item => item.id === sessionId) ?? null;
    const next = current.filter(item => item.id !== sessionId);
    sessionsByProject.value = {
      ...sessionsByProject.value,
      [projectId]: next,
    };
    const currentActive = activeSessionIdByProject.value[projectId];
    if (currentActive === sessionId) {
      activeSessionIdByProject.value = {
        ...activeSessionIdByProject.value,
        [projectId]: '',
      };
      persistActiveSessions(activeSessionIdByProject.value);
    }
    return removed;
  }

  function clearSessionRuntimeState(sessionId: string, projectId?: string) {
    const nextEvents = { ...eventsBySession.value };
    delete nextEvents[sessionId];
    eventsBySession.value = nextEvents;
    const nextHistory = { ...historyBySession.value };
    delete nextHistory[sessionId];
    historyBySession.value = nextHistory;
    appliedSnapshotVersionBySession.delete(sessionId);
    const nextPendingInputs = { ...pendingInputsBySession.value };
    delete nextPendingInputs[sessionId];
    pendingInputsBySession.value = nextPendingInputs;
    completedTransitionVersionBySession.delete(sessionId);
    redirectAbortSessions.delete(sessionId);
    flushingSessions.delete(sessionId);
    const timer = pendingFlushTimers.get(sessionId);
    if (timer != null) {
      window.clearTimeout(timer);
      pendingFlushTimers.delete(sessionId);
    }
    if (projectId) {
      clearDraft(projectId, sessionId);
    }
  }

  function upsertCurrentSession(summary: WebSessionSummary) {
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

  function upsertSession(summary: WebSessionSummary) {
    if (summary.archivedAt) {
      removeCurrentSessionRecord(summary.projectId, summary.id);
      upsertArchivedSession(summary);
      return;
    }
    removeArchivedSessionRecord(summary.id);
    upsertCurrentSession(summary);
  }

  function removeSession(projectId: string, sessionId: string) {
    const removed =
      removeCurrentSessionRecord(projectId, sessionId) ??
      archivedSessionsById.value[sessionId] ??
      null;
    removeArchivedSessionRecord(sessionId);
    clearSessionRuntimeState(sessionId, projectId);
    if (removed) {
      emitter.emit('ai:closed', {
        sessionId: removed.id,
        sessionTitle: removed.title,
        projectId: removed.projectId,
        assistant: getAssistantDescriptor(removed),
      } satisfies WebSessionAIEvent);
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

  function mergeEvents(sessionId: string, incoming: WebSessionBlock[]) {
    const merged = [...(eventsBySession.value[sessionId] ?? [])];
    const indexById = new Map(merged.map((item, index) => [item.id, index]));
    incoming.forEach(item => {
      if (!item || !item.id) {
        return;
      }
      const existingIndex = indexById.get(item.id);
      if (existingIndex == null) {
        merged.push(item);
        indexById.set(item.id, merged.length - 1);
        return;
      }
      merged.splice(existingIndex, 1, {
        ...merged[existingIndex],
        ...item,
      });
    });
    merged.sort((left, right) => left.orderIndex - right.orderIndex);
    eventsBySession.value = {
      ...eventsBySession.value,
      [sessionId]: merged,
    };
  }

  function resetSessionEvents(sessionId: string, events: WebSessionBlock[]) {
    eventsBySession.value = {
      ...eventsBySession.value,
      [sessionId]: [...events].sort((left, right) => left.orderIndex - right.orderIndex),
    };
  }

  function buildBlocks(sessionId: string): WebSessionBlock[] {
    return eventsBySession.value[sessionId] ?? [];
  }

  const getBlocks = (sessionId: string) => buildBlocks(sessionId);

  function getPendingApproval(sessionId: string): WebSessionApprovalState | null {
    const blocks = buildBlocks(sessionId);
    let pending: WebSessionApprovalState | null = null;
    for (const block of blocks) {
      if (block.detail?.type === 'approval_request') {
        pending = {
          id: block.id,
          prompt: block.detail.prompt ?? block.text,
          requestedAt: block.timestamp,
          stale: false,
        };
        continue;
      }
      if (block.detail?.type === 'approval_response' || block.kind === 'user') {
        pending = null;
        continue;
      }
      if (
        block.itemType === 'run_abort' &&
        pending &&
        isProcessRestartPayload(block.payload ?? undefined)
      ) {
        pending = {
          ...pending,
          stale: true,
          recoveryReason: String(block.payload?.reason ?? ''),
          recoveryMessage: getRecoveryMessage(block.payload ?? undefined),
        };
        continue;
      }
      if (block.itemType === 'run_abort' || block.itemType === 'run_fail') {
        pending = null;
      }
    }
    return pending;
  }

  function getPendingUserInput(sessionId: string): WebSessionUserInputState | null {
    const blocks = buildBlocks(sessionId);
    let pending: WebSessionUserInputState | null = null;
    for (const block of blocks) {
      if (block.detail?.type === 'user_input_request') {
        pending = {
          id: block.id,
          itemId: block.sourceItemId || block.id,
          prompt: block.detail.prompt ?? block.text,
          questions: block.detail.questions ?? [],
          requestedAt: block.timestamp,
          stale: false,
        };
        continue;
      }
      if (block.detail?.type === 'user_input_response' || block.kind === 'user') {
        pending = null;
        continue;
      }
      if (
        block.itemType === 'run_abort' &&
        pending &&
        isProcessRestartPayload(block.payload ?? undefined)
      ) {
        pending = {
          ...pending,
          stale: true,
          recoveryReason: String(block.payload?.reason ?? ''),
          recoveryMessage: getRecoveryMessage(block.payload ?? undefined),
        };
        continue;
      }
      if (block.itemType === 'run_abort' || block.itemType === 'run_fail') {
        pending = null;
      }
    }
    return pending;
  }

  function getLiveState(sessionId: string): WebSessionLiveState {
    const session = findSessionById(sessionId);
    const approval = getPendingApproval(sessionId);
    const userInput = getPendingUserInput(sessionId);
    const assistantState = getSessionAssistantStateValue(session);
    let activeTool:
      | {
          id: string;
          name: string;
          kind?: string;
          summary?: string;
          count?: number;
          groupId?: string;
          startedAt?: number;
        }
      | undefined;
    let sawAssistantOutput = false;
    let assistantDone = false;
    let firstAssistantOutputAt: number | undefined;
    let errorMessage = '';
    let updatedAt = session ? Date.parse(session.updatedAt) || Date.now() : Date.now();
    const assistantStateUpdatedAt = getAssistantStateUpdatedAt(session);
    let runStartedAt: number | undefined;
    let retryState:
      | {
          code: string;
          message: string;
          attempt?: number;
          maxAttempts?: number;
          updatedAt: number;
        }
      | undefined;

    for (const block of buildBlocks(sessionId)) {
      updatedAt = block.observedAt || block.timestamp || updatedAt;
      if (block.kind === 'assistant') {
        sawAssistantOutput = true;
        assistantDone = block.done === true;
        retryState = undefined;
        if (!firstAssistantOutputAt && block.timestamp > 0) {
          firstAssistantOutputAt = block.timestamp;
        }
      }
      if (block.kind === 'user' && block.timestamp > 0) {
        runStartedAt = block.timestamp;
        sawAssistantOutput = false;
        assistantDone = false;
        firstAssistantOutputAt = undefined;
        activeTool = undefined;
        errorMessage = '';
        retryState = undefined;
      }
      const retryPayload = getTransportRetryPayload(block.payload);
      if (block.itemType === 'note' && retryPayload) {
        retryState = {
          ...retryPayload,
          updatedAt: block.observedAt || block.timestamp || updatedAt,
        };
      }
      if (block.kind === 'tool' && block.tool) {
        if (block.tool.kind === 'reasoning') {
          continue;
        }
        if (block.tool.status === 'running') {
          activeTool = {
            id: block.tool.id,
            name: block.tool.name,
            kind: block.tool.kind,
            summary: extractToolSummary({
              kind: block.tool.kind,
              in: asRecord(block.tool.input) ?? block.tool.input,
              meta: block.tool.meta,
              out: block.tool.output,
            } as Record<string, unknown>),
            count: block.tool.commandGroup?.count,
            groupId: block.tool.commandGroup?.id,
            startedAt: block.tool.startedAt ?? block.timestamp,
          };
          retryState = undefined;
        } else if (activeTool?.id === block.tool.id) {
          activeTool = undefined;
          retryState = undefined;
        }
      }
      if (block.itemType === 'run_fail') {
        errorMessage = block.text || 'Run failed';
        retryState = undefined;
      }
    }

    if (assistantState === 'waiting_approval') {
      return {
        phase: 'waiting_approval',
        running: session?.status === 'running',
        updatedAt: approval?.requestedAt ?? assistantStateUpdatedAt ?? updatedAt,
        startedAt: approval?.requestedAt ?? assistantStateUpdatedAt ?? runStartedAt,
        approval,
        tool: activeTool,
      };
    }

    if (assistantState === 'waiting_plan_approval') {
      return {
        phase: 'waiting_plan_approval',
        running: false,
        updatedAt: assistantStateUpdatedAt || updatedAt,
        startedAt: assistantStateUpdatedAt ?? runStartedAt,
      };
    }

    if (assistantState === 'waiting_input') {
      return {
        phase: 'waiting_input',
        running: session?.status === 'running',
        updatedAt: userInput?.requestedAt ?? assistantStateUpdatedAt ?? updatedAt,
        startedAt: userInput?.requestedAt ?? assistantStateUpdatedAt ?? runStartedAt,
        tool: activeTool,
        userInput,
      };
    }

    if (session?.status === 'running') {
      if (retryState) {
        return {
          phase: 'retrying',
          running: true,
          updatedAt: retryState.updatedAt,
          startedAt: runStartedAt,
          retry: {
            code: retryState.code,
            message: retryState.message,
            attempt: retryState.attempt,
            maxAttempts: retryState.maxAttempts,
          },
        };
      }
      if (activeTool) {
        return {
          phase: 'tool',
          running: true,
          updatedAt,
          startedAt: activeTool.startedAt ?? assistantStateUpdatedAt ?? runStartedAt,
          tool: activeTool,
        };
      }
      if (sawAssistantOutput && !assistantDone) {
        return {
          phase: 'thinking',
          running: true,
          updatedAt,
          startedAt: firstAssistantOutputAt ?? assistantStateUpdatedAt ?? runStartedAt,
        };
      }
      return {
        phase: 'starting',
        running: true,
        updatedAt,
        startedAt: assistantStateUpdatedAt ?? runStartedAt,
      };
    }

    if (session?.status === 'done') {
      return {
        phase: 'done',
        running: false,
        updatedAt,
        startedAt: runStartedAt,
      };
    }

    if (session?.status === 'err') {
      return {
        phase: 'error',
        running: false,
        updatedAt,
        startedAt: runStartedAt,
        errorMessage,
      };
    }

    return {
      phase: 'idle',
      running: false,
      updatedAt,
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
      return;
    }

    const archived = archivedSessionsById.value[sessionId];
    if (archived) {
      archivedSessionsById.value = {
        ...archivedSessionsById.value,
        [sessionId]: updater(archived),
      };
      archivedSessionIds.value = sortArchivedSessionIds(archivedSessionIds.value);
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
    const approvalForNotification =
      nextApproval ??
      (nextState.phase === 'waiting_approval' || nextState.phase === 'waiting_plan_approval'
        ? {
            id: `status:${sessionId}:${nextState.updatedAt}`,
            prompt: '',
            requestedAt: nextState.updatedAt,
            stale: false,
          }
        : null);
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
      approvalForNotification &&
      (!previousApproval ||
        previousApproval.id !== approvalForNotification.id ||
        previousApproval.requestedAt !== approvalForNotification.requestedAt ||
        (previousState.phase !== 'waiting_approval' &&
          previousState.phase !== 'waiting_plan_approval'))
    ) {
      emitter.emit('ai:approval-needed', {
        ...baseEvent,
        approval: approvalForNotification,
      } satisfies WebSessionApprovalEvent);
    }

    if (nextState.phase === 'done' && previousState.phase !== 'done') {
      const completionVersion = Math.max(
        nextState.updatedAt,
        Date.parse(session.updatedAt || '') || 0,
        Date.parse(session.lastMessageAt || '') || 0
      );
      const lastCompletionVersion = completedTransitionVersionBySession.get(sessionId) ?? -1;
      if (completionVersion > lastCompletionVersion) {
        completedTransitionVersionBySession.set(sessionId, completionVersion);
        emitter.emit('ai:completed', baseEvent);
      }
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
      const historyTotal = Number(frame.h?.tot ?? frame.h?.its?.length ?? 0);
      const snapshotInput = currentSnapshotVersionInput(frame.sid);
      const appliedVersion = appliedSnapshotVersionBySession.get(frame.sid) ?? null;
      const currentVersion = selectLatestWebSessionSnapshotVersion(
        appliedVersion,
        snapshotInput ? buildWebSessionSnapshotVersion(snapshotInput) : null
      );
      const incomingSnapshot = {
        session: summary,
        historyTotal,
      };
      if (
        !shouldApplyIncomingWebSessionSnapshot({
          appliedVersion,
          currentSnapshot: snapshotInput,
          incomingSnapshot,
        })
      ) {
        if (import.meta.env.DEV) {
          console.debug('[Web Session] Dropped stale snapshot frame', {
            sessionId: frame.sid,
            currentVersion,
            incomingVersion: buildWebSessionSnapshotVersion(incomingSnapshot),
          });
        }
        setHistoryLoading(frame.sid, false);
        return;
      }
      applySessionSnapshot(
        frame.sid,
        summary,
        Array.isArray(frame.h?.its) ? frame.h.its.map(item => normalizeHistoryItem(item)) : [],
        {
          hasMore: frame.h?.hm ?? false,
          beforeCursor: frame.h?.bc ?? '',
          total: historyTotal,
        }
      );
      return;
    }

    if (frame.k === 'evt' && frame.sid) {
      const previousState = getLiveState(frame.sid);
      const previousApproval = getPendingApproval(frame.sid);
      if (frame.s) {
        upsertSession(normalizeSession(frame.s));
      }

      if (frame.op === 'hist_page' && frame.h) {
        const historicalItems = Array.isArray(frame.h.its)
          ? frame.h.its.map(item => normalizeHistoryItem(item))
          : [];
        mergeEvents(frame.sid, historicalItems);
        historyBySession.value = {
          ...historyBySession.value,
          [frame.sid]: {
            ...getHistoryMeta(frame.sid),
            hasMore: Boolean(frame.h.hm),
            beforeCursor: String(frame.h.bc ?? ''),
            loading: false,
          },
        };
        return;
      }

      if (frame.op === 'hist_item' && frame.i) {
        const item = normalizeHistoryItem(frame.i);
        mergeEvents(frame.sid, [item]);
        if (item.kind === 'tool' && item.tool?.status !== 'running') {
          maybeAbortForRedirect(frame.sid);
        }
      }

      const session = findSessionById(frame.sid);
      if (session?.status === 'done' || session?.status === 'err' || session?.status === 'idle') {
        redirectAbortSessions.delete(frame.sid);
        schedulePendingFlush(frame.sid);
      }

      if (
        frame.op === 'hist_item' &&
        frame.i &&
        normalizeHistoryItem(frame.i).tool?.status !== 'running'
      ) {
        maybeAbortForRedirect(frame.sid);
      }

      emitStateTransition(frame.sid, previousState, previousApproval);
    }
  }

  function rejectPendingCommands(reason: Error) {
    pending.forEach(entry => {
      entry.reject(reason);
    });
    pending.clear();
  }

  function openEventStream(): Promise<void> {
    if (eventSocket && eventSocket.readyState === WebSocket.OPEN) {
      connectionState.value = 'open';
      return Promise.resolve();
    }
    if (eventConnectPromise) {
      return eventConnectPromise;
    }
    connectionState.value = 'connecting';
    eventConnectPromise = new Promise((resolve, reject) => {
      let settled = false;
      const ws = new WebSocket(resolveWsUrl(EVENTS_WS_PATH));
      ws.onopen = () => {
        settled = true;
        eventSocket = ws;
        connectionState.value = 'open';
        eventConnectPromise = null;
        resolve();
      };
      ws.onmessage = event => {
        try {
          const frame = JSON.parse(event.data) as WireFrame;
          applyFrame(frame);
        } catch (error) {
          console.error('[Web Session] Failed to parse event websocket frame', error);
        }
      };
      ws.onerror = event => {
        console.error('[Web Session] event websocket error', event);
      };
      ws.onclose = () => {
        eventSocket = null;
        connectionState.value = 'closed';
        eventConnectPromise = null;
        if (!settled) {
          reject(new Error('websocket event stream closed before opening'));
          return;
        }
        if (eventReconnectTimer != null) {
          window.clearTimeout(eventReconnectTimer);
        }
        eventReconnectTimer = window.setTimeout(() => {
          eventReconnectTimer = null;
          if (allSessionIds.value.size > 0) {
            void openEventStream();
          }
        }, 1200);
      };
    });
    return eventConnectPromise.catch(error => {
      eventConnectPromise = null;
      connectionState.value = 'closed';
      throw error;
    });
  }

  function openCommandSocket(): Promise<void> {
    if (commandSocket && commandSocket.readyState === WebSocket.OPEN) {
      return Promise.resolve();
    }
    if (commandConnectPromise) {
      return commandConnectPromise;
    }
    commandConnectPromise = new Promise((resolve, reject) => {
      let settled = false;
      const ws = new WebSocket(resolveWsUrl(COMMAND_WS_PATH));
      ws.onopen = () => {
        settled = true;
        commandSocket = ws;
        commandConnectPromise = null;
        resolve();
      };
      ws.onmessage = event => {
        try {
          const frame = JSON.parse(event.data) as WireFrame;
          applyFrame(frame);
        } catch (error) {
          console.error('[Web Session] Failed to parse command websocket frame', error);
        }
      };
      ws.onerror = event => {
        console.error('[Web Session] command websocket error', event);
      };
      ws.onclose = () => {
        commandSocket = null;
        commandConnectPromise = null;
        if (!settled) {
          reject(new Error('websocket command channel closed before opening'));
          return;
        }
        rejectPendingCommands(new Error('websocket command channel closed'));
      };
    });
    return commandConnectPromise.catch(error => {
      commandConnectPromise = null;
      throw error;
    });
  }

  async function sendCommand(op: string, sessionId: string, payload: Record<string, unknown> = {}) {
    await openCommandSocket();
    if (!commandSocket || commandSocket.readyState !== WebSocket.OPEN) {
      throw new Error('websocket command channel is not connected');
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
    commandSocket.send(JSON.stringify(frame));
    return promise;
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

  function invalidateArchivedSessions() {
    archivedSessionIds.value = [];
    archivedListMeta.value = defaultArchivedListMeta();
  }

  async function loadArchivedSessions(
    projectIds: string[],
    options?: {
      reset?: boolean;
      limit?: number;
    }
  ) {
    const scope = normalizeProjectScope(projectIds);
    if (!scope.key) {
      invalidateArchivedSessions();
      return [];
    }

    const limit = Math.max(1, options?.limit ?? 20);
    const reset = options?.reset === true || archivedListMeta.value.scopeKey !== scope.key;
    const previousMeta = getArchivedMeta(scope.ids);
    const offset = reset ? 0 : previousMeta.offset;

    archivedListMeta.value = {
      scopeKey: scope.key,
      total: reset ? 0 : previousMeta.total,
      offset,
      hasMore: reset ? false : previousMeta.hasMore,
      loading: true,
    };

    try {
      const result = await webSessionApi.queryArchived({
        projectIds: scope.ids,
        offset,
        limit,
      });
      result.items.forEach(item => {
        upsertArchivedSession(item, { includeInList: false });
      });
      archivedSessionIds.value = sortArchivedSessionIds(
        reset
          ? result.items.map(item => item.id)
          : Array.from(new Set([...archivedSessionIds.value, ...result.items.map(item => item.id)]))
      );
      archivedListMeta.value = {
        scopeKey: scope.key,
        total: result.total,
        offset: result.nextOffset,
        hasMore: result.hasMore,
        loading: false,
      };
      return getArchivedSessions(scope.ids);
    } catch (error) {
      archivedListMeta.value = {
        scopeKey: scope.key,
        total: reset ? 0 : previousMeta.total,
        offset,
        hasMore: reset ? false : previousMeta.hasMore,
        loading: false,
      };
      throw error;
    }
  }

  async function loadSessionSnapshot(
    projectId: string,
    sessionId: string,
    options?: LoadSessionSnapshotOptions
  ) {
    if (!projectId || !sessionId) {
      return null;
    }
    setHistoryLoading(sessionId, true);
    try {
      const snapshot = await webSessionApi.snapshot(projectId, sessionId);
      if (snapshot?.session) {
        applySessionSnapshot(
          sessionId,
          snapshot.session,
          Array.isArray(snapshot.history?.items)
            ? snapshot.history.items.map(item => normalizeHistoryItem(item as WireHistoryItem))
            : [],
          {
            hasMore: Boolean(snapshot.history?.hasMore),
            beforeCursor: String(snapshot.history?.beforeCursor ?? ''),
            total: Number(snapshot.history?.total ?? 0),
          }
        );
      } else {
        setHistoryLoading(sessionId, false);
      }
      if (options?.rememberActive !== false) {
        rememberActiveSession(projectId, sessionId);
      }
      return snapshot;
    } catch (error) {
      setHistoryLoading(sessionId, false);
      throw error;
    }
  }

  async function renameSession(projectId: string, sessionId: string, title: string) {
    await sendCommand('rename', sessionId, { ttl: title });
    rememberActiveSession(projectId, sessionId);
  }

  async function archiveSession(projectId: string, sessionId: string) {
    const summary = await webSessionApi.archive(projectId, sessionId);
    removeCurrentSessionRecord(projectId, sessionId);
    upsertArchivedSession(summary, { includeInList: false });
    return summary;
  }

  async function unarchiveSession(projectId: string, sessionId: string) {
    const summary = await webSessionApi.unarchive(projectId, sessionId);
    removeArchivedSessionRecord(sessionId);
    upsertCurrentSession(summary);
    return summary;
  }

  async function syncSession(
    projectId: string,
    sessionId: string,
    mode?: 'fast' | 'deep',
    clearExisting = false,
    options?: SyncSessionOptions
  ) {
    const session = findSessionById(sessionId);
    const rememberActive = options?.rememberActive ?? !session?.archivedAt;
    updateSessionStatus(sessionId, current => ({
      ...current,
      syncState: 'syncing',
      syncError: null,
      updatedAt: new Date().toISOString(),
    }));
    setHistoryLoading(sessionId, true);
    try {
      const snapshot = await webSessionApi.sync(projectId, sessionId, mode, clearExisting);
      if (snapshot?.session) {
        applySessionSnapshot(
          sessionId,
          snapshot.session,
          Array.isArray(snapshot?.history?.items)
            ? snapshot.history.items.map(item => normalizeHistoryItem(item as WireHistoryItem))
            : [],
          {
            hasMore: Boolean(snapshot?.history?.hasMore),
            beforeCursor: String(snapshot?.history?.beforeCursor ?? ''),
            total: Number(snapshot?.history?.total ?? 0),
          }
        );
      }
      if (rememberActive) {
        rememberActiveSession(projectId, sessionId);
      }
      return snapshot;
    } catch (error) {
      setHistoryLoading(sessionId, false);
      updateSessionStatus(sessionId, current => ({
        ...current,
        syncState: 'error',
        syncError: error instanceof Error ? error.message : String(error),
        updatedAt: new Date().toISOString(),
      }));
      throw error;
    }
  }

  async function deleteSession(projectId: string, sessionId: string) {
    await webSessionApi.delete(projectId, sessionId);
    removeSession(projectId, sessionId);
  }

  async function sendMessage(
    sessionId: string,
    text: string,
    attachmentIds: string[],
    mode?: 'redirect' | 'queue'
  ) {
    const session = findSessionById(sessionId);
    if (session?.archivedAt) {
      throw new Error('session is archived');
    }
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
    const session = findSessionById(sessionId);
    if (!session) {
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
      const history = await webSessionApi.history(session.projectId, sessionId, {
        beforeCursor: meta.beforeCursor,
        limit,
      });
      const historicalItems = Array.isArray(history.items)
        ? history.items.map(item => normalizeHistoryItem(item as WireHistoryItem))
        : [];
      mergeEvents(sessionId, historicalItems);
      historyBySession.value = {
        ...historyBySession.value,
        [sessionId]: {
          ...getHistoryMeta(sessionId),
          hasMore: Boolean(history.hasMore),
          beforeCursor: String(history.beforeCursor ?? ''),
          total: Number(history.total ?? getHistoryMeta(sessionId).total),
          loading: false,
        },
      };
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

  async function moveSession(
    projectId: string,
    sessionId: string,
    previousSessionId = '',
    nextSessionId = ''
  ) {
    const current = getSessions(projectId);
    if (
      !projectId ||
      !sessionId ||
      (previousSessionId && previousSessionId === sessionId) ||
      (nextSessionId && nextSessionId === sessionId)
    ) {
      return;
    }

    const original = [...current];
    const reordered = current.filter(session => session.id !== sessionId);
    const moving = current.find(session => session.id === sessionId);
    if (!moving) {
      return;
    }

    let insertIndex = reordered.length;
    if (previousSessionId) {
      const previousIndex = reordered.findIndex(session => session.id === previousSessionId);
      insertIndex = previousIndex >= 0 ? previousIndex + 1 : reordered.length;
    } else if (nextSessionId) {
      const nextIndex = reordered.findIndex(session => session.id === nextSessionId);
      insertIndex = nextIndex >= 0 ? nextIndex : 0;
    }

    reordered.splice(insertIndex, 0, moving);
    const reorderedWithOrder = reordered.map((session, index) => ({
      ...session,
      orderIndex: (index + 1) * 1000,
    }));
    sessionsByProject.value = {
      ...sessionsByProject.value,
      [projectId]: reorderedWithOrder,
    };

    try {
      await sendCommand('move', moving.id, {
        prv: previousSessionId,
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

  async function uploadAttachment(projectId: string, sessionId: string, file: File) {
    const result = await uploadAttachments(projectId, sessionId, [file]);
    if (result.errors.length > 0) {
      throw new Error(result.errors[0]?.message || 'failed to upload attachment');
    }
    const attachment = result.attachments[0];
    if (!attachment) {
      throw new Error('failed to upload attachment');
    }
    return attachment;
  }

  function removeDraftAttachment(projectId: string, sessionId: string, attachmentId: string) {
    updateDraft(projectId, sessionId, draft => ({
      ...draft,
      attachments: draft.attachments.filter(item => item.id !== attachmentId),
      updatedAt: Date.now(),
    }));
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
    return session;
  }

  return {
    connectionState,
    lastError,
    getDraft,
    getSessions,
    getArchivedSessions,
    getArchivedMeta,
    getActiveSessionId,
    hasStoredActiveSession,
    getActiveSession,
    getDraftAttachments,
    getDraftAttachmentUpload,
    setDraftText,
    getPendingInputs,
    getHistoryMeta,
    getBlocks,
    getLatestEventSeq,
    loadSessions,
    loadArchivedSessions,
    invalidateArchivedSessions,
    setActiveSession,
    loadSessionSnapshot,
    createSession: createSessionViaHttp,
    renameSession,
    archiveSession,
    unarchiveSession,
    syncSession,
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
    uploadAttachments,
    uploadAttachment,
    removeDraftAttachment,
    removePendingInput,
    clearDraft,
    moveDraft,
    openEventStream,
    emitter,
  };
});
