package websession

import "time"

type Agent string

const (
	AgentClaude Agent = "claude"
	AgentCodex  Agent = "codex"
)

type PermissionMode string

const (
	PermissionModeDefault PermissionMode = "default"
	PermissionModePlan    PermissionMode = "plan"
	PermissionModeYolo    PermissionMode = "yolo"
)

type Status string

const (
	StatusIdle     Status = "idle"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusError    Status = "err"
	StatusAborting Status = "aborting"
)

type ReasoningEffort string

const (
	ReasoningEffortDefault ReasoningEffort = "default"
	ReasoningEffortNone    ReasoningEffort = "none"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortHigh    ReasoningEffort = "high"
	ReasoningEffortXHigh   ReasoningEffort = "xhigh"
)

type Usage struct {
	InputTokens       int64   `json:"inputTokens"`
	CachedInputTokens int64   `json:"cachedInputTokens"`
	OutputTokens      int64   `json:"outputTokens"`
	Cost              float64 `json:"cost"`
}

type SessionSummary struct {
	ID              string          `json:"id"`
	ProjectID       string          `json:"projectId"`
	WorktreeID      *string         `json:"worktreeId,omitempty"`
	OrderIndex      float64         `json:"orderIndex"`
	Agent           Agent           `json:"agent"`
	Title           string          `json:"title"`
	Model           string          `json:"model"`
	ReasoningEffort ReasoningEffort `json:"reasoningEffort"`
	PermissionMode  PermissionMode  `json:"permissionMode"`
	Cwd             string          `json:"cwd"`
	NativeSessionID *string         `json:"nativeSessionId,omitempty"`
	Status          Status          `json:"status"`
	HasUnread       bool            `json:"hasUnread"`
	LastMessageAt   *time.Time      `json:"lastMessageAt,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	Usage           Usage           `json:"usage"`
}

type Attachment struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"createdAt"`
}

type Event struct {
	ID        string         `json:"id"`
	Seq       int64          `json:"seq"`
	Type      string         `json:"type"`
	RunID     string         `json:"runId,omitempty"`
	ParentID  string         `json:"parentId,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type HistoryWindow struct {
	Events       []Event `json:"events"`
	HasMore      bool    `json:"hasMore"`
	BeforeCursor string  `json:"beforeCursor,omitempty"`
	Total        int     `json:"total"`
}

type SessionSnapshot struct {
	Session SessionSummary `json:"session"`
	History HistoryWindow  `json:"history"`
}

type CreateParams struct {
	ProjectID       string
	WorktreeID      string
	Agent           Agent
	Model           string
	ReasoningEffort ReasoningEffort
	PermissionMode  PermissionMode
	Title           string
}
