import { describe, expect, it } from 'vitest';

import { renderMarkdown } from '@/utils/markdown';

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

  it('keeps link rendering intact when highlighting is disabled', () => {
    const html = renderMarkdown('[docs](https://example.com)', {
      disableCodeHighlight: true,
    });

    expect(html).toContain('href="https://example.com"');
    expect(html).toContain('data-message-link="true"');
    expect(html).toContain('target="_blank"');
  });
});
