import { describe, expect, it } from 'vitest';

import type { WebSessionSummary } from '@/types/models';
import {
  buildWebSessionSnapshotVersion,
  compareWebSessionSnapshotVersion,
  shouldApplyIncomingWebSessionSnapshot,
} from '@/stores/webSessionSnapshotVersion';

function makeSession(overrides: Partial<WebSessionSummary> = {}): WebSessionSummary {
  return {
    id: 'session-1',
    projectId: 'project-1',
    worktreeId: null,
    orderIndex: 1,
    agent: 'codex',
    title: 'Codex Session',
    model: 'gpt-5.4',
    reasoningEffort: 'medium',
    workflowMode: 'default',
    permissionLevel: 'elevated',
    cwd: '/tmp/project',
    nativeSessionId: 'native-1',
    status: 'done',
    assistantState: null,
    hasUnread: false,
    archivedAt: null,
    activityAt: '2026-04-09T10:00:00.000Z',
    lastMessageAt: '2026-04-09T10:00:00.000Z',
    assistantStateUpdatedAt: null,
    sourceKind: 'codex_app_server',
    syncState: 'fresh',
    lastSyncMode: 'fast',
    sourceCreatedAt: '2026-04-09T09:00:00.000Z',
    sourceUpdatedAt: '2026-04-09T10:00:00.000Z',
    lastSyncedAt: '2026-04-09T10:00:00.000Z',
    threadPath: '/tmp/session.jsonl',
    threadPreview: 'preview',
    turnCount: 3,
    itemCount: 6,
    syncError: null,
    createdAt: '2026-04-09T09:00:00.000Z',
    updatedAt: '2026-04-09T10:00:00.000Z',
    usage: {
      inputTokens: 1,
      cachedInputTokens: 0,
      outputTokens: 1,
      cost: 0,
    },
    contextWindowTokens: null,
    contextWindowSource: 'default',
    ...overrides,
  };
}

describe('webSessionSnapshotVersion', () => {
  it('orders versions by updatedAt first', () => {
    const older = buildWebSessionSnapshotVersion({
      session: makeSession({
        updatedAt: '2026-04-09T10:00:00.000Z',
      }),
      historyTotal: 6,
    });
    const newer = buildWebSessionSnapshotVersion({
      session: makeSession({
        updatedAt: '2026-04-09T10:00:01.000Z',
      }),
      historyTotal: 6,
    });

    expect(compareWebSessionSnapshotVersion(newer, older)).toBeGreaterThan(0);
    expect(compareWebSessionSnapshotVersion(older, newer)).toBeLessThan(0);
  });

  it('rejects incoming snapshots with an older lastSyncedAt when updatedAt matches', () => {
    const current = makeSession({
      updatedAt: '2026-04-09T10:00:05.000Z',
      lastSyncedAt: '2026-04-09T10:00:05.000Z',
    });
    const incoming = makeSession({
      updatedAt: '2026-04-09T10:00:05.000Z',
      lastSyncedAt: '2026-04-09T10:00:01.000Z',
    });

    expect(
      shouldApplyIncomingWebSessionSnapshot({
        currentSnapshot: {
          session: current,
          historyTotal: 6,
        },
        incomingSnapshot: {
          session: incoming,
          historyTotal: 6,
        },
      })
    ).toBe(false);
  });

  it('rejects syncing snapshots when a settled snapshot exists at the same timestamp', () => {
    const current = makeSession({
      updatedAt: '2026-04-09T10:00:05.000Z',
      lastSyncedAt: '2026-04-09T10:00:05.000Z',
      syncState: 'fresh',
    });
    const incoming = makeSession({
      updatedAt: '2026-04-09T10:00:05.000Z',
      lastSyncedAt: '2026-04-09T10:00:05.000Z',
      syncState: 'syncing',
    });

    expect(
      shouldApplyIncomingWebSessionSnapshot({
        currentSnapshot: {
          session: current,
          historyTotal: 6,
        },
        incomingSnapshot: {
          session: incoming,
          historyTotal: 6,
        },
      })
    ).toBe(false);
  });

  it('rejects snapshots with fewer items when timestamps are equal', () => {
    const current = makeSession({
      updatedAt: '2026-04-09T10:00:05.000Z',
      lastSyncedAt: '2026-04-09T10:00:05.000Z',
      itemCount: 12,
    });
    const incoming = makeSession({
      updatedAt: '2026-04-09T10:00:05.000Z',
      lastSyncedAt: '2026-04-09T10:00:05.000Z',
      itemCount: 9,
    });

    expect(
      shouldApplyIncomingWebSessionSnapshot({
        currentSnapshot: {
          session: current,
          historyTotal: 12,
        },
        incomingSnapshot: {
          session: incoming,
          historyTotal: 9,
        },
      })
    ).toBe(false);
  });

  it('keeps a newer applied version authoritative even if current session state is older', () => {
    const appliedVersion = buildWebSessionSnapshotVersion({
      session: makeSession({
        updatedAt: '2026-04-09T10:00:08.000Z',
        lastSyncedAt: '2026-04-09T10:00:08.000Z',
        itemCount: 14,
      }),
      historyTotal: 14,
    });

    expect(
      shouldApplyIncomingWebSessionSnapshot({
        appliedVersion,
        currentSnapshot: {
          session: makeSession({
            updatedAt: '2026-04-09T10:00:04.000Z',
            lastSyncedAt: '2026-04-09T10:00:04.000Z',
            itemCount: 7,
          }),
          historyTotal: 7,
        },
        incomingSnapshot: {
          session: makeSession({
            updatedAt: '2026-04-09T10:00:06.000Z',
            lastSyncedAt: '2026-04-09T10:00:06.000Z',
            itemCount: 10,
          }),
          historyTotal: 10,
        },
      })
    ).toBe(false);
  });

  it('accepts snapshots that are newer or equally complete', () => {
    const current = makeSession({
      updatedAt: '2026-04-09T10:00:05.000Z',
      lastSyncedAt: '2026-04-09T10:00:05.000Z',
      itemCount: 8,
    });
    const incoming = makeSession({
      updatedAt: '2026-04-09T10:00:06.000Z',
      lastSyncedAt: '2026-04-09T10:00:06.000Z',
      itemCount: 10,
    });

    expect(
      shouldApplyIncomingWebSessionSnapshot({
        currentSnapshot: {
          session: current,
          historyTotal: 8,
        },
        incomingSnapshot: {
          session: incoming,
          historyTotal: 10,
        },
      })
    ).toBe(true);

    expect(
      shouldApplyIncomingWebSessionSnapshot({
        currentSnapshot: {
          session: incoming,
          historyTotal: 10,
        },
        incomingSnapshot: {
          session: incoming,
          historyTotal: 10,
        },
      })
    ).toBe(true);
  });
});
