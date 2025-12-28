package claude_code

import (
	"regexp"
	"strings"
	"time"

	"github.com/tuzig/vt10x"

	"code-kanban/utils/ai_assistant2/types"
)

// StatusDetector implements state detection for Claude Code
type StatusDetector struct {
	recentInput   string
	recentInput2  string
	grayHintText  string // 灰色提示文字（Mode 128 或光标位置的 Mode 1）
	grayHintText2 string // 上一次的灰色提示文字
}

// NewStatusDetector creates a new Claude Code state detector
func NewStatusDetector() *StatusDetector {
	return &StatusDetector{}
}

// DetectStateFromLines implements structure-based state detection.
// The raw grid is currently unused but reserved for future improvements.
func (d *StatusDetector) DetectStateFromLines(lines []string, raw [][]vt10x.Glyph, cols int, timestamp time.Time, currentState types.State, lastDetectedAt time.Time, cursorX int, cursorY int) (types.State, bool) {
	// Claude Code doesn't need stability checking like Codex
	// Its UI is more stable and reliable
	s := d.detectStateWorkingAndWaiting(lines, raw, cols)
	if s == types.StateUnknown {
		s = d.detectStateApproval(lines, cols)
	}

	// If state detected, it was actually detected from display
	if s != types.StateUnknown {
		return s, true
	}

	return types.StateUnknown, false
}

// containsTipLine checks if a line contains the Tip/Next indicator
func (d *StatusDetector) containsTipLine(line string) bool {
	// Match both old "Tip:" and new "Next:" patterns
	return strings.HasPrefix(line, "  ⎿  Tip: ") || strings.HasPrefix(line, "  ⎿  Next: ")
}

// isWorkingTaskLine checks if a line represents a working task
func (d *StatusDetector) isWorkingTaskLine(line string) bool {
	// Pattern: optional leading spaces + symbol + text + … + (esc to interrupt
	pattern := regexp.MustCompile(`^\s*[✻✽✶∴·○◆▪▫□■☐☑☒★☆✓✔✗✘⚬⚫⚪⬤◯▸▹►▻◂◃◄◅✢*]\s+.+…\s*\(esc\s+to\s+interrupt`)
	return pattern.MatchString(line)
}

func (d *StatusDetector) isSeparatorLine(line string, cols int) bool {
	// Claude Code 的输入框分隔线由 "─" 组成
	// 由于 vt10x 可能将 "─" 当作宽字符处理（每个占 2 cell），
	// 实际的分隔符数量可能是 cols 或 cols/2
	separatorChar := '─'

	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	// 检查行是否全部由分隔符组成
	for _, r := range trimmed {
		if r != separatorChar {
			return false
		}
	}

	// 分隔符数量至少为 cols/2（考虑宽字符情况）
	sepCount := len([]rune(trimmed))
	return sepCount >= cols/2
}

func (d *StatusDetector) GetRecentInput() string {
	if d.recentInput == "" {
		return d.recentInput2
	}
	return d.recentInput
}

func (d *StatusDetector) GetGrayHintText() string {
	if d.grayHintText == "" {
		return d.grayHintText2
	}
	return d.grayHintText
}
