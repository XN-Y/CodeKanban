package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const appLockFileName = "app.lock"

var ErrAppInstanceLocked = errors.New("application data directory is already in use")

type AppInstanceLockedError struct {
	DataDir  string
	LockPath string
}

func (e *AppInstanceLockedError) Error() string {
	target := strings.TrimSpace(e.DataDir)
	if target == "" {
		target = strings.TrimSpace(e.LockPath)
	}
	if target == "" {
		return "another CodeKanban instance is already using this data directory"
	}
	return fmt.Sprintf("another CodeKanban instance is already using data directory %s", target)
}

func (e *AppInstanceLockedError) Unwrap() error {
	return ErrAppInstanceLocked
}

type AppInstanceLock struct {
	path string
	file *os.File
}

func (l *AppInstanceLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}

	file := l.file
	l.file = nil

	errUnlock := unlockFile(file)
	errClose := file.Close()
	return errors.Join(errUnlock, errClose)
}

func AcquireAppInstanceLock(dataDir string) (*AppInstanceLock, error) {
	trimmedDir := strings.TrimSpace(dataDir)
	if trimmedDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}

	if err := os.MkdirAll(trimmedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory %s: %w", trimmedDir, err)
	}

	lockPath := filepath.Join(trimmedDir, appLockFileName)
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open app lock file %s: %w", lockPath, err)
	}

	if err := lockFile(file); err != nil {
		_ = file.Close()
		if errors.Is(err, errAppLockBusy) {
			return nil, &AppInstanceLockedError{
				DataDir:  trimmedDir,
				LockPath: lockPath,
			}
		}
		return nil, fmt.Errorf("acquire app lock %s: %w", lockPath, err)
	}

	if err := writeAppLockMetadata(file); err != nil {
		_ = unlockFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("write app lock metadata %s: %w", lockPath, err)
	}

	return &AppInstanceLock{
		path: lockPath,
		file: file,
	}, nil
}

func writeAppLockMetadata(file *os.File) error {
	if file == nil {
		return fmt.Errorf("lock file is nil")
	}

	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}

	_, err := fmt.Fprintf(file, "pid=%d\nstarted_at=%s\n", os.Getpid(), time.Now().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return file.Sync()
}
