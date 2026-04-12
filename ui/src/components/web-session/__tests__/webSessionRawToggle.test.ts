import { describe, expect, it } from 'vitest';

import {
  buildTimelineRawModeKey,
  pruneActiveTimelineRawBlockKey,
  resolveActivatedTimelineRawBlockKey,
  shouldClearActiveTimelineRawBlockKey,
  shouldShowTimelineRawToggle,
} from '@/components/web-session/webSessionRawToggle';

describe('webSessionRawToggle', () => {
  it('builds stable per-session raw keys', () => {
    expect(
      buildTimelineRawModeKey({
        sessionId: 'session-1',
        surface: 'message',
        blockKey: 'block-1',
      })
    ).toBe('session-1:message:block-1');

    expect(
      buildTimelineRawModeKey({
        sessionId: '',
        surface: 'plan',
        blockKey: 'block-2',
      })
    ).toBe('unknown:plan:block-2');
  });

  it('only activates raw-capable cards', () => {
    expect(resolveActivatedTimelineRawBlockKey(true, 'session-1:message:block-1')).toBe(
      'session-1:message:block-1'
    );
    expect(resolveActivatedTimelineRawBlockKey(false, 'session-1:message:block-1')).toBe('');
  });

  it('shows the raw toggle only for the active or already-raw card', () => {
    expect(
      shouldShowTimelineRawToggle({
        activeKey: 'session-1:message:block-1',
        rawKey: 'session-1:message:block-1',
        rawCapable: true,
        rawMode: false,
      })
    ).toBe(true);

    expect(
      shouldShowTimelineRawToggle({
        activeKey: '',
        rawKey: 'session-1:message:block-1',
        rawCapable: true,
        rawMode: true,
      })
    ).toBe(true);

    expect(
      shouldShowTimelineRawToggle({
        activeKey: '',
        rawKey: 'session-1:message:block-1',
        rawCapable: true,
        rawMode: false,
      })
    ).toBe(false);

    expect(
      shouldShowTimelineRawToggle({
        activeKey: 'session-1:message:block-1',
        rawKey: 'session-1:message:block-1',
        rawCapable: false,
        rawMode: true,
      })
    ).toBe(false);
  });

  it('prunes the active key when its card leaves the visible list', () => {
    expect(
      pruneActiveTimelineRawBlockKey('session-1:message:block-1', [
        'session-1:message:block-1',
        'session-1:plan:block-2',
      ])
    ).toBe('session-1:message:block-1');

    expect(
      pruneActiveTimelineRawBlockKey('session-1:message:block-1', ['session-1:plan:block-2'])
    ).toBe('');
  });

  it('clears active state only when clicking outside raw-capable cards', () => {
    expect(shouldClearActiveTimelineRawBlockKey('session-1:message:block-1', false)).toBe(true);
    expect(shouldClearActiveTimelineRawBlockKey('session-1:message:block-1', true)).toBe(false);
    expect(shouldClearActiveTimelineRawBlockKey('', false)).toBe(false);
  });
});
