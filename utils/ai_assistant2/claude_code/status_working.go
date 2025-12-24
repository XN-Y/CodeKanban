package claude_code

import (
	"code-kanban/utils/ai_assistant2/types"
	"fmt"
	"strings"

	"github.com/tuzig/vt10x"
)

func (d *StatusDetector) detectStateWorkingAndWaiting(lines []string, raw [][]vt10x.Glyph, cols int) types.State {
	if len(lines) == 0 || cols <= 0 {
		return types.StateUnknown
	}

	currentLine := len(lines) - 1

	// Step 1: Find the input text box by locating two separator lines
	// Search from bottom to top for lines filled with '─' characters
	firstSepIdx := -1
	secondSepIdx := -1

	for ; currentLine >= 0; currentLine-- {
		line := lines[currentLine]

		// Check if this line is a separator (filled with ─)
		if d.isSeparatorLine(line, cols) {
			if firstSepIdx == -1 {
				firstSepIdx = currentLine
			} else {
				secondSepIdx = currentLine
				break
			}
		}
	}

	if firstSepIdx == -1 || secondSepIdx == -1 {
		return types.StateUnknown
	}

	// 顺手取出两线之中的内容，过滤掉灰色字（AttrFaint，mode & 128）
	// 注意：光标位置的字符可能是 Mode 1，如果后面紧跟灰色字则也视为灰色字
	recentInputs := lines[secondSepIdx+1 : firstSepIdx]
	grayHintLines := make([]string, len(recentInputs)) // 收集灰色提示文字

	for i := range recentInputs {
		lineIdx := secondSepIdx + 1 + i
		// 如果有 raw 数据，过滤掉灰色字
		if raw != nil && lineIdx < len(raw) {
			runes := []rune(recentInputs[i])
			var filtered strings.Builder
			var grayHint strings.Builder

			for colIdx, ch := range runes {
				// 检查对应位置的 mode 是否为灰色字
				if colIdx < len(raw[lineIdx]) {
					mode := raw[lineIdx][colIdx].Mode
					isGray := false

					// 灰色字（AttrFaint）直接跳过
					if mode&int16(vt10x.AttrFaint) != 0 {
						isGray = true
					}
					// 光标位置（Mode 1）：如果下一个字符是灰色字，则当前字符也视为灰色字跳过
					// 这处理了 "> Try ..." 中 T 因光标而变成 Mode 1 的情况
					if mode == 1 && colIdx+1 < len(raw[lineIdx]) {
						if raw[lineIdx][colIdx+1].Mode&int16(vt10x.AttrFaint) != 0 {
							isGray = true
						}
					}

					if isGray {
						grayHint.WriteRune(ch)
						continue
					}
				}
				filtered.WriteRune(ch)
			}
			recentInputs[i] = filtered.String()
			grayHintLines[i] = grayHint.String()
		}
		recentInputs[i] = strings.TrimSpace(recentInputs[i])
		grayHintLines[i] = strings.TrimSpace(grayHintLines[i])
	}

	recentInput := strings.Join(recentInputs, "")
	recentInput, _ = strings.CutPrefix(recentInput, ">")
	recentInput = strings.TrimSpace(recentInput)
	// 只有当过滤后有实际内容时才更新 recentInput
	if recentInput != "" && recentInput != d.recentInput {
		d.recentInput2 = d.recentInput
		d.recentInput = recentInput
	}

	// 保存灰色提示文字
	grayHint := strings.Join(grayHintLines, "")
	grayHint = strings.TrimSpace(grayHint)
	if grayHint != "" && grayHint != d.grayHintText {
		d.grayHintText2 = d.grayHintText
		d.grayHintText = grayHint
	}

	// If we found the input text box (two separator lines)
	// The text box is located, which means the interface is active
	// Now search upward from the second separator to determine the state

	// Step 2: Look for "  ⎿  Tip: " above the text box
	currentLine = secondSepIdx - 1
	for ; currentLine >= 0; currentLine-- {
		line := lines[currentLine]
		if d.containsTipLine(line) {
			fmt.Println(firstSepIdx, secondSepIdx, d.isWorkingTaskLine(lines[currentLine-1]))

			if currentLine > 0 && d.isWorkingTaskLine(lines[currentLine-1]) {
				return types.StateWorking
			}
		}
		if d.isWorkingTaskLine(line) {
			return types.StateWorking
		}
	}

	// No Tip line found = waiting for input
	return types.StateWaitingInput
}
