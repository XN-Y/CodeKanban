import { describe, expect, it } from 'vitest';

import { resolveWebSessionAttachmentPreviewMode } from '@/components/web-session/webSessionAttachmentPreview';

describe('webSessionAttachmentPreview', () => {
  it('uses hover preview on desktop for previewable attachments', () => {
    expect(
      resolveWebSessionAttachmentPreviewMode({
        previewable: true,
        isMobile: false,
      })
    ).toBe('popover');
  });

  it('opens the modal directly on mobile for previewable attachments', () => {
    expect(
      resolveWebSessionAttachmentPreviewMode({
        previewable: true,
        isMobile: true,
      })
    ).toBe('modal');
  });

  it('keeps non-previewable attachments static on every device', () => {
    expect(
      resolveWebSessionAttachmentPreviewMode({
        previewable: false,
        isMobile: false,
      })
    ).toBe('static');
    expect(
      resolveWebSessionAttachmentPreviewMode({
        previewable: false,
        isMobile: true,
      })
    ).toBe('static');
  });
});
