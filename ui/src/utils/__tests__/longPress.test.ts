import { beforeEach, describe, expect, it, vi } from 'vitest';

import { createLongPressTracker } from '@/utils/longPress';

describe('createLongPressTracker', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  it('fires after the threshold and suppresses the following click once', () => {
    const onLongPress = vi.fn();
    const tracker = createLongPressTracker({
      onLongPress,
    });

    tracker.pointerDown(1, { clientX: 24, clientY: 32 });
    vi.advanceTimersByTime(379);
    expect(onLongPress).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1);
    expect(onLongPress).toHaveBeenCalledTimes(1);

    tracker.pointerUp(1);
    expect(tracker.consumeClick()).toBe(true);
    expect(tracker.consumeClick()).toBe(false);
  });

  it('does not fire when the pointer is released early', () => {
    const onLongPress = vi.fn();
    const tracker = createLongPressTracker({
      onLongPress,
    });

    tracker.pointerDown(1, { clientX: 24, clientY: 32 });
    vi.advanceTimersByTime(160);
    tracker.pointerUp(1);
    vi.advanceTimersByTime(500);

    expect(onLongPress).not.toHaveBeenCalled();
    expect(tracker.consumeClick()).toBe(false);
  });

  it('cancels when the pointer moves past the tolerance', () => {
    const onLongPress = vi.fn();
    const tracker = createLongPressTracker({
      onLongPress,
    });

    tracker.pointerDown(1, { clientX: 24, clientY: 32 });
    tracker.pointerMove(1, { clientX: 40, clientY: 32 });
    vi.advanceTimersByTime(500);

    expect(onLongPress).not.toHaveBeenCalled();
    expect(tracker.isPressing()).toBe(false);
  });

  it('ignores unrelated pointer ids for move and release', () => {
    const onLongPress = vi.fn();
    const tracker = createLongPressTracker({
      onLongPress,
    });

    tracker.pointerDown(3, { clientX: 10, clientY: 10 });
    tracker.pointerMove(8, { clientX: 64, clientY: 64 });
    tracker.pointerUp(8);
    vi.advanceTimersByTime(380);

    expect(onLongPress).toHaveBeenCalledTimes(1);
  });

  it('clears the press state on cancel without suppressing a click', () => {
    const onLongPress = vi.fn();
    const tracker = createLongPressTracker({
      onLongPress,
    });

    tracker.pointerDown(1, { clientX: 24, clientY: 32 });
    expect(tracker.isPressing()).toBe(true);

    tracker.pointerCancel(1);
    vi.advanceTimersByTime(500);

    expect(onLongPress).not.toHaveBeenCalled();
    expect(tracker.isPressing()).toBe(false);
    expect(tracker.consumeClick()).toBe(false);
  });

  it('drops stale click suppression when a new press starts', () => {
    const onLongPress = vi.fn();
    const tracker = createLongPressTracker({
      onLongPress,
    });

    tracker.pointerDown(1, { clientX: 24, clientY: 32 });
    vi.advanceTimersByTime(380);
    tracker.pointerUp(1);

    tracker.pointerDown(2, { clientX: 48, clientY: 64 });
    tracker.pointerUp(2);

    expect(tracker.consumeClick()).toBe(false);
  });
});
