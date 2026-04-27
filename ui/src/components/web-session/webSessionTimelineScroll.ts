export interface WebSessionTimelineScrollMetrics {
  scrollTop: number;
  scrollHeight: number;
  clientHeight: number;
}

export interface WebSessionTimelineFollowState {
  autoFollowBottom: boolean;
  showJumpToBottom: boolean;
  lastScrollTop: number;
}

export const WEB_SESSION_TIMELINE_AT_BOTTOM_THRESHOLD_PX = 4;
const WEB_SESSION_TIMELINE_SCROLL_UP_EPSILON_PX = 1;

function normalizeScrollTop(value: number) {
  return Number.isFinite(value) ? Math.max(0, value) : 0;
}

export function getWebSessionTimelineBottomDistance(metrics: WebSessionTimelineScrollMetrics) {
  const scrollTop = normalizeScrollTop(metrics.scrollTop);
  const scrollHeight = Number.isFinite(metrics.scrollHeight)
    ? Math.max(0, metrics.scrollHeight)
    : 0;
  const clientHeight = Number.isFinite(metrics.clientHeight)
    ? Math.max(0, metrics.clientHeight)
    : 0;
  return Math.max(0, scrollHeight - (scrollTop + clientHeight));
}

export function isWebSessionTimelineAtBottom(metrics: WebSessionTimelineScrollMetrics) {
  return (
    getWebSessionTimelineBottomDistance(metrics) <= WEB_SESSION_TIMELINE_AT_BOTTOM_THRESHOLD_PX
  );
}

export function createWebSessionTimelineFollowState(
  metrics: WebSessionTimelineScrollMetrics,
  autoFollowBottom = true
): WebSessionTimelineFollowState {
  const atBottom = isWebSessionTimelineAtBottom(metrics);
  const follow = autoFollowBottom || atBottom;
  return {
    autoFollowBottom: follow,
    showJumpToBottom: !follow,
    lastScrollTop: normalizeScrollTop(metrics.scrollTop),
  };
}

export function resolveWebSessionTimelineFollowState(
  previous: WebSessionTimelineFollowState,
  metrics: WebSessionTimelineScrollMetrics
): WebSessionTimelineFollowState {
  const scrollTop = normalizeScrollTop(metrics.scrollTop);
  const atBottom = isWebSessionTimelineAtBottom(metrics);
  const movedUp = scrollTop < previous.lastScrollTop - WEB_SESSION_TIMELINE_SCROLL_UP_EPSILON_PX;

  if (atBottom) {
    return {
      autoFollowBottom: true,
      showJumpToBottom: false,
      lastScrollTop: scrollTop,
    };
  }

  if (movedUp) {
    return {
      autoFollowBottom: false,
      showJumpToBottom: true,
      lastScrollTop: scrollTop,
    };
  }

  return {
    autoFollowBottom: previous.autoFollowBottom,
    showJumpToBottom: !previous.autoFollowBottom,
    lastScrollTop: scrollTop,
  };
}
