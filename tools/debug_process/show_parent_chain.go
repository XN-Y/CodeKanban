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
		fmt.Println("Usage: go run show_parent_chain.go <pid>")
		os.Exit(1)
	}

	pid, _ := strconv.ParseInt(os.Args[1], 10, 32)

	fmt.Printf("Parent chain of PID %d:\n\n", pid)

	for pid > 0 {
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			fmt.Printf("  [%d] <not found>\n", pid)
			break
		}

		name, _ := proc.Name()
		ppid, _ := proc.Ppid()

		fmt.Printf("  [%d] %s (parent: %d)\n", pid, name, ppid)

		pid = int64(ppid)
	}
}
