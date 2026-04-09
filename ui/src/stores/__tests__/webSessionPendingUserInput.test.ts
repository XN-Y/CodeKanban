import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { WebSessionSummary } from '@/types/models';
import { useWebSessionStore } from '@/stores/webSession';

const { listMock, syncMock } = vi.hoisted(() => ({
  listMock: vi.fn(),
  syncMock: vi.fn(),
}));

vi.mock('@/api/webSession', () => ({
  webSessionApi: {
    list: listMock,
    sync: syncMock,
  },
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

describe('webSession pending user input', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.stubGlobal('localStorage', createStorageMock());
    listMock.mockReset();
    syncMock.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('recovers sourceItemId from payload.iid for older user input history items', async () => {
    const store = useWebSessionStore();
    const session = makeSession();
    const requestID = 'req_input_123';

    listMock.mockResolvedValue([session]);
    syncMock.mockResolvedValue({
      session,
      history: {
        items: [
          {
            id: 'history-item-1',
            oi: 1,
            kd: 'system',
            tp: 'user_input_request',
            txt: 'Please choose a scope',
            ts2: Date.parse('2026-04-09T10:00:00.000Z'),
            dt: {
              type: 'user_input_request',
              prompt: 'Please choose a scope',
              questions: [
                {
                  id: 'scope',
                  header: 'Scope',
                  question: 'Which scope should I use?',
                  isOther: false,
                  isSecret: false,
                  options: [
                    {
                      label: 'Full migration',
                      description: 'Apply all changes',
                    },
                  ],
                },
              ],
            },
            pl: {
              iid: requestID,
            },
          },
        ],
        hasMore: false,
        total: 1,
      },
    });

    await store.loadSessions(session.projectId);
    await store.syncSession(session.projectId, session.id);

    const blocks = store.getBlocks(session.id);
    expect(blocks).toHaveLength(1);
    expect(blocks[0]?.sourceItemId).toBe(requestID);

    const pending = store.getPendingUserInput(session.id);
    expect(pending?.itemId).toBe(requestID);
    expect(pending?.prompt).toBe('Please choose a scope');
  });
});
