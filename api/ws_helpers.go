package api

import "code-kanban/service/terminal"

type wsMessage struct {
	Type                       string                    `json:"type"`
	Data                       string                    `json:"data,omitempty"`
	Cols                       int                       `json:"cols,omitempty"`
	Rows                       int                       `json:"rows,omitempty"`
	Mode                       string                    `json:"mode,omitempty"`
	SnapshotIntervalMs         int                       `json:"snapshotIntervalMs,omitempty"`
	SnapshotCompressionEnabled bool                      `json:"snapshotCompressionEnabled"`
	SnapshotIncrementalEnabled bool                      `json:"snapshotIncrementalEnabled"`
	Metadata                   *terminal.SessionMetadata `json:"metadata,omitempty"`
}
