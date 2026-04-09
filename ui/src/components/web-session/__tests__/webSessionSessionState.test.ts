import { describe, expect, it } from 'vitest';

import { resolveWebSessionDisplayState } from '@/components/web-session/webSessionSessionState';

function makeInput(
  overrides: Partial<{
    isDraft: boolean;
    hasUnread: boolean;
    status: 'idle' | 'running' | 'waiting_approval' | 'done' | 'err' | 'aborting';
    syncState: 'fresh' | 'stale' | 'missing' | 'syncing' | 'error';
    livePhase:
      | 'idle'
      | 'starting'
      | 'thinking'
      | 'tool'
      | 'retrying'
      | 'waiting_approval'
      | 'waiting_plan_approval'
      | 'waiting_input'
      | 'done'
      | 'error'
      | null;
    assistantState:
      | 'working'
      | 'waiting_approval'
      | 'waiting_input'
      | 'waiting_plan_approval'
      | null;
  }> = {}
) {
  return {
    isDraft: false,
    hasUnread: false,
    status: 'running' as const,
    syncState: 'fresh' as const,
    livePhase: 'idle' as const,
    assistantState: null,
    ...overrides,
  };
}

describe('webSessionSessionState', () => {
  it('maps working phases to working labels and pills', () => {
    const state = resolveWebSessionDisplayState(makeInput({ livePhase: 'retrying' }));

    expect(state.assistantStateClass).toBe('working');
    expect(state.statusLabelKey).toBe('terminal.aiStatusWorking');
    expect(state.pillStateClass).toBe('working');
    expect(state.statusEmoji).toBe('🤔');
  });

  it('maps plan approval to the approval label', () => {
    const state = resolveWebSessionDisplayState(
      makeInput({
        livePhase: 'waiting_plan_approval',
      })
    );

    expect(state.assistantStateClass).toBe('waiting_approval');
    expect(state.statusLabelKey).toBe('terminal.aiStatusWaitingApproval');
    expect(state.hasUnviewedApproval).toBe(false);
  });

  it('treats draft sessions as waiting for input without status dots', () => {
    const state = resolveWebSessionDisplayState(
      makeInput({
        isDraft: true,
        livePhase: null,
        status: 'idle',
        syncState: 'fresh',
      })
    );

    expect(state.assistantStateClass).toBe('waiting_input');
    expect(state.statusLabelKey).toBe('terminal.aiStatusWaitingInput');
    expect(state.showStatusDot).toBe(false);
    expect(state.statusDotClass).toBeNull();
  });

  it('uses completion pills for unread finished sessions', () => {
    const state = resolveWebSessionDisplayState(
      makeInput({
        hasUnread: true,
        livePhase: 'done',
        status: 'done',
      })
    );

    expect(state.assistantStateClass).toBe('waiting_input');
    expect(state.hasUnviewedCompletion).toBe(true);
    expect(state.pillStateClass).toBe('completion');
  });

  it('falls back to assistantState when livePhase is unavailable', () => {
    const state = resolveWebSessionDisplayState(
      makeInput({
        livePhase: null,
        assistantState: 'waiting_input',
        status: 'idle',
      })
    );

    expect(state.assistantStateClass).toBe('waiting_input');
    expect(state.statusLabelKey).toBe('terminal.aiStatusWaitingInput');
  });

  it('shows error dots and prefers stale dots over status dots', () => {
    const errorState = resolveWebSessionDisplayState(
      makeInput({
        livePhase: 'error',
        status: 'err',
      })
    );
    const staleState = resolveWebSessionDisplayState(
      makeInput({
        livePhase: 'idle',
        status: 'running',
        syncState: 'stale',
      })
    );

    expect(errorState.showStatusDot).toBe(true);
    expect(errorState.statusDotClass).toBe('err');
    expect(staleState.showStatusDot).toBe(true);
    expect(staleState.statusDotClass).toBe('stale');
  });
});
