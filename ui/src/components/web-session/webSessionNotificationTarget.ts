import type { LocationQuery, RouteLocationRaw } from 'vue-router';

import { buildWebSessionProjectLocation } from '@/utils/webSessionRoute';

export interface WebSessionNotificationTargetEvent {
  projectId?: string;
  sessionId?: string;
}

export interface OpenWebSessionNotificationTargetOptions {
  event: WebSessionNotificationTargetEvent;
  query?: LocationQuery | null;
  addRecentProject: (projectId: string) => void;
  push: (location: RouteLocationRaw) => Promise<unknown>;
}

export async function openWebSessionNotificationTarget(
  options: OpenWebSessionNotificationTargetOptions
) {
  const projectId = String(options.event.projectId || '').trim();
  const sessionId = String(options.event.sessionId || '').trim();
  const location = buildWebSessionProjectLocation({
    projectId,
    sessionId,
    query: options.query,
  });
  if (!location) {
    return false;
  }

  options.addRecentProject(projectId);
  await options.push(location);
  return true;
}
