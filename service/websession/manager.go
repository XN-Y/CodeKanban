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
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"code-kanban/model"
	"code-kanban/model/tables"
	"code-kanban/service"
	"code-kanban/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	DefaultHistoryWindow          = 80
	MaxHistoryWindow              = 120
	sessionOrderStep              = 1000.0
	defaultToolOutputLimit        = 4000
	planPromptPreamble            = "You are operating in planning mode. Inspect the project first, summarize the goal, and propose a concrete plan before making changes. Do not mutate files until the user confirms execution or explicitly asks you to proceed immediately. If additional permissions are needed, call them out explicitly."
	recoveryReasonProcessRestart  = "process_restart"
	recoveryMessageProcessRestart = "Session runtime was interrupted because the app restarted. Send a new message to continue."
)

var (
	webSessionHeartbeatInterval = 15 * time.Second
	webSessionHeartbeatTimeout  = 45 * time.Second
)

type Config struct {
	DataDir                 string
	AttachmentSizeLimit     int64
	ClaudePath              string
	CodexPath               string
	DefaultCodexSyncMode    func() SyncMode
	ActiveCallTimeoutConfig func() utils.WebSessionActiveCallTimeoutConfig
}

type Manager struct {
	cfg          Config
	logger       *zap.Logger
	store        *store
	projectSvc   *model.ProjectService
	worktreeSvc  *service.WorktreeService
	aiSessionSvc *service.AISessionService

	mu                 sync.RWMutex
	runs               map[string]*activeRun
	clients            map[*client]struct{}
	autoRetryTimers    map[string]*time.Timer
	pendingInputs      map[string][]PendingInput
	pendingProcessing  map[string]bool
	pendingDirty       map[string]bool
	codexContextWindow codexContextWindowResolver
}

type clientKind string

const (
	clientKindCommand clientKind = "command"
	clientKindEvent   clientKind = "event"
)

var ErrSessionHistoryUnavailable = errors.New("session history not found")

type client struct {
	conn       wsConn
	logger     *zap.Logger
	kind       clientKind
	writeMu    sync.Mutex
	focusMu    sync.RWMutex
	focusedSID string
	done       chan struct{}
	once       sync.Once
	lastSeenAt atomic.Int64
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
	lastErrorCode      string
	transportRetrySeen bool
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
	completedPlanTool  bool
	commandGroupID     string
	commandGroupKind   string
	commandGroupFirst  int64
	commandGroupCount  int
	commandGroupTools  map[string]struct{}
	abortPayload       map[string]any
	activeCalls        map[string]trackedActiveCall
	activeCallPausedAt *time.Time
	activeCallTimer    *time.Timer
	activeCallInFlight bool
}

