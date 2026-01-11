//go:build windows

package process

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	gopsprocess "github.com/shirou/gopsutil/v4/process"
)

// KillProcessTree best-effort terminates the given process and its descendants.
// It returns nil when the root process doesn't exist.
func KillProcessTree(pid int32) error {
	if pid <= 0 {
		return nil
	}

	// If the process is already gone, treat as success.
	if _, err := gopsprocess.NewProcess(pid); err != nil {
		return nil
	}

	args := []string{"/PID", strconv.FormatInt(int64(pid), 10), "/T", "/F"}
	cmd := exec.Command("taskkill", args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	// Re-check existence: taskkill can fail if the process exited between checks.
	if _, lookupErr := gopsprocess.NewProcess(pid); lookupErr != nil {
		return nil
	}

	return fmt.Errorf("taskkill %v failed: %w (%s)", args, err, strings.TrimSpace(string(output)))
}
