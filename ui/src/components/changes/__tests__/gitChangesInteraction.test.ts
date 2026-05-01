import { describe, expect, it } from 'vitest';

import { createGitChangesLoadController } from '@/components/changes/gitChangesLoadController';
import { buildGitChangesRequestOptions } from '@/components/changes/gitChangesSupport';

function queueIgnoreUntrackedReload(
  controller: ReturnType<typeof createGitChangesLoadController>,
  scopeId: string,
  ignoreUntracked: boolean
) {
  return {
    scopeId,
    includeUntracked: buildGitChangesRequestOptions(ignoreUntracked).includeUntracked,
    handle: controller.begin(),
  };
}

describe('gitChangesInteraction', () => {
  it('reloads the active scope and keeps only the latest ignore-untracked toggle current', () => {
    const controller = createGitChangesLoadController();

    const first = queueIgnoreUntrackedReload(controller, 'scope-1', true);
    const second = queueIgnoreUntrackedReload(controller, 'scope-1', false);
    const third = queueIgnoreUntrackedReload(controller, 'scope-1', true);

    expect(
      [first, second, third].map(item => ({
        scopeId: item.scopeId,
        includeUntracked: item.includeUntracked,
      }))
    ).toEqual([
      {
        scopeId: 'scope-1',
        includeUntracked: false,
      },
      {
        scopeId: 'scope-1',
        includeUntracked: true,
      },
      {
        scopeId: 'scope-1',
        includeUntracked: false,
      },
    ]);
    expect(first.handle.signal.aborted).toBe(true);
    expect(second.handle.signal.aborted).toBe(true);
    expect(third.handle.signal.aborted).toBe(false);
    expect(controller.isCurrent(first.handle)).toBe(false);
    expect(controller.isCurrent(second.handle)).toBe(false);
    expect(controller.isCurrent(third.handle)).toBe(true);
  });
});
