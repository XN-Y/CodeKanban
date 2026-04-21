export type WebSessionAttachmentPreviewMode = 'static' | 'modal' | 'popover';

export function resolveWebSessionAttachmentPreviewMode(options: {
  previewable: boolean;
  isMobile: boolean;
}): WebSessionAttachmentPreviewMode {
  if (!options.previewable) {
    return 'static';
  }

  return options.isMobile ? 'modal' : 'popover';
}
