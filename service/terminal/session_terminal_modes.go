package terminal

import (
	"bytes"
	"regexp"
	"strings"
)

var decPrivateModeSequencePattern = regexp.MustCompile(`\x1b\[\?([0-9;]+)([hl])`)

type terminalModesState struct {
	mouseX10            bool
	mouseButtonEvent    bool
	mouseAnyEvent       bool
	mouseSGR            bool
	focusReporting      bool
	bracketedPaste      bool
	alternateScreen47   bool
	alternateScreen1047 bool
	alternateScreen1049 bool
}

func (s *Session) updateTerminalModes(chunk []byte) (*TerminalModesSnapshot, bool) {
	if len(chunk) == 0 {
		return nil, false
	}

	data := s.combineTerminalModesChunk(chunk)
	matches := decPrivateModeSequencePattern.FindAllStringSubmatch(data, -1)
	s.terminalModesMu.Lock()
	defer s.terminalModesMu.Unlock()
	s.terminalModesPartial = terminalModesPartialSuffix(data)
	if len(matches) == 0 {
		return nil, false
	}

	changed := false
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}

		enable := match[2] == "h"
		for _, rawParam := range strings.Split(match[1], ";") {
			param := strings.TrimSpace(rawParam)
			if param == "" {
				continue
			}
			if s.terminalModes.apply(param, enable) {
				changed = true
			}
		}
	}

	if !changed {
		return nil, false
	}
	return s.terminalModes.snapshot(), true
}

func (s *Session) TerminalModesSnapshot() *TerminalModesSnapshot {
	s.terminalModesMu.RLock()
	defer s.terminalModesMu.RUnlock()
	return s.terminalModes.snapshot()
}

func (s *Session) combineTerminalModesChunk(chunk []byte) string {
	s.terminalModesMu.RLock()
	partial := s.terminalModesPartial
	s.terminalModesMu.RUnlock()
	return partial + string(chunk)
}

func (m *terminalModesState) apply(param string, enable bool) bool {
	switch param {
	case "1000":
		return setBool(&m.mouseX10, enable)
	case "1002":
		return setBool(&m.mouseButtonEvent, enable)
	case "1003":
		return setBool(&m.mouseAnyEvent, enable)
	case "1004":
		return setBool(&m.focusReporting, enable)
	case "1006":
		return setBool(&m.mouseSGR, enable)
	case "2004":
		return setBool(&m.bracketedPaste, enable)
	case "47":
		return setBool(&m.alternateScreen47, enable)
	case "1047":
		return setBool(&m.alternateScreen1047, enable)
	case "1049":
		return setBool(&m.alternateScreen1049, enable)
	default:
		return false
	}
}

func (m terminalModesState) snapshot() *TerminalModesSnapshot {
	snapshot := &TerminalModesSnapshot{
		MouseSGR:       m.mouseSGR,
		FocusReporting: m.focusReporting,
		BracketedPaste: m.bracketedPaste,
	}

	switch {
	case m.mouseAnyEvent:
		snapshot.MouseTracking = "any-event"
	case m.mouseButtonEvent:
		snapshot.MouseTracking = "button-event"
	case m.mouseX10:
		snapshot.MouseTracking = "x10"
	}

	switch {
	case m.alternateScreen1049:
		snapshot.AlternateScreen = "1049"
	case m.alternateScreen1047:
		snapshot.AlternateScreen = "1047"
	case m.alternateScreen47:
		snapshot.AlternateScreen = "47"
	}

	return snapshot
}

func setBool(target *bool, next bool) bool {
	if *target == next {
		return false
	}
	*target = next
	return true
}

func terminalModesPartialSuffix(data string) string {
	index := strings.LastIndex(data, "\x1b[?")
	if index == -1 {
		return ""
	}

	tail := data[index:]
	if !strings.HasPrefix(tail, "\x1b[?") {
		return ""
	}
	if len(tail) <= len("\x1b[?") {
		return tail
	}

	for i := len("\x1b[?"); i < len(tail); i += 1 {
		char := tail[i]
		if (char >= '0' && char <= '9') || char == ';' {
			continue
		}
		return ""
	}

	return tail
}

func BuildTerminalModesReplayPrefix(snapshot *TerminalModesSnapshot, includeAlternateScreen bool) []byte {
	var buffer bytes.Buffer

	if includeAlternateScreen {
		buffer.WriteString("\x1b[?1049l\x1b[?1047l\x1b[?47l")
	}
	buffer.WriteString("\x1b[?1006l\x1b[?1003l\x1b[?1002l\x1b[?1000l\x1b[?1004l\x1b[?2004l")

	if snapshot == nil {
		return buffer.Bytes()
	}

	if includeAlternateScreen {
		switch snapshot.AlternateScreen {
		case "47":
			buffer.WriteString("\x1b[?47h")
		case "1047":
			buffer.WriteString("\x1b[?1047h")
		case "1049":
			buffer.WriteString("\x1b[?1049h")
		}
	}

	if snapshot.FocusReporting {
		buffer.WriteString("\x1b[?1004h")
	}
	if snapshot.BracketedPaste {
		buffer.WriteString("\x1b[?2004h")
	}
	if snapshot.MouseSGR {
		buffer.WriteString("\x1b[?1006h")
	}

	switch snapshot.MouseTracking {
	case "x10":
		buffer.WriteString("\x1b[?1000h")
	case "button-event":
		buffer.WriteString("\x1b[?1002h")
	case "any-event":
		buffer.WriteString("\x1b[?1003h")
	}

	return buffer.Bytes()
}
