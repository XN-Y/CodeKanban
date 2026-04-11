import { describe, expect, it } from 'vitest';

import { projectSupportsGit } from '@/utils/projectGitCapability';

describe('projectGitCapability', () => {
  it('treats projects with a remote or committed main worktree as git repositories', () => {
    expect(projectSupportsGit({ path: '/repo', remoteUrl: 'https://example.com/repo.git' })).toBe(
      true
    );
    expect(
      projectSupportsGit({ path: '/repo', remoteUrl: null }, [
        { isMain: true, path: '/repo', headCommit: 'abc123' },
      ])
    ).toBe(true);
  });

  it('treats a virtual main worktree placeholder as non-git', () => {
    expect(
      projectSupportsGit({ path: '/plain', remoteUrl: null }, [
        { isMain: true, path: '/plain', headCommit: null },
      ])
    ).toBe(false);
  });

  it('treats missing project state as non-git', () => {
    expect(projectSupportsGit(null)).toBe(false);
    expect(projectSupportsGit(undefined)).toBe(false);
  });
});
