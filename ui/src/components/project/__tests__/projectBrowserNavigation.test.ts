import { describe, expect, it } from 'vitest';

import {
  buildProjectBrowserProjectLocation,
  buildProjectBrowserRouteQuery,
  isCurrentProjectSelection,
} from '@/components/project/projectBrowserNavigation';

describe('projectBrowserNavigation', () => {
  it('detects no-op selection for the current project', () => {
    expect(isCurrentProjectSelection('project-1', 'project-1')).toBe(true);
    expect(isCurrentProjectSelection('project-1', 'project-2')).toBe(false);
    expect(isCurrentProjectSelection('', 'project-2')).toBe(false);
  });

  it('clears workspace and web session query state while preserving unrelated parameters', () => {
    expect(
      buildProjectBrowserRouteQuery(
        {
          filter: 'active',
          tab: 'projects',
          webSessionId: 'session-1',
        },
        'files'
      )
    ).toEqual({
      filter: 'active',
      tab: 'files',
    });
  });

  it('builds a workspace project location with the requested mobile tab', () => {
    expect(
      buildProjectBrowserProjectLocation({
        mode: 'mobile-workspace',
        projectId: 'project-2',
        currentProjectId: 'project-1',
        query: {
          filter: 'active',
          tab: 'projects',
          webSessionId: 'session-9',
        },
        workspaceTab: 'projects',
      })
    ).toEqual({
      name: 'project',
      params: { id: 'project-2' },
      query: {
        filter: 'active',
        tab: 'projects',
      },
    });
  });

  it('keeps page-mode navigation query-free', () => {
    expect(
      buildProjectBrowserProjectLocation({
        mode: 'page',
        projectId: 'project-2',
        currentProjectId: 'project-1',
        query: {
          filter: 'active',
        },
      })
    ).toEqual({
      name: 'project',
      params: { id: 'project-2' },
    });
  });

  it('returns null when selecting the current project', () => {
    expect(
      buildProjectBrowserProjectLocation({
        mode: 'mobile-workspace',
        projectId: 'project-1',
        currentProjectId: 'project-1',
        query: {
          tab: 'projects',
        },
        workspaceTab: 'projects',
      })
    ).toBeNull();
  });
});
