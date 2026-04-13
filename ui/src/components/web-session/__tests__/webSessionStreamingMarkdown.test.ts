import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  createWebSessionStreamingMarkdownController,
  WEB_SESSION_STREAMING_MARKDOWN_DELAY_MS,
} from '@/components/web-session/webSessionStreamingMarkdown';

describe('webSessionStreamingMarkdown', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows the first chunk immediately and throttles later updates', () => {
    const states: Array<Record<string, string>> = [];
    const controller = createWebSessionStreamingMarkdownController({
      delayMs: WEB_SESSION_STREAMING_MARKDOWN_DELAY_MS,
      onStateChange: state => {
        states.push(state);
      },
    });

    controller.sync([{ key: 'assistant:message', text: 'first' }]);
    expect(controller.getDisplayedText('assistant:message')).toBe('first');
    expect(states).toEqual([{ 'assistant:message': 'first' }]);

    controller.sync([{ key: 'assistant:message', text: 'second' }]);
    expect(controller.getDisplayedText('assistant:message')).toBe('first');

    vi.advanceTimersByTime(WEB_SESSION_STREAMING_MARKDOWN_DELAY_MS - 1);
    expect(controller.getDisplayedText('assistant:message')).toBe('first');

    vi.advanceTimersByTime(1);
    expect(controller.getDisplayedText('assistant:message')).toBe('second');
    expect(states).toEqual([{ 'assistant:message': 'first' }, { 'assistant:message': 'second' }]);
  });

  it('keeps only the latest pending update within the throttle window', () => {
    const controller = createWebSessionStreamingMarkdownController({
      delayMs: WEB_SESSION_STREAMING_MARKDOWN_DELAY_MS,
    });

    controller.sync([{ key: 'assistant:message', text: 'first' }]);
    controller.sync([{ key: 'assistant:message', text: 'second' }]);
    controller.sync([{ key: 'assistant:message', text: 'third' }]);

    vi.advanceTimersByTime(WEB_SESSION_STREAMING_MARKDOWN_DELAY_MS);
    expect(controller.getDisplayedText('assistant:message')).toBe('third');
  });

  it('removes stale keys and clears pending updates when a block stops streaming', () => {
    const states: Array<Record<string, string>> = [];
    const controller = createWebSessionStreamingMarkdownController({
      delayMs: WEB_SESSION_STREAMING_MARKDOWN_DELAY_MS,
      onStateChange: state => {
        states.push(state);
      },
    });

    controller.sync([{ key: 'assistant:message', text: 'first' }]);
    controller.sync([{ key: 'assistant:message', text: 'second' }]);
    controller.sync([]);

    expect(controller.snapshotState()).toEqual({});

    vi.advanceTimersByTime(WEB_SESSION_STREAMING_MARKDOWN_DELAY_MS);
    expect(controller.snapshotState()).toEqual({});
    expect(states).toEqual([{ 'assistant:message': 'first' }, {}]);
  });
});
