package websession

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"code-kanban/model"
	"code-kanban/model/tables"
	"code-kanban/service"
	"code-kanban/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	DefaultHistoryWindow = 80
	MaxHistoryWindow     = 120
	sessionOrderStep     = 1000.0
	planPromptPreamble   = "You are operating in planning mode. Inspect the project first, summarize the goal, and propose a concrete plan before making changes. Do not mutate files until the user confirms execution or explicitly asks you to proceed immediately. If additional permissions are needed, call them out explicitly."
)

type Config struct {
	DataDir             string
	AttachmentSizeLimit int64
	ClaudePath          string
	CodexPath           string
}

type Manager struct {
	cfg         Config
	logger      *zap.Logger
	store       *store
	projectSvc  *model.ProjectService
	worktreeSvc *service.WorktreeService

	mu      sync.RWMutex
	runs    map[string]*activeRun
	clients map[*client]struct{}
}

type client struct {
	conn    wsConn
	logger  *zap.Logger
	writeMu sync.Mutex
}

type wsConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteJSON(v any) error
	Close() error
}

type activeRun struct {
	sessionID          string
	agent              Agent
	backend            SessionBackend
	runID              string
	assistantMessageID string
	currentToolMessage string
	lastError          string
	cmd                *exec.Cmd
	cancel             context.CancelFunc
	done               chan struct{}
	mu                 sync.Mutex
	stdin              io.WriteCloser
	recentRuntimeLines []string
	pendingApproval    string
	pendingServerReq   *pendingServerRequest
	app                *codexAppServerClient
	assistantDeltaSeen map[string]bool
}

type attachmentMeta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewManager(cfg Config, logger *zap.Logger) (*Manager, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = utils.GetDataDir()
	}
	if cfg.AttachmentSizeLimit <= 0 {
		cfg.AttachmentSizeLimit = 10 * 1024 * 1024
	}
	if cfg.ClaudePath == "" {
		cfg.ClaudePath = getenvDefault("CLAUDE_PATH", "claude")
	}
	if cfg.CodexPath == "" {
		cfg.CodexPath = getenvDefault("CODEX_PATH", "codex")
	}
	if logger == nil {
		logger = utils.Logger()
	}

	eventStore, err := newStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		cfg:         cfg,
		logger:      logger.Named("web-session-manager"),
		store:       eventStore,
		projectSvc:  model.NewProjectService(),
		worktreeSvc: service.NewWorktreeService(),
		runs:        make(map[string]*activeRun),
		clients:     make(map[*client]struct{}),
	}
	if err := manager.migrateLegacySessionModes(context.Background()); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) RegisterClient(conn wsConn) *client {
	client := &client{
		conn:   conn,
		logger: m.logger.Named("client"),
	}
	m.mu.Lock()
	m.clients[client] = struct{}{}
	m.mu.Unlock()
	return client
}

func (m *Manager) UnregisterClient(client *client) {
	if client == nil {
		return
	}
	m.mu.Lock()
	delete(m.clients, client)
	m.mu.Unlock()
}

func (m *Manager) ListSessions(ctx context.Context, projectID string) ([]SessionSummary, error) {
	db := model.GetDB()
	if db == nil {
		return nil, model.ErrDBNotInitialized
	}

	records, err := m.listSessionRecordsWithDB(db.WithContext(ctx), projectID)
	if err != nil {
		return nil, err
	}

	items := make([]SessionSummary, 0, len(records))
	for _, record := range records {
		items = append(items, mapSessionRecord(record))
	}
	return items, nil
}

func (m *Manager) CreateSession(ctx context.Context, params CreateParams) (SessionSummary, error) {
	project, worktreeID, cwd, err := m.resolveContext(ctx, params.ProjectID, params.WorktreeID)
	if err != nil {
		return SessionSummary{}, err
	}

	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = defaultTitle(params.Agent, project.Name)
	}

	orderIndex, err := m.getNextSessionOrderIndex(ctx, project.Id)
	if err != nil {
		return SessionSummary{}, err
	}

	record := tables.WebSessionTable{
		ProjectID:              project.Id,
		WorktreeID:             nilIfEmpty(worktreeID),
		OrderIndex:             orderIndex,
		Agent:                  string(normalizeAgent(params.Agent)),
		Backend:                string(normalizeSessionBackend(params.Backend, normalizeAgent(params.Agent))),
		Title:                  title,
		TitleAuto:              strings.TrimSpace(params.Title) == "",
		Model:                  defaultModel(normalizeAgent(params.Agent), params.Model),
		ReasoningEffort:        string(defaultReasoningEffort(normalizeAgent(params.Agent), params.ReasoningEffort)),
		WorkflowMode:           string(normalizeWorkflowMode(params.WorkflowMode)),
		PermissionLevel:        string(normalizePermissionLevel(params.PermissionLevel)),
		Cwd:                    cwd,
		Status:                 string(StatusIdle),
		HasUnread:              false,
		LastEventSeq:           0,
		TotalInputTokens:       0,
		TotalCachedInputTokens: 0,
		TotalOutputTokens:      0,
		TotalCost:              0,
	}
	record.Init()

	if err := model.GetDB().WithContext(ctx).Create(&record).Error; err != nil {
		return SessionSummary{}, err
	}

	return mapSessionRecord(record), nil
}

func (m *Manager) GetSession(ctx context.Context, sessionID string) (tables.WebSessionTable, error) {
	db := model.GetDB()
	if db == nil {
		return tables.WebSessionTable{}, model.ErrDBNotInitialized
	}
	var record tables.WebSessionTable
	if err := db.WithContext(ctx).First(&record, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tables.WebSessionTable{}, gorm.ErrRecordNotFound
		}
		return tables.WebSessionTable{}, err
	}
	return record, nil
}

func (m *Manager) Snapshot(ctx context.Context, sessionID string, limit int) (SessionSnapshot, error) {
	return m.loadSnapshot(ctx, sessionID, limit, true)
}

func (m *Manager) loadSnapshot(ctx context.Context, sessionID string, limit int, clearUnread bool) (SessionSnapshot, error) {
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	if limit <= 0 || limit > MaxHistoryWindow {
		limit = DefaultHistoryWindow
	}
	history, err := m.store.readWindow(sessionID, limit, nil)
	if err != nil {
		return SessionSnapshot{}, err
	}

	// Entering a session clears the unread state.
	if clearUnread && record.HasUnread {
		record.HasUnread = false
		if err := model.GetDB().WithContext(ctx).Model(&tables.WebSessionTable{}).
			Where("id = ?", sessionID).
			Update("has_unread", false).Error; err != nil {
			m.logger.Warn("failed to clear unread flag", zap.String("sessionId", sessionID), zap.Error(err))
		}
	}

	summary := mapSessionRecord(record)
	if clearUnread {
		summary.HasUnread = false
	}
	return SessionSnapshot{
		Session: summary,
		History: history,
	}, nil
}

func (m *Manager) History(ctx context.Context, sessionID string, limit int, beforeSeq *int64) (HistoryWindow, error) {
	if _, err := m.GetSession(ctx, sessionID); err != nil {
		return HistoryWindow{}, err
	}
	if limit <= 0 || limit > MaxHistoryWindow {
		limit = DefaultHistoryWindow
	}
	return m.store.readWindow(sessionID, limit, beforeSeq)
}

