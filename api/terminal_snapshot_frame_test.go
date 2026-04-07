package api

import (
	"encoding/binary"
	"testing"
	"time"

	"code-kanban/service/terminal"
)

func TestTerminalMirrorSenderStateFirstFrameUsesFullSnapshot(t *testing.T) {
	state := newTerminalMirrorSenderState()
	frame, sent, err := state.EncodeFrame(
		testTerminalMirrorSnapshot(10, "hello", "world"),
		false,
		false,
	)
	if err != nil {
		t.Fatalf("EncodeFrame returned error: %v", err)
	}
	if !sent {
		t.Fatal("expected first frame to be sent")
	}
	assertSnapshotFrameHeader(t, frame, terminalSnapshotFrameKindFull, 1, 0)
}

func TestTerminalMirrorSenderStateUsesDeltaForSmallRowChanges(t *testing.T) {
	state := newTerminalMirrorSenderState()
	if _, sent, err := state.EncodeFrame(testTerminalMirrorSnapshot(10, "hello", "world"), false, false); err != nil || !sent {
		t.Fatalf("failed to prime baseline: sent=%v err=%v", sent, err)
	}

	frame, sent, err := state.EncodeFrame(
		testTerminalMirrorSnapshot(10, "hello", "there"),
		false,
		false,
	)
	if err != nil {
		t.Fatalf("EncodeFrame returned error: %v", err)
	}
	if !sent {
		t.Fatal("expected delta frame to be sent")
	}
	assertSnapshotFrameHeader(t, frame, terminalSnapshotFrameKindDelta, 2, 1)
}

func TestTerminalMirrorSenderStateFallsBackToFullWhenResizeChangesShape(t *testing.T) {
	state := newTerminalMirrorSenderState()
	if _, sent, err := state.EncodeFrame(testTerminalMirrorSnapshot(10, "hello", "world"), false, false); err != nil || !sent {
		t.Fatalf("failed to prime baseline: sent=%v err=%v", sent, err)
	}

	frame, sent, err := state.EncodeFrame(
		testTerminalMirrorSnapshot(10, "hello", "world", "again"),
		false,
		false,
	)
	if err != nil {
		t.Fatalf("EncodeFrame returned error: %v", err)
	}
	if !sent {
		t.Fatal("expected resized snapshot to be sent")
	}
	assertSnapshotFrameHeader(t, frame, terminalSnapshotFrameKindFull, 2, 0)
}

func TestTerminalMirrorSenderStateFallsBackToFullWhenDeltaIsLarger(t *testing.T) {
	state := newTerminalMirrorSenderState()
	if _, sent, err := state.EncodeFrame(testTerminalMirrorSnapshot(1, "a"), false, false); err != nil || !sent {
		t.Fatalf("failed to prime baseline: sent=%v err=%v", sent, err)
	}

	frame, sent, err := state.EncodeFrame(testTerminalMirrorSnapshot(1, "b"), false, false)
	if err != nil {
		t.Fatalf("EncodeFrame returned error: %v", err)
	}
	if !sent {
		t.Fatal("expected changed snapshot to be sent")
	}
	assertSnapshotFrameHeader(t, frame, terminalSnapshotFrameKindFull, 2, 0)
}

func testTerminalMirrorSnapshot(cols int, lines ...string) *terminal.TerminalMirrorSnapshot {
	rowBytes := make([][]byte, len(lines))
	for index, line := range lines {
		rowBytes[index] = []byte(line)
	}
	return &terminal.TerminalMirrorSnapshot{
		Rows:          len(lines),
		Cols:          cols,
		Lines:         rowBytes,
		Cursor:        []byte("\x1b[1;1H\x1b[?25h"),
		TerminalModes: &terminal.TerminalModesSnapshot{},
		CursorVisible: true,
		CapturedAt:    time.UnixMilli(1_700_000_000_000),
	}
}

func assertSnapshotFrameHeader(
	t *testing.T,
	frame []byte,
	kind terminalSnapshotFrameKind,
	sequence uint32,
	baseSequence uint32,
) {
	t.Helper()
	if len(frame) < 27 {
		t.Fatalf("frame too short: %d", len(frame))
	}
	if got := frame[0]; got != terminalSnapshotFrameVersion {
		t.Fatalf("version = %d, want %d", got, terminalSnapshotFrameVersion)
	}
	if got := terminalSnapshotFrameKind(frame[26]); got != kind {
		t.Fatalf("kind = %d, want %d", got, kind)
	}
	if got := binary.BigEndian.Uint32(frame[18:22]); got != sequence {
		t.Fatalf("sequence = %d, want %d", got, sequence)
	}
	if got := binary.BigEndian.Uint32(frame[22:26]); got != baseSequence {
		t.Fatalf("baseSequence = %d, want %d", got, baseSequence)
	}
}
