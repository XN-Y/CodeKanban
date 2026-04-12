import type { WebSessionLiveState } from '@/stores/webSession';
import type { WebSessionSummary } from '@/types/models';

export type WebSessionDisplayAssistantState =
  | 'working'
  | 'waiting_approval'
  | 'waiting_plan_approval'
  | 'waiting_input'
  | 'idle'
  | 'unknown';

export type WebSessionDisplayPillState = WebSessionDisplayAssistantState | 'completion';

export type WebSessionDisplayStatusKey =
  | 'terminal.aiStatusWorking'
  | 'terminal.aiStatusWaitingApproval'
  | 'terminal.aiStatusWaitingInput'
  | 'terminal.aiIdle'
  | 'terminal.aiStatusDone';

export interface WebSessionDisplayAssistantStateInput {
  isDraft: boolean;
  status: WebSessionSummary['status'];
  livePhase?: WebSessionLiveState['phase'] | null;
  assistantState?: WebSessionSummary['assistantState'];
}

export interface WebSessionDisplayStateInput extends WebSessionDisplayAssistantStateInput {
  hasUnread: boolean;
  syncState: WebSessionSummary['syncState'];
}

export interface WebSessionDisplayState {
  assistantStateClass: WebSessionDisplayAssistantState;
  statusLabelKey: WebSessionDisplayStatusKey | null;
  pillStateClass: WebSessionDisplayPillState;
  statusEmoji: string;
  hasUnviewedApproval: boolean;
  hasUnviewedCompletion: boolean;
  showStatusDot: boolean;
  statusDotClass: WebSessionSummary['status'] | null;
}

function mapPhaseToAssistantState(
  phase?: WebSessionLiveState['phase'] | null
): WebSessionDisplayAssistantState | null {
  switch (phase) {
    case 'starting':
    case 'thinking':
    case 'tool':
    case 'retrying':
      return 'working';
    case 'waiting_approval':
      return 'waiting_approval';
    case 'waiting_plan_approval':
      return 'waiting_plan_approval';
    case 'waiting_input':
      return 'waiting_input';
    case 'done':
    case 'idle':
      return 'idle';
    case 'error':
      return 'unknown';
    default:
      return null;
  }
}

function mapAssistantStateToDisplayState(
  assistantState?: WebSessionSummary['assistantState']
): WebSessionDisplayAssistantState | null {
  switch (assistantState) {
    case 'working':
      return 'working';
    case 'waiting_approval':
      return 'waiting_approval';
    case 'waiting_plan_approval':
      return 'waiting_plan_approval';
    case 'waiting_input':
      return 'waiting_input';
    default:
      return null;
  }
}

function mapStatusToAssistantState(
  status: WebSessionSummary['status']
): WebSessionDisplayAssistantState {
  switch (status) {
    case 'running':
      return 'working';
    case 'waiting_approval':
      return 'waiting_approval';
    case 'idle':
    case 'done':
      return 'idle';
    case 'err':
    case 'aborting':
      return 'unknown';
    default:
      return 'unknown';
  }
}

export function resolveWebSessionDisplayAssistantState(
  input: WebSessionDisplayAssistantStateInput
): WebSessionDisplayAssistantState {
  return input.isDraft
    ? 'waiting_input'
    : (mapPhaseToAssistantState(input.livePhase) ??
        mapAssistantStateToDisplayState(input.assistantState) ??
        mapStatusToAssistantState(input.status));
}

function sortableTimestamp(value?: string | null) {
  const parsed = Date.parse(typeof value === 'string' ? value : '');
  return Number.isFinite(parsed) ? parsed : 0;
}

export function resolveWebSessionSidebarSortTimestamp(
  session: Pick<
    WebSessionSummary,
    'statusUpdatedAt' | 'assistantStateUpdatedAt' | 'updatedAt' | 'createdAt'
  >
) {
  return sortableTimestamp(
    session.statusUpdatedAt ||
      session.assistantStateUpdatedAt ||
      session.updatedAt ||
      session.createdAt
  );
}

export function resolveWebSessionDisplayState(
  input: WebSessionDisplayStateInput
): WebSessionDisplayState {
  const assistantStateClass = resolveWebSessionDisplayAssistantState(input);

  const hasUnviewedApproval =
    input.hasUnread &&
    (assistantStateClass === 'waiting_approval' || assistantStateClass === 'waiting_plan_approval');
  const hasUnviewedCompletion =
    input.hasUnread &&
    !hasUnviewedApproval &&
    assistantStateClass === 'idle' &&
    input.status !== 'err';
  const showStatusDot = !input.isDraft && input.status === 'err';

  let statusLabelKey: WebSessionDisplayStatusKey | null = null;
  let statusEmoji = '';

  switch (assistantStateClass) {
    case 'working':
      statusLabelKey = 'terminal.aiStatusWorking';
      statusEmoji = '🤔';
      break;
    case 'waiting_approval':
    case 'waiting_plan_approval':
      statusLabelKey = 'terminal.aiStatusWaitingApproval';
      statusEmoji = '✋';
      break;
    case 'waiting_input':
      statusLabelKey = 'terminal.aiStatusWaitingInput';
      statusEmoji = '✓';
      break;
    case 'idle':
      statusLabelKey = hasUnviewedCompletion ? 'terminal.aiStatusDone' : 'terminal.aiIdle';
      statusEmoji = hasUnviewedCompletion ? '✓' : '';
      break;
    default:
      break;
  }

  return {
    assistantStateClass,
    statusLabelKey,
    pillStateClass: hasUnviewedCompletion ? 'completion' : assistantStateClass,
    statusEmoji,
    hasUnviewedApproval,
    hasUnviewedCompletion,
    showStatusDot,
    statusDotClass: showStatusDot ? input.status : null,
  };
}
