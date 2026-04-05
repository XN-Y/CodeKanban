//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/shirou/gopsutil/v4/process"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check_parent.go <parent_pid>")
		os.Exit(1)
	}

	parentPid, _ := strconv.ParseInt(os.Args[1], 10, 32)

	fmt.Printf("Finding children of PID %d using Ppid()...\n\n", parentPid)

	procs, err := process.Processes()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	count := 0
	for _, proc := range procs {
		ppid, err := proc.Ppid()
		if err != nil {
			continue
		}
		if ppid == int32(parentPid) {
			name, _ := proc.Name()
			cmdline, _ := proc.Cmdline()
			fmt.Printf("  [%d] %s\n", proc.Pid, name)
			fmt.Printf("      cmd: %s\n", cmdline)
			count++
		}
	}

	fmt.Printf("\nTotal children found: %d\n", count)
}
