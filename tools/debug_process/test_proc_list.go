//go:build ignore

package main

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func main() {
	fmt.Println("Testing process.Processes()...")

	for i := 0; i < 3; i++ {
		start := time.Now()
		procs, err := process.Processes()
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("  Attempt %d: error %v\n", i+1, err)
		} else {
			fmt.Printf("  Attempt %d: %d processes (took %v)\n", i+1, len(procs), elapsed)
		}
	}
}
