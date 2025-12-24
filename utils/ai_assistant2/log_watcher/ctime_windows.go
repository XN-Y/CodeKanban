//go:build windows

package log_watcher

import (
	"os"
	"syscall"
	"time"
)

// getWindowsCreationTime returns the file creation time on Windows
func getWindowsCreationTime(path string, info os.FileInfo) time.Time {
	if info == nil {
		return time.Time{}
	}

	sys := info.Sys()
	if sys == nil {
		return info.ModTime()
	}

	winData, ok := sys.(*syscall.Win32FileAttributeData)
	if !ok {
		return info.ModTime()
	}

	// CreationTime is a Windows FILETIME
	nsec := winData.CreationTime.Nanoseconds()
	return time.Unix(0, nsec)
}
