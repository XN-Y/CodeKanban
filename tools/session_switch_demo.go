//go:build ignore

// Windows-oriented session switch debug script.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// getFileCreationTime 获取文件的创建时间（Windows）
func getFileCreationTime(path string, info os.FileInfo) time.Time {
	if info == nil {
		return time.Time{}
	}

	sys := info.Sys()
	if sys == nil {
		return info.ModTime()
	}

	winData, ok := sys.(*syscall.Win32FileAttributeData)
	if !ok {
		return info.ModTime()
	}

	nsec := winData.CreationTime.Nanoseconds()
	return time.Unix(0, nsec)
}

// 模拟 session 切换场景的测试工具

type SessionSwitchTester struct {
	projectDir       string
	currentSessionID string
	currentFilePath  string
	lastFileModTime  time.Time
	referenceTime    time.Time // 用于搜索新 session 的参考时间
}

func NewSessionSwitchTester(projectDir string) *SessionSwitchTester {
	return &SessionSwitchTester{
		projectDir: projectDir,
	}
}

// SetCurrentSession 设置当前正在监控的 session
func (t *SessionSwitchTester) SetCurrentSession(sessionID, filePath string) {
	t.currentSessionID = sessionID
	t.currentFilePath = filePath

	// 获取文件的最后修改时间
	if info, err := os.Stat(filePath); err == nil {
		t.lastFileModTime = info.ModTime()
	}
	t.referenceTime = t.lastFileModTime

	fmt.Printf("[SetCurrentSession]\n")
	fmt.Printf("  SessionID: %s\n", sessionID)
	fmt.Printf("  FilePath: %s\n", filePath)
	fmt.Printf("  LastModTime: %s\n", t.lastFileModTime.Format(time.RFC3339))
}

// CheckForSessionSwitch 检查是否需要切换 session
// 返回：(needSwitch bool, newSessionID string, newFilePath string)
func (t *SessionSwitchTester) CheckForSessionSwitch(userInputTime time.Time) (bool, string, string) {
	fmt.Printf("\n[CheckForSessionSwitch]\n")
	fmt.Printf("  UserInputTime: %s\n", userInputTime.Format(time.RFC3339))
	fmt.Printf("  LastFileModTime: %s\n", t.lastFileModTime.Format(time.RFC3339))

	// 检查当前文件是否有更新
	currentInfo, err := os.Stat(t.currentFilePath)
	if err != nil {
		fmt.Printf("  Error: cannot stat current file: %v\n", err)
		// 文件不存在或无法访问，可能需要切换
	} else {
		currentModTime := currentInfo.ModTime()
		fmt.Printf("  CurrentModTime: %s\n", currentModTime.Format(time.RFC3339))

		// 如果文件在用户输入后有更新，说明还是同一个 session
		if currentModTime.After(userInputTime) {
			fmt.Printf("  Result: 文件在用户输入后有更新，无需切换\n")
			t.lastFileModTime = currentModTime
			return false, "", ""
		}
	}

	// 文件没有更新，可能是 /new 或 /clear 创建了新会话
	// 使用用户输入时间（或上次文件更新时间的较大者）作为新的参考点
	newReferenceTime := userInputTime
	if t.lastFileModTime.After(newReferenceTime) {
		newReferenceTime = t.lastFileModTime
	}

	fmt.Printf("  使用新参考时间搜索: %s\n", newReferenceTime.Format(time.RFC3339))

	// 搜索在新参考时间之后创建的 session 文件
	newSessionID, newFilePath := t.searchNewSession(newReferenceTime)

	if newSessionID != "" && newSessionID != t.currentSessionID {
		fmt.Printf("  Result: 找到新 session!\n")
		fmt.Printf("    NewSessionID: %s\n", newSessionID)
		fmt.Printf("    NewFilePath: %s\n", newFilePath)
		return true, newSessionID, newFilePath
	}

	fmt.Printf("  Result: 未找到新 session\n")
	return false, "", ""
}

