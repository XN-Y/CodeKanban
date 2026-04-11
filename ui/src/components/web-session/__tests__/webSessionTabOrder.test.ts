import { describe, expect, it } from 'vitest';

import {
  buildOrderedTabSessions,
  clampTabAnchorIndex,
  resolveTabAnchorInsertIndex,
} from '@/components/web-session/webSessionTabOrder';

function makeSessions(ids: string[]) {
  return ids.map(id => ({ id }));
}

describe('webSessionTabOrder', () => {
  it('inserts a fixed archived preview at the anchored index', () => {
    const baseSessions = makeSessions(['real-1', 'draft-1', 'real-2']);
    const archivedPreview = { id: 'archived-1' };

    const ordered = buildOrderedTabSessions(
      ['real-2', 'draft-1', 'real-1'],
      baseSessions,
      archivedPreview,
      1
    );

    expect(ordered.map(session => session.id)).toEqual([
      'real-2',
      'archived-1',
      'draft-1',
      'real-1',
    ]);
  });

  it('keeps the fixed archived preview anchored even if an archived id appears in order ids', () => {
    const baseSessions = makeSessions(['real-1', 'draft-1', 'real-2']);
    const archivedPreview = { id: 'archived-1' };

    const ordered = buildOrderedTabSessions(
      ['real-2', 'archived-1', 'real-1', 'draft-1'],
      baseSessions,
      archivedPreview,
      1
    );

    expect(ordered.map(session => session.id)).toEqual([
      'real-2',
      'archived-1',
      'real-1',
      'draft-1',
    ]);
  });

  it('resolves the archived anchor position after the current tab', () => {
    const orderedSessions = makeSessions(['real-1', 'draft-1', 'real-2']);

    expect(resolveTabAnchorInsertIndex(orderedSessions, 'draft-1')).toBe(2);
    expect(resolveTabAnchorInsertIndex(orderedSessions, 'missing')).toBe(3);
    expect(resolveTabAnchorInsertIndex(orderedSessions, '')).toBe(3);
  });

  it('clamps archived anchor indexes into the current base range', () => {
    expect(clampTabAnchorIndex(-5, 3)).toBe(0);
    expect(clampTabAnchorIndex(2, 3)).toBe(2);
    expect(clampTabAnchorIndex(99, 3)).toBe(3);
    expect(clampTabAnchorIndex(Number.NaN, 3)).toBe(3);
  });
});
