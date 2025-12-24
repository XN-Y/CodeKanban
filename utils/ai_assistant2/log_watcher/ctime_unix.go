//go:build !windows

package log_watcher

import (
	"os"
	"syscall"
	"time"
)

// getWindowsCreationTime on non-Windows systems returns the best available creation time
// On Linux/macOS, this uses ctime (status change time) as a proxy for creation time
func getWindowsCreationTime(path string, info os.FileInfo) time.Time {
	if info == nil {
		return time.Time{}
	}

	sys := info.Sys()
	if sys == nil {
		return info.ModTime()
	}

	stat, ok := sys.(*syscall.Stat_t)
	if !ok {
		return info.ModTime()
	}

	// Try to get birth time (creation time) on systems that support it
	// On macOS (darwin), Birthtimespec contains the creation time
	// On Linux, we fall back to ctime

	// Use ctime (status change time) as a proxy
	// Note: ctime is updated when file metadata changes, not true creation time
	return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
}
