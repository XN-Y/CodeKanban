import type { WebSessionSummary } from '@/types/models';

type ComparableSessionFields = Pick<
  WebSessionSummary,
  'updatedAt' | 'lastSyncedAt' | 'syncState' | 'itemCount'
>;

export interface WebSessionSnapshotVersion {
  updatedAtMs: number;
  lastSyncedAtMs: number;
  syncStateRank: number;
  itemCount: number;
  historyTotal: number;
}

export interface WebSessionSnapshotVersionInput {
  session: ComparableSessionFields;
  historyTotal?: number | null;
}

function parseTimeMs(value?: string | null): number {
  const parsed = Date.parse(value ?? '');
  return Number.isFinite(parsed) ? parsed : 0;
}

function normalizeCount(value?: number | null): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.trunc(value));
}

function syncStateRank(syncState?: WebSessionSummary['syncState'] | null): number {
  switch (syncState) {
    case 'fresh':
      return 4;
    case 'stale':
      return 3;
    case 'error':
      return 2;
    case 'missing':
      return 1;
    case 'syncing':
    default:
      return 0;
  }
}

export function buildWebSessionSnapshotVersion(
  input: WebSessionSnapshotVersionInput
): WebSessionSnapshotVersion {
  return {
    updatedAtMs: parseTimeMs(input.session.updatedAt),
    lastSyncedAtMs: parseTimeMs(input.session.lastSyncedAt),
    syncStateRank: syncStateRank(input.session.syncState),
    itemCount: normalizeCount(input.session.itemCount),
    historyTotal: normalizeCount(input.historyTotal),
  };
}

export function compareWebSessionSnapshotVersion(
  left: WebSessionSnapshotVersion,
  right: WebSessionSnapshotVersion
): number {
  if (left.updatedAtMs !== right.updatedAtMs) {
    return left.updatedAtMs - right.updatedAtMs;
  }
  if (left.lastSyncedAtMs !== right.lastSyncedAtMs) {
    return left.lastSyncedAtMs - right.lastSyncedAtMs;
  }
  if (left.syncStateRank !== right.syncStateRank) {
    return left.syncStateRank - right.syncStateRank;
  }
  if (left.itemCount !== right.itemCount) {
    return left.itemCount - right.itemCount;
  }
  if (left.historyTotal !== right.historyTotal) {
    return left.historyTotal - right.historyTotal;
  }
  return 0;
}

export function selectLatestWebSessionSnapshotVersion(
  ...versions: Array<WebSessionSnapshotVersion | null | undefined>
): WebSessionSnapshotVersion | null {
  let latest: WebSessionSnapshotVersion | null = null;
  for (const version of versions) {
    if (!version) {
      continue;
    }
    if (!latest || compareWebSessionSnapshotVersion(version, latest) > 0) {
      latest = version;
    }
  }
  return latest;
}

export function shouldApplyIncomingWebSessionSnapshot(args: {
  appliedVersion?: WebSessionSnapshotVersion | null;
  currentSnapshot?: WebSessionSnapshotVersionInput | null;
  incomingSnapshot: WebSessionSnapshotVersionInput;
}): boolean {
  const currentVersion = selectLatestWebSessionSnapshotVersion(
    args.appliedVersion ?? null,
    args.currentSnapshot ? buildWebSessionSnapshotVersion(args.currentSnapshot) : null
  );
  if (!currentVersion) {
    return true;
  }
  const incomingVersion = buildWebSessionSnapshotVersion(args.incomingSnapshot);
  return compareWebSessionSnapshotVersion(incomingVersion, currentVersion) >= 0;
}