type attachmentMeta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"createdAt"`
}

func normalizeAssistantState(state AssistantState) AssistantState {
	switch strings.ToLower(strings.TrimSpace(string(state))) {
	case string(AssistantStateWorking):
		return AssistantStateWorking
	case string(AssistantStateWaitingApproval):
		return AssistantStateWaitingApproval
	case string(AssistantStateWaitingInput):
		return AssistantStateWaitingInput
	case string(AssistantStateWaitingPlanApproval):
		return AssistantStateWaitingPlanApproval
	default:
		return AssistantStateNone
	}
}

func normalizeAutoRetryScope(scope AutoRetryScope) AutoRetryScope {
	switch strings.ToLower(strings.TrimSpace(string(scope))) {
	case string(AutoRetryScopeNetworkAndRateLimit):
		return AutoRetryScopeNetworkAndRateLimit
	case string(AutoRetryScopeAllFailures):
		return AutoRetryScopeAllFailures
	default:
		return AutoRetryScopeNetworkOnly
	}
}

func normalizeAutoRetryPreset(preset AutoRetryPreset) AutoRetryPreset {
	switch strings.ToLower(strings.TrimSpace(string(preset))) {
	case string(AutoRetryPresetAggressiveStop):
		return AutoRetryPresetAggressiveStop
	case string(AutoRetryPresetSustain60s):
		return AutoRetryPresetSustain60s
	default:
		return AutoRetryPresetGentleStop
	}
}

func effectiveAssistantState(record tables.WebSessionTable) AssistantState {
	if normalized := normalizeAssistantState(AssistantState(record.AssistantState)); normalized != AssistantStateNone {
		return normalized
	}
	if strings.EqualFold(strings.TrimSpace(record.Status), string(StatusWaitingApproval)) {
		return AssistantStateWaitingPlanApproval
	}
	return AssistantStateNone
}

func effectiveAssistantStateUpdatedAt(record tables.WebSessionTable, state AssistantState) *time.Time {
	if record.AssistantStateUpdatedAt != nil {
		return record.AssistantStateUpdatedAt
	}
	if state == AssistantStateWaitingPlanApproval && strings.EqualFold(strings.TrimSpace(record.Status), string(StatusWaitingApproval)) {
		value := record.UpdatedAt
		return &value
	}
	return nil
}

func effectiveStatusUpdatedAt(record tables.WebSessionTable, assistantState AssistantState) *time.Time {
	if record.StatusUpdatedAt != nil {
		return record.StatusUpdatedAt
	}
	if assistantStateUpdatedAt := effectiveAssistantStateUpdatedAt(record, assistantState); assistantStateUpdatedAt != nil {
		value := *assistantStateUpdatedAt
		return &value
	}
	if !record.UpdatedAt.IsZero() {
		value := record.UpdatedAt
		return &value
	}
	if !record.CreatedAt.IsZero() {
		value := record.CreatedAt
		return &value
	}
	return nil
}

func effectiveStatus(record tables.WebSessionTable, assistantState AssistantState) Status {
	switch strings.ToLower(strings.TrimSpace(record.Status)) {
	case string(StatusRunning):
		return StatusRunning
	case string(StatusWaitingApproval):
		if assistantState == AssistantStateWaitingPlanApproval {
			return StatusRunning
		}
		return StatusWaitingApproval
	case string(StatusDone):
		return StatusDone
	case string(StatusError):
		return StatusError
	case string(StatusAborting):
		return StatusAborting
	default:
		return StatusIdle
	}
}

func applyAssistantStateUpdates(updates map[string]any, state AssistantState, updatedAt time.Time) map[string]any {
	if updates == nil {
		updates = map[string]any{}
	}
	updates["status_updated_at"] = updatedAt
	normalized := normalizeAssistantState(state)
	if normalized == AssistantStateNone {
		updates["assistant_state"] = nil
		updates["assistant_state_updated_at"] = nil
		return updates
	}
	updates["assistant_state"] = string(normalized)
	updates["assistant_state_updated_at"] = updatedAt
	return updates
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
		cfg:               cfg,
		logger:            logger.Named("web-session-manager"),
		store:             eventStore,
		projectSvc:        model.NewProjectService(),
		worktreeSvc:       service.NewWorktreeService(),
		aiSessionSvc:      service.NewAISessionService(),
		runs:              make(map[string]*activeRun),
		clients:           make(map[*client]struct{}),
		autoRetryTimers:   make(map[string]*time.Timer),
		pendingInputs:     make(map[string][]PendingInput),
		pendingProcessing: make(map[string]bool),
		pendingDirty:      make(map[string]bool),
	}
	if err := manager.migrateLegacySessionModes(context.Background()); err != nil {
		return nil, err
	}
	if err := manager.backfillSessionActivityAt(context.Background()); err != nil {
		return nil, err
	}
	if err := manager.recoverInterruptedSessions(context.Background()); err != nil {
		return nil, err
	}
	if err := manager.recoverPendingAutoRetrySessions(context.Background()); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) registerClient(conn wsConn, kind clientKind) *client {
	client := &client{
		conn:   conn,
		logger: m.logger.Named("client"),
		kind:   kind,
		done:   make(chan struct{}),
	}
	client.MarkSeen()
	m.mu.Lock()
	m.clients[client] = struct{}{}
	m.mu.Unlock()
	client.startHeartbeat()
	return client
}

func (m *Manager) RegisterCommandClient(conn wsConn) *client {
	return m.registerClient(conn, clientKindCommand)
}

func (m *Manager) RegisterEventClient(conn wsConn) *client {
	return m.registerClient(conn, clientKindEvent)
}

var autoRetryNetworkFailureKeywords = []string{
	"network",
	"timeout",
	"timed out",
	"connection reset",
	"connection closed",
	"connection failed",
	"socket hang up",
	"transport error",
	"temporarily unavailable",
	"upstream service temporarily unavailable",
	"bad gateway",
	"502",
	"websocket",
}

var autoRetryRateLimitFailureKeywords = []string{
	"429",
	"rate limit",
	"too many requests",
}

func shouldAutoRetryFailure(scope AutoRetryScope, code string, message string) bool {
	normalizedScope := normalizeAutoRetryScope(scope)
	if normalizedScope == AutoRetryScopeAllFailures {
		return true
	}
	normalizedCode := strings.ToLower(strings.TrimSpace(code))
	normalizedMessage := strings.ToLower(strings.TrimSpace(message))
	isNetworkFailure := normalizedCode == codexTransportRetryExhaustedCode
	if !isNetworkFailure {
		for _, keyword := range autoRetryNetworkFailureKeywords {
			if strings.Contains(normalizedMessage, keyword) {
				isNetworkFailure = true
				break
			}
		}
	}
	if isNetworkFailure {
		return true
	}
	if normalizedScope != AutoRetryScopeNetworkAndRateLimit {
		return false
	}
	for _, keyword := range autoRetryRateLimitFailureKeywords {
		if strings.Contains(normalizedMessage, keyword) {
			return true
		}
	}
	return false
}

func autoRetryDelay(preset AutoRetryPreset, attempt int) (time.Duration, bool) {
	if attempt <= 0 {
		return 0, false
	}
	switch normalizeAutoRetryPreset(preset) {
	case AutoRetryPresetAggressiveStop:
		delays := []time.Duration{2 * time.Second, 5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second}
		if attempt > len(delays) {
			return 0, false
		}
		return delays[attempt-1], true
	case AutoRetryPresetSustain60s:
		delays := []time.Duration{3 * time.Second, 10 * time.Second, 30 * time.Second}
		if attempt <= len(delays) {
			return delays[attempt-1], true
		}
		return 60 * time.Second, true
	default:
		delays := []time.Duration{3 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second}
		if attempt > len(delays) {
			return 0, false
		}
		return delays[attempt-1], true
	}
}

func (m *Manager) UnregisterClient(client *client) {
	if client == nil {
		return
	}
	client.stop()
	m.mu.Lock()
	delete(m.clients, client)
	m.mu.Unlock()
}

func (c *client) MarkSeen() {
	if c == nil {
		return
	}
	c.lastSeenAt.Store(time.Now().UnixMilli())
}

func (c *client) SetFocusedSessionID(sessionID string) {
	if c == nil {
		return
	}
	c.focusMu.Lock()
	c.focusedSID = strings.TrimSpace(sessionID)
	c.focusMu.Unlock()
}

func (c *client) FocusedSessionID() string {
	if c == nil {
		return ""
	}
	c.focusMu.RLock()
	defer c.focusMu.RUnlock()
	return c.focusedSID
}

func (c *client) stop() {
	if c == nil {
		return
	}
	c.once.Do(func() {
		close(c.done)
	})
}

func (c *client) closeWithReason(reason string) {
	if c == nil {
		return
	}
	if c.logger != nil && strings.TrimSpace(reason) != "" {
		c.logger.Debug("closing web session websocket", zap.String("reason", reason))
	}
	c.stop()
	_ = c.conn.Close()
}

func (c *client) startHeartbeat() {
	if c == nil {
		return
	}
	go func() {
		interval := webSessionHeartbeatInterval
		if interval <= 0 {
			interval = 15 * time.Second
		}
		timeout := webSessionHeartbeatTimeout
		if timeout <= interval {
			timeout = interval * 3
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				lastSeenAt := c.lastSeenAt.Load()
				if lastSeenAt <= 0 {
					c.MarkSeen()
					lastSeenAt = c.lastSeenAt.Load()
				}
				if time.Since(time.UnixMilli(lastSeenAt)) > timeout {
					c.closeWithReason("heartbeat-timeout")
					return
				}
				if err := c.send(newHeartbeatFrame("ping")); err != nil {
					if c.logger != nil {
						c.logger.Debug("failed to send web session heartbeat", zap.Error(err))
					}
					c.closeWithReason("heartbeat-send-failed")
					return
				}
			}
		}
	}()
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
	records = m.refreshSessionSourceStates(ctx, records)

	items := make([]SessionSummary, 0, len(records))
	for _, record := range records {
		items = append(items, m.mapSessionSummary(record))
	}
	return items, nil
}

func (m *Manager) CountSessionsByProject(ctx context.Context) (map[string]int, error) {
	db := model.GetDB()
	if db == nil {
		return nil, model.ErrDBNotInitialized
	}

	var rows []struct {
		ProjectID string
		Count     int64
	}
	if err := db.WithContext(ctx).
		Model(&tables.WebSessionTable{}).
		Select("project_id, COUNT(1) AS count").
		Where("archived_at IS NULL").
		Group("project_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		projectID := strings.TrimSpace(row.ProjectID)
		if projectID == "" {
			continue
		}
		counts[projectID] = int(row.Count)
	}
	return counts, nil
}

func (m *Manager) ListArchivedSessions(
	ctx context.Context,
	projectIDs []string,
	limit int,
	offset int,
) (ArchivedQueryResult, error) {
	db := model.GetDB()
	if db == nil {
		return ArchivedQueryResult{}, model.ErrDBNotInitialized
	}

	normalizedProjectIDs := make([]string, 0, len(projectIDs))
	seen := make(map[string]struct{}, len(projectIDs))
	for _, projectID := range projectIDs {
		trimmed := strings.TrimSpace(projectID)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalizedProjectIDs = append(normalizedProjectIDs, trimmed)
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := db.WithContext(ctx).
		Model(&tables.WebSessionTable{}).
		Where("archived_at IS NOT NULL")
	if len(normalizedProjectIDs) > 0 {
		query = query.Where("project_id IN ?", normalizedProjectIDs)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ArchivedQueryResult{}, err
	}

	var records []tables.WebSessionTable
	if err := query.
		Order("activity_at DESC").
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&records).Error; err != nil {
		return ArchivedQueryResult{}, err
	}

	items := make([]SessionSummary, 0, len(records))
	for _, record := range records {
		items = append(items, m.mapSessionSummary(record))
	}

	nextOffset := offset + len(items)
	return ArchivedQueryResult{
		Items:      items,
		Total:      int(total),
		HasMore:    int64(nextOffset) < total,
		NextOffset: nextOffset,
	}, nil
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

	now := time.Now()
	record := tables.WebSessionTable{
		ProjectID:               project.Id,
		WorktreeID:              nilIfEmpty(worktreeID),
		OrderIndex:              orderIndex,
		Agent:                   string(normalizeAgent(params.Agent)),
		Backend:                 string(normalizeSessionBackend(params.Backend, normalizeAgent(params.Agent))),
		Title:                   title,
		TitleAuto:               strings.TrimSpace(params.Title) == "",
		Model:                   defaultModel(normalizeAgent(params.Agent), params.Model),
		ReasoningEffort:         string(defaultReasoningEffort(normalizeAgent(params.Agent), params.ReasoningEffort)),
		WorkflowMode:            string(normalizeWorkflowMode(params.WorkflowMode)),
		PermissionLevel:         string(normalizePermissionLevel(params.PermissionLevel)),
		AutoRetryEnabled:        params.AutoRetryEnabled,
		AutoRetryScope:          string(normalizeAutoRetryScope(params.AutoRetryScope)),
		AutoRetryPreset:         string(normalizeAutoRetryPreset(params.AutoRetryPreset)),
		Cwd:                     cwd,
		Status:                  string(StatusIdle),
		AssistantState:          "",
		HasUnread:               false,
		ArchivedAt:              nil,
		ActivityAt:              now,
		StatusUpdatedAt:         &now,
		AssistantStateUpdatedAt: nil,
		SourceKind:              string(defaultSessionBackend(normalizeAgent(params.Agent))),
		SyncState:               string(SyncStateMissing),
		LastSyncMode:            "",
		SourceCreatedAt:         nil,
		SourceUpdatedAt:         nil,
		LastSyncedAt:            nil,
		ThreadPath:              nil,
		ThreadPreview:           nil,
		TurnCount:               0,
		ItemCount:               0,
		LastEventSeq:            0,
		TotalInputTokens:        0,
		TotalCachedInputTokens:  0,
		TotalOutputTokens:       0,
		TotalCost:               0,
	}
	record.Init()

	if err := model.GetDB().WithContext(ctx).Create(&record).Error; err != nil {
		return SessionSummary{}, err
	}

	return m.mapSessionSummary(record), nil
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
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	return m.loadSnapshotLocal(ctx, record, limit, true)
}

func (m *Manager) SnapshotWithAutoSync(ctx context.Context, sessionID string, limit int) (SessionSnapshot, error) {
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	snapshot, err := m.loadSnapshotLocal(ctx, record, limit, true)
	if err != nil {
		return SessionSnapshot{}, err
	}
	if !shouldAutoSyncSnapshot(record, snapshot.History.Total) {
		return snapshot, nil
	}
	snapshot, err = m.syncSessionFromSource(ctx, sessionID, m.defaultCodexSyncMode(), true, false)
	if err != nil {
		return SessionSnapshot{}, err
	}
	if snapshot.History.Total == 0 {
		return SessionSnapshot{}, ErrSessionHistoryUnavailable
	}
	return snapshot, nil
}

func shouldAutoSyncSnapshot(record tables.WebSessionTable, historyTotal int) bool {
	if historyTotal > 0 {
		return false
	}
	if normalizeAgent(Agent(record.Agent)) != AgentCodex {
		return false
	}
	if record.NativeSessionID == nil || strings.TrimSpace(*record.NativeSessionID) == "" {
		return false
	}
	return true
}

func (m *Manager) loadSnapshotLocal(
	ctx context.Context,
	record tables.WebSessionTable,
	limit int,
	clearUnread bool,
) (SessionSnapshot, error) {
	if limit <= 0 || limit > MaxHistoryWindow {
		limit = DefaultHistoryWindow
	}
	history, err := m.loadHistoryWindow(ctx, record.ID, limit, nil)
	if err != nil {
		return SessionSnapshot{}, err
	}

	// Entering a session clears the unread state.
	if clearUnread && record.HasUnread {
		record.HasUnread = false
		if err := model.GetDB().WithContext(ctx).Model(&tables.WebSessionTable{}).
			Where("id = ?", record.ID).
			Update("has_unread", false).Error; err != nil {
			m.logger.Warn("failed to clear unread flag", zap.String("sessionId", record.ID), zap.Error(err))
		}
	}

	summary := m.mapSessionSummary(record)
	if clearUnread {
		summary.HasUnread = false
	}
	return SessionSnapshot{
		Session:       summary,
		History:       history,
		PendingInputs: m.pendingInputsSnapshot(record.ID),
	}, nil
}

func (m *Manager) History(ctx context.Context, sessionID string, limit int, beforeSeq *int64) (HistoryWindow, error) {
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return HistoryWindow{}, err
	}
	if limit <= 0 || limit > MaxHistoryWindow {
		limit = DefaultHistoryWindow
	}
	window, err := m.loadHistoryWindow(ctx, sessionID, limit, beforeSeq)
	if err != nil {
		return HistoryWindow{}, err
	}
	projected, err := m.projectedHistoryWindow(record, limit, beforeSeq)
	if err == nil {
		window.Events = projected.Events
	}
	return window, nil
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
	return m.mapSessionSummary(record), nil
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

func (m *Manager) UpdateAutoRetry(
	ctx context.Context,
	sessionID string,
	enabled bool,
	scope AutoRetryScope,
	preset AutoRetryPreset,
) (SessionSummary, error) {
	summary, err := m.updateFields(ctx, sessionID, map[string]any{
		"auto_retry_enabled": enabled,
		"auto_retry_scope":   string(normalizeAutoRetryScope(scope)),
		"auto_retry_preset":  string(normalizeAutoRetryPreset(preset)),
		"auto_retry_attempt": 0,
		"auto_retry_next_at": nil,
		"updated_at":         time.Now(),
	})
	if err != nil {
		return SessionSummary{}, err
	}
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}
	m.cancelAutoRetryTimer(sessionID)
	if enabled && effectiveStatus(record, effectiveAssistantState(record)) == StatusError {
		code := ""
		if record.AutoRetryLastErrorCode != nil {
			code = strings.TrimSpace(*record.AutoRetryLastErrorCode)
		}
		message := ""
		if record.LastError != nil {
			message = strings.TrimSpace(*record.LastError)
		}
		m.scheduleAutoRetry(record, code, message, time.Now())
	}
	return summary, nil
}

func (m *Manager) UpdateAgent(ctx context.Context, sessionID string, agent Agent) (SessionSummary, error) {
	normalized := normalizeAgent(agent)
	return m.updateFields(ctx, sessionID, map[string]any{
		"agent":             string(normalized),
		"backend":           string(defaultSessionBackend(normalized)),
		"model":             defaultModel(normalized, ""),
		"reasoning_effort":  string(defaultReasoningEffort(normalized, "")),
		"native_session_id": nil,
		"source_kind":       string(defaultSessionBackend(normalized)),
		"sync_state":        SyncStateMissing,
		"last_sync_mode":    "",
		"source_created_at": nil,
		"source_updated_at": nil,
		"last_synced_at":    nil,
		"thread_path":       nil,
		"thread_preview":    nil,
		"turn_count":        0,
		"item_count":        0,
		"sync_error":        nil,
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
		if moving.ArchivedAt != nil {
			return fmt.Errorf("archived sessions cannot be reordered")
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

		summary = m.mapSessionSummary(moving)
		return nil
	})
	if err != nil {
		return SessionSummary{}, err
	}
	return summary, nil
}

func (m *Manager) ArchiveSession(ctx context.Context, sessionID string) (SessionSummary, error) {
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}
	if record.ArchivedAt != nil {
		return m.mapSessionSummary(record), nil
	}

	hadActiveRun := m.hasActiveRun(sessionID)
	if err := m.stopRunIfActive(sessionID, 5*time.Second); err != nil {
		return SessionSummary{}, err
	}

	now := time.Now()
	updates := map[string]any{
		"archived_at":                now,
		"has_unread":                 false,
		"updated_at":                 now,
		"auto_retry_attempt":         0,
		"auto_retry_next_at":         nil,
		"auto_retry_last_error_code": nil,
	}

	current, currentErr := m.GetSession(ctx, sessionID)
	if currentErr == nil {
		record = current
	}
	if hadActiveRun || record.Status == string(StatusAborting) {
		updates["status"] = string(StatusIdle)
		updates = applyAssistantStateUpdates(updates, AssistantStateNone, now)
	}

	if err := m.updateRuntimeState(ctx, sessionID, updates); err != nil {
		return SessionSummary{}, err
	}
	m.cancelAutoRetryTimer(sessionID)
	m.clearPendingInputs(sessionID)
	archived, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}
	return m.mapSessionSummary(archived), nil
}

func (m *Manager) UnarchiveSession(ctx context.Context, sessionID string) (SessionSummary, error) {
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}
	if record.ArchivedAt == nil {
		return m.mapSessionSummary(record), nil
	}

	orderIndex, err := m.getNextSessionOrderIndex(ctx, record.ProjectID)
	if err != nil {
		return SessionSummary{}, err
	}

	now := time.Now()
	if err := m.updateRuntimeState(ctx, sessionID, map[string]any{
		"archived_at": nil,
		"order_index": orderIndex,
		"updated_at":  now,
	}); err != nil {
		return SessionSummary{}, err
	}

	current, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}
	return m.mapSessionSummary(current), nil
}

func (m *Manager) DeleteSession(ctx context.Context, sessionID string) error {
	_ = m.AbortSession(sessionID)
	m.cancelAutoRetryTimer(sessionID)
	m.clearPendingInputs(sessionID)
	db := model.GetDB()
	if db == nil {
		return model.ErrDBNotInitialized
	}
	if err := db.WithContext(ctx).Where("web_session_id = ?", sessionID).Delete(&tables.WebSessionTurnTable{}).Error; err != nil {
		return err
	}
	if err := db.WithContext(ctx).Where("web_session_id = ?", sessionID).Delete(&tables.WebSessionItemTable{}).Error; err != nil {
		return err
	}
	if err := model.GetDB().WithContext(ctx).Delete(&tables.WebSessionTable{}, "id = ?", sessionID).Error; err != nil {
		return err
	}
	return m.store.deleteSessionFiles(sessionID)
}

func (m *Manager) cancelAutoRetryTimer(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	timer := m.autoRetryTimers[sessionID]
	if timer != nil {
		timer.Stop()
		delete(m.autoRetryTimers, sessionID)
	}
}

func (m *Manager) setAutoRetryTimer(sessionID string, nextAt time.Time) {
	m.cancelAutoRetryTimer(sessionID)
	delay := time.Until(nextAt)
	if delay < 0 {
		delay = 0
	}
	timer := time.AfterFunc(delay, func() {
		m.cancelAutoRetryTimer(sessionID)
		m.executeAutoRetry(sessionID)
	})
	m.mu.Lock()
	m.autoRetryTimers[sessionID] = timer
	m.mu.Unlock()
}

func (m *Manager) resetAutoRetryProgress(ctx context.Context, sessionID string) {
	m.cancelAutoRetryTimer(sessionID)
	_ = m.updateRuntimeState(ctx, sessionID, map[string]any{
		"auto_retry_attempt": 0,
		"auto_retry_next_at": nil,
		"updated_at":         time.Now(),
	})
}

func (m *Manager) clearAutoRetryNextAt(ctx context.Context, sessionID string) {
	m.cancelAutoRetryTimer(sessionID)
	_ = m.updateRuntimeState(ctx, sessionID, map[string]any{
		"auto_retry_next_at": nil,
		"updated_at":         time.Now(),
	})
}

func (m *Manager) scheduleAutoRetry(record tables.WebSessionTable, code string, message string, now time.Time) {
	if record.ArchivedAt != nil {
		m.resetAutoRetryProgress(context.Background(), record.ID)
		return
	}
	if !record.AutoRetryEnabled {
		m.resetAutoRetryProgress(context.Background(), record.ID)
		return
	}
	if !shouldAutoRetryFailure(AutoRetryScope(record.AutoRetryScope), code, message) {
		m.resetAutoRetryProgress(context.Background(), record.ID)
		return
	}

	nextAttempt := record.AutoRetryAttempt + 1
	delay, ok := autoRetryDelay(AutoRetryPreset(record.AutoRetryPreset), nextAttempt)
	if !ok {
		m.cancelAutoRetryTimer(record.ID)
		_ = m.updateRuntimeState(context.Background(), record.ID, map[string]any{
			"auto_retry_attempt": nextAttempt,
			"auto_retry_next_at": nil,
			"updated_at":         now,
		})
		return
	}

	nextAt := now.Add(delay)
	_ = m.updateRuntimeState(context.Background(), record.ID, map[string]any{
		"auto_retry_attempt": nextAttempt,
		"auto_retry_next_at": nextAt,
		"updated_at":         now,
	})
	m.setAutoRetryTimer(record.ID, nextAt)
}

func (m *Manager) executeAutoRetry(sessionID string) {
	ctx := context.Background()
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return
	}
	if record.ArchivedAt != nil || !record.AutoRetryEnabled || effectiveStatus(record, effectiveAssistantState(record)) != StatusError {
		m.clearAutoRetryNextAt(ctx, sessionID)
		return
	}
	message := ""
	if record.LastError != nil {
		message = strings.TrimSpace(*record.LastError)
	}
	code := ""
	if record.AutoRetryLastErrorCode != nil {
		code = strings.TrimSpace(*record.AutoRetryLastErrorCode)
	}
	if !shouldAutoRetryFailure(AutoRetryScope(record.AutoRetryScope), code, message) {
		m.clearAutoRetryNextAt(ctx, sessionID)
		return
	}
	if err := m.sendMessageInternal(ctx, sessionID, "continue", nil, true); err != nil && m.logger != nil {
		m.logger.Warn("auto retry send failed", zap.String("sessionId", sessionID), zap.Error(err))
	}
}

func (m *Manager) stopRunIfActive(sessionID string, timeout time.Duration) error {
	m.mu.RLock()
	run, ok := m.runs[sessionID]
	m.mu.RUnlock()
	if !ok || run == nil {
		return nil
	}
	if err := m.AbortSession(sessionID); err != nil {
		return err
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	select {
	case <-run.done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for session to stop")
	}
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
	killCmdTree(run.cmd)
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
	case "set_ar":
		return m.handleSetAutoRetryCommand(ctx, client, frame)
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
	case "pending_del":
		return m.handlePendingDeleteCommand(client, frame)
	case "del":
		return m.handleDeleteCommand(ctx, client, frame)
	case "list":
		return m.handleListCommand(ctx, client, frame)
	default:
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "unknown operation", false))
	}
}

func (m *Manager) HandleHeartbeatPayload(client *client, payload []byte) (bool, error) {
	var frame wireHeartbeatFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return false, nil
	}
	if frame.Kind != "hb" {
		return false, nil
	}
	client.MarkSeen()
	switch strings.ToLower(strings.TrimSpace(frame.Operation)) {
	case "ping":
		return true, client.send(newHeartbeatFrame("pong"))
	case "pong":
		return true, nil
	case "focus":
		client.SetFocusedSessionID(frame.SessionID)
		return true, nil
	default:
		return true, nil
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
	} else if parsedMime, _, err := mime.ParseMediaType(attachment.Mime); err == nil && parsedMime != "" {
		attachment.Mime = parsedMime
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
		ProjectID        string `json:"pid"`
		WorktreeID       string `json:"wid"`
		Agent            string `json:"ag"`
		Model            string `json:"md"`
		ReasoningEffort  string `json:"re"`
		WorkflowMode     string `json:"wm"`
		PermissionLevel  string `json:"pl"`
		AutoRetryEnabled bool   `json:"ae"`
		AutoRetryScope   string `json:"ars"`
		AutoRetryPreset  string `json:"arp"`
		PermissionMode   string `json:"pm"`
		Title            string `json:"ttl"`
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
		ProjectID:        payload.ProjectID,
		WorktreeID:       payload.WorktreeID,
		Agent:            Agent(payload.Agent),
		Model:            payload.Model,
		ReasoningEffort:  ReasoningEffort(payload.ReasoningEffort),
		WorkflowMode:     workflowMode,
		PermissionLevel:  permissionLevel,
		AutoRetryEnabled: payload.AutoRetryEnabled,
		AutoRetryScope:   AutoRetryScope(payload.AutoRetryScope),
		AutoRetryPreset:  AutoRetryPreset(payload.AutoRetryPreset),
		Title:            payload.Title,
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
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	return client.send(newHistoryPageFrame(frame.SessionID, window))
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

func (m *Manager) handlePendingDeleteCommand(client *client, frame wireCommandFrame) error {
	var payload struct {
		PendingID string `json:"id"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid pending delete payload", false))
	}
	if strings.TrimSpace(payload.PendingID) == "" {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "pending id is required", false))
	}
	if !m.removePendingInput(frame.SessionID, payload.PendingID) {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "not_found", "pending input not found", false))
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
	m.broadcastSessionSummary(ctx, summary.ID)
	return nil
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
	m.broadcastSessionSummary(ctx, frame.SessionID)
	return nil
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
	m.broadcastSessionSummary(ctx, frame.SessionID)
	return nil
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
	m.broadcastSessionSummary(ctx, frame.SessionID)
	return nil
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
	m.broadcastSessionSummary(ctx, frame.SessionID)
	return nil
}

