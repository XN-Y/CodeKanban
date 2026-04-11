import { describe, expect, it } from 'vitest';

import {
  DEFAULT_MOBILE_VIEW,
  normalizeMobileView,
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
});
