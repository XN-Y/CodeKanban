import { describe, expect, it } from 'vitest';

import {
  CLAUDE_MODEL_OPTIONS,
  CODEX_MODEL_OPTIONS,
  defaultModelForAgent,
} from '@/components/web-session/webSessionModelOptions';

describe('webSessionModelOptions', () => {
  it('includes the new codex 5.5 models', () => {
    const values = CODEX_MODEL_OPTIONS.map(option => option.value);

    expect(values).toContain('gpt-5.5');
    expect(values).toContain('gpt-5.5-pro');
  });

  it('keeps the existing built-in models available', () => {
    expect(CODEX_MODEL_OPTIONS.map(option => option.value)).toEqual(
      expect.arrayContaining([
        'gpt-5.3-codex',
        'gpt-5.3-codex-spark',
        'gpt-5.4',
        'gpt-5.4-mini',
        'gpt-5.4-nano',
        'gpt-5.4-pro',
      ])
    );
    expect(CLAUDE_MODEL_OPTIONS.map(option => option.value)).toEqual(['opus', 'sonnet', 'haiku']);
  });

  it('uses the latest codex model by default', () => {
    expect(defaultModelForAgent('codex')).toBe('gpt-5.5');
    expect(defaultModelForAgent('claude')).toBe('opus');
  });
});
