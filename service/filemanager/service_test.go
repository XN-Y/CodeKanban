package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code-kanban/model"
)

func TestEnsureProtectedPathRejectsGitSegments(t *testing.T) {
	t.Parallel()

	cases := []string{
		".git",
		".git/config",
		"docs/.git/hooks",
	}
	for _, path := range cases {
		if err := ensureProtectedPath(path); err == nil {
			t.Fatalf("expected protected path error for %q", path)
		}
	}

	if err := ensureProtectedPath("docs/guide.md"); err != nil {
		t.Fatalf("unexpected error for normal path: %v", err)
	}
}

func TestResolveAbsolutePathRejectsScopeEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, _, err := resolveAbsolutePath(root, "../outside"); err == nil {
		t.Fatal("expected scope escape to fail")
	}

	normalized, absPath, err := resolveAbsolutePath(root, "docs/readme.md")
	if err != nil {
		t.Fatalf("resolveAbsolutePath returned error: %v", err)
	}
	if normalized != "docs/readme.md" {
		t.Fatalf("normalized path = %q, want %q", normalized, "docs/readme.md")
	}
	if !strings.HasPrefix(absPath, root) {
		t.Fatalf("resolved path %q does not stay under root %q", absPath, root)
	}
}

func TestAppendUploadChunkPersistsOffsetAndData(t *testing.T) {
	service, err := NewService(Config{
		DataDir:         t.TempDir(),
		UploadChunkSize: 8,
	}, nil)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	partPath := filepath.Join(service.uploadsDir, "up1.part")
	if err := os.WriteFile(partPath, nil, 0o644); err != nil {
		t.Fatalf("failed to create part file: %v", err)
	}

	now := time.Now()
	meta := uploadMeta{
		ID:        "up1",
		ProjectID: "project-1",
		ScopeID:   "project:project-1",
		FileName:  "demo.txt",
		Size:      11,
		Offset:    0,
		ChunkSize: 8,
		PartPath:  partPath,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	if err := service.writeJSONFile(service.uploadMetaPath(meta.ID), meta); err != nil {
		t.Fatalf("failed to persist upload meta: %v", err)
	}

	session, err := service.AppendUploadChunk(meta.ProjectID, meta.ID, 0, 5, strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("AppendUploadChunk returned error: %v", err)
	}
	if session.Offset != 5 {
		t.Fatalf("offset = %d, want %d", session.Offset, 5)
	}

	data, err := os.ReadFile(partPath)
	if err != nil {
		t.Fatalf("failed to read part file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("part file content = %q, want %q", data, "hello")
	}

	if _, err := service.AppendUploadChunk(meta.ProjectID, meta.ID, 0, 1, strings.NewReader("x")); err == nil {
		t.Fatal("expected offset mismatch error")
	}
}

func TestListScopesPrefersMainWorktreeOverProjectScopeWhenPathsMatch(t *testing.T) {
	t.Parallel()

	cleanup := initFileManagerTestDB(t)
	defer cleanup()

	projectDir := t.TempDir()
	projectService := &model.ProjectService{}
	project, err := projectService.CreateProject(context.Background(), model.CreateProjectParams{
		Name: "Plain Folder Project",
		Path: projectDir,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	service, err := NewService(Config{
		DataDir: t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	scopes, err := service.ListScopes(context.Background(), project.Id)
	if err != nil {
		t.Fatalf("ListScopes returned error: %v", err)
	}
	if len(scopes) != 1 {
		t.Fatalf("expected exactly one scope, got %d", len(scopes))
	}
	if scopes[0].Kind != ScopeKindWorktree {
		t.Fatalf("expected main worktree scope to be retained, got %s", scopes[0].Kind)
	}
	if filepath.Clean(scopes[0].RootPath) != filepath.Clean(projectDir) {
		t.Fatalf("scope root = %q, want %q", scopes[0].RootPath, filepath.Clean(projectDir))
	}
}

func initFileManagerTestDB(t *testing.T) func() {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	if err := model.InitWithDSN(dsn, 0, true); err != nil {
		t.Fatalf("InitWithDSN: %v", err)
	}

	return func() {
		model.DBClose()
	}
}
