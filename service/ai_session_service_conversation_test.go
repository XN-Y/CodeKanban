package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCodexConversationPreservesImageOnlyMessages(t *testing.T) {
	filePath := writeConversationTempFile(t, `{"timestamp":"2026-01-01T00:00:00Z","type":"event_msg","payload":{"type":"user_message","message":"","images":["C:/temp/example.png"]}}`)

	svc := NewAISessionService()
	messages, err := svc.parseCodexConversation(filePath)
	if err != nil {
		t.Fatalf("parseCodexConversation returned error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Role != "user" {
		t.Fatalf("expected user role, got %q", messages[0].Role)
	}
	if len(messages[0].Images) != 1 {
		t.Fatalf("expected 1 image attachment, got %d", len(messages[0].Images))
	}
	if messages[0].Images[0].Label != "example.png" {
		t.Fatalf("expected image label example.png, got %q", messages[0].Images[0].Label)
	}
}

func TestParseCodexConversationUsesLocalImagesAndPlaceholders(t *testing.T) {
	imagePath := filepath.Join(t.TempDir(), "codex-clipboard-demo.png")
	if err := os.WriteFile(imagePath, []byte("png-data"), 0o644); err != nil {
		t.Fatalf("failed to write temp image: %v", err)
	}
	filePath := writeConversationTempFile(t, `{"timestamp":"2026-01-01T00:00:00Z","type":"event_msg","payload":{"type":"user_message","message":"[Image #1]","images":[],"local_images":["`+filepath.ToSlash(imagePath)+`"],"text_elements":[{"placeholder":"[Image #1]"}]}}`)

	svc := NewAISessionService()
	messages, err := svc.parseCodexConversation(filePath)
	if err != nil {
		t.Fatalf("parseCodexConversation returned error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if len(messages[0].Images) != 1 {
		t.Fatalf("expected 1 image attachment, got %d", len(messages[0].Images))
	}
	if messages[0].Images[0].Label != "[Image #1]" {
		t.Fatalf("expected placeholder label [Image #1], got %q", messages[0].Images[0].Label)
	}
	if !messages[0].Images[0].Previewable {
		t.Fatal("expected local image attachment to be previewable")
	}
}

func TestParseClaudeConversationExtractsImageAttachments(t *testing.T) {
	filePath := writeConversationTempFile(t, `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Look at this"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGVsbG8="}}]},"timestamp":"2026-01-01T00:00:00Z","isMeta":false}`)

	svc := NewAISessionService()
	messages, err := svc.parseClaudeCodeConversation(filePath)
	if err != nil {
		t.Fatalf("parseClaudeCodeConversation returned error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "Look at this" {
		t.Fatalf("expected text content to be preserved, got %q", messages[0].Content)
	}
	if len(messages[0].Images) != 1 {
		t.Fatalf("expected 1 image attachment, got %d", len(messages[0].Images))
	}
	if !messages[0].Images[0].Previewable {
		t.Fatal("expected embedded image to be previewable")
	}
	if messages[0].Images[0].MimeType != "image/png" {
		t.Fatalf("expected image/png mime type, got %q", messages[0].Images[0].MimeType)
	}
}

func TestDecorateConversationMessagesAssignsPreviewURLs(t *testing.T) {
	messages := []*ConversationMessage{
		{
			Role:    "user",
			Content: "hello",
			Images: []ConversationImageAttachment{{
				Label:       "demo.png",
				Previewable: true,
				MimeType:    "image/png",
				Source:      `C:\temp\demo.png`,
			}},
		},
	}

	decorateConversationMessages(messages, "session-123")
	attachment := messages[0].Images[0]
	if attachment.ID != "m0-i0" {
		t.Fatalf("expected attachment id m0-i0, got %q", attachment.ID)
	}
	if attachment.PreviewURL != "/api/v1/ai-sessions/by-session-id/session-123/conversation/images/m0-i0" {
		t.Fatalf("unexpected preview URL: %q", attachment.PreviewURL)
	}
}

func TestResolveConversationImagePreviewSupportsDataURIAndFile(t *testing.T) {
	t.Run("data-uri", func(t *testing.T) {
		preview, err := resolveConversationImagePreview(ConversationImageAttachment{
			Previewable: true,
			Source:      "data:image/png;base64,aGVsbG8=",
		})
		if err != nil {
			t.Fatalf("resolveConversationImagePreview returned error: %v", err)
		}
		if preview.MimeType != "image/png" {
			t.Fatalf("expected image/png mime type, got %q", preview.MimeType)
		}
		if string(preview.Data) != "hello" {
			t.Fatalf("expected decoded data hello, got %q", string(preview.Data))
		}
	})

	t.Run("local-file", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "sample.png")
		if err := os.WriteFile(filePath, []byte("png-data"), 0o644); err != nil {
			t.Fatalf("failed to write temp image: %v", err)
		}

		preview, err := resolveConversationImagePreview(ConversationImageAttachment{
			Previewable: true,
			Source:      filePath,
		})
		if err != nil {
			t.Fatalf("resolveConversationImagePreview returned error: %v", err)
		}
		if preview.FilePath != filePath {
			t.Fatalf("expected preview file path %q, got %q", filePath, preview.FilePath)
		}
		if preview.MimeType != "image/png" {
			t.Fatalf("expected image/png mime type, got %q", preview.MimeType)
		}
	})
}

func TestBuildConversationImageAttachmentMarksRemoteURLsAsUnpreviewable(t *testing.T) {
	attachment, ok := buildConversationImageAttachment("https://example.com/demo.png", "", 0, false)
	if !ok {
		t.Fatal("expected remote image attachment to be created")
	}
	if attachment.Previewable {
		t.Fatal("expected remote image attachment to be unpreviewable")
	}
	if attachment.Label != "demo.png" {
		t.Fatalf("expected demo.png label, got %q", attachment.Label)
	}
}

func writeConversationTempFile(t *testing.T, content string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "conversation-*.jsonl")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := file.WriteString(content + "\n"); err != nil {
		_ = file.Close()
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	return file.Name()
}
