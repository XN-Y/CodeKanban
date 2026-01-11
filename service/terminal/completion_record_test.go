package terminal

import (
	"testing"
	"time"

	"code-kanban/utils/ai_assistant2"
)

func TestRecordManager_AddAndGetCompletions(t *testing.T) {
	rm := NewRecordManager()

	record := &CompletionRecord{
		ID:            "rec1",
		SessionID:     "sess1",
		ProjectID:     "proj1",
		Title:         "Test Session",
		LastUserInput: "help me fix the bug",
		State:         "completed",
		CompletedAt:   time.Now(),
	}

	rm.AddCompletion(record)

	completions := rm.GetCompletions()
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}
	if completions[0].LastUserInput != "help me fix the bug" {
		t.Fatalf("expected LastUserInput 'help me fix the bug', got %q", completions[0].LastUserInput)
	}
}

func TestRecordManager_UpdateCompletionBySession(t *testing.T) {
	rm := NewRecordManager()

	record := &CompletionRecord{
		ID:            "rec1",
		SessionID:     "sess1",
		ProjectID:     "proj1",
		Title:         "Test Session",
		LastUserInput: "initial input",
		State:         "completed",
		CompletedAt:   time.Now(),
	}

	rm.AddCompletion(record)

	// 更新状态和用户输入
	updated := rm.UpdateCompletionBySession("sess1", "working", "new user input")
	if !updated {
		t.Fatal("expected UpdateCompletionBySession to return true")
	}

	completions := rm.GetCompletions()
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}
	if completions[0].State != "working" {
		t.Fatalf("expected state 'working', got %q", completions[0].State)
	}
	if completions[0].LastUserInput != "new user input" {
		t.Fatalf("expected LastUserInput 'new user input', got %q", completions[0].LastUserInput)
	}
}

func TestRecordManager_UpdateCompletionBySession_EmptyInput(t *testing.T) {
	rm := NewRecordManager()

	record := &CompletionRecord{
		ID:            "rec1",
		SessionID:     "sess1",
		ProjectID:     "proj1",
		Title:         "Test Session",
		LastUserInput: "original input",
		State:         "completed",
		CompletedAt:   time.Now(),
	}

	rm.AddCompletion(record)

	// 空输入不应该覆盖原有的 LastUserInput
	updated := rm.UpdateCompletionBySession("sess1", "working", "")
	if !updated {
		t.Fatal("expected UpdateCompletionBySession to return true")
	}

	completions := rm.GetCompletions()
	if completions[0].LastUserInput != "original input" {
		t.Fatalf("expected LastUserInput to remain 'original input', got %q", completions[0].LastUserInput)
	}
}

func TestRecordManager_UpdateCompletionBySession_NotFound(t *testing.T) {
	rm := NewRecordManager()

	updated := rm.UpdateCompletionBySession("nonexistent", "working", "input")
	if updated {
		t.Fatal("expected UpdateCompletionBySession to return false for nonexistent session")
	}
}

func TestRecordManager_UpdateCompletionBySession_UpdatesTimestamp(t *testing.T) {
	rm := NewRecordManager()

	// 创建两条记录，sess2 比 sess1 新
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now().Add(-time.Minute) // sess2 比 sess1 新，但比当前时间早一分钟

	rm.AddCompletion(&CompletionRecord{
		ID:          "rec1",
		SessionID:   "sess1",
		ProjectID:   "proj1",
		State:       "completed",
		CompletedAt: oldTime,
	})
	rm.AddCompletion(&CompletionRecord{
		ID:          "rec2",
		SessionID:   "sess2",
		ProjectID:   "proj1",
		State:       "completed",
		CompletedAt: newTime,
	})

	// 验证初始排序：sess2 在前（更新）
	completions := rm.GetCompletions()
	if completions[0].SessionID != "sess2" {
		t.Fatalf("expected sess2 to be first initially, got %s", completions[0].SessionID)
	}

	// 记录更新前 sess1 的时间戳
	oldSess1Time := completions[1].CompletedAt

	// 更新 sess1 的状态，应该刷新时间戳使其排到最前
	rm.UpdateCompletionBySession("sess1", "working", "")

	// 验证更新后排序：sess1 应该在前（因为时间戳被更新为 now）
	completions = rm.GetCompletions()
	if completions[0].SessionID != "sess1" {
		t.Fatalf("expected sess1 to be first after update, got %s", completions[0].SessionID)
	}
	if completions[0].State != "working" {
		t.Fatalf("expected state 'working', got %s", completions[0].State)
	}
	// 验证时间戳确实被更新了
	if !completions[0].CompletedAt.After(oldSess1Time) {
		t.Fatalf("expected CompletedAt to be updated, old=%v, new=%v", oldSess1Time, completions[0].CompletedAt)
	}
}

