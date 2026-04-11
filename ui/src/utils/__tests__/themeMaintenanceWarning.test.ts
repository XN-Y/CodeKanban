import { describe, expect, it, vi } from 'vitest';

import {
  createThemeMaintenanceWarningController,
  createThemeSelectionController,
  type ThemeMaintenanceDialogOptions,
} from '@/utils/themeMaintenanceWarning';

function translate(key: string) {
  const messages: Record<string, string> = {
    'common.confirm': 'Confirm',
    'common.cancel': 'Cancel',
    'theme.maintenanceWarningTitle': 'Theme Warning',
    'theme.maintenanceWarningPreset': 'Preset warning',
    'theme.maintenanceWarningFollowSystem': 'Follow warning',
  };
  return messages[key] ?? key;
}

describe('theme maintenance warning controller', () => {
  it('opens the preset warning dialog and resolves true when confirmed', async () => {
    let options: ThemeMaintenanceDialogOptions | null = null;
    const controller = createThemeMaintenanceWarningController({
      t: translate,
      warning: nextOptions => {
        options = nextOptions;
      },
    });

    const resultPromise = controller.confirmPresetThemeChange();

    expect(options?.title).toBe('Theme Warning');
    expect(options?.content).toBe('Preset warning');
    expect(options?.positiveText).toBe('Confirm');
    expect(options?.negativeText).toBe('Cancel');
    expect(options?.maskClosable).toBe(false);
    expect(options?.closeOnEsc).toBe(true);

    options?.onPositiveClick?.();

    await expect(resultPromise).resolves.toBe(true);
  });

  it('opens the follow-system warning dialog and resolves false when closed', async () => {
    let options: ThemeMaintenanceDialogOptions | null = null;
    const controller = createThemeMaintenanceWarningController({
      t: translate,
      warning: nextOptions => {
        options = nextOptions;
      },
    });

    const resultPromise = controller.confirmFollowSystemEnable();

    expect(options?.title).toBe('Theme Warning');
    expect(options?.content).toBe('Follow warning');

    options?.onClose?.();

    await expect(resultPromise).resolves.toBe(false);
  });
});

describe('theme selection controller', () => {
  it('applies preset selection after confirmation', async () => {
    const confirmPresetThemeChange = vi.fn().mockResolvedValue(true);
    const selectPreset = vi.fn();
    const controller = createThemeSelectionController({
      getCurrentPresetId: () => 'light',
      isFollowSystemTheme: () => false,
      selectPreset,
      toggleFollowSystemTheme: vi.fn(),
      confirmPresetThemeChange,
      confirmFollowSystemEnable: vi.fn(),
    });

    await expect(controller.selectPresetWithConfirmation('dark')).resolves.toBe(true);

    expect(confirmPresetThemeChange).toHaveBeenCalledTimes(1);
    expect(selectPreset).toHaveBeenCalledWith('dark');
  });

  it('skips preset confirmation when selecting the active preset outside follow-system mode', async () => {
    const confirmPresetThemeChange = vi.fn();
    const selectPreset = vi.fn();
    const controller = createThemeSelectionController({
      getCurrentPresetId: () => 'light',
      isFollowSystemTheme: () => false,
      selectPreset,
      toggleFollowSystemTheme: vi.fn(),
      confirmPresetThemeChange,
      confirmFollowSystemEnable: vi.fn(),
    });

    await expect(controller.selectPresetWithConfirmation('light')).resolves.toBe(false);

    expect(confirmPresetThemeChange).not.toHaveBeenCalled();
    expect(selectPreset).not.toHaveBeenCalled();
  });

  it('does not apply preset selection when confirmation is canceled', async () => {
    const confirmPresetThemeChange = vi.fn().mockResolvedValue(false);
    const selectPreset = vi.fn();
    const controller = createThemeSelectionController({
      getCurrentPresetId: () => 'light',
      isFollowSystemTheme: () => false,
      selectPreset,
      toggleFollowSystemTheme: vi.fn(),
      confirmPresetThemeChange,
      confirmFollowSystemEnable: vi.fn(),
    });

    await expect(controller.selectPresetWithConfirmation('dark')).resolves.toBe(false);

    expect(confirmPresetThemeChange).toHaveBeenCalledTimes(1);
    expect(selectPreset).not.toHaveBeenCalled();
  });

  it('switches back to the default light preset without showing a warning', async () => {
    const confirmPresetThemeChange = vi.fn();
    const selectPreset = vi.fn();
    const controller = createThemeSelectionController({
      getCurrentPresetId: () => 'dark',
      isFollowSystemTheme: () => false,
      selectPreset,
      toggleFollowSystemTheme: vi.fn(),
      confirmPresetThemeChange,
      confirmFollowSystemEnable: vi.fn(),
    });

    await expect(controller.selectPresetWithConfirmation('light')).resolves.toBe(true);

    expect(confirmPresetThemeChange).not.toHaveBeenCalled();
    expect(selectPreset).toHaveBeenCalledWith('light');
  });

  it('enables follow-system mode only after confirmation', async () => {
    const confirmFollowSystemEnable = vi.fn().mockResolvedValue(true);
    const toggleFollowSystemTheme = vi.fn();
    const controller = createThemeSelectionController({
      getCurrentPresetId: () => 'light',
      isFollowSystemTheme: () => false,
      selectPreset: vi.fn(),
      toggleFollowSystemTheme,
      confirmPresetThemeChange: vi.fn(),
      confirmFollowSystemEnable,
    });

    await expect(controller.toggleFollowSystemThemeWithConfirmation(true)).resolves.toBe(true);

    expect(confirmFollowSystemEnable).toHaveBeenCalledTimes(1);
    expect(toggleFollowSystemTheme).toHaveBeenCalledWith(true);
  });

  it('disables follow-system mode without showing a warning', async () => {
    const confirmFollowSystemEnable = vi.fn();
    const toggleFollowSystemTheme = vi.fn();
    const controller = createThemeSelectionController({
      getCurrentPresetId: () => 'dark',
      isFollowSystemTheme: () => true,
      selectPreset: vi.fn(),
      toggleFollowSystemTheme,
      confirmPresetThemeChange: vi.fn(),
      confirmFollowSystemEnable,
    });

    await expect(controller.toggleFollowSystemThemeWithConfirmation(false)).resolves.toBe(true);

    expect(confirmFollowSystemEnable).not.toHaveBeenCalled();
    expect(toggleFollowSystemTheme).toHaveBeenCalledWith(false);
  });

  it('does nothing when quick-toggle is used outside light/dark presets', async () => {
    const confirmPresetThemeChange = vi.fn();
    const selectPreset = vi.fn();
    const controller = createThemeSelectionController({
      getCurrentPresetId: () => 'dim',
      isFollowSystemTheme: () => false,
      selectPreset,
      toggleFollowSystemTheme: vi.fn(),
      confirmPresetThemeChange,
      confirmFollowSystemEnable: vi.fn(),
    });

    await expect(controller.quickToggleLightDark()).resolves.toBe(false);

    expect(confirmPresetThemeChange).not.toHaveBeenCalled();
    expect(selectPreset).not.toHaveBeenCalled();
  });
});
