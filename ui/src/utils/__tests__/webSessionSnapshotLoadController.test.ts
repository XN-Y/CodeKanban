import { describe, expect, it } from 'vitest';

import { createWebSessionSnapshotLoadController } from '@/utils/webSessionSnapshotLoadController';

describe('webSessionSnapshotLoadController', () => {
  it('keeps only the latest snapshot load current and aborts the previous one', () => {
    const controller = createWebSessionSnapshotLoadController();

    const first = controller.begin();
    const second = controller.begin();

    expect(first.signal.aborted).toBe(true);
    expect(controller.isCurrent(first)).toBe(false);
    expect(second.signal.aborted).toBe(false);
    expect(controller.isCurrent(second)).toBe(true);
  });

  it('cancels the active load and invalidates its handle', () => {
    const controller = createWebSessionSnapshotLoadController();

    const active = controller.begin();
    controller.cancel();

    expect(active.signal.aborted).toBe(true);
    expect(controller.isCurrent(active)).toBe(false);
  });

  it('releasing a stale handle does not clear the current load', () => {
    const controller = createWebSessionSnapshotLoadController();

    const first = controller.begin();
    const second = controller.begin();

    controller.release(first);

    expect(controller.isCurrent(second)).toBe(true);

    controller.release(second);

    expect(controller.isCurrent(second)).toBe(false);
  });
});
