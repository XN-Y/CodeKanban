import { describe, expect, it, vi } from 'vitest';

import {
  DAILY_TIP_STATE_STORAGE_KEY,
  formatLocalDateKey,
  getDailyTips,
  loadDailyTipState,
  saveDailyTipState,
  sanitizeDailyTipState,
  selectDailyTipIndex,
  selectAnotherRandomDailyTipIndex,
  selectRandomDailyTipIndex,
  shouldShowDailyTip,
} from '@/utils/dailyTips';

function createStorageMock(initial: Record<string, string> = {}) {
  const store = new Map<string, string>(Object.entries(initial));
  return {
    getItem(key: string) {
      return store.has(key) ? store.get(key)! : null;
    },
    setItem(key: string, value: string) {
      store.set(key, String(value));
    },
  };
}

describe('dailyTips', () => {
  it('formats local dates as yyyy-mm-dd', () => {
    const date = new Date(2026, 3, 14, 9, 30, 0);
    expect(formatLocalDateKey(date)).toBe('2026-04-14');
  });

  it('returns localized tips and falls back to zh-CN', () => {
    expect(getDailyTips('zh-CN')).toHaveLength(3);
    expect(getDailyTips('en-US')).toHaveLength(3);
    expect(getDailyTips('unknown-locale')).toEqual(getDailyTips('zh-CN'));
  });

  it('selects a stable daily tip index from the date key', () => {
    expect(selectDailyTipIndex('2026-04-14', 5)).toBe(selectDailyTipIndex('2026-04-14', 5));
    expect(selectDailyTipIndex('2026-04-14', 5)).not.toBe(selectDailyTipIndex('2026-04-15', 5));
    expect(selectDailyTipIndex('invalid', 5)).toBeGreaterThanOrEqual(0);
    expect(selectDailyTipIndex('2026-04-14', 0)).toBe(-1);
  });

  it('selects a random tip index within range', () => {
    expect(selectRandomDailyTipIndex(0, 5)).toBe(0);
    expect(selectRandomDailyTipIndex(0.2, 5)).toBe(1);
    expect(selectRandomDailyTipIndex(0.999999, 5)).toBe(4);
    expect(selectRandomDailyTipIndex(2, 5)).toBe(4);
    expect(selectRandomDailyTipIndex(-1, 5)).toBe(0);
    expect(selectRandomDailyTipIndex(0.5, 0)).toBe(-1);
  });

  it('selects another random tip index and avoids returning the current one when possible', () => {
    expect(selectAnotherRandomDailyTipIndex(2, 0.5, 5)).toBe(3);
    expect(selectAnotherRandomDailyTipIndex(2, 0.2, 5)).toBe(1);
    expect(selectAnotherRandomDailyTipIndex(0, 0, 1)).toBe(0);
    expect(selectAnotherRandomDailyTipIndex(0, 0.5, 0)).toBe(-1);
  });

  it('sanitizes persisted state and restores defaults for invalid values', () => {
    expect(sanitizeDailyTipState(null)).toEqual({ lastShownDate: null });
    expect(sanitizeDailyTipState({ lastShownDate: '' })).toEqual({ lastShownDate: null });
    expect(sanitizeDailyTipState({ lastShownDate: '2026-04-14' })).toEqual({
      lastShownDate: '2026-04-14',
    });
  });

  it('loads and saves local storage state', () => {
    const storage = createStorageMock();

    expect(loadDailyTipState(storage)).toEqual({ lastShownDate: null });

    saveDailyTipState({ lastShownDate: '2026-04-14' }, storage);

    expect(loadDailyTipState(storage)).toEqual({ lastShownDate: '2026-04-14' });
    expect(storage.getItem(DAILY_TIP_STATE_STORAGE_KEY)).toBe('{"lastShownDate":"2026-04-14"}');
  });

  it('falls back to defaults when stored state is malformed', () => {
    const storage = createStorageMock({
      [DAILY_TIP_STATE_STORAGE_KEY]: '{"lastShownDate":}',
    });
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});

    expect(loadDailyTipState(storage)).toEqual({ lastShownDate: null });

    warn.mockRestore();
  });

  it('only allows the modal on project routes once per local day', () => {
    expect(
      shouldShowDailyTip({
        routeName: 'project',
        projectId: 'project-1',
        enabled: true,
        lastShownDate: '2026-04-13',
        todayDateKey: '2026-04-14',
        tipCount: 5,
      })
    ).toBe(true);

    expect(
      shouldShowDailyTip({
        routeName: 'project',
        projectId: 'project-1',
        enabled: true,
        lastShownDate: '2026-04-14',
        todayDateKey: '2026-04-14',
        tipCount: 5,
      })
    ).toBe(false);

    expect(
      shouldShowDailyTip({
        routeName: 'settings',
        projectId: 'project-1',
        enabled: true,
        lastShownDate: null,
        todayDateKey: '2026-04-14',
        tipCount: 5,
      })
    ).toBe(false);

    expect(
      shouldShowDailyTip({
        routeName: 'project',
        projectId: 'project-1',
        enabled: false,
        lastShownDate: null,
        todayDateKey: '2026-04-14',
        tipCount: 5,
      })
    ).toBe(false);
  });
});
