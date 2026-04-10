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
        makeSession({
          id: 'session-1',
          title: 'Current Session',
          workflowMode: 'default',
          livePhase: 'starting',
        }),
        makeSession({
          id: 'session-2',
          title: 'Default Thinking',
          workflowMode: 'default',
          livePhase: 'thinking',
        }),
        makeSession({
          id: 'session-3',
          title: 'Plan Tool',
          workflowMode: 'plan',
          livePhase: 'tool',
        }),
        makeSession({
          id: 'session-4',
          title: 'Default Retrying',
          workflowMode: 'default',
          livePhase: 'retrying',
        }),
        makeSession({
          id: 'session-5',
          title: 'Waiting Approval',
          workflowMode: 'default',
          livePhase: 'waiting_approval',
        }),
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

  it('re-arms confirmation when the signature changes or the previous arm expires', () => {
    const signature = buildWebSessionSendConfirmationSignature({
      ownerId: 'draft-session',
      text: 'Implement this',
      attachmentIds: ['attachment-1'],
      conflictSessionIds: ['session-2'],
    });
    const changedSignature = buildWebSessionSendConfirmationSignature({
      ownerId: 'draft-session',
      text: 'Implement this now',
      attachmentIds: ['attachment-1'],
      conflictSessionIds: ['session-2'],
    });
    const existingState = {
      signature,
      expiresAt: 6000,
    };
    const conflicts = [makeSession({ id: 'session-2', title: 'Busy Session' })];

    const changedAttempt = resolveWebSessionSendConfirmation({
      conflicts,
      currentState: existingState,
      signature: changedSignature,
      now: 2000,
      ttlMs: 5000,
    });
    const expiredAttempt = resolveWebSessionSendConfirmation({
      conflicts,
      currentState: existingState,
      signature,
      now: 7000,
      ttlMs: 5000,
    });

    expect(changedAttempt.shouldProceed).toBe(false);
    expect(changedAttempt.nextState).toEqual({
      signature: changedSignature,
      expiresAt: 7000,
    });

    expect(expiredAttempt.shouldProceed).toBe(false);
    expect(expiredAttempt.nextState).toEqual({
      signature,
      expiresAt: 12000,
    });
  });

  it('bypasses confirmation when there are no conflicting sessions', () => {
    const result = resolveWebSessionSendConfirmation({
      conflicts: [],
      currentState: {
        signature: 'stale',
        expiresAt: 9999,
      },
      signature: 'current',
      now: 1000,
      ttlMs: 5000,
    });

    expect(result.shouldProceed).toBe(true);
    expect(result.nextState).toBeNull();
  });
});
