import assert from 'node:assert/strict';

export function createJsonResponse(payload, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    async text() {
      return JSON.stringify(payload);
    },
  };
}

export function createFetchMock(handlers) {
  return async (input, init = {}) => {
    const url = input instanceof URL ? input : new URL(String(input));
    const method = (init.method || 'GET').toUpperCase();
    const key = `${method} ${url.pathname}`;
    const handler = handlers.get(key);
    assert.ok(handler, `unexpected request: ${key}`);

    let parsedBody = init.body;
    if (typeof init.body === 'string') {
      parsedBody = JSON.parse(init.body);
    }

    return handler({
      url,
      method,
      body: parsedBody,
      headers: init.headers || {},
    });
  };
}

export class FakeWebSocket {
  static instances = [];
  static factory = null;

  static reset() {
    FakeWebSocket.instances.length = 0;
    FakeWebSocket.factory = null;
  }

  static setFactory(factory) {
    FakeWebSocket.factory = factory;
  }

  constructor(url, options) {
    this.url = url;
    this.options = options || null;
    this.readyState = 0;
    this.listeners = new Map();
    this.sent = [];
    FakeWebSocket.instances.push(this);
    FakeWebSocket.factory?.(this, url);
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

  open() {
    this.readyState = 1;
    this.emit('open', { type: 'open' });
  }

  emitJson(payload) {
    this.emit('message', { data: JSON.stringify(payload) });
  }

  emitRaw(data) {
    this.emit('message', { data });
  }

  send(payload) {
    this.sent.push(JSON.parse(payload));
  }

  close(event = { type: 'close', code: 1000, reason: '', wasClean: true }) {
    this.readyState = 3;
    this.emit('close', event);
  }
}
