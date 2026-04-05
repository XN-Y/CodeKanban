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
		fmt.Println("Usage: go run check_cwd.go <pid>")
		os.Exit(1)
	}

	pid, _ := strconv.ParseInt(os.Args[1], 10, 32)

	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		fmt.Printf("Process %d not found\n", pid)
		return
	}

	name, _ := proc.Name()
	cwd, cwdErr := proc.Cwd()

	fmt.Printf("PID: %d\n", pid)
	fmt.Printf("Name: %s\n", name)
	if cwdErr != nil {
		fmt.Printf("CWD: <error: %v>\n", cwdErr)
	} else {
		fmt.Printf("CWD: %s\n", cwd)
	}
}
