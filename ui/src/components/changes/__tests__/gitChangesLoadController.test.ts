import { describe, expect, it } from 'vitest';

import { createGitChangesLoadController } from '@/components/changes/gitChangesLoadController';

describe('gitChangesLoadController', () => {
  it('keeps only the latest load current and aborts the previous request', () => {
    const controller = createGitChangesLoadController();

    const first = controller.begin();
    const second = controller.begin();

    expect(first.signal.aborted).toBe(true);
    expect(controller.isCurrent(first)).toBe(false);
    expect(second.signal.aborted).toBe(false);
    expect(controller.isCurrent(second)).toBe(true);
  });

  it('cancels the active load and invalidates its handle', () => {
    const controller = createGitChangesLoadController();

    const active = controller.begin();
    controller.cancel();

    expect(active.signal.aborted).toBe(true);
    expect(controller.isCurrent(active)).toBe(false);
  });
});