// searchNewSession 搜索在 afterTime 之后创建的新 session
func (t *SessionSwitchTester) searchNewSession(afterTime time.Time) (string, string) {
	entries, err := os.ReadDir(t.projectDir)
	if err != nil {
		return "", ""
	}

	type candidate struct {
		sessionID string
		filePath  string
		ctime     time.Time
	}

	var candidates []candidate
	tolerance := 5 * time.Second

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if len(name) < 7 || name[len(name)-6:] != ".jsonl" {
			continue
		}

		// 跳过 agent- 前缀的文件
		if len(name) > 6 && name[:6] == "agent-" {
			continue
		}

		sessionID := name[:len(name)-6]
		filePath := filepath.Join(t.projectDir, name)

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 获取文件创建时间（在 Windows 上使用 ModTime 作为近似）
		ctime := info.ModTime()

		// 检查是否在 afterTime 之后创建
		if ctime.Add(tolerance).After(afterTime) && sessionID != t.currentSessionID {
			candidates = append(candidates, candidate{
				sessionID: sessionID,
				filePath:  filePath,
				ctime:     ctime,
			})
			fmt.Printf("    候选: %s (ctime: %s)\n", sessionID, ctime.Format(time.RFC3339))
		}
	}

	if len(candidates) == 0 {
		return "", ""
	}

	// 选择最早创建的（第一个在参考时间之后创建的）
	earliest := candidates[0]
	for _, c := range candidates[1:] {
		if c.ctime.Before(earliest.ctime) {
			earliest = c
		}
	}

	return earliest.sessionID, earliest.filePath
}

// SwitchToNewSession 切换到新的 session
func (t *SessionSwitchTester) SwitchToNewSession(newSessionID, newFilePath string) {
	oldSessionID := t.currentSessionID

	t.currentSessionID = newSessionID
	t.currentFilePath = newFilePath

	if info, err := os.Stat(newFilePath); err == nil {
		t.lastFileModTime = info.ModTime()
	}
	t.referenceTime = t.lastFileModTime

	fmt.Printf("\n[SwitchToNewSession]\n")
	fmt.Printf("  OldSessionID: %s\n", oldSessionID)
	fmt.Printf("  NewSessionID: %s\n", newSessionID)
	fmt.Printf("  可以在此处添加会话链接标记\n")
}

// 创建测试用的 session 文件
func createTestSessionFile(dir, sessionID string, content []map[string]interface{}) error {
	filePath := filepath.Join(dir, sessionID+".jsonl")
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range content {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		f.Write(data)
		f.WriteString("\n")
	}

	return nil
}

