import { describe, expect, it } from 'vitest';

import {
  buildWebSessionSendConfirmationSignature,
  findWebSessionSendConflicts,
  resolveWebSessionSendConfirmation,
} from '@/components/web-session/webSessionSendGuard';

function makeSession(
  overrides: Partial<{
    id: string;
    title: string;
    workflowMode: 'default' | 'plan';
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
      | 'error';
  }> = {}
) {
  return {
    id: 'session-1',
    title: 'Session 1',
    workflowMode: 'default' as const,
    livePhase: 'starting' as const,
    ...overrides,
  };
}

describe('webSessionSendGuard', () => {
  it('finds only other non-plan sessions that are actively executing', () => {
    const conflicts = findWebSessionSendConflicts({
      currentSessionId: 'session-1',
      sessions: [
        makeSession({ id: 'session-1', livePhase: 'starting' }),
        makeSession({ id: 'session-2', title: 'Default Thinking', livePhase: 'thinking' }),
        makeSession({
          id: 'session-3',
          title: 'Plan Tool',
          workflowMode: 'plan',
          livePhase: 'tool',
        }),
        makeSession({ id: 'session-4', title: 'Default Retrying', livePhase: 'retrying' }),
        makeSession({ id: 'session-5', title: 'Waiting Approval', livePhase: 'waiting_approval' }),
      ],
    });

    expect(conflicts.map(session => session.id)).toEqual(['session-2', 'session-4']);
  });

  it('ignores waiting and completed phases even when the workflow is default', () => {
    const conflicts = findWebSessionSendConflicts({
      sessions: [
        makeSession({ id: 'session-waiting-input', livePhase: 'waiting_input' }),
        makeSession({ id: 'session-waiting-approval', livePhase: 'waiting_approval' }),
        makeSession({ id: 'session-waiting-plan', livePhase: 'waiting_plan_approval' }),
        makeSession({ id: 'session-done', livePhase: 'done' }),
        makeSession({ id: 'session-idle', livePhase: 'idle' }),
        makeSession({ id: 'session-error', livePhase: 'error' }),
      ],
    });

    expect(conflicts).toEqual([]);
  });

  it('normalizes attachment and conflict ids when building the confirmation signature', () => {
    const first = buildWebSessionSendConfirmationSignature({
      ownerId: ' draft-session ',
      text: 'Implement this',
      attachmentIds: ['b', 'a', 'b', ''],
      conflictSessionIds: ['session-2', 'session-1'],
    });
    const second = buildWebSessionSendConfirmationSignature({
      ownerId: 'draft-session',
      text: 'Implement this',
      attachmentIds: ['a', 'b'],
      conflictSessionIds: ['session-1', 'session-2'],
    });

    expect(first).toBe(second);
  });

  it('arms confirmation on the first conflicting send and clears it on the second click', () => {
    const signature = buildWebSessionSendConfirmationSignature({
      ownerId: 'draft-session',
      text: 'Implement this',
      attachmentIds: ['attachment-1'],
      conflictSessionIds: ['session-2'],
    });
    const conflicts = [makeSession({ id: 'session-2', title: 'Busy Session' })];

    const firstAttempt = resolveWebSessionSendConfirmation({
      conflicts,
      currentState: null,
      signature,
      now: 1000,
      ttlMs: 5000,
    });

    expect(firstAttempt.shouldProceed).toBe(false);
    expect(firstAttempt.nextState).toEqual({
      signature,
      expiresAt: 6000,
    });

    const secondAttempt = resolveWebSessionSendConfirmation({
      conflicts,
      currentState: firstAttempt.nextState,
      signature,
      now: 1500,
      ttlMs: 5000,
    });

    expect(secondAttempt.shouldProceed).toBe(true);
    expect(secondAttempt.nextState).toBeNull();
  });
});
