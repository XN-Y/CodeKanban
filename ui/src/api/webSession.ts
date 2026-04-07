import type { WebSessionAttachment, WebSessionSummary } from '@/types/models';
import { http } from './http';

type ItemResponse<T> = {
  item?: T;
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
          title: data.title ?? '',
        })
        .send()) ?? {};
    if (!body.item) {
      throw new Error('failed to create web session');
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
