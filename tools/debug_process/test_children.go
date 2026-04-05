//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_children.go <pid>")
		os.Exit(1)
	}

	pid, _ := strconv.ParseInt(os.Args[1], 10, 32)

	fmt.Printf("Testing getChildrenByParent for PID %d\n\n", pid)

	// Test our getChildrenByParent function
	start := time.Now()
	children := getChildrenByParent(int32(pid))
	fmt.Printf("getChildrenByParent: found %d children (took %v)\n", len(children), time.Since(start))

	for _, child := range children {
		name, _ := child.Name()
		fmt.Printf("  [%d] %s\n", child.Pid, name)

		// Test recursive
		start = time.Now()
		grandchildren := getChildrenByParent(child.Pid)
		fmt.Printf("    getChildrenByParent for %d: found %d (took %v)\n", child.Pid, len(grandchildren), time.Since(start))
	}
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
