package terminal

import (
	"context"
	"testing"
	"time"

	"code-kanban/model"
	"code-kanban/model/tables"
	"code-kanban/utils/ai_assistant2"
	assistanttypes "code-kanban/utils/ai_assistant2/types"
)

func TestHandleRecentInputDoesNotAutoCreateTask(t *testing.T) {
	cleanup := initTerminalTaskTestDB(t)
	defer cleanup()

	ctx := context.Background()
	project := seedTerminalTaskProject(t)
	worktree := seedTerminalTaskWorktree(t, project.ID, "feature/no-auto-create")

	session := &Session{
		id:         "sess-no-auto-create",
		projectID:  project.ID,
		worktreeID: worktree.ID,
		title:      "Terminal 1",
	}

	session.handleRecentInput(ai_assistant2.StateChangeEvent{
		RecentInput: "Implement terminal decoupling",
		Timestamp:   time.Now(),
	})

	taskSvc := &model.TaskService{}
	tasks, total, err := taskSvc.ListTasks(ctx, &model.ListTasksRequest{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("ListTasks returned error: %v", err)
	}
	if total != 0 || len(tasks) != 0 {
		t.Fatalf("expected no auto-created tasks, got total=%d len=%d", total, len(tasks))
	}
	if got := session.TaskID(); got != "" {
		t.Fatalf("expected session to remain unlinked, got taskId=%q", got)
	}
	if got := session.LastRecentInput(); got != "Implement terminal decoupling" {
		t.Fatalf("expected last recent input to be recorded, got %q", got)
	}
}

func TestHandleRecentInputDoesNotUpdateLinkedTaskContent(t *testing.T) {
	cleanup := initTerminalTaskTestDB(t)
	defer cleanup()

	ctx := context.Background()
	project := seedTerminalTaskProject(t)
	worktree := seedTerminalTaskWorktree(t, project.ID, "feature/no-task-writeback")
	task := createTerminalTask(t, ctx, project.ID, worktree.ID, "todo", "Original description")

	session := &Session{
		id:               "sess-linked-content",
		projectID:        project.ID,
		worktreeID:       worktree.ID,
		title:            "Terminal 2",
		associatedTaskID: task.ID,
	}

	session.handleRecentInput(ai_assistant2.StateChangeEvent{
		RecentInput: "Investigate flaky terminal close path",
		Timestamp:   time.Now(),
	})

	taskSvc := &model.TaskService{}
	updated, err := taskSvc.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask returned error: %v", err)
	}
	if updated.Status != "todo" {
		t.Fatalf("expected linked task status to stay todo, got %s", updated.Status)
	}
	if updated.Description != "Original description" {
		t.Fatalf("expected linked task description to stay unchanged, got %q", updated.Description)
	}

	commentSvc := model.NewTaskCommentService()
	comments, err := commentSvc.ListComments(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListComments returned error: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("expected no task comments to be appended, got %d", len(comments))
	}
}

func TestHandleStateChangeFromTrackerDoesNotUpdateTaskStatus(t *testing.T) {
	cleanup := initTerminalTaskTestDB(t)
	defer cleanup()

	ctx := context.Background()
	project := seedTerminalTaskProject(t)
	worktree := seedTerminalTaskWorktree(t, project.ID, "feature/no-status-sync")

	testCases := []struct {
		name          string
		initialStatus string
		previousState assistanttypes.State
		nextState     assistanttypes.State
	}{
		{
			name:          "working start leaves todo untouched",
			initialStatus: "todo",
			previousState: assistanttypes.StateWaitingInput,
			nextState:     assistanttypes.StateWorking,
		},
		{
			name:          "working completion leaves in progress untouched",
			initialStatus: "in_progress",
			previousState: assistanttypes.StateWorking,
			nextState:     assistanttypes.StateWaitingInput,
		},
	}

	taskSvc := &model.TaskService{}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task := createTerminalTask(t, ctx, project.ID, worktree.ID, tc.initialStatus, "Stable task")
			session := &Session{
				id:               "sess-state-" + tc.name,
				projectID:        project.ID,
				worktreeID:       worktree.ID,
				title:            "Terminal 3",
				associatedTaskID: task.ID,
			}

			session.handleStateChangeFromTracker(ai_assistant2.StateChangeEvent{
				PreviousState: tc.previousState,
				State:         tc.nextState,
				Timestamp:     time.Now(),
			})

			updated, err := taskSvc.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask returned error: %v", err)
			}
			if updated.Status != tc.initialStatus {
				t.Fatalf("expected task status to remain %s, got %s", tc.initialStatus, updated.Status)
			}
		})
	}
}

func initTerminalTaskTestDB(t *testing.T) func() {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	if err := model.InitWithDSN(dsn, 0, true); err != nil {
		t.Fatalf("InitWithDSN failed: %v", err)
	}

	return func() {
		model.DBClose()
	}
}

func seedTerminalTaskProject(t *testing.T) *tables.ProjectTable {
	t.Helper()

	project := &tables.ProjectTable{
		Name:          "Terminal Test Project",
		Path:          t.TempDir(),
		DefaultBranch: "main",
	}
	if err := model.GetDB().Create(project).Error; err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}
	return project
}

func seedTerminalTaskWorktree(t *testing.T, projectID, branch string) *tables.WorktreeTable {
	t.Helper()

	worktree := &tables.WorktreeTable{
		ProjectID:  projectID,
		BranchName: branch,
		Path:       t.TempDir(),
		IsMain:     false,
		IsBare:     false,
	}
	if err := model.GetDB().Create(worktree).Error; err != nil {
		t.Fatalf("failed to seed worktree: %v", err)
	}
	return worktree
}

func createTerminalTask(
	t *testing.T,
	ctx context.Context,
	projectID string,
	worktreeID string,
	status string,
	description string,
) *tables.TaskTable {
	t.Helper()

	taskSvc := &model.TaskService{}
	task, err := taskSvc.CreateTask(ctx, &model.CreateTaskRequest{
		ProjectID:   projectID,
		WorktreeID:  &worktreeID,
		Title:       "Linked task",
		Description: description,
		Status:      status,
	})
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	return task
}
