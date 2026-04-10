import { onBeforeUnmount, onMounted } from 'vue';

export type MobileViewportMetrics = {
  width: number;
  height: number;
};

type MobileKeyboardTrackerOptions = {
  enabled?: () => boolean;
  isTouchDevice?: () => boolean;
  measureViewport?: () => MobileViewportMetrics;
  onDismissed?: () => void;
  dismissSettleMs?: number;
  setTimeoutFn?: (handler: () => void, timeout: number) => number;
  clearTimeoutFn?: (timerId: number) => void;
};

const KEYBOARD_OPEN_MIN_HEIGHT_PX = 120;
const KEYBOARD_OPEN_MIN_HEIGHT_RATIO = 0.18;
const KEYBOARD_DISMISS_MAX_HEIGHT_PX = 72;
const KEYBOARD_DISMISS_MAX_HEIGHT_RATIO = 0.1;
const VIEWPORT_WIDTH_RESET_PX = 48;
const DISMISS_SETTLE_MS = 160;

function isBrowserTouchDevice() {
  if (typeof window === 'undefined' || typeof navigator === 'undefined') {
    return false;
  }
  return 'ontouchstart' in window || navigator.maxTouchPoints > 0;
}

function readViewportMetrics(): MobileViewportMetrics {
  if (typeof window === 'undefined') {
    return { width: 0, height: 0 };
  }

  const visualViewport = window.visualViewport;
  if (visualViewport) {
    return {
      width: Math.round(visualViewport.width),
      height: Math.round(visualViewport.height),
    };
  }

  return {
    width: Math.round(window.innerWidth),
    height: Math.round(window.innerHeight),
  };
}

export function createMobileKeyboardTracker(options: MobileKeyboardTrackerOptions = {}) {
  let started = false;
  let focused = false;
  let keyboardOpen = false;
  let dismissSettling = false;
  let focusAnchor: MobileViewportMetrics | null = null;
  let dismissTimerId: number | null = null;

  const measureViewport = options.measureViewport ?? readViewportMetrics;
  const setTimeoutFn =
    options.setTimeoutFn ?? ((handler, timeout) => window.setTimeout(handler, timeout));
  const clearTimeoutFn = options.clearTimeoutFn ?? (timerId => window.clearTimeout(timerId));

  function isEnabled() {
    return (options.enabled?.() ?? true) && (options.isTouchDevice?.() ?? isBrowserTouchDevice());
  }

  function clearDismissTimer() {
    if (dismissTimerId == null) {
      return;
    }
    clearTimeoutFn(dismissTimerId);
    dismissTimerId = null;
  }

  function clearKeyboardState() {
    clearDismissTimer();
    keyboardOpen = false;
    dismissSettling = false;
  }

  function resetAnchor(snapshot: MobileViewportMetrics | null) {
    focusAnchor = snapshot;
  }

  function scheduleDismissRecovery() {
    clearDismissTimer();
    dismissSettling = true;
    dismissTimerId = setTimeoutFn(() => {
      dismissTimerId = null;
      if (!dismissSettling) {
        return;
      }
      dismissSettling = false;
      focusAnchor = focused ? measureViewport() : null;
      options.onDismissed?.();
    }, options.dismissSettleMs ?? DISMISS_SETTLE_MS);
  }

  function syncViewportState(snapshot = measureViewport()) {
    if (!isEnabled()) {
      clearKeyboardState();
      if (!focused) {
        resetAnchor(null);
      }
      return;
    }

    if (snapshot.width <= 0 || snapshot.height <= 0) {
      return;
    }

    if (!focusAnchor) {
      if (focused) {
        resetAnchor(snapshot);
      }
      return;
    }

    const widthDelta = Math.abs(snapshot.width - focusAnchor.width);
    if (widthDelta > VIEWPORT_WIDTH_RESET_PX) {
      clearKeyboardState();
      resetAnchor(focused ? snapshot : null);
      return;
    }

    if (!keyboardOpen && !dismissSettling && snapshot.height > focusAnchor.height) {
      resetAnchor(snapshot);
    }

    const anchor = focusAnchor;
    if (!anchor) {
      return;
    }

    const heightDelta = Math.max(anchor.height - snapshot.height, 0);
    const openThreshold = Math.max(
      KEYBOARD_OPEN_MIN_HEIGHT_PX,
      Math.round(anchor.height * KEYBOARD_OPEN_MIN_HEIGHT_RATIO)
    );
    const dismissThreshold = Math.max(
      KEYBOARD_DISMISS_MAX_HEIGHT_PX,
      Math.round(anchor.height * KEYBOARD_DISMISS_MAX_HEIGHT_RATIO)
    );

    if (!keyboardOpen && !dismissSettling) {
      if (focused && heightDelta >= openThreshold) {
        keyboardOpen = true;
      }
      return;
    }

    if (heightDelta > dismissThreshold) {
      clearDismissTimer();
      dismissSettling = false;
      keyboardOpen = true;
      return;
    }

    if (keyboardOpen || !dismissSettling) {
      keyboardOpen = false;
      scheduleDismissRecovery();
    }
  }

  function setFocused(nextFocused: boolean) {
    focused = nextFocused;

    if (!isEnabled()) {
      if (!focused) {
        clearKeyboardState();
        resetAnchor(null);
      }
      return;
    }

    if (focused) {
      if (!keyboardOpen && !dismissSettling) {
        resetAnchor(measureViewport());
      } else if (!focusAnchor) {
        resetAnchor(measureViewport());
      }
      return;
    }

    if (!keyboardOpen && !dismissSettling) {
      clearDismissTimer();
      resetAnchor(null);
    }
  }

  function shouldFreezeResizeNow() {
    syncViewportState();
    return isEnabled() && (keyboardOpen || dismissSettling);
  }

  function handleViewportChange() {
    syncViewportState();
  }

  function start() {
    if (started || typeof window === 'undefined') {
      return;
    }
    started = true;
    window.addEventListener('resize', handleViewportChange);
    window.visualViewport?.addEventListener('resize', handleViewportChange);
    window.visualViewport?.addEventListener('scroll', handleViewportChange);
    syncViewportState();
  }

  function stop() {
    if (started && typeof window !== 'undefined') {
      window.removeEventListener('resize', handleViewportChange);
      window.visualViewport?.removeEventListener('resize', handleViewportChange);
      window.visualViewport?.removeEventListener('scroll', handleViewportChange);
    }
    started = false;
    clearKeyboardState();
    resetAnchor(null);
    focused = false;
  }

  return {
    start,
    stop,
    dispose: stop,
    setFocused,
    syncViewportState,
    shouldFreezeResizeNow,
    get isKeyboardOpen() {
      return keyboardOpen;
    },
    get isResizeFrozen() {
      return keyboardOpen || dismissSettling;
    },
  };
}

export function useMobileKeyboard(options: MobileKeyboardTrackerOptions = {}) {
  const tracker = createMobileKeyboardTracker(options);

  onMounted(() => {
    tracker.start();
  });

  onBeforeUnmount(() => {
    tracker.dispose();
  });

  return tracker;
}
