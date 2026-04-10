import { afterEach, describe, expect, it, vi } from 'vitest';

import { createMobileKeyboardTracker } from '@/composables/useMobileKeyboard';

describe('createMobileKeyboardTracker', () => {
  afterEach(() => {
    delete (globalThis as { window?: Window }).window;
  });

  it('freezes resize when a focused mobile viewport shrinks like a soft keyboard', () => {
    let viewport = { width: 390, height: 844 };
    const tracker = createMobileKeyboardTracker({
      enabled: () => true,
      isTouchDevice: () => true,
      measureViewport: () => viewport,
    });

    tracker.setFocused(true);

    expect(tracker.shouldFreezeResizeNow()).toBe(false);

    viewport = { width: 390, height: 560 };

    expect(tracker.shouldFreezeResizeNow()).toBe(true);
    expect(tracker.isKeyboardOpen).toBe(true);
    expect(tracker.isResizeFrozen).toBe(true);
  });

  it('keeps resize frozen until dismissal settles, then runs recovery once', () => {
    let viewport = { width: 390, height: 844 };
    let nextTimerId = 0;
    const pendingTimers = new Map<number, () => void>();
    const onDismissed = vi.fn();

    const tracker = createMobileKeyboardTracker({
      enabled: () => true,
      isTouchDevice: () => true,
      measureViewport: () => viewport,
      onDismissed,
      setTimeoutFn: handler => {
        nextTimerId += 1;
        pendingTimers.set(nextTimerId, handler);
        return nextTimerId;
      },
      clearTimeoutFn: timerId => {
        pendingTimers.delete(timerId);
      },
    });

    tracker.setFocused(true);
    viewport = { width: 390, height: 560 };
    expect(tracker.shouldFreezeResizeNow()).toBe(true);

    viewport = { width: 390, height: 820 };
    expect(tracker.shouldFreezeResizeNow()).toBe(true);
    expect(onDismissed).not.toHaveBeenCalled();
    expect(pendingTimers.size).toBe(1);

    const settle = Array.from(pendingTimers.values())[0];
    settle?.();

    expect(onDismissed).toHaveBeenCalledTimes(1);
    expect(tracker.shouldFreezeResizeNow()).toBe(false);
    expect(tracker.isResizeFrozen).toBe(false);
  });

  it('treats meaningful width changes as a real viewport resize instead of a keyboard event', () => {
    let viewport = { width: 390, height: 844 };
    const tracker = createMobileKeyboardTracker({
      enabled: () => true,
      isTouchDevice: () => true,
      measureViewport: () => viewport,
    });

    tracker.setFocused(true);
    viewport = { width: 470, height: 560 };

    expect(tracker.shouldFreezeResizeNow()).toBe(false);
    expect(tracker.isKeyboardOpen).toBe(false);
  });

  it('falls back to window.innerHeight when visualViewport is unavailable', () => {
    const fakeWindow = {
      innerWidth: 390,
      innerHeight: 844,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      setTimeout: vi.fn((_handler: () => void, _timeout: number) => 1),
      clearTimeout: vi.fn(),
    } as unknown as Window;

    globalThis.window = fakeWindow;

    const tracker = createMobileKeyboardTracker({
      enabled: () => true,
      isTouchDevice: () => true,
    });

    tracker.setFocused(true);
    fakeWindow.innerHeight = 560;

    expect(tracker.shouldFreezeResizeNow()).toBe(true);
  });
});
