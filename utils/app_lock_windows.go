//go:build windows

package utils

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

var errAppLockBusy = errors.New("app lock is already held")

func lockFile(file *os.File) error {
	if file == nil {
		return os.ErrInvalid
	}

	handle := windows.Handle(file.Fd())
	overlapped := new(windows.Overlapped)
	err := windows.LockFileEx(
		handle,
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		overlapped,
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return errAppLockBusy
		}
		return err
	}
	return nil
}

func unlockFile(file *os.File) error {
	if file == nil {
		return nil
	}

	handle := windows.Handle(file.Fd())
	overlapped := new(windows.Overlapped)
	return windows.UnlockFileEx(handle, 0, 1, 0, overlapped)
}
