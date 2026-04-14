package model

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectServiceCreateProject(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	repoPath := createProjectTestRepo(t)
	service := &ProjectService{}

	ctx := context.Background()
	project, err := service.CreateProject(ctx, CreateProjectParams{
		Name:        "Demo Project",
		Path:        repoPath,
		Description: "example project",
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	if project.Id == "" {
		t.Fatalf("expected project ID to be set")
	}
	if project.WorktreeBasePath == nil || strings.TrimSpace(*project.WorktreeBasePath) == "" {
		t.Fatalf("expected worktree base path to be populated")
	}

	q, err := resolveQueries(nil)
	if err != nil {
		t.Fatalf("resolveQueries: %v", err)
	}

	stored, err := q.ProjectGetByID(ctx, project.Id)
	if err != nil {
		t.Fatalf("failed to reload project: %v", err)
	}

	if stored.DefaultBranch == nil || *stored.DefaultBranch != "main" {
		t.Fatalf("expected default branch main, got %v", stored.DefaultBranch)
	}

	worktrees, err := q.WorktreeListByProject(ctx, project.Id)
	if err != nil {
		t.Fatalf("query worktrees failed: %v", err)
	}
	if len(worktrees) == 0 {
		t.Fatalf("expected at least one worktree record")
	}
}

func TestProjectServiceCreateProjectInvalidPath(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	service := &ProjectService{}
	_, err := service.CreateProject(context.Background(), CreateProjectParams{
		Name: "Invalid Project",
		Path: "C:/does/not/exist",
	})
	if err == nil {
		t.Fatalf("expected error for invalid repository path")
	}
	if !errors.Is(err, ErrInvalidProjectPath) {
		t.Fatalf("expected ErrInvalidProjectPath, got %v", err)
	}
}

func TestProjectServiceCreateProjectWithoutGitRepo(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	service := &ProjectService{}
	tmpDir := t.TempDir()

	project, err := service.CreateProject(context.Background(), CreateProjectParams{
		Name: "Plain Folder Project",
		Path: tmpDir,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	if project.RemoteUrl != nil {
		t.Fatalf("expected remote URL to be nil for non-git directory")
	}
	if project.DefaultBranch == nil || strings.TrimSpace(*project.DefaultBranch) == "" {
		t.Fatalf("expected default branch to fallback to main")
	}

	q, err := resolveQueries(nil)
	if err != nil {
		t.Fatalf("resolveQueries: %v", err)
	}
	stored, err := q.ProjectGetByID(context.Background(), project.Id)
	if err != nil {
		t.Fatalf("failed to reload project: %v", err)
	}
	if stored.Path != filepath.Clean(tmpDir) {
		t.Fatalf("expected stored path %s, got %s", filepath.Clean(tmpDir), stored.Path)
	}
	worktrees, err := q.WorktreeListByProject(context.Background(), project.Id)
	if err != nil {
		t.Fatalf("query worktrees failed: %v", err)
	}
	if len(worktrees) == 0 {
		t.Fatalf("expected virtual main worktree for non-git project")
	}
}

func TestProjectServiceUpdateProject(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	repoPath := createProjectTestRepo(t)
	service := &ProjectService{}

	ctx := context.Background()
	project, err := service.CreateProject(ctx, CreateProjectParams{
		Name:        "Sample",
		Path:        repoPath,
		Description: "initial",
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	updated, err := service.UpdateProject(ctx, project.Id, UpdateProjectParams{
		Name:        "Renamed Project",
		Description: "updated description",
		HidePath:    true,
	})
	if err != nil {
		t.Fatalf("UpdateProject returned error: %v", err)
	}
	if updated.Name != "Renamed Project" {
		t.Fatalf("expected project name to update, got %s", updated.Name)
	}
	if updated.Description == nil || *updated.Description != "updated description" {
		t.Fatalf("expected description to update")
	}
	if !updated.HidePath {
		t.Fatalf("expected hidePath to be true")
	}
}

func initTestDB(t *testing.T) func() {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	if err := InitWithDSN(dsn, 0, true); err != nil {
		t.Fatalf("InitWithDSN: %v", err)
	}
	return func() {
		DBClose()
	}
}

func createProjectTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runGitCommand(t, dir, "init", "-b", "main")
	runGitCommand(t, dir, "config", "user.email", "test@example.com")
	runGitCommand(t, dir, "config", "user.name", "Test User")

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("demo"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	runGitCommand(t, dir, "add", "README.md")
	runGitCommand(t, dir, "commit", "-m", "init")
	return dir
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}
