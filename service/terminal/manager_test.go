package terminal

import (
	"testing"
	"time"

	"code-kanban/utils/ai_assistant2"
)

func TestMonitorAssistantRecordsClearsApprovalOnWaitingInput(t *testing.T) {
	manager := NewManager(Config{}, nil)
	session := &Session{
		id:        "sess-1",
		projectID: "proj-1",
		title:     "Terminal 1",
	}

	events := make(chan StreamEvent, 3)
	stream := &SessionStream{events: events}

	events <- StreamEvent{
		Type: StreamEventMetadata,
		Metadata: &SessionMetadata{
			AIAssistant: &ai_assistant2.AIAssistantInfo{
				Detected: true,
				State:    "waiting_approval",
			},
		},
	}
	events <- StreamEvent{
		Type: StreamEventMetadata,
		Metadata: &SessionMetadata{
			AIAssistant: &ai_assistant2.AIAssistantInfo{
				Detected: true,
				State:    "waiting_input",
			},
		},
	}
	close(events)

	manager.monitorAssistantRecordsWithStream(session, stream)

	if approvals := manager.recordManager.GetApprovals(); len(approvals) != 0 {
		t.Fatalf("expected approvals to be cleared after leaving waiting_approval, got %d", len(approvals))
	}
}

func seedTerminalManagerSession(manager *Manager, projectID, sessionID string, orderIndex float64) *Session {
	session := &Session{
		id:         sessionID,
		projectID:  projectID,
		worktreeID: "worktree-1",
		title:      sessionID,
		createdAt:  time.Unix(0, int64(orderIndex)),
		orderIndex: orderIndex,
		closed:     make(chan struct{}),
	}
	session.status.Store(SessionStatusRunning)
	manager.sessions.Store(sessionID, session)
	return session
}

func TestManagerMoveSessionRenormalizesOrder(t *testing.T) {
	manager := NewManager(Config{}, nil)
	seedTerminalManagerSession(manager, "project-1", "session-a", 1000)
	seedTerminalManagerSession(manager, "project-1", "session-b", 2000)
	seedTerminalManagerSession(manager, "project-1", "session-c", 3000)

	moved, err := manager.MoveSession("project-1", "session-c", "", "session-a")
	if err != nil {
		t.Fatalf("MoveSession returned error: %v", err)
	}
	if moved.ID() != "session-c" {
		t.Fatalf("expected moved session-c, got %q", moved.ID())
	}

	sessions := manager.ListSessions("project-1")
	gotIDs := []string{sessions[0].ID, sessions[1].ID, sessions[2].ID}
	expectedIDs := []string{"session-c", "session-a", "session-b"}
	for index, expectedID := range expectedIDs {
		if gotIDs[index] != expectedID {
			t.Fatalf("expected order %v, got %v", expectedIDs, gotIDs)
		}
		expectedOrder := float64(index+1) * terminalSessionOrderStep
		if sessions[index].OrderIndex != expectedOrder {
			t.Fatalf("expected orderIndex %.0f at index %d, got %.0f", expectedOrder, index, sessions[index].OrderIndex)
		}
	}
}

func TestManagerAddSessionInsertsAfterAnchor(t *testing.T) {
	manager := NewManager(Config{}, nil)
	seedTerminalManagerSession(manager, "project-1", "session-a", 1000)
	seedTerminalManagerSession(manager, "project-1", "session-b", 2000)
	newSession := seedTerminalManagerSession(manager, "project-1", "session-c", 0)
	manager.sessions.Delete(newSession.ID())

	if err := manager.addSession(newSession, "session-a"); err != nil {
		t.Fatalf("addSession returned error: %v", err)
	}

	sessions := manager.ListSessions("project-1")
	gotIDs := []string{sessions[0].ID, sessions[1].ID, sessions[2].ID}
	expectedIDs := []string{"session-a", "session-c", "session-b"}
	for index, expectedID := range expectedIDs {
		if gotIDs[index] != expectedID {
			t.Fatalf("expected order %v, got %v", expectedIDs, gotIDs)
		}
	}
}

func TestManagerMoveSessionBroadcastsProjectList(t *testing.T) {
	manager := NewManager(Config{}, nil)
	seedTerminalManagerSession(manager, "project-1", "session-a", 1000)
	seedTerminalManagerSession(manager, "project-1", "session-b", 2000)

	events, unsubscribe := manager.SubscribeSessionListEvents()
	defer unsubscribe()

	if _, err := manager.MoveSession("project-1", "session-b", "", "session-a"); err != nil {
		t.Fatalf("MoveSession returned error: %v", err)
	}

	select {
	case event := <-events:
		if event.Type != "sessions" || event.ProjectID != "project-1" {
			t.Fatalf("unexpected event metadata: %#v", event)
		}
		if len(event.Sessions) != 2 || event.Sessions[0].ID != "session-b" || event.Sessions[1].ID != "session-a" {
			t.Fatalf("unexpected event sessions: %#v", event.Sessions)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for session list event")
	}
}
