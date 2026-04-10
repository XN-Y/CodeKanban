import type { WebSessionLiveState } from '@/stores/webSession';
import type { WebSessionSummary } from '@/types/models';

export type WebSessionSendGuardPhase = WebSessionLiveState['phase'];

export interface WebSessionSendGuardSession {
  id: string;
  title: string;
  workflowMode: WebSessionSummary['workflowMode'];
  livePhase: WebSessionSendGuardPhase;
}

export interface WebSessionSendConfirmationState {
  signature: string;
  expiresAt: number;
}

export interface ResolveWebSessionSendConfirmationInput {
  conflicts: WebSessionSendGuardSession[];
  currentState?: WebSessionSendConfirmationState | null;
  signature: string;
  now: number;
  ttlMs: number;
}

export interface ResolveWebSessionSendConfirmationResult {
  shouldProceed: boolean;
  nextState: WebSessionSendConfirmationState | null;
}

const ACTIVE_EXECUTION_PHASES = new Set<WebSessionSendGuardPhase>([
  'starting',
  'thinking',
  'tool',
  'retrying',
]);

function normalizeWebSessionSendOwnerId(ownerId: string) {
  return String(ownerId || '').trim();
}

function normalizeWebSessionSendText(text: string) {
  return String(text ?? '');
}

function normalizeWebSessionSendIds(ids: string[]) {
  return Array.from(new Set(ids.map(id => String(id || '').trim()).filter(Boolean))).sort();
}

export function isWebSessionActiveExecutionPhase(phase: WebSessionSendGuardPhase) {
  return ACTIVE_EXECUTION_PHASES.has(phase);
}

export function findWebSessionSendConflicts(input: {
  currentSessionId?: string;
  sessions: WebSessionSendGuardSession[];
}) {
  const currentSessionId = String(input.currentSessionId || '').trim();
  return input.sessions.filter(session => {
    if (!session.id || session.id === currentSessionId) {
      return false;
    }
    if (session.workflowMode === 'plan') {
      return false;
    }
    return isWebSessionActiveExecutionPhase(session.livePhase);
  });
}

export function buildWebSessionSendConfirmationSignature(input: {
  ownerId: string;
  text: string;
  attachmentIds: string[];
  conflictSessionIds: string[];
}) {
  return JSON.stringify({
    ownerId: normalizeWebSessionSendOwnerId(input.ownerId),
    text: normalizeWebSessionSendText(input.text),
    attachmentIds: normalizeWebSessionSendIds(input.attachmentIds),
    conflictSessionIds: normalizeWebSessionSendIds(input.conflictSessionIds),
  });
}

export function resolveWebSessionSendConfirmation(
  input: ResolveWebSessionSendConfirmationInput
): ResolveWebSessionSendConfirmationResult {
  if (input.conflicts.length === 0) {
    return {
      shouldProceed: true,
      nextState: null,
    };
  }

  const isConfirmed =
    input.currentState != null &&
    input.currentState.signature === input.signature &&
    input.currentState.expiresAt > input.now;

  if (isConfirmed) {
    return {
      shouldProceed: true,
      nextState: null,
    };
  }

  return {
    shouldProceed: false,
    nextState: {
      signature: input.signature,
      expiresAt: input.now + Math.max(0, Math.trunc(input.ttlMs)),
    },
  };
}
