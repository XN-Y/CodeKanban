import {
  normalizeDesktopWorkspaceRouteTab,
  type DesktopWorkspaceRouteTab,
} from '@/utils/workspaceRoute';

export function resolveWorkspaceShortcutTarget(
  activeTab: unknown,
  previousTab?: unknown
): DesktopWorkspaceRouteTab {
  const normalizedActiveTab = normalizeDesktopWorkspaceRouteTab(activeTab);
  const normalizedPreviousTab =
    previousTab == null ? null : normalizeDesktopWorkspaceRouteTab(previousTab);

  if (normalizedPreviousTab && normalizedPreviousTab !== normalizedActiveTab) {
    return normalizedPreviousTab;
  }

  return normalizedActiveTab === 'web' ? 'terminal' : 'web';
}
