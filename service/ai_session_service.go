package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"code-kanban/model"
	"code-kanban/model/tables"
	"code-kanban/utils"
	"code-kanban/utils/ai_assistant2/log_watcher"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// dirCacheEntry holds cached directory scan results.
type dirCacheEntry struct {
	modTime  time.Time           // Directory modification time
	sessions []*AISessionSummary // Cached session list
	cachedAt time.Time           // When this cache was created
}

// dirCacheTTL is the minimum time before re-checking directory mod time.
const dirCacheTTL = 3 * time.Second

// Scan phase constants
const (
	ScanPhaseRecent   = "recent"   // Last 24 hours - fast, priority scan
	ScanPhaseExtended = "extended" // 1-15 days - background scan
	ScanPhaseComplete = "complete" // Scan finished
)

// Time thresholds for phased scanning
const (
	recentThreshold   = 24 * time.Hour // Files within 24 hours
	maxAgeThreshold   = 15 * 24 * time.Hour // Ignore files older than 15 days
)

// scanState tracks the scanning progress for a project
type scanState struct {
	phase       string              // Current scan phase
	sessions    []*AISessionSummary // Accumulated sessions
	pendingDirs []string            // Directories pending for extended scan
	mu          sync.RWMutex
}

// Global directory cache (projectDir -> cache entry)
var (
	dirCache   = make(map[string]*dirCacheEntry)
	dirCacheMu sync.RWMutex

	// Scan state tracking (projectPath -> state)
	scanStates   = make(map[string]*scanState)
	scanStatesMu sync.RWMutex

	// Background scan queue
	bgScanQueue   = make(chan *bgScanTask, 100)
	bgScanStarted bool
	bgScanMu      sync.Mutex
)

// bgScanTask represents a background scan task
type bgScanTask struct {
	projectPath string
	scanType    string // "claude" or "codex"
	projectDir  string // For claude: the specific project directory
}

// AISessionService manages AI assistant session detection and caching.
type AISessionService struct{}

// startBackgroundScanner starts the background scan worker if not already started
func startBackgroundScanner() {
	bgScanMu.Lock()
	defer bgScanMu.Unlock()

	if bgScanStarted {
		return
	}
	bgScanStarted = true

	go func() {
		service := NewAISessionService()
		for task := range bgScanQueue {
			ctx := context.Background()
			logger := service.logger(ctx)

			switch task.scanType {
			case "claude":
				if err := service.scanClaudeExtendedPhase(ctx, task.projectPath, task.projectDir); err != nil {
					logger.Debug("background claude scan failed",
						zap.String("projectPath", task.projectPath),
						zap.Error(err))
				}
			case "codex":
				if err := service.scanCodexExtendedPhase(ctx, task.projectPath); err != nil {
					logger.Debug("background codex scan failed",
						zap.String("projectPath", task.projectPath),
						zap.Error(err))
				}
			}
		}
	}()
}

// NewAISessionService creates a new AISessionService.
func NewAISessionService() *AISessionService {
	return &AISessionService{}
}

// ProjectAISessions contains AI session information for a project.
type ProjectAISessions struct {
	HasClaudeCode      bool                `json:"hasClaudeCode"`
	HasCodex           bool                `json:"hasCodex"`
	ClaudeSessions     []*AISessionSummary `json:"claudeSessions,omitempty"`
	CodexSessions      []*AISessionSummary `json:"codexSessions,omitempty"`
	ClaudeScanPhase    string              `json:"claudeScanPhase,omitempty"`    // "recent", "extended", "complete"
	CodexScanPhase     string              `json:"codexScanPhase,omitempty"`     // "recent", "extended", "complete"
}

// AISessionSummary contains summary information about an AI session.
type AISessionSummary struct {
	ID               string     `json:"id"`
	SessionID        string     `json:"sessionId"`
	Type             string     `json:"type"`
	Model            string     `json:"model,omitempty"`
	Title            string     `json:"title,omitempty"`
	SessionStartedAt time.Time  `json:"sessionStartedAt"`
	LastMessageAt    *time.Time `json:"lastMessageAt,omitempty"`
	MessageCount     int        `json:"messageCount"`
	FilePath         string     `json:"filePath"`
}

// GetProjectAISessions returns AI session information for a project path.
// It uses database caching to avoid repeated filesystem scans.
// First call returns recent (24h) sessions quickly, then background scans older files.
func (s *AISessionService) GetProjectAISessions(ctx context.Context, projectPath string) (*ProjectAISessions, error) {
	ctx = ensureContext(ctx)
	logger := s.logger(ctx)

	// Ensure background scanner is running
	startBackgroundScanner()

	result := &ProjectAISessions{
		ClaudeSessions:  make([]*AISessionSummary, 0),
		CodexSessions:   make([]*AISessionSummary, 0),
		ClaudeScanPhase: ScanPhaseComplete,
		CodexScanPhase:  ScanPhaseComplete,
	}

	// Normalize the project path
	projectPath = filepath.Clean(projectPath)

	// Get Claude Code sessions (phased)
	claudeSessions, claudePhase, err := s.getClaudeCodeSessionsPhased(ctx, projectPath)
	if err != nil {
		logger.Warn("failed to get Claude Code sessions", zap.Error(err), zap.String("path", projectPath))
	} else {
		result.ClaudeSessions = claudeSessions
		result.HasClaudeCode = len(claudeSessions) > 0
		result.ClaudeScanPhase = claudePhase
	}

	// Get Codex sessions (phased)
	codexSessions, codexPhase, err := s.getCodexSessionsPhased(ctx, projectPath)
	if err != nil {
		logger.Warn("failed to get Codex sessions", zap.Error(err), zap.String("path", projectPath))
	} else {
		result.CodexSessions = codexSessions
		result.HasCodex = len(codexSessions) > 0
		result.CodexScanPhase = codexPhase
	}

	return result, nil
}

