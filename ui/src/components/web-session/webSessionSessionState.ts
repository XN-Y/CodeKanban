import type { WebSessionLiveState } from '@/stores/webSession';
import type { WebSessionSummary } from '@/types/models';

export type WebSessionDisplayAssistantState =
  | 'working'
  | 'waiting_approval'
  | 'waiting_input'
  | 'unknown';

export type WebSessionDisplayPillState = WebSessionDisplayAssistantState | 'completion';

export type WebSessionDisplayStatusKey =
  | 'terminal.aiStatusWorking'
  | 'terminal.aiStatusWaitingApproval'
  | 'terminal.aiStatusWaitingInput';

export interface WebSessionDisplayStateInput {
  isDraft: boolean;
  hasUnread: boolean;
  status: WebSessionSummary['status'];
  syncState: WebSessionSummary['syncState'];
  livePhase?: WebSessionLiveState['phase'] | null;
  assistantState?: WebSessionSummary['assistantState'];
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
    case 'waiting_plan_approval':
      return 'waiting_approval';
    case 'waiting_input':
    case 'done':
    case 'idle':
      return 'waiting_input';
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
    case 'waiting_plan_approval':
      return 'waiting_approval';
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
    case 'err':
    case 'aborting':
      return 'waiting_input';
    default:
      return 'unknown';
  }
}

export function resolveWebSessionDisplayState(
  input: WebSessionDisplayStateInput
): WebSessionDisplayState {
  const assistantStateClass = input.isDraft
    ? 'waiting_input'
    : (mapPhaseToAssistantState(input.livePhase) ??
      mapAssistantStateToDisplayState(input.assistantState) ??
      mapStatusToAssistantState(input.status));

  const hasUnviewedApproval = input.hasUnread && assistantStateClass === 'waiting_approval';
  const hasUnviewedCompletion =
    input.hasUnread &&
    !hasUnviewedApproval &&
    assistantStateClass === 'waiting_input' &&
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
      statusLabelKey = 'terminal.aiStatusWaitingApproval';
      statusEmoji = '✋';
      break;
    case 'waiting_input':
      statusLabelKey = 'terminal.aiStatusWaitingInput';
      statusEmoji = '✓';
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
