//go:build darwin

package log_watcher

import (
	"os"
	"syscall"
	"time"
)

// getWindowsCreationTime on macOS returns the file's birth time (true creation time)
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

	// macOS supports Birthtimespec which is the true file creation time
	return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}