// getClaudeCodeSessionsPhased returns Claude Code sessions using phased scanning.
// Phase 1: Quickly return sessions from last 24 hours
// Phase 2: Background scan for 1-15 days old sessions
// Files older than 15 days are ignored
func (s *AISessionService) getClaudeCodeSessionsPhased(ctx context.Context, projectPath string) ([]*AISessionSummary, string, error) {
	logger := s.logger(ctx)

	// Create searcher to find Claude Code session directory
	searcher, err := log_watcher.NewClaudeCodeFileSearcher(projectPath)
	if err != nil {
		return nil, ScanPhaseComplete, err
	}

	// Get the project-specific session directory
	encodedPath := encodePathForClaude(projectPath)
	projectDir := filepath.Join(searcher.GetSessionDir(), encodedPath)

	// Check if directory exists and get its mod time
	dirInfo, err := os.Stat(projectDir)
	if os.IsNotExist(err) {
		return nil, ScanPhaseComplete, nil // No sessions for this project
	}
	if err != nil {
		return nil, ScanPhaseComplete, err
	}
	dirModTime := dirInfo.ModTime()

	// Check directory cache - fast path
	dirCacheMu.RLock()
	cached, hasCached := dirCache[projectDir]
	dirCacheMu.RUnlock()

	if hasCached {
		// If within TTL, return cached without checking mod time
		if time.Since(cached.cachedAt) < dirCacheTTL {
			// Check scan state for phase
			phase := s.getClaudeScanPhase(projectPath)
			return cached.sessions, phase, nil
		}
		// If mod time unchanged, refresh TTL and return cached
		if cached.modTime.Equal(dirModTime) {
			dirCacheMu.Lock()
			cached.cachedAt = time.Now()
			dirCacheMu.Unlock()
			phase := s.getClaudeScanPhase(projectPath)
			return cached.sessions, phase, nil
		}
	}

	// Directory changed or no cache - do phased scan
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, ScanPhaseComplete, err
	}

	db := model.GetDB()
	if db == nil {
		return nil, ScanPhaseComplete, model.ErrDBNotInitialized
	}

	now := time.Now()
	var sessions []*AISessionSummary
	var extendedFiles []string // Files for background scan (1-15 days old)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip agent files and non-jsonl files
		if strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		filePath := filepath.Join(projectDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileAge := now.Sub(info.ModTime())

		// Skip files older than 15 days
		if fileAge > maxAgeThreshold {
			continue
		}

		// Extract session ID from filename (remove .jsonl extension)
		sessionID := strings.TrimSuffix(name, ".jsonl")

		// For files older than 24 hours, check if already cached in DB
		if fileAge > recentThreshold {
			// Check if we have a valid cached entry in DB
			var cached tables.AISessionTable
			err := db.WithContext(ctx).
				Where("session_id = ? AND type = ?", sessionID, tables.AISessionTypeClaudeCode).
				First(&cached).Error

			if err == nil && cached.FileModTime.Equal(info.ModTime()) && cached.FileSize == info.Size() {
				// Cache hit - use cached data
				sessions = append(sessions, &AISessionSummary{
					ID:               cached.ID,
					SessionID:        cached.SessionID,
					Type:             string(cached.Type),
					Model:            cached.Model,
					Title:            cached.Title,
					SessionStartedAt: cached.SessionStartedAt,
					LastMessageAt:    cached.LastMessageAt,
					MessageCount:     cached.MessageCount,
					FilePath:         cached.FilePath,
				})
			} else {
				// Need to scan - add to extended queue
				extendedFiles = append(extendedFiles, filePath)
			}
			continue
		}

		// Recent files (within 24 hours) - process immediately
		session, err := s.getOrUpdateClaudeSession(ctx, db, sessionID, filePath, info, projectPath)
		if err != nil {
			logger.Debug("failed to process session file",
				zap.String("file", filePath),
				zap.Error(err))
			continue
		}

		if session != nil {
			sessions = append(sessions, session)
		}
	}

	// Sort by last message time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		ti := sessions[i].LastMessageAt
		tj := sessions[j].LastMessageAt
		if ti != nil && tj != nil {
			return ti.After(*tj)
		}
		if ti != nil {
			return true
		}
		if tj != nil {
			return false
		}
		return sessions[i].SessionStartedAt.After(sessions[j].SessionStartedAt)
	})

	// Determine scan phase and queue background work if needed
	var phase string
	if len(extendedFiles) > 0 {
		phase = ScanPhaseRecent

		// Store pending files in scan state
		scanStatesMu.Lock()
		state, exists := scanStates[projectPath+":claude"]
		if !exists {
			state = &scanState{
				phase:       ScanPhaseRecent,
				sessions:    sessions,
				pendingDirs: extendedFiles,
			}
			scanStates[projectPath+":claude"] = state
		} else {
			state.mu.Lock()
			state.pendingDirs = extendedFiles
			state.sessions = sessions
			state.phase = ScanPhaseRecent
			state.mu.Unlock()
		}
		scanStatesMu.Unlock()

		// Queue background scan
		select {
		case bgScanQueue <- &bgScanTask{
			projectPath: projectPath,
			scanType:    "claude",
			projectDir:  projectDir,
		}:
		default:
			// Queue full, will try again later
			logger.Debug("background scan queue full, skipping")
		}
	} else {
		phase = ScanPhaseComplete
		// Mark as complete in scan state
		scanStatesMu.Lock()
		if state, exists := scanStates[projectPath+":claude"]; exists {
			state.mu.Lock()
			state.phase = ScanPhaseComplete
			state.mu.Unlock()
		}
		scanStatesMu.Unlock()
	}

	// Update directory cache
	dirCacheMu.Lock()
	dirCache[projectDir] = &dirCacheEntry{
		modTime:  dirModTime,
		sessions: sessions,
		cachedAt: time.Now(),
	}
	dirCacheMu.Unlock()

	return sessions, phase, nil
}

