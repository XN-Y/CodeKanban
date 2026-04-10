import type { WebSessionUserInputQuestion } from '@/stores/webSession';
import { buildWebSessionSubmitOwnerId } from '@/components/web-session/webSessionSubmitState';

export const WEB_SESSION_USER_INPUT_SLOW_HINT_DELAY_MS = 4000;

type SlowHintTimer = ReturnType<typeof globalThis.setTimeout>;

export interface WebSessionUserInputSlowHintOptions {
  delayMs?: number;
  setTimeoutFn?: (handler: () => void, timeout: number) => SlowHintTimer;
  clearTimeoutFn?: (timer: SlowHintTimer) => void;
}

export function buildWebSessionUserInputSubmitOwnerId(sessionId: string, itemId: string) {
  return buildWebSessionSubmitOwnerId('user_input', sessionId, itemId);
}

export function hasMissingWebSessionUserInputAnswers(
  questions: Array<Pick<WebSessionUserInputQuestion, 'id'>>,
  answers: Record<string, string[]>
) {
  return questions.some(
    question => !Array.isArray(answers[question.id]) || answers[question.id].length === 0
  );
}

export function scheduleWebSessionUserInputSlowHint(
  ownerId: string,
  onSlow: (ownerId: string) => void,
  options: WebSessionUserInputSlowHintOptions = {}
) {
  const normalizedOwnerId = String(ownerId || '').trim();
  if (!normalizedOwnerId) {
    return () => undefined;
  }

  const delayMs = Math.max(
    0,
    Math.trunc(options.delayMs ?? WEB_SESSION_USER_INPUT_SLOW_HINT_DELAY_MS)
  );
  const setTimeoutFn =
    options.setTimeoutFn ??
    ((handler: () => void, timeout: number) => globalThis.setTimeout(handler, timeout));
  const clearTimeoutFn =
    options.clearTimeoutFn ?? ((timer: SlowHintTimer) => globalThis.clearTimeout(timer));

  const timer = setTimeoutFn(() => {
    onSlow(normalizedOwnerId);
  }, delayMs);

  return () => {
    clearTimeoutFn(timer);
  };
}
