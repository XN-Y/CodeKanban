const ALLOWED_PROTOCOLS = new Set(['http:', 'https:']);
const MESSAGE_LINK_COPY_SELECTOR = '[data-message-link-copy="true"]';
const MESSAGE_CODE_COPY_SELECTOR = '[data-message-code-copy="true"]';
const MESSAGE_LINK_SELECTOR = 'a[href]';

export function resolveNavigableHref(rawHref: string, baseHref: string): string | null {
  const href = rawHref.trim();
  if (!href || href.startsWith('#')) {
    return null;
  }

  try {
    const resolved = new URL(href, baseHref);
    if (!ALLOWED_PROTOCOLS.has(resolved.protocol)) {
      return null;
    }
    return resolved.toString();
  } catch {
    return null;
  }
}

export function resolveCopyableAbsoluteHref(rawHref: string): string | null {
  const href = rawHref.trim();
  if (!href) {
    return null;
  }

  try {
    const resolved = new URL(href);
    if (!ALLOWED_PROTOCOLS.has(resolved.protocol)) {
      return null;
    }
    return resolved.toString();
  } catch {
    return null;
  }
}

function isMarkdownLinkTarget(
  target: EventTarget | null,
  currentTarget: EventTarget | null
): target is Element & Node {
  return (
    target instanceof Element &&
    currentTarget instanceof HTMLElement &&
    currentTarget.contains(target) &&
    !!target.closest('.chat-markdown')
  );
}

export function getClickedMarkdownCodeCopyText(
  target: EventTarget | null,
  currentTarget: EventTarget | null
): string | null {
  if (!isMarkdownLinkTarget(target, currentTarget)) {
    return null;
  }

  const button = target.closest(MESSAGE_CODE_COPY_SELECTOR);
  if (!(button instanceof HTMLElement)) {
    return null;
  }

  const codeElement = button.closest('pre.markdown-code-block')?.querySelector('code');
  if (!(codeElement instanceof HTMLElement)) {
    return null;
  }

  const text = codeElement.textContent?.replace(/\n$/, '') ?? '';
  return text.trim() ? text : null;
}

export function getClickedMarkdownLink(
  target: EventTarget | null,
  currentTarget: EventTarget | null
): HTMLAnchorElement | null {
  if (!isMarkdownLinkTarget(target, currentTarget)) {
    return null;
  }

  const anchor = target.closest(MESSAGE_LINK_SELECTOR);
  return anchor instanceof HTMLAnchorElement ? anchor : null;
}

export function getClickedMarkdownLinkCopyHref(
  target: EventTarget | null,
  currentTarget: EventTarget | null
): string | null {
  if (!isMarkdownLinkTarget(target, currentTarget)) {
    return null;
  }

  const button = target.closest(MESSAGE_LINK_COPY_SELECTOR);
  if (!(button instanceof HTMLElement)) {
    return null;
  }

  return resolveCopyableAbsoluteHref(button.getAttribute('data-message-link-copy-href') ?? '');
}
