export type WebSessionSidebarScope = 'all' | 'current';

function normalizeProjectId(value: unknown) {
  return String(value || '').trim();
}

export function normalizeWebSessionSidebarScope(value: unknown): WebSessionSidebarScope {
  return value === 'current' ? 'current' : 'all';
}

export function resolveWebSessionSidebarToggleScope(scope: unknown): WebSessionSidebarScope {
  return normalizeWebSessionSidebarScope(scope) === 'current' ? 'all' : 'current';
}

export function resolveWebSessionSidebarProjectIds(input: {
  scope: WebSessionSidebarScope;
  currentProjectId?: string | null;
  allProjectIds: string[];
}) {
  const currentProjectId = normalizeProjectId(input.currentProjectId);
  if (normalizeWebSessionSidebarScope(input.scope) === 'current') {
    return currentProjectId ? [currentProjectId] : [];
  }

  const ordered: string[] = [];
  input.allProjectIds.forEach(projectId => {
    const normalizedProjectId = normalizeProjectId(projectId);
    if (normalizedProjectId && !ordered.includes(normalizedProjectId)) {
      ordered.push(normalizedProjectId);
    }
  });
  return ordered;
}
