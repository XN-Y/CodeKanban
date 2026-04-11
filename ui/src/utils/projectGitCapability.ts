import type { Project, Worktree } from '@/types/models';

export function projectSupportsGit(
  project: Pick<Project, 'path' | 'remoteUrl'> | null | undefined,
  worktrees: Pick<Worktree, 'isMain' | 'path' | 'headCommit'>[] = []
): boolean {
  if (!project) {
    return false;
  }

  if (typeof project.remoteUrl === 'string' && project.remoteUrl.trim()) {
    return true;
  }

  const mainWorktree = worktrees.find(worktree => worktree.isMain);
  if (!mainWorktree) {
    return false;
  }

  const normalizedProjectPath = project.path.replace(/[\\/]+$/, '');
  const normalizedMainPath = mainWorktree.path.replace(/[\\/]+$/, '');
  const hasCommit =
    typeof mainWorktree.headCommit === 'string' && mainWorktree.headCommit.trim() !== '';

  // Non-git projects currently get a single virtual main worktree that mirrors the
  // project directory but has no commit metadata. Real git projects will either have
  // commit metadata or additional worktree metadata beyond this virtual placeholder shape.
  if (worktrees.length === 1 && normalizedProjectPath === normalizedMainPath && !hasCommit) {
    return false;
  }

  return true;
}