func (m *Manager) handleSetAutoRetryCommand(
	ctx context.Context,
	client *client,
	frame wireCommandFrame,
) error {
	var payload struct {
		Enabled bool   `json:"ae"`
		Scope   string `json:"ars"`
		Preset  string `json:"arp"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid auto retry payload", false))
	}
	if _, err := m.UpdateAutoRetry(
		ctx,
		frame.SessionID,
		payload.Enabled,
		AutoRetryScope(payload.Scope),
		AutoRetryPreset(payload.Preset),
	); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", err.Error(), false))
	}
	if err := client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil)); err != nil {
		return err
	}
	m.broadcastSessionSummary(ctx, frame.SessionID)
	return nil
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
	m.broadcastSessionSummary(ctx, frame.SessionID)
	return nil
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
	m.broadcastSessionSummary(ctx, frame.SessionID)
	return nil
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
	m.broadcastSessionSummary(ctx, summary.ID)
	return nil
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
		Mode        string   `json:"mode"`
		PendingID   string   `json:"pid"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "bad_req", "invalid send payload", false))
	}
	if err := m.sendMessageWithMode(
		ctx,
		frame.SessionID,
		payload.Text,
		payload.Attachments,
		PendingInputMode(payload.Mode),
		payload.PendingID,
	); err != nil {
		return client.send(newErrorFrame(frame.RequestID, frame.SessionID, "invalid_state", err.Error(), false))
	}
	return client.send(newAckFrame(frame.RequestID, frame.Operation, frame.SessionID, nil))
}

