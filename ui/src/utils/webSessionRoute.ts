import type { LocationQuery, LocationQueryRaw } from 'vue-router';

export const WEB_SESSION_ID_QUERY_KEY = 'webSessionId';

type WebSessionDeepLinkSummary = {
  id: string;
  projectId: string;
  archivedAt?: string | null;
};

export type WebSessionDeepLinkTarget =
  | { action: 'none' }
  | { action: 'activate-loaded'; sessionId: string }
  | { action: 'load-snapshot'; sessionId: string }
  | { action: 'activate-real'; sessionId: string }
  | { action: 'open-archived-preview'; sessionId: string }
  | { action: 'clear-invalid' };

export function normalizeWebSessionRouteSessionId(value: unknown): string {
  if (Array.isArray(value)) {
    for (const item of value) {
      const normalized = normalizeWebSessionRouteSessionId(item);
      if (normalized) {
        return normalized;
      }
    }
    return '';
  }
  return typeof value === 'string' ? value.trim() : '';
}

export function getWebSessionRouteSessionId(query?: LocationQuery | null): string {
  if (!query) {
    return '';
  }
  return normalizeWebSessionRouteSessionId(query[WEB_SESSION_ID_QUERY_KEY]);
}

export function buildWebSessionRouteQuery(
  query: LocationQuery = {},
  sessionId?: string
): LocationQueryRaw {
  const nextQuery: LocationQueryRaw = { ...query };
  const normalizedSessionId = normalizeWebSessionRouteSessionId(sessionId);
  if (normalizedSessionId) {
    nextQuery[WEB_SESSION_ID_QUERY_KEY] = normalizedSessionId;
  } else {
    delete nextQuery[WEB_SESSION_ID_QUERY_KEY];
  }
  return nextQuery;
}

export function isWebSessionRouteQuerySynced(
  query?: LocationQuery | null,
  sessionId?: string
): boolean {
  return getWebSessionRouteSessionId(query) === normalizeWebSessionRouteSessionId(sessionId);
}

function normalizeComparableQuery(
  query?: LocationQuery | null,
  ignoredKeys: string[] = []
): Record<string, string[]> {
  const ignored = new Set(ignoredKeys);
  const result: Record<string, string[]> = {};
  if (!query) {
    return result;
  }

  Object.entries(query).forEach(([key, value]) => {
    if (ignored.has(key)) {
      return;
    }

    const values = (Array.isArray(value) ? value : [value])
      .flatMap(item => (item == null ? [] : [String(item).trim()]))
      .filter(Boolean)
      .sort((left, right) => left.localeCompare(right));

    if (values.length > 0) {
      result[key] = values;
    }
  });

  return result;
}

function normalizeComparableParams(
  params?: Record<string, unknown> | null
): Record<string, string[]> {
  const result: Record<string, string[]> = {};
  if (!params) {
    return result;
  }

  Object.entries(params).forEach(([key, value]) => {
    const values = (Array.isArray(value) ? value : [value])
      .flatMap(item => (item == null ? [] : [String(item).trim()]))
      .filter(Boolean)
      .sort((left, right) => left.localeCompare(right));

    if (values.length > 0) {
      result[key] = values;
    }
  });

  return result;
}

function compareComparableRecords(
  left: Record<string, string[]>,
  right: Record<string, string[]>
): boolean {
  const leftKeys = Object.keys(left).sort((a, b) => a.localeCompare(b));
  const rightKeys = Object.keys(right).sort((a, b) => a.localeCompare(b));
  if (leftKeys.length !== rightKeys.length) {
    return false;
  }

  return leftKeys.every((key, index) => {
    if (key !== rightKeys[index]) {
      return false;
    }
    const leftValues = left[key] ?? [];
    const rightValues = right[key] ?? [];
    if (leftValues.length !== rightValues.length) {
      return false;
    }
    return leftValues.every((value, valueIndex) => value === rightValues[valueIndex]);
  });
}

export function isWebSessionOnlyRouteChange(
  to: {
    name?: string | symbol | null;
    path?: string;
    params?: Record<string, unknown>;
    query?: LocationQuery | null;
  },
  from: {
    name?: string | symbol | null;
    path?: string;
    params?: Record<string, unknown>;
    query?: LocationQuery | null;
  }
): boolean {
  if ((to.name ?? null) !== (from.name ?? null)) {
    return false;
  }
  if ((to.path ?? '') !== (from.path ?? '')) {
    return false;
  }
  if (
    !compareComparableRecords(
      normalizeComparableParams(to.params),
      normalizeComparableParams(from.params)
    )
  ) {
    return false;
  }

  return compareComparableRecords(
    normalizeComparableQuery(to.query, [WEB_SESSION_ID_QUERY_KEY]),
    normalizeComparableQuery(from.query, [WEB_SESSION_ID_QUERY_KEY])
  );
}

export function resolveWebSessionDeepLinkTarget(options: {
  currentProjectId: string;
  requestedSessionId: string;
  loadedSessions?: Array<Pick<WebSessionDeepLinkSummary, 'id'>>;
  snapshotSession?: WebSessionDeepLinkSummary | null;
}): WebSessionDeepLinkTarget {
  const requestedSessionId = normalizeWebSessionRouteSessionId(options.requestedSessionId);
  if (!requestedSessionId) {
    return { action: 'none' };
  }

  const loadedSessions = options.loadedSessions ?? [];
  if (
    loadedSessions.some(
      session => normalizeWebSessionRouteSessionId(session.id) === requestedSessionId
    )
  ) {
    return {
      action: 'activate-loaded',
      sessionId: requestedSessionId,
    };
  }

  if (options.snapshotSession === undefined) {
    return {
      action: 'load-snapshot',
      sessionId: requestedSessionId,
    };
  }

  const snapshotSession = options.snapshotSession;
  if (!snapshotSession) {
    return { action: 'clear-invalid' };
  }

  const currentProjectId = normalizeWebSessionRouteSessionId(options.currentProjectId);
  const snapshotSessionId = normalizeWebSessionRouteSessionId(snapshotSession.id);
  const snapshotProjectId = normalizeWebSessionRouteSessionId(snapshotSession.projectId);

  if (
    !currentProjectId ||
    snapshotSessionId !== requestedSessionId ||
    snapshotProjectId !== currentProjectId
  ) {
    return { action: 'clear-invalid' };
  }

  if (snapshotSession.archivedAt) {
    return {
      action: 'open-archived-preview',
      sessionId: requestedSessionId,
    };
  }

  return {
    action: 'activate-real',
    sessionId: requestedSessionId,
  };
}
