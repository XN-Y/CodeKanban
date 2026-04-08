const IMAGE_PLACEHOLDER_TOKEN_PATTERN = /\[Image #\d+\]/g;
const IMAGE_PLACEHOLDER_LINE_PATTERN = /(^|\n)\s*(?:\[Image #\d+\]\s*)+(?=\n|$)/g;

export function buildImagePlaceholder(index: number) {
  return `[Image #${index}]`;
}

export function buildImagePlaceholderLine(count: number) {
  if (count <= 0) {
    return '';
  }

  return Array.from({ length: count }, (_, index) => buildImagePlaceholder(index + 1)).join(' ');
}

export function stripManagedComposerImagePlaceholders(value: string) {
  if (!value) {
    return '';
  }

  let normalized = value.replace(/\r\n/g, '\n');
  normalized = normalized.replace(IMAGE_PLACEHOLDER_LINE_PATTERN, '\n');
  normalized = normalized
    .split('\n')
    .map(line => {
      if (!/\[Image #\d+\]/.test(line)) {
        return line.replace(/[ \t]+$/g, '');
      }

      return line
        .replace(IMAGE_PLACEHOLDER_TOKEN_PATTERN, ' ')
        .trim()
        .replace(/[ \t]{2,}/g, ' ');
    })
    .join('\n');
  normalized = normalized.replace(/[ \t]+\n/g, '\n');
  normalized = normalized.replace(/\n{3,}/g, '\n\n');
  return normalized.trim();
}

export function buildComposerTextWithImagePlaceholders(text: string, attachmentCount: number) {
  const placeholderLine = buildImagePlaceholderLine(attachmentCount);
  if (!placeholderLine) {
    return text;
  }

  const trimmedText = text.replace(/\s+$/g, '');
  if (!trimmedText) {
    return placeholderLine;
  }
  return `${trimmedText}\n\n${placeholderLine}`;
}

export function isGenericImageAttachmentName(name: string) {
  const normalized = String(name || '')
    .trim()
    .split(/[\\/]/)
    .pop()
    ?.toLowerCase();
  if (!normalized) {
    return true;
  }

  const extensionIndex = normalized.lastIndexOf('.');
  const stem = extensionIndex > 0 ? normalized.slice(0, extensionIndex) : normalized;
  if (stem === 'image' || stem === 'blob' || stem === 'clipboard-image') {
    return true;
  }
  return stem.startsWith('pasted-image-');
}

export function resolveImageAttachmentDisplayName(name: string, index: number) {
  const trimmed = String(name || '')
    .trim()
    .split(/[\\/]/)
    .pop();
  if (!trimmed || isGenericImageAttachmentName(trimmed)) {
    return `image ${index}`;
  }
  return trimmed;
}
