import { describe, expect, it } from 'vitest';

import {
  chooseGitChangesScope,
  formatGitChangesSummary,
  orderGitChangesEntries,
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
  };
}

describe('gitChangesSummary', () => {
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
});
