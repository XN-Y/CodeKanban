export const DEFAULT_MOBILE_VIEW = 'projects' as const;

const MOBILE_VIEWS = ['kanban', 'terminal', 'webSession', 'files', 'projects', 'notifications'] as const;

export type MobileView = (typeof MOBILE_VIEWS)[number];

export function normalizeMobileView(value: unknown): MobileView {
  if (value === 'kanban') {
    return DEFAULT_MOBILE_VIEW;
  }
  return MOBILE_VIEWS.includes(value as MobileView) ? (value as MobileView) : DEFAULT_MOBILE_VIEW;
}

export function restorePersistedMobileView(value: unknown): MobileView {
  const normalized = normalizeMobileView(value);
  if (normalized === 'notifications') {
    return DEFAULT_MOBILE_VIEW;
  }
  return normalized;
}
