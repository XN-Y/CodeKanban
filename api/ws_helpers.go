package api

import (
	"code-kanban/service/terminal"
)

type wsMessage struct {
	Type     string                          `json:"type"`
	Data     string                          `json:"data,omitempty"`
	Cols     int                             `json:"cols,omitempty"`
	Rows     int                             `json:"rows,omitempty"`
	Snapshot *terminal.TerminalStateSnapshot `json:"snapshot,omitempty"`
	Metadata *terminal.SessionMetadata       `json:"metadata,omitempty"`
}
