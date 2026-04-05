package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

type ProcessInfo struct {
	PID        int32          `json:"pid"`
	Name       string         `json:"name,omitempty"`
	CreateTime int64          `json:"create_time"`
	StartTime  string         `json:"start_time"`
	Running    int64          `json:"running_seconds"`
	Children   []*ProcessInfo `json:"children,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <pid> [--json] [--children]\n", os.Args[0])
		os.Exit(1)
	}

	pid, err := strconv.ParseInt(os.Args[1], 10, 32)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid PID: %v\n", err)
		os.Exit(1)
	}

	var jsonOutput, showChildren bool
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--children":
			showChildren = true
		}
	}

	info, err := getProcessInfo(int32(pid), showChildren)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(info)
	} else {
		printProcessInfo(info, 0)
	}
}

func getProcessInfo(pid int32, withChildren bool) (*ProcessInfo, error) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("process %d not found: %v", pid, err)
	}

	createTime, err := p.CreateTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get create time: %v", err)
	}

	startTime := time.UnixMilli(createTime)
	name, _ := p.Name()
	running := time.Since(startTime)

	info := &ProcessInfo{
		PID:        pid,
		Name:       name,
		CreateTime: createTime,
		StartTime:  startTime.Format(time.RFC3339),
		Running:    int64(running.Seconds()),
	}

	if withChildren {
		children, _ := p.Children()
		for _, child := range children {
			childInfo, err := getProcessInfo(child.Pid, true)
			if err == nil {
				info.Children = append(info.Children, childInfo)
			}
		}
	}

	return info, nil
}

func printProcessInfo(info *ProcessInfo, indent int) {
	prefix := ""
	for i := 0; i < indent; i++ {
		prefix += "  "
	}

	fmt.Printf("%sPID:        %d\n", prefix, info.PID)
	if info.Name != "" {
		fmt.Printf("%sName:       %s\n", prefix, info.Name)
	}
	fmt.Printf("%sStarted:    %s\n", prefix, info.StartTime)
	fmt.Printf("%sTimestamp:  %d (ms)\n", prefix, info.CreateTime)
	fmt.Printf("%sRunning:    %s\n", prefix, formatDuration(time.Duration(info.Running)*time.Second))

	if len(info.Children) > 0 {
		fmt.Printf("%sChildren:   %d\n", prefix, len(info.Children))
		for i, child := range info.Children {
			fmt.Printf("%s  [%d] ─────────────────\n", prefix, i+1)
			printProcessInfo(child, indent+2)
		}
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
