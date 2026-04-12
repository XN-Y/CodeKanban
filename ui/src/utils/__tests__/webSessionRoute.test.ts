import { describe, expect, it } from 'vitest';

import {
  buildWebSessionRouteQuery,
  getWebSessionRouteSessionId,
  isWebSessionRouteQuerySynced,
  isWebSessionOnlyRouteChange,
  resolveWebSessionDeepLinkTarget,
} from '@/utils/webSessionRoute';

describe('webSessionRoute', () => {
  it('reads the first valid webSessionId from the route query', () => {
    expect(getWebSessionRouteSessionId({ webSessionId: ' session-1 ' })).toBe('session-1');
    expect(getWebSessionRouteSessionId({ webSessionId: ['', 'session-2'] })).toBe('session-2');
    expect(getWebSessionRouteSessionId({ webSessionId: null })).toBe('');
  });

  it('builds route queries while preserving unrelated query parameters', () => {
    expect(
      buildWebSessionRouteQuery(
        {
          filter: 'active',
          tab: 'web',
          webSessionId: 'old-session',
        },
        'new-session'
      )
    ).toEqual({
      filter: 'active',
      tab: 'web',
      webSessionId: 'new-session',
    });
  });

  it('clears webSessionId without removing unrelated query parameters', () => {
    expect(
      buildWebSessionRouteQuery({
        filter: 'archived',
        tab: 'terminal',
        webSessionId: 'session-1',
      })
    ).toEqual({
      filter: 'archived',
      tab: 'terminal',
    });
  });

  it('compares the current query and target session id after normalization', () => {
    expect(isWebSessionRouteQuerySynced({ webSessionId: ' session-1 ' }, 'session-1')).toBe(true);
    expect(isWebSessionRouteQuerySynced({ webSessionId: 'session-1' }, 'session-2')).toBe(false);
  });

  it('detects query-only session changes on the same route', () => {
    expect(
      isWebSessionOnlyRouteChange(
        {
          name: 'project',
          path: '/project/project-1',
          params: { id: 'project-1' },
          query: { webSessionId: 'session-1', filter: 'active' },
        },
        {
          name: 'project',
          path: '/project/project-1',
          params: { id: 'project-1' },
          query: { webSessionId: 'session-2', filter: 'active' },
        }
      )
    ).toBe(true);
  });

  it('does not treat project switches or other query changes as silent session changes', () => {
    expect(
      isWebSessionOnlyRouteChange(
        {
          name: 'project',
          path: '/project/project-1',
          params: { id: 'project-1' },
          query: { webSessionId: 'session-1', filter: 'active' },
        },
        {
          name: 'project',
          path: '/project/project-2',
          params: { id: 'project-2' },
          query: { webSessionId: 'session-2', filter: 'active' },
        }
      )
    ).toBe(false);

    expect(
      isWebSessionOnlyRouteChange(
        {
          name: 'project',
          path: '/project/project-1',
          params: { id: 'project-1' },
          query: { webSessionId: 'session-1', filter: 'active' },
        },
        {
          name: 'project',
          path: '/project/project-1',
          params: { id: 'project-1' },
          query: { webSessionId: 'session-2', filter: 'archived' },
        }
      )
    ).toBe(false);
  });

  it('activates a loaded session without requesting an extra snapshot', () => {
    expect(
      resolveWebSessionDeepLinkTarget({
        currentProjectId: 'project-1',
        requestedSessionId: 'session-1',
        loadedSessions: [{ id: 'session-1' }],
      })
    ).toEqual({
      action: 'activate-loaded',
      sessionId: 'session-1',
    });
  });

  it('opens an archived preview when the snapshot belongs to the current project', () => {
    expect(
      resolveWebSessionDeepLinkTarget({
        currentProjectId: 'project-1',
        requestedSessionId: 'session-2',
        loadedSessions: [],
        snapshotSession: {
          id: 'session-2',
          projectId: 'project-1',
          archivedAt: '2026-04-11T10:00:00.000Z',
        },
      })
    ).toEqual({
      action: 'open-archived-preview',
      sessionId: 'session-2',
    });
  });

  it('activates a live session returned by snapshot loading', () => {
    expect(
      resolveWebSessionDeepLinkTarget({
        currentProjectId: 'project-1',
        requestedSessionId: 'session-3',
        loadedSessions: [],
        snapshotSession: {
          id: 'session-3',
          projectId: 'project-1',
          archivedAt: null,
        },
      })
    ).toEqual({
      action: 'activate-real',
      sessionId: 'session-3',
    });
  });

  it('clears invalid deep links when the snapshot is missing or mismatched', () => {
    expect(
      resolveWebSessionDeepLinkTarget({
        currentProjectId: 'project-1',
        requestedSessionId: 'session-4',
        loadedSessions: [],
        snapshotSession: null,
      })
    ).toEqual({
      action: 'clear-invalid',
    });

    expect(
      resolveWebSessionDeepLinkTarget({
        currentProjectId: 'project-1',
        requestedSessionId: 'session-4',
        loadedSessions: [],
        snapshotSession: {
          id: 'session-4',
          projectId: 'project-2',
          archivedAt: null,
        },
      })
    ).toEqual({
      action: 'clear-invalid',
    });
  });
});
