package tables

import (
	"time"

	"code-kanban/utils/model_base"
)

// WebSessionTable stores metadata for browser-based Claude/Codex sessions.
// The actual event history is persisted as JSONL files under the app data dir.
type WebSessionTable struct {
	model_base.StringPKBaseModel

	ProjectID  string  `gorm:"type:text;not null;index" json:"projectId"`
	WorktreeID *string `gorm:"type:text;index" json:"worktreeId"`
	OrderIndex float64 `gorm:"type:real;not null;default:0;index" json:"orderIndex"`

	Agent                string `gorm:"type:text;not null;index" json:"agent"`
	Backend              string `gorm:"type:text;not null;default:legacy_exec" json:"-"`
	Title                string `gorm:"type:text;not null" json:"title"`
	TitleAuto            bool   `gorm:"type:boolean;not null;default:false" json:"-"`
	Model                string `gorm:"type:text" json:"model"`
	ReasoningEffort      string `gorm:"type:text" json:"reasoningEffort"`
	WorkflowMode         string `gorm:"type:text;not null;default:default" json:"workflowMode"`
	PermissionLevel      string `gorm:"type:text;not null;default:elevated" json:"permissionLevel"`
	LegacyPermissionMode string `gorm:"column:permission_mode;type:text" json:"-"`
	Cwd                  string `gorm:"type:text;not null" json:"cwd"`

	NativeSessionID *string    `gorm:"type:text" json:"nativeSessionId"`
	Status          string     `gorm:"type:text;not null;index" json:"status"`
	HasUnread       bool       `gorm:"type:boolean;not null;default:false" json:"hasUnread"`
	ArchivedAt      *time.Time `gorm:"type:datetime;index" json:"archivedAt"`
	ActivityAt      time.Time  `gorm:"type:datetime;index" json:"activityAt"`

	LastMessageAt *time.Time `gorm:"type:datetime" json:"lastMessageAt"`
	LastEventSeq  int64      `gorm:"type:integer;not null;default:0" json:"lastEventSeq"`

	TotalInputTokens       int64   `gorm:"type:integer;not null;default:0" json:"totalInputTokens"`
	TotalCachedInputTokens int64   `gorm:"type:integer;not null;default:0" json:"totalCachedInputTokens"`
	TotalOutputTokens      int64   `gorm:"type:integer;not null;default:0" json:"totalOutputTokens"`
	TotalCost              float64 `gorm:"type:real;not null;default:0" json:"totalCost"`

	LastError *string `gorm:"type:text" json:"lastError"`
}

func (WebSessionTable) TableName() string {
	return "web_sessions"
}
