export type WebSessionSubmitState = Record<string, true>;

function normalizeSubmitOwnerId(ownerId: string) {
  return String(ownerId || '').trim();
}

export function buildWebSessionSubmitOwnerId(...ownerIdParts: string[]) {
  return ownerIdParts.map(normalizeSubmitOwnerId).filter(Boolean).join('::');
}

export function beginWebSessionSubmit(
  state: WebSessionSubmitState,
  ownerId: string
): WebSessionSubmitState {
  const normalizedOwnerId = normalizeSubmitOwnerId(ownerId);
  if (!normalizedOwnerId || state[normalizedOwnerId]) {
    return state;
  }
  return {
    ...state,
    [normalizedOwnerId]: true,
  };
}

export function endWebSessionSubmit(
  state: WebSessionSubmitState,
  ownerId: string
): WebSessionSubmitState {
  const normalizedOwnerId = normalizeSubmitOwnerId(ownerId);
  if (!normalizedOwnerId || !state[normalizedOwnerId]) {
    return state;
  }
  const nextState = { ...state };
  delete nextState[normalizedOwnerId];
  return nextState;
}

export function isWebSessionSubmitting(state: WebSessionSubmitState, ownerId: string) {
  const normalizedOwnerId = normalizeSubmitOwnerId(ownerId);
  return Boolean(normalizedOwnerId && state[normalizedOwnerId]);
}

export function transferWebSessionSubmit(
  state: WebSessionSubmitState,
  fromOwnerId: string,
  toOwnerId: string
): WebSessionSubmitState {
  const normalizedFromOwnerId = normalizeSubmitOwnerId(fromOwnerId);
  const normalizedToOwnerId = normalizeSubmitOwnerId(toOwnerId);
  if (
    !normalizedFromOwnerId ||
    normalizedFromOwnerId === normalizedToOwnerId ||
    !state[normalizedFromOwnerId]
  ) {
    return state;
  }

  const nextState = { ...state };
  delete nextState[normalizedFromOwnerId];
  if (normalizedToOwnerId) {
    nextState[normalizedToOwnerId] = true;
  }
  return nextState;
}
