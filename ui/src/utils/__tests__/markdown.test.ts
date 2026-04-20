import { describe, expect, it } from 'vitest';

import { renderHighlightedCodeBlock, renderMarkdown } from '@/utils/markdown';

describe('renderMarkdown', () => {
  it('highlights fenced code blocks by default', () => {
    const html = renderMarkdown('```go\nfmt.Println("hi")\n```');

    expect(html).toContain('class="hljs language-go"');
    expect(html).toContain('data-language="go"');
  });

  it('can skip code highlighting for streaming renders', () => {
    const html = renderMarkdown('```html\n<div class="box">\n```', {
      disableCodeHighlight: true,
    });

    expect(html).toContain('class="hljs"');
    expect(html).toContain('data-language="html"');
    expect(html).not.toContain('language-xml');
    expect(html).not.toContain('<span class="hljs-');
    expect(html).toContain('&lt;div class=&quot;box&quot;&gt;');
  });

  it('adds a code-block copy button only when enabled', () => {
    const enabledHtml = renderMarkdown('```bash\necho "hi"\n```', {
      enableCodeBlockCopy: true,
      codeBlockCopyLabel: 'copy',
    });
    const disabledHtml = renderMarkdown('```bash\necho "hi"\n```');

    expect(enabledHtml).toContain('data-message-code-copy="true"');
    expect(enabledHtml).toContain('class="markdown-code-copy-button"');
    expect(enabledHtml).toContain('>copy</button>');
    expect(enabledHtml).toContain('data-code-copy="true"');
    expect(disabledHtml).not.toContain('data-message-code-copy="true"');
  });

  it('keeps link rendering intact when highlighting is disabled', () => {
    const html = renderMarkdown('[docs](https://example.com)', {
      disableCodeHighlight: true,
    });

    expect(html).toContain('href="https://example.com"');
    expect(html).toContain('data-message-link="true"');
    expect(html).toContain('target="_blank"');
  });

  it('adds a copy button only for absolute http and https links when enabled', () => {
    const html = renderMarkdown(
      [
        '[http](http://10.128.128.111:6032/)',
        '[https](https://example.com/docs)',
        '[relative](/docs/getting-started)',
      ].join('\n\n'),
      {
        enableLinkCopy: true,
        linkCopyLabel: 'Copy link',
      }
    );

    expect(html).toContain('data-message-link-copy="true"');
    expect(html).toContain('data-message-link-copy-href="http://10.128.128.111:6032/"');
    expect(html).toContain('data-message-link-copy-href="https://example.com/docs"');
    expect(html).not.toContain('data-message-link-copy-href="/docs/getting-started"');
  });

  it('does not add copy buttons when link copy is disabled', () => {
    const html = renderMarkdown('[http](http://10.128.128.111:6032/)');

    expect(html).not.toContain('data-message-link-copy="true"');
  });

  it('preserves ordered, unordered, and nested list structure', () => {
    const html = renderMarkdown(
      [
        '1. first',
        '2. second',
        '   - child',
        '   - child two',
        '3. third',
        '   1. nested one',
        '   2. nested two',
      ].join('\n')
    );

    expect(html).toContain('<ol>');
    expect(html).toContain('<ul>');
    expect(html).toContain('<li>first</li>');
    expect(html).toContain('<li>child</li>');
    expect(html).toContain('<li>nested one</li>');
  });

  it('renders standalone highlighted diff blocks', () => {
    const html = renderHighlightedCodeBlock('@@ -1 +1 @@\n-old\n+new\n', 'diff');

    expect(html).toContain('class="hljs language-diff"');
    expect(html).toContain('hljs-addition');
    expect(html).toContain('hljs-deletion');
  });
});