// printFirstUserMessage 打印 session 文件的第一条用户消息
func printFirstUserMessage(filePath string) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry struct {
			Type    string `json:"type"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.Type == "user" && entry.Message.Role == "user" {
			content := entry.Message.Content
			if len(content) > 60 {
				content = content[:60] + "..."
			}
			// 替换换行符
			content = strings.ReplaceAll(content, "\n", " ")
			fmt.Printf("     首条消息: \"%s\"\n", content)
			return
		}
	}
}

// testRealSessionSwitch 测试真实的 session 切换场景
// 使用真实的 Claude Code session 目录
func testRealSessionSwitch(projectDir string, processStartTime time.Time) {
	fmt.Printf("=== 真实 Session 切换测试 ===\n")
	fmt.Printf("Project Dir: %s\n", projectDir)
	fmt.Printf("Process Start: %s\n\n", processStartTime.Format("2006-01-02 15:04:05"))

	tester := NewSessionSwitchTester(projectDir)

	// 列出所有在进程启动后创建的文件（用 Birth 时间判断）
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		fmt.Printf("Error reading dir: %v\n", err)
		return
	}

	type sessionFile struct {
		id        string
		path      string
		birthTime time.Time // 创建时间
		modTime   time.Time // 修改时间
	}

	var sessions []sessionFile
	fmt.Printf("扫描文件（只看 Birth > ProcessStart）:\n")
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "agent-") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(projectDir, entry.Name())
		birthTime := getFileCreationTime(filePath, info)
		modTime := info.ModTime()

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")

		// 只看进程启动后【创建】的文件（不是修改）
		if birthTime.After(processStartTime) {
			sessions = append(sessions, sessionFile{
				id:        sessionID,
				path:      filePath,
				birthTime: birthTime,
				modTime:   modTime,
			})
			fmt.Printf("  ✓ %s (Birth: %s, Mod: %s)\n", sessionID[:8]+"...", birthTime.Format("15:04:05"), modTime.Format("15:04:05"))
		} else if modTime.After(processStartTime) {
			// 创建时间在进程启动前，但修改时间在进程启动后（可能是 resume）
			fmt.Printf("  - %s (Birth: %s < Start, Mod: %s) [可能是resume]\n", sessionID[:8]+"...", birthTime.Format("15:04:05"), modTime.Format("15:04:05"))
		}
	}

	// 按创建时间排序
	for i := 0; i < len(sessions)-1; i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[i].birthTime.After(sessions[j].birthTime) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	fmt.Printf("\n找到 %d 个在进程启动后创建的 session 文件:\n", len(sessions))
	for i, s := range sessions {
		fmt.Printf("  %d. %s (创建于 %s)\n", i+1, s.id[:8]+"...", s.birthTime.Format("15:04:05"))
		// 打印第一条用户消息
		printFirstUserMessage(s.path)
	}
	fmt.Println()

	if len(sessions) == 0 {
		fmt.Printf("没有找到相关的 session 文件\n")
		return
	}

	// 模拟 session 切换检测流程
	fmt.Printf("=== 模拟 Session 切换检测 ===\n\n")

	// 初始绑定第一个 session
	tester.SetCurrentSession(sessions[0].id, sessions[0].path)
	fmt.Println()

	// 对于每个后续的 session，模拟检测切换
	for i := 1; i < len(sessions); i++ {
		prevSession := sessions[i-1]
		currSession := sessions[i]

		fmt.Printf("--- 检测第 %d 次切换 ---\n", i)
		fmt.Printf("当前绑定: %s...\n", tester.currentSessionID[:8])
		fmt.Printf("上次更新: %s\n", tester.lastFileModTime.Format("15:04:05"))

		// 使用上一个 session 的最后更新时间作为参考点（modTime）
		referenceTime := prevSession.modTime

		// 检查是否需要切换（使用 Birth 时间来找新 session）
		needSwitch, newSessionID, newFilePath := tester.CheckForSessionSwitchWithRefBirth(referenceTime)

		if needSwitch {
			fmt.Printf("✓ 检测到切换需求\n")
			tester.SwitchToNewSession(newSessionID, newFilePath)

			// 验证是否切换到了正确的 session
			if newSessionID == currSession.id {
				fmt.Printf("✓ 正确切换到: %s...\n", currSession.id[:8])
			} else {
				fmt.Printf("△ 切换到: %s... (预期: %s...)\n", newSessionID[:8], currSession.id[:8])
			}
		} else {
			fmt.Printf("✗ 未能检测到切换\n")
		}
		fmt.Println()
	}

	fmt.Printf("=== 测试完成 ===\n")
}

// CheckForSessionSwitchWithRef 使用指定的参考时间检查是否需要切换
func (t *SessionSwitchTester) CheckForSessionSwitchWithRef(referenceTime time.Time) (bool, string, string) {
	fmt.Printf("[CheckForSessionSwitchWithRef]\n")
	fmt.Printf("  ReferenceTime: %s\n", referenceTime.Format("15:04:05"))

	// 搜索在参考时间之后创建的 session 文件
	newSessionID, newFilePath := t.searchNewSession(referenceTime)

	if newSessionID != "" && newSessionID != t.currentSessionID {
		fmt.Printf("  Result: 找到新 session!\n")
		fmt.Printf("    NewSessionID: %s...\n", newSessionID[:8])
		return true, newSessionID, newFilePath
	}

	fmt.Printf("  Result: 未找到新 session\n")
	return false, "", ""
}

// CheckForSessionSwitchWithRefBirth 使用 Birth 时间来搜索新 session
func (t *SessionSwitchTester) CheckForSessionSwitchWithRefBirth(referenceTime time.Time) (bool, string, string) {
	fmt.Printf("[CheckForSessionSwitchWithRefBirth]\n")
	fmt.Printf("  ReferenceTime: %s\n", referenceTime.Format("15:04:05"))

	// 搜索在参考时间之后【创建】的 session 文件
	newSessionID, newFilePath := t.searchNewSessionByBirth(referenceTime)

	if newSessionID != "" && newSessionID != t.currentSessionID {
		fmt.Printf("  Result: 找到新 session!\n")
		fmt.Printf("    NewSessionID: %s...\n", newSessionID[:8])
		return true, newSessionID, newFilePath
	}

	fmt.Printf("  Result: 未找到新 session\n")
	return false, "", ""
}

// testSessionBasedSwitch 基于 sessionId 的会话切换检测（不依赖 PID）
func testSessionBasedSwitch(projectDir string) {
	fmt.Printf("=== 基于 SessionId 的会话切换检测 ===\n")
	fmt.Printf("Project Dir: %s\n\n", projectDir)

	// 先扫描所有文件，列出在 04:56 之后有活动的文件
	fmt.Printf("=== 扫描所有在 04:56 之后有活动的文件 ===\n")
	refTime := time.Date(2025, 12, 25, 4, 56, 0, 0, time.Local)

	type fileInfo struct {
		id        string
		path      string
		birthTime time.Time
		modTime   time.Time
		firstMsg  string
	}

	var allFiles []fileInfo

	entries, _ := os.ReadDir(projectDir)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "agent-") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		filePath := filepath.Join(projectDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}

		birthTime := getFileCreationTime(filePath, info)
		modTime := info.ModTime()

		// 只看 modTime > 04:56 的文件
		if modTime.After(refTime) {
			// 读取第一条用户消息
			firstMsg := getFirstUserMessage(filePath)
			allFiles = append(allFiles, fileInfo{
				id:        sessionID,
				path:      filePath,
				birthTime: birthTime,
				modTime:   modTime,
				firstMsg:  firstMsg,
			})
		}
	}

	// 按 birthTime 排序
	for i := 0; i < len(allFiles)-1; i++ {
		for j := i + 1; j < len(allFiles); j++ {
			if allFiles[i].birthTime.After(allFiles[j].birthTime) {
				allFiles[i], allFiles[j] = allFiles[j], allFiles[i]
			}
		}
	}

	fmt.Printf("找到 %d 个文件:\n", len(allFiles))
	for _, f := range allFiles {
		diff := f.modTime.Sub(f.birthTime).Seconds()
		empty := ""
		if diff < 2 {
			empty = " [空]"
		}
		fmt.Printf("  %s | Birth: %s | Mod: %s | diff=%.0fs%s | msg: %s\n",
			f.id[:8]+"...",
			f.birthTime.Format("15:04:05"),
			f.modTime.Format("15:04:05"),
			diff,
			empty,
			truncate(f.firstMsg, 30))
	}
	fmt.Println()

	// 找出 PID 35592 的 3 个会话（111、222、333）
	// 这三个是在 04:56:07 到 05:00:37 之间创建的
	startBound := time.Date(2025, 12, 25, 4, 56, 0, 0, time.Local)
	endBound := time.Date(2025, 12, 25, 5, 1, 0, 0, time.Local)

	var targetSessions []fileInfo
	for _, f := range allFiles {
		// 条件：Birth 在范围内 且 有内容（diff > 2s）
		if f.birthTime.After(startBound) && f.birthTime.Before(endBound) {
			if f.modTime.Sub(f.birthTime) >= 2*time.Second {
				targetSessions = append(targetSessions, f)
			}
		}
	}

	// 按 birthTime 排序
	for i := 0; i < len(targetSessions)-1; i++ {
		for j := i + 1; j < len(targetSessions); j++ {
			if targetSessions[i].birthTime.After(targetSessions[j].birthTime) {
				targetSessions[i], targetSessions[j] = targetSessions[j], targetSessions[i]
			}
		}
	}

	fmt.Printf("=== PID 35592 的会话 (%d 个) ===\n", len(targetSessions))
	for i, f := range targetSessions {
		fmt.Printf("  %d. %s (Birth: %s, Mod: %s) - \"%s\"\n",
			i+1, f.id[:8]+"...",
			f.birthTime.Format("15:04:05"),
			f.modTime.Format("15:04:05"),
			truncate(f.firstMsg, 40))
	}
	fmt.Println()

	if len(targetSessions) == 0 {
		fmt.Printf("没有找到目标会话\n")
		return
	}

	// 模拟切换检测
	fmt.Printf("=== 模拟切换检测 ===\n\n")

	tester := NewSessionSwitchTester(projectDir)

	// 初始绑定第一个 session (111)
	tester.SetCurrentSession(targetSessions[0].id, targetSessions[0].path)
	fmt.Println()

	// 对于后续的每个 session，模拟检测
	for i := 1; i < len(targetSessions); i++ {
		fmt.Printf("--- 检测第 %d 次切换 ---\n", i)
		fmt.Printf("当前绑定: %s... (Mod: %s)\n", tester.currentSessionID[:8], tester.lastFileModTime.Format("15:04:05"))

		referenceTime := tester.lastFileModTime
		fmt.Printf("参考时间: %s\n", referenceTime.Format("15:04:05"))

		newSessionID, newFilePath, birthTime := tester.searchNewSessionAfter(referenceTime)

		if newSessionID != "" {
			fmt.Printf("找到: %s... (Birth: %s)\n", newSessionID[:8], birthTime.Format("15:04:05"))
			if newSessionID == targetSessions[i].id {
				fmt.Printf("✓ 正确\n")
			} else {
				fmt.Printf("△ 预期: %s...\n", targetSessions[i].id[:8])
			}
			tester.SwitchToNewSession(newSessionID, newFilePath)
		} else {
			fmt.Printf("✗ 未找到\n")
		}
		fmt.Println()
	}

	fmt.Printf("=== 测试完成 ===\n")
}

func getFirstUserMessage(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry struct {
			Type    string `json:"type"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.Type == "user" && entry.Message.Role == "user" {
			content := entry.Message.Content
			// 跳过 Caveat 消息
			if strings.HasPrefix(content, "Caveat:") {
				continue
			}
			// 跳过命令消息
			if strings.HasPrefix(content, "<command-") || strings.HasPrefix(content, "<local-command") {
				continue
			}
			return strings.ReplaceAll(content, "\n", " ")
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// searchNewSessionAfter 搜索在 afterTime 之后创建的新 session
// 返回: sessionID, filePath, birthTime
func (t *SessionSwitchTester) searchNewSessionAfter(afterTime time.Time) (string, string, time.Time) {
	entries, err := os.ReadDir(t.projectDir)
	if err != nil {
		return "", "", time.Time{}
	}

	type candidate struct {
		sessionID string
		filePath  string
		birthTime time.Time
		modTime   time.Time
	}

	var candidates []candidate

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "agent-") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		filePath := filepath.Join(t.projectDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}

		birthTime := getFileCreationTime(filePath, info)
		modTime := info.ModTime()

		// 条件1: Birth > afterTime（新创建的 session）
		// 条件2: 不是当前 session
		// 条件3: 有内容（Mod > Birth，说明有写入）
		if birthTime.After(afterTime) && sessionID != t.currentSessionID {
			// 排除空文件：Mod - Birth < 2秒 的跳过
			if modTime.Sub(birthTime) < 2*time.Second {
				fmt.Printf("    跳过空文件: %s (Birth≈Mod)\n", sessionID[:8]+"...")
				continue
			}
			candidates = append(candidates, candidate{
				sessionID: sessionID,
				filePath:  filePath,
				birthTime: birthTime,
				modTime:   modTime,
			})
		}
	}

	if len(candidates) == 0 {
		return "", "", time.Time{}
	}

	// 选择 Birth 最早的（第一个新创建的）
	earliest := candidates[0]
	for _, c := range candidates[1:] {
		if c.birthTime.Before(earliest.birthTime) {
			earliest = c
		}
	}

	return earliest.sessionID, earliest.filePath, earliest.birthTime
}

