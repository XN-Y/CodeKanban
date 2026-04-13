import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { WebSessionSummary } from '@/types/models';
import { useWebSessionStore } from '@/stores/webSession';

const { listMock, queryArchivedMock, archiveMock, unarchiveMock, deleteMock, snapshotMock } =
  vi.hoisted(() => ({
    listMock: vi.fn(),
    queryArchivedMock: vi.fn(),
    archiveMock: vi.fn(),
    unarchiveMock: vi.fn(),
    deleteMock: vi.fn(),
    snapshotMock: vi.fn(),
  }));

vi.mock('@/api/webSession', () => ({
  webSessionApi: {
    list: listMock,
    queryArchived: queryArchivedMock,
    archive: archiveMock,
    unarchive: unarchiveMock,
    delete: deleteMock,
    snapshot: snapshotMock,
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
    title: 'Session',
    model: 'gpt-5.4',
    reasoningEffort: 'medium',
    workflowMode: 'default',
    permissionLevel: 'elevated',
    cwd: '/tmp/project',
    nativeSessionId: 'native-1',
    status: 'idle',
    assistantState: null,
    hasUnread: false,
    archivedAt: null,
    activityAt: '2026-04-10T10:00:00.000Z',
    lastMessageAt: '2026-04-10T10:00:00.000Z',
    assistantStateUpdatedAt: null,
    sourceKind: 'codex_app_server',
    syncState: 'fresh',
    lastSyncMode: 'fast',
    sourceCreatedAt: '2026-04-10T09:00:00.000Z',
    sourceUpdatedAt: '2026-04-10T10:00:00.000Z',
    lastSyncedAt: '2026-04-10T10:00:00.000Z',
    threadPath: '/tmp/session.jsonl',
    threadPreview: 'preview',
    turnCount: 1,
    itemCount: 1,
    syncError: null,
    createdAt: '2026-04-10T09:00:00.000Z',
    updatedAt: '2026-04-10T10:00:00.000Z',
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

describe('webSession archived scopes', () => {
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
      setInterval,
      clearInterval,
    });
    listMock.mockReset();
    queryArchivedMock.mockReset();
    archiveMock.mockReset();
    unarchiveMock.mockReset();
    deleteMock.mockReset();
    snapshotMock.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('keeps cross-project and current-project archived scopes independent while mutations sync both', async () => {
    const store = useWebSessionStore();
    const currentSession = makeSession({
      id: 'current-p1',
      title: 'Current P1',
      activityAt: '2026-04-10T14:00:00.000Z',
      updatedAt: '2026-04-10T14:00:00.000Z',
      lastMessageAt: '2026-04-10T14:00:00.000Z',
    });
    const archivedP1 = makeSession({
      id: 'archived-p1',
      title: 'Archived P1',
      archivedAt: '2026-04-10T12:00:00.000Z',
      activityAt: '2026-04-10T12:00:00.000Z',
      updatedAt: '2026-04-10T12:00:00.000Z',
      lastMessageAt: '2026-04-10T12:00:00.000Z',
    });
    const archivedP2 = makeSession({
      id: 'archived-p2',
      projectId: 'project-2',
      title: 'Archived P2',
      archivedAt: '2026-04-10T13:00:00.000Z',
      activityAt: '2026-04-10T13:00:00.000Z',
      updatedAt: '2026-04-10T13:00:00.000Z',
      lastMessageAt: '2026-04-10T13:00:00.000Z',
    });
    const archivedCurrent = {
      ...currentSession,
      archivedAt: '2026-04-10T15:00:00.000Z',
      activityAt: '2026-04-10T15:00:00.000Z',
      updatedAt: '2026-04-10T15:00:00.000Z',
      lastMessageAt: '2026-04-10T15:00:00.000Z',
    };

    listMock.mockResolvedValue([currentSession]);
    queryArchivedMock.mockImplementation(({ projectIds }: { projectIds: string[] }) => {
      const scopeKey = [...projectIds].sort().join('::');
      if (scopeKey === 'project-1::project-2') {
        return Promise.resolve({
          items: [archivedP2, archivedP1],
          total: 4,
          hasMore: true,
          nextOffset: 2,
        });
      }
      if (scopeKey === 'project-1') {
        return Promise.resolve({
          items: [archivedP1],
          total: 3,
          hasMore: true,
          nextOffset: 1,
        });
      }
      throw new Error(`unexpected archived scope ${scopeKey}`);
    });
    archiveMock.mockResolvedValue(archivedCurrent);
    unarchiveMock.mockResolvedValue(currentSession);

    await store.loadSessions('project-1');
    await store.loadArchivedSessions(['project-1', 'project-2'], {
      reset: true,
      limit: 20,
    });
    await store.loadArchivedSessions(['project-1'], {
      reset: true,
      limit: 20,
    });

    expect(store.getArchivedSessions(['project-1', 'project-2']).map(item => item.id)).toEqual([
      'archived-p2',
      'archived-p1',
    ]);
    expect(store.getArchivedSessions(['project-1']).map(item => item.id)).toEqual(['archived-p1']);
    expect(store.getArchivedMeta(['project-1', 'project-2'])).toMatchObject({
      scopeKey: 'project-1::project-2',
      total: 4,
      offset: 2,
      hasMore: true,
    });
    expect(store.getArchivedMeta(['project-1'])).toMatchObject({
      scopeKey: 'project-1',
      total: 3,
      offset: 1,
      hasMore: true,
    });

    await store.archiveSession('project-1', currentSession.id);

    expect(store.getArchivedSessions(['project-1']).map(item => item.id)).toEqual([
      'current-p1',
      'archived-p1',
    ]);
    expect(store.getArchivedSessions(['project-1', 'project-2']).map(item => item.id)).toEqual([
      'current-p1',
      'archived-p2',
      'archived-p1',
    ]);
    expect(store.getArchivedMeta(['project-1'])).toMatchObject({
      total: 4,
      offset: 2,
      hasMore: true,
    });
    expect(store.getArchivedMeta(['project-1', 'project-2'])).toMatchObject({
      total: 5,
      offset: 3,
      hasMore: true,
    });

    await store.unarchiveSession('project-1', currentSession.id);

    expect(store.getArchivedSessions(['project-1']).map(item => item.id)).toEqual(['archived-p1']);
    expect(store.getArchivedSessions(['project-1', 'project-2']).map(item => item.id)).toEqual([
      'archived-p2',
      'archived-p1',
    ]);
    expect(store.getArchivedMeta(['project-1'])).toMatchObject({
      total: 3,
      offset: 1,
      hasMore: true,
    });
    expect(store.getArchivedMeta(['project-1', 'project-2'])).toMatchObject({
      total: 4,
      offset: 2,
      hasMore: true,
    });
  });

  it('removes deleted archived sessions from every loaded scope and rebalances offsets', async () => {
    const store = useWebSessionStore();
    const currentSession = makeSession({
      id: 'current-p1',
      title: 'Current P1',
    });
    const archivedP1 = makeSession({
      id: 'archived-p1',
      title: 'Archived P1',
      archivedAt: '2026-04-10T12:00:00.000Z',
      activityAt: '2026-04-10T12:00:00.000Z',
      updatedAt: '2026-04-10T12:00:00.000Z',
      lastMessageAt: '2026-04-10T12:00:00.000Z',
    });
    const archivedP2 = makeSession({
      id: 'archived-p2',
      projectId: 'project-2',
      title: 'Archived P2',
      archivedAt: '2026-04-10T13:00:00.000Z',
      activityAt: '2026-04-10T13:00:00.000Z',
      updatedAt: '2026-04-10T13:00:00.000Z',
      lastMessageAt: '2026-04-10T13:00:00.000Z',
    });

    listMock.mockResolvedValue([currentSession]);
    queryArchivedMock.mockImplementation(({ projectIds }: { projectIds: string[] }) => {
      const scopeKey = [...projectIds].sort().join('::');
      if (scopeKey === 'project-1::project-2') {
        return Promise.resolve({
          items: [archivedP2, archivedP1],
          total: 4,
          hasMore: true,
          nextOffset: 2,
        });
      }
      if (scopeKey === 'project-1') {
        return Promise.resolve({
          items: [archivedP1],
          total: 3,
          hasMore: true,
          nextOffset: 1,
        });
      }
      throw new Error(`unexpected archived scope ${scopeKey}`);
    });
    deleteMock.mockResolvedValue(undefined);

    await store.loadSessions('project-1');
    await store.loadArchivedSessions(['project-1', 'project-2'], {
      reset: true,
      limit: 20,
    });
    await store.loadArchivedSessions(['project-1'], {
      reset: true,
      limit: 20,
    });

    await store.deleteSession('project-1', archivedP1.id);

    expect(store.getArchivedSessions(['project-1']).map(item => item.id)).toEqual([]);
    expect(store.getArchivedSessions(['project-1', 'project-2']).map(item => item.id)).toEqual([
      'archived-p2',
    ]);
    expect(store.getArchivedMeta(['project-1'])).toMatchObject({
      total: 2,
      offset: 0,
      hasMore: true,
    });
    expect(store.getArchivedMeta(['project-1', 'project-2'])).toMatchObject({
      total: 3,
      offset: 1,
      hasMore: true,
    });
  });

  it('preserves the loaded archived order while preview snapshots refresh an archived session', async () => {
    const store = useWebSessionStore();
    const archivedRecent = makeSession({
      id: 'archived-recent',
      archivedAt: '2026-04-10T13:00:00.000Z',
      activityAt: '2026-04-10T13:00:00.000Z',
      updatedAt: '2026-04-10T13:00:00.000Z',
      lastMessageAt: '2026-04-10T13:00:00.000Z',
    });
    const archivedPreview = makeSession({
      id: 'archived-preview',
      archivedAt: '2026-04-10T12:00:00.000Z',
      activityAt: '2026-04-10T12:00:00.000Z',
      updatedAt: '2026-04-10T12:00:00.000Z',
      lastMessageAt: '2026-04-10T12:00:00.000Z',
    });
    const refreshedPreview = {
      ...archivedPreview,
      title: 'Archived Preview Updated',
      activityAt: '2026-04-10T15:00:00.000Z',
      updatedAt: '2026-04-10T15:00:00.000Z',
      lastMessageAt: '2026-04-10T15:00:00.000Z',
    };

    queryArchivedMock.mockResolvedValue({
      items: [archivedRecent, archivedPreview],
      total: 2,
      hasMore: false,
      nextOffset: 2,
    });
    snapshotMock.mockResolvedValue({
      session: refreshedPreview,
      history: {
        items: [],
        hasMore: false,
        beforeCursor: '',
        total: 0,
      },
      pendingInputs: [],
    });

    await store.loadArchivedSessions(['project-1'], {
      reset: true,
      limit: 20,
    });
    expect(store.hasArchivedScope(['project-1'])).toBe(true);

    await store.loadSessionSnapshot('project-1', archivedPreview.id, {
      rememberActive: false,
      preserveArchivedPosition: true,
    });

    expect(store.getArchivedSessions(['project-1']).map(item => item.id)).toEqual([
      archivedRecent.id,
      archivedPreview.id,
    ]);
    expect(store.getArchivedSessions(['project-1'])[1]).toMatchObject({
      id: archivedPreview.id,
      title: refreshedPreview.title,
      activityAt: refreshedPreview.activityAt,
    });
    expect(store.getArchivedMeta(['project-1'])).toMatchObject({
      total: 2,
      offset: 2,
      hasMore: false,
    });
  });
});
