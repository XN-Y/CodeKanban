import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

import { runCli } from '../src/cli.js';
import { createFetchMock, createJsonResponse, FakeWebSocket } from './helpers.js';

function createAckingWebSocket(assertSent) {
  FakeWebSocket.reset();
  FakeWebSocket.setFactory(socket => {
    queueMicrotask(() => socket.open());
    const originalSend = socket.send.bind(socket);
    socket.send = payload => {
      originalSend(payload);
      const frame = JSON.parse(payload);
      assertSent?.(frame);
      queueMicrotask(() => {
        socket.emitJson({
          v: 1,
          k: 'ack',
          rid: frame.rid,
          sid: frame.sid,
          ts: 1710000300000,
          op: frame.op,
          ok: 1,
        });
      });
    };
  });
}

async function runCliCaptured(argv, options = {}) {
  const stdout = [];
  const stderr = [];
  const originalFetch = globalThis.fetch;
  const originalWebSocket = globalThis.WebSocket;

  globalThis.fetch = options.fetchImpl || (async () => createJsonResponse({}));
  globalThis.WebSocket = options.WebSocketImpl || FakeWebSocket;

  try {
    const exitCode = await runCli(argv, {
      stdout: {
        write(chunk) {
          stdout.push(String(chunk));
          return true;
        },
      },
      stderr: {
        write(chunk) {
          stderr.push(String(chunk));
          return true;
        },
      },
    });
    return {
      exitCode,
      stdout: stdout.join(''),
      stderr: stderr.join(''),
    };
  } finally {
    globalThis.fetch = originalFetch;
    globalThis.WebSocket = originalWebSocket;
    FakeWebSocket.reset();
  }
}

test('CLI web-session create prints the created session JSON', { concurrency: false }, async () => {
  const handlers = new Map([
    ['GET /api/v1/projects/p1', () => createJsonResponse({ item: { id: 'p1', path: '/repo/demo', name: 'demo' } })],
    ['POST /api/v1/projects/p1/web-sessions', ({ body }) => {
      assert.equal(body.agent, 'codex');
      assert.equal(body.workflowMode, 'plan');
      return createJsonResponse({ item: { id: 'ws-created', projectId: 'p1', title: 'Created' } }, 201);
    }],
  ]);

  const result = await runCliCaptured(
    ['web-session', 'create', '--base-url', 'http://127.0.0.1:3000', '--project-id', 'p1', '--agent', 'codex', '--workflow-mode', 'plan'],
    { fetchImpl: createFetchMock(handlers) },
  );

  assert.equal(result.exitCode, 0);
  const payload = JSON.parse(result.stdout);
  assert.equal(payload.id, 'ws-created');
});

test('CLI web-session send sends a websocket command and prints the ack', { concurrency: false }, async () => {
  createAckingWebSocket(frame => {
    assert.equal(frame.op, 'send');
    assert.equal(frame.sid, 'ws1');
    assert.deepEqual(frame.p, {
      txt: 'continue',
      atts: ['att-1'],
    });
  });

  const result = await runCliCaptured([
    'web-session',
    'send',
    '--base-url',
    'http://127.0.0.1:3000',
    '--session-id',
    'ws1',
    '--text',
    'continue',
    '--attachment-id',
    'att-1',
  ]);

  assert.equal(result.exitCode, 0);
  const payload = JSON.parse(result.stdout);
  assert.equal(payload.type, 'ack');
  assert.equal(payload.operation, 'send');
});

test('CLI web-session user-input parses answers JSON and prints the ack', { concurrency: false }, async () => {
  createAckingWebSocket(frame => {
    assert.equal(frame.op, 'user_input');
    assert.deepEqual(frame.p, {
      iid: 'item-1',
      ans: { choice: ['A'] },
    });
  });

  const result = await runCliCaptured([
    'web-session',
    'user-input',
    '--base-url',
    'http://127.0.0.1:3000',
    '--session-id',
    'ws1',
    '--item-id',
    'item-1',
    '--answers-json',
    '{"choice":["A"]}',
  ]);

  assert.equal(result.exitCode, 0);
  const payload = JSON.parse(result.stdout);
  assert.equal(payload.operation, 'user_input');
});

test('CLI web-session set-workflow sends the workflow update command', { concurrency: false }, async () => {
  createAckingWebSocket(frame => {
    assert.equal(frame.op, 'set_wm');
    assert.deepEqual(frame.p, { wm: 'plan' });
  });

  const result = await runCliCaptured([
    'web-session',
    'set-workflow',
    '--base-url',
    'http://127.0.0.1:3000',
    '--session-id',
    'ws1',
    '--workflow-mode',
    'plan',
  ]);

  assert.equal(result.exitCode, 0);
  const payload = JSON.parse(result.stdout);
  assert.equal(payload.operation, 'set_wm');
});

