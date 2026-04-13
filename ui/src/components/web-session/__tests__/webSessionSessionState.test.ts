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

  it('maps plan approval to a distinct pill state while keeping the approval label', () => {
    const state = resolveWebSessionDisplayState(
      makeInput({
        livePhase: 'waiting_plan_approval',
        hasUnread: true,
      })
    );

    expect(state.assistantStateClass).toBe('waiting_plan_approval');
    expect(state.statusLabelKey).toBe('terminal.aiStatusWaitingApproval');
    expect(state.pillStateClass).toBe('waiting_plan_approval');
    expect(state.attentionStateClass).toBe('plan_approval');
    expect(state.hasUnviewedApproval).toBe(true);
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
    expect(state.attentionStateClass).toBe('waiting_input');
    expect(state.hasUnviewedApproval).toBe(false);
    expect(state.showStatusDot).toBe(false);
    expect(state.statusDotClass).toBeNull();
  });

  it('keeps waiting_input labels while promoting real sessions to approval attention', () => {
    const state = resolveWebSessionDisplayState(
      makeInput({
        livePhase: 'waiting_input',
        hasUnread: true,
        status: 'running',
      })
    );

    expect(state.assistantStateClass).toBe('waiting_input');
    expect(state.statusLabelKey).toBe('terminal.aiStatusWaitingInput');
    expect(state.pillStateClass).toBe('waiting_input');
    expect(state.attentionStateClass).toBe('approval');
    expect(state.hasUnviewedApproval).toBe(true);
  });

  it('uses completion pills for unread finished sessions', () => {
    const state = resolveWebSessionDisplayState(
      makeInput({
        hasUnread: true,
        livePhase: 'done',
        status: 'done',
      })
    );

    expect(state.assistantStateClass).toBe('idle');
    expect(state.hasUnviewedCompletion).toBe(true);
    expect(state.pillStateClass).toBe('completion');
    expect(state.statusLabelKey).toBe('terminal.aiStatusDone');
  });

  it('treats done and idle sessions as idle instead of waiting for input', () => {
    const doneState = resolveWebSessionDisplayState(
      makeInput({
        hasUnread: false,
        livePhase: 'done',
        status: 'done',
      })
    );
    const idleState = resolveWebSessionDisplayState(
      makeInput({
        hasUnread: false,
        livePhase: 'idle',
        status: 'idle',
      })
    );

    expect(doneState.assistantStateClass).toBe('idle');
    expect(doneState.statusLabelKey).toBe('terminal.aiIdle');
    expect(idleState.assistantStateClass).toBe('idle');
    expect(idleState.statusLabelKey).toBe('terminal.aiIdle');
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
    expect(state.attentionStateClass).toBe('approval');
  });

  it('shows error dots and ignores legacy stale sync state markers', () => {
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
    expect(staleState.showStatusDot).toBe(false);
    expect(staleState.statusDotClass).toBeNull();
  });
});
