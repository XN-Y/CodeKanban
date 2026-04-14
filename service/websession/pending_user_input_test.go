package websession

import (
	"testing"
	"time"
)

func TestPendingUserInputFromHistoryTracksLatestOpenRequest(t *testing.T) {
	timestamp := time.Date(2026, time.April, 10, 0, 0, 3, 0, time.UTC)
	requestID := "call_1"
	pending := pendingUserInputFromHistory([]HistoryItem{
		{
			ID:           "req-1",
			SourceItemID: &requestID,
			Kind:         "system",
			ItemType:     "user_input_request",
			Text:         "Pick one",
			Timestamp:    &timestamp,
			ObservedAt:   &timestamp,
			Detail: &HistoryDetail{
				Type:   "user_input_request",
				Prompt: "Pick one",
				Questions: []toolRequestQuestion{{
					ID:       "scope",
					Question: "Pick one",
				}},
			},
		},
	})
	if pending == nil {
		t.Fatal("expected pending user input")
	}
	if pending.ItemID != "call_1" {
		t.Fatalf("expected item id call_1, got %q", pending.ItemID)
	}
	if pending.RequestedAt == nil || !pending.RequestedAt.Equal(timestamp) {
		t.Fatalf("expected requestedAt %v, got %#v", timestamp, pending.RequestedAt)
	}
}

func TestPendingUserInputFromHistoryClearsAfterResponse(t *testing.T) {
	requestID := "call_1"
	pending := pendingUserInputFromHistory([]HistoryItem{
		{
			ID:           "req-1",
			SourceItemID: &requestID,
			Kind:         "system",
			ItemType:     "user_input_request",
			Text:         "Pick one",
			Detail: &HistoryDetail{
				Type:   "user_input_request",
				Prompt: "Pick one",
			},
		},
		{
			ID:       "req-2",
			Kind:     "system",
			ItemType: "user_input_response",
			Detail: &HistoryDetail{
				Type: "user_input_response",
			},
		},
	})
	if pending != nil {
		t.Fatalf("expected pending user input to be cleared, got %#v", pending)
	}
}