test('CLI web-session sync calls the HTTP sync endpoint', { concurrency: false }, async () => {
  const handlers = new Map([
    ['GET /api/v1/projects/p1', () => createJsonResponse({ item: { id: 'p1', path: '/repo/demo', name: 'demo' } })],
    ['POST /api/v1/projects/p1/web-sessions/ws1/sync', ({ body }) => {
      assert.deepEqual(body, { mode: 'deep', clearExisting: true });
      return createJsonResponse({
        item: {
          session: { id: 'ws1', projectId: 'p1' },
          history: { items: [], hasMore: false, total: 0 },
        },
      });
    }],
  ]);

  const result = await runCliCaptured(
    [
      'web-session',
      'sync',
      '--base-url',
      'http://127.0.0.1:3000',
      '--project-id',
      'p1',
      '--session-id',
      'ws1',
      '--mode',
      'deep',
      '--clear-existing',
    ],
    { fetchImpl: createFetchMock(handlers) },
  );

  assert.equal(result.exitCode, 0);
  const payload = JSON.parse(result.stdout);
  assert.equal(payload.session.id, 'ws1');
});

test('CLI web-session attach uploads a multipart image file', { concurrency: false }, async t => {
  const tempDir = await mkdtemp(path.join(os.tmpdir(), 'codekanban-cli-'));
  t.after(async () => {
    await rm(tempDir, { recursive: true, force: true });
  });

  const filePath = path.join(tempDir, 'upload.png');
  await writeFile(filePath, Buffer.from([0x89, 0x50, 0x4e, 0x47]));

  const handlers = new Map([
    ['GET /api/v1/projects/p1', () => createJsonResponse({ item: { id: 'p1', path: '/repo/demo', name: 'demo' } })],
    ['POST /api/v1/projects/p1/web-sessions/attachments', ({ body }) => {
      const file = body.get('file');
      assert.equal(file.name, 'upload.png');
      assert.equal(file.type, 'image/png');
      return createJsonResponse({
        item: {
          id: 'att-1',
          name: 'upload.png',
          mime: 'image/png',
          size: 4,
          path: '/tmp/upload.png',
          createdAt: '2026-04-10T00:00:00Z',
        },
      }, 201);
    }],
  ]);

  const result = await runCliCaptured(
    [
      'web-session',
      'attach',
      '--base-url',
      'http://127.0.0.1:3000',
      '--project-id',
      'p1',
      '--file',
      filePath,
    ],
    { fetchImpl: createFetchMock(handlers) },
  );

  assert.equal(result.exitCode, 0);
  const payload = JSON.parse(result.stdout);
  assert.equal(payload.id, 'att-1');
});

test('CLI web-session watch streams one NDJSON frame when max-events is set to 1', { concurrency: false }, async () => {
  FakeWebSocket.reset();
  FakeWebSocket.setFactory(socket => {
    queueMicrotask(() => {
      socket.open();
      socket.emitJson({
        v: 1,
        k: 'snap',
        sid: 'ws1',
        ts: 1710000400000,
        s: {
          id: 'ws1',
          pid: 'p1',
          oi: 1000,
          ag: 'codex',
          md: 'gpt-5',
          re: 'high',
          wm: 'plan',
          pl: 'elevated',
          ttl: 'watch',
          cwd: '/repo/demo',
          st: 'running',
          unr: false,
          act: 1710000400000,
          ca: 1710000400000,
          lu: 1710000400001,
          sk: 'codex_app_server',
          ss: 'fresh',
          usa: { in: 1, cin: 0, out: 1 },
          cost: 0.01,
          cws: 'default',
        },
        h: {
          its: [],
          hm: false,
          tot: 0,
        },
      });
    });
  });

  const result = await runCliCaptured([
    'web-session',
    'watch',
    '--base-url',
    'http://127.0.0.1:3000',
    '--session-id',
    'ws1',
    '--max-events',
    '1',
    '--raw',
  ]);

  assert.equal(result.exitCode, 0);
  const lines = result.stdout.trim().split('\n');
  assert.equal(lines.length, 1);
  const payload = JSON.parse(lines[0]);
  assert.equal(payload.k, 'snap');
  assert.equal(payload.sid, 'ws1');
});
