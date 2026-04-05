//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"code-kanban/utils/ai_assistant2"
	procutil "code-kanban/utils/process"

	"github.com/shirou/gopsutil/v4/process"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <pid>")
		fmt.Println("       go run main.go self")
		fmt.Println("       go run main.go tree <pid>")
		fmt.Println("       go run main.go detect <pid>")
		fmt.Println("       go run main.go scan")
		os.Exit(1)
	}

	arg := os.Args[1]

	if arg == "self" {
		// Show current process and its parent chain
		showSelfAndParents()
		return
	}

	if arg == "scan" {
		scanForAIAssistants()
		return
	}

	if arg == "tree" && len(os.Args) >= 3 {
		pid, err := strconv.ParseInt(os.Args[2], 10, 32)
		if err != nil {
			fmt.Printf("Invalid PID: %s\n", os.Args[2])
			os.Exit(1)
		}
		showProcessTree(int32(pid), 0)
		return
	}

	if arg == "detect" && len(os.Args) >= 3 {
		pid, err := strconv.ParseInt(os.Args[2], 10, 32)
		if err != nil {
			fmt.Printf("Invalid PID: %s\n", os.Args[2])
			os.Exit(1)
		}
		testDetection(int32(pid))
		return
	}

	pid, err := strconv.ParseInt(arg, 10, 32)
	if err != nil {
		fmt.Printf("Invalid PID: %s\n", arg)
		os.Exit(1)
	}

	showProcessInfo(int32(pid))
}

func scanForAIAssistants() {
	fmt.Println("=== Scanning for AI Assistants ===\n")

	procs, err := process.Processes()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for _, proc := range procs {
		cmdline, err := proc.Cmdline()
		if err != nil || cmdline == "" {
			continue
		}

		aiInfo := ai_assistant2.DetectFromCommand(cmdline)
		if aiInfo != nil {
			name, _ := proc.Name()
			ppid, _ := proc.Ppid()
			fmt.Printf("Found: %s\n", aiInfo.DisplayName)
			fmt.Printf("  PID: %d, Name: %s, Parent: %d\n", proc.Pid, name, ppid)
			fmt.Printf("  Cmd: %s\n", truncate(cmdline, 100))

			// Show parent chain
			fmt.Printf("  Parent chain: ")
			showParentChainBrief(ppid)
			fmt.Println()
		}
	}
}

func showParentChainBrief(pid int32) {
	for pid > 0 {
		proc, err := process.NewProcess(pid)
		if err != nil {
			break
		}
		name, _ := proc.Name()
		fmt.Printf("%s(%d)", name, pid)
		ppid, _ := proc.Ppid()
		if ppid > 0 {
			fmt.Printf(" <- ")
		}
		pid = ppid
	}
}

func testDetection(pid int32) {
	fmt.Printf("=== Testing Detection for PID %d ===\n\n", pid)

	// Test GetForegroundCommand with timing
	fmt.Println("Calling GetForegroundCommand...")
	start := time.Now()
	cmd := procutil.GetForegroundCommand(pid)
	elapsed := time.Since(start)
	fmt.Printf("GetForegroundCommand: %s (took %v)\n", cmd, elapsed)

	// Test IsProcessBusy
	busy := procutil.IsProcessBusy(pid)
	fmt.Printf("IsProcessBusy: %v\n", busy)

	// Test GetProcessStatus
	status := procutil.GetProcessStatus(pid)
	fmt.Printf("GetProcessStatus: %s\n", status)

	// Test AI detection
	if cmd != "" {
		aiInfo := ai_assistant2.DetectFromCommand(cmd)
		if aiInfo != nil {
			fmt.Printf("\nAI Assistant Detected:\n")
			fmt.Printf("  Type: %s\n", aiInfo.Type)
			fmt.Printf("  Name: %s\n", aiInfo.Name)
			fmt.Printf("  DisplayName: %s\n", aiInfo.DisplayName)
		} else {
			fmt.Printf("\nNo AI Assistant detected from command\n")
		}
	}
}

func showSelfAndParents() {
	pid := int32(os.Getpid())
	fmt.Printf("=== Current Process Chain ===\n\n")

	for pid > 0 {
		proc, err := process.NewProcess(pid)
		if err != nil {
			break
		}

		name, _ := proc.Name()
		cmdline, _ := proc.Cmdline()
		ppid, _ := proc.Ppid()

		fmt.Printf("PID: %d\n", pid)
		fmt.Printf("  Name: %s\n", name)
		fmt.Printf("  Cmdline: %s\n", truncate(cmdline, 100))
		fmt.Printf("  Parent PID: %d\n", ppid)
		fmt.Println()

		pid = ppid
	}
}

func showProcessInfo(pid int32) {
	proc, err := process.NewProcess(pid)
	if err != nil {
		fmt.Printf("Error: process %d not found: %v\n", pid, err)
		return
	}

	name, _ := proc.Name()
	cmdline, _ := proc.Cmdline()
	ppid, _ := proc.Ppid()
	status, _ := proc.Status()

	fmt.Printf("=== Process %d ===\n", pid)
	fmt.Printf("Name: %s\n", name)
	fmt.Printf("Cmdline: %s\n", cmdline)
	fmt.Printf("Parent PID: %d\n", ppid)
	fmt.Printf("Status: %v\n", status)

	// Get children
	children, err := proc.Children()
	if err != nil {
		fmt.Printf("Children: error getting children: %v\n", err)
	} else {
		fmt.Printf("Children count: %d\n", len(children))
		for i, child := range children {
			childName, _ := child.Name()
			childCmd, _ := child.Cmdline()
			fmt.Printf("  [%d] PID: %d, Name: %s\n", i, child.Pid, childName)
			fmt.Printf("      Cmdline: %s\n", truncate(childCmd, 80))
		}
	}

	fmt.Println("\n=== Full Tree from this PID ===")
	showProcessTree(pid, 0)
}

func showProcessTree(pid int32, depth int) {
	if depth > 10 {
		return
	}

	proc, err := process.NewProcess(pid)
	if err != nil {
		return
	}

	name, _ := proc.Name()
	cmdline, _ := proc.Cmdline()

	indent := strings.Repeat("  ", depth)
	fmt.Printf("%s[%d] %s\n", indent, pid, name)
	if cmdline != "" {
		fmt.Printf("%s     cmd: %s\n", indent, truncate(cmdline, 60))
	}

	children, err := proc.Children()
	if err != nil || len(children) == 0 {
		return
	}

	for _, child := range children {
		showProcessTree(child.Pid, depth+1)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