func (m *Manager) SendMessage(ctx context.Context, sessionID, text string, attachmentIDs []string) error {
	return m.sendMessageInternal(ctx, sessionID, text, attachmentIDs, false)
}

func (m *Manager) sendMessageInternal(
	ctx context.Context,
	sessionID,
	text string,
	attachmentIDs []string,
	fromAutoRetry bool,
) error {
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if record.ArchivedAt != nil {
		return fmt.Errorf("session is archived")
	}
	m.cancelAutoRetryTimer(sessionID)
	if m.hasActiveRun(sessionID) {
		return fmt.Errorf("session is already running")
	}

	attachments := make([]Attachment, 0, len(attachmentIDs))
	for _, id := range attachmentIDs {
		attachment, err := m.loadAttachment(strings.TrimSpace(id))
		if err != nil {
			return fmt.Errorf("attachment %s not found", id)
		}
		attachments = append(attachments, attachment)
	}
	text = strings.TrimSpace(text)
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
		"status":                     string(markStatus),
		"has_unread":                 false,
		"last_error":                 nil,
		"auto_retry_last_error_code": nil,
		"updated_at":                 now,
		"last_message_at":            now,
	}
	if fromAutoRetry {
		updates["auto_retry_next_at"] = nil
	} else {
		updates["auto_retry_attempt"] = 0
		updates["auto_retry_next_at"] = nil
	}
	updates = applyAssistantStateUpdates(updates, AssistantStateWorking, now)
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
	m.broadcastSessionSummary(ctx, sessionID)
	if titleChanged && m.logger != nil {
		m.logger.Debug("auto-renamed web session title",
			zap.String("sessionId", sessionID),
		)
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
		run.resetActiveCallTracking()
		run.closeInput()
		run.clearPendingApproval()
		run.clearPendingServerRequest()
		close(run.done)
		m.mu.Lock()
		delete(m.runs, session.ID)
		m.mu.Unlock()
		m.triggerPendingProcessing(session.ID)
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
		abortPayload := activeCallTimeoutAbortPayload(session, run.abortEventPayload())
		now := time.Now()
		_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
			ID:        utils.NewID(),
			Seq:       0,
			Type:      "run_abort",
			RunID:     run.runID,
			Timestamp: now,
			Payload:   abortPayload,
		})
		_ = m.updateRuntimeState(
			context.Background(),
			session.ID,
			applyAssistantStateUpdates(map[string]any{
				"status":                     string(StatusIdle),
				"updated_at":                 now,
				"auto_retry_attempt":         0,
				"auto_retry_next_at":         nil,
				"auto_retry_last_error_code": nil,
			}, AssistantStateNone, now),
		)
		m.cancelAutoRetryTimer(session.ID)
		m.broadcastSessionSummary(context.Background(), session.ID)
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
	finalStatus, finalAssistantState := m.completedRunState(context.Background(), session, run)
	now := time.Now()
	_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "run_done",
		RunID:     run.runID,
		Timestamp: now,
		Payload: map[string]any{
			"ok": true,
			"st": string(finalStatus),
		},
	})
	_ = m.updateRuntimeState(
		context.Background(),
		session.ID,
		applyAssistantStateUpdates(map[string]any{
			"status":                     string(finalStatus),
			"updated_at":                 now,
			"auto_retry_attempt":         0,
			"auto_retry_next_at":         nil,
			"auto_retry_last_error_code": nil,
		}, finalAssistantState, now),
	)
	m.cancelAutoRetryTimer(session.ID)
	m.broadcastSessionSummary(context.Background(), session.ID)
	m.maybeSyncSessionAfterRun(session)
}

