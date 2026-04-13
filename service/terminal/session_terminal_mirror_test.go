package terminal

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/tuzig/vt10x"
)

func TestTerminalMirrorSnapshotPreservesDefaultColorsForReverseCells(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("server-side terminal state snapshots are disabled on windows")
	}

	session := &Session{
		rows: 4,
		cols: 80,
	}
	session.SetTerminalStateSnapshotEnabled(true)
	session.appendTerminalState([]byte("\x1b[7mT\x1b[27m\x1b[2mry\x1b[22m"))

	snapshot := session.TerminalMirrorSnapshot()
	if snapshot == nil {
		t.Fatal("expected mirror snapshot")
	}

	if !bytes.Contains(snapshot.Lines[0], []byte("\x1b[7;24;22;23;25;39;49mT")) {
		t.Fatalf("expected reverse cell to use terminal defaults, got %q", snapshot.Lines[0])
	}
	if bytes.Contains(snapshot.Lines[0], []byte("38;2;0;0;0")) || bytes.Contains(snapshot.Lines[0], []byte("48;2;0;0;0")) {
		t.Fatalf("expected no literal black truecolor fallback in mirror snapshot, got %q", snapshot.Lines[0])
	}
}

func TestTerminalMirrorSnapshotIncludesTerminalModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("server-side terminal state snapshots are disabled on windows")
	}

	session := &Session{
		rows: 4,
		cols: 80,
	}
	session.SetTerminalStateSnapshotEnabled(true)
	if _, changed := session.updateTerminalModes([]byte("\x1b[?1002;1006;2004;1049h")); !changed {
		t.Fatal("expected mode change")
	}
	session.appendTerminalState([]byte("ready"))

	snapshot := session.TerminalMirrorSnapshot()
	if snapshot == nil {
		t.Fatal("expected mirror snapshot")
	}
	if snapshot.TerminalModes == nil {
		t.Fatal("expected terminal modes in mirror snapshot")
	}
	if snapshot.TerminalModes.MouseTracking != "button-event" {
		t.Fatalf("expected button-event mouse tracking, got %q", snapshot.TerminalModes.MouseTracking)
	}
	if !snapshot.TerminalModes.MouseSGR {
		t.Fatal("expected SGR mouse mode to be enabled")
	}
	if !snapshot.TerminalModes.BracketedPaste {
		t.Fatal("expected bracketed paste to be enabled")
	}
	if snapshot.TerminalModes.AlternateScreen != "1049" {
		t.Fatalf("expected alternate screen 1049, got %q", snapshot.TerminalModes.AlternateScreen)
	}
}

func TestTerminalMirrorSnapshotTrimsDefaultTrailingPadding(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("server-side terminal state snapshots are disabled on windows")
	}

	session := &Session{
		rows: 3,
		cols: 8,
	}
	session.SetTerminalStateSnapshotEnabled(true)
	session.appendTerminalState([]byte("hi"))

	snapshot := session.TerminalMirrorSnapshot()
	if snapshot == nil {
		t.Fatal("expected mirror snapshot")
	}

	expectedLine := []byte(buildTerminalSnapshotSGRFromColors(0, vt10x.DefaultFG, vt10x.DefaultBG) + "hi")
	if !bytes.Equal(snapshot.Lines[0], expectedLine) {
		t.Fatalf("expected trimmed line %q, got %q", expectedLine, snapshot.Lines[0])
	}
	if len(snapshot.Lines[1]) != 0 || len(snapshot.Lines[2]) != 0 {
		t.Fatalf("expected fully blank rows to trim to empty lines, got %q and %q", snapshot.Lines[1], snapshot.Lines[2])
	}
}

func TestTerminalMirrorSnapshotPreservesStyledTrailingBlanks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("server-side terminal state snapshots are disabled on windows")
	}

	session := &Session{
		rows: 2,
		cols: 8,
	}
	session.SetTerminalStateSnapshotEnabled(true)
	session.appendTerminalState([]byte("hi\x1b[41m\x1b[K"))

	snapshot := session.TerminalMirrorSnapshot()
	if snapshot == nil {
		t.Fatal("expected mirror snapshot")
	}

	expectedLine := []byte(
		buildTerminalSnapshotSGRFromColors(0, vt10x.DefaultFG, vt10x.DefaultBG) +
			"hi" +
			buildTerminalSnapshotSGRFromColors(0, vt10x.DefaultFG, vt10x.Color(0xcc0000)) +
			strings.Repeat(" ", 6),
	)
	if !bytes.Equal(snapshot.Lines[0], expectedLine) {
		t.Fatalf("expected styled trailing blanks to be preserved, got %q", snapshot.Lines[0])
	}
}
