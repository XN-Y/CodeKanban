package websession

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"code-kanban/model"
	"code-kanban/model/tables"

	"go.uber.org/zap"
)

type captureWSConn struct {
	frames []wireFrame
}

func (c *captureWSConn) ReadMessage() (messageType int, p []byte, err error) {
	return 0, nil, io.EOF
}

func (c *captureWSConn) WriteJSON(v any) error {
	frame, ok := v.(wireFrame)
	if !ok {
		return fmt.Errorf("unexpected frame type %T", v)
	}
	c.frames = append(c.frames, frame)
	return nil
}

func (c *captureWSConn) Close() error {
	return nil
}

func attachmentExtensionFromMime(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".png"
	}
}

func TestManagerCreateSessionAppendsOrderIndex(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	seedWebSession(t, project.ID, "First", 1000)
	seedWebSession(t, project.ID, "Second", 2000)

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID: project.ID,
		Agent:     AgentCodex,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if created.OrderIndex != 3000 {
		t.Fatalf("expected orderIndex 3000, got %.2f", created.OrderIndex)
	}
	if created.WorkflowMode != WorkflowModeDefault {
		t.Fatalf("expected default workflow mode, got %q", created.WorkflowMode)
	}
	if created.PermissionLevel != PermissionLevelElevated {
		t.Fatalf("expected elevated permission level, got %q", created.PermissionLevel)
	}
}

func TestManagerCreateSessionDefaultsCodexToAppServerBackend(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID: project.ID,
		Agent:     AgentCodex,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if effectiveSessionBackend(record) != SessionBackendCodexAppServer {
		t.Fatalf("expected codex sessions to default to %q, got %q", SessionBackendCodexAppServer, effectiveSessionBackend(record))
	}
}

func TestManagerBroadcastOnlyTargetsEventClients(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	commandConn := &captureWSConn{}
	eventConn := &captureWSConn{}
	commandClient := manager.RegisterCommandClient(commandConn)
	eventClient := manager.RegisterEventClient(eventConn)
	defer manager.UnregisterClient(commandClient)
	defer manager.UnregisterClient(eventClient)

	manager.broadcast(newSessionFrame("session-1", SessionSummary{
		ID:        "session-1",
		ProjectID: "project-1",
		Title:     "Session 1",
		Agent:     AgentCodex,
		Status:    StatusRunning,
	}))

	if len(commandConn.frames) != 0 {
		t.Fatalf("expected command client to receive no broadcast frames, got %d", len(commandConn.frames))
	}
	if len(eventConn.frames) != 1 {
		t.Fatalf("expected event client to receive exactly one broadcast frame, got %d", len(eventConn.frames))
	}
	if eventConn.frames[0].Kind != "evt" || eventConn.frames[0].Operation != "session" {
		t.Fatalf("expected session event frame, got kind=%q op=%q", eventConn.frames[0].Kind, eventConn.frames[0].Operation)
	}
}

func TestParseCodexContextWindowRootLevelOnly(t *testing.T) {
	raw := `
model_context_window = 1000000 # root setting

[model_providers.OpenAI]
model_context_window = 123
`

	got, ok := parseCodexContextWindow(raw)
	if !ok {
		t.Fatal("expected parseCodexContextWindow to succeed")
	}
	if got != 1000000 {
		t.Fatalf("expected 1000000, got %d", got)
	}
}

func TestManagerListSessionsIncludesConfiguredContextWindow(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	configDir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("model_context_window = 1000000\n"), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	project := seedProject(t)
	seedWebSession(t, project.ID, "Codex", 1000)

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	items, err := manager.ListSessions(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 session, got %d", len(items))
	}
	if items[0].ContextWindowTokens == nil || *items[0].ContextWindowTokens != 1000000 {
		t.Fatalf("expected contextWindowTokens 1000000, got %#v", items[0].ContextWindowTokens)
	}
	if items[0].ContextWindowSource != ContextWindowSourceConfig {
		t.Fatalf("expected contextWindowSource %q, got %q", ContextWindowSourceConfig, items[0].ContextWindowSource)
	}
	config := manager.GetCodexRuntimeConfig()
	if config.ContextWindowTokens != 1000000 {
		t.Fatalf("expected runtime context window 1000000, got %d", config.ContextWindowTokens)
	}
	if config.CompactLimitTokens != 1000000 {
		t.Fatalf("expected runtime compact limit fallback 1000000, got %d", config.CompactLimitTokens)
	}
}

func TestManagerListSessionsMarksClaudeContextWindowUnavailable(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Claude", 1000)
	if err := model.GetDB().Model(session).Update("agent", string(AgentClaude)).Error; err != nil {
		t.Fatalf("update session agent failed: %v", err)
	}

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	items, err := manager.ListSessions(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 session, got %d", len(items))
	}
	if items[0].ContextWindowTokens != nil {
		t.Fatalf("expected nil contextWindowTokens, got %#v", items[0].ContextWindowTokens)
	}
	if items[0].ContextWindowSource != ContextWindowSourceUnavailable {
		t.Fatalf("expected contextWindowSource %q, got %q", ContextWindowSourceUnavailable, items[0].ContextWindowSource)
	}
}

func TestDecodeToolQuestionsPreservesStructuredQuestions(t *testing.T) {
	questions := []toolRequestQuestion{
		{
			ID:       "scope",
			Header:   "范围",
			Question: "这次要验证哪种计划模式交互？",
			IsOther:  true,
			Options: []toolRequestOption{
				{Label: "仅草稿组内", Description: "保持现在的草稿分组。"},
				{Label: "整个标签系统统一", Description: "统一插入逻辑。"},
			},
		},
	}

	got := decodeToolQuestions(questions)
	if len(got) != 1 {
		t.Fatalf("expected 1 question, got %d", len(got))
	}
	if got[0].ID != questions[0].ID || got[0].Header != questions[0].Header || got[0].Question != questions[0].Question {
		t.Fatalf("expected question to be preserved, got %#v", got[0])
	}
	if len(got[0].Options) != len(questions[0].Options) {
		t.Fatalf("expected %d options, got %d", len(questions[0].Options), len(got[0].Options))
	}
}

