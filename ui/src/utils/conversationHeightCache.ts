const STORAGE_KEY = 'codekanban-conversation-height-cache-v1';
const STORAGE_VERSION = 1;
const MAX_SESSION_RECORDS = 12;
const MAX_SIGNATURE_RECORDS = 240;
const MIN_HEIGHT = 72;
const MAX_HEIGHT = 4000;

type SignatureRecord = {
  avg: number;
  count: number;
  updatedAt: number;
};

type SessionRecord = {
  updatedAt: number;
  heights: Record<string, number>;
};

type ConversationHeightStorage = {
  version: number;
  sessions: Record<string, SessionRecord>;
  signatures: Record<string, SignatureRecord>;
};

export interface ConversationHeightInput {
  sessionId?: string | null;
  sourceIndex: number;
  variant: string;
  role: 'user' | 'assistant';
  kind?: string;
  content: string;
  imageCount?: number;
  hasToolControls?: boolean;
}

export interface ConversationMeasuredHeightInput extends ConversationHeightInput {
  height: number;
}

let storageState: ConversationHeightStorage | null = null;
let persistTimer: ReturnType<typeof setTimeout> | null = null;

function createDefaultState(): ConversationHeightStorage {
  return {
    version: STORAGE_VERSION,
    sessions: {},
    signatures: {},
  };
}

function clampHeight(value: number) {
  return Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, Math.round(value)));
}

function getStorageState() {
  if (storageState) {
    return storageState;
  }
  if (typeof window === 'undefined' || !window.localStorage) {
    storageState = createDefaultState();
    return storageState;
  }

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      storageState = createDefaultState();
      return storageState;
    }

    const parsed = JSON.parse(raw) as Partial<ConversationHeightStorage>;
    if (parsed.version !== STORAGE_VERSION) {
      storageState = createDefaultState();
      return storageState;
    }

    const nextState = createDefaultState();
    const now = Date.now();

    if (parsed.sessions && typeof parsed.sessions === 'object') {
      Object.entries(parsed.sessions).forEach(([sessionId, record]) => {
        if (!sessionId || !record || typeof record !== 'object') {
          return;
        }
        const candidate = record as SessionRecord;
        const heights = Object.fromEntries(
          Object.entries(candidate.heights ?? {}).filter(([, value]) => {
            return typeof value === 'number' && Number.isFinite(value) && value >= MIN_HEIGHT;
          })
        );
        if (!Object.keys(heights).length) {
          return;
        }
        nextState.sessions[sessionId] = {
          updatedAt:
            typeof candidate.updatedAt === 'number' && Number.isFinite(candidate.updatedAt)
              ? candidate.updatedAt
              : now,
          heights,
        };
      });
    }

    if (parsed.signatures && typeof parsed.signatures === 'object') {
      Object.entries(parsed.signatures).forEach(([signature, record]) => {
        if (!signature || !record || typeof record !== 'object') {
          return;
        }
        const candidate = record as SignatureRecord;
        if (
          typeof candidate.avg !== 'number' ||
          !Number.isFinite(candidate.avg) ||
          typeof candidate.count !== 'number' ||
          !Number.isFinite(candidate.count) ||
          candidate.count <= 0
        ) {
          return;
        }
        nextState.signatures[signature] = {
          avg: clampHeight(candidate.avg),
          count: Math.max(1, Math.round(candidate.count)),
          updatedAt:
            typeof candidate.updatedAt === 'number' && Number.isFinite(candidate.updatedAt)
              ? candidate.updatedAt
              : now,
        };
      });
    }

    pruneState(nextState);
    storageState = nextState;
    return storageState;
  } catch (error) {
    console.warn('[Conversation Height Cache] Failed to load cache', error);
    storageState = createDefaultState();
    return storageState;
  }
}

