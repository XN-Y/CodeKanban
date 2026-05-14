package model

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestProjectServiceAccessOrdering(t *testing.T) {
	cleanup := initTestDB(t)
	defer cleanup()

	service := &ProjectService{}
	ctx := context.Background()
	firstDir := t.TempDir()
	secondDir := t.TempDir()

	firstProject, err := service.CreateProject(ctx, CreateProjectParams{
		Name: "First",
		Path: firstDir,
	})
	if err != nil {
		t.Fatalf("CreateProject first returned error: %v", err)
	}
	if firstProject.LastAccessedAt != nil {
		t.Fatalf("expected a new project to have no access timestamp")
	}

	secondProject, err := service.CreateProject(ctx, CreateProjectParams{
		Name: "Second",
		Path: secondDir,
	})
	if err != nil {
		t.Fatalf("CreateProject second returned error: %v", err)
	}

	if _, err := service.TouchProjectAccess(ctx, firstProject.Id); err != nil {
		t.Fatalf("TouchProjectAccess first returned error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	touchedSecond, err := service.TouchProjectAccess(ctx, secondProject.Id)
	if err != nil {
		t.Fatalf("TouchProjectAccess second returned error: %v", err)
	}
	if touchedSecond.LastAccessedAt == nil {
		t.Fatalf("expected access timestamp to be recorded")
	}

	projects, err := service.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) < 2 || projects[0].Id != secondProject.Id {
		t.Fatalf("expected most recently accessed project first, got %#v", projects)
	}

	clearedSecond, err := service.ClearProjectAccess(ctx, secondProject.Id)
	if err != nil {
		t.Fatalf("ClearProjectAccess returned error: %v", err)
	}
	if clearedSecond.LastAccessedAt != nil {
		t.Fatalf("expected access timestamp to be cleared")
	}

	projects, err = service.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects after clear returned error: %v", err)
	}
	if len(projects) < 2 || projects[0].Id != firstProject.Id {
		t.Fatalf("expected remaining accessed project first, got %#v", projects)
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
