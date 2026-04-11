import { readFile } from 'node:fs/promises';

import { buildAgentLaunchSpec } from './command-builder.js';
import { CodeKanbanConfigError, CodeKanbanHttpError, CodeKanbanValidationError } from './errors.js';
import { TerminalConnection } from './terminal-connection.js';
import { WebSessionCommandChannel } from './web-session-command-channel.js';
import { WebSessionEventStream } from './web-session-event-stream.js';
import {
  WEB_SESSION_COMMAND_WS_PATH,
  WEB_SESSION_EVENTS_WS_PATH,
  analyzeWebSession,
  ensureImageMimeType,
  normalizeWebSessionAttachment,
} from './web-session-shared.js';
import {
  ensureOptionalString,
  ensureString,
  normalizeBaseUrl,
  normalizeFsPath,
  normalizeTerminalEnter,
  pathBasename,
  sleep,
  toWsUrl,
} from './utils.js';

const DEFAULT_REQUEST_RETRY = Object.freeze({
  attempts: 2,
  baseDelayMs: 250,
  maxDelayMs: 1000,
});

const RETRYABLE_HTTP_STATUS_CODES = new Set([408, 425, 429, 500, 502, 503, 504]);

const DEFAULT_WEB_SESSION_AUTO_RETRY_SCOPE = 'network_only';
const DEFAULT_WEB_SESSION_AUTO_RETRY_PRESET = 'gentle_stop';

function defaultWebSessionModel(agent) {
  return agent === 'claude' ? 'opus' : 'gpt-5.4';
}

function defaultWebSessionReasoningEffort(agent) {
  return agent === 'claude' ? 'default' : 'xhigh';
}

function defaultWebSessionWorkflowMode(permissionMode) {
  return permissionMode === 'plan' ? 'plan' : 'default';
}

function defaultWebSessionPermissionLevel(permissionMode) {
  return permissionMode === 'yolo' ? 'yolo' : 'elevated';
}

function normalizeWebSessionAutoRetryScope(scope) {
  const normalized = ensureOptionalString(scope);
  if (['network_only', 'network_and_rate_limit', 'all_failures'].includes(normalized)) {
    return normalized;
  }
  return DEFAULT_WEB_SESSION_AUTO_RETRY_SCOPE;
}

function normalizeWebSessionAutoRetryPreset(preset) {
  const normalized = ensureOptionalString(preset);
  if (['gentle_stop', 'aggressive_stop', 'sustain_60s'].includes(normalized)) {
    return normalized;
  }
  return DEFAULT_WEB_SESSION_AUTO_RETRY_PRESET;
}

function normalizeRequestRetryConfig(value = {}) {
  const attempts = Number.isFinite(value.attempts) ? Math.max(1, Math.trunc(value.attempts)) : DEFAULT_REQUEST_RETRY.attempts;
  const baseDelayMs = Number.isFinite(value.baseDelayMs)
    ? Math.max(1, Math.trunc(value.baseDelayMs))
    : DEFAULT_REQUEST_RETRY.baseDelayMs;
  const maxDelayMs = Number.isFinite(value.maxDelayMs)
    ? Math.max(baseDelayMs, Math.trunc(value.maxDelayMs))
    : DEFAULT_REQUEST_RETRY.maxDelayMs;
  return {
    attempts,
    baseDelayMs,
    maxDelayMs,
  };
}

function resolveRequestRetryConfig(method, defaults, override) {
  if (override === false) {
    return null;
  }

  const normalizedMethod = String(method || 'GET').toUpperCase();
  const source = override && typeof override === 'object' ? override : {};
  const enabled =
    typeof source.enabled === 'boolean'
      ? source.enabled
      : ['GET', 'HEAD', 'OPTIONS'].includes(normalizedMethod);
  if (!enabled) {
    return null;
  }

  return normalizeRequestRetryConfig({
    ...defaults,
    ...source,
  });
}

function isRetryableRequestError(error) {
  if (error instanceof CodeKanbanHttpError) {
    return RETRYABLE_HTTP_STATUS_CODES.has(error.status);
  }
  if (error instanceof CodeKanbanValidationError || error instanceof CodeKanbanConfigError) {
    return false;
  }
  if (error instanceof SyntaxError) {
    return false;
  }
  if (error?.name === 'AbortError') {
    return true;
  }
  if (error instanceof TypeError) {
    return true;
  }
  return false;
}