// searchNewSessionByBirth 搜索在 afterTime 之后【创建】的新 session（使用 Birth 时间）
func (t *SessionSwitchTester) searchNewSessionByBirth(afterTime time.Time) (string, string) {
	entries, err := os.ReadDir(t.projectDir)
	if err != nil {
		return "", ""
	}

	type candidate struct {
		sessionID string
		filePath  string
		birthTime time.Time
	}

	var candidates []candidate
	tolerance := 5 * time.Second

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if len(name) < 7 || name[len(name)-6:] != ".jsonl" {
			continue
		}

		if len(name) > 6 && name[:6] == "agent-" {
			continue
		}

		sessionID := name[:len(name)-6]
		filePath := filepath.Join(t.projectDir, name)

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 使用真正的创建时间
		birthTime := getFileCreationTime(filePath, info)

		// 检查是否在 afterTime 之后创建
		if birthTime.Add(tolerance).After(afterTime) && sessionID != t.currentSessionID {
			candidates = append(candidates, candidate{
				sessionID: sessionID,
				filePath:  filePath,
				birthTime: birthTime,
			})
			fmt.Printf("    候选: %s (birth: %s)\n", sessionID[:8]+"...", birthTime.Format("15:04:05"))
		}
	}

	if len(candidates) == 0 {
		return "", ""
	}

	// 选择最早创建的
	earliest := candidates[0]
	for _, c := range candidates[1:] {
		if c.birthTime.Before(earliest.birthTime) {
			earliest = c
		}
	}

	return earliest.sessionID, earliest.filePath
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "real" {
		// 基于 sessionId 的监控测试（不依赖 PID）
		homeDir, _ := os.UserHomeDir()
		projectDir := filepath.Join(homeDir, ".claude", "projects", "D--codes-2025-aicode-kanban")

		// 模拟 PID 35592 的场景：初始绑定 6418a1a3，然后检测切换
		// 已知的 3 个会话：
		// 1. 6418a1a3 (Birth: 04:56:07, Mod: 04:56:55) - "111"
		// 2. de1f9f14 (Birth: 04:57:12, Mod: 04:57:19) - "222"
		// 3. ae662da3 (Birth: 05:00:28, Mod: 05:00:37) - "333"

		testSessionBasedSwitch(projectDir)
		return
	}

	// 原有的模拟测试...
	// 创建临时测试目录
	testDir, err := os.MkdirTemp("", "session_switch_test")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(testDir)

	fmt.Printf("=== Session Switch Test ===\n")
	fmt.Printf("Test directory: %s\n\n", testDir)

	// 场景1: 正常情况 - 文件持续更新
	fmt.Printf("=== 场景1: 正常情况 ===\n")
	{
		session1 := "session-001"
		createTestSessionFile(testDir, session1, []map[string]interface{}{
			{"type": "user", "message": "hello"},
		})

		tester := NewSessionSwitchTester(testDir)
		tester.SetCurrentSession(session1, filepath.Join(testDir, session1+".jsonl"))

		// 模拟用户输入后文件更新
		time.Sleep(100 * time.Millisecond)
		userInputTime := time.Now()

		// 模拟文件更新
		time.Sleep(100 * time.Millisecond)
		f, _ := os.OpenFile(filepath.Join(testDir, session1+".jsonl"), os.O_APPEND|os.O_WRONLY, 0644)
		f.WriteString(`{"type": "assistant", "message": "hi"}` + "\n")
		f.Close()

		needSwitch, _, _ := tester.CheckForSessionSwitch(userInputTime)
		if !needSwitch {
			fmt.Printf("✓ 正确：无需切换 session\n")
		} else {
			fmt.Printf("✗ 错误：不应该切换 session\n")
		}
	}

	fmt.Printf("\n=== 场景2: /new 创建新会话 ===\n")
	{
		session1 := "session-002"
		createTestSessionFile(testDir, session1, []map[string]interface{}{
			{"type": "user", "message": "hello"},
		})

		tester := NewSessionSwitchTester(testDir)
		tester.SetCurrentSession(session1, filepath.Join(testDir, session1+".jsonl"))

		// 模拟用户输入 /new
		time.Sleep(200 * time.Millisecond)
		userInputTime := time.Now()

		// 模拟创建新的 session 文件（旧文件不更新）
		time.Sleep(200 * time.Millisecond)
		session2 := "session-003-new"
		createTestSessionFile(testDir, session2, []map[string]interface{}{
			{"type": "user", "message": "new session"},
		})

		needSwitch, newSessionID, newFilePath := tester.CheckForSessionSwitch(userInputTime)
		if needSwitch {
			fmt.Printf("✓ 正确：检测到需要切换 session\n")
			tester.SwitchToNewSession(newSessionID, newFilePath)
		} else {
			fmt.Printf("✗ 错误：应该检测到新 session\n")
		}
	}

	fmt.Printf("\n=== 场景3: 连续多次 /new ===\n")
	{
		// 模拟用户连续执行多次 /new
		session1 := "session-multi-1"
		createTestSessionFile(testDir, session1, []map[string]interface{}{
			{"type": "user", "message": "first"},
		})

		tester := NewSessionSwitchTester(testDir)
		tester.SetCurrentSession(session1, filepath.Join(testDir, session1+".jsonl"))

		// 第一次 /new
		time.Sleep(200 * time.Millisecond)
		userInputTime1 := time.Now()
		time.Sleep(200 * time.Millisecond)
		session2 := "session-multi-2"
		createTestSessionFile(testDir, session2, []map[string]interface{}{
			{"type": "user", "message": "second"},
		})

		needSwitch, newSessionID, newFilePath := tester.CheckForSessionSwitch(userInputTime1)
		if needSwitch {
			fmt.Printf("✓ 第一次切换成功\n")
			tester.SwitchToNewSession(newSessionID, newFilePath)
		}

		// 第二次 /new
		time.Sleep(200 * time.Millisecond)
		userInputTime2 := time.Now()
		time.Sleep(200 * time.Millisecond)
		session3 := "session-multi-3"
		createTestSessionFile(testDir, session3, []map[string]interface{}{
			{"type": "user", "message": "third"},
		})

		needSwitch, newSessionID, newFilePath = tester.CheckForSessionSwitch(userInputTime2)
		if needSwitch {
			fmt.Printf("✓ 第二次切换成功\n")
			tester.SwitchToNewSession(newSessionID, newFilePath)
		}
	}

	fmt.Printf("\n=== 测试完成 ===\n")
}
