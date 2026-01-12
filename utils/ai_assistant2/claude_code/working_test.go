package claude_code

import (
	"strings"
	"testing"
	"time"

	"code-kanban/utils/ai_assistant2/types"
)

func TestDetectStateFromLines_WorkingWithUnrecognizedHintLine(t *testing.T) {
	d := NewStatusDetector()

	cols := 40
	sep := strings.Repeat("─", cols)

	lines := []string{
		"Some previous output",
		"✶ Processing request (esc to interrupt · ctrl+t to show todos · 1m 2s · ↑ 123 tokens)",
		"  ⎿  Hint: Something that is not Tip/Next",
		sep,
		"> ",
		sep,
		"  ⏵⏵ bypass permissions on (shift+tab to cycle)",
	}

	state, _ := d.DetectStateFromLines(lines, nil, cols, time.Now(), types.StateWaitingInput, time.Time{}, 0, 0)
	if state != types.StateWorking {
		t.Fatalf("DetectStateFromLines()=%s want %s", state, types.StateWorking)
	}
}

func TestIsWorkingTaskLine_AllowsNoEllipsis(t *testing.T) {
	d := NewStatusDetector()

	line := "✶ Processing request (esc to interrupt · ctrl+t to show todos · 1m 2s · ↑ 123 tokens)"
	if !d.isWorkingTaskLine(line) {
		t.Fatalf("isWorkingTaskLine()=false want true")
	}
}

func TestDetectStateFromLines_FallbackWorkingWithoutSeparators(t *testing.T) {
	d := NewStatusDetector()

	lines := []string{
		"✶ Processing request… (esc to interrupt · ctrl+t to show todos · 1m 2s · ↑ 123 tokens)",
	}

	state, _ := d.DetectStateFromLines(lines, nil, 80, time.Now(), types.StateWaitingInput, time.Time{}, 0, 0)
	if state != types.StateWorking {
		t.Fatalf("DetectStateFromLines()=%s want %s", state, types.StateWorking)
	}
}
