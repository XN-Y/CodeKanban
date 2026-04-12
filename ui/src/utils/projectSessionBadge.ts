export type ProjectSessionBadgeKind = 'terminal' | 'webSession';
export type CombinedProjectSessionBadgeKind = 'combined';

type PreferredProjectSessionKindInput = {
  isMobile: boolean;
  isProjectWorkspace?: boolean;
  mobileActiveView?: string | null;
  workspaceActiveTab?: string | null;
};

type ProjectSessionBadgeInput = {
  terminalCount: number;
  webSessionCount: number;
  preferredKind: ProjectSessionBadgeKind;
};

export type ProjectSessionBadge =
  | {
      kind: ProjectSessionBadgeKind;
      count: number;
    }
  | {
      kind: CombinedProjectSessionBadgeKind;
      terminalCount: number;
      webSessionCount: number;
    }
  | null;

function normalizeCount(value: number | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.trunc(value));
}

export function resolvePreferredProjectSessionKind(
  input: PreferredProjectSessionKindInput
): ProjectSessionBadgeKind {
  if (input.isMobile) {
    return input.mobileActiveView === 'webSession' ? 'webSession' : 'terminal';
  }
  if (input.isProjectWorkspace) {
    return input.workspaceActiveTab === 'web' ? 'webSession' : 'terminal';
  }
  return 'terminal';
}

export function resolveProjectSessionBadge(input: ProjectSessionBadgeInput): ProjectSessionBadge {
  const terminalCount = normalizeCount(input.terminalCount);
  const webSessionCount = normalizeCount(input.webSessionCount);

  if (terminalCount <= 0 && webSessionCount <= 0) {
    return null;
  }
  if (terminalCount > 0 && webSessionCount <= 0) {
    return { kind: 'terminal', count: terminalCount };
  }
  if (webSessionCount > 0 && terminalCount <= 0) {
    return { kind: 'webSession', count: webSessionCount };
  }
  return {
    kind: 'combined',
    terminalCount,
    webSessionCount,
  };
}
