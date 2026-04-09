import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { WebSessionSummary } from '@/types/models';
import { useWebSessionStore } from '@/stores/webSession';

const { listMock, queryArchivedMock, snapshotMock, historyMock, syncMock } = vi.hoisted(() => ({
  listMock: vi.fn(),
  queryArchivedMock: vi.fn(),
  snapshotMock: vi.fn(),
  historyMock: vi.fn(),
  syncMock: vi.fn(),
}));

vi.mock('@/api/webSession', () => ({
  webSessionApi: {
    list: listMock,
    queryArchived: queryArchivedMock,
    snapshot: snapshotMock,
    history: historyMock,
    sync: syncMock,
  },
}));

vi.mock('@/utils/ws', () => ({
  resolveWsUrl: (path: string) => path,
}));

function createStorageMock() {
  const store = new Map<string, string>();
  return {
    getItem(key: string) {
      return store.has(key) ? store.get(key)! : null;
    },
    setItem(key: string, value: string) {
      store.set(key, String(value));
    },
    removeItem(key: string) {
      store.delete(key);
    },
    clear() {
      store.clear();
    },
  };
}

function makeSession(overrides: Partial<WebSessionSummary> = {}): WebSessionSummary {
  return {
    id: 'session-1',
    projectId: 'project-1',
    worktreeId: null,
    orderIndex: 1000,
    agent: 'codex',
    title: 'Codex Session',
    model: 'gpt-5.4',
    reasoningEffort: 'medium',
    workflowMode: 'default',
    permissionLevel: 'elevated',
    cwd: '/tmp/project',
    nativeSessionId: 'native-1',
    status: 'running',
    assistantState: 'waiting_input',
    hasUnread: false,
    archivedAt: null,
    activityAt: '2026-04-09T10:00:00.000Z',
    lastMessageAt: '2026-04-09T10:00:00.000Z',
    assistantStateUpdatedAt: '2026-04-09T10:00:00.000Z',
    sourceKind: 'codex_app_server',
    syncState: 'fresh',
    lastSyncMode: 'fast',
    sourceCreatedAt: '2026-04-09T09:00:00.000Z',
    sourceUpdatedAt: '2026-04-09T10:00:00.000Z',
    lastSyncedAt: '2026-04-09T10:00:00.000Z',
    threadPath: '/tmp/session.jsonl',
    threadPreview: 'preview',
    turnCount: 1,
    itemCount: 1,
    syncError: null,
    createdAt: '2026-04-09T09:00:00.000Z',
    updatedAt: '2026-04-09T10:00:00.000Z',
    usage: {
      inputTokens: 1,
      cachedInputTokens: 0,
      outputTokens: 1,
      cost: 0,
    },
    contextEstimate: {
      inputTokens: 1,
      cachedInputTokens: 0,
      outputTokens: 1,
      usedTokens: 2,
    },
    contextEstimateMode: 'cumulative_total',
    lastContextCompactionAt: null,
    contextWindowTokens: null,
    contextWindowSource: 'default',
    ...overrides,
  };
}

function toMillis(value?: string | null) {
  const parsed = Date.parse(value ?? '');
  return Number.isFinite(parsed) ? parsed : Date.now();
}

function toWireSession(session: WebSessionSummary) {
  return {
    id: session.id,
    pid: session.projectId,
    wid: session.worktreeId,
    oi: session.orderIndex,
    ag: session.agent,
    md: session.model,
    re: session.reasoningEffort,
    wm: session.workflowMode,
    pl: session.permissionLevel,
    ttl: session.title,
    cwd: session.cwd,
    nsid: session.nativeSessionId,
    st: session.status,
    ast: session.assistantState ?? undefined,
    unr: session.hasUnread,
    aa: session.archivedAt ? toMillis(session.archivedAt) : null,
    act: toMillis(session.activityAt),
    ca: toMillis(session.createdAt),
    lu: toMillis(session.updatedAt),
    lma: session.lastMessageAt ? toMillis(session.lastMessageAt) : null,
    asu: session.assistantStateUpdatedAt ? toMillis(session.assistantStateUpdatedAt) : null,
    sk: session.sourceKind,
    ss: session.syncState,
    lsm: session.lastSyncMode ?? undefined,
    sca: session.sourceCreatedAt ? toMillis(session.sourceCreatedAt) : null,
    sua: session.sourceUpdatedAt ? toMillis(session.sourceUpdatedAt) : null,
    lsa: session.lastSyncedAt ? toMillis(session.lastSyncedAt) : null,
    tp: session.threadPath,
    tpv: session.threadPreview,
    tc: session.turnCount,
    ic: session.itemCount,
    se: session.syncError,
    usa: {
      in: session.usage.inputTokens,
      cin: session.usage.cachedInputTokens,
      out: session.usage.outputTokens,
    },
    cea: {
      in: session.contextEstimate.inputTokens,
      cin: session.contextEstimate.cachedInputTokens,
      out: session.contextEstimate.outputTokens,
      usd: session.contextEstimate.usedTokens,
    },
    cem: session.contextEstimateMode,
    lcca: session.lastContextCompactionAt ? toMillis(session.lastContextCompactionAt) : null,
    cost: session.usage.cost,
    cwt: session.contextWindowTokens,
    cws: session.contextWindowSource,
  };
}

