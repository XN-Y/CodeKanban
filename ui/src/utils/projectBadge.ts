export type ProjectBadge = {
  label: string;
  color: string;
};

export const PROJECT_BADGE_COLORS = [
  '#10b981',
  '#3b82f6',
  '#f59e0b',
  '#8b5cf6',
  '#ec4899',
  '#14b8a6',
  '#ef4444',
  '#6366f1',
];

const PROJECT_BADGE_CONTENT_PATTERN = /[\p{L}\p{N}]/u;
const ASCII_LOWERCASE_PATTERN = /^[a-z]$/u;

export function resolveProjectBadgeLabel(projectName?: string | null, fallback = '?') {
  const trimmedName = typeof projectName === 'string' ? projectName.trim() : '';
  if (!trimmedName) {
    return fallback;
  }

  const characters = Array.from(trimmedName);
  const label = characters.find(character => PROJECT_BADGE_CONTENT_PATTERN.test(character));
  if (!label) {
    return characters[0] ?? fallback;
  }
  return ASCII_LOWERCASE_PATTERN.test(label) ? label.toUpperCase() : label;
}

export function buildProjectBadgeMap(
  projectIds: Array<string | null | undefined>,
  getProjectName: (projectId: string) => string
) {
  const badgeMap = new Map<string, ProjectBadge>();

  projectIds.forEach(projectId => {
    if (!projectId || badgeMap.has(projectId)) {
      return;
    }

    const projectName = getProjectName(projectId) || projectId;
    const badgeIndex = badgeMap.size;
    badgeMap.set(projectId, {
      label: resolveProjectBadgeLabel(projectName),
      color: PROJECT_BADGE_COLORS[badgeIndex % PROJECT_BADGE_COLORS.length],
    });
  });

  return badgeMap;
}
