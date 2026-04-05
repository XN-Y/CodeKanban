//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_timing.go <pid>")
		os.Exit(1)
	}

	pid, _ := strconv.ParseInt(os.Args[1], 10, 32)

	// Get shell CWD
	start := time.Now()
	shellProc, _ := process.NewProcess(int32(pid))
	shellCwd, _ := shellProc.Cwd()
	fmt.Printf("Shell CWD: %s (took %v)\n", shellCwd, time.Since(start))

	shellCwd = normalizePath(shellCwd)

	// List all processes
	start = time.Now()
	procs, _ := process.Processes()
	fmt.Printf("Listed %d processes (took %v)\n", len(procs), time.Since(start))

	// Check each node.exe process
	nodeCount := 0
	cwdCheckTime := time.Duration(0)

	for _, proc := range procs {
		name, _ := proc.Name()
		if strings.ToLower(name) != "node.exe" {
			continue
		}
		nodeCount++

		start = time.Now()
		procCwd, err := proc.Cwd()
		elapsed := time.Since(start)
		cwdCheckTime += elapsed

		if err != nil {
			fmt.Printf("  [%d] node.exe CWD error: %v (took %v)\n", proc.Pid, err, elapsed)
		} else if elapsed > 100*time.Millisecond {
			fmt.Printf("  [%d] node.exe CWD: %s (took %v) SLOW!\n", proc.Pid, truncate(procCwd, 30), elapsed)
		}
	}

	fmt.Printf("\nTotal node.exe processes: %d\n", nodeCount)
	fmt.Printf("Total CWD check time: %v\n", cwdCheckTime)
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
