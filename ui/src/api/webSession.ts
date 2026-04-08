import type { WebSessionAttachment, WebSessionSummary } from '@/types/models';
import { http } from './http';

type ItemResponse<T> = {
  item?: T;
};

type ArchivedQueryResult = {
  items: WebSessionSummary[];
  total: number;
  hasMore: boolean;
  nextOffset: number;
};

export const webSessionApi = {
  async list(projectId: string): Promise<WebSessionSummary[]> {
    const body =
      (await http
        .Get<{ items?: WebSessionSummary[] }>(`/projects/${projectId}/web-sessions`)
        .send(true)) ?? {};
    return body.items ?? [];
  },

  async create(
    projectId: string,
    data: {
      worktreeId?: string;
      agent: 'claude' | 'codex';
      model?: string;
      reasoningEffort?: 'default' | 'none' | 'low' | 'medium' | 'high' | 'xhigh';
      workflowMode?: 'default' | 'plan';
      permissionLevel?: 'default' | 'elevated' | 'yolo';
      permissionMode?: string;
      title?: string;
    }
  ): Promise<WebSessionSummary> {
    const body =
      (await http
        .Post<ItemResponse<WebSessionSummary>>(`/projects/${projectId}/web-sessions`, {
          worktreeId: data.worktreeId ?? '',
          agent: data.agent,
          model: data.model ?? '',
          reasoningEffort: data.reasoningEffort ?? 'default',
          workflowMode: data.workflowMode ?? 'default',
          permissionLevel: data.permissionLevel ?? 'elevated',
          permissionMode: data.permissionMode ?? '',
          title: data.title ?? '',
        })
        .send()) ?? {};
    if (!body.item) {
      throw new Error('failed to create web session');
    }
    return body.item;
  },

  async archive(projectId: string, sessionId: string): Promise<WebSessionSummary> {
    const body =
      (await http
        .Post<
          ItemResponse<WebSessionSummary>
        >(`/projects/${projectId}/web-sessions/${sessionId}/archive`)
        .send()) ?? {};
    if (!body.item) {
      throw new Error('failed to archive web session');
    }
    return body.item;
  },

  async unarchive(projectId: string, sessionId: string): Promise<WebSessionSummary> {
    const body =
      (await http
        .Post<
          ItemResponse<WebSessionSummary>
        >(`/projects/${projectId}/web-sessions/${sessionId}/unarchive`)
        .send()) ?? {};
    if (!body.item) {
      throw new Error('failed to unarchive web session');
    }
    return body.item;
  },

  async delete(projectId: string, sessionId: string): Promise<void> {
    await http.Delete(`/projects/${projectId}/web-sessions/${sessionId}`).send();
  },

  async queryArchived(data: {
    projectIds: string[];
    offset?: number;
    limit?: number;
  }): Promise<ArchivedQueryResult> {
    const body =
      (await http
        .Post<ItemResponse<ArchivedQueryResult>>('/web-sessions/archived/query', {
          projectIds: data.projectIds,
          offset: data.offset ?? 0,
          limit: data.limit ?? 20,
        })
        .send()) ?? {};
    if (!body.item) {
      throw new Error('failed to query archived web sessions');
    }
    return body.item;
  },

  async uploadAttachment(projectId: string, file: File): Promise<WebSessionAttachment> {
    const formData = new FormData();
    formData.append('file', file);
    const body =
      (await http
        .Post<
          ItemResponse<WebSessionAttachment>
        >(`/projects/${projectId}/web-sessions/attachments`, formData)
        .send()) ?? {};
    if (!body.item) {
      throw new Error('failed to upload attachment');
    }
    return body.item;
  },
};
