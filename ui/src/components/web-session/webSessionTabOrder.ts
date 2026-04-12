export type OrderedTabSessionLike = {
  id: string;
};

export function clampTabAnchorIndex(anchorIndex: number, baseLength: number) {
  if (!Number.isFinite(anchorIndex)) {
    return Math.max(0, baseLength);
  }
  const normalizedBaseLength = Math.max(0, Math.trunc(baseLength));
  return Math.min(normalizedBaseLength, Math.max(0, Math.trunc(anchorIndex)));
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
