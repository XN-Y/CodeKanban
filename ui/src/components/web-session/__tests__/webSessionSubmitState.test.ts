import { describe, expect, it } from 'vitest';

import {
  buildWebSessionSubmitOwnerId,
  beginWebSessionSubmit,
  endWebSessionSubmit,
  isWebSessionSubmitting,
  transferWebSessionSubmit,
} from '@/components/web-session/webSessionSubmitState';

describe('webSessionSubmitState', () => {
  it('tracks submit state per session', () => {
    const state = beginWebSessionSubmit({}, 'session-a');

    expect(isWebSessionSubmitting(state, 'session-a')).toBe(true);
    expect(isWebSessionSubmitting(state, 'session-b')).toBe(false);
  });

  it('transfers submit ownership from draft to created session', () => {
    const initial = beginWebSessionSubmit({}, 'draft-session');
    const transferred = transferWebSessionSubmit(initial, 'draft-session', 'session-1');

    expect(isWebSessionSubmitting(transferred, 'draft-session')).toBe(false);
    expect(isWebSessionSubmitting(transferred, 'session-1')).toBe(true);
  });

  it('clears only the final owner when finishing', () => {
    const initial = beginWebSessionSubmit({}, 'draft-session');
    const transferred = transferWebSessionSubmit(initial, 'draft-session', 'session-1');
    const finished = endWebSessionSubmit(transferred, 'session-1');

    expect(isWebSessionSubmitting(finished, 'draft-session')).toBe(false);
    expect(isWebSessionSubmitting(finished, 'session-1')).toBe(false);
  });

  it('ignores duplicate finish and invalid transfers', () => {
    const initial = beginWebSessionSubmit({}, 'session-a');
    const duplicateFinish = endWebSessionSubmit(initial, 'session-b');
    const invalidTransfer = transferWebSessionSubmit(initial, 'session-b', 'session-c');
    const blankTargetTransfer = transferWebSessionSubmit(initial, 'session-a', '');

    expect(duplicateFinish).toEqual(initial);
    expect(invalidTransfer).toEqual(initial);
    expect(isWebSessionSubmitting(blankTargetTransfer, 'session-a')).toBe(false);
    expect(isWebSessionSubmitting(blankTargetTransfer, 'session-c')).toBe(false);
  });

  it('builds stable composite owner ids', () => {
    expect(buildWebSessionSubmitOwnerId(' user_input ', 'session-1', ' item-2 ')).toBe(
      'user_input::session-1::item-2'
    );
    expect(buildWebSessionSubmitOwnerId(' ', 'session-1', '')).toBe('session-1');
  });
});
