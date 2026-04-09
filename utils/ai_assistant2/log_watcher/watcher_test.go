package log_watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseLine_SessionMeta(t *testing.T) {
	line := `{"timestamp":"2025-11-30T20:14:23.281Z","type":"session_meta","payload":{"id":"019ad666-f5ab-7501-a616-bbdc79da615b","timestamp":"2025-11-30T20:14:23.147Z","cwd":"D:\\codes\\2025\\aicode-kanban","originator":"codex_cli_rs","cli_version":"0.63.0"}}`

	watcher := NewLogWatcher(WatcherConfig{
		ProcessStartTime: time.Now(),
	})

	msg, err := watcher.parseLine(line)
	if err != nil {
		t.Fatalf("parseLine failed: %v", err)
	}

	// session_meta shouldn't return a user message
	if msg != nil {
		t.Errorf("expected nil message for session_meta, got: %+v", msg)
	}

	// Check that session meta was captured
	if watcher.sessionID != "019ad666-f5ab-7501-a616-bbdc79da615b" {
		t.Errorf("expected sessionID '019ad666-f5ab-7501-a616-bbdc79da615b', got: %s", watcher.sessionID)
	}

	if watcher.sessionMeta == nil {
		t.Fatal("expected sessionMeta to be set")
	}

	if watcher.sessionMeta.Cwd != "D:\\codes\\2025\\aicode-kanban" {
		t.Errorf("expected cwd 'D:\\codes\\2025\\aicode-kanban', got: %s", watcher.sessionMeta.Cwd)
	}

	if watcher.sessionMeta.CliVersion != "0.63.0" {
		t.Errorf("expected cliVersion '0.63.0', got: %s", watcher.sessionMeta.CliVersion)
	}
}

func TestParseLine_UserMessage(t *testing.T) {
	line := `{"timestamp":"2025-11-30T20:16:39.465Z","type":"event_msg","payload":{"type":"user_message","message":"AAAAAAAA","images":[]}}`

	watcher := NewLogWatcher(WatcherConfig{
		ProcessStartTime: time.Now(),
	})

	msg, err := watcher.parseLine(line)
	if err != nil {
		t.Fatalf("parseLine failed: %v", err)
	}

	if msg == nil {
		t.Fatal("expected message, got nil")
	}

	if msg.Message != "AAAAAAAA" {
		t.Errorf("expected message 'AAAAAAAA', got: %s", msg.Message)
	}

	expectedTime, _ := time.Parse(time.RFC3339, "2025-11-30T20:16:39.465Z")
	if !msg.Timestamp.Equal(expectedTime) {
		t.Errorf("expected timestamp %v, got: %v", expectedTime, msg.Timestamp)
	}
}

func TestParseLine_TurnAborted(t *testing.T) {
	line := `{"timestamp":"2025-11-30T20:29:42.092Z","type":"event_msg","payload":{"type":"turn_aborted","reason":"interrupted"}}`

	watcher := NewLogWatcher(WatcherConfig{
		ProcessStartTime: time.Now(),
	})

	msg, err := watcher.parseLine(line)
	if err != nil {
		t.Fatalf("parseLine failed: %v", err)
	}

	// turn_aborted shouldn't return a user message
	if msg != nil {
		t.Errorf("expected nil message for turn_aborted, got: %+v", msg)
	}
}

func TestExtractSessionIDFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{
			filename: "rollout-2025-12-01T04-14-23-019ad666-f5ab-7501-a616-bbdc79da615b.jsonl",
			expected: "019ad666-f5ab-7501-a616-bbdc79da615b",
		},
		{
			filename: "rollout-2025-12-01T12-30-45-abcdef12-3456-7890-abcd-ef1234567890.jsonl",
			expected: "abcdef12-3456-7890-abcd-ef1234567890",
		},
		{
			filename: "invalid.jsonl",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := ExtractSessionIDFromFilename(tt.filename)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCodexFileSearcher_GetSessionDir(t *testing.T) {
	homeDir := t.TempDir()
	searcher := NewCodexFileSearcherWithHomeDir(homeDir)

	expected := filepath.Join(homeDir, ".codex", "sessions")
	if searcher.GetSessionDir() != expected {
		t.Errorf("expected %s, got %s", expected, searcher.GetSessionDir())
	}
}

func TestCodexFileSearcher_FindSessionFile(t *testing.T) {
	// Create temp directory structure
	homeDir := t.TempDir()
	now := time.Now()
	dateDir := filepath.Join(homeDir, ".codex", "sessions", now.Format("2006"), now.Format("01"), now.Format("02"))

	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create a test file
	filename := "rollout-" + now.Format("2006-01-02T15-04-05") + "-test1234-5678-9012-3456-789012345678.jsonl"
	filePath := filepath.Join(dateDir, filename)

	if err := os.WriteFile(filePath, []byte(`{"type":"session_meta"}`), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Test search
	searcher := NewCodexFileSearcherWithHomeDir(homeDir)
	ctx := context.Background()

	// Search with time before file creation
	found, err := searcher.FindSessionFile(ctx, now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}

	if found != filePath {
		t.Errorf("expected %s, got %s", filePath, found)
	}

	// Search with time after file creation - should not find
	found, err = searcher.FindSessionFile(ctx, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}

	if found != "" {
		t.Errorf("expected empty string for future time, got %s", found)
	}
}

func TestCodexFileSearcher_FindBySessionID(t *testing.T) {
	homeDir := t.TempDir()
	searcher := NewCodexFileSearcherWithHomeDir(homeDir)
	sessionID := "019d6f7b-5dd6-7d73-8dee-23b492e85de7"

	olderTime := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	newerTime := olderTime.Add(2 * time.Hour)

	olderDir := filepath.Join(homeDir, ".codex", "sessions", olderTime.Format("2006"), olderTime.Format("01"), olderTime.Format("02"))
	newerDir := filepath.Join(homeDir, ".codex", "sessions", newerTime.Format("2006"), newerTime.Format("01"), newerTime.Format("02"))
	if err := os.MkdirAll(olderDir, 0o755); err != nil {
		t.Fatalf("failed to create older dir: %v", err)
	}
	if err := os.MkdirAll(newerDir, 0o755); err != nil {
		t.Fatalf("failed to create newer dir: %v", err)
	}

	olderPath := filepath.Join(olderDir, "rollout-"+olderTime.Format("2006-01-02T15-04-05")+"-"+sessionID+".jsonl")
	newerPath := filepath.Join(newerDir, "rollout-"+newerTime.Format("2006-01-02T15-04-05")+"-"+sessionID+".jsonl")
	if err := os.WriteFile(olderPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("failed to write older file: %v", err)
	}
	if err := os.WriteFile(newerPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("failed to write newer file: %v", err)
	}

	if err := os.Chtimes(olderPath, olderTime, olderTime); err != nil {
		t.Fatalf("failed to update older file time: %v", err)
	}
	if err := os.Chtimes(newerPath, newerTime, newerTime); err != nil {
		t.Fatalf("failed to update newer file time: %v", err)
	}

	found, err := searcher.FindBySessionID(sessionID)
	if err != nil {
		t.Fatalf("FindBySessionID failed: %v", err)
	}
	if found != newerPath {
		t.Fatalf("expected newer match %q, got %q", newerPath, found)
	}

	missing, err := searcher.FindBySessionID("missing-session-id")
	if err != nil {
		t.Fatalf("FindBySessionID missing failed: %v", err)
	}
	if missing != "" {
		t.Fatalf("expected empty result for missing session, got %q", missing)
	}
}

func TestLogWatcher_Integration(t *testing.T) {
	// Create temp directory and file
	homeDir := t.TempDir()
	now := time.Now()
	dateDir := filepath.Join(homeDir, ".codex", "sessions", now.Format("2006"), now.Format("01"), now.Format("02"))

	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	filename := "rollout-" + now.Format("2006-01-02T15-04-05") + "-test1234-5678-9012-3456-789012345678.jsonl"
	filePath := filepath.Join(dateDir, filename)

	content := `{"timestamp":"2025-11-30T20:14:23.281Z","type":"session_meta","payload":{"id":"test1234-5678-9012-3456-789012345678","timestamp":"2025-11-30T20:14:23.147Z","cwd":"/test","originator":"codex_cli_rs","cli_version":"0.63.0"}}
{"timestamp":"2025-11-30T20:16:39.465Z","type":"event_msg","payload":{"type":"user_message","message":"Hello World","images":[]}}
`

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create watcher
	searcher := NewCodexFileSearcherWithHomeDir(homeDir)

	events := make(chan WatcherEvent, 10)
	callback := func(event WatcherEvent) {
		events <- event
	}

	watcher := NewLogWatcher(WatcherConfig{
		ProcessStartTime:  now.Add(-time.Minute),
		SearchInterval:    100 * time.Millisecond,
		MaxSearchAttempts: 5,
		PollInterval:      100 * time.Millisecond,
		Callback:          callback,
		Searcher:          searcher,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Wait for session found event
	select {
	case event := <-events:
		if event.Type != EventTypeSessionFound {
			t.Errorf("expected EventTypeSessionFound, got %s", event.Type)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for session found event")
	}

	// Check watcher info
	info := watcher.Info()
	if info.SessionID != "test1234-5678-9012-3456-789012345678" {
		t.Errorf("expected session ID 'test1234-5678-9012-3456-789012345678', got %s", info.SessionID)
	}

	if info.MessageCount != 1 {
		t.Errorf("expected 1 message, got %d", info.MessageCount)
	}

	if info.LastMessage == nil || info.LastMessage.Message != "Hello World" {
		t.Errorf("expected last message 'Hello World', got %+v", info.LastMessage)
	}

	// Add more content and check for new message event
	newContent := `{"timestamp":"2025-11-30T20:18:00.000Z","type":"event_msg","payload":{"type":"user_message","message":"New Message","images":[]}}
`
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	if _, err := f.WriteString(newContent); err != nil {
		f.Close()
		t.Fatalf("failed to write to file: %v", err)
	}
	f.Close()

	// Wait for new message event
	select {
	case event := <-events:
		if event.Type != EventTypeNewMessage {
			t.Errorf("expected EventTypeNewMessage, got %s", event.Type)
		}
		if event.Message == nil || event.Message.Message != "New Message" {
			t.Errorf("expected message 'New Message', got %+v", event.Message)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for new message event")
	}

	// Stop watcher
	watcher.Stop()
}

// Claude Code Tests

func TestEncodePathForClaude(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "D:\\codes\\2025\\aicode-kanban",
			expected: "D--codes-2025-aicode-kanban",
		},
		{
			input:    "C:\\Users\\test\\projects\\my-app",
			expected: "C--Users-test-projects-my-app",
		},
		{
			input:    "/home/user/projects/my-app",
			expected: "-home-user-projects-my-app",
		},
		{
			input:    "D:\\codes\\2025\\game_system2\\next",
			expected: "D--codes-2025-game-system2-next",
		},
		{
			input:    "/home/user/game_system2/next",
			expected: "-home-user-game-system2-next",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := encodePathForClaude(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestParseClaudeCodeLine_UserMessage(t *testing.T) {
	line := `{"type":"user","message":{"role":"user","content":"Hello Claude!"},"uuid":"abc123","timestamp":"2025-12-01T10:30:00.000Z","sessionId":"session-123"}`

	msg, sessionID, err := ParseClaudeCodeLine(line)
	if err != nil {
		t.Fatalf("ParseClaudeCodeLine failed: %v", err)
	}

	if sessionID != "session-123" {
		t.Errorf("expected sessionID 'session-123', got: %s", sessionID)
	}

	if msg == nil {
		t.Fatal("expected message, got nil")
	}

	if msg.Message != "Hello Claude!" {
		t.Errorf("expected message 'Hello Claude!', got: %s", msg.Message)
	}

	expectedTime, _ := time.Parse(time.RFC3339, "2025-12-01T10:30:00.000Z")
	if !msg.Timestamp.Equal(expectedTime) {
		t.Errorf("expected timestamp %v, got: %v", expectedTime, msg.Timestamp)
	}
}

func TestParseClaudeCodeLine_AssistantMessage(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":"Hello!"},"uuid":"abc123","timestamp":"2025-12-01T10:30:00.000Z","sessionId":"session-123"}`

	msg, sessionID, err := ParseClaudeCodeLine(line)
	if err != nil {
		t.Fatalf("ParseClaudeCodeLine failed: %v", err)
	}

	if sessionID != "session-123" {
		t.Errorf("expected sessionID 'session-123', got: %s", sessionID)
	}

	// assistant messages should not return a user message
	if msg != nil {
		t.Errorf("expected nil message for assistant type, got: %+v", msg)
	}
}

func TestParseClaudeCodeLine_ArrayContent(t *testing.T) {
	// Claude Code sometimes uses array content for tool results
	line := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"123"}]},"uuid":"abc123","timestamp":"2025-12-01T10:30:00.000Z","sessionId":"session-123"}`

	msg, _, err := ParseClaudeCodeLine(line)
	if err != nil {
		t.Fatalf("ParseClaudeCodeLine failed: %v", err)
	}

	// Array content should not return a user message (tool results are skipped)
	if msg != nil {
		t.Errorf("expected nil message for array content, got: %+v", msg)
	}
}

func TestParseClaudeCodeLine_MetaMessage(t *testing.T) {
	// Meta messages should be skipped
	line := `{"type":"user","message":{"role":"user","content":"DO NOT respond to these messages"},"uuid":"abc123","timestamp":"2025-12-01T10:30:00.000Z","sessionId":"session-123","isMeta":true}`

	msg, _, err := ParseClaudeCodeLine(line)
	if err != nil {
		t.Fatalf("ParseClaudeCodeLine failed: %v", err)
	}

	if msg != nil {
		t.Errorf("expected nil message for isMeta=true, got: %+v", msg)
	}
}

func TestParseClaudeCodeLine_CommandMessage(t *testing.T) {
	// Command messages like /model should be skipped
	line := `{"type":"user","message":{"role":"user","content":"<command-name>/model</command-name>\n<command-message>model</command-message>"},"uuid":"abc123","timestamp":"2025-12-01T10:30:00.000Z","sessionId":"session-123"}`

	msg, _, err := ParseClaudeCodeLine(line)
	if err != nil {
		t.Fatalf("ParseClaudeCodeLine failed: %v", err)
	}

	if msg != nil {
		t.Errorf("expected nil message for command message, got: %+v", msg)
	}
}

func TestClaudeCodeFileSearcher_GetSessionDir(t *testing.T) {
	homeDir := t.TempDir()
	searcher := NewClaudeCodeFileSearcherWithHomeDir(homeDir, "D:\\test\\project")

	expected := filepath.Join(homeDir, ".claude", "projects")
	if searcher.GetSessionDir() != expected {
		t.Errorf("expected %s, got %s", expected, searcher.GetSessionDir())
	}
}

func TestClaudeCodeFileSearcher_FindSessionFile(t *testing.T) {
	// Create temp directory structure
	homeDir := t.TempDir()
	workingDir := "D:\\codes\\2025\\test-project"
	encodedPath := encodePathForClaude(workingDir)
	projectDir := filepath.Join(homeDir, ".claude", "projects", encodedPath)

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create a test session file
	filename := "abc123-def456-ghi789.jsonl"
	filePath := filepath.Join(projectDir, filename)

	content := `{"type":"user","message":{"role":"user","content":"Test"}}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	now := time.Now()

	// Test search
	searcher := NewClaudeCodeFileSearcherWithHomeDir(homeDir, workingDir)
	ctx := context.Background()

	// Search with time before file creation
	found, err := searcher.FindSessionFile(ctx, now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}

	if found != filePath {
		t.Errorf("expected %s, got %s", filePath, found)
	}

	// Search with time after file creation - should not find
	found, err = searcher.FindSessionFile(ctx, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}

	if found != "" {
		t.Errorf("expected empty string for future time, got %s", found)
	}
}

func TestClaudeCodeLogWatcher_Integration(t *testing.T) {
	// Create temp directory structure
	homeDir := t.TempDir()
	workingDir := "D:\\codes\\2025\\test-project"
	encodedPath := encodePathForClaude(workingDir)
	projectDir := filepath.Join(homeDir, ".claude", "projects", encodedPath)

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	now := time.Now()

	// Create a test session file
	filename := "test-session-123.jsonl"
	filePath := filepath.Join(projectDir, filename)

	content := `{"type":"user","message":{"role":"user","content":"Hello Claude!"},"uuid":"msg-1","timestamp":"2025-12-01T10:30:00.000Z","sessionId":"session-123"}
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create Claude Code searcher with custom home dir
	searcher := NewClaudeCodeFileSearcherWithHomeDir(homeDir, workingDir)

	events := make(chan WatcherEvent, 10)
	callback := func(event WatcherEvent) {
		events <- event
	}

	watcher := NewLogWatcher(WatcherConfig{
		ProcessStartTime:  now.Add(-time.Minute),
		SearchInterval:    100 * time.Millisecond,
		MaxSearchAttempts: 5,
		PollInterval:      100 * time.Millisecond,
		Callback:          callback,
		Searcher:          searcher,
	})

	// Set Claude Code parser
	watcher.parseLineFn = ParseClaudeCodeLineWrapper

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Wait for session found event
	select {
	case event := <-events:
		if event.Type != EventTypeSessionFound {
			t.Errorf("expected EventTypeSessionFound, got %s", event.Type)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for session found event")
	}

	// Check watcher info
	info := watcher.Info()
	if info.SessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got %s", info.SessionID)
	}

	if info.MessageCount != 1 {
		t.Errorf("expected 1 message, got %d", info.MessageCount)
	}

	if info.LastMessage == nil || info.LastMessage.Message != "Hello Claude!" {
		t.Errorf("expected last message 'Hello Claude!', got %+v", info.LastMessage)
	}

	// Add more content and check for new message event
	newContent := `{"type":"user","message":{"role":"user","content":"Another message"},"uuid":"msg-2","timestamp":"2025-12-01T10:31:00.000Z","sessionId":"session-123"}
`
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	if _, err := f.WriteString(newContent); err != nil {
		f.Close()
		t.Fatalf("failed to write to file: %v", err)
	}
	f.Close()

	// Wait for new message event
	select {
	case event := <-events:
		if event.Type != EventTypeNewMessage {
			t.Errorf("expected EventTypeNewMessage, got %s", event.Type)
		}
		if event.Message == nil || event.Message.Message != "Another message" {
			t.Errorf("expected message 'Another message', got %+v", event.Message)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for new message event")
	}

	// Stop watcher
	watcher.Stop()
}
