import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

import {
  createWebSessionTimelineFollowState,
  resolveWebSessionTimelineFollowState,
} from '@/components/web-session/webSessionTimelineScroll';

const webSessionPanelPath = fileURLToPath(new URL('../WebSessionPanel.vue', import.meta.url));

describe('webSessionTimelineScroll', () => {
  it('leaves bottom follow mode when the user scrolls upward from the bottom', () => {
    const previous = createWebSessionTimelineFollowState({
      scrollTop: 800,
      scrollHeight: 1000,
      clientHeight: 200,
    });

    const next = resolveWebSessionTimelineFollowState(previous, {
      scrollTop: 780,
      scrollHeight: 1000,
      clientHeight: 200,
    });

    expect(next).toEqual({
      autoFollowBottom: false,
      showJumpToBottom: true,
      lastScrollTop: 780,
    });
  });

  it('does not re-enable follow mode until the timeline reaches the real bottom', () => {
    const previous = {
      autoFollowBottom: false,
      showJumpToBottom: true,
      lastScrollTop: 780,
    };

    expect(
      resolveWebSessionTimelineFollowState(previous, {
        scrollTop: 796,
        scrollHeight: 1000,
        clientHeight: 200,
      })
    ).toEqual({
      autoFollowBottom: true,
      showJumpToBottom: false,
      lastScrollTop: 796,
    });

    expect(
      resolveWebSessionTimelineFollowState(previous, {
        scrollTop: 795,
        scrollHeight: 1000,
        clientHeight: 200,
      })
    ).toEqual({
      autoFollowBottom: false,
      showJumpToBottom: true,
      lastScrollTop: 795,
    });
  });

  it('keeps follow mode after programmatic bottom sync', () => {
    const previous = {
      autoFollowBottom: true,
      showJumpToBottom: false,
      lastScrollTop: 760,
    };

    expect(
      resolveWebSessionTimelineFollowState(previous, {
        scrollTop: 800,
        scrollHeight: 1000,
        clientHeight: 200,
      })
    ).toEqual({
      autoFollowBottom: true,
      showJumpToBottom: false,
      lastScrollTop: 800,
    });
  });

  it('opts the runtime strip out of browser scroll anchoring', () => {
    const source = readFileSync(webSessionPanelPath, 'utf8');

    expect(source).toMatch(/\.runtime-strip\s*\{[^}]*overflow-anchor:\s*none;/s);
  });
});
