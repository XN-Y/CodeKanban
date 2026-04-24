export type MobileSessionSelectionTarget = {
  id: string;
  projectId: string;
  archivedAt?: string | null;
};

export type WebSessionMobileSelectionAction =
  | { type: 'none' }
  | { type: 'select-local'; sessionId: string }
  | { type: 'open-archived-preview'; sessionId: string }
  | { type: 'focus-archived-preview'; sessionId: string }
  | { type: 'navigate-project'; projectId: string; sessionId: string };

function normalizeText(value: unknown) {
  return typeof value === 'string' ? value.trim() : '';
}

function hasArchivedAtValue(value: unknown) {
  if (typeof value === 'string') {
    return value.trim().length > 0;
  }
  return value != null;
}

export function resolveWebSessionMobileSelectionAction(input: {
  currentProjectId?: string | null;
  activeArchivedPreviewId?: string | null;
  target?: MobileSessionSelectionTarget | null;
}): WebSessionMobileSelectionAction {
  const targetSessionId = normalizeText(input.target?.id);
  if (!targetSessionId) {
    return { type: 'none' };
  }

  const currentProjectId = normalizeText(input.currentProjectId);
  const targetProjectId = normalizeText(input.target?.projectId);
  const activeArchivedPreviewId = normalizeText(input.activeArchivedPreviewId);
  const targetIsArchived = hasArchivedAtValue(input.target?.archivedAt);

  if (targetIsArchived) {
    if (currentProjectId && targetProjectId && targetProjectId !== currentProjectId) {
      return {
        type: 'navigate-project',
        projectId: targetProjectId,
        sessionId: targetSessionId,
      };
    }
    if (targetSessionId === activeArchivedPreviewId) {
      return {
        type: 'focus-archived-preview',
        sessionId: targetSessionId,
      };
    }
    return {
      type: 'open-archived-preview',
      sessionId: targetSessionId,
    };
  }

  if (currentProjectId && targetProjectId && targetProjectId !== currentProjectId) {
    return {
      type: 'navigate-project',
      projectId: targetProjectId,
      sessionId: targetSessionId,
    };
  }

  return {
    type: 'select-local',
    sessionId: targetSessionId,
  };
}
