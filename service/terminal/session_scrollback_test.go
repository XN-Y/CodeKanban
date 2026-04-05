package terminal

import (
	"bytes"
	"runtime"
	"testing"
	"time"
)

func TestScrollbackSinceFiltersByTimestamp(t *testing.T) {
	now := time.Now()
	session := &Session{
		scrollback: [][]byte{
			[]byte("old"),
			[]byte("newer"),
			[]byte("newest"),
		},
		scrollbackTimestamps: []time.Time{
			now.Add(-3 * time.Second),
			now.Add(-1500 * time.Millisecond),
			now.Add(-500 * time.Millisecond),
		},
	}

	got := session.ScrollbackSince(now.Add(-2 * time.Second))
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(got))
	}
	if !bytes.Equal(got[0], []byte("newer")) || !bytes.Equal(got[1], []byte("newest")) {
		t.Fatalf("unexpected chunks: %q", got)
	}
}

func TestScrollbackSinceReturnsAllWhenTimestampMissing(t *testing.T) {
	session := &Session{
		scrollback: [][]byte{
			[]byte("a"),
			[]byte("b"),
		},
	}

	got := session.ScrollbackSince(time.Time{})
	if len(got) != 2 {
		t.Fatalf("expected all chunks, got %d", len(got))
	}
}

func TestTerminalStateSnapshotReflectsVisibleScreen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("server-side terminal state snapshots are disabled on windows")
	}

	session := &Session{
		rows: 4,
		cols: 12,
	}
	session.SetTerminalStateSnapshotEnabled(true)
	session.appendTerminalState([]byte("hello\r\nworld"))

	snapshot := session.TerminalStateSnapshot()
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.Rows != 4 || snapshot.Cols != 12 {
		t.Fatalf("unexpected size: %+v", snapshot)
	}
	if len(snapshot.Cells) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(snapshot.Cells))
	}
	if got := snapshot.Cells[0][0].Char; got != "h" {
		t.Fatalf("unexpected first cell: %q", got)
	}
	if got := snapshot.Cells[1][0].Char; got != "w" {
		t.Fatalf("unexpected second line start: %q", got)
	}
}
