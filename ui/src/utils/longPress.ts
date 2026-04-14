export type LongPressPoint = {
  clientX: number;
  clientY: number;
};

type LongPressTrackerOptions = {
  onLongPress: () => void;
  thresholdMs?: number;
  moveTolerancePx?: number;
  setTimeoutFn?: (handler: () => void, timeout: number) => number;
  clearTimeoutFn?: (timerId: number) => void;
};

const DEFAULT_LONG_PRESS_THRESHOLD_MS = 380;
const DEFAULT_MOVE_TOLERANCE_PX = 12;

export function createLongPressTracker(options: LongPressTrackerOptions) {
  let activePointerId: number | null = null;
  let origin: LongPressPoint | null = null;
  let timerId: number | null = null;
  let suppressNextClick = false;
  let pressing = false;

  const thresholdMs = options.thresholdMs ?? DEFAULT_LONG_PRESS_THRESHOLD_MS;
  const moveTolerancePx = options.moveTolerancePx ?? DEFAULT_MOVE_TOLERANCE_PX;
  const setTimeoutFn =
    options.setTimeoutFn ?? ((handler, timeout) => globalThis.setTimeout(handler, timeout));
  const clearTimeoutFn = options.clearTimeoutFn ?? (timer => globalThis.clearTimeout(timer));

  function clearTimer() {
    if (timerId == null) {
      return;
    }
    clearTimeoutFn(timerId);
    timerId = null;
  }

  function resetPress() {
    clearTimer();
    activePointerId = null;
    origin = null;
    pressing = false;
  }

  function pointerDown(pointerId: number, point: LongPressPoint) {
    resetPress();
    suppressNextClick = false;
    activePointerId = pointerId;
    origin = point;
    pressing = true;
    timerId = setTimeoutFn(() => {
      timerId = null;
      if (activePointerId == null || !origin) {
        return;
      }
      suppressNextClick = true;
      options.onLongPress();
    }, thresholdMs);
  }

  function pointerMove(pointerId: number, point: LongPressPoint) {
    if (pointerId !== activePointerId || !origin) {
      return;
    }
    const deltaX = point.clientX - origin.clientX;
    const deltaY = point.clientY - origin.clientY;
    if (Math.hypot(deltaX, deltaY) > moveTolerancePx) {
      resetPress();
    }
  }

  function pointerUp(pointerId: number) {
    if (pointerId !== activePointerId) {
      return;
    }
    resetPress();
  }

  function pointerCancel(pointerId?: number) {
    if (pointerId != null && pointerId !== activePointerId) {
      return;
    }
    resetPress();
  }

  function consumeClick() {
    if (!suppressNextClick) {
      return false;
    }
    suppressNextClick = false;
    return true;
  }

  function isPressing() {
    return pressing;
  }

  return {
    pointerDown,
    pointerMove,
    pointerUp,
    pointerCancel,
    consumeClick,
    isPressing,
  };
}
