import { describe, expect, it } from 'vitest';

import { buildProjectBadgeMap, resolveProjectBadgeLabel } from '@/utils/projectBadge';

describe('projectBadge', () => {
  it('uses the first meaningful Latin character and uppercases it', () => {
    expect(resolveProjectBadgeLabel('alpha project')).toBe('A');
    expect(resolveProjectBadgeLabel('  beta')).toBe('B');
  });

  it('keeps the first meaningful CJK character intact', () => {
    expect(resolveProjectBadgeLabel('项目看板')).toBe('项');
  });

  it('skips leading punctuation when deriving the badge label', () => {
    expect(resolveProjectBadgeLabel('【gamma】')).toBe('G');
  });

  it('falls back when the project name is empty', () => {
    expect(resolveProjectBadgeLabel('')).toBe('?');
    expect(resolveProjectBadgeLabel(undefined, '#')).toBe('#');
  });

  it('builds unique project badges in the provided order', () => {
    const badgeMap = buildProjectBadgeMap(['project-b', 'project-a', 'project-b'], projectId => {
      if (projectId === 'project-a') {
        return 'Alpha';
      }
      if (projectId === 'project-b') {
        return 'Beta';
      }
      return projectId;
    });

    expect(Array.from(badgeMap.entries())).toEqual([
      [
        'project-b',
        {
          label: 'B',
          color: '#10b981',
        },
      ],
      [
        'project-a',
        {
          label: 'A',
          color: '#3b82f6',
        },
      ],
    ]);
  });
});
