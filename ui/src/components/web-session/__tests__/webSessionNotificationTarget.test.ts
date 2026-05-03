import { describe, expect, it, vi } from 'vitest';

import { openWebSessionNotificationTarget } from '@/components/web-session/webSessionNotificationTarget';

describe('webSessionNotificationTarget', () => {
  it('opens a web-session deep link without requiring eager session activation', async () => {
    const addRecentProject = vi.fn();
    const push = vi.fn().mockResolvedValue(undefined);

    await expect(
      openWebSessionNotificationTarget({
        event: {
          projectId: ' project-1 ',
          sessionId: ' session-2 ',
        },
        query: {
          tab: 'notifications',
          webSessionId: 'old-session',
        },
        addRecentProject,
        push,
      })
    ).resolves.toBe(true);

    expect(addRecentProject).toHaveBeenCalledTimes(1);
    expect(addRecentProject).toHaveBeenCalledWith('project-1');
    expect(push).toHaveBeenCalledTimes(1);
    expect(push).toHaveBeenCalledWith({
      name: 'project',
      params: { id: 'project-1' },
      query: {
        tab: 'web',
        webSessionId: 'session-2',
      },
    });
  });

  it('ignores incomplete notification events', async () => {
    const addRecentProject = vi.fn();
    const push = vi.fn().mockResolvedValue(undefined);

    await expect(
      openWebSessionNotificationTarget({
        event: {
          projectId: 'project-1',
          sessionId: '',
        },
        addRecentProject,
        push,
      })
    ).resolves.toBe(false);

    expect(addRecentProject).not.toHaveBeenCalled();
    expect(push).not.toHaveBeenCalled();
  });
});
