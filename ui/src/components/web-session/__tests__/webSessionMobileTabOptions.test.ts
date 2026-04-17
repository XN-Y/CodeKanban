import { describe, expect, it } from 'vitest';

import {
  buildWebSessionMobileTabDescriptors,
  MOBILE_ARCHIVED_LOAD_MORE_OPTION_KEY,
  MOBILE_NEW_SESSION_OPTION_KEY,
} from '@/components/web-session/webSessionMobileTabOptions';

function makeSessions(ids: string[]) {
  return ids.map(id => ({ id }));
}

describe('webSessionMobileTabOptions', () => {
  it('appends new-session after current sessions', () => {
    const descriptors = buildWebSessionMobileTabDescriptors({
      section: 'current',
      sessions: makeSessions(['session-1', 'session-2']),
    });

    expect(descriptors.map(item => item.key)).toEqual([
      'mobile-session-switcher:current',
      'session-1',
      'session-2',
      MOBILE_NEW_SESSION_OPTION_KEY,
    ]);
  });

  it('keeps current empty state ahead of the new-session action', () => {
    const descriptors = buildWebSessionMobileTabDescriptors({
      section: 'current',
      sessions: [],
    });

    expect(
      descriptors.map(item => ({
        kind: item.kind,
        key: item.key,
      }))
    ).toEqual([
      {
        kind: 'header',
        key: 'mobile-session-switcher:current',
      },
      {
        kind: 'empty',
        key: 'mobile-session-empty:current',
      },
      {
        kind: 'new-session',
        key: MOBILE_NEW_SESSION_OPTION_KEY,
      },
    ]);
  });

  it('keeps archived load-more as the final archived item', () => {
    const descriptors = buildWebSessionMobileTabDescriptors({
      section: 'archived',
      sessions: makeSessions(['archived-1']),
      hasArchivedLoadMore: true,
    });

    expect(descriptors.map(item => item.key)).toEqual([
      'mobile-session-switcher:archived',
      'archived-1',
      MOBILE_ARCHIVED_LOAD_MORE_OPTION_KEY,
    ]);
    expect(descriptors.some(item => item.key === MOBILE_NEW_SESSION_OPTION_KEY)).toBe(false);
  });

  it('does not add new-session to archived empty states', () => {
    const descriptors = buildWebSessionMobileTabDescriptors({
      section: 'archived',
      sessions: [],
    });

    expect(
      descriptors.map(item => ({
        kind: item.kind,
        key: item.key,
      }))
    ).toEqual([
      {
        kind: 'header',
        key: 'mobile-session-switcher:archived',
      },
      {
        kind: 'empty',
        key: 'mobile-session-empty:archived',
      },
    ]);
  });

  it('marks the archived load-more descriptor as loading when needed', () => {
    const descriptors = buildWebSessionMobileTabDescriptors({
      section: 'archived',
      sessions: makeSessions(['archived-1']),
      isArchivedLoading: true,
    });

    expect(descriptors.at(-1)).toEqual({
      kind: 'load-more',
      key: MOBILE_ARCHIVED_LOAD_MORE_OPTION_KEY,
      loading: true,
      section: 'archived',
    });
  });
});
