package terminal

import (
	"testing"

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
