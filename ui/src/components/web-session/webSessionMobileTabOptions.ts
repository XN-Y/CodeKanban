export type MobileSessionCategory = 'current' | 'archived';

export const MOBILE_NEW_SESSION_OPTION_KEY = '__mobile-new-session__';
export const MOBILE_ARCHIVED_LOAD_MORE_OPTION_KEY = 'mobile-session-load-more-archived';

export type WebSessionMobileTabDescriptor<TSession extends { id: string }> =
  | {
      kind: 'header';
      key: string;
      section: MobileSessionCategory;
    }
  | {
      kind: 'session';
      key: string;
      section: MobileSessionCategory;
      session: TSession;
    }
  | {
      kind: 'empty';
      key: string;
      section: MobileSessionCategory;
    }
  | {
      kind: 'load-more';
      key: typeof MOBILE_ARCHIVED_LOAD_MORE_OPTION_KEY;
      section: 'archived';
      loading: boolean;
    }
  | {
      kind: 'new-session';
      key: typeof MOBILE_NEW_SESSION_OPTION_KEY;
      section: 'current';
    };

export function buildWebSessionMobileTabDescriptors<TSession extends { id: string }>(input: {
  section: MobileSessionCategory;
  sessions: TSession[];
  hasArchivedLoadMore?: boolean;
  isArchivedLoading?: boolean;
}) {
  const descriptors: WebSessionMobileTabDescriptor<TSession>[] = [
    {
      kind: 'header',
      key: `mobile-session-switcher:${input.section}`,
      section: input.section,
    },
  ];

  if (input.sessions.length > 0) {
    input.sessions.forEach(session => {
      descriptors.push({
        kind: 'session',
        key: session.id,
        section: input.section,
        session,
      });
    });
  } else {
    descriptors.push({
      kind: 'empty',
      key: `mobile-session-empty:${input.section}`,
      section: input.section,
    });
  }

  if (input.section === 'current') {
    descriptors.push({
      kind: 'new-session',
      key: MOBILE_NEW_SESSION_OPTION_KEY,
      section: 'current',
    });
    return descriptors;
  }

  if (input.hasArchivedLoadMore || input.isArchivedLoading) {
    descriptors.push({
      kind: 'load-more',
      key: MOBILE_ARCHIVED_LOAD_MORE_OPTION_KEY,
      section: 'archived',
      loading: input.isArchivedLoading === true,
    });
  }

  return descriptors;
}
