export type TimelineRawSurface = 'message' | 'plan';

export interface TimelineRawKeyInput {
  sessionId?: string | null;
  surface: TimelineRawSurface;
  blockKey: string;
}

export interface TimelineRawToggleVisibilityInput {
  activeKey: string;
  rawKey: string;
  rawCapable: boolean;
  rawMode: boolean;
}

export function buildTimelineRawModeKey(input: TimelineRawKeyInput): string {
  return `${input.sessionId || 'unknown'}:${input.surface}:${input.blockKey}`;
}

export function resolveActivatedTimelineRawBlockKey(rawCapable: boolean, rawKey: string): string {
  return rawCapable ? rawKey : '';
}

export function shouldShowTimelineRawToggle(input: TimelineRawToggleVisibilityInput): boolean {
  if (!input.rawCapable) {
    return false;
  }
  return input.activeKey === input.rawKey || input.rawMode;
}

export function pruneActiveTimelineRawBlockKey(activeKey: string, visibleKeys: string[]): string {
  if (!activeKey || visibleKeys.includes(activeKey)) {
    return activeKey;
  }
  return '';
}

export function shouldClearActiveTimelineRawBlockKey(
  activeKey: string,
  clickedInsideRawCard: boolean
): boolean {
  return Boolean(activeKey) && !clickedInsideRawCard;
}
