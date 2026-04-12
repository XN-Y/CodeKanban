import { describe, expect, it } from 'vitest';

import {
  collapseProjectDraftTabs,
  pickPreferredDraftTab,
} from '@/components/web-session/webSessionDraftTabs';

function makeDraft(id: string, updatedAt: string) {
  return {
    id,
    updatedAt,
  };
}

describe('webSessionDraftTabs', () => {
  it('prefers the stored active draft when choosing a reusable draft tab', () => {
    const draftA = makeDraft('draft-a', '2026-04-12T09:00:00.000Z');
    const draftB = makeDraft('draft-b', '2026-04-12T10:00:00.000Z');

    const selected = pickPreferredDraftTab([draftA, draftB], {
      activeDraftId: ' draft-a ',
      mruIds: ['draft-b'],
    });

    expect(selected).toEqual(draftA);
  });

  it('falls back to MRU order when the stored active draft is missing', () => {
    const draftA = makeDraft('draft-a', '2026-04-12T09:00:00.000Z');
    const draftB = makeDraft('draft-b', '2026-04-12T10:00:00.000Z');

    const selected = pickPreferredDraftTab([draftA, draftB], {
      activeDraftId: 'missing',
      mruIds: ['session-1', ' draft-b ', 'draft-a'],
    });

    expect(selected).toEqual(draftB);
  });

  it('falls back to the latest updated draft when neither active nor MRU can decide', () => {
    const draftA = makeDraft('draft-a', '2026-04-12T09:00:00.000Z');
    const draftB = makeDraft('draft-b', '2026-04-12T10:00:00.000Z');
    const draftC = makeDraft('draft-c', '2026-04-12T08:00:00.000Z');

    const selected = pickPreferredDraftTab([draftA, draftB, draftC]);

    expect(selected).toEqual(draftB);
  });

  it('collapses legacy multi-draft state to a single draft and cleans tab navigation ids', () => {
    const draftA = makeDraft('draft-a', '2026-04-12T09:00:00.000Z');
    const draftB = makeDraft('draft-b', '2026-04-12T10:00:00.000Z');
    const draftC = makeDraft('draft-c', '2026-04-12T08:00:00.000Z');

    const collapsed = collapseProjectDraftTabs({
      drafts: [draftA, draftB, draftC],
      activeDraftId: 'missing',
      orderIds: ['session-1', 'draft-a', 'draft-b', 'session-2', 'draft-c'],
      mruIds: ['draft-b', 'draft-c', 'session-2'],
    });

    expect(collapsed.drafts).toEqual([draftB]);
    expect(collapsed.keptDraft).toEqual(draftB);
    expect(collapsed.activeDraftId).toBe('');
    expect(collapsed.removedDraftIds).toEqual(['draft-a', 'draft-c']);
    expect(collapsed.orderIds).toEqual(['session-1', 'draft-b', 'session-2']);
    expect(collapsed.mruIds).toEqual(['draft-b', 'session-2']);
  });

  it('preserves the active draft only when the kept draft was already active', () => {
    const draftA = makeDraft('draft-a', '2026-04-12T09:00:00.000Z');
    const draftB = makeDraft('draft-b', '2026-04-12T10:00:00.000Z');

    const collapsed = collapseProjectDraftTabs({
      drafts: [draftA, draftB],
      activeDraftId: 'draft-a',
      orderIds: ['draft-a', 'session-1', 'draft-b'],
      mruIds: ['draft-a', 'draft-b'],
    });

    expect(collapsed.drafts).toEqual([draftA]);
    expect(collapsed.activeDraftId).toBe('draft-a');
    expect(collapsed.removedDraftIds).toEqual(['draft-b']);
  });
});
