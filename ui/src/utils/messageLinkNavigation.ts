const ALLOWED_PROTOCOLS = new Set(['http:', 'https:']);

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
