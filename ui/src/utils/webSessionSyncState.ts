import type { WebSessionSummary } from '@/types/models';

export function normalizeWebSessionSyncState(
  value?: WebSessionSummary['syncState'] | string | null
): WebSessionSummary['syncState'] {
  switch (value) {
    case 'fresh':
    case 'stale':
      return 'fresh';
    case 'missing':
    case 'syncing':
    case 'error':
      return value;
    default:
      return 'missing';
  }
}
