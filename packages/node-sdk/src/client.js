import { buildAgentLaunchSpec } from './command-builder.js';
import { CodeKanbanConfigError, CodeKanbanHttpError, CodeKanbanValidationError } from './errors.js';
import { TerminalConnection } from './terminal-connection.js';
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

export class CodeKanbanClient {
  constructor(options = {}) {
    this.baseURL = normalizeBaseUrl(options.baseURL);
    this.headers = { ...(options.headers || {}) };
    this.fetchImpl = options.fetchImpl || globalThis.fetch;
    this.WebSocketImpl = options.WebSocketImpl || globalThis.WebSocket;
    if (!this.fetchImpl) {
      throw new CodeKanbanConfigError('fetch implementation is unavailable');
    }
  }

  async requestJson(path, options = {}) {
    const method = options.method || 'GET';
    const headers = { Accept: 'application/json', ...this.headers, ...(options.headers || {}) };
    const request = {
      method,
      headers,
    };
    if (options.body !== undefined) {
      request.body = JSON.stringify(options.body);
      request.headers['Content-Type'] = 'application/json';
    }
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

  async createTerminalSession({ projectId, worktreeId, workingDir = '', title = '', rows = 0, cols = 0, taskId = '' }) {
    ensureString(projectId, 'projectId');
    ensureString(worktreeId, 'worktreeId');
    const response = await this.requestJson(`/api/v1/projects/${projectId}/worktrees/${worktreeId}/terminals`, {
      method: 'POST',
      body: {
        workingDir,
        title,
        rows,
        cols,
        taskId,
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
      taskId: ensureOptionalString(input.taskId),
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
