import path from 'node:path';

import { CodeKanbanConfigError, CodeKanbanValidationError } from './errors.js';

function isWindowsAbsolutePath(value) {
  return /^[a-zA-Z]:[\\/]/.test(value) || value.startsWith('\\\\');
}

function isPosixAbsolutePath(value) {
  return value.startsWith('/');
}

export function normalizeBaseUrl(baseURL) {
  const value = String(baseURL || '').trim();
  if (!value) {
    throw new CodeKanbanConfigError('baseURL is required');
  }
  const normalized = value.endsWith('/') ? value : `${value}/`;
  let url;
  try {
    url = new URL(normalized);
  } catch (error) {
    throw new CodeKanbanConfigError(`invalid baseURL: ${value}`, { cause: error });
  }
  if (!['http:', 'https:'].includes(url.protocol)) {
    throw new CodeKanbanConfigError(`unsupported baseURL protocol: ${url.protocol}`);
  }
  return url;
}

export function ensureString(value, fieldName) {
  const normalized = typeof value === 'string' ? value.trim() : '';
  if (!normalized) {
    throw new CodeKanbanValidationError(`${fieldName} is required`);
  }
  return normalized;
}

export function ensureOptionalString(value) {
  return typeof value === 'string' ? value.trim() : '';
}

export function ensureArrayOfStrings(value, fieldName) {
  if (value == null) {
    return [];
  }
  if (!Array.isArray(value)) {
    throw new CodeKanbanValidationError(`${fieldName} must be an array of strings`);
  }
  return value
    .map(item => (typeof item === 'string' ? item.trim() : ''))
    .filter(Boolean);
}

export function normalizeFsPath(inputPath) {
  const value = ensureString(inputPath, 'path');
  if (isWindowsAbsolutePath(value)) {
    return path.win32.normalize(value).toLowerCase();
  }
  if (isPosixAbsolutePath(value)) {
    return path.posix.normalize(value);
  }
  const resolved = path.resolve(value);
  const normalized = path.normalize(resolved);
  return process.platform === 'win32' ? normalized.toLowerCase() : normalized;
}

export function pathBasename(inputPath) {
  const value = String(inputPath || '').trim();
  if (!value) {
    return 'project';
  }
  const base = isWindowsAbsolutePath(value)
    ? path.win32.basename(path.win32.normalize(value))
    : isPosixAbsolutePath(value)
      ? path.posix.basename(path.posix.normalize(value))
      : path.basename(path.resolve(value));
  return base || 'project';
}

export function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

export function toWsUrl(baseURL, wsPath) {
  const base = normalizeBaseUrl(baseURL);
  const wsBase = new URL(base.toString());
  wsBase.protocol = base.protocol === 'https:' ? 'wss:' : 'ws:';
  return new URL(wsPath, wsBase).toString();
}

export function normalizeTerminalEnter(value) {
  const trimmed = String(value || '').replace(/\s+$/, '');
  if (!trimmed) {
    return '';
  }
  if (trimmed.endsWith('\n') || trimmed.endsWith('\r')) {
    return trimmed;
  }
  return `${trimmed}\r`;
}

export function shellQuote(arg) {
  const value = String(arg ?? '');
  if (!value) {
    return '""';
  }
  if (/^[A-Za-z0-9_./:\\=-]+$/.test(value)) {
    return value;
  }
  return `"${value.replace(/"/g, '\\"')}"`;
}

export function toCommandString(argv) {
  return argv.map(shellQuote).join(' ');
}

export function createJsonOutput(value) {
  return `${JSON.stringify(value, null, 2)}\n`;
}

