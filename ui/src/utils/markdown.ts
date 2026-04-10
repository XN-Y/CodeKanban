import hljs from 'highlight.js/lib/core';
import bash from 'highlight.js/lib/languages/bash';
import c from 'highlight.js/lib/languages/c';
import cpp from 'highlight.js/lib/languages/cpp';
import css from 'highlight.js/lib/languages/css';
import diff from 'highlight.js/lib/languages/diff';
import dockerfile from 'highlight.js/lib/languages/dockerfile';
import go from 'highlight.js/lib/languages/go';
import ini from 'highlight.js/lib/languages/ini';
import java from 'highlight.js/lib/languages/java';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import makefile from 'highlight.js/lib/languages/makefile';
import markdown from 'highlight.js/lib/languages/markdown';
import python from 'highlight.js/lib/languages/python';
import rust from 'highlight.js/lib/languages/rust';
import scss from 'highlight.js/lib/languages/scss';
import sql from 'highlight.js/lib/languages/sql';
import typescript from 'highlight.js/lib/languages/typescript';
import xml from 'highlight.js/lib/languages/xml';
import yaml from 'highlight.js/lib/languages/yaml';
import { Marked, type Tokens } from 'marked';

type HljsLanguageModule = Parameters<typeof hljs.registerLanguage>[1];

const registeredLanguages = new Set<string>();

function registerLanguage(name: string, language: HljsLanguageModule) {
  hljs.registerLanguage(name, language);
  registeredLanguages.add(name);
}

registerLanguage('bash', bash);
registerLanguage('c', c);
registerLanguage('cpp', cpp);
registerLanguage('css', css);
registerLanguage('diff', diff);
registerLanguage('dockerfile', dockerfile);
registerLanguage('go', go);
registerLanguage('ini', ini);
registerLanguage('java', java);
registerLanguage('javascript', javascript);
registerLanguage('json', json);
registerLanguage('makefile', makefile);
registerLanguage('markdown', markdown);
registerLanguage('python', python);
registerLanguage('rust', rust);
registerLanguage('scss', scss);
registerLanguage('sql', sql);
registerLanguage('typescript', typescript);
registerLanguage('xml', xml);
registerLanguage('yaml', yaml);

const languageAliases: Record<string, string> = {
  cc: 'cpp',
  cjs: 'javascript',
  conf: 'ini',
  cxx: 'cpp',
  docker: 'dockerfile',
  env: 'ini',
  h: 'c',
  hpp: 'cpp',
  htm: 'xml',
  html: 'xml',
  js: 'javascript',
  jsx: 'javascript',
  md: 'markdown',
  mjs: 'javascript',
  mts: 'typescript',
  properties: 'ini',
  py: 'python',
  rs: 'rust',
  sass: 'scss',
  sh: 'bash',
  shell: 'bash',
  svg: 'xml',
  ts: 'typescript',
  tsx: 'typescript',
  toml: 'ini',
  vue: 'xml',
  xhtml: 'xml',
  xml: 'xml',
  yml: 'yaml',
  zsh: 'bash',
};

function escapeHtml(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function pickLanguageName(value?: string) {
  if (!value) {
    return '';
  }
  return value.trim().toLowerCase().split(/\s+/, 1)[0] ?? '';
}

function normalizeLanguage(value?: string) {
  const rawLanguage = pickLanguageName(value);
  if (!rawLanguage) {
    return '';
  }

  const normalized = languageAliases[rawLanguage] ?? rawLanguage;
  return registeredLanguages.has(normalized) ? normalized : '';
}

function renderCodeBlock({ text, lang }: Tokens.Code) {
  const normalizedLanguage = normalizeLanguage(lang);
  const languageLabel = pickLanguageName(lang);
  const highlightedCode = normalizedLanguage
    ? hljs.highlight(text, {
        language: normalizedLanguage,
        ignoreIllegals: true,
      }).value
    : escapeHtml(text);

  const dataLanguage = languageLabel ? ` data-language="${escapeHtml(languageLabel)}"` : '';
  const languageClass = normalizedLanguage ? ` language-${normalizedLanguage}` : '';

  return `<pre class="markdown-code-block"${dataLanguage}><code class="hljs${languageClass}">${highlightedCode}</code></pre>`;
}

const markdownRenderer = new Marked({
  async: false,
  breaks: true,
  gfm: true,
});

markdownRenderer.use({
  renderer: {
    code(token) {
      return renderCodeBlock(token);
    },
    link(token) {
      const href = token.href ? ` href="${escapeHtml(token.href)}"` : '';
      const title = token.title ? ` title="${escapeHtml(token.title)}"` : '';
      const text = this.parser.parseInline(token.tokens);
      return `<a${href}${title} target="_blank" rel="noopener noreferrer" data-message-link="true">${text}</a>`;
    },
  },
});

export function renderMarkdown(value: string) {
  if (!value) {
    return '';
  }

  try {
    return markdownRenderer.parse(value) as string;
  } catch {
    return escapeHtml(value).replace(/\n/g, '<br>');
  }
}
