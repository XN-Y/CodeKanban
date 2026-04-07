package terminal

import "testing"

func TestTerminalModesSnapshotTracksAndClearsModes(t *testing.T) {
	session := &Session{}

	snapshot, changed := session.updateTerminalModes([]byte("\x1b[?1002;1006;1004;2004;1049h"))
	if !changed {
		t.Fatal("expected mode change")
	}
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.MouseTracking != "button-event" {
		t.Fatalf("expected button-event mouse tracking, got %q", snapshot.MouseTracking)
	}
	if !snapshot.MouseSGR {
		t.Fatal("expected SGR mouse mode to be enabled")
	}
	if !snapshot.FocusReporting {
		t.Fatal("expected focus reporting to be enabled")
	}
	if !snapshot.BracketedPaste {
		t.Fatal("expected bracketed paste to be enabled")
	}
	if snapshot.AlternateScreen != "1049" {
		t.Fatalf("expected alternate screen 1049, got %q", snapshot.AlternateScreen)
	}

	snapshot, changed = session.updateTerminalModes([]byte("\x1b[?1006;1002;1049l"))
	if !changed {
		t.Fatal("expected mode reset change")
	}
	if snapshot.MouseTracking != "" {
		t.Fatalf("expected mouse tracking to be cleared, got %q", snapshot.MouseTracking)
	}
	if snapshot.MouseSGR {
		t.Fatal("expected SGR mouse mode to be cleared")
	}
	if snapshot.AlternateScreen != "" {
		t.Fatalf("expected alternate screen to be cleared, got %q", snapshot.AlternateScreen)
	}
	if !snapshot.FocusReporting {
		t.Fatal("expected focus reporting to remain enabled")
	}
	if !snapshot.BracketedPaste {
		t.Fatal("expected bracketed paste to remain enabled")
	}
}

func TestTerminalModesSnapshotFallsBackToPreviousMouseMode(t *testing.T) {
	session := &Session{}

	if _, changed := session.updateTerminalModes([]byte("\x1b[?1000h\x1b[?1002h")); !changed {
		t.Fatal("expected mode change")
	}

	snapshot := session.TerminalModesSnapshot()
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.MouseTracking != "button-event" {
		t.Fatalf("expected button-event mouse tracking, got %q", snapshot.MouseTracking)
	}

	if _, changed := session.updateTerminalModes([]byte("\x1b[?1002l")); !changed {
		t.Fatal("expected fallback mode change")
	}

	snapshot = session.TerminalModesSnapshot()
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.MouseTracking != "x10" {
		t.Fatalf("expected x10 mouse tracking fallback, got %q", snapshot.MouseTracking)
	}
}

func TestTerminalModesSnapshotHandlesSplitSequences(t *testing.T) {
	session := &Session{}

	if _, changed := session.updateTerminalModes([]byte("\x1b[?10")); changed {
		t.Fatal("did not expect change for incomplete sequence")
	}
	if _, changed := session.updateTerminalModes([]byte("02;1006h")); !changed {
		t.Fatal("expected change after completing split sequence")
	}

	snapshot := session.TerminalModesSnapshot()
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.MouseTracking != "button-event" {
		t.Fatalf("expected button-event mouse tracking, got %q", snapshot.MouseTracking)
	}
	if !snapshot.MouseSGR {
		t.Fatal("expected SGR mouse mode to be enabled")
	}
}
