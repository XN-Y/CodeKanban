import { describe, expect, it, vi } from 'vitest';

import {
  buildWebSessionUserInputSubmitOwnerId,
  hasMissingWebSessionUserInputAnswers,
  scheduleWebSessionUserInputSlowHint,
} from '@/components/web-session/webSessionUserInputSubmit';

describe('webSessionUserInputSubmit', () => {
  it('builds a request-scoped submit owner id', () => {
    expect(buildWebSessionUserInputSubmitOwnerId(' session-1 ', ' item-7 ')).toBe(
      'user_input::session-1::item-7'
    );
  });

  it('detects when any requested answer is missing', () => {
    const questions = [{ id: 'scope' }, { id: 'target' }];

    expect(hasMissingWebSessionUserInputAnswers(questions, { scope: ['full'] })).toBe(true);
    expect(
      hasMissingWebSessionUserInputAnswers(questions, {
        scope: ['full'],
        target: ['api'],
      })
    ).toBe(false);
  });

  it('shows the slow hint after the delay', () => {
    vi.useFakeTimers();
    const onSlow = vi.fn();

    scheduleWebSessionUserInputSlowHint('user_input::session-1::item-1', onSlow, {
      delayMs: 4000,
    });

    vi.advanceTimersByTime(3999);
    expect(onSlow).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1);
    expect(onSlow).toHaveBeenCalledWith('user_input::session-1::item-1');

    vi.useRealTimers();
  });

  it('cancels the slow hint when the request completes early', () => {
    vi.useFakeTimers();
    const onSlow = vi.fn();

    const cancel = scheduleWebSessionUserInputSlowHint('user_input::session-1::item-1', onSlow, {
      delayMs: 4000,
    });
    cancel();

    vi.advanceTimersByTime(4000);
    expect(onSlow).not.toHaveBeenCalled();

    vi.useRealTimers();
  });
});
