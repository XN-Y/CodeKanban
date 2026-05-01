package websession

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code-kanban/model"
	"code-kanban/model/tables"

	"go.uber.org/zap"
)

func TestScheduleInputIncludesScheduledInputsInSnapshot(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	manager, err := NewManager(Config{
		DataDir: t.TempDir(),
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

	scheduledFor := time.Now().Add(30 * time.Minute).Round(time.Millisecond)
	item, err := manager.ScheduleInput(
		context.Background(),
		created.ID,
		"Follow up later",
		nil,
		ScheduledInputModeSend,
		scheduledFor,
	)
	if err != nil {
		t.Fatalf("ScheduleInput returned error: %v", err)
	}
	t.Cleanup(func() {
		manager.cancelScheduledInputTimersForSession(created.ID)
	})

	snapshot, err := manager.Snapshot(context.Background(), created.ID, DefaultHistoryWindow)
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if len(snapshot.ScheduledInputs) != 1 {
		t.Fatalf("expected 1 scheduled input, got %#v", snapshot.ScheduledInputs)
	}
	got := snapshot.ScheduledInputs[0]
	if got.ID != item.ID {
		t.Fatalf("expected scheduled input id %q, got %#v", item.ID, got)
	}
	if got.Mode != ScheduledInputModeSend {
		t.Fatalf("expected scheduled input mode %q, got %#v", ScheduledInputModeSend, got.Mode)
	}
	if got.Status != ScheduledInputStatusScheduled {
		t.Fatalf("expected scheduled input status %q, got %#v", ScheduledInputStatusScheduled, got.Status)
	}
	if got.Text != "Follow up later" {
		t.Fatalf("expected scheduled input text %q, got %#v", "Follow up later", got.Text)
	}
	if !got.ScheduledFor.Equal(scheduledFor) {
		t.Fatalf("expected scheduled time %v, got %v", scheduledFor, got.ScheduledFor)
	}
}

func TestScheduledInputDispatchesAtDueTime(t *testing.T) {
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

	if _, err := manager.ScheduleInput(
		context.Background(),
		created.ID,
		"Run this later",
		nil,
		ScheduledInputModeSend,
		time.Now().Add(60*time.Millisecond),
	); err != nil {
		t.Fatalf("ScheduleInput returned error: %v", err)
	}

	waitForUserMessageCount(t, manager, created.ID, 1)
	waitForSessionToSettle(t, manager, created.ID)
	waitForScheduledInputCount(t, manager, created.ID, 0)

	rawEvents, err := manager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error: %v", err)
	}
	if got := strings.Join(userMessageTexts(rawEvents), "|"); got != "Run this later" {
		t.Fatalf("expected scheduled message to dispatch once, got %#v", got)
	}
}

func TestScheduledSendFollowsNormalSendBehaviorWhenRunActive(t *testing.T) {
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

	if err := manager.SendMessage(context.Background(), created.ID, "first", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	request := waitForPendingServerRequest(t, manager, created.ID, pendingServerRequestFileChangeApproval)
	if request == nil {
		t.Fatal("expected pending approval request for the first run")
	}

	if _, err := manager.ScheduleInput(
		context.Background(),
		created.ID,
		"Next after timer",
		nil,
		ScheduledInputModeSend,
		time.Now().Add(60*time.Millisecond),
	); err != nil {
		t.Fatalf("ScheduleInput returned error: %v", err)
	}

	waitForPendingInputCount(t, manager, created.ID, 1)
	pending := manager.pendingInputsSnapshot(created.ID)
	if len(pending) != 1 || pending[0].Mode != PendingInputModeRedirect || pending[0].Text != "Next after timer" {
		t.Fatalf("expected scheduled send to follow normal send pending behavior, got %#v", pending)
	}
	waitForScheduledInputCount(t, manager, created.ID, 0)

	rawEvents, err := manager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error: %v", err)
	}
	if got := strings.Join(userMessageTexts(rawEvents), "|"); got != "first" {
		t.Fatalf("expected queued dispatch not to append a second user message yet, got %#v", got)
	}

	if err := manager.respondToApproval(created.ID, "approve"); err != nil {
		t.Fatalf("respondToApproval returned error: %v", err)
	}

	waitForUserMessageCount(t, manager, created.ID, 2)

	rawEvents, err = manager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error after flush: %v", err)
	}
	if got := strings.Join(userMessageTexts(rawEvents), "|"); got != "first|Next after timer" {
		t.Fatalf("expected queued scheduled message to flush after approval, got %#v", got)
	}
}

