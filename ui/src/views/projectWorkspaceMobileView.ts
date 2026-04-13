export const DEFAULT_MOBILE_VIEW = 'projects' as const;

const MOBILE_VIEWS = [
  'kanban',
  'terminal',
  'webSession',
  'files',
  'projects',
  'notifications',
] as const;
const MOBILE_ROUTE_TABS = ['projects', 'terminal', 'web', 'files', 'notifications'] as const;

export type MobileView = (typeof MOBILE_VIEWS)[number];
export type MobileRouteTab = (typeof MOBILE_ROUTE_TABS)[number];

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

export function mobileViewToRouteTab(value: unknown): MobileRouteTab {
  switch (normalizeMobileView(value)) {
    case 'terminal':
      return 'terminal';
    case 'webSession':
      return 'web';
    case 'files':
      return 'files';
    case 'notifications':
      return 'notifications';
    case 'projects':
    case 'kanban':
    default:
      return 'projects';
  }
}

export function routeTabToMobileView(value: unknown): MobileView {
  switch (value) {
    case 'terminal':
      return 'terminal';
    case 'web':
      return 'webSession';
    case 'files':
      return 'files';
    case 'notifications':
      return 'notifications';
    case 'projects':
    case 'kanban':
    default:
      return DEFAULT_MOBILE_VIEW;
  }
}