function schedulePersist() {
  if (typeof window === 'undefined' || !window.localStorage) {
    return;
  }
  if (persistTimer) {
    clearTimeout(persistTimer);
  }
  persistTimer = setTimeout(() => {
    persistTimer = null;
    try {
      const state = getStorageState();
      pruneState(state);
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch (error) {
      console.warn('[Conversation Height Cache] Failed to persist cache', error);
    }
  }, 250);
}

function pruneState(state: ConversationHeightStorage) {
  const sessionEntries = Object.entries(state.sessions)
    .sort((left, right) => right[1].updatedAt - left[1].updatedAt)
    .slice(0, MAX_SESSION_RECORDS);
  state.sessions = Object.fromEntries(sessionEntries);

  const signatureEntries = Object.entries(state.signatures)
    .sort((left, right) => right[1].updatedAt - left[1].updatedAt)
    .slice(0, MAX_SIGNATURE_RECORDS);
  state.signatures = Object.fromEntries(signatureEntries);
}

export function buildConversationHeightKey(sourceIndex: number, variant: string) {
  return `${sourceIndex}:${variant}`;
}

function normalizeContent(content: string) {
  return content.replace(/\r/g, '');
}

function countWrappedLines(content: string, variant: string) {
  const normalized = normalizeContent(content);
  if (!normalized) {
    return 1;
  }
  const lines = normalized.split('\n');
  const charsPerLine = variant === 'raw' ? 82 : 68;
  let total = 0;
  for (const line of lines) {
    total += Math.max(1, Math.ceil(Math.max(1, line.length) / charsPerLine));
  }
  return Math.max(1, total);
}

function buildSignature(input: ConversationHeightInput) {
  const normalized = normalizeContent(input.content);
  const wrappedLineCount = countWrappedLines(normalized, input.variant);
  const charBucket = Math.min(18, Math.ceil(normalized.length / 200));
  const lineBucket = Math.min(16, Math.ceil(wrappedLineCount / 4));
  const imageBucket = Math.min(4, input.imageCount ?? 0);
  const codeBucket = normalized.includes('```') ? 'code' : 'plain';
  const toolBucket = input.hasToolControls ? 'tool' : 'none';

  return [
    input.role,
    input.kind || 'default',
    input.variant,
    `c${charBucket}`,
    `l${lineBucket}`,
    `i${imageBucket}`,
    codeBucket,
    toolBucket,
  ].join('|');
}

function getFallbackHeight(input: ConversationHeightInput) {
  const normalized = normalizeContent(input.content);
  const lineCount = countWrappedLines(normalized, input.variant);
  const baseHeight = input.variant === 'raw' ? 92 : 84;
  const lineHeight = input.variant === 'raw' ? 21 : 20;
  const codeBonus = normalized.includes('```') ? 24 : 0;
  const imageCount = input.imageCount ?? 0;
  const attachmentHeight = imageCount > 0 ? 36 + Math.ceil(imageCount / 4) * 30 : 0;
  const toolControlHeight = input.hasToolControls ? 30 : 0;
  const lengthBonus = normalized.length > 2400 ? Math.ceil(normalized.length / 1600) * 14 : 0;

  return clampHeight(
    baseHeight +
      lineCount * lineHeight +
      codeBonus +
      attachmentHeight +
      toolControlHeight +
      lengthBonus
  );
}

function getExactHeight(input: ConversationHeightInput) {
  if (!input.sessionId) {
    return null;
  }
  const session = getStorageState().sessions[input.sessionId];
  if (!session) {
    return null;
  }
  const key = buildConversationHeightKey(input.sourceIndex, input.variant);
  const value = session.heights[key];
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

export function estimateConversationMessageHeight(input: ConversationHeightInput) {
  const exactHeight = getExactHeight(input);
  if (exactHeight !== null) {
    return exactHeight;
  }

  const signature = buildSignature(input);
  const signatureRecord = getStorageState().signatures[signature];
  if (signatureRecord) {
    return clampHeight(signatureRecord.avg);
  }

  return getFallbackHeight(input);
}

export function recordConversationMessageHeight(input: ConversationMeasuredHeightInput) {
  if (!Number.isFinite(input.height) || input.height < MIN_HEIGHT) {
    return;
  }

  const state = getStorageState();
  const normalizedHeight = clampHeight(input.height);
  const now = Date.now();

  if (input.sessionId) {
    const sessionId = input.sessionId;
    const session = state.sessions[sessionId] ?? {
      updatedAt: now,
      heights: {},
    };
    session.updatedAt = now;
    session.heights[buildConversationHeightKey(input.sourceIndex, input.variant)] =
      normalizedHeight;
    state.sessions[sessionId] = session;
  }

  const signature = buildSignature(input);
  const previous = state.signatures[signature];
  if (!previous) {
    state.signatures[signature] = {
      avg: normalizedHeight,
      count: 1,
      updatedAt: now,
    };
  } else {
    const weight = Math.min(previous.count, 7);
    previous.avg = clampHeight((previous.avg * weight + normalizedHeight) / (weight + 1));
    previous.count += 1;
    previous.updatedAt = now;
  }

  schedulePersist();
}