func TestScheduledInterruptAbortsActiveRunBeforeSending(t *testing.T) {
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

	if err := manager.SendMessage(context.Background(), created.ID, "first", nil); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if request := waitForPendingServerRequest(t, manager, created.ID, pendingServerRequestFileChangeApproval); request == nil {
		t.Fatal("expected pending approval request for the first run")
	}

	if _, err := manager.ScheduleInput(
		context.Background(),
		created.ID,
		"Interrupt now",
		nil,
		ScheduledInputModeInterrupt,
		time.Now().Add(60*time.Millisecond),
	); err != nil {
		t.Fatalf("ScheduleInput returned error: %v", err)
	}

	waitForUserMessageCount(t, manager, created.ID, 2)
	waitForScheduledInputCount(t, manager, created.ID, 0)
	if pending := manager.pendingInputsSnapshot(created.ID); len(pending) != 0 {
		t.Fatalf("expected interrupt scheduled input not to create pending input, got %#v", pending)
	}

	rawEvents, err := manager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error: %v", err)
	}
	if got := strings.Join(userMessageTexts(rawEvents), "|"); got != "first|Interrupt now" {
		t.Fatalf("expected interrupt scheduled message to send after abort, got %#v", got)
	}

	if request := waitForPendingServerRequest(t, manager, created.ID, pendingServerRequestFileChangeApproval); request == nil {
		t.Fatal("expected pending approval request for the interrupted follow-up run")
	}
	if err := manager.respondToApproval(created.ID, "approve"); err != nil {
		t.Fatalf("respondToApproval returned error: %v", err)
	}
	waitForSessionToSettle(t, manager, created.ID)
}

func TestNewManagerRecoversPendingScheduledInputs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scheduled-recovery.db")
	if err := model.InitWithDSN(dbPath, 0, true); err != nil {
		t.Fatalf("InitWithDSN returned error: %v", err)
	}

	project := seedProject(t)
	dataDir := t.TempDir()
	firstManager, err := NewManager(Config{
		DataDir:   dataDir,
		CodexPath: writeFakeCodexAppServerCLI(t, "basic"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("first NewManager returned error: %v", err)
	}

	created, err := firstManager.CreateSession(context.Background(), CreateParams{
		ProjectID: project.ID,
		Agent:     AgentCodex,
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	item, err := firstManager.ScheduleInput(
		context.Background(),
		created.ID,
		"Recovered after restart",
		nil,
		ScheduledInputModeSend,
		time.Now().Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("ScheduleInput returned error: %v", err)
	}

	firstManager.cancelScheduledInputTimersForSession(created.ID)
	dispatchAt := time.Now().Add(80 * time.Millisecond)
	if err := model.GetDB().
		Model(&tables.WebSessionScheduledInputTable{}).
		Where("id = ?", item.ID).
		Update("scheduled_for", dispatchAt).Error; err != nil {
		t.Fatalf("failed to update scheduled_for: %v", err)
	}
	model.DBClose()

	if err := model.InitWithDSN(dbPath, 0, true); err != nil {
		t.Fatalf("reopen InitWithDSN returned error: %v", err)
	}
	defer model.DBClose()

	secondManager, err := NewManager(Config{
		DataDir:   dataDir,
		CodexPath: writeFakeCodexAppServerCLI(t, "basic"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("second NewManager returned error: %v", err)
	}

	waitForUserMessageCount(t, secondManager, created.ID, 1)
	waitForSessionToSettle(t, secondManager, created.ID)
	waitForScheduledInputCount(t, secondManager, created.ID, 0)

	rawEvents, err := secondManager.store.readEvents(created.ID)
	if err != nil {
		t.Fatalf("readEvents returned error after recovery: %v", err)
	}
	if got := strings.Join(userMessageTexts(rawEvents), "|"); got != "Recovered after restart" {
		t.Fatalf("expected recovered scheduled message to dispatch after restart, got %#v", got)
	}
}

func waitForScheduledInputCount(t *testing.T, manager *Manager, sessionID string, count int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		items, err := manager.scheduledInputsSnapshot(context.Background(), sessionID)
		if err == nil && len(items) == count {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	items, err := manager.scheduledInputsSnapshot(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("scheduledInputsSnapshot returned error: %v", err)
	}
	t.Fatalf("expected %d scheduled inputs, got %#v", count, items)
}

func waitForPendingInputCount(t *testing.T, manager *Manager, sessionID string, count int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(manager.pendingInputsSnapshot(sessionID)) == count {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected %d pending inputs, got %#v", count, manager.pendingInputsSnapshot(sessionID))
}
