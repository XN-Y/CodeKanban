import { beforeEach, describe, expect, it, vi } from 'vitest';

const { getMethodMock, getSendMock, getAbortMock } = vi.hoisted(() => {
  const getSendMock = vi.fn();
  const getAbortMock = vi.fn();
  return {
    getMethodMock: vi.fn(() => ({
      send: getSendMock,
      abort: getAbortMock,
    })),
    getSendMock,
    getAbortMock,
  };
});

vi.mock('@/api', () => ({
  ApiError: class ApiError extends Error {},
  urlBase: '',
}));

vi.mock('@/api/http', () => ({
  http: {
    Get: getMethodMock,
    Post: vi.fn(),
    Patch: vi.fn(),
    Delete: vi.fn(),
  },
}));

import { fileManagerApi } from '@/api/fileManager';

describe('fileManagerApi.listChanges', () => {
  beforeEach(() => {
    getMethodMock.mockClear();
    getSendMock.mockReset();
    getAbortMock.mockReset();
    getSendMock.mockResolvedValue({
      item: {
        scope: {
          id: 'scope-1',
          kind: 'project',
          label: 'Project',
          rootPath: '/tmp/project',
        },
        entries: [],
        truncated: false,
        statsComplete: true,
        statsTimedOut: false,
        untrackedIncluded: false,
      },
    });
  });

  it('passes backend guardrail params instead of relying on local filtering', async () => {
    await fileManagerApi.listChanges('project-1', 'scope-1', {
      includeUntracked: false,
      withStats: true,
      timeoutMs: 5000,
      maxEntries: 1000,
    });

    expect(getMethodMock).toHaveBeenCalledWith(
      '/projects/project-1/files/changes?scopeId=scope-1&includeUntracked=false&withStats=true&timeoutMs=5000&maxEntries=1000'
    );
  });

  it('aborts the in-flight request when the caller aborts the signal', async () => {
    let rejectRequest: ((reason?: unknown) => void) | null = null;
    getMethodMock.mockImplementationOnce(() => ({
      send: vi.fn(
        () =>
          new Promise((_, reject) => {
            rejectRequest = reject;
          })
      ),
      abort: vi.fn(() => {
        rejectRequest?.(new DOMException('git changes load aborted', 'AbortError'));
      }),
    }));
    const controller = new AbortController();

    const request = fileManagerApi.listChanges('project-1', 'scope-1', {
      signal: controller.signal,
    });
    controller.abort();

    await expect(request).rejects.toMatchObject({
      name: 'AbortError',
    });
  });
});