func TestRecordManager_DismissCompletion(t *testing.T) {
	rm := NewRecordManager()

	record := &CompletionRecord{
		ID:            "rec1",
		SessionID:     "sess1",
		ProjectID:     "proj1",
		Title:         "Test Session",
		LastUserInput: "test input",
		State:         "completed",
		CompletedAt:   time.Now(),
	}

	rm.AddCompletion(record)

	// Dismiss 后不应该出现在 GetCompletions 结果中
	rm.DismissCompletion("rec1")

	completions := rm.GetCompletions()
	if len(completions) != 0 {
		t.Fatalf("expected 0 completions after dismiss, got %d", len(completions))
	}
}

func TestRecordManager_MarkCompletionRead(t *testing.T) {
	rm := NewRecordManager()

	rm.AddCompletion(&CompletionRecord{
		ID:          "rec1",
		SessionID:   "sess1",
		ProjectID:   "proj1",
		Title:       "Test Session",
		State:       "completed",
		CompletedAt: time.Now(),
	})

	if ok := rm.MarkCompletionRead("rec1"); !ok {
		t.Fatal("expected MarkCompletionRead to return true")
	}

	completions := rm.GetCompletions()
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}
	if completions[0].ReadAt == nil {
		t.Fatal("expected ReadAt to be set")
	}
}

func TestRecordManager_UpdateCompletionBySession_ClearsReadAtWhenWorking(t *testing.T) {
	rm := NewRecordManager()

	rm.AddCompletion(&CompletionRecord{
		ID:          "rec1",
		SessionID:   "sess1",
		ProjectID:   "proj1",
		Title:       "Test Session",
		State:       "completed",
		CompletedAt: time.Now(),
	})

	rm.MarkCompletionRead("rec1")
	completions := rm.GetCompletions()
	if completions[0].ReadAt == nil {
		t.Fatal("expected ReadAt to be set before update")
	}

	rm.UpdateCompletionBySession("sess1", "working", "")
	completions = rm.GetCompletions()
	if completions[0].ReadAt != nil {
		t.Fatal("expected ReadAt to be cleared when state becomes working")
	}
}

func TestRecordManager_ClearSessionRecords(t *testing.T) {
	rm := NewRecordManager()

	rm.AddCompletion(&CompletionRecord{
		ID:        "rec1",
		SessionID: "sess1",
		ProjectID: "proj1",
	})
	rm.AddCompletion(&CompletionRecord{
		ID:        "rec2",
		SessionID: "sess1",
		ProjectID: "proj1",
	})
	rm.AddCompletion(&CompletionRecord{
		ID:        "rec3",
		SessionID: "sess2",
		ProjectID: "proj1",
	})

	rm.ClearSessionRecords("sess1")

	completions := rm.GetCompletions()
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion after clearing sess1, got %d", len(completions))
	}
	if completions[0].SessionID != "sess2" {
		t.Fatalf("expected remaining completion to be from sess2")
	}
}

func TestRecordManager_ApprovalRecords(t *testing.T) {
	rm := NewRecordManager()

	record := &ApprovalRecord{
		ID:          "apr1",
		SessionID:   "sess1",
		ProjectID:   "proj1",
		Title:       "Test Session",
		RequestedAt: time.Now(),
	}

	rm.AddApproval(record)

	approvals := rm.GetApprovals()
	if len(approvals) != 1 {
		t.Fatalf("expected 1 approval, got %d", len(approvals))
	}

	rm.DismissApproval("apr1")
	approvals = rm.GetApprovals()
	if len(approvals) != 0 {
		t.Fatalf("expected 0 approvals after dismiss, got %d", len(approvals))
	}
}

func TestRecordManager_ClearCompletionsBySession(t *testing.T) {
	rm := NewRecordManager()

	rm.AddCompletion(&CompletionRecord{
		ID:            "rec1",
		SessionID:     "sess1",
		LastUserInput: "input1",
	})

	// 清除后再添加新记录
	rm.ClearCompletionsBySession("sess1")
	rm.AddCompletion(&CompletionRecord{
		ID:            "rec2",
		SessionID:     "sess1",
		LastUserInput: "input2",
	})

	completions := rm.GetCompletions()
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}
	if completions[0].ID != "rec2" {
		t.Fatalf("expected new record rec2, got %s", completions[0].ID)
	}
	if completions[0].LastUserInput != "input2" {
		t.Fatalf("expected LastUserInput 'input2', got %q", completions[0].LastUserInput)
	}
}

func TestCompletionRecord_WithAssistantInfo(t *testing.T) {
	rm := NewRecordManager()

	assistant := &ai_assistant2.AIAssistantInfo{
		Name:        "claude",
		DisplayName: "Claude",
		Type:        "claude-code",
		State:       "working",
	}

	record := &CompletionRecord{
		ID:            "rec1",
		SessionID:     "sess1",
		ProjectID:     "proj1",
		Title:         "Test Session",
		Assistant:     assistant,
		LastUserInput: "write a function",
		State:         "working",
		CompletedAt:   time.Now(),
	}

	rm.AddCompletion(record)

	completions := rm.GetCompletions()
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}
	if completions[0].Assistant == nil {
		t.Fatal("expected Assistant to be set")
	}
	if completions[0].Assistant.DisplayName != "Claude" {
		t.Fatalf("expected Assistant.DisplayName 'Claude', got %q", completions[0].Assistant.DisplayName)
	}
}
