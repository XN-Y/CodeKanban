package ai_assistant2

import "testing"

func TestRenderLinesFromBuffer_PreservesSpacesFromCursorMoves(t *testing.T) {
	// Cursor moves can create blank cells without emitting literal spaces.
	// We still want the rendered line to preserve those spaces (at least up to trimming right).
	data := []byte("\x1b[HA\x1b[1CB")

	lines := RenderLinesFromBuffer(data, 1, 10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	want := "A B"
	if lines[0] != want {
		t.Fatalf("unexpected render: got %q want %q", lines[0], want)
	}
}

func TestRenderLinesFromBuffer_DoesNotInsertSpacesForWideChars(t *testing.T) {
	// Wide chars occupy 2 cells in the terminal grid (second one is AttrWideDummy),
	// but the rendered string should not contain an extra space for that dummy cell.
	data := []byte("\x1b[H你A")

	lines := RenderLinesFromBuffer(data, 1, 10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	want := "你A"
	if lines[0] != want {
		t.Fatalf("unexpected render: got %q want %q", lines[0], want)
	}
}
