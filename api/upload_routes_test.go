package api

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestHandleClipboardImageRejectsEmptyData(t *testing.T) {
	ctrl := &uploadController{logger: zap.NewNop()}
	input := &uploadClipboardImageInput{}
	input.Body.FileName = "pasted-image.png"

	if _, err := ctrl.handleClipboardImage(context.Background(), input); err == nil {
		t.Fatal("expected error for empty image data")
	}
}

func TestHandleClipboardImageSavesFileAndSanitizesName(t *testing.T) {
	ctrl := &uploadController{logger: zap.NewNop()}
	payload := []byte("fake-image-bytes")
	input := &uploadClipboardImageInput{}
	input.Body.FileName = "../nested/demo image.png"
	input.Body.Data = base64.StdEncoding.EncodeToString(payload)
	input.Body.Source = "paste"

	resp, err := ctrl.handleClipboardImage(context.Background(), input)
	if err != nil {
		t.Fatalf("handleClipboardImage returned error: %v", err)
	}

	filePath := resp.Body.Item.Path
	t.Cleanup(func() {
		_ = os.Remove(filePath)
	})

	if filePath == "" {
		t.Fatal("expected saved file path")
	}
	if filepath.Base(filePath) != resp.Body.Item.FileName {
		t.Fatalf("expected response file name to match saved file, got path=%q fileName=%q", filePath, resp.Body.Item.FileName)
	}
	if strings.Contains(resp.Body.Item.FileName, "..") {
		t.Fatalf("expected sanitized file name, got %q", resp.Body.Item.FileName)
	}
	if !strings.Contains(resp.Body.Item.FileName, "demo image.png") {
		t.Fatalf("expected original base file name to be preserved, got %q", resp.Body.Item.FileName)
	}

	savedData, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read saved file: %v", readErr)
	}
	if string(savedData) != string(payload) {
		t.Fatalf("saved file content mismatch, got %q want %q", savedData, payload)
	}
}
