package log_watcher

import (
	"time"
)

// WatcherState represents the current state of a log watcher
type WatcherState string

const (
	WatcherStateSearching WatcherState = "searching" // Searching for session file
	WatcherStateWatching  WatcherState = "watching"  // Actively watching file changes
	WatcherStateStopped   WatcherState = "stopped"   // Watcher stopped
	WatcherStateError     WatcherState = "error"     // Error occurred
)

// SessionMeta contains metadata from the session_meta line
type SessionMeta struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Cwd          string    `json:"cwd"`
	Originator   string    `json:"originator"`
	Source       string    `json:"source,omitempty"`
	CliVersion   string    `json:"cli_version"`
	Model        string    `json:"model,omitempty"`
	Instructions string    `json:"instructions,omitempty"`
}

// UserMessage represents a user message extracted from the log
type UserMessage struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Images    []string  `json:"images,omitempty"`
}

// WatcherInfo contains current watcher status information
type WatcherInfo struct {
	State         WatcherState   `json:"state"`
	SessionID     string         `json:"sessionId,omitempty"`
	FilePath      string         `json:"filePath,omitempty"`
	LinesRead     int            `json:"linesRead"`
	FileOffset    int64          `json:"fileOffset"`
	LastCheckTime time.Time      `json:"lastCheckTime"`
	LastMessage   *UserMessage   `json:"lastMessage,omitempty"`
	MessageCount  int            `json:"messageCount"`
	Error         string         `json:"error,omitempty"`
	SessionMeta   *SessionMeta   `json:"sessionMeta,omitempty"`
	UserMessages  []*UserMessage `json:"userMessages,omitempty"`
}

// WatcherEvent represents an event from the log watcher
type WatcherEvent struct {
	Type      WatcherEventType `json:"type"`
	Timestamp time.Time        `json:"timestamp"`
	Message   *UserMessage     `json:"message,omitempty"`
	Error     error            `json:"error,omitempty"`
}

// WatcherEventType defines the type of watcher event
type WatcherEventType string

const (
	EventTypeSessionFound WatcherEventType = "session_found"
	EventTypeNewMessage   WatcherEventType = "new_message"
	EventTypeError        WatcherEventType = "error"
	EventTypeStopped      WatcherEventType = "stopped"
)

// WatcherCallback is called when events occur
type WatcherCallback func(event WatcherEvent)

// LogEntry represents a generic log entry from the JSONL file
type LogEntry struct {
	Timestamp string      `json:"timestamp"`
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload"`
}

// SessionMetaPayload is the payload for session_meta entries
type SessionMetaPayload struct {
	ID           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	Cwd          string `json:"cwd"`
	Originator   string `json:"originator"`
	CliVersion   string `json:"cli_version"`
	Instructions string `json:"instructions,omitempty"`
	Model        string `json:"model,omitempty"`
	Source       string `json:"source,omitempty"`
}

// EventMsgPayload is the payload for event_msg entries
type EventMsgPayload struct {
	Type        string   `json:"type"`
	Message     string   `json:"message,omitempty"`
	Text        string   `json:"text,omitempty"` // For agent_reasoning
	Images      []string `json:"images,omitempty"`
	LocalImages []string `json:"local_images,omitempty"`
	Reason      string   `json:"reason,omitempty"`
}

// TurnContextPayload is the payload for turn_context entries
type TurnContextPayload struct {
	Cwd            string `json:"cwd"`
	ApprovalPolicy string `json:"approval_policy"`
	Model          string `json:"model"`
	Effort         string `json:"effort,omitempty"`
	Summary        string `json:"summary,omitempty"`
}
