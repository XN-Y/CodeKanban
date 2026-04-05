package terminal

import (
	"runtime"
	"time"

	"github.com/tuzig/vt10x"
)

func (s *Session) initTerminalStateLocked(cols, rows int) {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	s.terminalState = vt10x.New(vt10x.WithSize(cols, rows))
	s.terminalStateCapturedAt = time.Time{}
}

func (s *Session) terminalStateEnabledForPlatform() bool {
	return runtime.GOOS != "windows" && s.terminalStateEnabled.Load()
}

func (s *Session) appendTerminalState(chunk []byte) {
	if len(chunk) == 0 || !s.terminalStateEnabledForPlatform() {
		return
	}

	s.terminalStateMu.Lock()
	defer s.terminalStateMu.Unlock()

	if s.terminalState == nil {
		s.mu.RLock()
		cols := s.cols
		rows := s.rows
		s.mu.RUnlock()
		s.initTerminalStateLocked(cols, rows)
	}
	if s.terminalState == nil {
		return
	}

	_, _ = s.terminalState.Write(chunk)
	s.terminalStateCapturedAt = time.Now()
}

func (s *Session) resizeTerminalState(cols, rows int) {
	if !s.terminalStateEnabledForPlatform() {
		return
	}

	s.terminalStateMu.Lock()
	defer s.terminalStateMu.Unlock()

	if s.terminalState == nil {
		s.initTerminalStateLocked(cols, rows)
		return
	}
	s.terminalState.Resize(cols, rows)
}

func (s *Session) rebuildTerminalStateFromScrollbackLocked() {
	s.mu.RLock()
	cols := s.cols
	rows := s.rows
	s.mu.RUnlock()

	if s.terminalState == nil {
		s.initTerminalStateLocked(cols, rows)
	}
	if s.terminalState == nil {
		return
	}

	s.terminalState.Resize(cols, rows)
	_, _ = s.terminalState.Write([]byte("\x1bc\x1b[2J\x1b[H"))

	s.scrollMu.RLock()
	scrollback := make([][]byte, len(s.scrollback))
	copy(scrollback, s.scrollback)
	timestamps := make([]time.Time, len(s.scrollbackTimestamps))
	copy(timestamps, s.scrollbackTimestamps)
	s.scrollMu.RUnlock()

	for _, chunk := range scrollback {
		if len(chunk) == 0 {
			continue
		}
		_, _ = s.terminalState.Write(chunk)
	}
	if len(timestamps) > 0 {
		s.terminalStateCapturedAt = timestamps[len(timestamps)-1]
	} else {
		s.terminalStateCapturedAt = time.Time{}
	}
}

func (s *Session) SetTerminalStateSnapshotEnabled(enabled bool) {
	enabled = enabled && runtime.GOOS != "windows"
	s.terminalStateEnabled.Store(enabled)

	s.terminalStateMu.Lock()
	defer s.terminalStateMu.Unlock()

	if !enabled {
		s.terminalState = nil
		s.terminalStateCapturedAt = time.Time{}
		return
	}

	s.rebuildTerminalStateFromScrollbackLocked()
}

func (s *Session) TerminalStateSnapshot() *TerminalStateSnapshot {
	if !s.terminalStateEnabledForPlatform() {
		return nil
	}

	s.terminalStateMu.Lock()
	defer s.terminalStateMu.Unlock()

	if s.terminalState == nil {
		s.rebuildTerminalStateFromScrollbackLocked()
	}
	if s.terminalState == nil {
		return nil
	}

	cols, rows := s.terminalState.Size()
	if cols <= 0 || rows <= 0 {
		return nil
	}

	cells := make([][]TerminalStateCell, rows)
	for row := 0; row < rows; row++ {
		rowCells := make([]TerminalStateCell, cols)
		for col := 0; col < cols; col++ {
			cell := s.terminalState.Cell(col, row)
			rowCells[col] = snapshotCellFromGlyph(cell)
		}
		cells[row] = rowCells
	}

	cursor := s.terminalState.Cursor()
	return &TerminalStateSnapshot{
		Rows:            rows,
		Cols:            cols,
		Cells:           cells,
		CursorX:         cursor.X,
		CursorY:         cursor.Y,
		CursorVisible:   s.terminalState.CursorVisible(),
		CursorMode:      cursor.Attr.Mode,
		CursorFG:        snapshotColorValue(cursor.Attr.FG),
		CursorBG:        snapshotColorValue(cursor.Attr.BG),
		CursorFGDefault: cursor.Attr.FG == vt10x.DefaultFG,
		CursorBGDefault: cursor.Attr.BG == vt10x.DefaultBG,
		CapturedAt:      s.terminalStateCapturedAt,
	}
}

func snapshotCellFromGlyph(cell vt10x.Glyph) TerminalStateCell {
	char := ""
	if cell.Char != 0 {
		char = string(cell.Char)
	}
	return TerminalStateCell{
		Char:      char,
		Mode:      cell.Mode,
		FG:        snapshotColorValue(cell.FG),
		BG:        snapshotColorValue(cell.BG),
		FGDefault: cell.FG == vt10x.DefaultFG,
		BGDefault: cell.BG == vt10x.DefaultBG,
	}
}

func snapshotColorValue(color vt10x.Color) uint32 {
	if color == vt10x.DefaultFG || color == vt10x.DefaultBG || color == vt10x.DefaultCursor {
		return 0
	}
	return uint32(color)
}
