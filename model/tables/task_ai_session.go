package tables

import (
	"code-kanban/utils/model_base"
)

// TaskAISessionTable stores the relationship between tasks and AI sessions.
// This is a one-to-many relationship: one task can have multiple AI sessions.
type TaskAISessionTable struct {
	model_base.StringPKBaseModel

	// TaskID is the ID of the task
	TaskID string `gorm:"type:text;not null;index;uniqueIndex:idx_task_ai_session" json:"taskId"`

	// AISessionID is the ID of the AI session (references ai_sessions.id)
	AISessionID string `gorm:"type:text;not null;index;uniqueIndex:idx_task_ai_session" json:"aiSessionId"`

	// Relations
	Task      *TaskTable      `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE" json:"task,omitempty"`
	AISession *AISessionTable `gorm:"foreignKey:AISessionID;constraint:OnDelete:CASCADE" json:"aiSession,omitempty"`
}

// TableName maps the gorm model to the task_ai_sessions table.
func (TaskAISessionTable) TableName() string {
	return "task_ai_sessions"
}
