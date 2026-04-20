package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type FileChangeKind string

const (
	FileChangeKindModified   FileChangeKind = "modified"
	FileChangeKindAdded      FileChangeKind = "added"
	FileChangeKindDeleted    FileChangeKind = "deleted"
	FileChangeKindRenamed    FileChangeKind = "renamed"
	FileChangeKindUntracked  FileChangeKind = "untracked"
	FileChangeKindConflicted FileChangeKind = "conflicted"
	FileChangeKindDirty      FileChangeKind = "dirty"
)

type FileStatus struct {
	Path         string
	Kind         FileChangeKind
	PreviousPath string
}

type DiffStat struct {
	Additions int64
	Deletions int64
}

type FileStatusResult struct {
	Statuses   map[string]FileStatus
	Truncated  bool
	TotalCount int
}

func ListFileStatuses(path string) (map[string]FileStatus, error) {
	return ListFileStatusesContext(context.Background(), path, true)
}

func ListFileStatusesContext(
	ctx context.Context,
	path string,
	includeUntracked bool,
) (map[string]FileStatus, error) {
	result, err := ListFileStatusesLimitedContext(ctx, path, includeUntracked, 0)
	return result.Statuses, err
}

func ListFileStatusesLimitedContext(
	ctx context.Context,
	path string,
	includeUntracked bool,
	maxEntries int,
) (FileStatusResult, error) {
	untrackedMode := "--untracked-files=no"
	if includeUntracked {
		untrackedMode = "--untracked-files=all"
	}

	cmd, stdout, err := startGitCommandStdoutPipe(
		ctx,
		path,
		"status",
		"--porcelain=2",
		"-z",
		untrackedMode,
	)
	if err != nil {
		return FileStatusResult{}, err
	}
	defer stdout.Close()

	parser := newGitPorcelainStatusStreamParser(maxEntries)
	readErr := parser.consume(stdout)
	waitErr := cmd.Wait()
	result := parser.result()

	if readErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, ctxErr
		}
		return result, readErr
	}
	if waitErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, ctxErr
		}
		return result, waitErr
	}
	return result, nil
}

