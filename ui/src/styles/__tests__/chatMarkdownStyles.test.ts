import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

const chatMarkdownCssPath = fileURLToPath(new URL('../chat-markdown.css', import.meta.url));
const chatMarkdownCss = readFileSync(chatMarkdownCssPath, 'utf8');

describe('chat markdown styles', () => {
  it('restores standard ordered and unordered list markers', () => {
    expect(chatMarkdownCss).toMatch(
      /\.chat-markdown ul\s*\{[^}]*list-style-position:\s*outside;[^}]*list-style-type:\s*disc;/s
    );
    expect(chatMarkdownCss).toMatch(
      /\.chat-markdown ol\s*\{[^}]*list-style-position:\s*outside;[^}]*list-style-type:\s*decimal;/s
    );
  });

  it('keeps task list markers disabled', () => {
    expect(chatMarkdownCss).toMatch(
      /\.chat-markdown ul\.contains-task-list\s*\{[^}]*list-style:\s*none;/s
    );
    expect(chatMarkdownCss).toMatch(
      /\.chat-markdown \.task-list-item\s*\{[^}]*list-style:\s*none;/s
    );
  });

  it('defines nested marker styles for readability', () => {
    expect(chatMarkdownCss).toMatch(/\.chat-markdown ul ul\s*\{[^}]*list-style-type:\s*circle;/s);
    expect(chatMarkdownCss).toMatch(
      /\.chat-markdown ul ul ul\s*\{[^}]*list-style-type:\s*square;/s
    );
    expect(chatMarkdownCss).toMatch(
      /\.chat-markdown ol ol\s*\{[^}]*list-style-type:\s*lower-alpha;/s
    );
    expect(chatMarkdownCss).toMatch(
      /\.chat-markdown ol ol ol\s*\{[^}]*list-style-type:\s*lower-roman;/s
    );
  });
});