func (m *Manager) handleRunFailure(sessionID string, session tables.WebSessionTable, run *activeRun, err error) {
	m.handleRunFailureWithCode(sessionID, session, run, "", err)
}

func (m *Manager) handleRunFailureWithCode(
	sessionID string,
	session tables.WebSessionTable,
	run *activeRun,
	code string,
	err error,
) {
	if run != nil {
		run.resetActiveCallTracking()
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "runtime failed"
	}
	run.lastError = message
	if strings.TrimSpace(code) == "" && run != nil {
		code = strings.TrimSpace(run.lastErrorCode)
	}
	if strings.TrimSpace(code) == "" {
		code = "runtime_error"
	}
	now := time.Now()
	_, _ = m.appendAndBroadcast(context.Background(), sessionID, session, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "run_fail",
		RunID:     run.runID,
		Timestamp: now,
		Payload: map[string]any{
			"code": code,
			"msg":  message,
		},
	})
	_ = m.updateRuntimeState(
		context.Background(),
		sessionID,
		applyAssistantStateUpdates(map[string]any{
			"status":                     string(StatusError),
			"last_error":                 message,
			"auto_retry_last_error_code": nilIfEmpty(code),
			"updated_at":                 now,
		}, AssistantStateNone, now),
	)
	current, currentErr := m.GetSession(context.Background(), sessionID)
	if currentErr == nil {
		m.scheduleAutoRetry(current, code, message, now)
	}
	m.broadcastSessionSummary(context.Background(), sessionID)
}

func (m *Manager) appendRunNote(
	sessionID string,
	session tables.WebSessionTable,
	run *activeRun,
	level string,
	message string,
	payload map[string]any,
) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	nextPayload := cloneMap(payload)
	if nextPayload == nil {
		nextPayload = map[string]any{}
	}
	nextPayload["txt"] = trimmed
	if strings.TrimSpace(level) != "" {
		nextPayload["lvl"] = strings.TrimSpace(level)
	}
	_, _ = m.appendAndBroadcast(context.Background(), sessionID, session, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "note",
		RunID:     run.runID,
		ParentID:  run.assistantMessageID,
		Timestamp: time.Now(),
		Payload:   nextPayload,
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
	now := time.Now()
	_, _ = m.appendAndBroadcast(context.Background(), session.ID, session, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "approval_req",
		RunID:     run.runID,
		ParentID:  run.assistantMessageID,
		Timestamp: now,
		Payload: map[string]any{
			"prompt": prompt,
		},
	})
	_ = m.updateRuntimeState(
		context.Background(),
		session.ID,
		applyAssistantStateUpdates(map[string]any{
			"updated_at": now,
		}, AssistantStateWaitingApproval, now),
	)
	m.broadcastSessionSummary(context.Background(), session.ID)
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
		toolKind := normalizeCodexItemType(stringValue(item["type"]))
		toolName := codexToolName(item)
		toolInput := codexToolInput(item)
		toolMeta := codexToolMeta(item)
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
				"name": toolName,
				"kind": stringValue(item["type"]),
				"in":   toolInput,
				"meta": toolMeta,
			},
		})
		m.trackActiveCodexToolStart(run, toolID, toolKind, toolName, toolInput, toolMeta)
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
		toolSucceeded := codexToolSucceeded(item)
		if toolSucceeded && codexToolIsPlan(item) {
			run.markCompletedPlanTool()
		}
		if toolSucceeded && normalizeCodexItemType(stringValue(item["type"])) == "context_compaction" {
			record, err := m.GetSession(context.Background(), session.ID)
			if err == nil {
				_ = m.updateRuntimeState(
					context.Background(),
					session.ID,
					contextEstimateBaselineResetUpdate(record, time.Now()),
				)
			}
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
				"kind": normalizeCodexItemType(stringValue(item["type"])),
				"out":  codexToolOutput(item),
				"ok":   toolSucceeded,
				"meta": codexToolMeta(item),
			},
		})
		m.trackActiveCodexToolComplete(run, toolID)
	case "turn.completed":
		usage, _ := raw["usage"].(map[string]any)
		in := int64(numberValue(usage["input_tokens"]))
		cin := int64(numberValue(usage["cached_input_tokens"]))
		out := int64(numberValue(usage["output_tokens"]))
		_ = m.updateRuntimeState(context.Background(), session.ID, contextEstimateIncrementUpdate(in, cin, out))
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
	m.decorateProjectedEvent(sessionID, &event)
	if err := m.store.appendEvent(sessionID, event); err != nil {
		return Event{}, err
	}

	update := map[string]any{
		"last_event_seq": seq,
		"activity_at":    event.Timestamp,
	}
	if shouldMarkSessionUnreadForEvent(event) && !m.hasFocusedEventClient(sessionID) {
		update["has_unread"] = true
	}
	if event.Type == "msg_u" {
		now := time.Now()
		update["last_message_at"] = now
	}
	if err := m.updateRuntimeState(ctx, sessionID, update); err != nil {
		return Event{}, err
	}

	cachedItem, cacheErr := m.applyEventToHistoryCache(ctx, sessionID, event)
	if cacheErr != nil {
		return Event{}, cacheErr
	}
	if cachedItem != nil {
		m.broadcast(newHistoryItemFrame(sessionID, *cachedItem, nil))
	}
	if event.Type == "tool_end" {
		m.maybeInterruptForRedirect(sessionID)
	}
	return event, nil
}

