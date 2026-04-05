import { CodeKanbanConfigError, CodeKanbanError, CodeKanbanValidationError } from './errors.js';

function decodeJsonMessage(raw) {
  const text = typeof raw === 'string' ? raw : String(raw ?? '');
  return JSON.parse(text);
}

export class TerminalConnection {
  constructor({ sessionId, url, WebSocketImpl }) {
    if (!sessionId) {
      throw new CodeKanbanValidationError('sessionId is required');
    }
    if (!url) {
      throw new CodeKanbanValidationError('url is required');
    }
    const Socket = WebSocketImpl || globalThis.WebSocket;
    if (!Socket) {
      throw new CodeKanbanConfigError('WebSocket implementation is unavailable');
    }

    this.sessionId = sessionId;
    this.url = url;
    this.messages = [];
    this.lastMetadata = undefined;
    this.readyMessage = undefined;
    this._listeners = new Map();

    this.socket = new Socket(url);
    this._readyResolve = null;
    this._readyReject = null;
    this._readyPromise = new Promise((resolve, reject) => {
      this._readyResolve = resolve;
      this._readyReject = reject;
    });

    this.socket.addEventListener('message', event => {
      const payload = decodeJsonMessage(event.data);
      this.messages.push(payload);
      if (payload.type === 'metadata' && payload.metadata) {
        this.lastMetadata = payload.metadata;
      }
      if (payload.type === 'ready') {
        this.readyMessage = payload;
        this._readyResolve?.(payload);
      }
      this._emit('message', payload);
      this._emit(payload.type, payload);
    });

    this.socket.addEventListener('error', event => {
      const error = new CodeKanbanError('terminal websocket error', { event });
      this._readyReject?.(error);
      this._emit('error', error);
    });

    this.socket.addEventListener('close', event => {
      this._emit('close', event);
    });
  }

  on(type, handler) {
    const existing = this._listeners.get(type) || new Set();
    existing.add(handler);
    this._listeners.set(type, existing);
    return () => this.off(type, handler);
  }

  off(type, handler) {
    const existing = this._listeners.get(type);
    if (!existing) {
      return;
    }
    existing.delete(handler);
  }

  _emit(type, payload) {
    const existing = this._listeners.get(type);
    if (!existing) {
      return;
    }
    for (const handler of existing) {
      handler(payload);
    }
  }

  async waitForReady(timeoutMs = 8000) {
    if (this.readyMessage) {
      return this.readyMessage;
    }
    return await Promise.race([
      this._readyPromise,
      new Promise((_, reject) => {
        setTimeout(
          () => reject(new CodeKanbanValidationError(`terminal session ${this.sessionId} did not become ready in time`)),
          timeoutMs,
        );
      }),
    ]);
  }

  sendJson(payload) {
    this.socket.send(JSON.stringify(payload));
  }

  sendInput(data) {
    this.sendJson({ type: 'input', data });
  }

  resize(cols, rows) {
    this.sendJson({ type: 'resize', cols, rows });
  }

  close() {
    this.sendJson({ type: 'close' });
    this.socket.close();
  }

  async waitForMetadata(timeoutMs = 1500) {
    if (this.lastMetadata) {
      return this.lastMetadata;
    }
    return await new Promise(resolve => {
      const cleanup = this.on('metadata', payload => {
        cleanup();
        resolve(payload.metadata);
      });
      setTimeout(() => {
        cleanup();
        resolve(this.lastMetadata);
      }, timeoutMs);
    });
  }
}
