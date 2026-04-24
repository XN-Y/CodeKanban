import { afterEach, describe, expect, it, vi } from 'vitest';

import { createMobileKeyboardTracker } from '@/composables/useMobileKeyboard';

describe('createMobileKeyboardTracker', () => {
  afterEach(() => {
    vi.useRealTimers();
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

    expect(tracker.sync()).toEqual({
      isKeyboardOpen: false,
      isResizeFrozen: false,
    });

    viewport = { width: 390, height: 560 };

    expect(tracker.sync()).toEqual({
      isKeyboardOpen: true,
      isResizeFrozen: true,
    });
  });

  it('keeps resize frozen until dismissal settles, then runs recovery once', () => {
    vi.useFakeTimers();

    let viewport = { width: 390, height: 844 };
    const onDismissed = vi.fn();
    const tracker = createMobileKeyboardTracker({
      enabled: () => true,
      isTouchDevice: () => true,
      measureViewport: () => viewport,
      onDismissed,
    });

    tracker.setFocused(true);
    viewport = { width: 390, height: 560 };
    expect(tracker.sync().isResizeFrozen).toBe(true);

    viewport = { width: 390, height: 820 };
    expect(tracker.sync()).toEqual({
      isKeyboardOpen: false,
      isResizeFrozen: true,
    });
    expect(onDismissed).not.toHaveBeenCalled();

    vi.advanceTimersByTime(160);

    expect(onDismissed).toHaveBeenCalledTimes(1);
    expect(tracker.sync()).toEqual({
      isKeyboardOpen: false,
      isResizeFrozen: false,
    });
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

    expect(tracker.sync()).toEqual({
      isKeyboardOpen: false,
      isResizeFrozen: false,
    });
  });

  it('falls back to window.innerHeight when visualViewport is unavailable', () => {
    const fakeWindow = {
      innerWidth: 390,
      innerHeight: 844,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as Window;

    globalThis.window = fakeWindow;

    const tracker = createMobileKeyboardTracker({
      enabled: () => true,
      isTouchDevice: () => true,
    });

    tracker.setFocused(true);
    fakeWindow.innerHeight = 560;

    expect(tracker.sync()).toEqual({
      isKeyboardOpen: true,
      isResizeFrozen: true,
    });
  });

  it('emits state changes for keyboard transitions and reset', () => {
    let viewport = { width: 390, height: 844 };
    const onStateChange = vi.fn();
    const tracker = createMobileKeyboardTracker({
      enabled: () => true,
      isTouchDevice: () => true,
      measureViewport: () => viewport,
      onStateChange,
    });

    tracker.setFocused(true);
    viewport = { width: 390, height: 560 };

    expect(tracker.sync()).toEqual({
      isKeyboardOpen: true,
      isResizeFrozen: true,
    });
    expect(onStateChange).toHaveBeenLastCalledWith({
      isKeyboardOpen: true,
      isResizeFrozen: true,
    });

    tracker.reset();

    expect(onStateChange).toHaveBeenLastCalledWith({
      isKeyboardOpen: false,
      isResizeFrozen: false,
    });
  });
});
