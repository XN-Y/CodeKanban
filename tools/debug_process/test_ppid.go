//go:build ignore

package main

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func main() {
	fmt.Println("Testing Ppid() for all processes...")

	start := time.Now()
	procs, _ := process.Processes()
	fmt.Printf("Got %d processes in %v\n", len(procs), time.Since(start))

	slowCount := 0
	totalTime := time.Duration(0)

	start = time.Now()
	for _, proc := range procs {
		s := time.Now()
		_, err := proc.Ppid()
		elapsed := time.Since(s)
		totalTime += elapsed

		if elapsed > 100*time.Millisecond {
			name, _ := proc.Name()
			fmt.Printf("  [%d] %s Ppid took %v (err: %v)\n", proc.Pid, name, elapsed, err)
			slowCount++
		}
	}

	fmt.Printf("\nTotal Ppid calls: %d\n", len(procs))
	fmt.Printf("Slow calls (>100ms): %d\n", slowCount)
	fmt.Printf("Total time: %v\n", totalTime)
	fmt.Printf("Wall clock: %v\n", time.Since(start))
}
