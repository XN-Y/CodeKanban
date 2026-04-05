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
		fmt.Println("Usage: go run test_cwd_match.go <shell_pid>")
		os.Exit(1)
	}

	shellPid, _ := strconv.ParseInt(os.Args[1], 10, 32)

	// Get shell CWD
	shellProc, err := process.NewProcess(int32(shellPid))
	if err != nil {
		fmt.Printf("Shell process %d not found\n", shellPid)
		return
	}

	shellCwd, err := shellProc.Cwd()
	fmt.Printf("Shell PID: %d\n", shellPid)
	fmt.Printf("Shell CWD: %s (err: %v)\n", shellCwd, err)

	if shellCwd == "" {
		fmt.Println("Cannot get shell CWD, aborting")
		return
	}

	normalizedShellCwd := normalizePath(shellCwd)
	fmt.Printf("Normalized CWD: %s\n\n", normalizedShellCwd)

	// Scan for matching processes
	procs, _ := process.Processes()

	fmt.Println("Scanning for processes with matching CWD...")

	for _, proc := range procs {
		name, _ := proc.Name()
		nameLower := strings.ToLower(name)

		// Only check node.exe, python.exe, codex.exe
		if nameLower != "node.exe" && nameLower != "python.exe" && nameLower != "codex.exe" {
			continue
		}

		procCwd, err := proc.Cwd()
		if err != nil {
			continue
		}

		normalizedProcCwd := normalizePath(procCwd)

		if normalizedProcCwd == normalizedShellCwd {
			cmdline, _ := proc.Cmdline()
			aiInfo := ai_assistant2.DetectFromCommand(cmdline)

			fmt.Printf("\nMATCH: [%d] %s\n", proc.Pid, name)
			fmt.Printf("  CWD: %s\n", procCwd)
			fmt.Printf("  Cmd: %s\n", truncate(cmdline, 80))
			if aiInfo != nil {
				fmt.Printf("  AI: %s\n", aiInfo.DisplayName)
			}
		}
	}
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
