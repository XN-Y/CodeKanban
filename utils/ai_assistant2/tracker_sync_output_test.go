package ai_assistant2

import (
	"testing"

	"code-kanban/utils/ai_assistant2/types"
)

func visibleLines(tracker *StatusTracker) []string {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	lines, _ := getVisibleLinesLocked(tracker)
	return lines
}

func TestStatusTracker_SyncOutput_DelaysUpdateUntilCommit(t *testing.T) {
	tracker := NewStatusTracker()
	tracker.SetTerminalSize(5, 40)

	_, _, _ = tracker.ProcessChunk([]byte("Hello"))
	lines := visibleLines(tracker)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty lines")
	}
	if lines[0] != "Hello" {
		t.Fatalf("expected first line to be %q, got %q (all lines: %v)", "Hello", lines[0], lines)
	}

	_, _, _ = tracker.ProcessChunk([]byte("\x1b[?2026l\x1b[2J\x1b[HWorking"))
	lines = visibleLines(tracker)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty lines")
	}
	if lines[0] != "Hello" {
		t.Fatalf("expected first line to remain %q before commit, got %q (all lines: %v)", "Hello", lines[0], lines)
	}

	_, _, _ = tracker.ProcessChunk([]byte("\x1b[?2026h"))
	lines = visibleLines(tracker)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty lines")
	}
	if lines[0] != "Working" {
		t.Fatalf("expected first line to be %q after commit, got %q (all lines: %v)", "Working", lines[0], lines)
	}
}

func TestStatusTracker_SyncOutput_SupportsSplitSequences(t *testing.T) {
	tracker := NewStatusTracker()
	tracker.SetTerminalSize(5, 40)

	_, _, _ = tracker.ProcessChunk([]byte("Hello"))
	lines := visibleLines(tracker)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty lines")
	}
	if lines[0] != "Hello" {
		t.Fatalf("expected first line to be %q, got %q (all lines: %v)", "Hello", lines[0], lines)
	}

	// Split "\x1b[?2026l" across chunks.
	_, _, _ = tracker.ProcessChunk([]byte("\x1b[?202"))
	_, _, _ = tracker.ProcessChunk([]byte("6l\x1b[2J\x1b[HWorking"))
	lines = visibleLines(tracker)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty lines")
	}
	if lines[0] != "Hello" {
		t.Fatalf("expected first line to remain %q before commit, got %q (all lines: %v)", "Hello", lines[0], lines)
	}

	_, _, _ = tracker.ProcessChunk([]byte("\x1b[?2026h"))
	lines = visibleLines(tracker)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty lines")
	}
	if lines[0] != "Working" {
		t.Fatalf("expected first line to be %q after commit, got %q (all lines: %v)", "Working", lines[0], lines)
	}
}

func TestStatusTracker_SyncOutput_ForcesDetectionOnCommit(t *testing.T) {
	tracker := NewStatusTracker()
	tracker.SetTerminalSize(10, 120)
	tracker.Activate(types.AssistantTypeCodex, 10, 120)

	workingLine := "◦ Working (1s • esc to interrupt)"

	// Begin sync output and write the working line, but do not commit yet.
	_, _, _ = tracker.ProcessChunk([]byte("\x1b[?2026l\x1b[2J\x1b[H" + workingLine))
	state, _ := tracker.State()
	if state != types.StateWaitingInput {
		t.Fatalf("expected state to remain %q before commit, got %q", types.StateWaitingInput, state)
	}

	// Commit should bypass throttling and immediately detect the state change.
	newState, _, changed := tracker.ProcessChunk([]byte("\x1b[?2026h"))
	if !changed || newState != types.StateWorking {
		t.Fatalf("expected commit to detect working (%v), got state=%q changed=%v", types.StateWorking, newState, changed)
	}
}