// getClaudeScanPhase returns the current scan phase for a project
func (s *AISessionService) getClaudeScanPhase(projectPath string) string {
	scanStatesMu.RLock()
	defer scanStatesMu.RUnlock()

	if state, exists := scanStates[projectPath+":claude"]; exists {
		state.mu.RLock()
		defer state.mu.RUnlock()
		return state.phase
	}
	return ScanPhaseComplete
}

// scanClaudeExtendedPhase processes extended phase files in background
func (s *AISessionService) scanClaudeExtendedPhase(ctx context.Context, projectPath, projectDir string) error {
	logger := s.logger(ctx)

	scanStatesMu.RLock()
	state, exists := scanStates[projectPath+":claude"]
	scanStatesMu.RUnlock()

	if !exists {
		return nil
	}

	state.mu.RLock()
	pendingFiles := make([]string, len(state.pendingDirs))
	copy(pendingFiles, state.pendingDirs)
	state.mu.RUnlock()

	if len(pendingFiles) == 0 {
		return nil
	}

	db := model.GetDB()
	if db == nil {
		return model.ErrDBNotInitialized
	}

	var newSessions []*AISessionSummary

	for _, filePath := range pendingFiles {
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		name := filepath.Base(filePath)
		sessionID := strings.TrimSuffix(name, ".jsonl")

		session, err := s.getOrUpdateClaudeSession(ctx, db, sessionID, filePath, info, projectPath)
		if err != nil {
			logger.Debug("failed to process session file in background",
				zap.String("file", filePath),
				zap.Error(err))
			continue
		}

		if session != nil {
			newSessions = append(newSessions, session)
		}
	}

	// Update scan state and directory cache
	state.mu.Lock()
	state.sessions = append(state.sessions, newSessions...)
	state.pendingDirs = nil
	state.phase = ScanPhaseComplete

	// Sort all sessions
	allSessions := state.sessions
	sort.Slice(allSessions, func(i, j int) bool {
		ti := allSessions[i].LastMessageAt
		tj := allSessions[j].LastMessageAt
		if ti != nil && tj != nil {
			return ti.After(*tj)
		}
		if ti != nil {
			return true
		}
		if tj != nil {
			return false
		}
		return allSessions[i].SessionStartedAt.After(allSessions[j].SessionStartedAt)
	})
	state.mu.Unlock()

	// Update directory cache
	dirCacheMu.Lock()
	if cached, exists := dirCache[projectDir]; exists {
		cached.sessions = allSessions
		cached.cachedAt = time.Now()
	}
	dirCacheMu.Unlock()

	logger.Debug("completed extended phase scan for claude",
		zap.String("projectPath", projectPath),
		zap.Int("newSessions", len(newSessions)))

	return nil
}

// getClaudeCodeSessions returns Claude Code sessions for a project path.
// Deprecated: Use getClaudeCodeSessionsPhased instead
func (s *AISessionService) getClaudeCodeSessions(ctx context.Context, projectPath string) ([]*AISessionSummary, error) {
	sessions, _, err := s.getClaudeCodeSessionsPhased(ctx, projectPath)
	return sessions, err
}

// getClaudeCodeSessionsLegacy returns Claude Code sessions for a project path (legacy full scan).
func (s *AISessionService) getClaudeCodeSessionsLegacy(ctx context.Context, projectPath string) ([]*AISessionSummary, error) {
	logger := s.logger(ctx)

	// Create searcher to find Claude Code session directory
	searcher, err := log_watcher.NewClaudeCodeFileSearcher(projectPath)
	if err != nil {
		return nil, err
	}

	// Get the project-specific session directory
	encodedPath := encodePathForClaude(projectPath)
	projectDir := filepath.Join(searcher.GetSessionDir(), encodedPath)

	// Check if directory exists and get its mod time
	dirInfo, err := os.Stat(projectDir)
	if os.IsNotExist(err) {
		return nil, nil // No sessions for this project
	}
	if err != nil {
		return nil, err
	}
	dirModTime := dirInfo.ModTime()

	// Check directory cache - fast path
	dirCacheMu.RLock()
	cached, hasCached := dirCache[projectDir]
	dirCacheMu.RUnlock()

	if hasCached {
		// If within TTL, return cached without checking mod time
		if time.Since(cached.cachedAt) < dirCacheTTL {
			return cached.sessions, nil
		}
		// If mod time unchanged, refresh TTL and return cached
		if cached.modTime.Equal(dirModTime) {
			dirCacheMu.Lock()
			cached.cachedAt = time.Now()
			dirCacheMu.Unlock()
			return cached.sessions, nil
		}
	}

	// Directory changed or no cache - do full scan
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}

	db := model.GetDB()
	if db == nil {
		return nil, model.ErrDBNotInitialized
	}

	var sessions []*AISessionSummary

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip agent files and non-jsonl files
		if strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		filePath := filepath.Join(projectDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Extract session ID from filename (remove .jsonl extension)
		sessionID := strings.TrimSuffix(name, ".jsonl")

		// Check cache
		session, err := s.getOrUpdateClaudeSession(ctx, db, sessionID, filePath, info, projectPath)
		if err != nil {
			logger.Debug("failed to process session file",
				zap.String("file", filePath),
				zap.Error(err))
			continue
		}

		if session != nil {
			sessions = append(sessions, session)
		}
	}

	// Sort by last message time (newest first), fallback to session start time
	sort.Slice(sessions, func(i, j int) bool {
		ti := sessions[i].LastMessageAt
		tj := sessions[j].LastMessageAt
		if ti != nil && tj != nil {
			return ti.After(*tj)
		}
		if ti != nil {
			return true
		}
		if tj != nil {
			return false
		}
		return sessions[i].SessionStartedAt.After(sessions[j].SessionStartedAt)
	})

	// Update directory cache
	dirCacheMu.Lock()
	dirCache[projectDir] = &dirCacheEntry{
		modTime:  dirModTime,
		sessions: sessions,
		cachedAt: time.Now(),
	}
	dirCacheMu.Unlock()

	return sessions, nil
}

