import { createPinia, setActivePinia, storeToRefs } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { getMethodMock, getSendMock, postMethodMock, postSendMock } = vi.hoisted(() => {
  const getSendMock = vi.fn();
  const postSendMock = vi.fn();
  return {
    getMethodMock: vi.fn(() => ({ send: getSendMock })),
    getSendMock,
    postMethodMock: vi.fn(() => ({ send: postSendMock })),
    postSendMock,
  };
});

vi.mock('@/api/http', () => ({
  http: {
    Get: getMethodMock,
    Post: postMethodMock,
  },
}));

import { useSettingsStore } from '@/stores/settings';

function createStorageMock() {
  const store = new Map<string, string>();
  return {
    getItem(key: string) {
      return store.has(key) ? store.get(key)! : null;
    },
    setItem(key: string, value: string) {
      store.set(key, String(value));
    },
    removeItem(key: string) {
      store.delete(key);
    },
    clear() {
      store.clear();
    },
  };
}

describe('settings web session quick input', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.stubGlobal('localStorage', createStorageMock());
    getMethodMock.mockClear();
    getSendMock.mockReset();
    postMethodMock.mockClear();
    postSendMock.mockReset();
    getSendMock.mockResolvedValue({
      item: {
        pinned: ['continue'],
        recent: [],
      },
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('falls back to default quick input settings when storage is missing', () => {
    const store = useSettingsStore();
    const { webSessionQuickInput, webSessionQuickInputDirectSend } = storeToRefs(store);

    expect(webSessionQuickInput.value).toEqual({
      pinned: ['continue'],
      recent: [],
    });
    expect(webSessionQuickInputDirectSend.value).toBe(false);
  });

  it('sanitizes persisted pinned, recent, and direct-send quick input settings', () => {
    localStorage.setItem(
      'general_settings',
      JSON.stringify({
        webSessionQuickInput: {
          pinned: ['  Alpha  ', '', 'Beta', 'Alpha'],
          recent: ['  One ', 'Two', 'One', '', 'Three', 'Four', 'Five', 'Six', 'Seven'],
        },
        webSessionQuickInputDirectSend: true,
      })
    );

    const store = useSettingsStore();
    const { webSessionQuickInput, webSessionQuickInputDirectSend } = storeToRefs(store);

    expect(webSessionQuickInput.value).toEqual({
      pinned: ['Alpha', 'Beta'],
      recent: ['One', 'Two', 'Three', 'Four', 'Five', 'Six'],
    });
    expect(webSessionQuickInputDirectSend.value).toBe(true);
  });

  it('sanitizes pinned quick input updates', () => {
    const store = useSettingsStore();
    const { webSessionQuickInput } = storeToRefs(store);

    store.updateWebSessionQuickInputPinned(['  Build plan  ', '', 'Build plan', 'Ship it']);

    expect(webSessionQuickInput.value.pinned).toEqual(['Build plan', 'Ship it']);
  });

  it('deduplicates recent items and keeps only the latest six entries', () => {
    const store = useSettingsStore();
    const { webSessionQuickInput } = storeToRefs(store);

    for (let index = 1; index <= 8; index += 1) {
      store.recordWebSessionRecentInput(`item ${index}`);
    }
    store.recordWebSessionRecentInput('   ');
    store.recordWebSessionRecentInput(' item 6 ');

    expect(webSessionQuickInput.value.recent).toEqual([
      'item 6',
      'item 8',
      'item 7',
      'item 5',
      'item 4',
      'item 3',
    ]);
  });

  it('updates quick input direct-send setting', () => {
    const store = useSettingsStore();
    const { webSessionQuickInputDirectSend } = storeToRefs(store);

    store.updateWebSessionQuickInputDirectSend(true);
    expect(webSessionQuickInputDirectSend.value).toBe(true);

    store.updateWebSessionQuickInputDirectSend(false);
    expect(webSessionQuickInputDirectSend.value).toBe(false);
  });

  it('saves pinned quick input with sanitized items while preserving recent history', async () => {
    const store = useSettingsStore();
    const { webSessionQuickInput } = storeToRefs(store);

    store.recordWebSessionRecentInput('item 1');
    store.recordWebSessionRecentInput('item 2');
    postSendMock.mockResolvedValue({
      item: {
        pinned: ['Build plan', 'Ship it'],
        recent: ['item 2', 'item 1'],
      },
    });

    const saved = await store.saveWebSessionQuickInputPinned([
      '  Build plan  ',
      '',
      'Build plan',
      'Ship it',
    ]);

    expect(postMethodMock).toHaveBeenCalledWith('/system/web-session-quick-input/update', {
      pinned: ['Build plan', 'Ship it'],
      recent: ['item 2', 'item 1'],
    });
    expect(saved).toEqual({
      pinned: ['Build plan', 'Ship it'],
      recent: ['item 2', 'item 1'],
    });
    expect(webSessionQuickInput.value).toEqual({
      pinned: ['Build plan', 'Ship it'],
      recent: ['item 2', 'item 1'],
    });
  });

  it('keeps saved pinned quick input unchanged when manual save fails', async () => {
    const store = useSettingsStore();
    const { webSessionQuickInput } = storeToRefs(store);

    postSendMock.mockRejectedValue(new Error('save failed'));

    await expect(store.saveWebSessionQuickInputPinned(['Draft next step'])).rejects.toThrow(
      'save failed'
    );

    expect(webSessionQuickInput.value).toEqual({
      pinned: ['continue'],
      recent: [],
    });
  });
});
