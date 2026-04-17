import { describe, expect, it } from 'vitest';

import {
  resolveInitialChangePreviewMode,
  resolveDiffUnavailableReasonKey,
  resolveGitStatusLetter,
  resolveGitStatusTagType,
  resolveInitialFilePreviewMode,
  resolvePreviewGitStatus,
  shouldRequestFileDiff,
} from '@/components/files/fileManagerDiff';

describe('fileManagerDiff helpers', () => {
  it('defaults changed files to diff mode', () => {
    expect(
      resolveInitialFilePreviewMode({
        kind: 'file',
        gitStatus: { kind: 'modified' },
      })
    ).toBe('diff');
    expect(
      resolveInitialFilePreviewMode({
        kind: 'file',
      })
    ).toBe('file');
  });

  it('requests diff only for files with git status', () => {
    expect(
      shouldRequestFileDiff({
        kind: 'file',
        gitStatus: { kind: 'renamed', previousPath: 'old.txt' },
      })
    ).toBe(true);
    expect(
      shouldRequestFileDiff({
        kind: 'directory',
        gitStatus: { kind: 'dirty' },
      })
    ).toBe(false);
  });

  it('maps git status kinds to tag tones', () => {
    expect(resolveGitStatusTagType('added')).toBe('success');
    expect(resolveGitStatusTagType('deleted')).toBe('error');
    expect(resolveGitStatusTagType('conflicted')).toBe('error');
    expect(resolveGitStatusTagType('dirty')).toBe('warning');
    expect(resolveGitStatusTagType('untracked')).toBe('default');
  });

  it('maps git status kinds to VS Code style letters', () => {
    expect(resolveGitStatusLetter('added')).toBe('A');
    expect(resolveGitStatusLetter('modified')).toBe('M');
    expect(resolveGitStatusLetter('deleted')).toBe('D');
    expect(resolveGitStatusLetter('untracked')).toBe('U');
  });

  it('chooses a sensible default mode for change entries', () => {
    expect(
      resolveInitialChangePreviewMode({
        exists: true,
        status: { kind: 'modified' },
      })
    ).toBe('diff');
    expect(
      resolveInitialChangePreviewMode({
        exists: true,
        status: { kind: 'untracked' },
      })
    ).toBe('file');
    expect(
      resolveInitialChangePreviewMode({
        exists: false,
        status: { kind: 'deleted' },
      })
    ).toBe('diff');
  });

  it('maps diff unavailable reasons to i18n keys', () => {
    expect(resolveDiffUnavailableReasonKey('binary')).toBe('fileManager.diffUnavailable.binary');
    expect(resolveDiffUnavailableReasonKey('not_git_repository')).toBe(
      'fileManager.diffUnavailable.notGitRepository'
    );
    expect(resolveDiffUnavailableReasonKey('something-else')).toBe(
      'fileManager.diffUnavailable.unavailable'
    );
  });

  it('prefers preview status and falls back to diff status', () => {
    expect(
      resolvePreviewGitStatus(
        { kind: 'modified' },
        {
          path: 'README.md',
          available: true,
          comparedTo: 'HEAD',
          status: { kind: 'renamed', previousPath: 'README.old.md' },
          diffText: 'diff',
        }
      )
    ).toEqual({ kind: 'modified' });

    expect(
      resolvePreviewGitStatus(undefined, {
        path: 'README.md',
        available: false,
        comparedTo: 'HEAD',
        status: { kind: 'conflicted' },
      })
    ).toEqual({ kind: 'conflicted' });
  });
});
