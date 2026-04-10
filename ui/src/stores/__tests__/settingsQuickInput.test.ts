import { createPinia, setActivePinia, storeToRefs } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('falls back to default quick input settings when storage is missing', () => {
    const store = useSettingsStore();
    const { webSessionQuickInput } = storeToRefs(store);

    expect(webSessionQuickInput.value).toEqual({
      pinned: ['continue'],
      recent: [],
    });
  });

  it('sanitizes persisted pinned and recent quick input items', () => {
    localStorage.setItem(
      'general_settings',
      JSON.stringify({
        webSessionQuickInput: {
          pinned: ['  Alpha  ', '', 'Beta', 'Alpha'],
          recent: ['  One ', 'Two', 'One', '', 'Three', 'Four', 'Five', 'Six', 'Seven'],
        },
      })
    );

    const store = useSettingsStore();
    const { webSessionQuickInput } = storeToRefs(store);

    expect(webSessionQuickInput.value).toEqual({
      pinned: ['Alpha', 'Beta'],
      recent: ['One', 'Two', 'Three', 'Four', 'Five', 'Six'],
    });
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
});
