import type { WebSessionSummary } from '@/types/models';
import type { WebSessionLiveState } from '@/stores/webSession';

export interface AiStatusSummary {
  working: number;
  blocking: number;
  unreadCompleted: number;
}

type StatusBucket = keyof AiStatusSummary;

export const EMPTY_AI_STATUS_SUMMARY: Readonly<AiStatusSummary> = Object.freeze({
  working: 0,
  blocking: 0,
  unreadCompleted: 0,
});

type WebSessionSummaryLike = Pick<WebSessionSummary, 'id' | 'hasUnread'>;
type WebSessionLiveStateLike = Pick<WebSessionLiveState, 'phase' | 'running'>;

export function createAiStatusSummary(): AiStatusSummary {
  return {
    working: 0,
    blocking: 0,
    unreadCompleted: 0,
  };
}

export function hasAiStatusSummary(summary: AiStatusSummary) {
  return summary.working + summary.blocking + summary.unreadCompleted > 0;
}

export function getAiStatusSummaryTotal(summary: AiStatusSummary) {
  return summary.working + summary.blocking + summary.unreadCompleted;
}

export function formatAiStatusTriplet(summary: AiStatusSummary) {
  return `${summary.working}/${summary.blocking}/${summary.unreadCompleted}`;
}

export function formatAiStatusTripletWithTotal(summary: AiStatusSummary, totalCount: number) {
  const normalizedTotal = Math.max(0, Number(totalCount) || 0);
  if (!hasAiStatusSummary(summary)) {
    return String(normalizedTotal);
  }
  return `${formatAiStatusTriplet(summary)} · ${normalizedTotal}`;
}

export function formatAiStatusTitle(summary: AiStatusSummary, appName: string) {
  if (!hasAiStatusSummary(summary)) {
    return appName;
  }
  return `[${formatAiStatusTriplet(summary)}] ${appName}`;
}

export function getWebSessionStatusBucket(
  session: Pick<WebSessionSummaryLike, 'hasUnread'>,
  liveState: WebSessionLiveStateLike
): StatusBucket | null {
  if (
    liveState.phase === 'waiting_approval' ||
    liveState.phase === 'waiting_plan_approval' ||
    liveState.phase === 'waiting_input'
  ) {
    return 'blocking';
  }
  if (liveState.running) {
    return 'working';
  }
  if (session.hasUnread && liveState.phase === 'done') {
    return 'unreadCompleted';
  }
  return null;
}

export function summarizeWebSessions(
  sessions: readonly WebSessionSummaryLike[],
  getLiveState: (sessionId: string) => WebSessionLiveStateLike
): AiStatusSummary {
  const summary = createAiStatusSummary();
  sessions.forEach(session => {
    const bucket = getWebSessionStatusBucket(session, getLiveState(session.id));
    if (bucket) {
      summary[bucket] += 1;
    }
  });
  return summary;
}
