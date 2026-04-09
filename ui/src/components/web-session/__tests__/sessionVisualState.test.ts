import { describe, expect, it } from 'vitest';

import {
  getWebSessionPillTone,
  getWebSessionSidebarTone,
  getWebSessionTabTone,
} from '@/components/web-session/sessionVisualState';

function makeInput(
  overrides: Partial<{
    phase:
      | 'idle'
      | 'starting'
      | 'thinking'
      | 'tool'
      | 'retrying'
      | 'waiting_approval'
      | 'waiting_input'
      | 'waiting_plan_approval'
      | 'done'
      | 'error';
    hasUnread: boolean;
    status: 'idle' | 'running' | 'waiting_approval' | 'done' | 'err' | 'aborting';
  }> = {}
) {
  return {
    phase: 'idle' as const,
    hasUnread: false,
    status: 'running' as const,
    ...overrides,
  };
}

describe('sessionVisualState', () => {
  it('treats waiting_input as approval tone everywhere', () => {
    const input = makeInput({ phase: 'waiting_input' });

    expect(getWebSessionPillTone(input)).toBe('approval');
    expect(getWebSessionTabTone(input)).toBe('approval');
    expect(getWebSessionSidebarTone(input)).toBe('approval');
  });

  it('maps waiting_plan_approval to the plan approval tone', () => {
    const input = makeInput({ phase: 'waiting_plan_approval' });

    expect(getWebSessionPillTone(input)).toBe('plan_approval');
    expect(getWebSessionTabTone(input)).toBe('plan_approval');
    expect(getWebSessionSidebarTone(input)).toBe('plan_approval');
  });

  it('treats waiting_approval as approval tone', () => {
    const input = makeInput({ phase: 'waiting_approval' });

    expect(getWebSessionPillTone(input)).toBe('approval');
    expect(getWebSessionTabTone(input)).toBe('approval');
    expect(getWebSessionSidebarTone(input)).toBe('approval');
  });

  it('keeps unread completed sessions green', () => {
    const input = makeInput({ phase: 'done', hasUnread: true, status: 'done' });

    expect(getWebSessionPillTone(input)).toBe('completion');
    expect(getWebSessionTabTone(input)).toBe('completion');
    expect(getWebSessionSidebarTone(input)).toBe('completion');
  });

  it('treats retrying sessions as working', () => {
    const input = makeInput({ phase: 'retrying' });

    expect(getWebSessionPillTone(input)).toBe('working');
    expect(getWebSessionTabTone(input)).toBe('default');
    expect(getWebSessionSidebarTone(input)).toBe('working');
  });

  it('keeps plain idle sessions neutral', () => {
    const input = makeInput({ phase: 'idle', hasUnread: false, status: 'idle' });

    expect(getWebSessionPillTone(input)).toBe('unknown');
    expect(getWebSessionTabTone(input)).toBe('default');
    expect(getWebSessionSidebarTone(input)).toBe('idle');
  });
});