func TestManagerMoveSessionRenormalizesProjectOrder(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	first := seedWebSession(t, project.ID, "First", 1000)
	second := seedWebSession(t, project.ID, "Second", 2000)
	third := seedWebSession(t, project.ID, "Third", 3000)

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	moved, err := manager.MoveSession(context.Background(), third.ID, "", first.ID)
	if err != nil {
		t.Fatalf("MoveSession returned error: %v", err)
	}
	if moved.OrderIndex != 1000 {
		t.Fatalf("expected moved session orderIndex 1000, got %.2f", moved.OrderIndex)
	}

	sessions, err := manager.ListSessions(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	expectedIDs := []string{third.ID, first.ID, second.ID}
	for index, session := range sessions {
		if session.ID != expectedIDs[index] {
			t.Fatalf("expected session %s at index %d, got %s", expectedIDs[index], index, session.ID)
		}
		expectedOrder := float64(index+1) * sessionOrderStep
		if session.OrderIndex != expectedOrder {
			t.Fatalf("expected orderIndex %.2f at index %d, got %.2f", expectedOrder, index, session.OrderIndex)
		}
	}
}

func TestManagerArchiveSessionKeepsHistoryAndMovesSessionOutOfCurrentList(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	dataDir := t.TempDir()
	session := seedWebSession(t, project.ID, "Archive Me", 1000)

	manager, err := NewManager(Config{DataDir: dataDir}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	if err := manager.store.appendEvent(session.ID, Event{
		ID:        "evt_history",
		Seq:       1,
		Type:      "note",
		Timestamp: time.Now(),
		Payload: map[string]any{
			"txt": "keep this history",
		},
	}); err != nil {
		t.Fatalf("appendEvent returned error: %v", err)
	}

	archived, err := manager.ArchiveSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("ArchiveSession returned error: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatalf("expected archivedAt to be set")
	}

	current, err := manager.ListSessions(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(current) != 0 {
		t.Fatalf("expected archived session to be removed from current list, got %d items", len(current))
	}

	archivedResult, err := manager.ListArchivedSessions(context.Background(), []string{project.ID}, 20, 0)
	if err != nil {
		t.Fatalf("ListArchivedSessions returned error: %v", err)
	}
	if archivedResult.Total != 1 || len(archivedResult.Items) != 1 {
		t.Fatalf("expected exactly one archived session, got total=%d items=%d", archivedResult.Total, len(archivedResult.Items))
	}
	if archivedResult.Items[0].ID != session.ID {
		t.Fatalf("expected archived session %s, got %s", session.ID, archivedResult.Items[0].ID)
	}
	if _, err := os.Stat(manager.store.historyPath(session.ID)); err != nil {
		t.Fatalf("expected archived history file to remain on disk: %v", err)
	}
}

func TestManagerUnarchiveSessionRestoresSessionToCurrentListEnd(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	first := seedWebSession(t, project.ID, "First", 1000)
	second := seedWebSession(t, project.ID, "Second", 2000)
	archivedAt := time.Now().Add(-time.Hour)
	if err := model.GetDB().Model(&tables.WebSessionTable{}).
		Where("id = ?", first.ID).
		Updates(map[string]any{
			"archived_at": archivedAt,
			"updated_at":  time.Now(),
		}).Error; err != nil {
		t.Fatalf("failed to archive seed session: %v", err)
	}

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	restored, err := manager.UnarchiveSession(context.Background(), first.ID)
	if err != nil {
		t.Fatalf("UnarchiveSession returned error: %v", err)
	}
	if restored.ArchivedAt != nil {
		t.Fatalf("expected archivedAt to be cleared")
	}
	if restored.OrderIndex <= second.OrderIndex {
		t.Fatalf("expected restored session to move to the end, got orderIndex %.2f", restored.OrderIndex)
	}

	current, err := manager.ListSessions(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(current) != 2 {
		t.Fatalf("expected 2 current sessions, got %d", len(current))
	}
	if current[0].ID != second.ID || current[1].ID != first.ID {
		t.Fatalf("unexpected current session order after unarchive: %#v", current)
	}
}

func TestManagerListArchivedSessionsPaginatesByActivityDescending(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	now := time.Now()
	sessionA := seedWebSession(t, project.ID, "A", 1000)
	sessionB := seedWebSession(t, project.ID, "B", 2000)
	sessionC := seedWebSession(t, project.ID, "C", 3000)
	for id, activityAt := range map[string]time.Time{
		sessionA.ID: now.Add(-3 * time.Hour),
		sessionB.ID: now.Add(-1 * time.Hour),
		sessionC.ID: now.Add(-2 * time.Hour),
	} {
		if err := model.GetDB().Model(&tables.WebSessionTable{}).
			Where("id = ?", id).
			Updates(map[string]any{
				"archived_at": now,
				"activity_at": activityAt,
				"updated_at":  now,
			}).Error; err != nil {
			t.Fatalf("failed to update archived seed %s: %v", id, err)
		}
	}

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	pageOne, err := manager.ListArchivedSessions(context.Background(), []string{project.ID}, 2, 0)
	if err != nil {
		t.Fatalf("ListArchivedSessions page one returned error: %v", err)
	}
	if !pageOne.HasMore || pageOne.NextOffset != 2 {
		t.Fatalf("expected first page to have more results, got %+v", pageOne)
	}
	if len(pageOne.Items) != 2 || pageOne.Items[0].ID != sessionB.ID || pageOne.Items[1].ID != sessionC.ID {
		t.Fatalf("unexpected first archived page order: %#v", pageOne.Items)
	}

	pageTwo, err := manager.ListArchivedSessions(context.Background(), []string{project.ID}, 2, pageOne.NextOffset)
	if err != nil {
		t.Fatalf("ListArchivedSessions page two returned error: %v", err)
	}
	if pageTwo.HasMore {
		t.Fatalf("expected second page to be final, got %+v", pageTwo)
	}
	if len(pageTwo.Items) != 1 || pageTwo.Items[0].ID != sessionA.ID {
		t.Fatalf("unexpected second archived page order: %#v", pageTwo.Items)
	}
}

func TestDetectApprovalPrompt(t *testing.T) {
	t.Run("codex confirm prompt", func(t *testing.T) {
		prompt, ok := detectApprovalPrompt([]string{
			"❯ 1. Approve",
			"› 2. Cancel",
			"  Press enter to confirm or esc to cancel",
		})
		if !ok {
			t.Fatalf("expected approval prompt to be detected")
		}
		if prompt == "" {
			t.Fatalf("expected non-empty approval prompt")
		}
	})

	t.Run("claude proceed prompt", func(t *testing.T) {
		prompt, ok := detectApprovalPrompt([]string{
			"Do you want to proceed?",
			"Esc to exit",
		})
		if !ok {
			t.Fatalf("expected approval prompt to be detected")
		}
		if prompt == "" {
			t.Fatalf("expected non-empty approval prompt")
		}
	})
}

func TestBuildExecCommandCodexClosesStdinWhenPromptArgProvided(t *testing.T) {
	manager := &Manager{cfg: Config{CodexPath: "codex"}}
	session := tables.WebSessionTable{
		Agent:           string(AgentCodex),
		Model:           "gpt-5.4",
		WorkflowMode:    string(WorkflowModeDefault),
		PermissionLevel: string(PermissionLevelDefault),
		Cwd:             "/tmp/project",
	}

	cmd, stdinBytes, closeStdinAfterWrite, err := manager.buildExecCommand(
		context.Background(),
		session,
		"say hi briefly",
		nil,
	)
	if err != nil {
		t.Fatalf("buildExecCommand returned error: %v", err)
	}
	if closeStdinAfterWrite != true {
		t.Fatalf("expected stdin to be closed after launch when prompt arg is provided")
	}
	if len(stdinBytes) != 0 {
		t.Fatalf("expected no stdin bytes when using prompt argument, got %q", string(stdinBytes))
	}
	joinedArgs := strings.Join(cmd.Args, " ")
	if strings.Contains(joinedArgs, " - ") || strings.HasSuffix(joinedArgs, " -") {
		t.Fatalf("expected prompt argument mode, got args %v", cmd.Args)
	}
	if !strings.Contains(joinedArgs, "say hi briefly") {
		t.Fatalf("expected prompt to be passed as an argument, got args %v", cmd.Args)
	}
	if !strings.Contains(joinedArgs, "-s workspace-write") {
		t.Fatalf("expected default codex permissions to use workspace-write sandbox, got args %v", cmd.Args)
	}
}

func TestBuildExecCommandCodexElevatedPlanAddsPreambleAndFullAccess(t *testing.T) {
	manager := &Manager{cfg: Config{CodexPath: "codex"}}
	session := tables.WebSessionTable{
		Agent:           string(AgentCodex),
		Model:           "gpt-5.4",
		WorkflowMode:    string(WorkflowModePlan),
		PermissionLevel: string(PermissionLevelElevated),
		Cwd:             "/tmp/project",
	}

	cmd, stdinBytes, closeStdinAfterWrite, err := manager.buildExecCommand(
		context.Background(),
		session,
		"inspect this repo",
		nil,
	)
	if err != nil {
		t.Fatalf("buildExecCommand returned error: %v", err)
	}
	if closeStdinAfterWrite != true {
		t.Fatalf("expected stdin to be closed after launch when prompt arg is provided")
	}
	if len(stdinBytes) != 0 {
		t.Fatalf("expected no stdin bytes for prompt argument mode, got %q", string(stdinBytes))
	}
	joinedArgs := strings.Join(cmd.Args, " ")
	if !strings.Contains(joinedArgs, "-s danger-full-access") {
		t.Fatalf("expected elevated codex permissions to use danger-full-access, got args %v", cmd.Args)
	}
	if !strings.Contains(joinedArgs, "You are operating in planning mode.") {
		t.Fatalf("expected plan preamble to be injected, got args %v", cmd.Args)
	}
}

func TestNewManagerMigratesLegacyPermissionMode(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	legacySession := &tables.WebSessionTable{
		ProjectID:            project.ID,
		OrderIndex:           1000,
		Agent:                string(AgentCodex),
		Title:                "Legacy",
		Model:                "gpt-5.4",
		WorkflowMode:         "",
		PermissionLevel:      "",
		LegacyPermissionMode: "plan",
		Cwd:                  t.TempDir(),
		Status:               string(StatusIdle),
	}
	legacySession.Init()
	if err := model.GetDB().Create(legacySession).Error; err != nil {
		t.Fatalf("seed legacy web session failed: %v", err)
	}

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	record, err := manager.GetSession(context.Background(), legacySession.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if effectiveWorkflowMode(record) != WorkflowModePlan {
		t.Fatalf("expected migrated workflow mode plan, got %q", effectiveWorkflowMode(record))
	}
	if effectivePermissionLevel(record) != PermissionLevelElevated {
		t.Fatalf("expected migrated permission level elevated, got %q", effectivePermissionLevel(record))
	}
}

func TestNewManagerRecoversInterruptedSessionsOnStartup(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	dataDir := t.TempDir()
	eventStore, err := newStore(dataDir)
	if err != nil {
		t.Fatalf("newStore returned error: %v", err)
	}

	nativeSessionID := "thread_existing"
	session := &tables.WebSessionTable{
		ProjectID:            project.ID,
		OrderIndex:           1000,
		Agent:                string(AgentCodex),
		Backend:              string(SessionBackendCodexAppServer),
		Title:                "Recover Me",
		Model:                "gpt-5.4",
		WorkflowMode:         string(WorkflowModeDefault),
		PermissionLevel:      string(PermissionLevelElevated),
		Cwd:                  t.TempDir(),
		NativeSessionID:      &nativeSessionID,
		Status:               string(StatusRunning),
		LastEventSeq:         1,
		HasUnread:            true,
		LastMessageAt:        nil,
		LegacyPermissionMode: "default",
	}
	session.Init()
	if err := model.GetDB().Create(session).Error; err != nil {
		t.Fatalf("seed web session failed: %v", err)
	}

	if err := eventStore.appendEvent(session.ID, Event{
		ID:        "evt_approval",
		Seq:       1,
		Type:      "approval_req",
		Timestamp: time.Now().Add(-time.Minute),
		Payload: map[string]any{
			"prompt": "Need approval to continue",
		},
	}); err != nil {
		t.Fatalf("appendEvent returned error: %v", err)
	}

	manager, err := NewManager(Config{DataDir: dataDir}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	record, err := manager.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Status != string(StatusIdle) {
		t.Fatalf("expected recovered status %q, got %q", StatusIdle, record.Status)
	}
	if record.HasUnread {
		t.Fatalf("expected recovered session unread flag to be cleared")
	}
	if record.NativeSessionID == nil || strings.TrimSpace(*record.NativeSessionID) != nativeSessionID {
		t.Fatalf("expected native session id %q to be preserved, got %v", nativeSessionID, record.NativeSessionID)
	}
	if record.LastEventSeq != 2 {
		t.Fatalf("expected recovered lastEventSeq 2, got %d", record.LastEventSeq)
	}

	history, err := manager.History(context.Background(), session.ID, 10, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if len(history.Events) != 2 {
		t.Fatalf("expected 2 events after recovery, got %d", len(history.Events))
	}
	lastEvent := history.Events[len(history.Events)-1]
	if lastEvent.Type != "run_abort" {
		t.Fatalf("expected recovery event run_abort, got %q", lastEvent.Type)
	}
	if got := fmt.Sprint(lastEvent.Payload["reason"]); got != recoveryReasonProcessRestart {
		t.Fatalf("expected recovery reason %q, got %q", recoveryReasonProcessRestart, got)
	}
	if got := fmt.Sprint(lastEvent.Payload["msg"]); !strings.Contains(got, "app restarted") {
		t.Fatalf("expected recovery message to mention app restart, got %q", got)
	}
}

func TestSendMessageAutoRenamesTitleFromFirstUserMessage(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "basic"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID: project.ID,
		Agent:     AgentCodex,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	messageText := "修复网页会话标题自动命名的问题，并补一个回归测试。"
	if err := manager.SendMessage(context.Background(), created.ID, messageText, nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Title != messageText {
		t.Fatalf("expected auto title %q, got %q", messageText, record.Title)
	}
	if record.TitleAuto {
		t.Fatalf("expected title auto flag to be cleared")
	}
}

func TestSendMessageDoesNotOverrideManualTitle(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "basic"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID: project.ID,
		Agent:     AgentCodex,
		Title:     "Manual Title",
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "这条消息不应该覆盖手动标题。", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Title != "Manual Title" {
		t.Fatalf("expected manual title to be preserved, got %q", record.Title)
	}
	if record.TitleAuto {
		t.Fatalf("expected manual title to remain non-auto")
	}
}

func TestSendMessageCodexAppServerPersistsThreadID(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "basic"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID: project.ID,
		Agent:     AgentCodex,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "inspect", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.NativeSessionID == nil || strings.TrimSpace(*record.NativeSessionID) != "thread_test" {
		t.Fatalf("expected native session id thread_test, got %v", record.NativeSessionID)
	}
	if effectiveSessionBackend(record) != SessionBackendCodexAppServer {
		t.Fatalf("expected app-server backend, got %q", effectiveSessionBackend(record))
	}
	history, err := manager.History(context.Background(), created.ID, 200, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if historyHasToolKind(history.Events, "reasoning") {
		t.Fatalf("expected empty reasoning items to be filtered from projected history, got %#v", history.Events)
	}

	rawEvents, err := manager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error: %v", err)
	}
	if !historyHasToolKind(rawEvents, "reasoning") {
		t.Fatalf("expected raw history to retain reasoning items, got %#v", rawEvents)
	}
}

func TestCodexAppServerTransportRetryPersistsAsNoteAndCompletes(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "reconnect_then_success"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID: project.ID,
		Agent:     AgentCodex,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "inspect", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Status != string(StatusDone) {
		t.Fatalf("expected session status %q, got %q", StatusDone, record.Status)
	}

	rawEvents, err := manager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error: %v", err)
	}
	if historyHasEvent(rawEvents, "run_fail") {
		t.Fatalf("expected retrying run to avoid run_fail, got %#v", rawEvents)
	}
	retryNoteFound := false
	for _, event := range rawEvents {
		if event.Type != "note" {
			continue
		}
		if stringValue(event.Payload["code"]) != codexTransportRetryingCode {
			continue
		}
		retryNoteFound = true
		if got := int(numberValue(event.Payload["attempt"])); got != 1 {
			t.Fatalf("expected retry attempt 1, got %d", got)
		}
		if got := int(numberValue(event.Payload["maxAttempts"])); got != 5 {
			t.Fatalf("expected max attempts 5, got %d", got)
		}
		break
	}
	if !retryNoteFound {
		t.Fatalf("expected transport retry note in raw events, got %#v", rawEvents)
	}

	history, err := manager.History(context.Background(), created.ID, 50, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	retryItemFound := false
	for _, item := range history.Items {
		if item.ItemType != "note" {
			continue
		}
		if stringValue(item.Payload["code"]) != codexTransportRetryingCode {
			continue
		}
		retryItemFound = true
		break
	}
	if !retryItemFound {
		t.Fatalf("expected projected retry note in history items, got %#v", history.Items)
	}
}

func TestCodexAppServerTransportRetryExhaustionFailsRun(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "reconnect_then_fail"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID: project.ID,
		Agent:     AgentCodex,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "inspect", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Status != string(StatusError) {
		t.Fatalf("expected session status %q, got %q", StatusError, record.Status)
	}

	rawEvents, err := manager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error: %v", err)
	}
	retryNoteCount := 0
	var finalFailure Event
	for _, event := range rawEvents {
		if event.Type == "note" && stringValue(event.Payload["code"]) == codexTransportRetryingCode {
			retryNoteCount++
		}
		if event.Type == "run_fail" {
			finalFailure = event
		}
	}
	if retryNoteCount < 2 {
		t.Fatalf("expected multiple retry notes before failure, got %#v", rawEvents)
	}
	if finalFailure.Type != "run_fail" {
		t.Fatalf("expected final run_fail event, got %#v", rawEvents)
	}
	if got := stringValue(finalFailure.Payload["code"]); got != codexTransportRetryExhaustedCode {
		t.Fatalf("expected final failure code %q, got %q", codexTransportRetryExhaustedCode, got)
	}

	history, err := manager.History(context.Background(), created.ID, 50, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	historyRetryCount := 0
	historyFailFound := false
	for _, item := range history.Items {
		if item.ItemType == "note" && stringValue(item.Payload["code"]) == codexTransportRetryingCode {
			historyRetryCount++
		}
		if item.ItemType == "run_fail" && stringValue(item.Payload["code"]) == codexTransportRetryExhaustedCode {
			historyFailFound = true
		}
	}
	if historyRetryCount < 2 {
		t.Fatalf("expected retry notes in projected history, got %#v", history.Items)
	}
	if !historyFailFound {
		t.Fatalf("expected projected final run_fail item, got %#v", history.Items)
	}
}

func TestAutoRetryEnabledSessionContinuesAfterFailure(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "auto_retry_then_success"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID:        project.ID,
		Agent:            AgentCodex,
		AutoRetryEnabled: true,
		AutoRetryScope:   AutoRetryScopeNetworkOnly,
		AutoRetryPreset:  AutoRetryPresetAggressiveStop,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "inspect", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		record, getErr := manager.GetSession(context.Background(), created.ID)
		if getErr != nil {
			t.Fatalf("GetSession returned error: %v", getErr)
		}
		if record.Status == string(StatusDone) && !manager.hasActiveRun(created.ID) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Status != string(StatusDone) {
		t.Fatalf("expected session status %q after auto retry, got %q", StatusDone, record.Status)
	}

	rawEvents, err := manager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error: %v", err)
	}
	userMessages := make([]Event, 0, 2)
	for _, event := range rawEvents {
		if event.Type == "msg_u" {
			userMessages = append(userMessages, event)
		}
	}
	if len(userMessages) < 2 {
		t.Fatalf("expected auto retry to append a second user message, got %#v", rawEvents)
	}
	if got := stringValue(userMessages[len(userMessages)-1].Payload["txt"]); got != "continue" {
		t.Fatalf("expected automatic retry message %q, got %q", "continue", got)
	}
}

func TestRespondToUserInputCodexAppServer(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "user_input"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID: project.ID,
		Agent:     AgentCodex,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "plan this change", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	request := waitForPendingServerRequest(t, manager, created.ID, pendingServerRequestUserInput)
	if request == nil {
		t.Fatal("expected pending user input request")
	}
	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Status != string(StatusRunning) {
		t.Fatalf("expected session status %q while waiting for input, got %q", StatusRunning, record.Status)
	}
	if record.AssistantState != string(AssistantStateWaitingInput) {
		t.Fatalf("expected assistant state %q, got %q", AssistantStateWaitingInput, record.AssistantState)
	}

	if err := manager.respondToUserInput(created.ID, request.ItemID, map[string][]string{
		"scope": {"full migration"},
	}); err != nil {
		t.Fatalf("respondToUserInput returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	rawEvents, err := manager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error: %v", err)
	}
	if !historyHasEvent(rawEvents, "user_input_req") {
		t.Fatalf("expected user_input_req event, got %#v", rawEvents)
	}
	if !historyHasEvent(rawEvents, "user_input_res") {
		t.Fatalf("expected user_input_res event, got %#v", rawEvents)
	}
}

func TestUserInputRequestProjectionPersistsSourceItemID(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Needs Input", 1000)

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	requestID := "req_input_123"
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_user_input",
		Seq:       1,
		Type:      "user_input_req",
		Timestamp: time.Now(),
		Payload: map[string]any{
			"iid": requestID,
			"txt": "Please choose a scope",
			"qs": []map[string]any{
				{
					"id":       "scope",
					"header":   "Scope",
					"question": "Which scope should I use?",
					"options": []map[string]any{
						{
							"label":       "Full migration",
							"description": "Apply all changes",
						},
					},
				},
			},
		},
	})

	history, err := manager.History(context.Background(), session.ID, 10, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if len(history.Items) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(history.Items))
	}
	if history.Items[0].SourceItemID == nil || *history.Items[0].SourceItemID != requestID {
		t.Fatalf("expected source item id %q, got %v", requestID, history.Items[0].SourceItemID)
	}

	snapshot, err := manager.Snapshot(context.Background(), session.ID, 10)
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	frame := newSnapshotFrame(session.ID, snapshot)
	if frame.History == nil || len(frame.History.Items) != 1 {
		t.Fatalf("expected snapshot frame history item, got %#v", frame.History)
	}
	if frame.History.Items[0].SourceItemID == nil || *frame.History.Items[0].SourceItemID != requestID {
		t.Fatalf(
			"expected wire snapshot source item id %q, got %v",
			requestID,
			frame.History.Items[0].SourceItemID,
		)
	}
}

func TestRespondToApprovalCodexAppServer(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "approval"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID: project.ID,
		Agent:     AgentCodex,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "make the edit", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	request := waitForPendingServerRequest(t, manager, created.ID, pendingServerRequestFileChangeApproval)
	if request == nil {
		t.Fatal("expected pending approval request")
	}
	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Status != string(StatusRunning) {
		t.Fatalf("expected session status %q while waiting for approval, got %q", StatusRunning, record.Status)
	}

	if err := manager.respondToApproval(created.ID, "approve"); err != nil {
		t.Fatalf("respondToApproval returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	rawEvents, err := manager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error: %v", err)
	}
	if !historyHasEvent(rawEvents, "approval_req") {
		t.Fatalf("expected approval_req event, got %#v", rawEvents)
	}
	if !historyHasEvent(rawEvents, "approval_res") {
		t.Fatalf("expected approval_res event, got %#v", rawEvents)
	}
}

func TestCodexPlanCompletionSetsWaitingApprovalStatus(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "plan"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID:    project.ID,
		Agent:        AgentCodex,
		WorkflowMode: WorkflowModePlan,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "inspect and plan", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Status != string(StatusRunning) {
		t.Fatalf("expected session status %q, got %q", StatusRunning, record.Status)
	}
	if record.AssistantState != string(AssistantStateWaitingPlanApproval) {
		t.Fatalf("expected assistant state %q, got %q", AssistantStateWaitingPlanApproval, record.AssistantState)
	}

	history, err := manager.History(context.Background(), created.ID, 200, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if !historyHasToolKind(history.Events, "plan") {
		t.Fatalf("expected plan tool history, got %#v", history.Events)
	}
}

func TestCodexPlanCompletionUsesDoneStatusOutsidePlanWorkflow(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "plan"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID:    project.ID,
		Agent:        AgentCodex,
		WorkflowMode: WorkflowModeDefault,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "plan and continue", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Status != string(StatusDone) {
		t.Fatalf("expected session status %q, got %q", StatusDone, record.Status)
	}
	if record.AssistantState != "" {
		t.Fatalf("expected assistant state to be cleared, got %q", record.AssistantState)
	}
}

func TestSendMessageClearsWaitingApprovalStatus(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "plan"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	created, err := manager.CreateSession(context.Background(), CreateParams{
		ProjectID:    project.ID,
		Agent:        AgentCodex,
		WorkflowMode: WorkflowModePlan,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "plan first", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	record, err := manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Status != string(StatusRunning) {
		t.Fatalf("expected first completion status %q, got %q", StatusRunning, record.Status)
	}
	if record.AssistantState != string(AssistantStateWaitingPlanApproval) {
		t.Fatalf("expected first assistant state %q, got %q", AssistantStateWaitingPlanApproval, record.AssistantState)
	}

	if err := manager.SendMessage(context.Background(), created.ID, "implement now", nil); err != nil {
		t.Fatalf("second SendMessage returned error: %v", err)
	}

	record, err = manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error after second send: %v", err)
	}
	if record.Status != string(StatusRunning) {
		t.Fatalf("expected second send to move status to %q, got %q", StatusRunning, record.Status)
	}
	if record.AssistantState != string(AssistantStateWorking) {
		t.Fatalf("expected second send to move assistant state to %q, got %q", AssistantStateWorking, record.AssistantState)
	}

	waitForSessionToSettle(t, manager, created.ID)

	record, err = manager.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession returned error after second completion: %v", err)
	}
	if record.Status != string(StatusRunning) {
		t.Fatalf("expected second completion status %q, got %q", StatusRunning, record.Status)
	}
	if record.AssistantState != string(AssistantStateWaitingPlanApproval) {
		t.Fatalf("expected second assistant state %q, got %q", AssistantStateWaitingPlanApproval, record.AssistantState)
	}
}

func TestSendMessageResumesRecoveredCodexSession(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	nativeSessionID := "thread_resume_only"
	session := &tables.WebSessionTable{
		ProjectID:            project.ID,
		OrderIndex:           1000,
		Agent:                string(AgentCodex),
		Backend:              string(SessionBackendCodexAppServer),
		Title:                "Resume Existing",
		Model:                "gpt-5.4",
		WorkflowMode:         string(WorkflowModeDefault),
		PermissionLevel:      string(PermissionLevelElevated),
		LegacyPermissionMode: "default",
		Cwd:                  t.TempDir(),
		NativeSessionID:      &nativeSessionID,
		Status:               string(StatusRunning),
	}
	session.Init()
	if err := model.GetDB().Create(session).Error; err != nil {
		t.Fatalf("seed web session failed: %v", err)
	}

	manager, err := NewManager(Config{
		DataDir:   t.TempDir(),
		CodexPath: writeFakeCodexAppServerCLI(t, "resume_only"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	if err := manager.SendMessage(context.Background(), session.ID, "continue the existing thread", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, session.ID)

	record, err := manager.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if record.Status != string(StatusDone) {
		t.Fatalf("expected resumed session status %q, got %q", StatusDone, record.Status)
	}
	if record.NativeSessionID == nil || strings.TrimSpace(*record.NativeSessionID) != nativeSessionID {
		t.Fatalf("expected resumed native session id %q, got %v", nativeSessionID, record.NativeSessionID)
	}

	history, err := manager.History(context.Background(), session.ID, 20, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if historyHasEvent(history.Events, "run_fail") {
		t.Fatalf("expected resume_only session to avoid run_fail, got %#v", history.Events)
	}
}

func TestHistoryAggregatesConsecutiveCommandExecutions(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Grouped Commands", 1000)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd1_st",
		Seq:       1,
		Type:      "tool_st",
		Timestamp: time.UnixMilli(1_000),
		Payload: map[string]any{
			"tid":  "cmd1",
			"name": "CommandExecution",
			"kind": "command_execution",
			"in":   map[string]any{"command": "ls"},
			"meta": map[string]any{"kind": "command_execution", "title": "CommandExecution", "subtitle": "ls"},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd1_end",
		Seq:       2,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(2_000),
		Payload: map[string]any{
			"tid": "cmd1",
			"out": "ls output",
			"ok":  true,
			"meta": map[string]any{
				"kind":  "command_execution",
				"title": "CommandExecution",
			},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_reasoning_empty_end",
		Seq:       3,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(2_500),
		Payload: map[string]any{
			"tid":  "rs1",
			"name": "Reasoning",
			"kind": "reasoning",
			"out":  "",
			"ok":   true,
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd2_st",
		Seq:       4,
		Type:      "tool_st",
		Timestamp: time.UnixMilli(3_000),
		Payload: map[string]any{
			"tid":  "cmd2",
			"name": "CommandExecution",
			"kind": "command_execution",
			"in":   map[string]any{"command": "pwd"},
			"meta": map[string]any{"kind": "command_execution", "title": "CommandExecution", "subtitle": "pwd"},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd2_end",
		Seq:       5,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(4_000),
		Payload: map[string]any{
			"tid": "cmd2",
			"out": "pwd output",
			"ok":  true,
			"meta": map[string]any{
				"kind":  "command_execution",
				"title": "CommandExecution",
			},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_note",
		Seq:       6,
		Type:      "note",
		Timestamp: time.UnixMilli(5_000),
		Payload: map[string]any{
			"txt": "done",
		},
	})

	history, err := manager.History(context.Background(), session.ID, 20, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if len(history.Events) != 2 {
		t.Fatalf("expected 2 projected events, got %d", len(history.Events))
	}

	grouped := history.Events[0]
	if grouped.Type != "tool_end" {
		t.Fatalf("expected grouped event type tool_end, got %q", grouped.Type)
	}
	if grouped.Seq != 5 {
		t.Fatalf("expected grouped event seq 5, got %d", grouped.Seq)
	}
	if got := fmt.Sprint(grouped.Payload["tid"]); got != commandExecutionGroupID("cmd1") {
		t.Fatalf("expected grouped tool id %q, got %q", commandExecutionGroupID("cmd1"), got)
	}
	groupMeta := decodeRawObject(decodeRawObject(grouped.Payload["meta"])["commandGroup"])
	if got := int(numberValue(groupMeta["count"])); got != 2 {
		t.Fatalf("expected grouped count 2, got %d", got)
	}
	input := decodeRawObject(grouped.Payload["in"])
	if got := stringValue(input["command"]); got != "pwd" {
		t.Fatalf("expected latest grouped command pwd, got %q", got)
	}
	if got := stringValue(grouped.Payload["out"]); got != "pwd output" {
		t.Fatalf("expected latest grouped output, got %q", got)
	}

	if history.Events[1].Type != "note" {
		t.Fatalf("expected second event note, got %q", history.Events[1].Type)
	}
}

func TestGetCommandExecutionGroupReturnsFullItems(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Grouped Commands", 1000)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd1_st",
		Seq:       1,
		Type:      "tool_st",
		Timestamp: time.UnixMilli(1_000),
		Payload: map[string]any{
			"tid":  "cmd1",
			"name": "CommandExecution",
			"kind": "command_execution",
			"in":   map[string]any{"command": "ls"},
			"meta": map[string]any{"kind": "command_execution", "title": "CommandExecution", "subtitle": "ls"},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd1_end",
		Seq:       2,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(2_000),
		Payload: map[string]any{
			"tid": "cmd1",
			"out": "ls output",
			"ok":  true,
			"meta": map[string]any{
				"kind":  "command_execution",
				"title": "CommandExecution",
			},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd2_st",
		Seq:       3,
		Type:      "tool_st",
		Timestamp: time.UnixMilli(3_000),
		Payload: map[string]any{
			"tid":  "cmd2",
			"name": "CommandExecution",
			"kind": "command_execution",
			"in":   map[string]any{"command": "pwd"},
			"meta": map[string]any{"kind": "command_execution", "title": "CommandExecution", "subtitle": "pwd"},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd2_end",
		Seq:       4,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(4_000),
		Payload: map[string]any{
			"tid": "cmd2",
			"out": "pwd output",
			"ok":  false,
			"meta": map[string]any{
				"kind":  "command_execution",
				"title": "CommandExecution",
			},
		},
	})

	group, err := manager.GetCommandExecutionGroup(
		context.Background(),
		session.ID,
		commandExecutionGroupID("cmd1"),
	)
	if err != nil {
		t.Fatalf("GetCommandExecutionGroup returned error: %v", err)
	}
	if group.Count != 2 {
		t.Fatalf("expected group count 2, got %d", group.Count)
	}
	if group.FirstSeq != 1 || group.LastSeq != 4 {
		t.Fatalf("expected seq range 1-4, got %d-%d", group.FirstSeq, group.LastSeq)
	}
	if group.Status != "error" {
		t.Fatalf("expected latest status error, got %q", group.Status)
	}
	if len(group.Items) != 2 {
		t.Fatalf("expected 2 group items, got %d", len(group.Items))
	}
	if group.Items[0].Command != "ls" || group.Items[0].Output != "ls output" {
		t.Fatalf("unexpected first group item: %#v", group.Items[0])
	}
	if group.Items[1].Command != "pwd" || group.Items[1].Status != "error" {
		t.Fatalf("unexpected second group item: %#v", group.Items[1])
	}
}

func TestHistoryReasoningWithContentBreaksCommandExecutionGroup(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Grouped Commands", 1000)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd1_st",
		Seq:       1,
		Type:      "tool_st",
		Timestamp: time.UnixMilli(1_000),
		Payload: map[string]any{
			"tid":  "cmd1",
			"name": "CommandExecution",
			"kind": "command_execution",
			"in":   map[string]any{"command": "ls"},
			"meta": map[string]any{"kind": "command_execution", "title": "CommandExecution", "subtitle": "ls"},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd1_end",
		Seq:       2,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(2_000),
		Payload: map[string]any{
			"tid": "cmd1",
			"out": "ls output",
			"ok":  true,
			"meta": map[string]any{
				"kind":  "command_execution",
				"title": "CommandExecution",
			},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_reasoning_end",
		Seq:       3,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(2_500),
		Payload: map[string]any{
			"tid":  "rs1",
			"name": "Reasoning",
			"kind": "reasoning",
			"out":  "Need to try a different command.",
			"ok":   true,
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd2_st",
		Seq:       4,
		Type:      "tool_st",
		Timestamp: time.UnixMilli(3_000),
		Payload: map[string]any{
			"tid":  "cmd2",
			"name": "CommandExecution",
			"kind": "command_execution",
			"in":   map[string]any{"command": "pwd"},
			"meta": map[string]any{"kind": "command_execution", "title": "CommandExecution", "subtitle": "pwd"},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd2_end",
		Seq:       5,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(4_000),
		Payload: map[string]any{
			"tid": "cmd2",
			"out": "pwd output",
			"ok":  true,
			"meta": map[string]any{
				"kind":  "command_execution",
				"title": "CommandExecution",
			},
		},
	})

	history, err := manager.History(context.Background(), session.ID, 20, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if len(history.Events) != 3 {
		t.Fatalf("expected 3 projected events, got %d", len(history.Events))
	}
	if history.Events[0].Type != "tool_end" || eventToolKind(history.Events[0]) != "command_execution" {
		t.Fatalf("expected first event grouped command execution, got %#v", history.Events[0])
	}
	if history.Events[1].Type != "tool_end" || eventToolKind(history.Events[1]) != "reasoning" {
		t.Fatalf("expected second event reasoning, got %#v", history.Events[1])
	}
	if history.Events[2].Type != "tool_end" || eventToolKind(history.Events[2]) != "command_execution" {
		t.Fatalf("expected third event grouped command execution, got %#v", history.Events[2])
	}
}

func TestHistoryAggregatesConsecutiveFileChanges(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Grouped File Changes", 1000)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_fc1_st",
		Seq:       1,
		Type:      "tool_st",
		Timestamp: time.UnixMilli(1_000),
		Payload: map[string]any{
			"tid":  "fc1",
			"name": "FileChange",
			"kind": "file_change",
			"in": map[string]any{
				"path": "ui/src/App.vue",
				"changes": []any{
					map[string]any{"path": "ui/src/App.vue"},
				},
			},
			"meta": map[string]any{"kind": "file_change", "title": "FileChange", "subtitle": "ui/src/App.vue"},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_fc1_end",
		Seq:       2,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(2_000),
		Payload: map[string]any{
			"tid": "fc1",
			"out": "patched",
			"ok":  true,
			"meta": map[string]any{
				"kind": "file_change", "title": "FileChange", "subtitle": "ui/src/App.vue",
			},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_fc2_st",
		Seq:       3,
		Type:      "tool_st",
		Timestamp: time.UnixMilli(3_000),
		Payload: map[string]any{
			"tid":  "fc2",
			"name": "FileChange",
			"kind": "file_change",
			"in": map[string]any{
				"path": "ui/src/components/Panel.vue",
				"changes": []any{
					map[string]any{"path": "ui/src/components/Panel.vue"},
				},
			},
			"meta": map[string]any{"kind": "file_change", "title": "FileChange", "subtitle": "ui/src/components/Panel.vue"},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_fc2_end",
		Seq:       4,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(4_000),
		Payload: map[string]any{
			"tid": "fc2",
			"out": "patched",
			"ok":  true,
			"meta": map[string]any{
				"kind": "file_change", "title": "FileChange", "subtitle": "ui/src/components/Panel.vue",
			},
		},
	})

	history, err := manager.History(context.Background(), session.ID, 20, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if len(history.Events) != 1 {
		t.Fatalf("expected 1 projected file_change event, got %d", len(history.Events))
	}
	if got := eventToolKind(history.Events[0]); got != "file_change" {
		t.Fatalf("expected file_change kind, got %q", got)
	}
	groupMeta := decodeRawObject(decodeRawObject(history.Events[0].Payload["meta"])["commandGroup"])
	if got := int(numberValue(groupMeta["count"])); got != 2 {
		t.Fatalf("expected grouped count 2, got %d", got)
	}
	if got := stringValue(decodeRawObject(history.Events[0].Payload["meta"])["subtitle"]); got != "ui/src/components/Panel.vue" {
		t.Fatalf("expected latest file path summary, got %q", got)
	}
}

func TestHistorySeparatesDifferentCompactToolKinds(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Mixed Compact Tools", 1000)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd1_st",
		Seq:       1,
		Type:      "tool_st",
		Timestamp: time.UnixMilli(1_000),
		Payload: map[string]any{
			"tid":  "cmd1",
			"name": "CommandExecution",
			"kind": "command_execution",
			"in":   map[string]any{"command": "pwd"},
			"meta": map[string]any{"kind": "command_execution", "title": "CommandExecution", "subtitle": "pwd"},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_cmd1_end",
		Seq:       2,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(2_000),
		Payload: map[string]any{
			"tid": "cmd1",
			"out": "pwd output",
			"ok":  true,
			"meta": map[string]any{
				"kind": "command_execution", "title": "CommandExecution", "subtitle": "pwd",
			},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_mcp_st",
		Seq:       3,
		Type:      "tool_st",
		Timestamp: time.UnixMilli(3_000),
		Payload: map[string]any{
			"tid":  "mcp1",
			"name": "McpToolCall",
			"kind": "mcp_tool_call",
			"in": map[string]any{
				"tool_name": "fetch",
				"arguments": map[string]any{"url": "http://127.0.0.1:3007"},
			},
			"meta": map[string]any{"kind": "mcp_tool_call", "title": "McpToolCall", "subtitle": "fetch"},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_mcp_end",
		Seq:       4,
		Type:      "tool_end",
		Timestamp: time.UnixMilli(4_000),
		Payload: map[string]any{
			"tid": "mcp1",
			"out": "ok",
			"ok":  true,
			"meta": map[string]any{
				"kind": "mcp_tool_call", "title": "McpToolCall", "subtitle": "fetch",
			},
		},
	})

	history, err := manager.History(context.Background(), session.ID, 20, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if len(history.Events) != 2 {
		t.Fatalf("expected 2 projected events, got %d", len(history.Events))
	}
	if got := eventToolKind(history.Events[0]); got != "command_execution" {
		t.Fatalf("expected first kind command_execution, got %q", got)
	}
	if got := eventToolKind(history.Events[1]); got != "mcp_tool_call" {
		t.Fatalf("expected second kind mcp_tool_call, got %q", got)
	}
}

func TestCodexToolResultUsesCamelCaseAggregatedOutput(t *testing.T) {
	got := codexToolResult(map[string]any{
		"type":             "commandExecution",
		"aggregatedOutput": "const styles = {}",
	})
	if got != "const styles = {}" {
		t.Fatalf("expected camelCase aggregatedOutput to be used, got %q", got)
	}
}

func TestTruncateToolOutputKeepsPlanText(t *testing.T) {
	planText := testLongPlanText()
	if got := truncateToolOutput("plan", planText); got != planText {
		t.Fatalf("expected full plan text to be preserved, got length %d want %d", len(got), len(planText))
	}
}

func TestTruncateToolOutputTruncatesNonPlanSafely(t *testing.T) {
	output := strings.Repeat("计划步骤保持完整", 600)

	got := truncateToolOutput("commandExecution", output)
	if got == output {
		t.Fatal("expected non-plan output to be truncated")
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected truncated output suffix, got %q", got[len(got)-min(len(got), 12):])
	}
	if !utf8.ValidString(got) {
		t.Fatalf("expected truncated output to remain valid UTF-8, got %q", got)
	}
	if strings.ContainsRune(got, utf8.RuneError) {
		t.Fatalf("expected truncated output to avoid replacement rune, got %q", got)
	}
}

func TestHandleCodexAppServerItemCompletedPreservesFullPlanOutput(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Long Plan App Server", 1000)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	run := &activeRun{
		sessionID:          session.ID,
		runID:              "run_plan_app_server",
		assistantMessageID: "msg_plan_app_server",
		assistantDeltaSeen: make(map[string]bool),
	}
	planText := testLongPlanText()
	params := []byte(fmt.Sprintf(`{"item":{"type":"plan","id":"plan_test","text":%q}}`, planText))

	manager.handleCodexAppServerItemCompleted(*session, run, params)

	history, err := manager.History(context.Background(), session.ID, 20, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	event, ok := historyToolEventByKind(history.Events, "plan")
	if !ok {
		t.Fatalf("expected plan tool history, got %#v", history.Events)
	}
	if got := eventToolOutput(event); got != planText {
		t.Fatalf("expected app-server plan output to stay intact, got length %d want %d", len(got), len(planText))
	}
}

func TestHandleCodexEventPreservesFullPlanOutput(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Long Plan Legacy", 1000)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	run := &activeRun{
		sessionID:          session.ID,
		runID:              "run_plan_legacy",
		assistantMessageID: "msg_plan_legacy",
		assistantDeltaSeen: make(map[string]bool),
	}
	planText := testLongPlanText()

	manager.handleCodexEvent(*session, run, map[string]any{
		"type": "item.completed",
		"item": map[string]any{
			"type": "plan",
			"id":   "plan_test",
			"text": planText,
		},
	})

	history, err := manager.History(context.Background(), session.ID, 20, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	event, ok := historyToolEventByKind(history.Events, "plan")
	if !ok {
		t.Fatalf("expected plan tool history, got %#v", history.Events)
	}
	if got := eventToolOutput(event); got != planText {
		t.Fatalf("expected legacy plan output to stay intact, got length %d want %d", len(got), len(planText))
	}
}

func TestHandleCodexAppServerUsageDefaultsContextEstimateToCumulativeTotal(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Cumulative Context Estimate", 1000)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	run := &activeRun{
		sessionID:          session.ID,
		runID:              "run_usage_only",
		assistantDeltaSeen: make(map[string]bool),
	}
	manager.handleCodexAppServerUsage(
		*session,
		run,
		[]byte(`{"tokenUsage":{"total":{"inputTokens":120,"cachedInputTokens":30,"outputTokens":10}}}`),
	)

	record, err := manager.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	summary := manager.mapSessionSummary(record)
	if summary.ContextEstimateMode != ContextEstimateModeCumulativeTotal {
		t.Fatalf("expected context estimate mode %q, got %q", ContextEstimateModeCumulativeTotal, summary.ContextEstimateMode)
	}
	if summary.ContextEstimate.UsedTokens != 160 {
		t.Fatalf("expected usedTokens 160, got %d", summary.ContextEstimate.UsedTokens)
	}
	if summary.ContextEstimate.InputTokens != 120 || summary.ContextEstimate.CachedInputTokens != 30 || summary.ContextEstimate.OutputTokens != 10 {
		t.Fatalf("unexpected context estimate: %#v", summary.ContextEstimate)
	}
}

func TestHandleCodexAppServerContextCompactionResetsContextEstimateBaseline(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Context Compaction", 1000)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	run := &activeRun{
		sessionID:          session.ID,
		runID:              "run_context_compaction",
		assistantMessageID: "msg_context_compaction",
		assistantDeltaSeen: make(map[string]bool),
	}

	manager.handleCodexAppServerUsage(
		*session,
		run,
		[]byte(`{"tokenUsage":{"total":{"inputTokens":120,"cachedInputTokens":30,"outputTokens":10}}}`),
	)
	manager.handleCodexAppServerItemCompleted(
		*session,
		run,
		[]byte(`{"item":{"type":"contextCompaction","id":"compact_test","status":"completed","summary":["Compacted previous messages into a summary."]}}`),
	)
	manager.handleCodexAppServerUsage(
		*session,
		run,
		[]byte(`{"tokenUsage":{"total":{"inputTokens":150,"cachedInputTokens":35,"outputTokens":12}}}`),
	)

	record, err := manager.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	summary := manager.mapSessionSummary(record)
	if summary.ContextEstimateMode != ContextEstimateModeSinceCompaction {
		t.Fatalf("expected context estimate mode %q, got %q", ContextEstimateModeSinceCompaction, summary.ContextEstimateMode)
	}
	if summary.LastContextCompactionAt == nil {
		t.Fatal("expected lastContextCompactionAt to be recorded")
	}
	if summary.ContextEstimate.InputTokens != 30 || summary.ContextEstimate.CachedInputTokens != 5 || summary.ContextEstimate.OutputTokens != 2 {
		t.Fatalf("unexpected context estimate after compaction: %#v", summary.ContextEstimate)
	}
	if summary.ContextEstimate.UsedTokens != 37 {
		t.Fatalf("expected usedTokens 37 after compaction, got %d", summary.ContextEstimate.UsedTokens)
	}

	snapshot, err := manager.Snapshot(context.Background(), session.ID, 20)
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if len(snapshot.History.Items) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(snapshot.History.Items))
	}
	item := snapshot.History.Items[0]
	if item.Tool == nil || item.Tool.Kind != "context_compaction" {
		t.Fatalf("expected context_compaction tool item, got %#v", item)
	}
	if !strings.Contains(item.Tool.Output, "Compacted previous messages") {
		t.Fatalf("expected compaction output to be preserved, got %q", item.Tool.Output)
	}
}

func initTestDB(t *testing.T) func() {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	if err := model.InitWithDSN(dsn, 0, true); err != nil {
		t.Fatalf("InitWithDSN: %v", err)
	}
	return func() {
		model.DBClose()
	}
}

func seedProject(t *testing.T) *tables.ProjectTable {
	t.Helper()
	project := &tables.ProjectTable{
		Name: "Web Session Test",
		Path: t.TempDir(),
	}
	project.Init()
	if err := model.GetDB().Create(project).Error; err != nil {
		t.Fatalf("seed project failed: %v", err)
	}
	return project
}

func seedWebSession(t *testing.T, projectID, title string, orderIndex float64) *tables.WebSessionTable {
	t.Helper()
	session := &tables.WebSessionTable{
		ProjectID:            projectID,
		OrderIndex:           orderIndex,
		Agent:                string(AgentCodex),
		Title:                title,
		Model:                "gpt-5.4",
		WorkflowMode:         string(WorkflowModeDefault),
		PermissionLevel:      string(PermissionLevelElevated),
		LegacyPermissionMode: "default",
		Cwd:                  t.TempDir(),
		Status:               string(StatusIdle),
		ActivityAt:           time.Now(),
	}
	session.Init()
	if err := model.GetDB().Create(session).Error; err != nil {
		t.Fatalf("seed web session failed: %v", err)
	}
	return session
}

func writeFakeCodexCLI(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fake-codex.sh")
	script := `#!/bin/sh
printf '%s\n' '{"type":"thread.started","thread_id":"thread_test"}'
printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"done"}}'
printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}'
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex cli failed: %v", err)
	}
	return path
}

func writeFakeCodexAppServerCLI(t *testing.T, mode string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fake-codex-app-server.js")
	script := fmt.Sprintf(`#!/usr/bin/env node
const readline = require('readline');
const fs = require('fs');

const mode = %q;
const threadId = 'thread_test';
const turnId = 'turn_test';
const stateFile = __filename + '.state';

function send(message) {
  process.stdout.write(JSON.stringify(message) + '\n');
}

function respondThread(id, explicitThreadId) {
  send({ id, result: { thread: { id: explicitThreadId || threadId } } });
}

function emitReasoning() {
  send({
    method: 'item/started',
    params: {
      item: { type: 'reasoning', id: 'rs_test', summary: [], content: [] },
      threadId,
      turnId,
    },
  });
  send({
    method: 'item/completed',
    params: {
      item: { type: 'reasoning', id: 'rs_test', summary: [], content: [] },
      threadId,
      turnId,
    },
  });
}

function emitPlan() {
  send({
    method: 'item/started',
    params: {
      item: { type: 'plan', id: 'plan_test', text: '## Plan\n- Review the repo\n- Make the change' },
      threadId,
      turnId,
    },
  });
  send({
    method: 'item/completed',
    params: {
      item: { type: 'plan', id: 'plan_test', text: '## Plan\n- Review the repo\n- Make the change' },
      threadId,
      turnId,
    },
  });
}

function finishTurn(text) {
  emitReasoning();
  if (mode === 'plan') {
    emitPlan();
  }
  send({
    method: 'item/started',
    params: {
      item: { type: 'agentMessage', id: 'msg_test', text: '', phase: 'final_answer', memoryCitation: null },
      threadId,
      turnId,
    },
  });
  send({
    method: 'item/agentMessage/delta',
    params: { threadId, turnId, itemId: 'msg_test', delta: text },
  });
  send({
    method: 'item/completed',
    params: {
      item: { type: 'agentMessage', id: 'msg_test', text, phase: 'final_answer', memoryCitation: null },
      threadId,
      turnId,
    },
  });
  send({
    method: 'thread/tokenUsage/updated',
    params: {
      threadId,
      turnId,
      tokenUsage: {
        total: { inputTokens: 5, cachedInputTokens: 0, outputTokens: 3 },
      },
    },
  });
  send({
    method: 'turn/completed',
    params: {
      threadId,
      turn: { id: turnId, items: [], status: 'completed', error: null },
    },
  });
}

function failTurn(message) {
  send({
    method: 'turn/completed',
    params: {
      threadId,
      turn: {
        id: turnId,
        items: [],
        status: 'failed',
        error: { message },
      },
    },
  });
}

function readPersistentTurnCount() {
  try {
    return Number(fs.readFileSync(stateFile, 'utf8').trim()) || 0;
  } catch (error) {
    return 0;
  }
}

function writePersistentTurnCount(value) {
  fs.writeFileSync(stateFile, String(value));
}

let awaiting = null;
const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
let startedTurns = 0;
rl.on('line', line => {
  if (!line.trim()) {
    return;
  }

  const message = JSON.parse(line);
  if (message.method === 'initialize') {
    send({
      id: message.id,
      result: {
        userAgent: 'fake-codex-app-server',
        codexHome: '/tmp/codex',
        platformFamily: 'unix',
        platformOs: 'linux',
      },
    });
    return;
  }

  if (message.method === 'thread/start' || message.method === 'thread/resume') {
    if (mode === 'resume_only' && message.method !== 'thread/resume') {
      send({
        id: message.id,
        error: { message: 'expected thread/resume for existing session' },
      });
      return;
    }
    const resumedThreadId = message.params && typeof message.params.threadId === 'string'
      ? message.params.threadId
      : threadId;
    respondThread(message.id, resumedThreadId);
    return;
  }

  if (message.method === 'turn/start') {
    startedTurns += 1;
    send({
      id: message.id,
      result: {
        turn: { id: turnId, items: [], status: 'inProgress', error: null },
      },
    });

    if (mode === 'basic' || mode === 'resume_only' || mode === 'plan') {
      finishTurn('done');
      return;
    }

    if (mode === 'reconnect_then_success') {
      send({
        method: 'error',
        params: {
          message: 'Reconnecting... 1/5 (unexpected status 502 Bad Gateway: Upstream service temporarily unavailable)',
        },
      });
      finishTurn('done');
      return;
    }

    if (mode === 'reconnect_then_fail') {
      send({
        method: 'error',
        params: {
          message: 'Reconnecting... 1/5 (unexpected status 502 Bad Gateway: Upstream service temporarily unavailable)',
        },
      });
      send({
        method: 'error',
        params: {
          message: 'Reconnecting... 2/5 (unexpected status 502 Bad Gateway: Upstream service temporarily unavailable)',
        },
      });
      failTurn('unexpected status 502 Bad Gateway: Upstream service temporarily unavailable');
      return;
    }

    if (mode === 'auto_retry_then_success') {
      const persistedTurns = readPersistentTurnCount() + 1;
      writePersistentTurnCount(persistedTurns);
      if (persistedTurns === 1) {
        failTurn('unexpected status 502 Bad Gateway: Upstream service temporarily unavailable');
        return;
      }
      finishTurn('done');
      return;
    }

    if (mode === 'user_input') {
      awaiting = 'req_user_1';
      send({
        id: awaiting,
        method: 'item/tool/requestUserInput',
        params: {
          threadId,
          turnId,
          itemId: 'ask_scope',
          questions: [
            {
              id: 'scope',
              header: 'Scope',
              question: 'Which migration scope should be implemented?',
              isOther: false,
              isSecret: false,
              options: [
                { label: 'full migration', description: 'Move all Codex web sessions to app-server.' },
                { label: 'plan only', description: 'Only switch plan mode to the real runtime mode.' },
              ],
            },
          ],
        },
      });
      return;
    }

    if (mode === 'approval') {
      awaiting = 'req_approval_1';
      send({
        id: awaiting,
        method: 'item/fileChange/requestApproval',
        params: {
          threadId,
          turnId,
          itemId: 'write_patch',
          reason: 'Need approval to apply the patch.',
        },
      });
      return;
    }
  }

  if (awaiting && message.id === awaiting) {
    finishTurn(mode === 'user_input' ? 'answered' : 'approved');
    awaiting = null;
  }
});

rl.on('close', () => process.exit(0));
`, mode)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex app-server cli failed: %v", err)
	}
	return path
}

func waitForSessionToSettle(t *testing.T, manager *Manager, sessionID string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !manager.hasActiveRun(sessionID) {
			record, err := manager.GetSession(context.Background(), sessionID)
			if err == nil && (record.Status == string(StatusDone) ||
				record.Status == string(StatusError) ||
				record.Status == string(StatusIdle) ||
				(record.Status == string(StatusRunning) &&
					record.AssistantState == string(AssistantStateWaitingPlanApproval))) {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	record, err := manager.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetSession returned error while waiting: %v", err)
	}
	t.Fatalf("session %s did not settle, status=%s", sessionID, record.Status)
}

func waitForPendingServerRequest(
	t *testing.T,
	manager *Manager,
	sessionID string,
	kind pendingServerRequestKind,
) *pendingServerRequest {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		manager.mu.RLock()
		run := manager.runs[sessionID]
		manager.mu.RUnlock()
		if run != nil {
			if request, ok := run.pendingApprovalRequest(); ok && request.Kind == kind {
				return request
			}
			if request, ok := run.pendingUserInputRequest(); ok && request.Kind == kind {
				return request
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

func historyHasEvent(events []Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func historyHasToolKind(events []Event, kind string) bool {
	for _, event := range events {
		if event.Type != "tool_st" && event.Type != "tool_end" {
			continue
		}
		if value, ok := event.Payload["kind"].(string); ok && value == kind {
			return true
		}
	}
	return false
}

func historyToolEventByKind(events []Event, kind string) (Event, bool) {
	for _, event := range events {
		if event.Type != "tool_st" && event.Type != "tool_end" {
			continue
		}
		if eventToolKind(event) == kind {
			return event, true
		}
	}
	return Event{}, false
}

func testLongPlanText() string {
	return "## Plan\n" + strings.Repeat("- 计划步骤：保持中文内容完整，不要被截断。\n", 240)
}

func appendHistoryEvent(t *testing.T, manager *Manager, sessionID string, event Event) {
	t.Helper()
	manager.mu.Lock()
	if manager.runs[sessionID] == nil {
		manager.runs[sessionID] = &activeRun{
			sessionID:          sessionID,
			done:               make(chan struct{}),
			assistantDeltaSeen: make(map[string]bool),
		}
	}
	manager.mu.Unlock()
	manager.decorateProjectedEvent(sessionID, &event)
	if err := manager.store.appendEvent(sessionID, event); err != nil {
		t.Fatalf("appendEvent returned error: %v", err)
	}
	if _, err := manager.applyEventToHistoryCache(context.Background(), sessionID, event); err != nil {
		t.Fatalf("applyEventToHistoryCache returned error: %v", err)
	}
}
