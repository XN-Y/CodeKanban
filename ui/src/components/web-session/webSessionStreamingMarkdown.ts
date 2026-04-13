export interface WebSessionStreamingMarkdownEntry {
  key: string;
  text: string;
}

export interface WebSessionStreamingMarkdownControllerOptions {
  delayMs?: number;
  setTimeoutFn?: typeof globalThis.setTimeout;
  clearTimeoutFn?: typeof globalThis.clearTimeout;
  onStateChange?: (state: Record<string, string>) => void;
}

type StreamingMarkdownTimer = ReturnType<typeof globalThis.setTimeout>;

export const WEB_SESSION_STREAMING_MARKDOWN_DELAY_MS = 100;

export function createWebSessionStreamingMarkdownController(
  options: WebSessionStreamingMarkdownControllerOptions = {}
) {
  let delayMs = Math.max(0, options.delayMs ?? WEB_SESSION_STREAMING_MARKDOWN_DELAY_MS);
  const setTimeoutFn =
    options.setTimeoutFn ?? ((handler, delay) => globalThis.setTimeout(handler, delay));
  const clearTimeoutFn = options.clearTimeoutFn ?? (timer => globalThis.clearTimeout(timer));
  const onStateChange = options.onStateChange;

  const displayedTextByKey = new Map<string, string>();
  const pendingTextByKey = new Map<string, string>();
  const timerByKey = new Map<string, StreamingMarkdownTimer>();

  function snapshotState() {
    return Object.fromEntries(displayedTextByKey);
  }

  function emitChange() {
    onStateChange?.(snapshotState());
  }

  function clearTimer(key: string) {
    const timer = timerByKey.get(key);
    if (timer == null) {
      return;
    }
    clearTimeoutFn(timer);
    timerByKey.delete(key);
  }

  function setDisplayedText(key: string, text: string) {
    if (displayedTextByKey.get(key) === text) {
      return false;
    }
    displayedTextByKey.set(key, text);
    return true;
  }

  function flushKey(key: string) {
    clearTimer(key);
    const pendingText = pendingTextByKey.get(key);
    if (pendingText == null) {
      return false;
    }
    pendingTextByKey.delete(key);
    return setDisplayedText(key, pendingText);
  }

  function scheduleKey(key: string) {
    if (timerByKey.has(key)) {
      return;
    }
    const timer = setTimeoutFn(() => {
      timerByKey.delete(key);
      if (flushKey(key)) {
        emitChange();
      }
    }, delayMs);
    timerByKey.set(key, timer);
  }

  function setDelayMs(nextDelayMs: number) {
    delayMs = Math.max(0, Number(nextDelayMs) || 0);
    if (timerByKey.size === 0) {
      return;
    }
    const pendingKeys = Array.from(timerByKey.keys());
    pendingKeys.forEach(clearTimer);
    pendingKeys.forEach(scheduleKey);
  }

  function pruneMissingKeys(nextKeys: Set<string>) {
    let changed = false;
    Array.from(displayedTextByKey.keys()).forEach(key => {
      if (nextKeys.has(key)) {
        return;
      }
      clearTimer(key);
      pendingTextByKey.delete(key);
      changed = displayedTextByKey.delete(key) || changed;
    });
    return changed;
  }

  function sync(entries: WebSessionStreamingMarkdownEntry[]) {
    const nextKeys = new Set(entries.map(entry => entry.key));
    let changed = pruneMissingKeys(nextKeys);

    entries.forEach(entry => {
      const currentDisplayed = displayedTextByKey.get(entry.key);
      if (currentDisplayed == null) {
        changed = setDisplayedText(entry.key, entry.text) || changed;
        pendingTextByKey.delete(entry.key);
        clearTimer(entry.key);
        return;
      }
      if (currentDisplayed === entry.text && pendingTextByKey.get(entry.key) == null) {
        return;
      }
      pendingTextByKey.set(entry.key, entry.text);
      scheduleKey(entry.key);
    });

    if (changed) {
      emitChange();
    }
  }

  function flush(keys?: string[]) {
    let changed = false;
    const targetKeys =
      Array.isArray(keys) && keys.length > 0 ? keys : Array.from(pendingTextByKey.keys());
    targetKeys.forEach(key => {
      changed = flushKey(key) || changed;
    });
    if (changed) {
      emitChange();
    }
  }

  function clear() {
    Array.from(timerByKey.keys()).forEach(clearTimer);
    pendingTextByKey.clear();
    if (displayedTextByKey.size === 0) {
      return;
    }
    displayedTextByKey.clear();
    emitChange();
  }

  function getDisplayedText(key: string) {
    return displayedTextByKey.get(key);
  }

  return {
    clear,
    flush,
    getDisplayedText,
    snapshotState,
    sync,
    setDelayMs,
  };
}
