package log_watcher

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	// CodexSessionDirName is the subdirectory name for Codex sessions
	CodexSessionDirName = ".codex"
	// CodexSessionSubDir is the sessions subdirectory
	CodexSessionSubDir = "sessions"
	// CodexRolloutPrefix is the prefix for rollout files
	CodexRolloutPrefix = "rollout-"
	// CodexRolloutSuffix is the suffix for rollout files
	CodexRolloutSuffix = ".jsonl"
)

// CodexFileSearcher searches for Codex session files
type CodexFileSearcher struct {
	homeDir    string
	sessionDir string
}

// NewCodexFileSearcher creates a new Codex file searcher
func NewCodexFileSearcher() (*CodexFileSearcher, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sessionDir := filepath.Join(homeDir, CodexSessionDirName, CodexSessionSubDir)

	return &CodexFileSearcher{
		homeDir:    homeDir,
		sessionDir: sessionDir,
	}, nil
}

// NewCodexFileSearcherWithHomeDir creates a new Codex file searcher with custom home directory
func NewCodexFileSearcherWithHomeDir(homeDir string) *CodexFileSearcher {
	sessionDir := filepath.Join(homeDir, CodexSessionDirName, CodexSessionSubDir)

	return &CodexFileSearcher{
		homeDir:    homeDir,
		sessionDir: sessionDir,
	}
}

// GetSessionDir returns the base session directory
func (s *CodexFileSearcher) GetSessionDir() string {
	return s.sessionDir
}

// FindSessionFile searches for a session file created after the given time
func (s *CodexFileSearcher) FindSessionFile(ctx context.Context, afterTime time.Time) (string, error) {
	// Get today's date directory
	now := time.Now()
	dateDir := filepath.Join(s.sessionDir, now.Format("2006"), now.Format("01"), now.Format("02"))

	// Check if the directory exists
	if _, err := os.Stat(dateDir); os.IsNotExist(err) {
		return "", nil // Directory doesn't exist yet
	}

	// List all rollout files in the directory
	entries, err := os.ReadDir(dateDir)
	if err != nil {
		return "", err
	}

	// Find files matching the pattern and created after afterTime
	type fileWithTime struct {
		path    string
		modTime time.Time
		ctime   time.Time
	}

	var candidates []fileWithTime

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, CodexRolloutPrefix) || !strings.HasSuffix(name, CodexRolloutSuffix) {
			continue
		}

		filePath := filepath.Join(dateDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Get creation time (or mod time as fallback)
		ctime := getFileCreationTime(filePath, info)

		// Check if file was created after the process start time
		// Use a small tolerance (100ms) to account for timing differences
		tolerance := 100 * time.Millisecond
		if ctime.Add(tolerance).Before(afterTime) {
			continue
		}

		candidates = append(candidates, fileWithTime{
			path:    filePath,
			modTime: info.ModTime(),
			ctime:   ctime,
		})
	}

	if len(candidates) == 0 {
		return "", nil
	}

	// Sort by creation time (newest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ctime.After(candidates[j].ctime)
	})

	// Return the newest file that was created after afterTime
	return candidates[0].path, nil
}

// getFileCreationTime returns the file creation time
// On Windows, it returns the actual creation time
// On Unix systems, it returns the modification time as a fallback
func getFileCreationTime(path string, info os.FileInfo) time.Time {
	if runtime.GOOS == "windows" {
		// On Windows, we can get the creation time from the file info
		// The ModTime is the last modification time, but for newly created files
		// it's close to the creation time
		return getWindowsCreationTime(path, info)
	}

	// On Unix systems, use modification time as fallback
	// Most Unix filesystems don't track creation time
	return info.ModTime()
}

// ExtractSessionIDFromFilename extracts the session UUID from a rollout filename
// Filename format: rollout-2025-12-01T04-14-23-019ad666-f5ab-7501-a616-bbdc79da615b.jsonl
func ExtractSessionIDFromFilename(filename string) string {
	// Remove prefix and suffix
	name := strings.TrimPrefix(filename, CodexRolloutPrefix)
	name = strings.TrimSuffix(name, CodexRolloutSuffix)

	// The format is: timestamp-uuid
	// timestamp: 2025-12-01T04-14-23
	// uuid: 019ad666-f5ab-7501-a616-bbdc79da615b
	// Combined: 2025-12-01T04-14-23-019ad666-f5ab-7501-a616-bbdc79da615b

	// Find the UUID part (last 36 characters in standard UUID format)
	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (36 chars)
	if len(name) < 36 {
		return ""
	}

	// The UUID is at the end, but the timestamp also has dashes
	// We need to find where the UUID starts
	// UUID v7 format: timestamp-based, but we can identify by length

	// Split by dash and reconstruct UUID
	parts := strings.Split(name, "-")
	if len(parts) < 9 {
		// Not enough parts for timestamp + UUID
		return ""
	}

	// Last 5 parts form the UUID (8-4-4-4-12 format)
	uuidParts := parts[len(parts)-5:]
	return strings.Join(uuidParts, "-")
}

// BuildRolloutFilePath constructs the rollout file path from a session ID
// The session ID contains time information (UUID v7)
func BuildRolloutFilePath(baseDir string, sessionID string, timestamp time.Time) string {
	// Format: rollout-{timestamp}-{uuid}.jsonl
	// The timestamp in the filename uses the format: YYYY-MM-DDTHH-MM-SS
	dateDir := filepath.Join(baseDir, timestamp.Format("2006"), timestamp.Format("01"), timestamp.Format("02"))

	// Build filename from timestamp
	filename := CodexRolloutPrefix + timestamp.Format("2006-01-02T15-04-05") + "-" + sessionID + CodexRolloutSuffix

	return filepath.Join(dateDir, filename)
}