func (m *Manager) RenameSession(ctx context.Context, sessionID, title string) (SessionSummary, error) {
	normalized := strings.TrimSpace(title)
	if normalized == "" {
		return SessionSummary{}, fmt.Errorf("title is required")
	}
	if err := model.GetDB().WithContext(ctx).Model(&tables.WebSessionTable{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{
			"title":      normalized,
			"title_auto": false,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return SessionSummary{}, err
	}
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}
	return mapSessionRecord(record), nil
}

func (m *Manager) UpdateModel(ctx context.Context, sessionID, modelName string) (SessionSummary, error) {
	return m.updateFields(ctx, sessionID, map[string]any{
		"model":      strings.TrimSpace(modelName),
		"updated_at": time.Now(),
	})
}

func (m *Manager) UpdateReasoningEffort(
	ctx context.Context,
	sessionID string,
	effort ReasoningEffort,
) (SessionSummary, error) {
	return m.updateFields(ctx, sessionID, map[string]any{
		"reasoning_effort": string(normalizeReasoningEffort(effort)),
		"updated_at":       time.Now(),
	})
}

func (m *Manager) UpdateWorkflowMode(
	ctx context.Context,
	sessionID string,
	mode WorkflowMode,
) (SessionSummary, error) {
	return m.updateFields(ctx, sessionID, map[string]any{
		"workflow_mode": string(normalizeWorkflowMode(mode)),
		"updated_at":    time.Now(),
	})
}

func (m *Manager) UpdatePermissionLevel(
	ctx context.Context,
	sessionID string,
	level PermissionLevel,
) (SessionSummary, error) {
	return m.updateFields(ctx, sessionID, map[string]any{
		"permission_level": string(normalizePermissionLevel(level)),
		"updated_at":       time.Now(),
	})
}

func (m *Manager) UpdateAgent(ctx context.Context, sessionID string, agent Agent) (SessionSummary, error) {
	normalized := normalizeAgent(agent)
	return m.updateFields(ctx, sessionID, map[string]any{
		"agent":             string(normalized),
		"backend":           string(defaultSessionBackend(normalized)),
		"model":             defaultModel(normalized, ""),
		"reasoning_effort":  string(defaultReasoningEffort(normalized, "")),
		"native_session_id": nil,
		"updated_at":        time.Now(),
	})
}

func (m *Manager) MoveSession(ctx context.Context, sessionID, prevSessionID, nextSessionID string) (SessionSummary, error) {
	db := model.GetDB()
	if db == nil {
		return SessionSummary{}, model.ErrDBNotInitialized
	}

	var summary SessionSummary
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var moving tables.WebSessionTable
		if err := tx.First(&moving, "id = ?", sessionID).Error; err != nil {
			return err
		}

		ordered, err := m.listSessionRecordsWithDB(tx, moving.ProjectID)
		if err != nil {
			return err
		}
		if len(ordered) == 0 {
			return gorm.ErrRecordNotFound
		}

		filtered := make([]tables.WebSessionTable, 0, len(ordered)-1)
		for _, item := range ordered {
			if item.ID == moving.ID {
				continue
			}
			filtered = append(filtered, item)
		}

		insertIndex, err := resolveSessionInsertIndex(filtered, moving.ID, prevSessionID, nextSessionID)
		if err != nil {
			return err
		}

		reordered := make([]tables.WebSessionTable, 0, len(ordered))
		reordered = append(reordered, filtered[:insertIndex]...)
		reordered = append(reordered, moving)
		reordered = append(reordered, filtered[insertIndex:]...)

		for index, item := range reordered {
			nextOrderIndex := float64(index+1) * sessionOrderStep
			if item.OrderIndex == nextOrderIndex {
				continue
			}
			if err := tx.Model(&tables.WebSessionTable{}).
				Where("id = ?", item.ID).
				UpdateColumn("order_index", nextOrderIndex).Error; err != nil {
				return err
			}
			if item.ID == moving.ID {
				moving.OrderIndex = nextOrderIndex
			}
		}

		summary = mapSessionRecord(moving)
		return nil
	})
	if err != nil {
		return SessionSummary{}, err
	}
	return summary, nil
}

func (m *Manager) DeleteSession(ctx context.Context, sessionID string) error {
	_ = m.AbortSession(sessionID)
	if err := model.GetDB().WithContext(ctx).Delete(&tables.WebSessionTable{}, "id = ?", sessionID).Error; err != nil {
		return err
	}
	return m.store.deleteSessionFiles(sessionID)
}

func (m *Manager) AbortSession(sessionID string) error {
	m.mu.RLock()
	run, ok := m.runs[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	if run.cancel != nil {
		run.cancel()
	}
	if run.cmd != nil && run.cmd.Process != nil {
		_ = run.cmd.Process.Kill()
	}
	return nil
}

func (m *Manager) HandleCommand(ctx context.Context, client *client, payload []byte) error {
	var frame wireCommandFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return client.send(newErrorFrame("", "", "bad_req", "invalid json payload", false))
	}
	if frame.Kind != "cmd" {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "unsupported frame kind", false))
	}

	switch frame.Operation {
	case "create":
		return m.handleCreateCommand(ctx, client, frame)
	case "connect":
		return m.handleConnectCommand(ctx, client, frame)
	case "send":
		return m.handleSendCommand(ctx, client, frame)
	case "hist":
		return m.handleHistoryCommand(ctx, client, frame)
	case "abort":
		return m.handleAbortCommand(ctx, client, frame)
	case "rename":
		return m.handleRenameCommand(ctx, client, frame)
	case "set_md":
		return m.handleSetModelCommand(ctx, client, frame)
	case "set_re":
		return m.handleSetReasoningEffortCommand(ctx, client, frame)
	case "set_wm":
		return m.handleSetWorkflowModeCommand(ctx, client, frame)
	case "set_pl":
		return m.handleSetPermissionLevelCommand(ctx, client, frame)
	case "set_pm":
		return m.handleLegacySetModeCommand(ctx, client, frame)
	case "set_ag":
		return m.handleSetAgentCommand(ctx, client, frame)
	case "move":
		return m.handleMoveCommand(ctx, client, frame)
	case "approve":
		return m.handleApprovalCommand(client, frame, "approve")
	case "reject":
		return m.handleApprovalCommand(client, frame, "reject")
	case "user_input":
		return m.handleUserInputCommand(client, frame)
	case "del":
		return m.handleDeleteCommand(ctx, client, frame)
	case "list":
		return m.handleListCommand(ctx, client, frame)
	default:
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "unknown operation", false))
	}
}

func (m *Manager) SaveAttachment(fileHeader *multipart.FileHeader) (Attachment, error) {
	if fileHeader == nil {
		return Attachment{}, fmt.Errorf("file is required")
	}
	if fileHeader.Size <= 0 {
		return Attachment{}, fmt.Errorf("empty file")
	}
	if fileHeader.Size > m.cfg.AttachmentSizeLimit {
		return Attachment{}, fmt.Errorf("attachment too large")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return Attachment{}, err
	}
	defer file.Close()

	buffer := bytes.NewBuffer(nil)
	written, err := io.Copy(buffer, io.LimitReader(file, m.cfg.AttachmentSizeLimit+1))
	if err != nil {
		return Attachment{}, err
	}
	if written > m.cfg.AttachmentSizeLimit {
		return Attachment{}, fmt.Errorf("attachment too large")
	}

	attachmentID := utils.NewID()
	extension := filepath.Ext(fileHeader.Filename)
	targetPath := m.store.attachmentPath(attachmentID, extension)
	if err := os.WriteFile(targetPath, buffer.Bytes(), 0o644); err != nil {
		return Attachment{}, err
	}

	attachment := Attachment{
		ID:        attachmentID,
		Name:      filepath.Base(fileHeader.Filename),
		Mime:      fileHeader.Header.Get("Content-Type"),
		Size:      written,
		Path:      targetPath,
		CreatedAt: time.Now(),
	}
	if attachment.Mime == "" {
		attachment.Mime = http.DetectContentType(buffer.Bytes())
	}

	meta := attachmentMeta{
		ID:        attachment.ID,
		Name:      attachment.Name,
		Mime:      attachment.Mime,
		Size:      attachment.Size,
		Path:      attachment.Path,
		CreatedAt: attachment.CreatedAt,
	}
	metaBytes, err := json.Marshal(meta)
	if err == nil {
		_ = os.WriteFile(m.store.attachmentPath(attachmentID, ".json"), metaBytes, 0o644)
	}
	return attachment, nil
}

func (m *Manager) loadAttachment(id string) (Attachment, error) {
	metaPath := m.store.attachmentPath(id, ".json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return Attachment{}, err
	}
	var meta attachmentMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return Attachment{}, err
	}
	return Attachment{
		ID:        meta.ID,
		Name:      meta.Name,
		Mime:      meta.Mime,
		Size:      meta.Size,
		Path:      meta.Path,
		CreatedAt: meta.CreatedAt,
	}, nil
}

func (m *Manager) GetAttachment(id string) (Attachment, error) {
	return m.loadAttachment(strings.TrimSpace(id))
}