function getRetryDelayMs(retryConfig, attemptIndex) {
  return Math.min(retryConfig.maxDelayMs, retryConfig.baseDelayMs * 2 ** attemptIndex);
}

export class CodeKanbanClient {
  constructor(options = {}) {
    this.baseURL = normalizeBaseUrl(options.baseURL);
    this.headers = { ...(options.headers || {}) };
    this.fetchImpl = options.fetchImpl || globalThis.fetch;
    this.WebSocketImpl = options.WebSocketImpl || globalThis.WebSocket;
    this.requestRetry = normalizeRequestRetryConfig(options.requestRetry);
    if (!this.fetchImpl) {
      throw new CodeKanbanConfigError('fetch implementation is unavailable');
    }
  }

  async requestJson(path, options = {}) {
    const method = String(options.method || 'GET').toUpperCase();
    const headers = { Accept: 'application/json', ...this.headers, ...(options.headers || {}) };
    const request = {
      method,
      headers,
    };
    if (options.body !== undefined) {
      request.body = JSON.stringify(options.body);
      request.headers['Content-Type'] = 'application/json';
    }

    const retryConfig = resolveRequestRetryConfig(method, this.requestRetry, options.retry);
    const maxAttempts = retryConfig?.attempts ?? 1;

    for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
      try {
        const response = await this.fetchImpl(new URL(path, this.baseURL), request);
        const text = await response.text();
        const body = text ? JSON.parse(text) : null;
        if (!response.ok) {
          throw new CodeKanbanHttpError(`request failed with ${response.status}`, {
            status: response.status,
            method,
            path,
            body,
          });
        }
        return body;
      } catch (error) {
        const canRetry = retryConfig && attempt + 1 < maxAttempts && isRetryableRequestError(error);
        if (!canRetry) {
          throw error;
        }
        await sleep(getRetryDelayMs(retryConfig, attempt));
      }
    }

