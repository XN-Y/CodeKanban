package model

import (
	"context"
	"errors"
	"time"

	"code-kanban/model/tables"
	"code-kanban/utils"

	"gorm.io/gorm"
)

var (
	// ErrTaskAISessionNotFound indicates the requested task-AI session link does not exist.
	ErrTaskAISessionNotFound = errors.New("task AI session link not found")
	// ErrTaskAISessionExists indicates the task-AI session link already exists.
	ErrTaskAISessionExists = errors.New("task AI session link already exists")
)

// TaskAISessionService coordinates linking tasks to AI sessions.
type TaskAISessionService struct{}

// TaskAISessionWithDetails includes AI session details for display.
type TaskAISessionWithDetails struct {
	ID        string `json:"id"`
	TaskID    string `json:"taskId"`
	SessionID string `json:"sessionId"`
	// AI Session details
	AISessionDBID    string  `json:"aiSessionDbId"`
	Type             string  `json:"type"`
	Model            string  `json:"model"`
	Title            string  `json:"title"`
	SessionStartedAt string  `json:"sessionStartedAt"`
	LastMessageAt    *string `json:"lastMessageAt"`
	MessageCount     int     `json:"messageCount"`
}

// LinkTaskToAISession creates a link between a task and an AI session (by database ID).
func (s *TaskAISessionService) LinkTaskToAISession(ctx context.Context, taskID, aiSessionDBID string) (*tables.TaskAISessionTable, error) {
	dbCtx, err := s.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	// Check if task exists
	var task tables.TaskTable
	if err := dbCtx.First(&task, "id = ?", taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	// Check if AI session exists
	var aiSession tables.AISessionTable
	if err := dbCtx.First(&aiSession, "id = ?", aiSessionDBID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAISessionNotFound
		}
		return nil, err
	}

	// Check if link already exists
	var existing tables.TaskAISessionTable
	if err := dbCtx.First(&existing, "task_id = ? AND ai_session_id = ?", taskID, aiSessionDBID).Error; err == nil {
		return nil, ErrTaskAISessionExists
	}

	link := &tables.TaskAISessionTable{
		TaskID:      taskID,
		AISessionID: aiSessionDBID,
	}

	if err := dbCtx.Create(link).Error; err != nil {
		return nil, err
	}

	return link, nil
}

// LinkTaskToAISessionBySessionID creates a link between a task and an AI session (by session_id field).
// This is used when linking from terminal where we only have the AI assistant's session ID.
func (s *TaskAISessionService) LinkTaskToAISessionBySessionID(ctx context.Context, taskID, sessionID string) error {
	dbCtx, err := s.dbWithContext(ctx)
	if err != nil {
		return err
	}

	// Find AI session by session_id
	var aiSession tables.AISessionTable
	if err := dbCtx.First(&aiSession, "session_id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// AI session not cached yet, ignore
			return nil
		}
		return err
	}

	// Check if link already exists
	var existing tables.TaskAISessionTable
	if err := dbCtx.First(&existing, "task_id = ? AND ai_session_id = ?", taskID, aiSession.ID).Error; err == nil {
		// Already linked, ignore
		return nil
	}

	link := &tables.TaskAISessionTable{
		TaskID:      taskID,
		AISessionID: aiSession.ID,
	}

	return dbCtx.Create(link).Error
}

// EnsureAISessionAndLinkToTask ensures an AI session record exists and links it to a task.
// This is used for auto-linking when a terminal discovers an AI session.
// If the AI session doesn't exist in the database, it creates a minimal record.
func (s *TaskAISessionService) EnsureAISessionAndLinkToTask(
	ctx context.Context,
	taskID string,
	sessionID string,
	filePath string,
	projectPath string,
	sessionType tables.AISessionType,
) error {
	dbCtx, err := s.dbWithContext(ctx)
	if err != nil {
		return err
	}

	// Check if task exists
	var task tables.TaskTable
	if err := dbCtx.First(&task, "id = ?", taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTaskNotFound
		}
		return err
	}

	// Find or create AI session
	var aiSession tables.AISessionTable
	if err := dbCtx.First(&aiSession, "session_id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create a minimal AI session record
			// The full metadata will be populated later when AISessionService scans it
			now := time.Now()
			aiSession = tables.AISessionTable{
				SessionID:        sessionID,
				Type:             sessionType,
				ProjectPath:      projectPath,
				FilePath:         filePath,
				SessionStartedAt: now,
			}
			aiSession.ID = utils.NewID()
			aiSession.CreatedAt = now
			aiSession.UpdatedAt = now

			if err := dbCtx.Create(&aiSession).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Check if link already exists
	var existing tables.TaskAISessionTable
	if err := dbCtx.First(&existing, "task_id = ? AND ai_session_id = ?", taskID, aiSession.ID).Error; err == nil {
		// Already linked, success
		return nil
	}

	// Create the link
	link := &tables.TaskAISessionTable{
		TaskID:      taskID,
		AISessionID: aiSession.ID,
	}
	link.ID = utils.NewID()
	link.CreatedAt = time.Now()
	link.UpdatedAt = link.CreatedAt

	return dbCtx.Create(link).Error
}

// UnlinkTaskFromAISession removes a link between a task and an AI session.
func (s *TaskAISessionService) UnlinkTaskFromAISession(ctx context.Context, taskID, aiSessionID string) error {
	dbCtx, err := s.dbWithContext(ctx)
	if err != nil {
		return err
	}

	result := dbCtx.Where("task_id = ? AND ai_session_id = ?", taskID, aiSessionID).
		Delete(&tables.TaskAISessionTable{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskAISessionNotFound
	}
	return nil
}

// GetAISessionsForTask returns all AI sessions linked to a task with details.
func (s *TaskAISessionService) GetAISessionsForTask(ctx context.Context, taskID string) ([]TaskAISessionWithDetails, error) {
	dbCtx, err := s.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var links []tables.TaskAISessionTable
	if err := dbCtx.
		Preload("AISession").
		Where("task_id = ?", taskID).
		Find(&links).Error; err != nil {
		return nil, err
	}

	result := make([]TaskAISessionWithDetails, 0, len(links))
	for _, link := range links {
		if link.AISession == nil {
			continue
		}
		session := link.AISession
		detail := TaskAISessionWithDetails{
			ID:               link.ID,
			TaskID:           link.TaskID,
			SessionID:        session.SessionID,
			AISessionDBID:    session.ID,
			Type:             string(session.Type),
			Model:            session.Model,
			Title:            session.Title,
			SessionStartedAt: session.SessionStartedAt.Format("2006-01-02T15:04:05Z07:00"),
			MessageCount:     session.MessageCount,
		}
		if session.LastMessageAt != nil {
			ts := session.LastMessageAt.Format("2006-01-02T15:04:05Z07:00")
			detail.LastMessageAt = &ts
		}
		result = append(result, detail)
	}

	return result, nil
}

// GetTasksForAISession returns all task IDs linked to an AI session.
func (s *TaskAISessionService) GetTasksForAISession(ctx context.Context, aiSessionID string) ([]string, error) {
	dbCtx, err := s.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var links []tables.TaskAISessionTable
	if err := dbCtx.
		Where("ai_session_id = ?", aiSessionID).
		Find(&links).Error; err != nil {
		return nil, err
	}

	taskIDs := make([]string, len(links))
	for i, link := range links {
		taskIDs[i] = link.TaskID
	}

	return taskIDs, nil
}

func (s *TaskAISessionService) dbWithContext(ctx context.Context) (*gorm.DB, error) {
	if db == nil {
		return nil, ErrDBNotInitialized
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return db.WithContext(ctx), nil
}
