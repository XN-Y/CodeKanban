package codex

import (
	"testing"
	"time"

	"code-kanban/utils/ai_assistant2/types"
)

func TestWorkingLineDetection_WithCursorMovedSpaces(t *testing.T) {
	d := NewStatusDetector()

	cases := []struct {
		name string
		line string
		yes  bool
	}{
		{"mcpStartNoiseProgress", "\u2022 Starting MCP servers (3/4): foo (65s \u2022 esc to interrupt)", false},
		// TODO: We have not confirmed whether compact Codex frames like "◦ Working"
		// and "◦ Working  1" should still count as working with the current CLI UI.
		// Re-enable once we capture real terminal output that proves the expected state.
		// {"compactWorking", "◦ Working", true},
		// {"compactWorkingWithSeconds", "◦ Working  1", true},
		{"compactWorkingNotStatus", "• Working on it", false},
		{"normal", "• Working (65s • esc to interrupt)", true},
		{"noLeadingSpace", "•Working (65s • esc to interrupt)", true},
		{"missingClosingParen", "• Working (65s • esc to interrupt", true},
		{"truncatedT", "• Working (65s • esc to interrup", true},
		{"mcpStartNoise", "• Working (65s • esc to interrupt) Starting MCP servers", false},
		{"notWorking", "› Explain this codebase", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := d.isWorkingLine(tc.line)
			if got != tc.yes {
				t.Fatalf("isWorkingLine(%q)=%v want %v", tc.line, got, tc.yes)
			}
		})
	}
}

func TestDetectStateFromLines_WaitingApproval(t *testing.T) {
	d := NewStatusDetector()

	lines := []string{
		"\u276F 1. Approve",
		"\u203A 2. Cancel",
		"  Press enter to confirm or esc to cancel",
	}

	state, ok := d.DetectStateFromLines(lines, nil, 80, time.Now(), types.StateWaitingInput, time.Time{}, 0, 0)
	if !ok {
		t.Fatalf("DetectStateFromLines() ok=false want true")
	}
	if state != types.StateWaitingApproval {
		t.Fatalf("DetectStateFromLines()=%s want %s", state, types.StateWaitingApproval)
	}
}