// getOrUpdateClaudeSession gets or updates a Claude Code session from cache.
func (s *AISessionService) getOrUpdateClaudeSession(
	ctx context.Context,
	db *gorm.DB,
	sessionID string,
	filePath string,
	fileInfo os.FileInfo,
	projectPath string,
) (*AISessionSummary, error) {
	// Check if we have a valid cached entry
	var cached tables.AISessionTable
	err := db.WithContext(ctx).
		Where("session_id = ? AND type = ?", sessionID, tables.AISessionTypeClaudeCode).
		First(&cached).Error

	if err == nil {
		// Check if cache is still valid (file hasn't changed)
		if cached.FileModTime.Equal(fileInfo.ModTime()) && cached.FileSize == fileInfo.Size() {
			return &AISessionSummary{
				ID:               cached.ID,
				SessionID:        cached.SessionID,
				Type:             string(cached.Type),
				Model:            cached.Model,
				Title:            cached.Title,
				SessionStartedAt: cached.SessionStartedAt,
				LastMessageAt:    cached.LastMessageAt,
				MessageCount:     cached.MessageCount,
				FilePath:         cached.FilePath,
			}, nil
		}
	}

	// Parse the session file
	sessionData, err := s.parseClaudeCodeSessionFile(filePath)
	if err != nil {
		return nil, err
	}

	// Create or update cache entry
	now := time.Now()
	record := tables.AISessionTable{
		SessionID:        sessionID,
		Type:             tables.AISessionTypeClaudeCode,
		ProjectPath:      projectPath,
		FilePath:         filePath,
		Model:            sessionData.Model,
		Title:            sessionData.Title,
		SessionStartedAt: sessionData.StartedAt,
		LastMessageAt:    sessionData.LastMessageAt,
		MessageCount:     sessionData.MessageCount,
		FileModTime:      fileInfo.ModTime(),
		FileSize:         fileInfo.Size(),
	}

	if cached.ID != "" {
		// Update existing
		record.ID = cached.ID
		record.CreatedAt = cached.CreatedAt
		record.UpdatedAt = now
		if err := db.WithContext(ctx).Save(&record).Error; err != nil {
			return nil, err
		}
	} else {
		// Create new
		record.ID = utils.NewID()
		record.CreatedAt = now
		record.UpdatedAt = now
		if err := db.WithContext(ctx).Create(&record).Error; err != nil {
			return nil, err
		}
	}

	return &AISessionSummary{
		ID:               record.ID,
		SessionID:        record.SessionID,
		Type:             string(record.Type),
		Model:            record.Model,
		Title:            record.Title,
		SessionStartedAt: record.SessionStartedAt,
		LastMessageAt:    record.LastMessageAt,
		MessageCount:     record.MessageCount,
		FilePath:         record.FilePath,
	}, nil
}

// claudeSessionData holds parsed data from a Claude Code session file.
type claudeSessionData struct {
	Model         string
	Title         string
	StartedAt     time.Time
	LastMessageAt *time.Time
	MessageCount  int
}

