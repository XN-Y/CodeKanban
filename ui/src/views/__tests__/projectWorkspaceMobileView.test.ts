import { describe, expect, it } from 'vitest';

import {
  DEFAULT_MOBILE_VIEW,
  mobileViewToRouteTab,
  normalizeMobileView,
  routeTabToMobileView,
  restorePersistedMobileView,
} from '@/views/projectWorkspaceMobileView';

describe('projectWorkspaceMobileView', () => {
  it('keeps visible mobile views unchanged', () => {
    expect(normalizeMobileView('projects')).toBe('projects');
    expect(normalizeMobileView('terminal')).toBe('terminal');
    expect(normalizeMobileView('webSession')).toBe('webSession');
    expect(normalizeMobileView('files')).toBe('files');
    expect(normalizeMobileView('notifications')).toBe('notifications');
  });

  it('blocks kanban and falls back invalid values to the default mobile view', () => {
    expect(normalizeMobileView('kanban')).toBe(DEFAULT_MOBILE_VIEW);
    expect(normalizeMobileView('unknown')).toBe(DEFAULT_MOBILE_VIEW);
    expect(normalizeMobileView(null)).toBe(DEFAULT_MOBILE_VIEW);
    expect(normalizeMobileView(undefined)).toBe(DEFAULT_MOBILE_VIEW);
  });

  it('maps hidden persisted views back to projects', () => {
    expect(restorePersistedMobileView('notifications')).toBe(DEFAULT_MOBILE_VIEW);
    expect(restorePersistedMobileView('kanban')).toBe(DEFAULT_MOBILE_VIEW);
  });

  it('keeps non-hidden persisted views unchanged', () => {
    expect(restorePersistedMobileView('projects')).toBe('projects');
    expect(restorePersistedMobileView('terminal')).toBe('terminal');
    expect(restorePersistedMobileView('webSession')).toBe('webSession');
    expect(restorePersistedMobileView('files')).toBe('files');
  });

  it('maps mobile views to route tabs', () => {
    expect(mobileViewToRouteTab('projects')).toBe('projects');
    expect(mobileViewToRouteTab('terminal')).toBe('terminal');
    expect(mobileViewToRouteTab('webSession')).toBe('web');
    expect(mobileViewToRouteTab('files')).toBe('files');
    expect(mobileViewToRouteTab('notifications')).toBe('notifications');
    expect(mobileViewToRouteTab('kanban')).toBe('projects');
  });

  it('maps route tabs back to visible mobile views', () => {
    expect(routeTabToMobileView('projects')).toBe('projects');
    expect(routeTabToMobileView('terminal')).toBe('terminal');
    expect(routeTabToMobileView('web')).toBe('webSession');
    expect(routeTabToMobileView('files')).toBe('files');
    expect(routeTabToMobileView('notifications')).toBe('notifications');
    expect(routeTabToMobileView('kanban')).toBe(DEFAULT_MOBILE_VIEW);
    expect(routeTabToMobileView('unknown')).toBe(DEFAULT_MOBILE_VIEW);
  });
});
