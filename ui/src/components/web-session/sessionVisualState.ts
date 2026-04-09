import type { WebSessionLiveState } from '@/stores/webSession';
import type { WebSessionSummary } from '@/types/models';

type SessionPhase = WebSessionLiveState['phase'];
type SessionStatus = WebSessionSummary['status'];

export type WebSessionPillTone = 'working' | 'approval' | 'completion' | 'unknown';
export type WebSessionTabTone = 'approval' | 'completion' | 'default';
export type WebSessionSidebarTone =
  | 'working'
  | 'approval'
  | 'completion'
  | 'idle'
  | 'error'
  | 'default';

type SessionVisualInput = {
  phase: SessionPhase;
  hasUnread: boolean;
  status?: SessionStatus | '' | null;
};

const APPROVAL_PHASES = new Set<SessionPhase>([
  'waiting_approval',
  'waiting_input',
  'waiting_plan_approval',
]);

const WORKING_PHASES = new Set<SessionPhase>(['starting', 'thinking', 'tool', 'retrying']);

function isApprovalPhase(phase: SessionPhase) {
  return APPROVAL_PHASES.has(phase);
}

function isWorkingPhase(phase: SessionPhase) {
  return WORKING_PHASES.has(phase);
}

export function getWebSessionPillTone({
  phase,
  hasUnread,
}: SessionVisualInput): WebSessionPillTone {
  if (isApprovalPhase(phase)) {
    return 'approval';
  }
  if (isWorkingPhase(phase)) {
    return 'working';
  }
  if ((phase === 'done' || phase === 'idle') && hasUnread) {
    return 'completion';
  }
  return 'unknown';
}

export function getWebSessionTabTone({ phase, hasUnread }: SessionVisualInput): WebSessionTabTone {
  if (isApprovalPhase(phase)) {
    return 'approval';
  }
  if ((phase === 'done' || phase === 'idle') && hasUnread) {
    return 'completion';
  }
  return 'default';
}

export function getWebSessionSidebarTone({
  phase,
  hasUnread,
  status,
}: SessionVisualInput): WebSessionSidebarTone {
  if (isApprovalPhase(phase)) {
    return 'approval';
  }
  if (isWorkingPhase(phase)) {
    return 'working';
  }
  if ((phase === 'done' || phase === 'idle') && hasUnread) {
    return 'completion';
  }
  if (phase === 'done' || phase === 'idle') {
    return 'idle';
  }
  if (phase === 'error' || status === 'err') {
    return 'error';
  }
  return 'default';
}
