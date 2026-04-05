package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"code-kanban/utils/ai_assistant2/claude_code"
	"code-kanban/utils/ai_assistant2/codex"
	"code-kanban/utils/ai_assistant2/types"

	"github.com/tuzig/vt10x"
)

var (
	syncOutputSeqH = []byte("\x1b[?2026h")
	syncOutputSeqL = []byte("\x1b[?2026l")
)

const (
	syncOutputMaxBufferedBytes    = 4 * 1024 * 1024
	syncOutputMaxBufferedDuration = 5 * time.Second
)

type TerminalDebugInfo struct {
	Item struct {
		SessionID                 string      `json:"sessionId"`
		ProjectID                 string      `json:"projectId"`
		WorktreeID                string      `json:"worktreeId"`
		Status                    string      `json:"status"`
		Rows                      int         `json:"rows"`
		Cols                      int         `json:"cols"`
		ScrollbackChunks          []string    `json:"scrollbackChunks"`
		ScrollbackChunksTimestamp []time.Time `json:"scrollbackChunksTimestamp"`
		ScrollbackSize            int         `json:"scrollbackSize"`
		ScrollbackLimit           int         `json:"scrollbackLimit"`
		AIAssistant               struct {
			Type           string    `json:"type"`
			Name           string    `json:"name"`
			DisplayName    string    `json:"displayName"`
			Detected       bool      `json:"detected"`
			Command        string    `json:"command"`
			State          string    `json:"state"`
			StateUpdatedAt time.Time `json:"stateUpdatedAt"`
			Stats          struct {
				ThinkingDuration        int64 `json:"thinkingDuration"`
				ExecutingDuration       int64 `json:"executingDuration"`
				WaitingApprovalDuration int64 `json:"waitingApprovalDuration"`
				WaitingInputDuration    int64 `json:"waitingInputDuration"`
				CurrentStateDuration    int64 `json:"currentStateDuration"`
			} `json:"stats"`
		} `json:"aiAssistant"`
	} `json:"item"`
}

type StateChange struct {
	Index       int       `json:"index"`
	Timestamp   time.Time `json:"timestamp,omitempty"`
	State       string    `json:"state"`
	Indicator   string    `json:"indicator"`
	Message     string    `json:"message"`
	RawContent  string    `json:"rawContent"`
	CleanedText string    `json:"cleanedText"`
}

type AnalysisReport struct {
	Summary struct {
		TotalChunks      int       `json:"totalChunks"`
		StateChanges     int       `json:"stateChanges"`
		UniqueStates     []string  `json:"uniqueStates"`
		AnalysisTime     time.Time `json:"analysisTime"`
		SessionID        string    `json:"sessionId"`
		CurrentState     string    `json:"currentState"`
		StateUpdatedAt   time.Time `json:"stateUpdatedAt"`
		TotalThinkingMs  int64     `json:"totalThinkingMs"`
		TotalExecutingMs int64     `json:"totalExecutingMs"`
		TotalWaitingMs   int64     `json:"totalWaitingMs"`
	} `json:"summary"`
	StateChanges []StateChange `json:"stateChanges"`
}

type syncOutputBuffering struct {
	depth       int
	startedAt   time.Time
	pending     []byte
	buffer      []byte
	frameTail   []byte
	fullFrame   bool
	afterCommit bool
}

