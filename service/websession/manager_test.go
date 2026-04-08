package websession

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code-kanban/model"
	"code-kanban/model/tables"

	"go.uber.org/zap"
)

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
	if !historyHasToolKind(history.Events, "reasoning") {
		t.Fatalf("expected reasoning items to be persisted for optional display, got %#v", history.Events)
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

	if err := manager.respondToUserInput(created.ID, request.ItemID, map[string][]string{
		"scope": {"full migration"},
	}); err != nil {
		t.Fatalf("respondToUserInput returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	history, err := manager.History(context.Background(), created.ID, 200, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if !historyHasEvent(history.Events, "user_input_req") {
		t.Fatalf("expected user_input_req event, got %#v", history.Events)
	}
	if !historyHasEvent(history.Events, "user_input_res") {
		t.Fatalf("expected user_input_res event, got %#v", history.Events)
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

	if err := manager.respondToApproval(created.ID, "approve"); err != nil {
		t.Fatalf("respondToApproval returned error: %v", err)
	}

	waitForSessionToSettle(t, manager, created.ID)

	history, err := manager.History(context.Background(), created.ID, 200, nil)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if !historyHasEvent(history.Events, "approval_req") {
		t.Fatalf("expected approval_req event, got %#v", history.Events)
	}
	if !historyHasEvent(history.Events, "approval_res") {
		t.Fatalf("expected approval_res event, got %#v", history.Events)
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
			"ok":  true,
			"meta": map[string]any{
				"kind":  "command_execution",
				"title": "CommandExecution",
			},
		},
	})
	appendHistoryEvent(t, manager, session.ID, Event{
		ID:        "evt_note",
		Seq:       5,
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
	if grouped.Seq != 4 {
		t.Fatalf("expected grouped event seq 4, got %d", grouped.Seq)
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

func TestCodexToolResultUsesCamelCaseAggregatedOutput(t *testing.T) {
	got := codexToolResult(map[string]any{
		"type":             "commandExecution",
		"aggregatedOutput": "const styles = {}",
	})
	if got != "const styles = {}" {
		t.Fatalf("expected camelCase aggregatedOutput to be used, got %q", got)
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

const mode = %q;
const threadId = 'thread_test';
const turnId = 'turn_test';

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

function finishTurn(text) {
  emitReasoning();
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

let awaiting = null;
const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
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
    send({
      id: message.id,
      result: {
        turn: { id: turnId, items: [], status: 'inProgress', error: null },
      },
    });

    if (mode === 'basic' || mode === 'resume_only') {
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
			if err == nil && (record.Status == string(StatusDone) || record.Status == string(StatusError) || record.Status == string(StatusIdle)) {
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

func appendHistoryEvent(t *testing.T, manager *Manager, sessionID string, event Event) {
	t.Helper()
	if err := manager.store.appendEvent(sessionID, event); err != nil {
		t.Fatalf("appendEvent returned error: %v", err)
	}
}
