package tables

import "code-kanban/utils/model_base"

// WebSessionTurnTable caches normalized turn metadata from an upstream provider.
type WebSessionTurnTable struct {
	model_base.StringPKBaseModel

	WebSessionID  string  `gorm:"type:text;not null;index:idx_web_session_turn_order,priority:1;index" json:"webSessionId"`
	SourceTurnID  *string `gorm:"type:text;index" json:"sourceTurnId"`
	OrderIndex    int64   `gorm:"type:integer;not null;index:idx_web_session_turn_order,priority:2" json:"orderIndex"`
	Status        string  `gorm:"type:text;not null;default:completed" json:"status"`
	ErrorJSON     string  `gorm:"type:text" json:"errorJson"`
	SourceCreated bool    `gorm:"type:boolean;not null;default:false" json:"sourceCreated"`
}

func (WebSessionTurnTable) TableName() string {
	return "web_session_turns"
}
