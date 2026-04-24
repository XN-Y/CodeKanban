import { describe, expect, it } from 'vitest';

import { resolveWebSessionMobileSelectionAction } from '@/components/web-session/webSessionMobileSelection';

function makeTarget(
  overrides: Partial<{
    id: string;
    projectId: string;
    archivedAt: string | null;
  }> = {}
) {
  return {
    id: 'session-1',
    projectId: 'project-1',
    archivedAt: null,
    ...overrides,
  };
}

describe('webSessionMobileSelection', () => {
  it('ignores empty targets', () => {
    expect(
      resolveWebSessionMobileSelectionAction({
        currentProjectId: 'project-1',
        target: makeTarget({ id: '' }),
      })
    ).toEqual({ type: 'none' });
  });

  it('selects local sessions in the current project', () => {
    expect(
      resolveWebSessionMobileSelectionAction({
        currentProjectId: 'project-1',
        target: makeTarget(),
      })
    ).toEqual({
      type: 'select-local',
      sessionId: 'session-1',
    });
  });

  it('navigates to other projects for live sessions outside the current project', () => {
    expect(
      resolveWebSessionMobileSelectionAction({
        currentProjectId: 'project-1',
        target: makeTarget({ projectId: 'project-2' }),
      })
    ).toEqual({
      type: 'navigate-project',
      projectId: 'project-2',
      sessionId: 'session-1',
    });
  });

  it('opens archived previews within the current project', () => {
    expect(
      resolveWebSessionMobileSelectionAction({
        currentProjectId: 'project-1',
        target: makeTarget({
          archivedAt: '2026-04-21T00:00:00.000Z',
        }),
      })
    ).toEqual({
      type: 'open-archived-preview',
      sessionId: 'session-1',
    });
  });

  it('keeps the current archived preview focused when selecting it again', () => {
    expect(
      resolveWebSessionMobileSelectionAction({
        currentProjectId: 'project-1',
        activeArchivedPreviewId: 'session-1',
        target: makeTarget({
          archivedAt: '2026-04-21T00:00:00.000Z',
        }),
      })
    ).toEqual({
      type: 'focus-archived-preview',
      sessionId: 'session-1',
    });
  });

  it('navigates to the owning project for archived sessions outside the current project', () => {
    expect(
      resolveWebSessionMobileSelectionAction({
        currentProjectId: 'project-1',
        target: makeTarget({
          projectId: 'project-2',
          archivedAt: '2026-04-21T00:00:00.000Z',
        }),
      })
    ).toEqual({
      type: 'navigate-project',
      projectId: 'project-2',
      sessionId: 'session-1',
    });
  });
});
