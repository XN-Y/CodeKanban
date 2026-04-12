export type DraftTabLike = {
  id: string;
  updatedAt?: string | number | null;
};

export type CollapseProjectDraftTabsInput<T extends DraftTabLike> = {
  drafts: T[];
  activeDraftId?: string;
  orderIds?: string[];
  mruIds?: string[];
};

export type CollapseProjectDraftTabsResult<T extends DraftTabLike> = {
  drafts: T[];
  keptDraft: T | null;
  removedDraftIds: string[];
  activeDraftId: string;
  orderIds: string[];
  mruIds: string[];
};

function normalizeId(value: string | null | undefined) {
  return String(value || '').trim();
}

function normalizeIdList(values: string[] | undefined) {
  if (!Array.isArray(values) || values.length === 0) {
    return [];
  }

  const next: string[] = [];
  values.forEach(value => {
    const normalized = normalizeId(value);
    if (!normalized || next.includes(normalized)) {
      return;
    }
    next.push(normalized);
  });
  return next;
}

function normalizeDraftTabs<T extends DraftTabLike>(drafts: T[]) {
  if (!Array.isArray(drafts) || drafts.length === 0) {
    return [];
  }

  const next: T[] = [];
  const seen = new Set<string>();
  drafts.forEach(draft => {
    const normalizedId = normalizeId(draft.id);
    if (!normalizedId || seen.has(normalizedId)) {
      return;
    }
    seen.add(normalizedId);
    next.push({
      ...draft,
      id: normalizedId,
    });
  });
  return next;
}

function parseDraftUpdatedAt(value: DraftTabLike['updatedAt']) {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Date.parse(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return 0;
}

export function pickPreferredDraftTab<T extends DraftTabLike>(
  drafts: T[],
  options?: {
    activeDraftId?: string;
    mruIds?: string[];
  }
) {
  const normalizedDrafts = normalizeDraftTabs(drafts);
  if (normalizedDrafts.length === 0) {
    return null;
  }

  const draftById = new Map(normalizedDrafts.map(draft => [draft.id, draft]));
  const preferredActiveId = normalizeId(options?.activeDraftId);
  if (preferredActiveId) {
    const activeDraft = draftById.get(preferredActiveId);
    if (activeDraft) {
      return activeDraft;
    }
  }

  for (const draftId of normalizeIdList(options?.mruIds)) {
    const draft = draftById.get(draftId);
    if (draft) {
      return draft;
    }
  }

  let latestDraft: T | null = null;
  let latestUpdatedAt = 0;
  normalizedDrafts.forEach(draft => {
    const updatedAt = parseDraftUpdatedAt(draft.updatedAt);
    if (!latestDraft || updatedAt > latestUpdatedAt) {
      latestDraft = draft;
      latestUpdatedAt = updatedAt;
    }
  });
  if (latestDraft && latestUpdatedAt > 0) {
    return latestDraft;
  }

  return normalizedDrafts[0] ?? null;
}

export function collapseProjectDraftTabs<T extends DraftTabLike>(
  input: CollapseProjectDraftTabsInput<T>
): CollapseProjectDraftTabsResult<T> {
  const normalizedDrafts = normalizeDraftTabs(input.drafts);
  const keptDraft = pickPreferredDraftTab(normalizedDrafts, {
    activeDraftId: input.activeDraftId,
    mruIds: input.mruIds,
  });
  const keptDraftId = keptDraft?.id ?? '';
  const preferredActiveDraftId = normalizeId(input.activeDraftId);
  const removedDraftIds = normalizedDrafts
    .map(draft => draft.id)
    .filter(draftId => draftId !== keptDraftId);
  const removedDraftIdSet = new Set(removedDraftIds);

  return {
    drafts: keptDraft ? [keptDraft] : [],
    keptDraft,
    removedDraftIds,
    activeDraftId: preferredActiveDraftId === keptDraftId ? keptDraftId : '',
    orderIds: normalizeIdList(input.orderIds).filter(id => !removedDraftIdSet.has(id)),
    mruIds: normalizeIdList(input.mruIds).filter(id => !removedDraftIdSet.has(id)),
  };
}
