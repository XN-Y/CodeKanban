export type OrderedTabSessionLike = {
  id: string;
};

export type MobileCurrentSessionLike = OrderedTabSessionLike & {
  orderIndex: number;
  isDraft?: boolean;
};

export function clampTabAnchorIndex(anchorIndex: number, baseLength: number) {
  if (!Number.isFinite(anchorIndex)) {
    return Math.max(0, baseLength);
  }
  const normalizedBaseLength = Math.max(0, Math.trunc(baseLength));
  return Math.min(normalizedBaseLength, Math.max(0, Math.trunc(anchorIndex)));
}

function normalizeSessionId(sessionId = '') {
  return String(sessionId || '').trim();
}

export function resolveUnderlyingTabSessionId(options: {
  activeDraftSessionId?: string;
  activeRealSessionId?: string;
}) {
  return (
    normalizeSessionId(options.activeDraftSessionId) ||
    normalizeSessionId(options.activeRealSessionId)
  );
}

export function resolveActiveTabSessionId(options: {
  activeArchivedPreviewId?: string;
  activeDraftSessionId?: string;
  activeRealSessionId?: string;
}) {
  if (normalizeSessionId(options.activeArchivedPreviewId)) {
    return '';
  }
  return resolveUnderlyingTabSessionId(options);
}

export function resolveTabAnchorInsertIndex<T extends OrderedTabSessionLike>(
  orderedSessions: T[],
  anchorId = ''
) {
  const normalizedAnchorId = String(anchorId || '').trim();
  if (!normalizedAnchorId) {
    return orderedSessions.length;
  }
  const anchorIndex = orderedSessions.findIndex(session => session.id === normalizedAnchorId);
  return anchorIndex >= 0 ? anchorIndex + 1 : orderedSessions.length;
}

export function buildOrderedTabSessions<T extends OrderedTabSessionLike>(
  orderedIds: string[],
  baseSessions: T[],
  fixedSession?: T | null,
  fixedAnchorIndex = baseSessions.length
) {
  const sessionById = new Map<string, T>();
  baseSessions.forEach(session => {
    sessionById.set(session.id, session);
  });

  const ordered: T[] = [];
  const seen = new Set<string>();

  orderedIds.forEach(sessionId => {
    const session = sessionById.get(sessionId);
    if (!session || seen.has(session.id)) {
      return;
    }
    ordered.push(session);
    seen.add(session.id);
  });

  baseSessions.forEach(session => {
    if (seen.has(session.id)) {
      return;
    }
    ordered.push(session);
    seen.add(session.id);
  });

  if (!fixedSession) {
    return ordered;
  }

  const anchored = [...ordered];
  anchored.splice(clampTabAnchorIndex(fixedAnchorIndex, anchored.length), 0, fixedSession);
  return anchored;
}

function sortableNumber(value: number) {
  return Number.isFinite(value) ? value : 0;
}

export function sortMobileCurrentSessions<T extends MobileCurrentSessionLike>(
  sessions: T[],
  resolveSortTimestamp: (session: T) => number
) {
  const drafts: T[] = [];
  const realSessions: T[] = [];

  sessions.forEach(session => {
    if (session.isDraft) {
      drafts.push(session);
      return;
    }
    realSessions.push(session);
  });

  const sortedRealSessions = [...realSessions].sort((left, right) => {
    const rightTimestamp = sortableNumber(resolveSortTimestamp(right));
    const leftTimestamp = sortableNumber(resolveSortTimestamp(left));
    if (rightTimestamp !== leftTimestamp) {
      return rightTimestamp - leftTimestamp;
    }
    if (left.orderIndex !== right.orderIndex) {
      return left.orderIndex - right.orderIndex;
    }
    return left.id.localeCompare(right.id);
  });

  return [...drafts, ...sortedRealSessions];
}
