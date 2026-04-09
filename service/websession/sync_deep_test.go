package websession

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code-kanban/model"
	"code-kanban/model/tables"

	"go.uber.org/zap"
)

func TestParseCodexDeepHistoryCapturesToolsAndTimestamps(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	filePath := writeCodexDeepHistoryTempFile(t, []string{
		`{"timestamp":"2026-04-09T01:00:00Z","type":"session_meta","payload":{"id":"session-1","timestamp":"2026-04-09T01:00:00Z","cwd":"/tmp/test"}}`,
		`{"timestamp":"2026-04-09T01:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"inspect the repo","images":[]}}`,
		`{"timestamp":"2026-04-09T01:00:02Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"pwd\",\"workdir\":\"/tmp/test\"}","call_id":"call_1"}}`,
		`{"timestamp":"2026-04-09T01:00:03Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call_1","output":"Command: /bin/bash -lc pwd\nProcess exited with code 0\nOutput:\n/tmp/test\n"}}`,
		`{"timestamp":"2026-04-09T01:00:04Z","type":"response_item","payload":{"type":"reasoning","summary":["Need to inspect sync paths"],"content":null}}`,
		`{"timestamp":"2026-04-09T01:00:05Z","type":"event_msg","payload":{"type":"agent_message","message":"I found the sync entrypoints."}}`,
	})

	items, err := manager.parseCodexDeepHistory(filePath)
	if err != nil {
		t.Fatalf("parseCodexDeepHistory returned error: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 history items, got %d", len(items))
	}
	if items[0].Kind != "user" || items[0].Text != "inspect the repo" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Tool == nil {
		t.Fatalf("expected tool item at index 1, got %#v", items[1])
	}
	if items[1].Tool.Kind != "command_execution" {
		t.Fatalf("expected command_execution kind, got %q", items[1].Tool.Kind)
	}
	if items[1].Tool.Status != "done" {
		t.Fatalf("expected tool status done, got %q", items[1].Tool.Status)
	}
	if items[1].Timestamp == nil || items[1].Timestamp.Format(time.RFC3339) != "2026-04-09T01:00:02Z" {
		t.Fatalf("expected tool timestamp from function_call, got %#v", items[1].Timestamp)
	}
	if items[1].ObservedAt == nil || items[1].ObservedAt.Format(time.RFC3339) != "2026-04-09T01:00:03Z" {
		t.Fatalf("expected tool observedAt from function_call_output, got %#v", items[1].ObservedAt)
	}
	if items[2].Tool == nil || items[2].Tool.Kind != "reasoning" {
		t.Fatalf("expected reasoning tool at index 2, got %#v", items[2])
	}
	if items[3].Kind != "assistant" || items[3].Text != "I found the sync entrypoints." {
		t.Fatalf("unexpected last item: %#v", items[3])
	}
}

func TestSyncSessionFromLogSourceReplacesCacheAndMarksDeepSync(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Deep Sync Session", 1000)

	nativeSessionID := "session-deep-sync"
	if err := model.GetDB().Model(&tables.WebSessionTable{}).
		Where("id = ?", session.ID).
		Updates(map[string]any{
			"native_session_id": nativeSessionID,
			"cwd":               project.Path,
		}).Error; err != nil {
		t.Fatalf("failed to update web session: %v", err)
	}

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	filePath := writeCodexDeepHistoryTempFile(t, []string{
		`{"timestamp":"2026-04-09T02:00:00Z","type":"session_meta","payload":{"id":"session-deep-sync","timestamp":"2026-04-09T02:00:00Z","cwd":"` + filepath.ToSlash(project.Path) + `"}}`,
		`{"timestamp":"2026-04-09T02:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"run deep sync","images":[]}}`,
		`{"timestamp":"2026-04-09T02:00:02Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"pwd\",\"workdir\":\"` + filepath.ToSlash(project.Path) + `\"}","call_id":"call_sync"}}`,
		`{"timestamp":"2026-04-09T02:00:03Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call_sync","output":"Command: /bin/bash -lc pwd\nOutput:\n` + filepath.ToSlash(project.Path) + `\n"}}`,
		`{"timestamp":"2026-04-09T02:00:04Z","type":"event_msg","payload":{"type":"agent_message","message":"deep sync finished"}}`,
	})
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}

	now := time.Now()
	aiRecord := tables.AISessionTable{
		SessionID:             nativeSessionID,
		Type:                  tables.AISessionTypeCodex,
		ProjectPath:           project.Path,
		FilePath:              filePath,
		Model:                 "gpt-5.4",
		Title:                 "run deep sync",
		SessionStartedAt:      time.Date(2026, 4, 9, 2, 0, 0, 0, time.UTC),
		LastMessageAt:         ptr(time.Date(2026, 4, 9, 2, 0, 4, 0, time.UTC)),
		MessageCount:          1,
		AssistantMessageCount: 1,
		FileModTime:           info.ModTime(),
		FileSize:              info.Size(),
	}
	aiRecord.Init()
	aiRecord.CreatedAt = now
	aiRecord.UpdatedAt = now
	if err := model.GetDB().Create(&aiRecord).Error; err != nil {
		t.Fatalf("failed to seed ai session record: %v", err)
	}

	refreshedSession, err := manager.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}

	snapshot, err := manager.syncSessionFromLogSource(context.Background(), refreshedSession, true, false)
	if err != nil {
		t.Fatalf("syncSessionFromLogSource returned error: %v", err)
	}
	if snapshot.Session.LastSyncMode != SyncModeDeep {
		t.Fatalf("expected last sync mode deep, got %q", snapshot.Session.LastSyncMode)
	}
	if snapshot.Session.ItemCount != 3 {
		t.Fatalf("expected 3 cached items, got %d", snapshot.Session.ItemCount)
	}
	if len(snapshot.History.Items) != 3 {
		t.Fatalf("expected 3 history items, got %d", len(snapshot.History.Items))
	}
	if snapshot.History.Items[1].Tool == nil || snapshot.History.Items[1].Tool.Kind != "command_execution" {
		t.Fatalf("expected cached command_execution tool, got %#v", snapshot.History.Items[1])
	}
}

func TestShouldPreserveExistingHistoryOnFastSync(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	if !manager.shouldPreserveExistingHistoryOnFastSync(tables.WebSessionTable{LastSyncMode: "deep"}) {
		t.Fatal("expected deep-synced cache to be preserved on fast sync")
	}
	if !manager.shouldPreserveExistingHistoryOnFastSync(tables.WebSessionTable{LastEventSeq: 3}) {
		t.Fatal("expected live event cache to be preserved on fast sync")
	}
	if manager.shouldPreserveExistingHistoryOnFastSync(tables.WebSessionTable{}) {
		t.Fatal("expected empty cache to be replaceable on fast sync")
	}
}

func writeCodexDeepHistoryTempFile(t *testing.T, lines []string) string {
	t.Helper()
	filePath := filepath.Join(t.TempDir(), "codex-deep-history.jsonl")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp history: %v", err)
	}
	return filePath
}
