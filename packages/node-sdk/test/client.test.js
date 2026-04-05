import test from 'node:test';
import assert from 'node:assert/strict';

import { CodeKanbanClient } from '../src/client.js';

function createJsonResponse(payload, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    async text() {
      return JSON.stringify(payload);
    },
  };
}

function createFetchMock(handlers) {
  return async (input, init = {}) => {
    const url = input instanceof URL ? input : new URL(String(input));
    const method = (init.method || 'GET').toUpperCase();
    const key = `${method} ${url.pathname}`;
    const handler = handlers.get(key);
    assert.ok(handler, `unexpected request: ${key}`);
    const parsedBody = init.body ? JSON.parse(init.body) : undefined;
    return handler({ url, method, body: parsedBody, headers: init.headers || {} });
  };
}

class FakeWebSocket {
  static instances = [];

  constructor(url) {
    this.url = url;
    this.listeners = new Map();
    this.sent = [];
    FakeWebSocket.instances.push(this);
    queueMicrotask(() => {
      this.emit('open', { type: 'open' });
      this.emit('message', { data: JSON.stringify({ type: 'ready', data: 'running' }) });
      this.emit('message', { data: JSON.stringify({ type: 'metadata', metadata: { aiSessionId: 'ai-session-1' } }) });
    });
  }

  addEventListener(type, handler) {
    const bucket = this.listeners.get(type) || new Set();
    bucket.add(handler);
    this.listeners.set(type, bucket);
  }

  removeEventListener(type, handler) {
    const bucket = this.listeners.get(type);
    if (!bucket) {
      return;
    }
    bucket.delete(handler);
  }

  emit(type, payload) {
    const bucket = this.listeners.get(type);
    if (!bucket) {
      return;
    }
    for (const handler of bucket) {
      handler(payload);
    }
  }

  send(payload) {
    this.sent.push(JSON.parse(payload));
  }

  close() {
    this.emit('close', { type: 'close' });
  }
}

test('resolveProject creates a project when path is not registered', async () => {
  const handlers = new Map([
    ['GET /api/v1/projects', () => createJsonResponse({ items: [] })],
    ['POST /api/v1/projects/create', ({ body }) => createJsonResponse({ item: { id: 'p1', path: body.path, name: body.name } }, 201)],
  ]);

  const client = new CodeKanbanClient({
    baseURL: 'http://127.0.0.1:3000',
    fetchImpl: createFetchMock(handlers),
    WebSocketImpl: FakeWebSocket,
  });

  const result = await client.resolveProject({ path: 'D:/repo/demo' });
  assert.equal(result.project.id, 'p1');
  assert.equal(result.matchedBy, 'created');
});

test('startWorkflow creates a terminal and sends command plus prompt', async () => {
  FakeWebSocket.instances.length = 0;
  const handlers = new Map([
    ['GET /api/v1/projects', () => createJsonResponse({ items: [{ id: 'p1', path: 'D:/repo/demo', name: 'demo' }] })],
    ['GET /api/v1/projects/p1/worktrees', () => createJsonResponse({ items: [{ id: 'w1', path: 'D:/repo/demo', isMain: true }] })],
    ['POST /api/v1/projects/p1/worktrees/w1/terminals', () => createJsonResponse({ item: { id: 't1', wsPath: '/api/v1/terminal/ws?sessionId=t1', wsUrl: '/api/v1/terminal/ws?sessionId=t1', title: 'demo', projectId: 'p1', worktreeId: 'w1', workingDir: 'D:/repo/demo' } }, 201)],
  ]);

  const client = new CodeKanbanClient({
    baseURL: 'http://127.0.0.1:3000',
    fetchImpl: createFetchMock(handlers),
    WebSocketImpl: FakeWebSocket,
  });

  const result = await client.startWorkflow({
    path: 'D:/repo/demo',
    agent: 'codex',
    profile: 'plan',
    permissions: { addDirs: ['D:/shared'] },
    prompt: 'Inspect and plan first',
  });

  assert.equal(result.project.id, 'p1');
  assert.equal(result.worktree.id, 'w1');
  assert.equal(result.terminalSession.id, 't1');
  assert.equal(result.aiSessionId, 'ai-session-1');
  assert.equal(FakeWebSocket.instances.length, 1);
  assert.equal(FakeWebSocket.instances[0].url, 'ws://127.0.0.1:3000/api/v1/terminal/ws?sessionId=t1');
  assert.match(FakeWebSocket.instances[0].sent[0].data, /codex -s workspace-write -a on-request --add-dir D:\/shared/);
  assert.match(FakeWebSocket.instances[0].sent[1].data, /planning mode/i);
});

test('listSessions returns terminal and ai summaries', async () => {
  const handlers = new Map([
    ['GET /api/v1/projects/p1', () => createJsonResponse({ item: { id: 'p1', path: 'D:/repo/demo', name: 'demo' } })],
    ['GET /api/v1/projects/p1/terminals', () => createJsonResponse({ items: [{ id: 't1' }] })],
    ['GET /api/v1/projects/p1/ai-sessions', () => createJsonResponse({ item: { hasCodex: true, hasClaudeCode: false, codexSessions: [{ id: 'a1' }], claudeSessions: [] } })],
  ]);

  const client = new CodeKanbanClient({
    baseURL: 'http://127.0.0.1:3000',
    fetchImpl: createFetchMock(handlers),
    WebSocketImpl: FakeWebSocket,
  });

  const result = await client.listSessions({ projectId: 'p1' });
  assert.equal(result.project.id, 'p1');
  assert.equal(result.terminalSessions.length, 1);
  assert.equal(result.aiSessions.codexSessions.length, 1);
});
