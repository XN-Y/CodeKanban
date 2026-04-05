//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"code-kanban/utils/ai_assistant2"

	"github.com/shirou/gopsutil/v4/process"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_full_flow.go <pid>")
		os.Exit(1)
	}

	pid, _ := strconv.ParseInt(os.Args[1], 10, 32)

	fmt.Printf("=== Testing Full Flow for PID %d ===\n\n", pid)

	// Step 1: Try child process method
	fmt.Println("Step 1: findForegroundCommandRecursive...")
	cmd := findForegroundCommandRecursive(int32(pid), 0, 5)
	fmt.Printf("  Result: %s\n", truncate(cmd, 60))
	fmt.Printf("  isShellCommand: %v\n\n", isShellCommand(cmd))

	// Step 2: If shell command, try CWD method
	if cmd == "" || isShellCommand(cmd) {
		fmt.Println("Step 2: findCommandByWorkingDirectory...")
		cmd = findCommandByWorkingDirectory(int32(pid))
		fmt.Printf("  Result: %s\n\n", truncate(cmd, 60))
	}

	// Step 3: AI detection
	if cmd != "" {
		aiInfo := ai_assistant2.DetectFromCommand(cmd)
		if aiInfo != nil {
			fmt.Printf("AI Detected: %s\n", aiInfo.DisplayName)
		} else {
			fmt.Println("No AI detected")
		}
	} else {
		fmt.Println("No command found")
	}
}

func findForegroundCommandRecursive(pid int32, depth, maxDepth int) string {
	if pid <= 0 || depth >= maxDepth {
		return ""
	}

	children := getChildrenByParent(pid)
	if len(children) == 0 {
		return ""
	}

	for _, child := range children {
		cmdline, err := child.Cmdline()
		if err != nil || cmdline == "" {
			continue
		}

		if !isShellProcess(child) {
			return cmdline
		}

		if result := findForegroundCommandRecursive(child.Pid, depth+1, maxDepth); result != "" {
			return result
		}
	}

	if len(children) > 0 {
		if cmdline, err := children[0].Cmdline(); err == nil {
			return cmdline
		}
	}

	return ""
}

func findCommandByWorkingDirectory(shellPid int32) string {
	shellProc, err := process.NewProcess(shellPid)
	if err != nil {
		fmt.Printf("  Error: cannot create process: %v\n", err)
		return ""
	}

	shellCwd, err := shellProc.Cwd()
	if err != nil || shellCwd == "" {
		fmt.Printf("  Error: cannot get CWD: %v\n", err)
		return ""
	}

	shellCwd = normalizePath(shellCwd)
	fmt.Printf("  Shell CWD: %s\n", shellCwd)

	procs, err := process.Processes()
	if err != nil {
		fmt.Printf("  Error: cannot list processes: %v\n", err)
		return ""
	}

	matchCount := 0
	for _, proc := range procs {
		if proc.Pid == shellPid {
			continue
		}

		name, err := proc.Name()
		if err != nil {
			continue
		}
		name = strings.ToLower(name)

		if !isPotentialAIProcess(name) {
			continue
		}

		procCwd, err := proc.Cwd()
		if err != nil || procCwd == "" {
			continue
		}

		if normalizePath(procCwd) == shellCwd {
			matchCount++
			if cmdline, err := proc.Cmdline(); err == nil && cmdline != "" {
				fmt.Printf("  Found match [%d] %s: %s\n", proc.Pid, name, truncate(cmdline, 50))
				// Return the first match that is an AI assistant
				if ai_assistant2.DetectFromCommand(cmdline) != nil {
					return cmdline
				}
			}
		}
	}

	fmt.Printf("  Total matches: %d\n", matchCount)
	return ""
}

func getChildrenByParent(parentPid int32) []*process.Process {
	procs, err := process.Processes()
	if err != nil {
		return nil
	}

	var children []*process.Process
	for _, proc := range procs {
		ppid, err := proc.Ppid()
		if err != nil {
			continue
		}
		if ppid == parentPid {
			children = append(children, proc)
		}
	}
	return children
}

func isShellProcess(proc *process.Process) bool {
	name, err := proc.Name()
	if err != nil {
		return false
	}

	name = strings.ToLower(name)

	shellNames := []string{
		"bash", "bash.exe",
		"sh", "sh.exe",
		"zsh", "zsh.exe",
		"fish", "fish.exe",
		"cmd", "cmd.exe",
		"powershell", "powershell.exe",
		"pwsh", "pwsh.exe",
		"wsl", "wsl.exe",
		"conhost", "conhost.exe",
		"mintty", "mintty.exe",
	}

	for _, shell := range shellNames {
		if name == shell {
			return true
		}
	}

	return false
}

func isShellCommand(cmdline string) bool {
	cmdLower := strings.ToLower(cmdline)

	shellPatterns := []string{
		"bash.exe", "bash",
		"sh.exe", "/bin/sh",
		"zsh.exe", "zsh",
		"fish.exe", "fish",
		"cmd.exe",
		"powershell.exe",
		"pwsh.exe",
		"wsl.exe",
	}

	for _, pattern := range shellPatterns {
		if strings.Contains(cmdLower, pattern) {
			return true
		}
	}
	return false
}

func isPotentialAIProcess(name string) bool {
	aiProcessNames := []string{
		"node", "node.exe",
		"python", "python.exe", "python3", "python3.exe",
		"codex", "codex.exe",
	}

	for _, aiName := range aiProcessNames {
		if name == aiName {
			return true
		}
	}
	return false
}

func normalizePath(path string) string {
	path = strings.ToLower(path)
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimSuffix(path, "/")
	return path
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