// parseClaudeCodeSessionFile parses a Claude Code session file to extract metadata.
func (s *AISessionService) parseClaudeCodeSessionFile(filePath string) (*claudeSessionData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data := &claudeSessionData{}
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		msg, sessionID, err := log_watcher.ParseClaudeCodeLine(line)
		if err != nil {
			continue
		}

		// Try to extract session start time from first valid entry
		if data.StartedAt.IsZero() && sessionID != "" {
			// Parse timestamp from the line
			var entry struct {
				Timestamp string `json:"timestamp"`
			}
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Timestamp != "" {
				if ts, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
					data.StartedAt = ts
				}
			}
		}

		// Count user messages and extract first message as title
		if msg != nil {
			data.MessageCount++
			data.LastMessageAt = &msg.Timestamp
			// Use first user message as title (truncate to 100 chars)
			if data.Title == "" && msg.Message != "" {
				title := strings.TrimSpace(msg.Message)
				if len(title) > 100 {
					title = title[:100] + "..."
				}
				data.Title = title
			}
		}

		// Try to extract model information
		var entry struct {
			Type    string `json:"type"`
			Message struct {
				Model string `json:"model"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil {
			if entry.Message.Model != "" {
				data.Model = entry.Message.Model
			}
		}
	}

	// If we couldn't determine start time, use file mod time
	if data.StartedAt.IsZero() {
		if info, err := os.Stat(filePath); err == nil {
			data.StartedAt = info.ModTime()
		}
	}

	return data, scanner.Err()
}

// getCodexSessionsPhased returns Codex sessions using phased scanning.
// Phase 1: Quickly return sessions from last 24 hours (day 0)
// Phase 2: Background scan for 1-15 days old sessions
// Files older than 15 days are ignored
func (s *AISessionService) getCodexSessionsPhased(ctx context.Context, projectPath string) ([]*AISessionSummary, string, error) {
	logger := s.logger(ctx)

	searcher, err := log_watcher.NewCodexFileSearcher()
	if err != nil {
		return nil, ScanPhaseComplete, err
	}

	sessionDir := searcher.GetSessionDir()

	// Check if sessions directory exists
	dirInfo, err := os.Stat(sessionDir)
	if os.IsNotExist(err) {
		return nil, ScanPhaseComplete, nil
	}
	if err != nil {
		return nil, ScanPhaseComplete, err
	}

	// Use project path + "codex" as cache key
	cacheKey := projectPath + ":codex"
	dirModTime := dirInfo.ModTime()

	// Check directory cache - fast path
	dirCacheMu.RLock()
	cached, hasCached := dirCache[cacheKey]
	dirCacheMu.RUnlock()

	if hasCached {
		// If within TTL, return cached without checking mod time
		if time.Since(cached.cachedAt) < dirCacheTTL {
			phase := s.getCodexScanPhase(projectPath)
			return cached.sessions, phase, nil
		}
		// For codex, we also check today's date directory for new sessions
		todayDir := filepath.Join(sessionDir, time.Now().Format("2006"), time.Now().Format("01"), time.Now().Format("02"))
		todayInfo, err := os.Stat(todayDir)
		todayUnchanged := err != nil || (todayInfo != nil && !todayInfo.ModTime().After(cached.cachedAt))

		// If base dir and today's dir unchanged, use cache
		if cached.modTime.Equal(dirModTime) && todayUnchanged {
			dirCacheMu.Lock()
			cached.cachedAt = time.Now()
			dirCacheMu.Unlock()
			phase := s.getCodexScanPhase(projectPath)
			return cached.sessions, phase, nil
		}
	}

	// Do phased scan
	db := model.GetDB()
	if db == nil {
		return nil, ScanPhaseComplete, model.ErrDBNotInitialized
	}

	// First, check if we have cached sessions for this project
	var cachedSessions []tables.AISessionTable
	if err := db.WithContext(ctx).
		Where("project_path = ? AND type = ?", projectPath, tables.AISessionTypeCodex).
		Find(&cachedSessions).Error; err != nil {
		logger.Debug("failed to query cached codex sessions", zap.Error(err))
	}

	// Build a map of cached sessions for quick lookup
	cachedMap := make(map[string]*tables.AISessionTable)
	for i := range cachedSessions {
		cachedMap[cachedSessions[i].SessionID] = &cachedSessions[i]
	}

	var sessions []*AISessionSummary
	var extendedDays []int // Days for background scan (1-15 days ago)

	// Phase 1: Only scan today's directory (day 0) for immediate results
	now := time.Now()
	todayDir := filepath.Join(sessionDir, now.Format("2006"), now.Format("01"), now.Format("02"))

	if _, err := os.Stat(todayDir); err == nil {
		entries, err := os.ReadDir(todayDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				name := entry.Name()
				if !strings.HasPrefix(name, log_watcher.CodexRolloutPrefix) ||
					!strings.HasSuffix(name, log_watcher.CodexRolloutSuffix) {
					continue
				}

				filePath := filepath.Join(todayDir, name)
				info, err := entry.Info()
				if err != nil {
					continue
				}

				sessionID := log_watcher.ExtractSessionIDFromFilename(name)
				if sessionID == "" {
					continue
				}

				// Check cache first
				if dbCached, ok := cachedMap[sessionID]; ok {
					if dbCached.FileModTime.Equal(info.ModTime()) && dbCached.FileSize == info.Size() {
						if dbCached.ProjectPath == projectPath {
							sessions = append(sessions, &AISessionSummary{
								ID:               dbCached.ID,
								SessionID:        dbCached.SessionID,
								Type:             string(dbCached.Type),
								Model:            dbCached.Model,
								Title:            dbCached.Title,
								SessionStartedAt: dbCached.SessionStartedAt,
								LastMessageAt:    dbCached.LastMessageAt,
								MessageCount:     dbCached.MessageCount,
								FilePath:         dbCached.FilePath,
							})
						}
						continue
					}
				}

				// Parse the session file
				sessionData, err := s.parseCodexSessionFile(filePath)
				if err != nil {
					continue
				}

				if sessionData.Cwd != projectPath {
					continue
				}

				session, err := s.saveCodexSession(ctx, db, sessionID, filePath, info, sessionData)
				if err != nil {
					continue
				}

				sessions = append(sessions, session)
			}
		}
	}

	// Check for days 1-15 - add cached sessions and queue uncached for background
	for daysAgo := 1; daysAgo <= 15; daysAgo++ {
		date := now.AddDate(0, 0, -daysAgo)
		dateDir := filepath.Join(sessionDir, date.Format("2006"), date.Format("01"), date.Format("02"))

		if _, err := os.Stat(dateDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(dateDir)
		if err != nil {
			continue
		}

		hasUncached := false
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasPrefix(name, log_watcher.CodexRolloutPrefix) ||
				!strings.HasSuffix(name, log_watcher.CodexRolloutSuffix) {
				continue
			}

			sessionID := log_watcher.ExtractSessionIDFromFilename(name)
			if sessionID == "" {
				continue
			}

			// Check cache
			if dbCached, ok := cachedMap[sessionID]; ok {
				info, err := entry.Info()
				if err == nil && dbCached.FileModTime.Equal(info.ModTime()) && dbCached.FileSize == info.Size() {
					if dbCached.ProjectPath == projectPath {
						sessions = append(sessions, &AISessionSummary{
							ID:               dbCached.ID,
							SessionID:        dbCached.SessionID,
							Type:             string(dbCached.Type),
							Model:            dbCached.Model,
							Title:            dbCached.Title,
							SessionStartedAt: dbCached.SessionStartedAt,
							LastMessageAt:    dbCached.LastMessageAt,
							MessageCount:     dbCached.MessageCount,
							FilePath:         dbCached.FilePath,
						})
					}
					continue
				}
			}

			// Has uncached files - need to scan this day in background
			hasUncached = true
		}

		if hasUncached {
			extendedDays = append(extendedDays, daysAgo)
		}
	}

	// Sort by last message time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		ti := sessions[i].LastMessageAt
		tj := sessions[j].LastMessageAt
		if ti != nil && tj != nil {
			return ti.After(*tj)
		}
		if ti != nil {
			return true
		}
		if tj != nil {
			return false
		}
		return sessions[i].SessionStartedAt.After(sessions[j].SessionStartedAt)
	})

	// Determine scan phase
	var phase string
	if len(extendedDays) > 0 {
		phase = ScanPhaseRecent

		// Store pending days in scan state
		scanStatesMu.Lock()
		state, exists := scanStates[projectPath+":codex"]
		if !exists {
			// Convert days to strings for storage
			pendingDirs := make([]string, len(extendedDays))
			for i, day := range extendedDays {
				date := now.AddDate(0, 0, -day)
				pendingDirs[i] = filepath.Join(sessionDir, date.Format("2006"), date.Format("01"), date.Format("02"))
			}
			state = &scanState{
				phase:       ScanPhaseRecent,
				sessions:    sessions,
				pendingDirs: pendingDirs,
			}
			scanStates[projectPath+":codex"] = state
		} else {
			state.mu.Lock()
			pendingDirs := make([]string, len(extendedDays))
			for i, day := range extendedDays {
				date := now.AddDate(0, 0, -day)
				pendingDirs[i] = filepath.Join(sessionDir, date.Format("2006"), date.Format("01"), date.Format("02"))
			}
			state.pendingDirs = pendingDirs
			state.sessions = sessions
			state.phase = ScanPhaseRecent
			state.mu.Unlock()
		}
		scanStatesMu.Unlock()

		// Queue background scan
		select {
		case bgScanQueue <- &bgScanTask{
			projectPath: projectPath,
			scanType:    "codex",
		}:
		default:
			logger.Debug("background scan queue full, skipping codex")
		}
	} else {
		phase = ScanPhaseComplete
		scanStatesMu.Lock()
		if state, exists := scanStates[projectPath+":codex"]; exists {
			state.mu.Lock()
			state.phase = ScanPhaseComplete
			state.mu.Unlock()
		}
		scanStatesMu.Unlock()
	}

	// Update directory cache
	dirCacheMu.Lock()
	dirCache[cacheKey] = &dirCacheEntry{
		modTime:  dirModTime,
		sessions: sessions,
		cachedAt: time.Now(),
	}
	dirCacheMu.Unlock()

	return sessions, phase, nil
}

// getCodexScanPhase returns the current scan phase for a project's codex sessions
func (s *AISessionService) getCodexScanPhase(projectPath string) string {
	scanStatesMu.RLock()
	defer scanStatesMu.RUnlock()

	if state, exists := scanStates[projectPath+":codex"]; exists {
		state.mu.RLock()
		defer state.mu.RUnlock()
		return state.phase
	}
	return ScanPhaseComplete
}

// scanCodexExtendedPhase processes extended phase directories in background
func (s *AISessionService) scanCodexExtendedPhase(ctx context.Context, projectPath string) error {
	logger := s.logger(ctx)

	scanStatesMu.RLock()
	state, exists := scanStates[projectPath+":codex"]
	scanStatesMu.RUnlock()

	if !exists {
		return nil
	}

	state.mu.RLock()
	pendingDirs := make([]string, len(state.pendingDirs))
	copy(pendingDirs, state.pendingDirs)
	state.mu.RUnlock()

	if len(pendingDirs) == 0 {
		return nil
	}

	db := model.GetDB()
	if db == nil {
		return model.ErrDBNotInitialized
	}

	var newSessions []*AISessionSummary

	for _, dateDir := range pendingDirs {
		entries, err := os.ReadDir(dateDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasPrefix(name, log_watcher.CodexRolloutPrefix) ||
				!strings.HasSuffix(name, log_watcher.CodexRolloutSuffix) {
				continue
			}

			filePath := filepath.Join(dateDir, name)
			info, err := entry.Info()
			if err != nil {
				continue
			}

			sessionID := log_watcher.ExtractSessionIDFromFilename(name)
			if sessionID == "" {
				continue
			}

			// Check if already cached
			var cached tables.AISessionTable
			err = db.WithContext(ctx).
				Where("session_id = ? AND type = ?", sessionID, tables.AISessionTypeCodex).
				First(&cached).Error
			if err == nil && cached.FileModTime.Equal(info.ModTime()) && cached.FileSize == info.Size() {
				if cached.ProjectPath == projectPath {
					newSessions = append(newSessions, &AISessionSummary{
						ID:               cached.ID,
						SessionID:        cached.SessionID,
						Type:             string(cached.Type),
						Model:            cached.Model,
						Title:            cached.Title,
						SessionStartedAt: cached.SessionStartedAt,
						LastMessageAt:    cached.LastMessageAt,
						MessageCount:     cached.MessageCount,
						FilePath:         cached.FilePath,
					})
				}
				continue
			}

			// Parse the session file
			sessionData, err := s.parseCodexSessionFile(filePath)
			if err != nil {
				continue
			}

			if sessionData.Cwd != projectPath {
				continue
			}

			session, err := s.saveCodexSession(ctx, db, sessionID, filePath, info, sessionData)
			if err != nil {
				continue
			}

			newSessions = append(newSessions, session)
		}
	}

	// Update scan state
	state.mu.Lock()
	state.sessions = append(state.sessions, newSessions...)
	state.pendingDirs = nil
	state.phase = ScanPhaseComplete

	// Sort all sessions
	allSessions := state.sessions
	sort.Slice(allSessions, func(i, j int) bool {
		ti := allSessions[i].LastMessageAt
		tj := allSessions[j].LastMessageAt
		if ti != nil && tj != nil {
			return ti.After(*tj)
		}
		if ti != nil {
			return true
		}
		if tj != nil {
			return false
		}
		return allSessions[i].SessionStartedAt.After(allSessions[j].SessionStartedAt)
	})
	state.mu.Unlock()

	// Update directory cache
	cacheKey := projectPath + ":codex"
	dirCacheMu.Lock()
	if cached, exists := dirCache[cacheKey]; exists {
		cached.sessions = allSessions
		cached.cachedAt = time.Now()
	}
	dirCacheMu.Unlock()

	logger.Debug("completed extended phase scan for codex",
		zap.String("projectPath", projectPath),
		zap.Int("newSessions", len(newSessions)))

	return nil
}

// getCodexSessions returns Codex sessions for a project path.
// Deprecated: Use getCodexSessionsPhased instead
func (s *AISessionService) getCodexSessions(ctx context.Context, projectPath string) ([]*AISessionSummary, error) {
	sessions, _, err := s.getCodexSessionsPhased(ctx, projectPath)
	return sessions, err
}

// codexSessionData holds parsed data from a Codex session file.
type codexSessionData struct {
	Cwd           string
	Model         string
	Title         string
	StartedAt     time.Time
	LastMessageAt *time.Time
	MessageCount  int
}

// parseCodexSessionFile parses a Codex session file to extract metadata.
func (s *AISessionService) parseCodexSessionFile(filePath string) (*codexSessionData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data := &codexSessionData{}
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry log_watcher.LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		switch entry.Type {
		case "session_meta":
			payload, ok := entry.Payload.(map[string]interface{})
			if !ok {
				continue
			}
			if cwd, ok := payload["cwd"].(string); ok {
				data.Cwd = cwd
			}
			if ts, ok := payload["timestamp"].(string); ok {
				if t, err := time.Parse(time.RFC3339, ts); err == nil {
					data.StartedAt = t
				}
			}

		case "turn_context":
			payload, ok := entry.Payload.(map[string]interface{})
			if !ok {
				continue
			}
			if model, ok := payload["model"].(string); ok && data.Model == "" {
				data.Model = model
			}

		case "event_msg":
			payload, ok := entry.Payload.(map[string]interface{})
			if !ok {
				continue
			}
			if msgType, ok := payload["type"].(string); ok && msgType == "user_message" {
				data.MessageCount++
				if ts, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
					data.LastMessageAt = &ts
				}
				// Extract first user message as title
				if data.Title == "" {
					if msg, ok := payload["message"].(string); ok && msg != "" {
						title := strings.TrimSpace(msg)
						if len(title) > 100 {
							title = title[:100] + "..."
						}
						data.Title = title
					}
				}
			}
		}
	}

	return data, scanner.Err()
}

// saveCodexSession saves a Codex session to the database cache.
func (s *AISessionService) saveCodexSession(
	ctx context.Context,
	db *gorm.DB,
	sessionID string,
	filePath string,
	fileInfo os.FileInfo,
	data *codexSessionData,
) (*AISessionSummary, error) {
	now := time.Now()

	// Check if record exists
	var existing tables.AISessionTable
	err := db.WithContext(ctx).
		Where("session_id = ? AND type = ?", sessionID, tables.AISessionTypeCodex).
		First(&existing).Error

	record := tables.AISessionTable{
		SessionID:        sessionID,
		Type:             tables.AISessionTypeCodex,
		ProjectPath:      data.Cwd,
		FilePath:         filePath,
		Model:            data.Model,
		Title:            data.Title,
		SessionStartedAt: data.StartedAt,
		LastMessageAt:    data.LastMessageAt,
		MessageCount:     data.MessageCount,
		FileModTime:      fileInfo.ModTime(),
		FileSize:         fileInfo.Size(),
	}

	if err == nil {
		// Update existing
		record.ID = existing.ID
		record.CreatedAt = existing.CreatedAt
		record.UpdatedAt = now
		if err := db.WithContext(ctx).Save(&record).Error; err != nil {
			return nil, err
		}
	} else {
		// Create new
		record.ID = utils.NewID()
		record.CreatedAt = now
		record.UpdatedAt = now
		if err := db.WithContext(ctx).Create(&record).Error; err != nil {
			return nil, err
		}
	}

	return &AISessionSummary{
		ID:               record.ID,
		SessionID:        record.SessionID,
		Type:             string(record.Type),
		Model:            record.Model,
		Title:            record.Title,
		SessionStartedAt: record.SessionStartedAt,
		LastMessageAt:    record.LastMessageAt,
		MessageCount:     record.MessageCount,
		FilePath:         record.FilePath,
	}, nil
}

// CleanupStaleSessions removes cached sessions whose files no longer exist.
func (s *AISessionService) CleanupStaleSessions(ctx context.Context) (int64, error) {
	ctx = ensureContext(ctx)
	logger := s.logger(ctx)

	db := model.GetDB()
	if db == nil {
		return 0, model.ErrDBNotInitialized
	}

	var sessions []tables.AISessionTable
	if err := db.WithContext(ctx).Find(&sessions).Error; err != nil {
		return 0, err
	}

	var deletedCount int64
	for _, session := range sessions {
		if _, err := os.Stat(session.FilePath); os.IsNotExist(err) {
			if err := db.WithContext(ctx).Delete(&session).Error; err != nil {
				logger.Warn("failed to delete stale session",
					zap.String("sessionId", session.SessionID),
					zap.Error(err))
				continue
			}
			deletedCount++
		}
	}

	if deletedCount > 0 {
		logger.Info("cleaned up stale AI sessions", zap.Int64("count", deletedCount))
	}

	return deletedCount, nil
}

func (s *AISessionService) logger(ctx context.Context) *zap.Logger {
	return utils.LoggerFromContext(ctx).Named("ai-session-service")
}

// ConversationMessage represents a single message in a conversation.
type ConversationMessage struct {
	Role      string    `json:"role"`      // "user" or "assistant"
	Content   string    `json:"content"`   // Message content
	Timestamp time.Time `json:"timestamp"` // Message timestamp
}

// ConversationResponse contains the full conversation for a session.
type ConversationResponse struct {
	SessionID string                 `json:"sessionId"`
	Title     string                 `json:"title"`
	Messages  []*ConversationMessage `json:"messages"`
}

// GetSessionConversation retrieves the full conversation for a given database ID.
func (s *AISessionService) GetSessionConversation(ctx context.Context, dbID string) (*ConversationResponse, error) {
	return s.getConversationByQuery(ctx, "id = ?", dbID)
}

// GetSessionConversationBySessionID retrieves the full conversation for a given session ID (UUID).
func (s *AISessionService) GetSessionConversationBySessionID(ctx context.Context, sessionID string) (*ConversationResponse, error) {
	return s.getConversationByQuery(ctx, "session_id = ?", sessionID)
}

// getConversationByQuery retrieves conversation using a custom where clause.
func (s *AISessionService) getConversationByQuery(ctx context.Context, query string, args ...interface{}) (*ConversationResponse, error) {
	ctx = ensureContext(ctx)
	logger := s.logger(ctx)

	db := model.GetDB()
	if db == nil {
		return nil, model.ErrDBNotInitialized
	}

	// Find the session in database
	var session tables.AISessionTable
	err := db.WithContext(ctx).Where(query, args...).First(&session).Error
	if err != nil {
		logger.Debug("session not found", zap.String("query", query), zap.Error(err))
		return nil, err
	}

	// Parse the session file based on type
	var messages []*ConversationMessage

	switch session.Type {
	case tables.AISessionTypeClaudeCode:
		messages, err = s.parseClaudeCodeConversation(session.FilePath)
	case tables.AISessionTypeCodex:
		messages, err = s.parseCodexConversation(session.FilePath)
	default:
		return nil, errors.New("unknown session type")
	}

	if err != nil {
		logger.Error("failed to parse conversation", zap.String("filePath", session.FilePath), zap.Error(err))
		return nil, err
	}

	return &ConversationResponse{
		SessionID: session.SessionID,
		Title:     session.Title,
		Messages:  messages,
	}, nil
}

// parseClaudeCodeConversation parses a Claude Code session file and extracts messages.
func (s *AISessionService) parseClaudeCodeConversation(filePath string) ([]*ConversationMessage, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var messages []*ConversationMessage
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024) // 2MB buffer for large lines

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse the entry
		var entry struct {
			Type      string          `json:"type"`
			Message   json.RawMessage `json:"message"`
			Timestamp string          `json:"timestamp"`
			IsMeta    bool            `json:"isMeta"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		ts, _ := time.Parse(time.RFC3339, entry.Timestamp)

		// Handle user messages
		if entry.Type == "user" && !entry.IsMeta {
			var msgContent struct {
				Role    string      `json:"role"`
				Content interface{} `json:"content"`
			}
			if err := json.Unmarshal(entry.Message, &msgContent); err != nil {
				continue
			}

			if msgContent.Role == "user" {
				var textContent string
				switch content := msgContent.Content.(type) {
				case string:
					textContent = content
				}
				if textContent != "" && !strings.HasPrefix(textContent, "<command-") && !strings.HasPrefix(textContent, "<local-command") {
					messages = append(messages, &ConversationMessage{
						Role:      "user",
						Content:   textContent,
						Timestamp: ts,
					})
				}
			}
		}

		// Handle assistant messages
		if entry.Type == "assistant" {
			var msgContent struct {
				Role    string      `json:"role"`
				Content interface{} `json:"content"`
			}
			if err := json.Unmarshal(entry.Message, &msgContent); err != nil {
				continue
			}

			if msgContent.Role == "assistant" {
				var textContent string
				switch content := msgContent.Content.(type) {
				case string:
					textContent = content
				case []interface{}:
					// Extract text from content blocks
					for _, block := range content {
						if blockMap, ok := block.(map[string]interface{}); ok {
							if blockType, ok := blockMap["type"].(string); ok && blockType == "text" {
								if text, ok := blockMap["text"].(string); ok {
									textContent += text
								}
							}
						}
					}
				}
				if textContent != "" {
					messages = append(messages, &ConversationMessage{
						Role:      "assistant",
						Content:   textContent,
						Timestamp: ts,
					})
				}
			}
		}
	}

	return messages, scanner.Err()
}

