import { describe, expect, it } from 'vitest';

import {
  PROJECT_SIDEBAR_COMPACT_WIDTH,
  PROJECT_SIDEBAR_COMPACT_SNAP_THRESHOLD,
  PROJECT_SIDEBAR_EXPANDED_MIN_WIDTH,
  PROJECT_SIDEBAR_MAX_WIDTH,
  WORKTREE_SIDER_WIDTH,
  MIN_MAIN_WORKSPACE_WIDTH,
  clampProjectSidebarWidth,
  isProjectSidebarCompact,
  resolveProjectSidebarDragWidth,
  resolveProjectSidebarMaxWidth,
} from '@/views/projectWorkspaceSidebar';

describe('projectWorkspaceSidebar', () => {
  it('clamps widths within the supported range', () => {
    expect(clampProjectSidebarWidth(PROJECT_SIDEBAR_EXPANDED_MIN_WIDTH, 180, 340)).toBe(
      PROJECT_SIDEBAR_EXPANDED_MIN_WIDTH
    );
    expect(clampProjectSidebarWidth(PROJECT_SIDEBAR_EXPANDED_MIN_WIDTH, 280, 340)).toBe(280);
    expect(clampProjectSidebarWidth(PROJECT_SIDEBAR_EXPANDED_MIN_WIDTH, 420, 340)).toBe(340);
  });

  it('computes the max width from viewport and worktree visibility', () => {
    expect(
      resolveProjectSidebarMaxWidth({
        windowWidth: WORKTREE_SIDER_WIDTH + MIN_MAIN_WORKSPACE_WIDTH + 280,
        worktreeCollapsed: false,
      })
    ).toBe(280);

    expect(
      resolveProjectSidebarMaxWidth({
        windowWidth: 1400,
        worktreeCollapsed: true,
      })
    ).toBe(PROJECT_SIDEBAR_MAX_WIDTH);
  });

  it('clamps dragged widths between compact and max bounds', () => {
    expect(resolveProjectSidebarDragWidth(40, 312)).toBe(PROJECT_SIDEBAR_COMPACT_WIDTH);
    expect(resolveProjectSidebarDragWidth(188, 312)).toBe(188);
    expect(resolveProjectSidebarDragWidth(460, 312)).toBe(312);
  });

  it('detects compact mode based on the snap threshold', () => {
    expect(isProjectSidebarCompact(PROJECT_SIDEBAR_COMPACT_WIDTH)).toBe(true);
    expect(isProjectSidebarCompact(PROJECT_SIDEBAR_COMPACT_SNAP_THRESHOLD - 1)).toBe(true);
    expect(isProjectSidebarCompact(PROJECT_SIDEBAR_COMPACT_SNAP_THRESHOLD)).toBe(false);
  });

  it('keeps dragged widths continuous without snapping', () => {
    expect(resolveProjectSidebarDragWidth(120, 360)).toBe(120);
    expect(resolveProjectSidebarDragWidth(168, 360)).toBe(168);
    expect(resolveProjectSidebarDragWidth(320, 280)).toBe(280);
  });
});
