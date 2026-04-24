import { onBeforeUnmount, onMounted } from 'vue';

export type MobileViewportMetrics = {
  width: number;
  height: number;
};

export type MobileKeyboardTrackerState = {
  isKeyboardOpen: boolean;
  isResizeFrozen: boolean;
};

export type MobileKeyboardTrackerOptions = {
  enabled?: () => boolean;
  isTouchDevice?: () => boolean;
  measureViewport?: () => MobileViewportMetrics;
  onDismissed?: () => void;
  onStateChange?: (state: MobileKeyboardTrackerState) => void;
  dismissSettleMs?: number;
};

export type MobileKeyboardTracker = {
  start: () => void;
  stop: () => void;
  reset: () => void;
  setFocused: (nextFocused: boolean) => void;
  sync: (snapshot?: MobileViewportMetrics) => MobileKeyboardTrackerState;
};

export type MobileKeyboardHandle = Pick<MobileKeyboardTracker, 'reset' | 'setFocused' | 'sync'>;

type MobileKeyboardPhase = 'idle' | 'open' | 'settling';

type TimerHandle = ReturnType<typeof setTimeout>;

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

function buildTrackerState(phase: MobileKeyboardPhase): MobileKeyboardTrackerState {
  return {
    isKeyboardOpen: phase === 'open',
    isResizeFrozen: phase !== 'idle',
  };
}

function sameTrackerState(a: MobileKeyboardTrackerState | null, b: MobileKeyboardTrackerState) {
  return Boolean(
    a && a.isKeyboardOpen === b.isKeyboardOpen && a.isResizeFrozen === b.isResizeFrozen
  );
}

function getHeightThresholds(anchorHeight: number) {
  return {
    open: Math.max(
      KEYBOARD_OPEN_MIN_HEIGHT_PX,
      Math.round(anchorHeight * KEYBOARD_OPEN_MIN_HEIGHT_RATIO)
    ),
    dismiss: Math.max(
      KEYBOARD_DISMISS_MAX_HEIGHT_PX,
      Math.round(anchorHeight * KEYBOARD_DISMISS_MAX_HEIGHT_RATIO)
    ),
  };
}

export function createMobileKeyboardTracker(
  options: MobileKeyboardTrackerOptions = {}
): MobileKeyboardTracker {
  let started = false;
  let focused = false;
  let phase: MobileKeyboardPhase = 'idle';
  let focusAnchor: MobileViewportMetrics | null = null;
  let dismissTimerId: TimerHandle | null = null;
  let lastEmittedState: MobileKeyboardTrackerState | null = null;

  const measureViewport = options.measureViewport ?? readViewportMetrics;
  const dismissSettleMs = options.dismissSettleMs ?? DISMISS_SETTLE_MS;

  function getState() {
    return buildTrackerState(phase);
  }

  function emitStateChange(force = false) {
    if (!options.onStateChange) {
      return;
    }

    const nextState = getState();
    if (!force && sameTrackerState(lastEmittedState, nextState)) {
      return;
    }

    lastEmittedState = nextState;
    options.onStateChange(nextState);
  }

  function setPhase(nextPhase: MobileKeyboardPhase, forceEmit = false) {
    phase = nextPhase;
    emitStateChange(forceEmit);
  }

  function isTrackingEnabled() {
    return (options.enabled?.() ?? true) && (options.isTouchDevice?.() ?? isBrowserTouchDevice());
  }

  function clearDismissTimer() {
    if (dismissTimerId == null) {
      return;
    }
    clearTimeout(dismissTimerId);
    dismissTimerId = null;
  }

  function resetAnchor(snapshot: MobileViewportMetrics | null) {
    focusAnchor = snapshot;
  }

  function captureAnchor(snapshot = measureViewport()) {
    resetAnchor(snapshot);
  }

  function clearState(forceEmit = false) {
    clearDismissTimer();
    setPhase('idle', forceEmit);
  }

  function scheduleDismissRecovery() {
    clearDismissTimer();
    setPhase('settling');
    dismissTimerId = setTimeout(() => {
      dismissTimerId = null;
      if (phase !== 'settling') {
        return;
      }
      resetAnchor(focused ? measureViewport() : null);
      setPhase('idle');
      options.onDismissed?.();
    }, dismissSettleMs);
  }

  function sync(snapshot = measureViewport()) {
    if (!isTrackingEnabled()) {
      clearState();
      if (!focused) {
        resetAnchor(null);
      }
      return getState();
    }

    if (snapshot.width <= 0 || snapshot.height <= 0) {
      return getState();
    }

    if (!focusAnchor) {
      if (focused) {
        captureAnchor(snapshot);
      }
      return getState();
    }

    const widthDelta = Math.abs(snapshot.width - focusAnchor.width);
    if (widthDelta > VIEWPORT_WIDTH_RESET_PX) {
      clearState();
      resetAnchor(focused ? snapshot : null);
      return getState();
    }

    if (phase === 'idle' && snapshot.height > focusAnchor.height) {
      captureAnchor(snapshot);
    }

    const anchor = focusAnchor;
    if (!anchor) {
      return getState();
    }

    const heightDelta = Math.max(anchor.height - snapshot.height, 0);
    const thresholds = getHeightThresholds(anchor.height);

    if (phase === 'idle') {
      if (focused && heightDelta >= thresholds.open) {
        setPhase('open');
      }
      return getState();
    }

    if (heightDelta > thresholds.dismiss) {
      clearDismissTimer();
      setPhase('open');
      return getState();
    }

    scheduleDismissRecovery();
    return getState();
  }

  function setFocused(nextFocused: boolean) {
    focused = nextFocused;

    if (!isTrackingEnabled()) {
      if (!focused) {
        clearState();
        resetAnchor(null);
      }
      return;
    }

    if (focused) {
      if (!focusAnchor || phase === 'idle') {
        captureAnchor();
      }
      return;
    }

    if (phase === 'idle') {
      clearDismissTimer();
      resetAnchor(null);
    }
  }

  function handleViewportChange() {
    sync();
  }

  function start() {
    if (started || typeof window === 'undefined') {
      return;
    }

    started = true;
    window.addEventListener('resize', handleViewportChange);
    window.visualViewport?.addEventListener('resize', handleViewportChange);
    window.visualViewport?.addEventListener('scroll', handleViewportChange);
    sync();
  }

  function reset() {
    clearState(true);
    resetAnchor(null);
    focused = false;
  }

  function stop() {
    if (started && typeof window !== 'undefined') {
      window.removeEventListener('resize', handleViewportChange);
      window.visualViewport?.removeEventListener('resize', handleViewportChange);
      window.visualViewport?.removeEventListener('scroll', handleViewportChange);
    }

    started = false;
    reset();
  }

  return {
    start,
    stop,
    reset,
    setFocused,
    sync,
  };
}

export function useMobileKeyboard(
  options: MobileKeyboardTrackerOptions = {}
): MobileKeyboardHandle {
  const tracker = createMobileKeyboardTracker(options);

  onMounted(() => {
    tracker.start();
  });

  onBeforeUnmount(() => {
    tracker.stop();
  });

  return {
    reset: tracker.reset,
    setFocused: tracker.setFocused,
    sync: tracker.sync,
  };
}
