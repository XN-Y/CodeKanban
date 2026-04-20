import { describe, expect, it } from 'vitest';

import {
  buildCodexSkillToken,
  filterCodexSkills,
  insertCodexSkillTokenAtCursor,
  replaceTextSelection,
} from '@/components/web-session/webSessionCodexSkills';
import type { CodexSkillSummary } from '@/types/models';

describe('webSessionCodexSkills', () => {
  const skills: CodexSkillSummary[] = [
    {
      name: 'codekanban-cli',
      displayName: 'CodeKanban CLI',
      description: 'Operate CodeKanban workflows and sessions',
      defaultPrompt: 'Use codekanban-cli',
      source: 'user',
    },
    {
      name: 'openai-docs',
      displayName: 'OpenAI Docs',
      description: 'Reference official OpenAI docs',
      defaultPrompt: 'Look up official docs',
      source: 'system',
    },
    {
      name: 'plugin-creator',
      displayName: 'Plugin Creator',
      description: 'Create and scaffold plugin directories for Codex',
      defaultPrompt: 'Create a plugin',
      source: 'system',
    },
    {
      name: 'skill-installer',
      displayName: 'Skill Installer',
      description: 'Install curated Codex skills',
      defaultPrompt: 'Install a skill',
      source: 'bundled',
    },
  ];

  it('prefers exact and prefix matches when filtering skills', () => {
    expect(filterCodexSkills(skills, 'openai').map(skill => skill.name)).toEqual(['openai-docs']);
    expect(filterCodexSkills(skills, 'skill').map(skill => skill.name)).toEqual([
      'skill-installer',
    ]);
    expect(filterCodexSkills(skills, 'creator').map(skill => skill.name)).toEqual([
      'plugin-creator',
    ]);
    expect(filterCodexSkills(skills, 'code')[0]?.name).toBe('codekanban-cli');
  });

  it('builds a skill token and inserts it with spacing', () => {
    expect(buildCodexSkillToken('openai-docs')).toBe('$openai-docs');

    const inserted = insertCodexSkillTokenAtCursor('Need help with docs', 14, 14, 'openai-docs');
    expect(inserted.text).toBe('Need help with $openai-docs docs');
    expect(inserted.cursor).toBe('Need help with $openai-docs'.length);
  });

  it('replaces the selected text when inserting templates', () => {
    const replaced = replaceTextSelection('before replace after', 7, 14, 'template');
    expect(replaced.text).toBe('before template after');
    expect(replaced.cursor).toBe(15);
  });
});
