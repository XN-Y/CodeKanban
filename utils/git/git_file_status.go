package git

import (
	"bytes"
	"errors"
	"fmt"
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

func ListFileStatuses(path string) (map[string]FileStatus, error) {
	cmd := newGitCommand(path, "status", "--porcelain=2", "-z", "--untracked-files=all")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseGitFileStatusesPorcelainV2(output), nil
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
	normalizedPath := normalizeGitRelativePath(status.Path)
	if normalizedPath == "" {
		return DiffStat{}, fmt.Errorf("path is required")
	}

	if repositoryHasHead(path) && status.Kind != FileChangeKindUntracked {
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
		output, err := runGitOutputAllowDiffExit(path, args...)
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
	output, err := runGitOutputAllowDiffExit(path, args...)
	if err != nil {
		return DiffStat{}, err
	}
	return parseGitDiffStatOutput(output), nil
}

func repositoryHasHead(path string) bool {
	cmd := newGitCommand(path, "rev-parse", "--verify", "HEAD^{commit}")
	return cmd.Run() == nil
}

func parseGitFileStatusesPorcelainV2(raw []byte) map[string]FileStatus {
	result := make(map[string]FileStatus)
	records := bytes.Split(raw, []byte{0})
	for index := 0; index < len(records); index++ {
		recordBytes := records[index]
		if len(recordBytes) == 0 {
			continue
		}

		record := string(recordBytes)
		switch record[0] {
		case '#':
			continue
		case '?':
			path := normalizeGitRelativePath(strings.TrimPrefix(record, "? "))
			if path == "" {
				continue
			}
			result[path] = FileStatus{
				Path: path,
				Kind: FileChangeKindUntracked,
			}
		case '1':
			fields, path, ok := splitGitPorcelainRecord(record, 8)
			if !ok {
				continue
			}
			normalizedPath := normalizeGitRelativePath(path)
			if normalizedPath == "" {
				continue
			}
			result[normalizedPath] = FileStatus{
				Path: normalizedPath,
				Kind: classifyGitFileChange(fields[1], false),
			}
		case '2':
			fields, path, ok := splitGitPorcelainRecord(record, 9)
			if !ok {
				continue
			}
			normalizedPath := normalizeGitRelativePath(path)
			if normalizedPath == "" {
				continue
			}
			previousPath := ""
			if index+1 < len(records) {
				previousPath = normalizeGitRelativePath(string(records[index+1]))
				index++
			}
			result[normalizedPath] = FileStatus{
				Path:         normalizedPath,
				Kind:         classifyGitFileChange(fields[1], true),
				PreviousPath: previousPath,
			}
		case 'u':
			_, path, ok := splitGitPorcelainRecord(record, 10)
			if !ok {
				continue
			}
			normalizedPath := normalizeGitRelativePath(path)
			if normalizedPath == "" {
				continue
			}
			result[normalizedPath] = FileStatus{
				Path: normalizedPath,
				Kind: FileChangeKindConflicted,
			}
		}
	}
	return result
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
	cmd := newGitCommand(path, args...)
	output, err := cmd.Output()
	if err == nil {
		return output, nil
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