func (m *Manager) sessionAgent(sessionID string) Agent {
	m.mu.RLock()
	run := m.runs[sessionID]
	m.mu.RUnlock()
	if run != nil {
		run.mu.Lock()
		agent := run.agent
		run.mu.Unlock()
		if agent != "" {
			return normalizeAgent(agent)
		}
	}

	db := model.GetDB()
	if db == nil {
		return AgentClaude
	}
	var record tables.WebSessionTable
	if err := db.WithContext(context.Background()).
		Select("id", "agent").
		First(&record, "id = ?", sessionID).Error; err != nil {
		return AgentClaude
	}
	return normalizeAgent(Agent(record.Agent))
}

func shouldMarkSessionUnreadForEvent(event Event) bool {
	switch strings.TrimSpace(event.Type) {
	case "approval_req", "user_input_req", "run_fail", "run_done":
		return true
	case "run_abort":
		return isUnexpectedRunAbortEvent(event)
	default:
		return false
	}
}

func isUnexpectedRunAbortEvent(event Event) bool {
	reason := strings.TrimSpace(stringValue(event.Payload["reason"]))
	msg := strings.TrimSpace(stringValue(event.Payload["msg"]))
	prevStatus := strings.TrimSpace(stringValue(event.Payload["prevStatus"]))
	return reason != "" || msg != "" || prevStatus != ""
}

func (m *Manager) hasFocusedEventClient(sessionID string) bool {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if normalizedSessionID == "" {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for client := range m.clients {
		if client == nil || client.kind != clientKindEvent {
			continue
		}
		if client.FocusedSessionID() == normalizedSessionID {
			return true
		}
	}
	return false
}

func (m *Manager) decorateProjectedEvent(sessionID string, event *Event) {
	if event == nil {
		return
	}
	if isCompactToolEvent(*event) {
		m.decorateCompactToolGroupEvent(sessionID, event)
		return
	}
	if isReasoningToolEvent(*event) {
		if reasoningEventHasDisplayContent(*event) && m.sessionAgent(sessionID) != AgentCodex {
			m.resetCommandExecutionGroup(sessionID)
		}
		return
	}
	m.resetCommandExecutionGroup(sessionID)
}

func (m *Manager) decorateCompactToolGroupEvent(sessionID string, event *Event) {
	toolID := eventToolID(*event)
	if toolID == "" {
		toolID = event.ID
	}
	kind := compactToolKind(*event)

	groupID := commandExecutionGroupID(toolID)
	firstSeq := event.Seq
	count := 1

	m.mu.RLock()
	run := m.runs[sessionID]
	m.mu.RUnlock()

	if run != nil {
		run.mu.Lock()
		if run.commandGroupKind != "" && run.commandGroupKind != kind {
			run.commandGroupID = ""
			run.commandGroupKind = ""
			run.commandGroupFirst = 0
			run.commandGroupCount = 0
			run.commandGroupTools = nil
		}
		if run.commandGroupTools == nil {
			run.commandGroupTools = make(map[string]struct{})
		}
		if run.commandGroupID == "" {
			run.commandGroupID = groupID
		}
		if run.commandGroupKind == "" {
			run.commandGroupKind = kind
		}
		groupID = run.commandGroupID
		if run.commandGroupFirst == 0 {
			run.commandGroupFirst = event.Seq
		}
		firstSeq = run.commandGroupFirst
		if _, exists := run.commandGroupTools[toolID]; !exists {
			run.commandGroupTools[toolID] = struct{}{}
			run.commandGroupCount += 1
		}
		if run.commandGroupCount > 0 {
			count = run.commandGroupCount
		}
		run.mu.Unlock()
	}

	meta := eventToolMeta(*event)
	if meta == nil {
		meta = make(map[string]any)
	}
	meta["kind"] = kind
	meta["title"] = firstNonEmpty(stringValue(meta["title"]), eventToolName(*event), compactToolTitle(kind))
	meta["subtitle"] = compactToolSummary(kind, eventToolInput(*event), meta, eventToolOutput(*event))
	meta["commandGroup"] = map[string]any{
		"id":           groupID,
		"count":        count,
		"firstSeq":     firstSeq,
		"lastSeq":      event.Seq,
		"latestToolId": toolID,
		"compacted":    true,
	}
	if event.Payload == nil {
		event.Payload = make(map[string]any)
	}
	event.Payload["meta"] = meta
}

func (m *Manager) resetCommandExecutionGroup(sessionID string) {
	m.mu.RLock()
	run := m.runs[sessionID]
	m.mu.RUnlock()
	if run == nil {
		return
	}
	run.mu.Lock()
	run.commandGroupID = ""
	run.commandGroupKind = ""
	run.commandGroupFirst = 0
	run.commandGroupCount = 0
	run.commandGroupTools = nil
	run.mu.Unlock()
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

func (m *Manager) completedRunState(ctx context.Context, session tables.WebSessionTable, run *activeRun) (Status, AssistantState) {
	current := session
	record, err := m.GetSession(ctx, session.ID)
	if err == nil {
		current = record
	}
	if effectiveWorkflowMode(current) == WorkflowModePlan && run.completedPlanToolSeen() {
		return StatusRunning, AssistantStateWaitingPlanApproval
	}
	return StatusDone, AssistantStateNone
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
	return m.mapSessionSummary(record), nil
}

func (m *Manager) getNextSessionOrderIndex(ctx context.Context, projectID string) (float64, error) {
	db := model.GetDB()
	if db == nil {
		return 0, model.ErrDBNotInitialized
	}

	var maxOrder float64
	if err := db.WithContext(ctx).
		Model(&tables.WebSessionTable{}).
		Where("project_id = ? AND archived_at IS NULL", projectID).
		Select("COALESCE(MAX(order_index), 0)").
		Scan(&maxOrder).Error; err != nil {
		return 0, err
	}
	return maxOrder + sessionOrderStep, nil
}

func (m *Manager) listSessionRecordsWithDB(db *gorm.DB, projectID string) ([]tables.WebSessionTable, error) {
	query := db.Model(&tables.WebSessionTable{}).
		Where("archived_at IS NULL").
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

func (m *Manager) backfillSessionActivityAt(ctx context.Context) error {
	db := model.GetDB()
	if db == nil {
		return model.ErrDBNotInitialized
	}

	var records []tables.WebSessionTable
	if err := db.WithContext(ctx).
		Select("id", "created_at", "updated_at", "last_message_at", "activity_at").
		Find(&records).Error; err != nil {
		return err
	}

	for _, record := range records {
		if !record.ActivityAt.IsZero() {
			continue
		}
		activityAt := chooseSessionActivityAt(record)
		if err := db.WithContext(ctx).
			Model(&tables.WebSessionTable{}).
			Where("id = ?", record.ID).
			Updates(map[string]any{
				"activity_at": activityAt,
				"updated_at":  time.Now(),
			}).Error; err != nil {
			return err
		}
	}
	return nil
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
		m.resumeActiveCallTimeout(run)
		record, err := m.GetSession(context.Background(), sessionID)
		if err != nil {
			return err
		}
		now := time.Now()
		_, _ = m.appendAndBroadcast(context.Background(), sessionID, record, Event{
			ID:        utils.NewID(),
			Seq:       0,
			Type:      "approval_res",
			RunID:     run.runID,
			ParentID:  run.assistantMessageID,
			Timestamp: now,
			Payload: map[string]any{
				"act":    action,
				"prompt": pending.Prompt,
			},
		})
		_ = m.updateRuntimeState(
			context.Background(),
			sessionID,
			applyAssistantStateUpdates(map[string]any{
				"updated_at": now,
			}, AssistantStateWorking, now),
		)
		m.broadcastSessionSummary(context.Background(), sessionID)
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
	now := time.Now()
	_, _ = m.appendAndBroadcast(context.Background(), sessionID, record, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "approval_res",
		RunID:     run.runID,
		ParentID:  run.assistantMessageID,
		Timestamp: now,
		Payload: map[string]any{
			"act":    action,
			"prompt": prompt,
		},
	})
	_ = m.updateRuntimeState(
		context.Background(),
		sessionID,
		applyAssistantStateUpdates(map[string]any{
			"updated_at": now,
		}, AssistantStateWorking, now),
	)
	m.broadcastSessionSummary(context.Background(), sessionID)
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
	m.resumeActiveCallTimeout(run)

	record, err := m.GetSession(context.Background(), sessionID)
	if err != nil {
		return err
	}
	now := time.Now()
	_, _ = m.appendAndBroadcast(context.Background(), sessionID, record, Event{
		ID:        utils.NewID(),
		Seq:       0,
		Type:      "user_input_res",
		RunID:     run.runID,
		ParentID:  run.assistantMessageID,
		Timestamp: now,
		Payload: map[string]any{
			"iid": pending.ItemID,
			"ans": answers,
		},
	})
	_ = m.updateRuntimeState(
		context.Background(),
		sessionID,
		applyAssistantStateUpdates(map[string]any{
			"updated_at": now,
		}, AssistantStateWorking, now),
	)
	m.broadcastSessionSummary(context.Background(), sessionID)
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
		if client.kind == clientKindEvent {
			clients = append(clients, client)
		}
	}
	m.mu.RUnlock()

	for _, client := range clients {
		if !shouldSendFrameToClient(client, frame) {
			continue
		}
		if err := client.send(frame); err != nil {
			m.logger.Debug("failed to send ws frame", zap.Error(err))
			client.closeWithReason("broadcast-send-failed")
		}
	}
}

func shouldSendFrameToClient(client *client, frame wireFrame) bool {
	if client == nil {
		return false
	}
	focusedSessionID := client.FocusedSessionID()
	switch frame.Kind {
	case "snap":
		return focusedSessionID != "" && focusedSessionID == strings.TrimSpace(frame.SessionID)
	case "evt":
		switch strings.ToLower(strings.TrimSpace(frame.Operation)) {
		case "hist_item", "hist_page", "pending":
			return focusedSessionID != "" && focusedSessionID == strings.TrimSpace(frame.SessionID)
		default:
			return true
		}
	default:
		return true
	}
}

func (m *Manager) broadcastSnapshot(ctx context.Context, sessionID string) error {
	record, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if record.ArchivedAt != nil {
		return nil
	}
	snap, err := m.loadSnapshotLocal(ctx, record, DefaultHistoryWindow, false)
	if err != nil {
		return err
	}
	m.broadcast(newSnapshotFrame(sessionID, snap))
	return nil
}

func (m *Manager) broadcastSessionSummary(ctx context.Context, sessionID string) {
	summary := m.summaryForBroadcast(ctx, sessionID)
	if summary == nil {
		return
	}
	m.broadcast(newSessionFrame(sessionID, *summary))
}

func (c *client) send(frame wireFrame) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteJSON(frame)
}

