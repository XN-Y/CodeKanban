export class CodeKanbanError extends Error {
  constructor(message, options = {}) {
    super(message);
    this.name = options.name || this.constructor.name;
    if (options.cause) {
      this.cause = options.cause;
    }
    for (const [key, value] of Object.entries(options)) {
      if (key === 'name' || key === 'cause') {
        continue;
      }
      this[key] = value;
    }
  }
}

export class CodeKanbanConfigError extends CodeKanbanError {}
export class CodeKanbanValidationError extends CodeKanbanError {}

export class CodeKanbanHttpError extends CodeKanbanError {
  constructor(message, options = {}) {
    super(message, { ...options, name: 'CodeKanbanHttpError' });
    this.status = options.status ?? 500;
    this.method = options.method ?? 'GET';
    this.path = options.path ?? '';
    this.body = options.body;
  }
}