func (m *Manager) handleCreateCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		ProjectID       string `json:"pid"`
		WorktreeID      string `json:"wid"`
		Agent           string `json:"ag"`
		Model           string `json:"md"`
		ReasoningEffort string `json:"re"`
		WorkflowMode    string `json:"wm"`
		PermissionLevel string `json:"pl"`
		PermissionMode  string `json:"pm"`
		Title           string `json:"ttl"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, "", "bad_req", "invalid create payload", false))
	}

	workflowMode := WorkflowMode(payload.WorkflowMode)
	permissionLevel := PermissionLevel(payload.PermissionLevel)
	if strings.TrimSpace(payload.PermissionMode) != "" {
		legacyWorkflowMode, legacyPermissionLevel := sessionModesFromLegacy(payload.PermissionMode)
		if strings.TrimSpace(payload.WorkflowMode) == "" {
			workflowMode = legacyWorkflowMode
		}
		if strings.TrimSpace(payload.PermissionLevel) == "" {
			permissionLevel = legacyPermissionLevel
		}
	}

	summary, err := m.CreateSession(ctx, CreateParams{
		ProjectID:       payload.ProjectID,
		WorktreeID:      payload.WorktreeID,
		Agent:           Agent(payload.Agent),
		Model:           payload.Model,
		ReasoningEffort: ReasoningEffort(payload.ReasoningEffort),
		WorkflowMode:    workflowMode,
		PermissionLevel: permissionLevel,
		Title:           payload.Title,
	})
	if err != nil {
		return client.send(newErrorFrame(frame.RequestID, "", "bad_req", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, summary.ID, nil)); err != nil {
		return err
	}
	snap, err := m.Snapshot(ctx, summary.ID, DefaultHistoryWindow)
	if err != nil {
		return client.send(newErrorFrame(frame.RequestID, summary.ID, "internal", err.Error(), false))
	}
	return client.send(newSnapshotFrame(summary.ID, snap))
}

func (m *Manager) handleConnectCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	snap, err := m.Snapshot(ctx, frame.SessionID, DefaultHistoryWindow)
	if err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "not_found", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return client.send(newSnapshotFrame(frame.SessionID, snap))
}

func (m *Manager) handleHistoryCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		Limit int `json:"lim"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid history payload", false))
	}
	beforeSeq, err := parseBeforeCursor(frame.Payload)
	if err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid history cursor", false))
	}
	window, err := m.History(ctx, frame.SessionID, payload.Limit, beforeSeq)
	if err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "not_found", err.Error(), false))
	}
	events := make([]wireEvent, 0, len(window.Events))
	for _, event := range window.Events {
		events = append(events, mapWireEvent(event))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return client.send(newEventFrame(frame.SessionID, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "hist_ch",
		Timestamp: time.Now(),
		Payload: map[string]any{
			"evs": events,
			"hm":  window.HasMore,
			"bc":  window.BeforeCursor,
		},
	}))
}

func (m *Manager) handleAbortCommand(_ context.Context, client *client, frame wireCommandFrame) error {
	if err := m.AbortSession(frame.SessionID); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "invalid_state", err.Error(), false))
	}
	return client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil))
}

func (m *Manager) handleApprovalCommand(client *client, frame wireCommandFrame, action string) error {
	if err := m.respondToApproval(frame.SessionID, action); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "invalid_state", err.Error(), false))
	}
	return client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil))
}

func (m *Manager) handleUserInputCommand(client *client, frame wireCommandFrame) error {
	var payload struct {
		ItemID  string              `json:"iid"`
		Answers map[string][]string `json:"ans"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid user input payload", false))
	}
	if err := m.respondToUserInput(frame.SessionID, payload.ItemID, payload.Answers); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "invalid_state", err.Error(), false))
	}
	return client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil))
}

func (m *Manager) handleRenameCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		Title string `json:"ttl"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid rename payload", false))
	}
	summary, err := m.RenameSession(ctx, frame.SessionID, payload.Title)
	if err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return m.broadcastSnapshot(ctx, summary.ID)
}

func (m *Manager) handleSetModelCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		Model string `json:"md"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid model payload", false))
	}
	if _, err := m.UpdateModel(ctx, frame.SessionID, payload.Model); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return m.broadcastSnapshot(ctx, frame.SessionID)
}

func (m *Manager) handleSetReasoningEffortCommand(
	ctx context.Context,
	client *client,
	frame wireCommandFrame,
) error {
	var payload struct {
		ReasoningEffort string `json:"re"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid reasoning payload", false))
	}
	if _, err := m.UpdateReasoningEffort(ctx, frame.SessionID, ReasoningEffort(payload.ReasoningEffort)); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return m.broadcastSnapshot(ctx, frame.SessionID)
}

func (m *Manager) handleSetWorkflowModeCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		WorkflowMode string `json:"wm"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid workflow payload", false))
	}
	if _, err := m.UpdateWorkflowMode(ctx, frame.SessionID, WorkflowMode(payload.WorkflowMode)); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return m.broadcastSnapshot(ctx, frame.SessionID)
}

func (m *Manager) handleSetPermissionLevelCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		PermissionLevel string `json:"pl"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid permission payload", false))
	}
	if _, err := m.UpdatePermissionLevel(ctx, frame.SessionID, PermissionLevel(payload.PermissionLevel)); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return m.broadcastSnapshot(ctx, frame.SessionID)
}

func (m *Manager) handleLegacySetModeCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		PermissionMode string `json:"pm"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid legacy mode payload", false))
	}
	workflowMode, permissionLevel := sessionModesFromLegacy(payload.PermissionMode)
	if _, err := m.UpdateWorkflowMode(ctx, frame.SessionID, workflowMode); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", err.Error(), false))
	}
	if _, err := m.UpdatePermissionLevel(ctx, frame.SessionID, permissionLevel); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return m.broadcastSnapshot(ctx, frame.SessionID)
}

func (m *Manager) handleSetAgentCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		Agent string `json:"ag"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid agent payload", false))
	}
	if _, err := m.UpdateAgent(ctx, frame.SessionID, Agent(payload.Agent)); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return m.broadcastSnapshot(ctx, frame.SessionID)
}

func (m *Manager) handleMoveCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		PrevSessionID string `json:"prv"`
		NextSessionID string `json:"nxt"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid move payload", false))
	}
	summary, err := m.MoveSession(ctx, frame.SessionID, payload.PrevSessionID, payload.NextSessionID)
	if err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return m.broadcastSnapshot(ctx, summary.ID)
}

func (m *Manager) handleDeleteCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	if err := m.DeleteSession(ctx, frame.SessionID); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "internal", err.Error(), false))
	}
	return client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil))
}

func (m *Manager) handleListCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		ProjectID string `json:"pid"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid list payload", false))
	}
	items, err := m.ListSessions(ctx, payload.ProjectID)
	if err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "internal", err.Error(), false))
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id":  item.ID,
			"ttl": item.Title,
			"ag":  item.Agent,
			"st":  item.Status,
			"oi":  item.OrderIndex,
			"lu":  item.UpdatedAt.UnixMilli(),
		})
	}
	return client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, map[string]any{"items": result}))
}

