import { useStorage } from '@vueuse/core';
import { effectScope } from 'vue';
import { describe, expect, it } from 'vitest';

import {
  chooseGitChangesScope,
  formatGitChangesBadgeDelta,
  formatGitChangesSummary,
  GIT_CHANGES_IGNORE_UNTRACKED_DEFAULT,
  GIT_CHANGES_IGNORE_UNTRACKED_STORAGE_KEY,
  orderGitChangesEntries,
  shouldShowGitChangesBadge,
  summarizeGitChangesEntries,
} from '@/components/changes/gitChangesSummary';
import type { FileManagerChangeEntry, FileManagerScope } from '@/types/fileManager';

function makeScope(
  input: Partial<FileManagerScope> & Pick<FileManagerScope, 'id'>
): FileManagerScope {
  return {
    id: input.id,
    kind: input.kind ?? 'project',
    label: input.label ?? input.id,
    rootPath: input.rootPath ?? `/tmp/${input.id}`,
    worktreeId: input.worktreeId,
  };
}

function makeChangeEntry(
  path: string,
  kind: FileManagerChangeEntry['status']['kind'],
  additions = 0,
  deletions = 0
): FileManagerChangeEntry {
  return {
    name: path.split('/').pop() ?? path,
    path,
    previewKind: 'text',
    hidden: false,
    exists: kind !== 'deleted',
    status: { kind },
    additions,
    deletions,
    statsAvailable: true,
  };
}

function createStorageMock(initial: Record<string, string> = {}) {
  const store = new Map<string, string>(Object.entries(initial));
  return {
    getItem(key: string) {
      return store.has(key) ? store.get(key)! : null;
    },
    setItem(key: string, value: string) {
      store.set(key, String(value));
    },
    removeItem(key: string) {
      store.delete(key);
    },
  };
}

function readStoredIgnoreUntrackedPreference(initial: Record<string, string> = {}) {
  const scope = effectScope();
  try {
    return scope.run(
      () =>
        useStorage<boolean>(
          GIT_CHANGES_IGNORE_UNTRACKED_STORAGE_KEY,
          GIT_CHANGES_IGNORE_UNTRACKED_DEFAULT,
          createStorageMock(initial) as Storage
        ).value
    );
  } finally {
    scope.stop();
  }
}

describe('gitChangesSummary', () => {
  it('defaults the ignore-untracked preference to true when nothing is stored', () => {
    expect(GIT_CHANGES_IGNORE_UNTRACKED_DEFAULT).toBe(true);
    expect(readStoredIgnoreUntrackedPreference()).toBe(true);
  });

  it('keeps an existing stored false ignore-untracked preference', () => {
    expect(
      readStoredIgnoreUntrackedPreference({
        [GIT_CHANGES_IGNORE_UNTRACKED_STORAGE_KEY]: 'false',
      })
    ).toBe(false);
  });

  it('prefers requested, active, then worktree scope', () => {
    const scopes = [
      makeScope({ id: 'project-1' }),
      makeScope({ id: 'worktree-1', kind: 'worktree', worktreeId: 'wt-1' }),
    ];

    expect(
      chooseGitChangesScope(scopes, {
        requestedScopeId: 'worktree-1',
      })?.id
    ).toBe('worktree-1');

    expect(
      chooseGitChangesScope(scopes, {
        activeScopeId: 'project-1',
      })?.id
    ).toBe('project-1');

    expect(
      chooseGitChangesScope(scopes, {
        preferredWorktreeId: 'wt-1',
      })?.id
    ).toBe('worktree-1');
  });

  it('keeps untracked entries at the bottom unless hidden', () => {
    const ordered = orderGitChangesEntries(
      [
        makeChangeEntry('z-last.ts', 'modified', 1, 0),
        makeChangeEntry('a-new.ts', 'untracked', 2, 0),
        makeChangeEntry('b-core.ts', 'added', 5, 0),
      ],
      false
    );

    expect(ordered.map(entry => entry.path)).toEqual(['b-core.ts', 'z-last.ts', 'a-new.ts']);

    const hidden = orderGitChangesEntries(ordered, true);
    expect(hidden.map(entry => entry.path)).toEqual(['b-core.ts', 'z-last.ts']);
  });

  it('summarizes visible changes and formats the badge text', () => {
    const summary = summarizeGitChangesEntries(
      [
        makeChangeEntry('a.ts', 'modified', 3, 1),
        makeChangeEntry('b.ts', 'deleted', 0, 4),
        makeChangeEntry('c.ts', 'untracked', 7, 0),
      ],
      true
    );

    expect(summary).toEqual({
      count: 2,
      additions: 3,
      deletions: 5,
    });
    expect(formatGitChangesSummary(summary)).toBe('2,+3,-5');
  });

  it('returns an empty badge text when there are no visible changes', () => {
    expect(
      formatGitChangesSummary({
        count: 0,
        additions: 0,
        deletions: 0,
      })
    ).toBe('');
  });

  it('formats pending badge deltas and keeps loading badges visible', () => {
    expect(formatGitChangesBadgeDelta('+', null)).toBe('+?');
    expect(formatGitChangesBadgeDelta('-', 4)).toBe('-4');

    expect(
      shouldShowGitChangesBadge({
        count: 0,
        additions: 0,
        deletions: 0,
        pending: true,
      })
    ).toBe(true);
    expect(
      shouldShowGitChangesBadge({
        count: 0,
        additions: 0,
        deletions: 0,
        pending: false,
      })
    ).toBe(false);
  });
});