func mapSessionRecord(record tables.WebSessionTable) SessionSummary {
	activityAt := record.ActivityAt
	if activityAt.IsZero() {
		activityAt = chooseSessionActivityAt(record)
	}
	assistantState := effectiveAssistantState(record)
	statusUpdatedAt := effectiveStatusUpdatedAt(record, assistantState)
	assistantStateUpdatedAt := effectiveAssistantStateUpdatedAt(record, assistantState)
	contextEstimate, contextEstimateMode := buildContextEstimate(record)
	return SessionSummary{
		ID:                      record.ID,
		ProjectID:               record.ProjectID,
		WorktreeID:              record.WorktreeID,
		OrderIndex:              record.OrderIndex,
		Agent:                   Agent(record.Agent),
		Title:                   record.Title,
		Model:                   record.Model,
		ReasoningEffort:         ReasoningEffort(record.ReasoningEffort),
		WorkflowMode:            effectiveWorkflowMode(record),
		PermissionLevel:         effectivePermissionLevel(record),
		AutoRetryEnabled:        record.AutoRetryEnabled,
		AutoRetryScope:          normalizeAutoRetryScope(AutoRetryScope(record.AutoRetryScope)),
		AutoRetryPreset:         normalizeAutoRetryPreset(AutoRetryPreset(record.AutoRetryPreset)),
		Cwd:                     record.Cwd,
		NativeSessionID:         record.NativeSessionID,
		Status:                  effectiveStatus(record, assistantState),
		AssistantState:          assistantState,
		HasUnread:               record.HasUnread,
		ArchivedAt:              record.ArchivedAt,
		ActivityAt:              activityAt,
		StatusUpdatedAt:         statusUpdatedAt,
		LastMessageAt:           record.LastMessageAt,
		AssistantStateUpdatedAt: assistantStateUpdatedAt,
		SourceKind:              record.SourceKind,
		SyncState:               normalizeSyncState(record.SyncState),
		LastSyncMode:            recordedSyncMode(record.LastSyncMode),
		SourceCreatedAt:         record.SourceCreatedAt,
		SourceUpdatedAt:         record.SourceUpdatedAt,
		LastSyncedAt:            record.LastSyncedAt,
		ThreadPath:              record.ThreadPath,
		ThreadPreview:           record.ThreadPreview,
		TurnCount:               record.TurnCount,
		ItemCount:               record.ItemCount,
		SyncError:               record.SyncError,
		CreatedAt:               record.CreatedAt,
		UpdatedAt:               record.UpdatedAt,
		Usage: Usage{
			InputTokens:       record.TotalInputTokens,
			CachedInputTokens: record.TotalCachedInputTokens,
			OutputTokens:      record.TotalOutputTokens,
			Cost:              record.TotalCost,
		},
		ContextEstimate:         contextEstimate,
		ContextEstimateMode:     contextEstimateMode,
		LastContextCompactionAt: record.LastContextCompactionAt,
	}
}