func (m *Manager) handleSendCommand(ctx context.Context, client *client, frame wireCommandFrame) error {
	var payload struct {
		Text        string   `json:"txt"`
		Attachments []string `json:"atts"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid send payload", false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	if err := m.SendMessage(ctx, frame.SessionID, payload.Text, payload.Attachments); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "invalid_state", err.Error(), false))
	}
	return nil
}

func (m *Manager) SendMessage(ctx context.Context, sessionID, text string, attachmentIDs []string) error {
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if m.hasActiveRun(sessionID) {
		return fmt.Errorf("session is already running")
	}

	text = strings.TrimSpace(text)
	attachments := make([]Attachment, 0, len(attachmentIDs))
	for _, id := range attachmentIDs {
		attachment, err := m.loadAttachment(strings.TrimSpace(id))
		if err != nil {
			return fmt.Errorf("attachment %s not found", id)
		}
		attachments = append(attachments, attachment)
	}
	if text == "" && len(attachments) == 0 {
		return fmt.Errorf("message is empty")
	}

	runID := utils.NewID()
	userMessageID := utils.NewID()

	if _, err := m.appendAndBroadcast(ctx, sessionID, record, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "msg_u",
		RunID:     runID,
		ParentID:  userMessageID,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"mid":  userMessageID,
			"txt":  text,
			"atts": attachmentPayloads(attachments),
		},
	}); err != nil {
		return err
	}
	if _, err := m.appendAndBroadcast(ctx, sessionID, record, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "run_st",
		RunID:     runID,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"ag": string(normalizeAgent(Agent(record.Agent))),
			"md": record.Model,
			"re": record.ReasoningEffort,
			"wm": effectiveWorkflowMode(record),
			"pl": effectivePermissionLevel(record),
		},
	}); err != nil {
		return err
	}

	now := time.Now()
	markStatus := StatusRunning
	updates := map[string]any{
		"status":          string(markStatus),
		"has_unread":      false,
		"last_error":      nil,
		"updated_at":      now,
		"last_message_at": now,
	}
	titleChanged := false
	if record.TitleAuto {
		if autoTitle := deriveAutoTitleFromMessage(text); autoTitle != "" {
			updates["title_auto"] = false
			if strings.TrimSpace(record.Title) != autoTitle {
				updates["title"] = autoTitle
				titleChanged = true
			}
		}
	}

	if err := model.GetDB().WithContext(ctx).Model(&tables.WebSessionTable{}).
		Where("id = ?", sessionID).
		Updates(updates).Error; err != nil {
		return err
	}
	if titleChanged {
		if err := m.broadcastSnapshot(ctx, sessionID); err != nil {
			m.logger.Warn("failed to broadcast auto-renamed session title",
				zap.String("sessionId", sessionID),
				zap.Error(err),
			)
		}
	}

	runCtx, cancel := context.WithCancel(context.Background())
	run := &activeRun{
		sessionID: sessionID,
		agent:     Agent(record.Agent),
		backend:   effectiveSessionBackend(record),
		runID:     runID,
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	m.mu.Lock()
	m.runs[sessionID] = run
	m.mu.Unlock()

	go m.runSession(runCtx, run, record, text, attachments)
	return nil
}

func (m *Manager) runSession(ctx context.Context, run *activeRun, session tables.WebSessionTable, text string, attachments []Attachment) {
	defer func() {
		run.closeInput()
		run.clearPendingApproval()
		run.clearPendingServerRequest()
		close(run.done)
		m.mu.Lock()
		delete(m.runs, session.ID)
		m.mu.Unlock()
	}()

	if run.backend == SessionBackendCodexAppServer && normalizeAgent(Agent(session.Agent)) == AgentCodex {
		m.runCodexAppServerSession(ctx, run, session, text, attachments)
		return
	}

	cmd, stdinBytes, closeStdinAfterWrite, err := m.buildExecCommand(ctx, session, text, attachments)
	if err != nil {
		m.handleRunFailure(session.ID, session, run, err)
		return
	}
	run.cmd = cmd

	stdin, err := cmd.StdinPipe()
	if err != nil {
		m.handleRunFailure(session.ID, session, run, err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.handleRunFailure(session.ID, session, run, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.handleRunFailure(session.ID, session, run, err)
		return
	}

	if err := cmd.Start(); err != nil {
		m.handleRunFailure(session.ID, session, run, err)
		return
	}
	run.setInput(stdin)

	go func() {
		if len(stdinBytes) > 0 {
			_, _ = stdin.Write(stdinBytes)
		}
		if closeStdinAfterWrite {
			_ = stdin.Close()
			run.clearInput()
		}
	}()

	stderrBuffer := bytes.NewBuffer(nil)
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		m.consumeRuntimePlainOutput(ctx, session, run, io.TeeReader(stderr, stderrBuffer))
	}()

	m.consumeRuntimeOutput(ctx, session, run, stdout)

	waitErr := cmd.Wait()
	<-stderrDone
	if ctx.Err() != nil {
		_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
			ID:        utils.NewID(),
			Seq:       0,
			Type:      "run_abort",
			RunID:     run.runID,
			Timestamp: time.Now(),
		})
		_ = m.updateRuntimeState(context.Background(), session.ID, map[string]any{
			"status":     string(StatusIdle),
			"updated_at": time.Now(),
		})
		return
	}

	if waitErr != nil {
		message := strings.TrimSpace(run.lastError)
		if message == "" {
			message = strings.TrimSpace(stderrBuffer.String())
		}
		if message == "" {
			message = waitErr.Error()
		}
		m.handleRunFailure(session.ID, session, run, errors.New(message))
		return
	}

	if run.assistantMessageID != "" {
		_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
			ID:        utils.NewID(),
			Seq:       0,
			Type:      "txt_end",
			RunID:     run.runID,
			ParentID:  run.assistantMessageID,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"mid": run.assistantMessageID,
			},
		})
	}
	_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "run_done",
		RunID:     run.runID,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"ok": true,
		},
	})
	_ = m.updateRuntimeState(context.Background(), session.ID, map[string]any{
		"status":     string(StatusDone),
		"updated_at": time.Now(),
	})
}

func (m *Manager) handleRunFailure(sessionID string, session tables.WebSessionTable, run *activeRun, err error) {
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "runtime failed"
	}
	run.lastError = message
	_, _ = m.appendAndBroadcast(context.Background(), sessionID, session, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "run_fail",
		RunID:     run.runID,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"code": "runtime_error",
			"msg":  message,
		},
	})
	_ = m.updateRuntimeState(context.Background(), sessionID, map[string]any{
		"status":     string(StatusError),
		"last_error": message,
		"updated_at": time.Now(),
	})
}

func (m *Manager) consumeRuntimeOutput(ctx context.Context, session tables.WebSessionTable, run *activeRun, stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	const maxLine = 1024 * 1024 * 8
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, maxLine)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			m.handleRuntimePlainLine(session, run, string(line))
			continue
		}
		switch run.agent {
		case AgentClaude:
			m.handleClaudeEvent(session, run, raw)
		case AgentCodex:
			m.handleCodexEvent(session, run, raw)
		}
	}
}

func (m *Manager) consumeRuntimePlainOutput(ctx context.Context, session tables.WebSessionTable, run *activeRun, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	const maxLine = 1024 * 1024 * 2
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, maxLine)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		m.handleRuntimePlainLine(session, run, scanner.Text())
	}
}

func (m *Manager) handleRuntimePlainLine(session tables.WebSessionTable, run *activeRun, line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	recent := run.pushRuntimeLine(trimmed)
	prompt, ok := detectApprovalPrompt(recent)
	if !ok {
		return
	}
	if !run.setPendingApproval(prompt) {
		return
	}
	_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "approval_req",
		RunID:     run.runID,
		ParentID:  run.assistantMessageID,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"prompt": prompt,
		},
	})
}

func (m *Manager) handleClaudeEvent(session tables.WebSessionTable, run *activeRun, raw map[string]any) {
	eventType, _ := raw["type"].(string)
	switch eventType {
	case "system":
		if sessionID, _ := raw["session_id"].(string); sessionID != "" {
			_ = m.updateRuntimeState(context.Background(), session.ID, map[string]any{
				"native_session_id": sessionID,
				"updated_at":        time.Now(),
			})
		}
	case "assistant":
		message, _ := raw["message"].(map[string]any)
		content, _ := message["content"].([]any)
		if run.assistantMessageID == "" {
			run.assistantMessageID = utils.NewID()
			_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
				ID:        utils.NewID(),
				Seq:       0,
				Type:      "msg_a_st",
				RunID:     run.runID,
				ParentID:  run.assistantMessageID,
				Timestamp: time.Now(),
				Payload: map[string]any{
					"mid": run.assistantMessageID,
				},
			})
		}

		for _, item := range content {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			switch blockType {
			case "text":
				text, _ := block["text"].(string)
				if strings.TrimSpace(text) == "" {
					continue
				}
				_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
					ID:        utils.NewID(),
					Seq:       0,
					Type:      "txt_d",
					RunID:     run.runID,
					ParentID:  run.assistantMessageID,
					Timestamp: time.Now(),
					Payload: map[string]any{
						"mid": run.assistantMessageID,
						"txt": text,
					},
				})
			case "tool_use":
				toolID, _ := block["id"].(string)
				if toolID == "" {
					toolID = utils.NewID()
				}
				run.currentToolMessage = toolID
				_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
					ID:        utils.NewID(),
					Seq:       0,
					Type:      "tool_st",
					RunID:     run.runID,
					ParentID:  run.assistantMessageID,
					Timestamp: time.Now(),
					Payload: map[string]any{
						"tid":  toolID,
						"name": stringValue(block["name"]),
						"kind": "tool_use",
						"in":   block["input"],
					},
				})
			case "tool_result":
				toolUseID, _ := block["tool_use_id"].(string)
				contentText := claudeToolResultText(block["content"])
				_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
					ID:        utils.NewID(),
					Seq:       0,
					Type:      "tool_end",
					RunID:     run.runID,
					ParentID:  run.assistantMessageID,
					Timestamp: time.Now(),
					Payload: map[string]any{
						"tid": toolUseID,
						"out": truncateString(contentText, 4000),
						"ok":  true,
					},
				})
			}
		}
	case "result":
		if sessionID, _ := raw["session_id"].(string); sessionID != "" {
			_ = m.updateRuntimeState(context.Background(), session.ID, map[string]any{
				"native_session_id": sessionID,
				"updated_at":        time.Now(),
			})
		}
		totalCost, _ := raw["total_cost_usd"].(float64)
		if totalCost > 0 {
			_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
				ID:        utils.NewID(),
				Seq:       0,
				Type:      "usage",
				RunID:     run.runID,
				Timestamp: time.Now(),
				Payload: map[string]any{
					"in":   session.TotalInputTokens,
					"cin":  session.TotalCachedInputTokens,
					"out":  session.TotalOutputTokens,
					"cost": totalCost,
				},
			})
			_ = model.GetDB().WithContext(context.Background()).
				Model(&tables.WebSessionTable{}).
				Where("id = ?", session.ID).
				Updates(map[string]any{
					"total_cost": gorm.Expr("total_cost + ?", totalCost),
					"updated_at": time.Now(),
				}).Error
		}
	case "error":
		run.lastError = stringValue(raw["message"])
	}
}

func (m *Manager) handleCodexEvent(session tables.WebSessionTable, run *activeRun, raw map[string]any) {
	eventType, _ := raw["type"].(string)
	switch eventType {
	case "thread.started":
		if threadID, _ := raw["thread_id"].(string); threadID != "" {
			_ = m.updateRuntimeState(context.Background(), session.ID, map[string]any{
				"native_session_id": threadID,
				"updated_at":        time.Now(),
			})
		}
	case "item.started":
		item, _ := raw["item"].(map[string]any)
		if stringValue(item["type"]) == "agent_message" {
			return
		}
		toolID := stringValue(item["id"])
		if toolID == "" {
			toolID = utils.NewID()
		}
		_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
			ID:        utils.NewID(),
			Seq:       0,
			Type:      "tool_st",
			RunID:     run.runID,
			ParentID:  run.assistantMessageID,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"tid":  toolID,
				"name": codexToolName(item),
				"kind": stringValue(item["type"]),
				"in":   codexToolInput(item),
				"meta": codexToolMeta(item),
			},
		})
	case "item.completed":
		item, _ := raw["item"].(map[string]any)
		if stringValue(item["type"]) == "agent_message" {
			if run.assistantMessageID == "" {
				run.assistantMessageID = utils.NewID()
				_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
					ID:        utils.NewID(),
					Seq:       0,
					Type:      "msg_a_st",
					RunID:     run.runID,
					ParentID:  run.assistantMessageID,
					Timestamp: time.Now(),
					Payload: map[string]any{
						"mid": run.assistantMessageID,
					},
				})
			}
			text := stringValue(item["text"])
			if strings.TrimSpace(text) != "" {
				_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
					ID:        utils.NewID(),
					Seq:       0,
					Type:      "txt_d",
					RunID:     run.runID,
					ParentID:  run.assistantMessageID,
					Timestamp: time.Now(),
					Payload: map[string]any{
						"mid": run.assistantMessageID,
						"txt": text,
					},
				})
			}
			return
		}
		toolID := stringValue(item["id"])
		if toolID == "" {
			toolID = utils.NewID()
		}
		_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
			ID:        utils.NewID(),
			Seq:       0,
			Type:      "tool_end",
			RunID:     run.runID,
			ParentID:  run.assistantMessageID,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"tid":  toolID,
				"out":  truncateString(codexToolResult(item), 4000),
				"ok":   true,
				"meta": codexToolMeta(item),
			},
		})
	case "turn.completed":
		usage, _ := raw["usage"].(map[string]any)
		in := int64(numberValue(usage["input_tokens"]))
		cin := int64(numberValue(usage["cached_input_tokens"]))
		out := int64(numberValue(usage["output_tokens"]))
		_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
			ID:        utils.NewID(),
			Seq:       0,
			Type:      "usage",
			RunID:     run.runID,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"in":  in,
				"cin": cin,
				"out": out,
			},
		})
		_ = model.GetDB().WithContext(context.Background()).
			Model(&tables.WebSessionTable{}).
			Where("id = ?", session.ID).
			Updates(map[string]any{
				"total_input_tokens":        gorm.Expr("total_input_tokens + ?", in),
				"total_cached_input_tokens": gorm.Expr("total_cached_input_tokens + ?", cin),
				"total_output_tokens":       gorm.Expr("total_output_tokens + ?", out),
				"updated_at":                time.Now(),
			}).Error
	case "turn.failed":
		errorMap, _ := raw["error"].(map[string]any)
		run.lastError = stringValue(errorMap["message"])
	case "error":
		run.lastError = stringValue(raw["message"])
	}
}

func (m *Manager) appendAndBroadcast(ctx context.Context, sessionID string, record tables.WebSessionTable, event Event) (Event, error) {
	seq, err := m.nextEventSeq(ctx, sessionID)
	if err != nil {
		return Event{}, err
	}
	event.Seq = seq
	if event.ID == "" {
		event.ID = utils.NewID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if err := m.store.appendEvent(sessionID, event); err != nil {
		return Event{}, err
	}

	update := map[string]any{
		"last_event_seq": seq,
		"updated_at":     time.Now(),
	}
	if event.Type != "msg_u" && event.Type != "hist_ch" {
		update["has_unread"] = true
	}
	if event.Type == "msg_u" {
		now := time.Now()
		update["last_message_at"] = now
	}
	if err := m.updateRuntimeState(ctx, sessionID, update); err != nil {
		return Event{}, err
	}

	m.broadcast(newEventFrame(sessionID, event))
	return event, nil
}

func (m *Manager) nextEventSeq(ctx context.Context, sessionID string) (int64, error) {
	var record tables.WebSessionTable
	if err := model.GetDB().WithContext(ctx).Select("id", "last_event_seq").First(&record, "id = ?", sessionID).Error; err != nil {
		return 0, err
	}
	return record.LastEventSeq + 1, nil
}

func (m *Manager) updateRuntimeState(ctx context.Context, sessionID string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	return model.GetDB().WithContext(ctx).Model(&tables.WebSessionTable{}).
		Where("id = ?", sessionID).
		Updates(updates).Error
}

func (m *Manager) updateFields(ctx context.Context, sessionID string, updates map[string]any) (SessionSummary, error) {
	if err := model.GetDB().WithContext(ctx).Model(&tables.WebSessionTable{}).
		Where("id = ?", sessionID).
		Updates(updates).Error; err != nil {
		return SessionSummary{}, err
	}
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}
	return mapSessionRecord(record), nil
}

func (m *Manager) getNextSessionOrderIndex(ctx context.Context, projectID string) (float64, error) {
	db := model.GetDB()
	if db == nil {
		return 0, model.ErrDBNotInitialized
	}

	var maxOrder float64
	if err := db.WithContext(ctx).
		Model(&tables.WebSessionTable{}).
		Where("project_id = ?", projectID).
		Select("COALESCE(MAX(order_index), 0)").
		Scan(&maxOrder).Error; err != nil {
		return 0, err
	}
	return maxOrder + sessionOrderStep, nil
}

func (m *Manager) listSessionRecordsWithDB(db *gorm.DB, projectID string) ([]tables.WebSessionTable, error) {
	query := db.Model(&tables.WebSessionTable{}).
		Order("order_index ASC").
		Order("updated_at DESC")
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	var records []tables.WebSessionTable
	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func resolveSessionInsertIndex(
	sessions []tables.WebSessionTable,
	sessionID string,
	prevSessionID string,
	nextSessionID string,
) (int, error) {
	prevSessionID = strings.TrimSpace(prevSessionID)
	nextSessionID = strings.TrimSpace(nextSessionID)
	if prevSessionID != "" && prevSessionID == nextSessionID {
		return 0, fmt.Errorf("invalid move target")
	}
	if prevSessionID == sessionID || nextSessionID == sessionID {
		return 0, fmt.Errorf("cannot move relative to itself")
	}

	findIndex := func(targetID string) int {
		for index, item := range sessions {
			if item.ID == targetID {
				return index
			}
		}
		return -1
	}

	if nextSessionID != "" {
		nextIndex := findIndex(nextSessionID)
		if nextIndex == -1 {
			return 0, fmt.Errorf("target session not found")
		}
		if prevSessionID != "" {
			prevIndex := findIndex(prevSessionID)
			if prevIndex == -1 {
				return 0, fmt.Errorf("target session not found")
			}
			if prevIndex >= nextIndex {
				return 0, fmt.Errorf("invalid move target")
			}
		}
		return nextIndex, nil
	}

	if prevSessionID != "" {
		prevIndex := findIndex(prevSessionID)
		if prevIndex == -1 {
			return 0, fmt.Errorf("target session not found")
		}
		return prevIndex + 1, nil
	}

	return 0, nil
}

func (m *Manager) resolveContext(ctx context.Context, projectID, worktreeID string) (*model.Project, string, string, error) {
	project, err := m.projectSvc.GetProject(ctx, projectID)
	if err != nil {
		return nil, "", "", err
	}

	if strings.TrimSpace(worktreeID) != "" {
		worktree, err := m.worktreeSvc.GetWorktree(ctx, worktreeID)
		if err != nil {
			return nil, "", "", err
		}
		if worktree.ProjectId != project.Id {
			return nil, "", "", fmt.Errorf("worktree does not belong to project")
		}
		return project, worktree.Id, worktree.Path, nil
	}

	worktrees, err := m.worktreeSvc.ListWorktrees(ctx, project.Id)
	if err == nil {
		for _, worktree := range worktrees {
			if worktree.IsMain {
				return project, worktree.Id, worktree.Path, nil
			}
		}
		if len(worktrees) > 0 {
			return project, worktrees[0].Id, worktrees[0].Path, nil
		}
	}
	return project, "", project.Path, nil
}

func (m *Manager) buildExecCommand(ctx context.Context, session tables.WebSessionTable, text string, attachments []Attachment) (*exec.Cmd, []byte, bool, error) {
	workflowMode := effectiveWorkflowMode(session)
	permissionLevel := effectivePermissionLevel(session)
	preparedText := preparePromptText(text, workflowMode)

	switch normalizeAgent(Agent(session.Agent)) {
	case AgentClaude:
		args := []string{"-p", "--output-format", "stream-json", "--verbose"}
		if len(attachments) > 0 {
			args = append(args, "--input-format", "stream-json")
		}
		switch permissionLevel {
		case PermissionLevelYolo:
			args = append(args, "--dangerously-skip-permissions")
		case PermissionLevelElevated:
			args = append(args, "--permission-mode", "acceptEdits")
		default:
			args = append(args, "--permission-mode", "default")
		}
		if session.NativeSessionID != nil && strings.TrimSpace(*session.NativeSessionID) != "" {
			args = append(args, "--resume", strings.TrimSpace(*session.NativeSessionID))
		}
		if strings.TrimSpace(session.Model) != "" {
			args = append(args, "--model", strings.TrimSpace(session.Model))
		}

		var stdin []byte
		if len(attachments) > 0 {
			content := make([]map[string]any, 0, len(attachments)+1)
			if strings.TrimSpace(preparedText) != "" {
				content = append(content, map[string]any{
					"type": "text",
					"text": preparedText,
				})
			}
			for _, attachment := range attachments {
				data, err := os.ReadFile(attachment.Path)
				if err != nil {
					return nil, nil, false, err
				}
				content = append(content, map[string]any{
					"type": "image",
					"source": map[string]any{
						"type":       "base64",
						"media_type": attachment.Mime,
						"data":       base64.StdEncoding.EncodeToString(data),
					},
				})
			}
			stdin, _ = json.Marshal(map[string]any{
				"type": "user",
				"message": map[string]any{
					"role":    "user",
					"content": content,
				},
			})
			stdin = append(stdin, '\n')
		} else {
			trimmedText := strings.TrimSpace(preparedText)
			if trimmedText != "" {
				args = append(args, trimmedText)
			}
		}
		cmd := exec.CommandContext(ctx, m.cfg.ClaudePath, args...)
		cmd.Dir = session.Cwd
		cmd.Env = os.Environ()
		return cmd, stdin, len(attachments) > 0, nil
	case AgentCodex:
		args := []string{"exec", "--json", "--skip-git-repo-check"}
		trimmedText := strings.TrimSpace(preparedText)
		useStdinPrompt := trimmedText == ""
		switch permissionLevel {
		case PermissionLevelYolo:
			args = append(args, "--dangerously-bypass-approvals-and-sandbox")
		case PermissionLevelElevated:
			args = append(args, "-s", "danger-full-access", "-c", `approval_policy="on-request"`)
		default:
			args = append(args, "-s", "workspace-write", "-c", `approval_policy="on-request"`)
		}
		if strings.TrimSpace(session.Model) != "" {
			args = append(args, "--model", strings.TrimSpace(session.Model))
		}
		if effort := normalizeReasoningEffort(ReasoningEffort(session.ReasoningEffort)); effort != ReasoningEffortDefault {
			args = append(args, "-c", fmt.Sprintf("reasoning_effort=%q", string(effort)))
		}
		for _, attachment := range attachments {
			args = append(args, "--image", attachment.Path)
		}
		if session.NativeSessionID != nil && strings.TrimSpace(*session.NativeSessionID) != "" {
			args = append(args, "resume")
			args = append(args, strings.TrimSpace(*session.NativeSessionID))
			if useStdinPrompt {
				args = append(args, "-")
			} else {
				args = append(args, trimmedText)
			}
		} else {
			if session.Cwd != "" {
				args = append(args, "-C", session.Cwd)
			}
			if useStdinPrompt {
				args = append(args, "-")
			} else {
				args = append(args, trimmedText)
			}
		}
		cmd := exec.CommandContext(ctx, m.cfg.CodexPath, args...)
		cmd.Dir = session.Cwd
		cmd.Env = os.Environ()
		if useStdinPrompt {
			return cmd, []byte(preparedText), true, nil
		}
		// Codex appends any piped stdin as an extra <stdin> block even when a prompt
		// argument is provided, so we must close stdin immediately for normal prompt runs.
		return cmd, nil, true, nil
	default:
		return nil, nil, false, fmt.Errorf("unsupported agent %q", session.Agent)
	}
}

func (m *Manager) respondToApproval(sessionID, action string) error {
	m.mu.RLock()
	run, ok := m.runs[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session is not running")
	}

	if pending, ok := run.pendingApprovalRequest(); ok {
		if run.app == nil {
			return fmt.Errorf("session approval channel is unavailable")
		}
		if err := run.app.respond(pending.RawID, approvalResponsePayload(pending, action)); err != nil {
			return err
		}
		run.clearPendingServerRequest()
		record, err := m.GetSession(context.Background(), sessionID)
		if err != nil {
			return err
		}
		_, _ = m.appendAndBroadcast(context.Background(), sessionID, record, Event{
			ID:        utils.NewID(),
			Seq:       0,
			Type:      "approval_res",
			RunID:     run.runID,
			ParentID:  run.assistantMessageID,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"act":    action,
				"prompt": pending.Prompt,
			},
		})
		return nil
	}

	prompt, ok := run.pendingApprovalPrompt()
	if !ok {
		return fmt.Errorf("no pending approval")
	}
	if err := run.writeInput(approvalInput(action)); err != nil {
		return err
	}
	run.clearPendingApproval()
	record, err := m.GetSession(context.Background(), sessionID)
	if err != nil {
		return err
	}
	_, _ = m.appendAndBroadcast(context.Background(), sessionID, record, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "approval_res",
		RunID:     run.runID,
		ParentID:  run.assistantMessageID,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"act":    action,
			"prompt": prompt,
		},
	})
	return nil
}

func (m *Manager) respondToUserInput(sessionID, itemID string, answers map[string][]string) error {
	m.mu.RLock()
	run, ok := m.runs[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session is not running")
	}
	pending, ok := run.pendingUserInputRequest()
	if !ok {
		return fmt.Errorf("no pending user input request")
	}
	if strings.TrimSpace(itemID) == "" || strings.TrimSpace(pending.ItemID) != strings.TrimSpace(itemID) {
		return fmt.Errorf("user input request does not match the active prompt")
	}
	if run.app == nil {
		return fmt.Errorf("session input channel is unavailable")
	}
	if err := run.app.respond(pending.RawID, userInputResponsePayload(answers)); err != nil {
		return err
	}
	run.clearPendingServerRequest()

	record, err := m.GetSession(context.Background(), sessionID)
	if err != nil {
		return err
	}
	_, _ = m.appendAndBroadcast(context.Background(), sessionID, record, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "user_input_res",
		RunID:     run.runID,
		ParentID:  run.assistantMessageID,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"iid": pending.ItemID,
			"ans": answers,
		},
	})
	return nil
}

func (m *Manager) hasActiveRun(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.runs[sessionID]
	return ok
}

func (m *Manager) broadcast(frame wireFrame) {
	m.mu.RLock()
	clients := make([]*client, 0, len(m.clients))
	for client := range m.clients {
		clients = append(clients, client)
	}
	m.mu.RUnlock()

	for _, client := range clients {
		if err := client.send(frame); err != nil {
			m.logger.Debug("failed to send ws frame", zap.Error(err))
		}
	}
}

func (m *Manager) broadcastSnapshot(ctx context.Context, sessionID string) error {
	snap, err := m.loadSnapshot(ctx, sessionID, DefaultHistoryWindow, false)
	if err != nil {
		return err
	}
	m.broadcast(newSnapshotFrame(sessionID, snap))
	return nil
}

func (c *client) send(frame wireFrame) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteJSON(frame)
}

func mapSessionRecord(record tables.WebSessionTable) SessionSummary {
	return SessionSummary{
		ID:              record.ID,
		ProjectID:       record.ProjectID,
		WorktreeID:      record.WorktreeID,
		OrderIndex:      record.OrderIndex,
		Agent:           Agent(record.Agent),
		Title:           record.Title,
		Model:           record.Model,
		ReasoningEffort: ReasoningEffort(record.ReasoningEffort),
		WorkflowMode:    effectiveWorkflowMode(record),
		PermissionLevel: effectivePermissionLevel(record),
		Cwd:             record.Cwd,
		NativeSessionID: record.NativeSessionID,
		Status:          Status(record.Status),
		HasUnread:       record.HasUnread,
		LastMessageAt:   record.LastMessageAt,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
		Usage: Usage{
			InputTokens:       record.TotalInputTokens,
			CachedInputTokens: record.TotalCachedInputTokens,
			OutputTokens:      record.TotalOutputTokens,
			Cost:              record.TotalCost,
		},
	}
}

func attachmentPayloads(items []Attachment) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id":   item.ID,
			"name": item.Name,
			"mime": item.Mime,
			"sz":   item.Size,
		})
	}
	return result
}

func defaultTitle(agent Agent, projectName string) string {
	prefix := "Chat"
	if normalizeAgent(agent) == AgentCodex {
		prefix = "Codex"
	} else if normalizeAgent(agent) == AgentClaude {
		prefix = "Claude"
	}
	if strings.TrimSpace(projectName) == "" {
		return prefix
	}
	return fmt.Sprintf("%s · %s", prefix, projectName)
}

func defaultModel(agent Agent, provided string) string {
	if strings.TrimSpace(provided) != "" {
		return strings.TrimSpace(provided)
	}
	if normalizeAgent(agent) == AgentCodex {
		return "gpt-5.4"
	}
	return "opus"
}

func defaultReasoningEffort(agent Agent, provided ReasoningEffort) ReasoningEffort {
	if normalized := normalizeReasoningEffort(provided); normalized != ReasoningEffortDefault {
		return normalized
	}
	if normalizeAgent(agent) == AgentCodex {
		return ReasoningEffortXHigh
	}
	return ReasoningEffortDefault
}

func defaultSessionBackend(agent Agent) SessionBackend {
	if normalizeAgent(agent) == AgentCodex {
		return SessionBackendCodexAppServer
	}
	return SessionBackendLegacyExec
}

func normalizeSessionBackend(backend SessionBackend, agent Agent) SessionBackend {
	switch strings.ToLower(strings.TrimSpace(string(backend))) {
	case string(SessionBackendCodexAppServer):
		if normalizeAgent(agent) == AgentCodex {
			return SessionBackendCodexAppServer
		}
		return SessionBackendLegacyExec
	case string(SessionBackendLegacyExec):
		return SessionBackendLegacyExec
	default:
		return defaultSessionBackend(agent)
	}
}

func normalizeAgent(agent Agent) Agent {
	switch agent {
	case AgentCodex:
		return AgentCodex
	default:
		return AgentClaude
	}
}

func normalizeReasoningEffort(effort ReasoningEffort) ReasoningEffort {
	switch strings.ToLower(strings.TrimSpace(string(effort))) {
	case string(ReasoningEffortNone):
		return ReasoningEffortNone
	case string(ReasoningEffortLow):
		return ReasoningEffortLow
	case string(ReasoningEffortMedium):
		return ReasoningEffortMedium
	case string(ReasoningEffortHigh):
		return ReasoningEffortHigh
	case string(ReasoningEffortXHigh):
		return ReasoningEffortXHigh
	default:
		return ReasoningEffortDefault
	}
}

func normalizeWorkflowMode(mode WorkflowMode) WorkflowMode {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case string(WorkflowModePlan):
		return WorkflowModePlan
	default:
		return WorkflowModeDefault
	}
}

func normalizePermissionLevel(level PermissionLevel) PermissionLevel {
	switch strings.ToLower(strings.TrimSpace(string(level))) {
	case string(PermissionLevelDefault):
		return PermissionLevelDefault
	case string(PermissionLevelYolo):
		return PermissionLevelYolo
	default:
		return PermissionLevelElevated
	}
}

func sessionModesFromLegacy(legacy string) (WorkflowMode, PermissionLevel) {
	switch strings.ToLower(strings.TrimSpace(legacy)) {
	case "plan":
		return WorkflowModePlan, PermissionLevelElevated
	case "yolo":
		return WorkflowModeDefault, PermissionLevelYolo
	default:
		return WorkflowModeDefault, PermissionLevelElevated
	}
}

func effectiveWorkflowMode(record tables.WebSessionTable) WorkflowMode {
	if normalized := normalizeWorkflowMode(WorkflowMode(record.WorkflowMode)); normalized != WorkflowModeDefault ||
		strings.EqualFold(strings.TrimSpace(record.WorkflowMode), string(WorkflowModeDefault)) {
		return normalized
	}
	workflowMode, _ := sessionModesFromLegacy(record.LegacyPermissionMode)
	return workflowMode
}

func effectivePermissionLevel(record tables.WebSessionTable) PermissionLevel {
	if normalized := normalizePermissionLevel(PermissionLevel(record.PermissionLevel)); normalized != PermissionLevelElevated ||
		strings.EqualFold(strings.TrimSpace(record.PermissionLevel), string(PermissionLevelElevated)) ||
		strings.EqualFold(strings.TrimSpace(record.PermissionLevel), string(PermissionLevelDefault)) ||
		strings.EqualFold(strings.TrimSpace(record.PermissionLevel), string(PermissionLevelYolo)) {
		return normalized
	}
	_, permissionLevel := sessionModesFromLegacy(record.LegacyPermissionMode)
	return permissionLevel
}

func effectiveSessionBackend(record tables.WebSessionTable) SessionBackend {
	normalized := strings.ToLower(strings.TrimSpace(record.Backend))
	switch normalized {
	case string(SessionBackendLegacyExec):
		return SessionBackendLegacyExec
	case string(SessionBackendCodexAppServer):
		if normalizeAgent(Agent(record.Agent)) == AgentCodex {
			return SessionBackendCodexAppServer
		}
		return SessionBackendLegacyExec
	default:
		if normalizeAgent(Agent(record.Agent)) == AgentCodex {
			// Existing Codex sessions predate backend persistence and must continue
			// using the legacy exec transport unless explicitly migrated.
			return SessionBackendLegacyExec
		}
		return SessionBackendLegacyExec
	}
}

func preparePromptText(text string, workflowMode WorkflowMode) string {
	trimmedText := strings.TrimSpace(text)
	if normalizeWorkflowMode(workflowMode) != WorkflowModePlan {
		return trimmedText
	}
	if trimmedText == "" {
		return planPromptPreamble
	}
	return fmt.Sprintf("%s\n\nUser request:\n%s", planPromptPreamble, trimmedText)
}

func (m *Manager) migrateLegacySessionModes(ctx context.Context) error {
	db := model.GetDB()
	if db == nil {
		return model.ErrDBNotInitialized
	}

	var records []tables.WebSessionTable
	if err := db.WithContext(ctx).
		Select("id", "workflow_mode", "permission_level", "permission_mode").
		Find(&records).Error; err != nil {
		return err
	}

	for _, record := range records {
		updates := map[string]any{}
		legacyMode := strings.ToLower(strings.TrimSpace(record.LegacyPermissionMode))
		workflowMode, permissionLevel := sessionModesFromLegacy(record.LegacyPermissionMode)
		hasBootstrapDefaults := normalizeWorkflowMode(WorkflowMode(record.WorkflowMode)) == WorkflowModeDefault &&
			normalizePermissionLevel(PermissionLevel(record.PermissionLevel)) == PermissionLevelElevated

		if strings.TrimSpace(record.WorkflowMode) == "" || (hasBootstrapDefaults && legacyMode == "plan") {
			updates["workflow_mode"] = string(workflowMode)
		}
		if strings.TrimSpace(record.PermissionLevel) == "" || (hasBootstrapDefaults && legacyMode == "yolo") {
			updates["permission_level"] = string(permissionLevel)
		}
		if len(updates) == 0 {
			continue
		}
		updates["updated_at"] = time.Now()
		if err := db.WithContext(ctx).
			Model(&tables.WebSessionTable{}).
			Where("id = ?", record.ID).
			Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func nilIfEmpty(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (r *activeRun) setInput(stdin io.WriteCloser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stdin = stdin
}

func (r *activeRun) clearInput() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stdin = nil
}

func (r *activeRun) closeInput() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stdin != nil {
		_ = r.stdin.Close()
		r.stdin = nil
	}
}

func (r *activeRun) writeInput(input string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stdin == nil {
		return fmt.Errorf("session input is unavailable")
	}
	_, err := io.WriteString(r.stdin, input)
	return err
}

func (r *activeRun) pushRuntimeLine(line string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recentRuntimeLines = append(r.recentRuntimeLines, strings.TrimSpace(line))
	if len(r.recentRuntimeLines) > 6 {
		r.recentRuntimeLines = append([]string(nil), r.recentRuntimeLines[len(r.recentRuntimeLines)-6:]...)
	}
	return append([]string(nil), r.recentRuntimeLines...)
}

func (r *activeRun) setPendingApproval(prompt string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || prompt == r.pendingApproval {
		return false
	}
	r.pendingApproval = prompt
	return true
}

func (r *activeRun) pendingApprovalPrompt() (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(r.pendingApproval) == "" {
		return "", false
	}
	return r.pendingApproval, true
}

func (r *activeRun) clearPendingApproval() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pendingApproval = ""
}

func (r *activeRun) setPendingServerRequest(request *pendingServerRequest) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if request == nil {
		return false
	}
	r.pendingServerReq = request
	return true
}

func (r *activeRun) pendingApprovalRequest() (*pendingServerRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pendingServerReq == nil || !r.pendingServerReq.isApproval() {
		return nil, false
	}
	return r.pendingServerReq.clone(), true
}

func (r *activeRun) pendingUserInputRequest() (*pendingServerRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pendingServerReq == nil || r.pendingServerReq.Kind != pendingServerRequestUserInput {
		return nil, false
	}
	return r.pendingServerReq.clone(), true
}

func (r *activeRun) clearPendingServerRequest() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pendingServerReq = nil
}

func (r *activeRun) markAssistantDeltaSeen(messageID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.assistantDeltaSeen == nil {
		r.assistantDeltaSeen = make(map[string]bool)
	}
	if strings.TrimSpace(messageID) == "" {
		return
	}
	r.assistantDeltaSeen[messageID] = true
}

func (r *activeRun) assistantDeltaWasSeen(messageID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.assistantDeltaSeen != nil && r.assistantDeltaSeen[strings.TrimSpace(messageID)]
}

func detectApprovalPrompt(lines []string) (string, bool) {
	if len(lines) == 0 {
		return "", false
	}
	joined := strings.Join(filterNonEmptyLines(lines), "\n")
	if strings.Contains(joined, "Press enter to confirm or esc to cancel") {
		return joinTrailingLines(lines, 3), true
	}
	if strings.Contains(joined, "Ready to submit your answers?") {
		return joinTrailingLines(lines, 4), true
	}
	if strings.Contains(joined, "Do you want to proceed?") {
		return joinTrailingLines(lines, 4), true
	}
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Do you want to ") {
			return joinTrailingLines(lines, 4), true
		}
	}
	return "", false
}

func joinTrailingLines(lines []string, limit int) string {
	filtered := filterNonEmptyLines(lines)
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return strings.Join(filtered, "\n")
}

func filterNonEmptyLines(lines []string) []string {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func approvalInput(action string) string {
	if action == "reject" {
		return "\x1b"
	}
	return "\n"
}

func getenvDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func truncateString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func claudeToolResultText(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case []any:
		lines := make([]string, 0, len(value))
		for _, item := range value {
			switch part := item.(type) {
			case map[string]any:
				if text := stringValue(part["text"]); text != "" {
					lines = append(lines, text)
				}
			case string:
				lines = append(lines, part)
			}
		}
		return strings.Join(lines, "\n")
	default:
		encoded, _ := json.Marshal(raw)
		return string(encoded)
	}
}

func codexToolName(item map[string]any) string {
	switch normalizeCodexItemType(stringValue(item["type"])) {
	case "command_execution":
		return "CommandExecution"
	case "mcp_tool_call":
		return "McpToolCall"
	case "file_change":
		return "FileChange"
	case "reasoning":
		return "Reasoning"
	default:
		return stringValue(item["type"])
	}
}

func codexToolInput(item map[string]any) any {
	switch normalizeCodexItemType(stringValue(item["type"])) {
	case "command_execution":
		return map[string]any{"command": stringValue(item["command"])}
	case "reasoning":
		return nil
	}
	return item
}

func codexToolMeta(item map[string]any) map[string]any {
	return map[string]any{
		"kind":  normalizeCodexItemType(stringValue(item["type"])),
		"title": codexToolName(item),
		"subtitle": firstNonEmpty(
			stringValue(item["command"]),
			stringValue(item["tool_name"]),
			stringValue(item["path"]),
			stringValue(item["text"]),
		),
	}
}

func codexToolResult(item map[string]any) string {
	if normalizeCodexItemType(stringValue(item["type"])) == "reasoning" {
		if text := extractReasoningText(item); text != "" {
			return text
		}
		return ""
	}
	if output := stringValue(item["aggregated_output"]); output != "" {
		return output
	}
	if text := stringValue(item["text"]); text != "" {
		return text
	}
	encoded, _ := json.Marshal(item)
	return string(encoded)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func numberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeCodexItemType(value string) string {
	switch strings.TrimSpace(value) {
	case "commandExecution":
		return "command_execution"
	case "mcpToolCall":
		return "mcp_tool_call"
	case "fileChange":
		return "file_change"
	case "agentMessage":
		return "agent_message"
	case "userMessage":
		return "user_message"
	default:
		return strings.TrimSpace(value)
	}
}

func extractReasoningText(item map[string]any) string {
	sections := make([]string, 0, 2)
	if summary := strings.TrimSpace(strings.Join(collectReasoningFragments(item["summary"]), "")); summary != "" {
		sections = append(sections, summary)
	}
	if content := strings.TrimSpace(strings.Join(collectReasoningFragments(item["content"]), "")); content != "" {
		sections = append(sections, content)
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func collectReasoningFragments(raw any) []string {
	switch typed := raw.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{typed}
	case []any:
		fragments := make([]string, 0, len(typed))
		for _, item := range typed {
			fragments = append(fragments, collectReasoningFragments(item)...)
		}
		return fragments
	case map[string]any:
		fragments := make([]string, 0, 2)
		for _, key := range []string{"text", "delta"} {
			if text := stringValue(typed[key]); strings.TrimSpace(text) != "" {
				fragments = append(fragments, text)
			}
		}
		for _, key := range []string{"summary", "content"} {
			if nested := typed[key]; nested != nil {
				fragments = append(fragments, collectReasoningFragments(nested)...)
			}
		}
		return fragments
	default:
		return nil
	}
}
