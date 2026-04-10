import { describe, expect, it } from 'vitest';

const {
  formatAiStatusTitle,
  formatAiStatusTriplet,
  formatAiStatusTripletWithTotal,
  getAiStatusSummaryTotal,
  getWebSessionStatusBucket,
  hasAiStatusSummary,
  summarizeWebSessions,
} = await import('@/composables/aiStatusSummary');

function makeSession(overrides: Partial<{ id: string; hasUnread: boolean }> = {}) {
  return {
    id: 'session-1',
    hasUnread: false,
    ...overrides,
  };
}

function makeLiveState(
  overrides: Partial<{
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
  }> = {}
) {
  return {
    phase: 'idle' as const,
    running: false,
    ...overrides,
  };
}

describe('useAiStatusSummary helpers', () => {
  it('treats waiting input as blocking even if the session is still marked running', () => {
    const bucket = getWebSessionStatusBucket(
      makeSession(),
      makeLiveState({ phase: 'waiting_input', running: true })
    );

    expect(bucket).toBe('blocking');
  });

  it('treats running web sessions as working', () => {
    const bucket = getWebSessionStatusBucket(
      makeSession(),
      makeLiveState({ phase: 'thinking', running: true })
    );

    expect(bucket).toBe('working');
  });

  it('treats unread completed web sessions as unreadCompleted', () => {
    const bucket = getWebSessionStatusBucket(
      makeSession({ hasUnread: true }),
      makeLiveState({ phase: 'done', running: false })
    );

    expect(bucket).toBe('unreadCompleted');
  });

  it('summarizes web sessions into the working/blocking/unread triplet order', () => {
    const liveStateBySession = new Map([
      ['working', makeLiveState({ phase: 'tool', running: true })],
      ['blocking-a', makeLiveState({ phase: 'waiting_approval', running: true })],
      ['blocking-b', makeLiveState({ phase: 'waiting_plan_approval', running: false })],
      ['unread-a', makeLiveState({ phase: 'done', running: false })],
      ['unread-b', makeLiveState({ phase: 'done', running: false })],
      ['unread-c', makeLiveState({ phase: 'done', running: false })],
      ['idle', makeLiveState({ phase: 'idle', running: false })],
    ]);

    const summary = summarizeWebSessions(
      [
        makeSession({ id: 'working' }),
        makeSession({ id: 'blocking-a' }),
        makeSession({ id: 'blocking-b' }),
        makeSession({ id: 'unread-a', hasUnread: true }),
        makeSession({ id: 'unread-b', hasUnread: true }),
        makeSession({ id: 'unread-c', hasUnread: true }),
        makeSession({ id: 'idle' }),
      ],
      sessionId => liveStateBySession.get(sessionId) ?? makeLiveState()
    );

    expect(summary).toEqual({
      working: 1,
      blocking: 2,
      unreadCompleted: 3,
    });
    expect(formatAiStatusTriplet(summary)).toBe('1/2/3');
    expect(getAiStatusSummaryTotal(summary)).toBe(6);
    expect(formatAiStatusTripletWithTotal(summary, 7)).toBe('1/2/3 · 7');
    expect(hasAiStatusSummary(summary)).toBe(true);
  });

  it('keeps empty summaries hidden from title decorations', () => {
    const emptySummary = summarizeWebSessions([makeSession()], () => makeLiveState());

    expect(emptySummary).toEqual({
      working: 0,
      blocking: 0,
      unreadCompleted: 0,
    });
    expect(hasAiStatusSummary(emptySummary)).toBe(false);
    expect(formatAiStatusTripletWithTotal(emptySummary, 1)).toBe('1');
    expect(formatAiStatusTripletWithTotal(emptySummary, 0)).toBe('0');
    expect(formatAiStatusTitle(emptySummary, 'CodeKanban')).toBe('CodeKanban');
  });
});