// parseCodexConversation parses a Codex session file and extracts messages.
func (s *AISessionService) parseCodexConversation(filePath string) ([]*ConversationMessage, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var messages []*ConversationMessage
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024) // 2MB buffer for large lines

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry log_watcher.LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		ts, _ := time.Parse(time.RFC3339, entry.Timestamp)

		if entry.Type == "event_msg" {
			payload, ok := entry.Payload.(map[string]interface{})
			if !ok {
				continue
			}

			msgType, _ := payload["type"].(string)
			msg, _ := payload["message"].(string)

			if msg == "" {
				continue
			}

			switch msgType {
			case "user_message":
				messages = append(messages, &ConversationMessage{
					Role:      "user",
					Content:   msg,
					Timestamp: ts,
				})
			case "assistant_message":
				messages = append(messages, &ConversationMessage{
					Role:      "assistant",
					Content:   msg,
					Timestamp: ts,
				})
			}
		}
	}

	return messages, scanner.Err()
}

// encodePathForClaude converts a path to Claude Code's folder naming convention.
// Example: D:\codes\2025\aicode-kanban -> D--codes-2025-aicode-kanban
func encodePathForClaude(path string) string {
	path = filepath.Clean(path)
	path = filepath.ToSlash(path)
	path = strings.TrimRight(path, "/\\")
	path = strings.ReplaceAll(path, ":", "-")
	path = strings.ReplaceAll(path, "/", "-")
	return path
}
