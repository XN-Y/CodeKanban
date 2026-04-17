import type { FileManagerChangeEntry, FileManagerScope } from '@/types/fileManager';

export const GIT_CHANGES_IGNORE_UNTRACKED_STORAGE_KEY = 'git-changes-ignore-untracked';

export interface GitChangesSummary {
  count: number;
  additions: number;
  deletions: number;
}

export function chooseGitChangesScope(
  scopes: FileManagerScope[],
  options?: {
    activeScopeId?: string;
    preferredWorktreeId?: string | null;
    requestedScopeId?: string;
  }
) {
  if (scopes.length === 0) {
    return null;
  }

  if (options?.requestedScopeId) {
    const explicit = scopes.find(scope => scope.id === options.requestedScopeId);
    if (explicit) {
      return explicit;
    }
  }

  if (options?.activeScopeId) {
    const active = scopes.find(scope => scope.id === options.activeScopeId);
    if (active) {
      return active;
    }
  }

  if (options?.preferredWorktreeId) {
    const preferred = scopes.find(scope => scope.worktreeId === options.preferredWorktreeId);
    if (preferred) {
      return preferred;
    }
  }

  return scopes[0];
}

export function orderGitChangesEntries(entries: FileManagerChangeEntry[], ignoreUntracked = false) {
  const filtered = ignoreUntracked
    ? entries.filter(entry => entry.status.kind !== 'untracked')
    : entries;
  return [...filtered].sort((left, right) => {
    const leftUntracked = left.status.kind === 'untracked' ? 1 : 0;
    const rightUntracked = right.status.kind === 'untracked' ? 1 : 0;
    if (leftUntracked !== rightUntracked) {
      return leftUntracked - rightUntracked;
    }
    return left.path.localeCompare(right.path, undefined, {
      sensitivity: 'base',
    });
  });
}

export function summarizeGitChangesEntries(
  entries: FileManagerChangeEntry[],
  ignoreUntracked = false
): GitChangesSummary {
  const orderedEntries = orderGitChangesEntries(entries, ignoreUntracked);
  return orderedEntries.reduce<GitChangesSummary>(
    (summary, entry) => ({
      count: summary.count + 1,
      additions: summary.additions + Math.max(0, Math.trunc(entry.additions ?? 0)),
      deletions: summary.deletions + Math.max(0, Math.trunc(entry.deletions ?? 0)),
    }),
    {
      count: 0,
      additions: 0,
      deletions: 0,
    }
  );
}

export function formatGitChangesSummary(summary: GitChangesSummary) {
  return `${summary.count},+${summary.additions},-${summary.deletions}`;
}