func buildContextEstimate(record tables.WebSessionTable) (ContextEstimate, ContextEstimateMode) {
	mode := ContextEstimateModeCumulativeTotal
	inputTokens := record.TotalInputTokens
	cachedInputTokens := record.TotalCachedInputTokens
	outputTokens := record.TotalOutputTokens
	if record.LastContextCompactionAt != nil {
		mode = ContextEstimateModeSinceCompaction
		inputTokens = maxInt64(0, record.TotalInputTokens-record.ContextBaselineInputTokens)
		cachedInputTokens = maxInt64(0, record.TotalCachedInputTokens-record.ContextBaselineCachedInputTokens)
		outputTokens = maxInt64(0, record.TotalOutputTokens-record.ContextBaselineOutputTokens)
	}
	return ContextEstimate{
		InputTokens:       inputTokens,
		CachedInputTokens: cachedInputTokens,
		OutputTokens:      outputTokens,
		UsedTokens:        maxInt64(0, inputTokens+cachedInputTokens+outputTokens),
	}, mode
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func contextEstimateTotalsUpdate(in, cin, out int64) map[string]any {
	return map[string]any{
		"total_input_tokens":        in,
		"total_cached_input_tokens": cin,
		"total_output_tokens":       out,
		"updated_at":                time.Now(),
	}
}

func contextEstimateIncrementUpdate(in, cin, out int64) map[string]any {
	return map[string]any{
		"total_input_tokens":        gorm.Expr("total_input_tokens + ?", in),
		"total_cached_input_tokens": gorm.Expr("total_cached_input_tokens + ?", cin),
		"total_output_tokens":       gorm.Expr("total_output_tokens + ?", out),
		"updated_at":                time.Now(),
	}
}

func contextEstimateBaselineResetUpdate(record tables.WebSessionTable, timestamp time.Time) map[string]any {
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	return map[string]any{
		"context_baseline_input_tokens":        record.TotalInputTokens,
		"context_baseline_cached_input_tokens": record.TotalCachedInputTokens,
		"context_baseline_output_tokens":       record.TotalOutputTokens,
		"last_context_compaction_at":           timestamp,
		"updated_at":                           time.Now(),
	}
}

func chooseSessionActivityAt(record tables.WebSessionTable) time.Time {
	if record.LastMessageAt != nil && !record.LastMessageAt.IsZero() {
		return *record.LastMessageAt
	}
	if !record.UpdatedAt.IsZero() {
		return record.UpdatedAt
	}
	if !record.CreatedAt.IsZero() {
		return record.CreatedAt
	}
	return time.Now()
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

func (m *Manager) recoverPendingAutoRetrySessions(ctx context.Context) error {
	db := model.GetDB()
	if db == nil {
		return model.ErrDBNotInitialized
	}

	var records []tables.WebSessionTable
	if err := db.WithContext(ctx).
		Where("auto_retry_enabled = ? AND auto_retry_next_at IS NOT NULL AND archived_at IS NULL", true).
		Order("auto_retry_next_at ASC").
		Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		if record.AutoRetryNextAt == nil {
			continue
		}
		m.setAutoRetryTimer(record.ID, *record.AutoRetryNextAt)
	}
	return nil
}

func (m *Manager) recoverInterruptedSessions(ctx context.Context) error {
	db := model.GetDB()
	if db == nil {
		return model.ErrDBNotInitialized
	}

	var records []tables.WebSessionTable
	if err := db.WithContext(ctx).
		Where("status IN ?", []string{string(StatusRunning), string(StatusAborting)}).
		Order("updated_at ASC").
		Find(&records).Error; err != nil {
		return err
	}
	recoverable := make([]tables.WebSessionTable, 0, len(records))
	for _, record := range records {
		if effectiveAssistantState(record) == AssistantStateWaitingPlanApproval {
			continue
		}
		recoverable = append(recoverable, record)
	}
	if len(recoverable) == 0 {
		return nil
	}

	if m.logger != nil {
		m.logger.Info("recovering interrupted web sessions", zap.Int("count", len(recoverable)))
	}

	for _, record := range recoverable {
		now := time.Now()
		if _, err := m.appendAndBroadcast(ctx, record.ID, record, Event{
			Type:      "run_abort",
			Timestamp: now,
			Payload: map[string]any{
				"reason":     recoveryReasonProcessRestart,
				"msg":        recoveryMessageProcessRestart,
				"prevStatus": record.Status,
			},
		}); err != nil {
			return err
		}
		if err := m.updateRuntimeState(ctx, record.ID, map[string]any{
			"status":                     string(StatusIdle),
			"last_error":                 nil,
			"updated_at":                 now,
			"status_updated_at":          now,
			"auto_retry_attempt":         0,
			"auto_retry_next_at":         nil,
			"auto_retry_last_error_code": nil,
			"assistant_state":            nil,
			"assistant_state_updated_at": nil,
		}); err != nil {
			return err
		}
		m.cancelAutoRetryTimer(record.ID)
		m.broadcastSessionSummary(ctx, record.ID)
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

func (r *activeRun) markCompletedPlanTool() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completedPlanTool = true
}

func (r *activeRun) completedPlanToolSeen() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.completedPlanTool
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
	end := 0
	for idx, r := range value {
		next := idx + utf8.RuneLen(r)
		if next > limit {
			break
		}
		end = next
	}
	return value[:end] + "..."
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
	case "context_compaction":
		return "Context Compaction"
	case "mcp_tool_call":
		return "McpToolCall"
	case "file_change":
		return "FileChange"
	case "reasoning":
		return "Reasoning"
	case "web_search":
		return "WebSearch"
	default:
		return stringValue(item["type"])
	}
}

func codexToolInput(item map[string]any) any {
	switch normalizeCodexItemType(stringValue(item["type"])) {
	case "command_execution":
		return map[string]any{"command": stringValue(item["command"])}
	case "context_compaction":
		return nil
	case "web_search":
		return map[string]any{
			"query":  item["query"],
			"action": item["action"],
		}
	case "reasoning":
		return nil
	}
	return item
}

func codexToolMeta(item map[string]any) map[string]any {
	kind := normalizeCodexItemType(stringValue(item["type"]))
	subtitle := firstNonEmpty(
		stringValue(item["command"]),
		stringValue(item["tool_name"]),
		stringValue(item["path"]),
		stringValue(item["query"]),
		stringValue(item["text"]),
	)
	if kind == "file_change" {
		subtitle = firstNonEmpty(fileChangeSummary(item), subtitle)
	}
	if kind == "context_compaction" {
		subtitle = contextCompactionSubtitle(item)
	}
	return map[string]any{
		"kind":     kind,
		"title":    codexToolName(item),
		"subtitle": subtitle,
	}
}

func codexToolResult(item map[string]any) string {
	switch normalizeCodexItemType(stringValue(item["type"])) {
	case "reasoning":
		if text := extractReasoningText(item); text != "" {
			return text
		}
		return ""
	case "context_compaction":
		return extractContextCompactionText(item)
	}
	if output := stringValue(item["aggregated_output"]); output != "" {
		return output
	}
	if output := stringValue(item["aggregatedOutput"]); output != "" {
		return output
	}
	if text := stringValue(item["text"]); text != "" {
		return text
	}
	encoded, _ := json.Marshal(item)
	return string(encoded)
}

func toolOutputLimit(kind string) int {
	if normalizeCodexItemType(kind) == "plan" {
		return 0
	}
	return defaultToolOutputLimit
}

func truncateToolOutput(kind, value string) string {
	return truncateString(value, toolOutputLimit(kind))
}

func codexToolOutput(item map[string]any) string {
	return truncateToolOutput(stringValue(item["type"]), codexToolResult(item))
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
	case "contextCompaction":
		return "context_compaction"
	case "mcpToolCall":
		return "mcp_tool_call"
	case "fileChange":
		return "file_change"
	case "webSearch":
		return "web_search"
	case "agentMessage":
		return "agent_message"
	case "userMessage":
		return "user_message"
	default:
		return strings.TrimSpace(value)
	}
}

func extractContextCompactionText(item map[string]any) string {
	sections := make([]string, 0, 3)
	if summary := strings.TrimSpace(strings.Join(collectReasoningFragments(item["summary"]), "")); summary != "" {
		sections = append(sections, summary)
	}
	if text := strings.TrimSpace(stringValue(item["text"])); text != "" {
		sections = append(sections, text)
	}
	if content := strings.TrimSpace(strings.Join(collectReasoningFragments(item["content"]), "")); content != "" {
		sections = append(sections, content)
	}
	if output := strings.TrimSpace(strings.Join(collectReasoningFragments(item["output"]), "")); output != "" {
		sections = append(sections, output)
	}
	if len(sections) > 0 {
		return strings.TrimSpace(strings.Join(sections, "\n\n"))
	}
	encoded, _ := json.Marshal(item)
	return string(encoded)
}

func contextCompactionSubtitle(item map[string]any) string {
	text := strings.TrimSpace(extractContextCompactionText(item))
	if text == "" {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
}

func normalizeToolChoiceText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func codexToolIsPlan(item map[string]any) bool {
	meta := codexToolMeta(item)
	candidates := []string{
		codexToolName(item),
		stringValue(item["type"]),
		stringValue(meta["kind"]),
		stringValue(meta["title"]),
	}
	for _, candidate := range candidates {
		if normalizeToolChoiceText(candidate) == "plan" {
			return true
		}
	}
	return false
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
