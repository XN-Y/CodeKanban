package websession

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestHistoryAttachmentsFromEventPayloadSupportsMapSlices(t *testing.T) {
	payload := map[string]any{
		"atts": []map[string]any{
			{
				"id":   "att_live",
				"name": "image.png",
				"mime": "image/png",
				"sz":   int64(42),
			},
		},
	}

	got := historyAttachmentsFromEventPayload(payload)
	if len(got) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(got))
	}
	if got[0].ID != "att_live" {
		t.Fatalf("expected attachment id %q, got %q", "att_live", got[0].ID)
	}
	if got[0].Name != "image.png" {
		t.Fatalf("expected attachment name %q, got %q", "image.png", got[0].Name)
	}
	if got[0].Mime != "image/png" {
		t.Fatalf("expected attachment mime %q, got %q", "image/png", got[0].Mime)
	}
	if got[0].Size != 42 {
		t.Fatalf("expected attachment size %d, got %d", 42, got[0].Size)
	}
}

func TestApplyEventToHistoryCacheIncludesUserAttachmentsInRealtimeFrames(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	project := seedProject(t)
	session := seedWebSession(t, project.ID, "Realtime Attachments", 1000)
	manager, err := NewManager(Config{DataDir: t.TempDir()}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	attachments := []Attachment{{
		ID:   "att_live",
		Name: "image.png",
		Mime: "image/png",
		Size: 42,
	}}

	item, err := manager.applyEventToHistoryCache(context.Background(), session.ID, Event{
		ID:        "evt_user_live",
		Type:      "msg_u",
		Timestamp: time.Now(),
		Payload: map[string]any{
			"txt":  "hello [Image #1]",
			"atts": attachmentPayloads(attachments),
		},
	})
	if err != nil {
		t.Fatalf("applyEventToHistoryCache returned error: %v", err)
	}
	if item == nil {
		t.Fatal("expected history item, got nil")
	}
	if len(item.Attachments) != 1 {
		t.Fatalf("expected 1 attachment in history item, got %d", len(item.Attachments))
	}
	if item.Attachments[0].ID != attachments[0].ID {
		t.Fatalf("expected attachment id %q, got %q", attachments[0].ID, item.Attachments[0].ID)
	}

	frame := newHistoryItemFrame(session.ID, *item, nil)
	if frame.Item == nil {
		t.Fatal("expected history item wire frame payload")
	}
	if len(frame.Item.Attachments) != 1 {
		t.Fatalf("expected 1 attachment in wire frame, got %d", len(frame.Item.Attachments))
	}
	if frame.Item.Attachments[0].ID != attachments[0].ID {
		t.Fatalf("expected wire attachment id %q, got %q", attachments[0].ID, frame.Item.Attachments[0].ID)
	}
}
