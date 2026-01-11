package codex

import (
	"testing"
)

func TestWorkingLineDetection_WithCursorMovedSpaces(t *testing.T) {
	d := NewStatusDetector()

	cases := []struct {
		name string
		line string
		yes  bool
	}{
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
