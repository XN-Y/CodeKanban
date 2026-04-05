//go:build windows && ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/shirou/gopsutil/v4/process"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole       = kernel32.NewProc("AttachConsole")
	procFreeConsole         = kernel32.NewProc("FreeConsole")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_console.go <pid>")
		os.Exit(1)
	}

	pid, _ := strconv.ParseInt(os.Args[1], 10, 32)

	fmt.Printf("Testing GetConsoleProcessList for PID %d\n\n", pid)

	// First, free our current console
	procFreeConsole.Call()

	// Attach to the target process's console
	ret, _, err := procAttachConsole.Call(uintptr(pid))
	if ret == 0 {
		fmt.Printf("AttachConsole failed: %v\n", err)
		return
	}
	defer procFreeConsole.Call()

	fmt.Println("Successfully attached to console!")

	// Get list of processes attached to this console
	var pids [128]uint32
	count, _, err := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		uintptr(len(pids)),
	)

	if count == 0 {
		fmt.Printf("GetConsoleProcessList failed: %v\n", err)
		return
	}

	fmt.Printf("Found %d processes attached to this console:\n\n", count)

	for i := 0; i < int(count); i++ {
		procPid := int32(pids[i])
		proc, err := process.NewProcess(procPid)
		if err != nil {
			fmt.Printf("  [%d] <error: %v>\n", procPid, err)
			continue
		}

		name, _ := proc.Name()
		cmdline, _ := proc.Cmdline()
		fmt.Printf("  [%d] %s\n", procPid, name)
		if cmdline != "" {
			fmt.Printf("       cmd: %s\n", truncate(cmdline, 70))
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
