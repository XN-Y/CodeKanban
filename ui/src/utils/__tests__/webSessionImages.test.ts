import { describe, expect, it } from 'vitest';

import {
  buildImagePlaceholderLine,
  stripImagePlaceholdersFromText,
} from '@/utils/webSessionImages';

describe('webSessionImages', () => {
  it('keeps text unchanged when there are no attachments', () => {
    const text = `Please inspect ${buildImagePlaceholderLine(1)}`;

    expect(stripImagePlaceholdersFromText(text, 0)).toBe(text);
  });

  it('removes inline image placeholders when attachments are present', () => {
    expect(stripImagePlaceholdersFromText('hello [Image #1]', 1)).toBe('hello');
    expect(stripImagePlaceholdersFromText('[Image #1] hello [Image #2]', 2)).toBe('hello');
  });

  it('preserves surrounding line structure while removing placeholders', () => {
    const text = 'first line\n[Image #1]\nsecond line';

    expect(stripImagePlaceholdersFromText(text, 1)).toBe('first line\nsecond line');
  });
});