class FakeWebSocket {
  static OPEN = 1;
  static instances: FakeWebSocket[] = [];

  url: string;
  readyState = 0;
  onopen: ((event: unknown) => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: ((event: unknown) => void) | null = null;
  onclose: (() => void) | null = null;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
    queueMicrotask(() => {
      this.readyState = FakeWebSocket.OPEN;
      this.onopen?.({});
    });
  }

  send(_payload: string) {}

  dispatch(frame: unknown) {
    this.onmessage?.({
      data: JSON.stringify(frame),
    });
  }

  close() {
    this.readyState = 3;
    this.onclose?.();
  }
}

function findSocket(url: string) {
  return FakeWebSocket.instances.find(instance => instance.url === url) ?? null;
}

describe('webSession loading behavior', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    const localStorage = createStorageMock();
    vi.stubGlobal('localStorage', localStorage);
    vi.stubGlobal('window', {
      localStorage,
      location: {
        protocol: 'http:',
        host: 'localhost:5173',
      },
      setTimeout,
      clearTimeout,
    });
    vi.stubGlobal('WebSocket', FakeWebSocket);
    FakeWebSocket.instances = [];
    listMock.mockReset();
    queryArchivedMock.mockReset();
    snapshotMock.mockReset();
    historyMock.mockReset();
    syncMock.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('loads archived session snapshots over HTTP without replacing the active session', async () => {
    const store = useWebSessionStore();
    const currentSession = makeSession({
      id: 'session-current',
      title: 'Current Session',
    });
    const archivedSession = makeSession({
      id: 'session-archived',
      title: 'Archived Session',
      archivedAt: '2026-04-09T11:00:00.000Z',
      status: 'done',
      syncState: 'fresh',
      itemCount: 1,
    });

    listMock.mockResolvedValue([currentSession]);
    queryArchivedMock.mockResolvedValue({
      items: [archivedSession],
      total: 1,
      hasMore: false,
      nextOffset: 1,
    });
    snapshotMock.mockResolvedValue({
      session: archivedSession,
      history: {
        items: [
          {
            id: 'history-archived',
            oi: 1,
            kd: 'assistant',
            tp: 'message',
            txt: 'Recovered archived history',
            ts2: Date.parse('2026-04-09T11:05:00.000Z'),
          },
        ],
        hasMore: false,
        total: 1,
      },
    });

    await store.loadSessions(currentSession.projectId);
    await store.loadArchivedSessions([currentSession.projectId], {
      reset: true,
      limit: 20,
    });
    store.setActiveSession(currentSession.projectId, currentSession.id);

    await store.loadSessionSnapshot(archivedSession.projectId, archivedSession.id, {
      rememberActive: false,
    });

    expect(snapshotMock).toHaveBeenCalledWith(archivedSession.projectId, archivedSession.id);
    expect(store.getActiveSessionId(currentSession.projectId)).toBe(currentSession.id);
    expect(store.getBlocks(archivedSession.id)).toHaveLength(1);
  });

  it('loads older history pages over HTTP and merges them into the session timeline', async () => {
    const store = useWebSessionStore();
    const session = makeSession({
      id: 'session-history',
      status: 'done',
      itemCount: 3,
      syncState: 'fresh',
    });

    listMock.mockResolvedValue([session]);
    snapshotMock.mockResolvedValue({
      session,
      history: {
        items: [
          {
            id: 'history-2',
            oi: 2,
            kd: 'assistant',
            tp: 'message',
            txt: 'second',
            ts2: Date.parse('2026-04-09T10:02:00.000Z'),
          },
          {
            id: 'history-3',
            oi: 3,
            kd: 'assistant',
            tp: 'message',
            txt: 'third',
            ts2: Date.parse('2026-04-09T10:03:00.000Z'),
          },
        ],
        hasMore: true,
        beforeCursor: '2',
        total: 3,
      },
    });
    historyMock.mockResolvedValue({
      items: [
        {
          id: 'history-1',
          oi: 1,
          kd: 'user',
          tp: 'message',
          txt: 'first',
          ts2: Date.parse('2026-04-09T10:01:00.000Z'),
        },
      ],
      hasMore: false,
      beforeCursor: '',
      total: 3,
    });

    await store.loadSessions(session.projectId);
    await store.loadSessionSnapshot(session.projectId, session.id);
    await store.loadMoreHistory(session.id, 80);

    expect(historyMock).toHaveBeenCalledWith(session.projectId, session.id, {
      beforeCursor: '2',
      limit: 80,
    });
    expect(store.getBlocks(session.id).map(item => item.orderIndex)).toEqual([1, 2, 3]);
    expect(store.getHistoryMeta(session.id)).toMatchObject({
      hasMore: false,
      beforeCursor: '',
      total: 3,
      loading: false,
    });
  });

  it('keeps completion notifications driven by the dedicated event stream websocket', async () => {
    const store = useWebSessionStore();
    const runningSession = makeSession({
      id: 'session-running',
      status: 'running',
      assistantState: null,
    });
    const doneSession = makeSession({
      ...runningSession,
      status: 'done',
      assistantState: null,
      updatedAt: '2026-04-09T10:05:00.000Z',
      lastMessageAt: '2026-04-09T10:05:00.000Z',
    });
    const handleCompleted = vi.fn();

    listMock.mockResolvedValue([runningSession]);
    await store.loadSessions(runningSession.projectId);
    store.emitter.on('ai:completed', handleCompleted);

    await store.openEventStream();
    const eventSocket = findSocket('/api/v1/web-sessions/events');
    expect(eventSocket).not.toBeNull();

    eventSocket?.dispatch({
      v: 1,
      k: 'evt',
      sid: runningSession.id,
      ts: Date.now(),
      op: 'session',
      s: toWireSession(doneSession),
    });

    expect(handleCompleted).toHaveBeenCalledWith(
      expect.objectContaining({
        sessionId: runningSession.id,
        projectId: runningSession.projectId,
      })
    );

    eventSocket?.dispatch({
      v: 1,
      k: 'evt',
      sid: runningSession.id,
      ts: Date.now(),
      op: 'session',
      s: toWireSession(runningSession),
    });
    eventSocket?.dispatch({
      v: 1,
      k: 'evt',
      sid: runningSession.id,
      ts: Date.now(),
      op: 'session',
      s: toWireSession(doneSession),
    });

    expect(handleCompleted).toHaveBeenCalledTimes(1);
    store.emitter.off('ai:completed', handleCompleted);
  });

  it('keeps realtime user message attachments on incoming history items', async () => {
    const store = useWebSessionStore();
    const session = makeSession({
      id: 'session-live-attachments',
      status: 'running',
      assistantState: null,
      itemCount: 0,
    });

    listMock.mockResolvedValue([session]);
    await store.loadSessions(session.projectId);
    await store.openEventStream();

    const eventSocket = findSocket('/api/v1/web-sessions/events');
    expect(eventSocket).not.toBeNull();

    eventSocket?.dispatch({
      v: 1,
      k: 'evt',
      sid: session.id,
      ts: Date.now(),
      op: 'hist_item',
      i: {
        id: 'history-live-1',
        oi: 1,
        kd: 'user',
        tp: 'user_message',
        txt: 'hello [Image #1]',
        ts2: Date.parse('2026-04-09T10:01:00.000Z'),
        atts: [
          {
            id: 'att-live-1',
            name: 'image.png',
            mime: 'image/png',
            sz: 42,
          },
        ],
      },
    });

    const blocks = store.getBlocks(session.id);
    expect(blocks).toHaveLength(1);
    expect(blocks[0]?.attachments).toEqual([
      expect.objectContaining({
        id: 'att-live-1',
        name: 'image.png',
        mime: 'image/png',
        size: 42,
      }),
    ]);
  });
});
