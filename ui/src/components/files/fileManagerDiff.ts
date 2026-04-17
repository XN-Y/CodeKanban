import type {
  FileManagerChangeEntry,
  FileManagerDiffResult,
  FileManagerEntry,
  FileManagerGitStatus,
  FileManagerGitStatusKind,
} from '@/types/fileManager';

export type FilePreviewMode = 'file' | 'diff';

export function shouldRequestFileDiff(
  entry: Pick<FileManagerEntry, 'kind' | 'gitStatus'> | null | undefined
) {
  return entry?.kind === 'file' && Boolean(entry.gitStatus);
}

export function resolveInitialFilePreviewMode(
  entry: Pick<FileManagerEntry, 'kind' | 'gitStatus'> | null | undefined
): FilePreviewMode {
  return shouldRequestFileDiff(entry) ? 'diff' : 'file';
}

export function resolveGitStatusTagType(kind: FileManagerGitStatusKind | undefined) {
  switch (kind) {
    case 'added':
      return 'success' as const;
    case 'deleted':
      return 'error' as const;
    case 'renamed':
      return 'info' as const;
    case 'conflicted':
      return 'error' as const;
    case 'modified':
    case 'dirty':
      return 'warning' as const;
    default:
      return 'default' as const;
  }
}

export function resolveGitStatusLetter(kind: FileManagerGitStatusKind | undefined) {
  switch (kind) {
    case 'added':
      return 'A';
    case 'deleted':
      return 'D';
    case 'renamed':
      return 'R';
    case 'untracked':
      return 'U';
    case 'conflicted':
      return 'C';
    case 'dirty':
    case 'modified':
    default:
      return 'M';
  }
}

export function resolveInitialChangePreviewMode(
  entry: Pick<FileManagerChangeEntry, 'exists' | 'status'> | null | undefined
): FilePreviewMode {
  if (!entry) {
    return 'file';
  }
  switch (entry.status.kind) {
    case 'untracked':
    case 'conflicted':
      return entry.exists ? 'file' : 'diff';
    default:
      return 'diff';
  }
}

export function resolveDiffUnavailableReasonKey(reason: string | undefined) {
  switch (reason) {
    case 'untracked':
      return 'fileManager.diffUnavailable.untracked';
    case 'binary':
      return 'fileManager.diffUnavailable.binary';
    case 'conflicted':
      return 'fileManager.diffUnavailable.conflicted';
    case 'clean':
      return 'fileManager.diffUnavailable.clean';
    case 'not_git_repository':
      return 'fileManager.diffUnavailable.notGitRepository';
    default:
      return 'fileManager.diffUnavailable.unavailable';
  }
}

export function resolvePreviewGitStatus(
  previewStatus: FileManagerGitStatus | undefined,
  diffResult: FileManagerDiffResult | null
) {
  return previewStatus ?? diffResult?.status ?? null;
}
