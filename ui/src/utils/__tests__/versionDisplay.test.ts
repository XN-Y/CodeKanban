import { describe, expect, it } from 'vitest';

import { formatVersionForDisplay } from '@/utils/versionDisplay';

describe('formatVersionForDisplay', () => {
  it('strips build metadata for stable semver versions', () => {
    expect(formatVersionForDisplay('0.31.0+20260412')).toBe('0.31.0');
    expect(formatVersionForDisplay('v0.31.0+20260412')).toBe('v0.31.0');
  });

  it('preserves prerelease versions even when they include build metadata', () => {
    expect(formatVersionForDisplay('0.31.0-alpha+20260412')).toBe('0.31.0-alpha+20260412');
    expect(formatVersionForDisplay('0.31.0-beta.1')).toBe('0.31.0-beta.1');
  });

  it('leaves non-semver strings unchanged', () => {
    expect(formatVersionForDisplay('nightly')).toBe('nightly');
    expect(formatVersionForDisplay('')).toBe('');
  });
});