func GenerateUnifiedDiffAgainstHEAD(path, relativePath, previousPath string) (string, error) {
	normalizedPath := normalizeGitRelativePath(relativePath)
	if normalizedPath == "" {
		return "", fmt.Errorf("path is required")
	}

	if repositoryHasHead(path) {
		args := []string{
			"diff",
			"--no-ext-diff",
			"--no-color",
			"-M",
			"HEAD",
			"--",
		}
		args = append(args, normalizedPath)
		if normalizedPrevious := normalizeGitRelativePath(previousPath); normalizedPrevious != "" && normalizedPrevious != normalizedPath {
			args = append(args, normalizedPrevious)
		}
		output, err := runGitOutputAllowDiffExit(path, args...)
		if err != nil {
			return "", err
		}
		return string(output), nil
	}

	output, err := runGitOutputAllowDiffExit(
		path,
		"diff",
		"--no-index",
		"--no-color",
		"--src-prefix=a/",
		"--dst-prefix=b/",
		"--",
		os.DevNull,
		filepath.FromSlash(normalizedPath),
	)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func GenerateDiffStatAgainstHEAD(path string, status FileStatus) (DiffStat, error) {
	return GenerateDiffStatAgainstHEADContext(context.Background(), path, status)
}

func GenerateDiffStatAgainstHEADContext(
	ctx context.Context,
	path string,
	status FileStatus,
) (DiffStat, error) {
	normalizedPath := normalizeGitRelativePath(status.Path)
	if normalizedPath == "" {
		return DiffStat{}, fmt.Errorf("path is required")
	}

	if repositoryHasHeadContext(ctx, path) && status.Kind != FileChangeKindUntracked {
		args := []string{
			"diff",
			"--numstat",
			"--no-ext-diff",
			"--no-color",
			"-M",
			"HEAD",
			"--",
			normalizedPath,
		}
		if normalizedPrevious := normalizeGitRelativePath(status.PreviousPath); normalizedPrevious != "" && normalizedPrevious != normalizedPath {
			args = append(args, normalizedPrevious)
		}
		output, err := runGitOutputAllowDiffExitContext(ctx, path, args...)
		if err != nil {
			return DiffStat{}, err
		}
		return parseGitDiffStatOutput(output), nil
	}

	args := []string{
		"diff",
		"--numstat",
		"--no-index",
		"--no-color",
		"--",
		os.DevNull,
		filepath.FromSlash(normalizedPath),
	}
	if status.Kind == FileChangeKindDeleted {
		args = []string{
			"diff",
			"--numstat",
			"--no-index",
			"--no-color",
			"--",
			filepath.FromSlash(normalizedPath),
			os.DevNull,
		}
	}
	output, err := runGitOutputAllowDiffExitContext(ctx, path, args...)
	if err != nil {
		return DiffStat{}, err
	}
	return parseGitDiffStatOutput(output), nil
}

func repositoryHasHead(path string) bool {
	return repositoryHasHeadContext(context.Background(), path)
}

func repositoryHasHeadContext(ctx context.Context, path string) bool {
	cmd := newGitCommandContext(ctx, path, "rev-parse", "--verify", "HEAD^{commit}")
	return cmd.Run() == nil
}

func parseGitFileStatusesPorcelainV2(raw []byte) map[string]FileStatus {
	parser := newGitPorcelainStatusStreamParser(0)
	if err := parser.consume(bytes.NewReader(raw)); err != nil {
		return map[string]FileStatus{}
	}
	return parser.result().Statuses
}

type gitPorcelainStatusStreamParser struct {
	maxEntries    int
	statuses      map[string]FileStatus
	truncated     bool
	totalCount    int
	pendingRename *FileStatus
}

func newGitPorcelainStatusStreamParser(maxEntries int) *gitPorcelainStatusStreamParser {
	return &gitPorcelainStatusStreamParser{
		maxEntries: maxEntries,
		statuses:   make(map[string]FileStatus),
	}
}

func (p *gitPorcelainStatusStreamParser) result() FileStatusResult {
	return FileStatusResult{
		Statuses:   p.statuses,
		Truncated:  p.truncated,
		TotalCount: p.totalCount,
	}
}

func (p *gitPorcelainStatusStreamParser) consume(reader io.Reader) error {
	buffered := bufio.NewReader(reader)
	for {
		recordBytes, err := buffered.ReadBytes(0)
		if len(recordBytes) > 0 {
			if recordBytes[len(recordBytes)-1] == 0 {
				recordBytes = recordBytes[:len(recordBytes)-1]
			}
			p.consumeRecord(string(recordBytes))
		}

		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return err
	}

	if p.pendingRename != nil {
		p.storeStatus(*p.pendingRename)
		p.pendingRename = nil
	}
	return nil
}

func (p *gitPorcelainStatusStreamParser) consumeRecord(record string) {
	if p.pendingRename != nil {
		p.pendingRename.PreviousPath = normalizeGitRelativePath(record)
		p.storeStatus(*p.pendingRename)
		p.pendingRename = nil
		return
	}
	if record == "" {
		return
	}

	switch record[0] {
	case '#':
		return
	case '?':
		path := normalizeGitRelativePath(strings.TrimPrefix(record, "? "))
		if path == "" {
			return
		}
		p.storeStatus(FileStatus{
			Path: path,
			Kind: FileChangeKindUntracked,
		})
	case '1':
		fields, path, ok := splitGitPorcelainRecord(record, 8)
		if !ok {
			return
		}
		normalizedPath := normalizeGitRelativePath(path)
		if normalizedPath == "" {
			return
		}
		p.storeStatus(FileStatus{
			Path: normalizedPath,
			Kind: classifyGitFileChange(fields[1], false),
		})
	case '2':
		fields, path, ok := splitGitPorcelainRecord(record, 9)
		if !ok {
			return
		}
		normalizedPath := normalizeGitRelativePath(path)
		if normalizedPath == "" {
			return
		}
		status := FileStatus{
			Path: normalizedPath,
			Kind: classifyGitFileChange(fields[1], true),
		}
		p.pendingRename = &status
	case 'u':
		_, path, ok := splitGitPorcelainRecord(record, 10)
		if !ok {
			return
		}
		normalizedPath := normalizeGitRelativePath(path)
		if normalizedPath == "" {
			return
		}
		p.storeStatus(FileStatus{
			Path: normalizedPath,
			Kind: FileChangeKindConflicted,
		})
	}
}

func (p *gitPorcelainStatusStreamParser) storeStatus(status FileStatus) {
	if status.Path == "" {
		return
	}

	if _, exists := p.statuses[status.Path]; exists {
		p.statuses[status.Path] = status
		return
	}

	p.totalCount++
	if p.maxEntries > 0 && len(p.statuses) >= p.maxEntries {
		p.truncated = true
		return
	}
	p.statuses[status.Path] = status
}

func splitGitPorcelainRecord(record string, fieldsBeforePath int) ([]string, string, bool) {
	fields := make([]string, 0, fieldsBeforePath)
	remaining := record
	for len(fields) < fieldsBeforePath {
		spaceIndex := strings.IndexByte(remaining, ' ')
		if spaceIndex == -1 {
			return nil, "", false
		}
		fields = append(fields, remaining[:spaceIndex])
		remaining = remaining[spaceIndex+1:]
	}
	if remaining == "" {
		return nil, "", false
	}
	return fields, remaining, true
}

func classifyGitFileChange(xy string, renamed bool) FileChangeKind {
	if renamed {
		return FileChangeKindRenamed
	}
	if isGitConflictXY(xy) {
		return FileChangeKindConflicted
	}
	if len(xy) < 2 {
		return FileChangeKindModified
	}
	x := xy[0]
	y := xy[1]
	if x == 'R' || y == 'R' {
		return FileChangeKindRenamed
	}
	if x == 'D' || y == 'D' {
		return FileChangeKindDeleted
	}
	if x == 'A' || y == 'A' {
		return FileChangeKindAdded
	}
	return FileChangeKindModified
}

func isGitConflictXY(xy string) bool {
	if len(xy) < 2 {
		return false
	}
	return xy[0] == 'U' || xy[1] == 'U'
}

func normalizeGitRelativePath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = filepath.ToSlash(trimmed)
	trimmed = strings.TrimPrefix(trimmed, "./")
	return strings.TrimPrefix(trimmed, "/")
}

func runGitOutputAllowDiffExit(path string, args ...string) ([]byte, error) {
	return runGitOutputAllowDiffExitContext(context.Background(), path, args...)
}

func runGitOutputAllowDiffExitContext(
	ctx context.Context,
	path string,
	args ...string,
) ([]byte, error) {
	cmd := newGitCommandContext(ctx, path, args...)
	output, err := cmd.Output()
	if err == nil {
		return output, nil
	}
	if ctx != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return output, nil
	}
	return nil, err
}

func parseGitDiffStatOutput(output []byte) DiffStat {
	line := strings.TrimSpace(string(output))
	if line == "" {
		return DiffStat{}
	}

	firstLine := strings.Split(line, "\n")[0]
	fields := strings.Split(firstLine, "\t")
	if len(fields) < 3 {
		return DiffStat{}
	}

	return DiffStat{
		Additions: parseGitNumstatField(fields[0]),
		Deletions: parseGitNumstatField(fields[1]),
	}
}

func parseGitNumstatField(value string) int64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "-" {
		return 0
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}
