package filemanager

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code-kanban/model"
	"code-kanban/utils"
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

func TestListIncludesGitStatus(t *testing.T) {
	cleanup := initFileManagerTestDB(t)
	defer cleanup()

	repoDir := initFileManagerGitRepo(t)
	service, err := NewService(Config{
		DataDir: t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	projectID := seedFileManagerProjectScope(t, repoDir)

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Repo\nupdated\n"), 0o644); err != nil {
		t.Fatalf("rewrite README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "docs", "draft.md"), []byte("draft\n"), 0o644); err != nil {
		t.Fatalf("write draft.md: %v", err)
	}

	result, err := service.List(context.Background(), projectID, "", "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	var readmeStatus *GitStatus
	var docsStatus *GitStatus
	for _, entry := range result.Entries {
		switch entry.Name {
		case "README.md":
			readmeStatus = entry.GitStatus
		case "docs":
			docsStatus = entry.GitStatus
		}
	}

	if readmeStatus == nil || readmeStatus.Kind != GitStatusKindModified {
		t.Fatalf("README.md git status = %#v", readmeStatus)
	}
	if docsStatus == nil || docsStatus.Kind != GitStatusKindDirty {
		t.Fatalf("docs git status = %#v", docsStatus)
	}
}

func TestChangesSummarySkipsUntrackedInFastCountAndCompletesStats(t *testing.T) {
	cleanup := initFileManagerTestDB(t)
	defer cleanup()

	repoDir := initFileManagerGitRepo(t)
	service, err := NewService(Config{
		DataDir: t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	projectID := seedFileManagerProjectScope(t, repoDir)

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Repo\nupdated\n"), 0o644); err != nil {
		t.Fatalf("rewrite README.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "scratch.txt"), []byte("draft\n"), 0o644); err != nil {
		t.Fatalf("write scratch.txt: %v", err)
	}

	fastSummary, err := service.ChangesSummary(context.Background(), projectID, "", ChangesSummaryOptions{
		IncludeUntracked: false,
		WithStats:        false,
	})
	if err != nil {
		t.Fatalf("ChangesSummary fast phase returned error: %v", err)
	}
	if fastSummary.Count != 1 {
		t.Fatalf("fast summary count = %d, want %d", fastSummary.Count, 1)
	}
	if fastSummary.Additions != nil || fastSummary.Deletions != nil {
		t.Fatalf("fast summary should not include stats: %#v", fastSummary)
	}
	if fastSummary.StatsComplete || fastSummary.StatsTimedOut {
		t.Fatalf("unexpected fast summary flags: %#v", fastSummary)
	}

	statsSummary, err := service.ChangesSummary(context.Background(), projectID, "", ChangesSummaryOptions{
		IncludeUntracked: false,
		WithStats:        true,
		StatsTimeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ChangesSummary stats phase returned error: %v", err)
	}
	if statsSummary.Count != 1 {
		t.Fatalf("stats summary count = %d, want %d", statsSummary.Count, 1)
	}
	if !statsSummary.StatsComplete || statsSummary.StatsTimedOut {
		t.Fatalf("unexpected stats summary flags: %#v", statsSummary)
	}
	if statsSummary.Additions == nil || *statsSummary.Additions != 1 {
		t.Fatalf("stats summary additions = %#v, want %d", statsSummary.Additions, 1)
	}
	if statsSummary.Deletions == nil || *statsSummary.Deletions != 0 {
		t.Fatalf("stats summary deletions = %#v, want %d", statsSummary.Deletions, 0)
	}
}

func TestDiffReturnsUnifiedPatchAndStatusReasons(t *testing.T) {
	cleanup := initFileManagerTestDB(t)
	defer cleanup()

	repoDir := initFileManagerGitRepo(t)
	service, err := NewService(Config{
		DataDir: t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	projectID := seedFileManagerProjectScope(t, repoDir)

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Repo\nupdated\n"), 0o644); err != nil {
		t.Fatalf("rewrite README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "scratch.txt"), []byte("draft\n"), 0o644); err != nil {
		t.Fatalf("write scratch.txt: %v", err)
	}
	if err := os.Remove(filepath.Join(repoDir, "docs", "guide.md")); err != nil {
		t.Fatalf("remove docs/guide.md: %v", err)
	}

	diffResult, err := service.Diff(context.Background(), projectID, "", "README.md")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if !diffResult.Available {
		t.Fatalf("expected README.md diff to be available: %#v", diffResult)
	}
	if diffResult.Status == nil || diffResult.Status.Kind != GitStatusKindModified {
		t.Fatalf("unexpected README.md status: %#v", diffResult.Status)
	}
	if !strings.Contains(diffResult.DiffText, "+updated") {
		t.Fatalf("diff text missing updated line: %s", diffResult.DiffText)
	}

	untrackedResult, err := service.Diff(context.Background(), projectID, "", "scratch.txt")
	if err != nil {
		t.Fatalf("Diff returned error for untracked file: %v", err)
	}
	if untrackedResult.Available {
		t.Fatalf("expected untracked diff to be unavailable: %#v", untrackedResult)
	}
	if untrackedResult.Reason != "untracked" {
		t.Fatalf("unexpected untracked reason: %#v", untrackedResult)
	}

	deletedResult, err := service.Diff(context.Background(), projectID, "", "docs/guide.md")
	if err != nil {
		t.Fatalf("Diff returned error for deleted file: %v", err)
	}
	if !deletedResult.Available {
		t.Fatalf("expected deleted diff to be available: %#v", deletedResult)
	}
	if deletedResult.Status == nil || deletedResult.Status.Kind != GitStatusKindDeleted {
		t.Fatalf("unexpected deleted status: %#v", deletedResult.Status)
	}
	if !strings.Contains(deletedResult.DiffText, "--- a/docs/guide.md") {
		t.Fatalf("deleted diff text missing old path: %s", deletedResult.DiffText)
	}
}

func TestListChangesReturnsDeletedAndModifiedEntries(t *testing.T) {
	cleanup := initFileManagerTestDB(t)
	defer cleanup()

	repoDir := initFileManagerGitRepo(t)
	service, err := NewService(Config{
		DataDir: t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	projectID := seedFileManagerProjectScope(t, repoDir)

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Repo\nupdated\n"), 0o644); err != nil {
		t.Fatalf("rewrite README: %v", err)
	}
	if err := os.Remove(filepath.Join(repoDir, "docs", "guide.md")); err != nil {
		t.Fatalf("remove docs/guide.md: %v", err)
	}

	result, err := service.ListChanges(context.Background(), projectID, "")
	if err != nil {
		t.Fatalf("ListChanges returned error: %v", err)
	}

	statusByPath := make(map[string]GitStatusKind, len(result.Entries))
	existsByPath := make(map[string]bool, len(result.Entries))
	additionsByPath := make(map[string]int64, len(result.Entries))
	deletionsByPath := make(map[string]int64, len(result.Entries))
	for _, entry := range result.Entries {
		statusByPath[entry.Path] = entry.Status.Kind
		existsByPath[entry.Path] = entry.Exists
		additionsByPath[entry.Path] = entry.Additions
		deletionsByPath[entry.Path] = entry.Deletions
	}

	if statusByPath["README.md"] != GitStatusKindModified {
		t.Fatalf("README.md change status = %q", statusByPath["README.md"])
	}
	if statusByPath["docs/guide.md"] != GitStatusKindDeleted {
		t.Fatalf("docs/guide.md change status = %q", statusByPath["docs/guide.md"])
	}
	if existsByPath["docs/guide.md"] {
		t.Fatalf("expected deleted change entry to have exists=false")
	}
	if additionsByPath["README.md"] != 1 || deletionsByPath["README.md"] != 0 {
		t.Fatalf("unexpected README.md diff stat: +%d -%d", additionsByPath["README.md"], deletionsByPath["README.md"])
	}
	if additionsByPath["docs/guide.md"] != 0 || deletionsByPath["docs/guide.md"] != 1 {
		t.Fatalf("unexpected docs/guide.md diff stat: +%d -%d", additionsByPath["docs/guide.md"], deletionsByPath["docs/guide.md"])
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

func initFileManagerGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runFileManagerGit(t, dir, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Repo\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "guide.md"), []byte("guide\n"), 0o644); err != nil {
		t.Fatalf("write docs/guide.md: %v", err)
	}
	runFileManagerGit(t, dir, "add", "README.md", "docs/guide.md")
	runFileManagerGit(t, dir, "commit", "-m", "initial commit")
	return dir
}

func seedFileManagerProjectScope(t *testing.T, repoDir string) string {
	t.Helper()

	q, err := model.ResolveQueries(nil)
	if err != nil {
		t.Fatalf("ResolveQueries returned error: %v", err)
	}

	now := time.Now()
	projectID := utils.NewID()
	worktreeID := utils.NewID()
	worktreeBasePath := filepath.Join(repoDir, ".worktrees")
	defaultBranch := "main"
	project, err := q.ProjectCreate(context.Background(), &model.ProjectCreateParams{
		Id:               projectID,
		CreatedAt:        now,
		UpdatedAt:        now,
		Name:             "Git Project",
		Path:             repoDir,
		DefaultBranch:    defaultBranch,
		WorktreeBasePath: &worktreeBasePath,
		HidePath:         false,
	})
	if err != nil {
		t.Fatalf("ProjectCreate returned error: %v", err)
	}

	headCommit := "HEAD"
	if _, err := q.WorktreeCreate(context.Background(), &model.WorktreeCreateParams{
		Id:         worktreeID,
		CreatedAt:  now,
		UpdatedAt:  now,
		ProjectId:  project.Id,
		BranchName: defaultBranch,
		Path:       repoDir,
		IsMain:     true,
		IsBare:     false,
		HeadCommit: &headCommit,
	}); err != nil {
		t.Fatalf("WorktreeCreate returned error: %v", err)
	}

	return project.Id
}

func runFileManagerGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"HOME="+os.TempDir(),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}