func (s *syncOutputBuffering) buffering() bool {
	return s.depth > 0
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\].*?(?:\x07|\x1b\\)`)

func isLowSignalChunk(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	visible, esc := measureChunkSignal(data)
	return esc && visible <= 8
}

func measureChunkSignal(data []byte) (visible int, escSeen bool) {
	for i := 0; i < len(data); {
		b := data[i]
		if b != 0x1b {
			if b >= 0x20 && b != 0x7f {
				visible++
			}
			i++
			continue
		}

		escSeen = true
		if i+1 >= len(data) {
			i++
			continue
		}

		switch data[i+1] {
		case '[':
			i += 2
			for i < len(data) {
				c := data[i]
				if c >= 0x40 && c <= 0x7e {
					i++
					break
				}
				i++
			}
		case ']':
			i += 2
			for i < len(data) {
				c := data[i]
				if c == 0x07 {
					i++
					break
				}
				if c == 0x1b && i+1 < len(data) && data[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		default:
			i += 2
		}
	}
	return visible, escSeen
}

func visibleLen(s string) int {
	clean := ansiEscapePattern.ReplaceAllString(s, "")
	return len([]rune(clean))
}

func (s *syncOutputBuffering) write(term vt10x.Terminal, chunk []byte, now time.Time) (committed bool, committedFullFrame bool) {
	if len(chunk) == 0 || term == nil {
		return false, false
	}

	// Fast path: no active buffering, no pending prefix, and no possible ?2026 toggles.
	if s.depth == 0 && len(s.pending) == 0 &&
		bytes.Index(chunk, syncOutputSeqH) == -1 &&
		bytes.Index(chunk, syncOutputSeqL) == -1 &&
		syncOutputPendingLen(chunk) == 0 {
		term.Write(chunk)
		return false, false
	}

	data := chunk
	if len(s.pending) > 0 {
		merged := make([]byte, 0, len(s.pending)+len(chunk))
		merged = append(merged, s.pending...)
		merged = append(merged, chunk...)
		data = merged
		s.pending = nil
	}

	pendingLen := syncOutputPendingLen(data)
	processable := data
	if pendingLen > 0 {
		processable = data[:len(data)-pendingLen]
		p := make([]byte, pendingLen)
		copy(p, data[len(data)-pendingLen:])
		s.pending = p
	}

	// If we are currently buffering and never see an end marker, avoid buffering forever.
	if s.depth > 0 {
		tooLarge := len(s.buffer) > syncOutputMaxBufferedBytes
		tooLong := !s.startedAt.IsZero() && now.Sub(s.startedAt) > syncOutputMaxBufferedDuration
		if tooLarge || tooLong {
			if len(s.buffer) > 0 {
				term.Write(s.buffer)
				s.buffer = s.buffer[:0]
				committed = true
			}
			s.depth = 0
			s.startedAt = time.Time{}
			s.fullFrame = false
			s.frameTail = nil
			s.afterCommit = true
		}
	}

	segmentStart := 0
	for i := 0; i < len(processable); {
		if processable[i] == 0x1b && i+len(syncOutputSeqH) <= len(processable) &&
			processable[i+1] == '[' && processable[i+2] == '?' &&
			processable[i+3] == '2' && processable[i+4] == '0' && processable[i+5] == '2' && processable[i+6] == '6' {
			switch processable[i+7] {
			case 'l':
				s.writeOrBuffer(term, processable[segmentStart:i])
				s.afterCommit = false
				s.depth++
				if s.depth == 1 {
					s.startedAt = now
					s.fullFrame = false
					s.frameTail = nil
				}
				i += len(syncOutputSeqL)
				segmentStart = i
				continue
			case 'h':
				s.writeOrBuffer(term, processable[segmentStart:i])
				if s.depth > 0 {
					s.depth--
				}
				if s.depth == 0 {
					committed = true
					committedFullFrame = s.fullFrame
					if len(s.buffer) > 0 {
						term.Write(s.buffer)
						s.buffer = s.buffer[:0]
					}
					s.startedAt = time.Time{}
					s.fullFrame = false
					s.frameTail = nil
					s.afterCommit = true
				}
				i += len(syncOutputSeqH)
				segmentStart = i
				continue
			}
		}
		i++
	}
	s.writeOrBuffer(term, processable[segmentStart:])

	return committed, committedFullFrame
}

func (s *syncOutputBuffering) writeOrBuffer(term vt10x.Terminal, data []byte) {
	if len(data) == 0 || term == nil {
		return
	}

	if s.depth > 0 {
		s.markFullFrame(data)
		s.buffer = append(s.buffer, data...)
		return
	}

	term.Write(data)
}

func (s *syncOutputBuffering) markFullFrame(data []byte) {
	if s.fullFrame {
		s.frameTail = updateTail(s.frameTail, data, 16)
		return
	}

	combined := data
	if len(s.frameTail) > 0 {
		tmp := make([]byte, 0, len(s.frameTail)+len(data))
		tmp = append(tmp, s.frameTail...)
		tmp = append(tmp, data...)
		combined = tmp
	}

	if containsFullFrameCSI(combined) {
		s.fullFrame = true
	}
	s.frameTail = updateTail(s.frameTail, data, 16)
}

func updateTail(tail []byte, data []byte, max int) []byte {
	if max <= 0 {
		return nil
	}
	if len(data) >= max {
		out := make([]byte, max)
		copy(out, data[len(data)-max:])
		return out
	}

	need := max - len(data)
	if need > len(tail) {
		need = len(tail)
	}
	out := make([]byte, 0, need+len(data))
	if need > 0 {
		out = append(out, tail[len(tail)-need:]...)
	}
	out = append(out, data...)
	return out
}

// containsFullFrameCSI reports whether data contains a cursor-home or clear-screen CSI,
// which typically indicates a full-frame redraw (vs. a tiny diff update).
func containsFullFrameCSI(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Fast paths for common sequences.
	if bytes.Contains(data, []byte("\x1b[H")) || bytes.Contains(data, []byte("\x1b[2J")) {
		return true
	}

	// Match ESC[<row>;<col>H where (row,col) == (1,1) (missing params default to 1).
	for i := 0; i+2 < len(data); i++ {
		if data[i] != 0x1b || data[i+1] != '[' {
			continue
		}
		j := i + 2
		row, col := 1, 1
		gotRow, gotCol := false, false
		val := 0
		inNumber := false
		flush := func() {
			if !gotRow {
				row = val
				gotRow = true
			} else if !gotCol {
				col = val
				gotCol = true
			}
			val = 0
			inNumber = false
		}
		for j < len(data) && j-i <= 12 {
			b := data[j]
			switch {
			case b >= '0' && b <= '9':
				val = val*10 + int(b-'0')
				inNumber = true
				j++
				continue
			case b == ';':
				if inNumber {
					flush()
				} else {
					if !gotRow {
						gotRow = true
					} else if !gotCol {
						gotCol = true
					}
				}
				j++
				continue
			case b == 'H':
				if inNumber {
					flush()
				}
				if row == 0 {
					row = 1
				}
				if col == 0 {
					col = 1
				}
				return row == 1 && col == 1
			case b == 'J':
				// Treat any clear-screen CSI as a full-frame signal.
				return true
			default:
				break
			}
			break
		}
	}
	return false
}

func syncOutputPendingLen(data []byte) int {
	if len(data) == 0 {
		return 0
	}

	maxCheck := len(syncOutputSeqH) - 1
	if maxCheck <= 0 {
		return 0
	}
	if len(data) < maxCheck {
		maxCheck = len(data)
	}

	for n := maxCheck; n > 0; n-- {
		suffix := data[len(data)-n:]
		if bytes.HasPrefix(syncOutputSeqH, suffix) || bytes.HasPrefix(syncOutputSeqL, suffix) {
			return n
		}
	}

	return 0
}

func main() {
	var (
		source            = flag.String("source", "", "JSON file path or URL")
		outputJSON        = flag.String("json", "analysis.json", "Output JSON report path")
		assistantType     = flag.String("type", "claude", "AI assistant type: claude or codex")
		enableSyncOutput  = flag.Bool("sync2026", true, "Enable CSI ?2026 synchronized output buffering")
		enableCodexFilter = flag.Bool("codex-filter", true, "Enable Codex diff-rendering blip filters")
	)
	flag.Parse()

	if *source == "" {
		fmt.Println("Usage: claude-terminal-analyzer -source <file_or_url> [-json output.json] [-type claude|codex] [-sync2026=true|false] [-codex-filter=true|false]")
		os.Exit(1)
	}

	// 验证助手类型
	if *assistantType != "claude" && *assistantType != "codex" {
		fmt.Printf("Error: invalid assistant type '%s', must be 'claude' or 'codex'\n", *assistantType)
		os.Exit(1)
	}

	// 加载数据
	var data TerminalDebugInfo
	if strings.HasPrefix(*source, "http://") || strings.HasPrefix(*source, "https://") {
		if err := loadFromURL(*source, &data); err != nil {
			fmt.Printf("Error loading from URL: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := loadFromFile(*source, &data); err != nil {
			fmt.Printf("Error loading from file: %v\n", err)
			os.Exit(1)
		}
	}

	// 分析状态变化
	report := analyzeStateChanges(&data, *assistantType, *enableSyncOutput, *enableCodexFilter)

	// 输出 JSON 报告
	if err := saveJSONReport(report, *outputJSON); err != nil {
		fmt.Printf("Error saving JSON report: %v\n", err)
	} else {
		fmt.Printf("JSON report saved to: %s\n", *outputJSON)
	}

	// 打印摘要
	printSummary(report)
}

func loadFromURL(url string, data *TerminalDebugInfo) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(data)
}

func loadFromFile(path string, data *TerminalDebugInfo) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(data)
}

// getTerminalDisplay extracts the visible content from the terminal emulator
func getTerminalDisplay(term vt10x.Terminal, rows, cols int) []string {
	lines := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		line := ""
		for j := 0; j < cols; j++ {
			cell := term.Cell(j, i)
			// Skip wide character dummy cells (second cell of double-width characters)
			if cell.Mode&vt10x.AttrWideDummy != 0 {
				continue
			}
			if cell.Char != 0 {
				line += string(cell.Char)
			} else {
				line += " "
			}
		}
		// Trim trailing spaces
		trimmed := strings.TrimRight(line, " ")
		lines = append(lines, trimmed)
	}
	return lines
}

// detectStateFromTerminal detects state from terminal display lines using structure-based detection
func detectStateFromTerminal(lines []string, cols int, assistantType string, timestamp time.Time, currentState types.State, lastDetectedAt time.Time) (state types.State, indicatorLine string) {
	var detected types.State
	var actuallyDetected bool

	// Use appropriate detector based on assistant type
	switch assistantType {
	case "codex":
		detector := codex.NewStatusDetector()
		detected, actuallyDetected = detector.DetectStateFromLines(lines, nil, cols, timestamp, currentState, lastDetectedAt, 0, 0)
	default: // claude
		detector := claude_code.NewStatusDetector()
		detected, actuallyDetected = detector.DetectStateFromLines(lines, nil, cols, timestamp, currentState, lastDetectedAt, 0, 0)
	}

	// For analysis tool, we only care about actually detected states
	if !actuallyDetected {
		detected = types.StateUnknown
	}

	if detected != types.StateUnknown {
		// Find the indicator line
		for _, line := range lines {
			// Claude Code indicators
			if strings.Contains(line, "⎿  Tip:") || strings.Contains(line, "esc to interrupt") {
				return detected, line
			}
			// Codex indicators
			if strings.Contains(line, "Press enter to confirm") {
				return detected, line
			}
		}
		// If no specific indicator found, return first non-empty line from bottom
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) != "" {
				return detected, lines[i]
			}
		}
	}

	return types.StateUnknown, ""
}

// stateToString converts State enum to string
func stateToString(state types.State) string {
	return string(state)
}

func analyzeStateChanges(data *TerminalDebugInfo, assistantType string, enableSyncOutput bool, enableCodexFilter bool) *AnalysisReport {
	report := &AnalysisReport{}
	report.Summary.TotalChunks = len(data.Item.ScrollbackChunks)
	report.Summary.AnalysisTime = time.Now()
	report.Summary.SessionID = data.Item.SessionID
	report.Summary.CurrentState = data.Item.AIAssistant.State
	report.Summary.StateUpdatedAt = data.Item.AIAssistant.StateUpdatedAt
	report.Summary.TotalThinkingMs = data.Item.AIAssistant.Stats.ThinkingDuration / 1000000
	report.Summary.TotalExecutingMs = data.Item.AIAssistant.Stats.ExecutingDuration / 1000000
	report.Summary.TotalWaitingMs = data.Item.AIAssistant.Stats.WaitingInputDuration / 1000000

	// Generate timestamps if not provided
	// If missing, create timestamps starting from now, each chunk +10ms
	timestamps := data.Item.ScrollbackChunksTimestamp
	if len(timestamps) == 0 || len(timestamps) != len(data.Item.ScrollbackChunks) {
		baseTime := time.Now()
		timestamps = make([]time.Time, len(data.Item.ScrollbackChunks))
		for i := range timestamps {
			timestamps[i] = baseTime.Add(time.Duration(i*10) * time.Millisecond)
		}
	}

	// Create virtual terminal
	rows, cols := data.Item.Rows, data.Item.Cols
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}
	term := vt10x.New(vt10x.WithSize(cols, rows))
	var syncOut *syncOutputBuffering
	if enableSyncOutput {
		syncOut = &syncOutputBuffering{}
	}

	stateMap := make(map[string]bool)
	var lastState types.State
	var lastStateTime time.Time

	// Feed chunks to terminal and detect state changes
	for i, chunk := range data.Item.ScrollbackChunks {
		inSyncGap := false
		committed := false
		committedFullFrame := false

		// Apply synchronized output semantics (CSI ?2026 h/l) so we don't read intermediate frames.
		chunkBytes := []byte(chunk)
		if syncOut != nil {
			inSyncGap = syncOut.afterCommit
			committed, committedFullFrame = syncOut.write(term, chunkBytes, timestamps[i])
			inSyncGap = inSyncGap && !committed
		} else {
			_, _ = term.Write(chunkBytes)
		}

		// If synchronized output is in progress, skip detection until it ends.
		if syncOut != nil && syncOut.buffering() {
			continue
		}

		// Get current terminal display
		lines := getTerminalDisplay(term, rows, cols)

		// Detect state from terminal display using structure-based detection
		currentState, indicatorLine := detectStateFromTerminal(lines, cols, assistantType, timestamps[i], lastState, lastStateTime)

		if currentState == types.StateUnknown {
			continue
		}

		// Same-state refresh: keep lastDetectedAt up to date.
		if currentState == lastState {
			lastStateTime = timestamps[i]
			continue
		}

		if enableCodexFilter {
			// Align with server-side tracker: Codex partial sync commits can create false
			// working -> waiting_input transitions. Only accept that transition on full-frame commits.
			if assistantType == "codex" &&
				lastState == types.StateWorking &&
				currentState == types.StateWaitingInput &&
				committed && !committedFullFrame {
				continue
			}

			// Also ignore low-signal diff chunks (cursor moves/clears with almost no text) that can
			// momentarily corrupt the status line and create a false working -> waiting_input blip.
			if assistantType == "codex" &&
				lastState == types.StateWorking &&
				currentState == types.StateWaitingInput &&
				!committed && isLowSignalChunk(chunkBytes) {
				continue
			}

			// The chunks immediately after a sync commit (before the next sync start) tend to contain
			// small cursor-move/clear diffs. Avoid exiting working based on those intermediate frames.
			if assistantType == "codex" &&
				lastState == types.StateWorking &&
				currentState == types.StateWaitingInput &&
				!committed && inSyncGap {
				continue
			}
		}

		// Record state change
		stateStr := stateToString(currentState)

		// Join all visible lines for the CleanedText (terminal snapshot)
		terminalSnapshot := strings.Join(lines, "\n")

		change := StateChange{
			Index:       i,
			Timestamp:   timestamps[i],
			State:       stateStr,
			Indicator:   indicatorLine,
			Message:     "", // Can extract from indicator line if needed
			RawContent:  chunk,
			CleanedText: terminalSnapshot,
		}

		report.StateChanges = append(report.StateChanges, change)
		stateMap[stateStr] = true
		lastState = currentState
		lastStateTime = timestamps[i]
	}

	report.Summary.StateChanges = len(report.StateChanges)
	for state := range stateMap {
		report.Summary.UniqueStates = append(report.Summary.UniqueStates, state)
	}

	return report
}

func saveJSONReport(report *AnalysisReport, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func printSummary(report *AnalysisReport) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 ANALYSIS SUMMARY")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Session ID:      %s\n", report.Summary.SessionID)
	fmt.Printf("Total Chunks:    %d\n", report.Summary.TotalChunks)
	fmt.Printf("State Changes:   %d\n", report.Summary.StateChanges)
	fmt.Printf("Unique States:   %v\n", report.Summary.UniqueStates)
	fmt.Printf("Current State:   %s\n", report.Summary.CurrentState)
	fmt.Printf("State Updated:   %s\n", report.Summary.StateUpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Thinking Time:   %.2fs\n", float64(report.Summary.TotalThinkingMs)/1000)
	fmt.Printf("Executing Time:  %.2fs\n", float64(report.Summary.TotalExecutingMs)/1000)
	fmt.Printf("Waiting Time:    %.2fs\n", float64(report.Summary.TotalWaitingMs)/1000)
	fmt.Println(strings.Repeat("=", 60))

	fmt.Println("\n🔄 STATE TRANSITIONS:")
	for i, change := range report.StateChanges {
		fmt.Printf("  %d. [Chunk #%d] %s", i+1, change.Index, strings.ToUpper(change.State))
		if change.Message != "" {
			fmt.Printf(" - %s", change.Message)
		}
		fmt.Println()
	}
	fmt.Println()
}