    throw new CodeKanbanValidationError(`request retry loop exited unexpectedly for ${method} ${path}`);
  }

  async resolveProjectReference({ projectId, path, ensureProject = true }) {
    const { project } = await this.resolveProject({
      projectId,
      path,
      ensureProject,
    });
    return project;
  }

  async resolveProjectId({ projectId, path, ensureProject = true }) {
    const resolvedProjectId = ensureOptionalString(projectId);
    if (resolvedProjectId) {
      return resolvedProjectId;
    }
    const { project } = await this.resolveProject({
      projectId,
      path,
      ensureProject,
    });
    return ensureString(project?.id, 'projectId');
  }

  async listProjects() {
    const response = await this.requestJson('/api/v1/projects');
    return response?.items || [];
  }

  async getProject(projectId) {
    ensureString(projectId, 'projectId');
    const response = await this.requestJson(`/api/v1/projects/${projectId}`);
    return response?.item;
  }

  async createProject({ path, name, description = '', worktreeBasePath, hidePath }) {
    ensureString(path, 'path');
    ensureString(name, 'name');
    const response = await this.requestJson('/api/v1/projects/create', {
      method: 'POST',
      body: {
        path,
        name,
        description,
        worktreeBasePath,
        hidePath,
      },
    });
    return response?.item;
  }

  async listWorktrees(projectId) {
    ensureString(projectId, 'projectId');
    const response = await this.requestJson(`/api/v1/projects/${projectId}/worktrees`);
    return response?.items || [];
  }

  async listTerminalSessions(projectId) {
    ensureString(projectId, 'projectId');
    const response = await this.requestJson(`/api/v1/projects/${projectId}/terminals`);
    return response?.items || [];
  }

  async listAISessionsByProject(projectId) {
    ensureString(projectId, 'projectId');
    const response = await this.requestJson(`/api/v1/projects/${projectId}/ai-sessions`);
    return response?.item || null;
  }

  async listAISessionsByPath(projectPath) {
    ensureString(projectPath, 'path');
    const response = await this.requestJson('/api/v1/ai-sessions/by-path', {
      method: 'POST',
      body: { path: projectPath },
    });
    return response?.item || null;
  }

  async getAISessionConversation({ id, sessionId, refresh = false }) {
    const dbId = ensureOptionalString(id);
    const rawSessionId = ensureOptionalString(sessionId);
    if (!dbId && !rawSessionId) {
      throw new CodeKanbanValidationError('id or sessionId is required');
    }
    if (refresh && !dbId) {
      throw new CodeKanbanValidationError('refresh currently requires a database id');
    }

    if (dbId) {
      const path = refresh ? `/api/v1/ai-sessions/${dbId}/refresh` : `/api/v1/ai-sessions/${dbId}/conversation`;
      const response = await this.requestJson(path, { method: refresh ? 'POST' : 'GET' });
      return response?.item || null;
    }

    const response = await this.requestJson(`/api/v1/ai-sessions/by-session-id/${rawSessionId}/conversation`);
    return response?.item || null;
  }

  async getAISessionToolResult({ id, sessionId, toolUseId }) {
    const dbId = ensureOptionalString(id);
    const rawSessionId = ensureOptionalString(sessionId);
    const resolvedToolUseId = ensureString(toolUseId, 'toolUseId');
    if (!dbId && !rawSessionId) {
      throw new CodeKanbanValidationError('id or sessionId is required');
    }
    const path = dbId
      ? `/api/v1/ai-sessions/${dbId}/conversation/tool-results/${resolvedToolUseId}`
      : `/api/v1/ai-sessions/by-session-id/${rawSessionId}/conversation/tool-results/${resolvedToolUseId}`;
    const response = await this.requestJson(path);
    return response?.item || null;
  }

  async createTerminalSession({ projectId, worktreeId, workingDir = '', title = '', rows = 0, cols = 0 }) {
    ensureString(projectId, 'projectId');
    ensureString(worktreeId, 'worktreeId');
    const response = await this.requestJson(`/api/v1/projects/${projectId}/worktrees/${worktreeId}/terminals`, {
      method: 'POST',
      body: {
        workingDir,
        title,
        rows,
        cols,
      },
    });
    return response?.item;
  }

  async resolveProject({ projectId, path, ensureProject = true }) {
    const resolvedProjectId = ensureOptionalString(projectId);
    const resolvedPath = ensureOptionalString(path);

    if (resolvedProjectId) {
      const project = await this.getProject(resolvedProjectId);
      return {
        project,
        matchedBy: 'projectId',
      };
    }

    if (!resolvedPath) {
      throw new CodeKanbanValidationError('projectId or path is required');
    }

    const target = normalizeFsPath(resolvedPath);
    const projects = await this.listProjects();
    const project = projects.find(item => normalizeFsPath(item.path) === target);
    if (project) {
      return {
        project,
        matchedBy: 'path',
      };
    }

    if (!ensureProject) {
      throw new CodeKanbanValidationError(`no CodeKanban project is registered for path: ${resolvedPath}`);
    }

    const created = await this.createProject({
      path: resolvedPath,
      name: pathBasename(resolvedPath),
      description: '',
    });
    return {
      project: created,
      matchedBy: 'created',
    };
  }

  async resolveWorktree({ projectId, worktreeId }) {
    const project = ensureString(projectId, 'projectId');
    const preferredWorktreeId = ensureOptionalString(worktreeId);
    const worktrees = await this.listWorktrees(project);
    if (worktrees.length === 0) {
      throw new CodeKanbanValidationError(`no worktrees are available for project ${project}`);
    }
    if (preferredWorktreeId) {
      const direct = worktrees.find(item => item.id === preferredWorktreeId);
      if (!direct) {
        throw new CodeKanbanValidationError(`worktree ${preferredWorktreeId} was not found in project ${project}`);
      }
      return direct;
    }
    return worktrees.find(item => item.isMain) || worktrees[0];
  }

  connectTerminal({ sessionId, wsPath, wsUrl }) {
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const resolvedPath =
      ensureOptionalString(wsUrl) || ensureOptionalString(wsPath) || `/api/v1/terminal/ws?sessionId=${resolvedSessionId}`;
    const url = resolvedPath.startsWith('ws://') || resolvedPath.startsWith('wss://') ? resolvedPath : toWsUrl(this.baseURL, resolvedPath);

    return new TerminalConnection({
      sessionId: resolvedSessionId,
      url,
      WebSocketImpl: this.WebSocketImpl,
    });
  }

  openWebSessionCommandChannel() {
    return new WebSessionCommandChannel({
      url: toWsUrl(this.baseURL, WEB_SESSION_COMMAND_WS_PATH),
      WebSocketImpl: this.WebSocketImpl,
    });
  }

  openWebSessionEventStream(options = {}) {
    return new WebSessionEventStream({
      url: toWsUrl(this.baseURL, WEB_SESSION_EVENTS_WS_PATH),
      sessionId: ensureOptionalString(options.sessionId),
      WebSocketImpl: this.WebSocketImpl,
    });
  }

  async withWebSessionCommandChannel(handler) {
    const channel = this.openWebSessionCommandChannel();
    try {
      await channel.waitForOpen();
      return await handler(channel);
    } finally {
      channel.close();
    }
  }

  async listSessions({ projectId, path, includeTerminal = true, includeAI = true, ensureProject = true }) {
    const { project, matchedBy } = await this.resolveProject({ projectId, path, ensureProject });

    const [terminalSessions, aiSessions] = await Promise.all([
      includeTerminal ? this.listTerminalSessions(project.id) : Promise.resolve([]),
      includeAI
        ? this.listAISessionsByProject(project.id)
        : Promise.resolve({
            hasClaudeCode: false,
            hasCodex: false,
            claudeSessions: [],
            codexSessions: [],
          }),
    ]);

    return {
      project,
      matchedBy,
      terminalSessions,
      aiSessions,
    };
  }

  async listWebSessions({ projectId, path, ensureProject = true } = {}) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject });
    const response = await this.requestJson(`/api/v1/projects/${resolvedProjectId}/web-sessions`);
    return response?.items || [];
  }

  async createWebSession(input = {}) {
    const projectId = await this.resolveProjectId({
      projectId: input.projectId,
      path: input.path,
      ensureProject: true,
    });

    const agent = ensureString(input.agent, 'agent');
    const permissionMode = ensureOptionalString(input.permissionMode).toLowerCase();
    const worktree = await this.resolveWorktree({
      projectId,
      worktreeId: input.worktreeId,
    });

    const response = await this.requestJson(`/api/v1/projects/${projectId}/web-sessions`, {
      method: 'POST',
      body: {
        worktreeId: ensureString(worktree?.id, 'worktreeId'),
        agent,
        model: ensureOptionalString(input.model) || defaultWebSessionModel(agent),
        reasoningEffort:
          ensureOptionalString(input.reasoningEffort) || defaultWebSessionReasoningEffort(agent),
        workflowMode: ensureOptionalString(input.workflowMode) || defaultWebSessionWorkflowMode(permissionMode),
        permissionLevel:
          ensureOptionalString(input.permissionLevel) || defaultWebSessionPermissionLevel(permissionMode),
        autoRetryEnabled: input.autoRetryEnabled === true,
        autoRetryScope: normalizeWebSessionAutoRetryScope(input.autoRetryScope),
        autoRetryPreset: normalizeWebSessionAutoRetryPreset(input.autoRetryPreset),
        permissionMode,
        title: ensureOptionalString(input.title),
      },
    });
    return response?.item || null;
  }

  async getWebSessionSnapshot({ projectId, path, sessionId, limit = 80 }) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject: true });
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const normalizedLimit = Number.isFinite(limit) ? Math.max(1, Math.trunc(limit)) : 80;
    const response = await this.requestJson(
      `/api/v1/projects/${resolvedProjectId}/web-sessions/${resolvedSessionId}/snapshot?limit=${normalizedLimit}`,
    );
    return response?.item || null;
  }

  async getWebSessionHistory({ projectId, path, sessionId, beforeCursor, limit = 80 }) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject: true });
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const params = new URLSearchParams();
    if (ensureOptionalString(beforeCursor)) {
      params.set('beforeCursor', ensureOptionalString(beforeCursor));
    }
    if (Number.isFinite(limit)) {
      params.set('limit', String(Math.max(1, Math.trunc(limit))));
    }
    const suffix = params.toString();
    const response = await this.requestJson(
      `/api/v1/projects/${resolvedProjectId}/web-sessions/${resolvedSessionId}/history${suffix ? `?${suffix}` : ''}`,
    );
    return response?.item || null;
  }

  async syncWebSession({ projectId, path, sessionId, mode, clearExisting = false }) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject: true });
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const response = await this.requestJson(`/api/v1/projects/${resolvedProjectId}/web-sessions/${resolvedSessionId}/sync`, {
      method: 'POST',
      body: {
        ...(ensureOptionalString(mode) ? { mode: ensureOptionalString(mode) } : {}),
        clearExisting: clearExisting === true,
      },
    });
    return response?.item || null;
  }

  async archiveWebSession({ projectId, path, sessionId }) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject: true });
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const response = await this.requestJson(`/api/v1/projects/${resolvedProjectId}/web-sessions/${resolvedSessionId}/archive`, {
      method: 'POST',
    });
    return response?.item || null;
  }

  async unarchiveWebSession({ projectId, path, sessionId }) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject: true });
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const response = await this.requestJson(`/api/v1/projects/${resolvedProjectId}/web-sessions/${resolvedSessionId}/unarchive`, {
      method: 'POST',
    });
    return response?.item || null;
  }

  async renameWebSession({ projectId, path, sessionId, title }) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject: true });
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const response = await this.requestJson(`/api/v1/projects/${resolvedProjectId}/web-sessions/${resolvedSessionId}/rename`, {
      method: 'POST',
      body: {
        title: ensureString(title, 'title'),
      },
    });
    return response?.item || null;
  }

  async closeWebSession({ projectId, path, sessionId }) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject: true });
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const response = await this.requestJson(`/api/v1/projects/${resolvedProjectId}/web-sessions/${resolvedSessionId}/close`, {
      method: 'POST',
    });
    return {
      message: response?.message || 'session aborted',
    };
  }

  async deleteWebSession({ projectId, path, sessionId }) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject: true });
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const response = await this.requestJson(`/api/v1/projects/${resolvedProjectId}/web-sessions/${resolvedSessionId}`, {
      method: 'DELETE',
    });
    return {
      message: response?.message || 'session deleted',
    };
  }

  async queryArchivedWebSessions({ projectIds, offset = 0, limit = 20 }) {
    const response = await this.requestJson('/api/v1/web-sessions/archived/query', {
      method: 'POST',
      body: {
        projectIds: Array.isArray(projectIds) ? projectIds : [],
        offset: Number.isFinite(offset) ? Math.max(0, Math.trunc(offset)) : 0,
        limit: Number.isFinite(limit) ? Math.max(1, Math.trunc(limit)) : 20,
      },
    });
    return response?.item || { items: [], total: 0, hasMore: false, nextOffset: 0 };
  }

  async getWebSessionCommandGroup({ projectId, path, sessionId, groupId }) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject: true });
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const resolvedGroupId = ensureString(groupId, 'groupId');
    const response = await this.requestJson(
      `/api/v1/projects/${resolvedProjectId}/web-sessions/${resolvedSessionId}/command-groups/${resolvedGroupId}`,
    );
    return response?.item || null;
  }

  async getWebSessionRuntimeConfig() {
    const response = await this.requestJson('/api/v1/web-sessions/runtime-config');
    return response?.item || null;
  }

  async uploadWebSessionAttachment({ projectId, path, filePath, fileName, mimeType }) {
    const resolvedProjectId = await this.resolveProjectId({ projectId, path, ensureProject: true });
    const resolvedFilePath = ensureString(filePath, 'filePath');
    const resolvedFileName = ensureOptionalString(fileName) || pathBasename(resolvedFilePath);
    const resolvedMimeType = ensureImageMimeType(mimeType, resolvedFileName);
    const fileBuffer = await readFile(resolvedFilePath);
    const formData = new FormData();
    formData.append('file', new File([fileBuffer], resolvedFileName, { type: resolvedMimeType }));

    const headers = {
      Accept: 'application/json',
      ...this.headers,
    };
    delete headers['Content-Type'];

    const response = await this.fetchImpl(
      new URL(`/api/v1/projects/${resolvedProjectId}/web-sessions/attachments`, this.baseURL),
      {
        method: 'POST',
        headers,
        body: formData,
      },
    );
    const text = await response.text();
    const body = text ? JSON.parse(text) : null;
    if (!response.ok) {
      throw new CodeKanbanHttpError(`request failed with ${response.status}`, {
        status: response.status,
        method: 'POST',
        path: `/api/v1/projects/${resolvedProjectId}/web-sessions/attachments`,
        body,
      });
    }
    return normalizeWebSessionAttachment(body?.item);
  }

  analyzeWebSession(snapshot) {
    return analyzeWebSession(snapshot);
  }

  async getWebSessionState({ projectId, path, sessionId, limit = 120 }) {
    const snapshot = await this.getWebSessionSnapshot({
      projectId,
      path,
      sessionId,
      limit,
    });
    return this.analyzeWebSession(snapshot);
  }

  async sendWebSessionMessage({ sessionId, text, attachmentIds = [] }) {
    return await this.withWebSessionCommandChannel(channel =>
      channel.sendMessage(sessionId, {
        text,
        attachmentIds,
      }),
    );
  }

  async updateWebSessionWorkflowMode({ sessionId, workflowMode }) {
    return await this.withWebSessionCommandChannel(channel =>
      channel.updateWorkflowMode(sessionId, {
        workflowMode,
      }),
    );
  }

  async answerWebSessionUserInput({ sessionId, itemId, answers }) {
    return await this.withWebSessionCommandChannel(channel =>
      channel.answerUserInput(sessionId, {
        itemId,
        answers,
      }),
    );
  }

  async approveWebSession({ sessionId }) {
    return await this.withWebSessionCommandChannel(channel => channel.approve(sessionId));
  }

  async rejectWebSession({ sessionId }) {
    return await this.withWebSessionCommandChannel(channel => channel.reject(sessionId));
  }

  async answerPendingUserInput({ projectId, path, sessionId, answers, limit = 120 }) {
    const state = await this.getWebSessionState({
      projectId,
      path,
      sessionId,
      limit,
    });
    if (!state.pendingUserInput?.itemId) {
      throw new CodeKanbanValidationError(`web session ${sessionId} has no pending user input`);
    }
    const ack = await this.answerWebSessionUserInput({
      sessionId,
      itemId: state.pendingUserInput.itemId,
      answers,
    });
    return {
      sessionId,
      itemId: state.pendingUserInput.itemId,
      prompt: state.pendingUserInput.prompt,
      ack,
      state,
    };
  }

  async approvePending({ projectId, path, sessionId, limit = 120 }) {
    const state = await this.getWebSessionState({
      projectId,
      path,
      sessionId,
      limit,
    });
    if (!state.pendingApproval) {
      throw new CodeKanbanValidationError(`web session ${sessionId} has no pending approval`);
    }
    const ack = await this.approveWebSession({ sessionId });
    return {
      sessionId,
      prompt: state.pendingApproval.prompt,
      ack,
      state,
    };
  }

  async rejectPending({ projectId, path, sessionId, limit = 120 }) {
    const state = await this.getWebSessionState({
      projectId,
      path,
      sessionId,
      limit,
    });
    if (!state.pendingApproval) {
      throw new CodeKanbanValidationError(`web session ${sessionId} has no pending approval`);
    }
    const ack = await this.rejectWebSession({ sessionId });
    return {
      sessionId,
      prompt: state.pendingApproval.prompt,
      ack,
      state,
    };
  }

  async executeLatestPlan({ projectId, path, sessionId, prompt = 'Implement the plan.', limit = 120 }) {
    const state = await this.getWebSessionState({
      projectId,
      path,
      sessionId,
      limit,
    });
    if (!state.latestPlan) {
      throw new CodeKanbanValidationError(`web session ${sessionId} has no latest plan to execute`);
    }
    if (!state.canSend && state.nextAction?.type !== 'execute_plan') {
      throw new CodeKanbanValidationError(`web session ${sessionId} is not ready to execute the latest plan`);
    }

    return await this.withWebSessionCommandChannel(async channel => {
      if (state.session?.workflowMode === 'plan') {
        await channel.updateWorkflowMode(sessionId, {
          workflowMode: 'default',
        });
      }

      if (
        state.pendingUserInput?.isPlanChoice &&
        state.pendingUserInput.itemId &&
        state.pendingUserInput.questionId &&
        state.pendingUserInput.executeOptionLabel
      ) {
        const ack = await channel.answerUserInput(sessionId, {
          itemId: state.pendingUserInput.itemId,
          answers: {
            [state.pendingUserInput.questionId]: [state.pendingUserInput.executeOptionLabel],
          },
        });
        return {
          sessionId,
          mode: 'plan_choice',
          latestPlan: state.latestPlan,
          ack,
          state,
        };
      }

      const ack = await channel.sendMessage(sessionId, {
        text: prompt,
        attachmentIds: [],
      });
      return {
        sessionId,
        mode: 'followup_message',
        prompt,
        latestPlan: state.latestPlan,
        ack,
        state,
      };
    });
  }

  async waitForWebSessionState({
    projectId,
    path,
    sessionId,
    until,
    intervalMs = 5000,
    timeoutMs = 60000,
    limit = 120,
  }) {
    if (!until) {
      throw new CodeKanbanValidationError('until is required');
    }

    const matches =
      typeof until === 'function'
        ? until
        : Array.isArray(until)
          ? state => until.includes(state.phase)
          : state => state.phase === until;

    const startedAt = Date.now();
    let lastRetryableError = null;
    while (Date.now() - startedAt <= timeoutMs) {
      try {
        const state = await this.getWebSessionState({
          projectId,
          path,
          sessionId,
          limit,
        });
        lastRetryableError = null;
        if (matches(state)) {
          return state;
        }
      } catch (error) {
        if (!isRetryableRequestError(error)) {
          throw error;
        }
        lastRetryableError = error;
      }

      await sleep(Math.max(1, Math.trunc(intervalMs)));
    }

    if (lastRetryableError) {
      throw new CodeKanbanValidationError(
        `web session ${sessionId} did not reach the requested state within ${timeoutMs}ms (last transient error: ${lastRetryableError.message})`,
        { cause: lastRetryableError },
      );
    }

    throw new CodeKanbanValidationError(`web session ${sessionId} did not reach the requested state within ${timeoutMs}ms`);
  }

  async startWorkflow(input = {}) {
    const launch = buildAgentLaunchSpec(input);
    const { project, matchedBy } = await this.resolveProject({
      projectId: input.projectId,
      path: input.path,
      ensureProject: true,
    });
    const worktree = await this.resolveWorktree({
      projectId: project.id,
      worktreeId: input.worktreeId,
    });

    const terminal = await this.createTerminalSession({
      projectId: project.id,
      worktreeId: worktree.id,
      workingDir: ensureOptionalString(input.workingDir) || worktree.path,
      title: ensureOptionalString(input.title) || ensureOptionalString(input.prompt) || 'AI workflow',
      rows: Number.isFinite(input.rows) ? input.rows : 0,
      cols: Number.isFinite(input.cols) ? input.cols : 0,
    });

    const connection = this.connectTerminal({
      sessionId: terminal.id,
      wsPath: terminal.wsPath,
      wsUrl: terminal.wsUrl,
    });

    await connection.waitForReady();
    connection.sendInput(normalizeTerminalEnter(launch.command));
    await sleep(500);
    connection.sendInput(normalizeTerminalEnter(launch.prompt));
    const metadata = await connection.waitForMetadata();

    return {
      project,
      matchedBy,
      worktree,
      terminalSession: terminal,
      agent: launch.agent,
      profile: launch.profile,
      command: launch.command,
      prompt: launch.prompt,
      promptAccepted: true,
      aiSessionId: metadata?.aiSessionId,
      connection,
    };
  }

  async continueTerminalSession({ projectId, path, sessionId, prompt }) {
    const resolvedSessionId = ensureString(sessionId, 'sessionId');
    const resolvedPrompt = ensureString(prompt, 'prompt');

    let project;
    if (projectId || path) {
      ({ project } = await this.resolveProject({ projectId, path, ensureProject: true }));
      const sessions = await this.listTerminalSessions(project.id);
      const exists = sessions.some(item => item.id === resolvedSessionId);
      if (!exists) {
        throw new CodeKanbanValidationError(`terminal session ${resolvedSessionId} does not belong to project ${project.id}`);
      }
    }

    const connection = this.connectTerminal({ sessionId: resolvedSessionId });
    await connection.waitForReady();
    connection.sendInput(normalizeTerminalEnter(resolvedPrompt));
    return {
      project,
      sessionId: resolvedSessionId,
      prompt: resolvedPrompt,
      promptAccepted: true,
      connection,
    };
  }
}
