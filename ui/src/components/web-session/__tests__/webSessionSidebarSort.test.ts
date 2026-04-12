import { describe, expect, it } from 'vitest';

import { resolveWebSessionSidebarSortTimestamp } from '@/components/web-session/webSessionSessionState';
import type { WebSessionSummary } from '@/types/models';

function makeSession(
  overrides: Partial<
    Pick<
      WebSessionSummary,
      'statusUpdatedAt' | 'assistantStateUpdatedAt' | 'updatedAt' | 'createdAt'
    >
  > = {}
) {
  return {
    statusUpdatedAt: '2026-04-10T10:00:00.000Z',
    assistantStateUpdatedAt: '2026-04-10T09:59:00.000Z',
    updatedAt: '2026-04-10T09:58:00.000Z',
    createdAt: '2026-04-10T09:00:00.000Z',
    ...overrides,
  };
}

describe('webSessionSidebarSort', () => {
  it('prefers the dedicated status transition timestamp', () => {
    const session = makeSession({
      statusUpdatedAt: '2026-04-10T11:00:00.000Z',
      assistantStateUpdatedAt: '2026-04-10T10:00:00.000Z',
      updatedAt: '2026-04-10T09:00:00.000Z',
    });

    expect(resolveWebSessionSidebarSortTimestamp(session)).toBe(
      Date.parse('2026-04-10T11:00:00.000Z')
    );
  });

  it('falls back to assistant state updates when the dedicated timestamp is missing', () => {
    const session = makeSession({
      statusUpdatedAt: null,
      assistantStateUpdatedAt: '2026-04-10T10:30:00.000Z',
      updatedAt: '2026-04-10T09:00:00.000Z',
    });

    expect(resolveWebSessionSidebarSortTimestamp(session)).toBe(
      Date.parse('2026-04-10T10:30:00.000Z')
    );
  });

  it('falls back to updatedAt and then createdAt for older records', () => {
    expect(
      resolveWebSessionSidebarSortTimestamp(
        makeSession({
          statusUpdatedAt: null,
          assistantStateUpdatedAt: null,
          updatedAt: '2026-04-10T10:15:00.000Z',
        })
      )
    ).toBe(Date.parse('2026-04-10T10:15:00.000Z'));

    expect(
      resolveWebSessionSidebarSortTimestamp(
        makeSession({
          statusUpdatedAt: null,
          assistantStateUpdatedAt: null,
          updatedAt: '',
          createdAt: '2026-04-10T08:30:00.000Z',
        })
      )
    ).toBe(Date.parse('2026-04-10T08:30:00.000Z'));
  });
});
