//go:build !windows

package utils

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

var errAppLockBusy = errors.New("app lock is already held")

func lockFile(file *os.File) error {
	if file == nil {
		return os.ErrInvalid
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) ||
			errors.Is(err, syscall.EWOULDBLOCK) ||
			errors.Is(err, unix.EAGAIN) ||
			errors.Is(err, syscall.EAGAIN) {
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
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
