//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_fg_cmd.go <pid>")
		os.Exit(1)
	}

	pid, _ := strconv.ParseInt(os.Args[1], 10, 32)

	fmt.Printf("Testing findForegroundCommandRecursive for PID %d\n\n", pid)

	result := findForegroundCommandRecursive(int32(pid), 0, 5)
	fmt.Printf("Result: %s\n", result)
}

func findForegroundCommandRecursive(pid int32, depth, maxDepth int) string {
	if pid <= 0 || depth >= maxDepth {
		fmt.Printf("%sDepth limit or invalid PID\n", indent(depth))
		return ""
	}

	children := getChildrenByParent(pid)
	fmt.Printf("%sPID %d has %d children\n", indent(depth), pid, len(children))

	if len(children) == 0 {
		return ""
	}

	for _, child := range children {
		cmdline, err := child.Cmdline()
		if err != nil || cmdline == "" {
			fmt.Printf("%s  Child %d: no cmdline\n", indent(depth), child.Pid)
			continue
		}

		name, _ := child.Name()
		isShell := isShellProcess(child)
		fmt.Printf("%s  Child %d (%s): isShell=%v\n", indent(depth), child.Pid, name, isShell)

		if !isShell {
			fmt.Printf("%s    -> Returning: %s\n", indent(depth), truncate(cmdline, 60))
			return cmdline
		}

		if result := findForegroundCommandRecursive(child.Pid, depth+1, maxDepth); result != "" {
			return result
		}
	}

	if len(children) > 0 {
		if cmdline, err := children[0].Cmdline(); err == nil {
			fmt.Printf("%sFallback to first child cmdline\n", indent(depth))
			return cmdline
		}
	}

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

func indent(depth int) string {
	return strings.Repeat("  ", depth)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
