package websession

import "testing"

func TestComposeUserMessageText(t *testing.T) {
	t.Run("appends placeholders after user text", func(t *testing.T) {
		got := composeUserMessageText("Review these screenshots", 2)
		want := "Review these screenshots\n\n[Image #1] [Image #2]"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("keeps image only messages as placeholder text", func(t *testing.T) {
		got := composeUserMessageText("", 1)
		if got != "[Image #1]" {
			t.Fatalf("expected image-only placeholder, got %q", got)
		}
	})

	t.Run("rebuilds managed placeholders from mixed input", func(t *testing.T) {
		got := composeUserMessageText("Review\n\n[Image #1] extra context", 2)
		want := "Review\n\nextra context\n\n[Image #1] [Image #2]"
		if got != want {
			t.Fatalf("expected rebuilt placeholder block %q, got %q", want, got)
		}
	})
}

func TestNormalizeAttachmentDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		index    int
		expected string
	}{
		{name: "empty name", input: "", index: 1, expected: "image 1"},
		{name: "plain image name", input: "image.png", index: 2, expected: "image 2"},
		{name: "blob name", input: "blob", index: 3, expected: "image 3"},
		{name: "clipboard image name", input: "clipboard-image.png", index: 4, expected: "image 4"},
		{name: "pasted image name", input: "pasted-image-20260409-101010.png", index: 5, expected: "image 5"},
		{name: "preserves real file names", input: "diagram-final.png", index: 6, expected: "diagram-final.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAttachmentDisplayName(tt.input, tt.index); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
