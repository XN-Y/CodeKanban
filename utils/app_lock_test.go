package utils

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	appLockHelperEnv    = "GO_WANT_APP_LOCK_HELPER"
	appLockHelperDirEnv = "CODEKANBAN_APP_LOCK_HELPER_DIR"
)

func TestAcquireAppInstanceLockBlocksConcurrentProcess(t *testing.T) {
	t.Helper()

	dataDir := t.TempDir()
	helper := startAppLockHelper(t, dataDir)
	defer helper.stop(t)

	_, err := AcquireAppInstanceLock(dataDir)
	if err == nil {
		t.Fatalf("expected concurrent acquire to fail")
	}

	var lockedErr *AppInstanceLockedError
	if !errors.As(err, &lockedErr) {
		t.Fatalf("expected AppInstanceLockedError, got %v", err)
	}
	if !errors.Is(err, ErrAppInstanceLocked) {
		t.Fatalf("expected ErrAppInstanceLocked, got %v", err)
	}
	if lockedErr.DataDir != dataDir {
		t.Fatalf("expected data dir %q, got %q", dataDir, lockedErr.DataDir)
	}
	if lockedErr.LockPath != filepath.Join(dataDir, appLockFileName) {
		t.Fatalf("expected lock path %q, got %q", filepath.Join(dataDir, appLockFileName), lockedErr.LockPath)
	}
}

func TestAcquireAppInstanceLockCanBeReacquiredAfterClose(t *testing.T) {
	dataDir := t.TempDir()

	lock, err := AcquireAppInstanceLock(dataDir)
	if err != nil {
		t.Fatalf("AcquireAppInstanceLock returned error: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	reacquired, err := AcquireAppInstanceLock(dataDir)
	if err != nil {
		t.Fatalf("reacquire returned error: %v", err)
	}
	defer func() {
		if closeErr := reacquired.Close(); closeErr != nil {
			t.Fatalf("reacquired Close returned error: %v", closeErr)
		}
	}()
}

func TestAppInstanceLockHelperProcess(t *testing.T) {
	if os.Getenv(appLockHelperEnv) != "1" {
		t.Skip("helper process")
	}

	dataDir := os.Getenv(appLockHelperDirEnv)
	lock, err := AcquireAppInstanceLock(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "helper failed to acquire app lock: %v\n", err)
		os.Exit(2)
	}
	defer func() {
		_ = lock.Close()
	}()

	if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
		fmt.Fprintf(os.Stderr, "helper failed to signal readiness: %v\n", err)
		os.Exit(3)
	}

	_, _ = io.ReadAll(os.Stdin)
	os.Exit(0)
}

type appLockHelper struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

func startAppLockHelper(t *testing.T, dataDir string) *appLockHelper {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestAppInstanceLockHelperProcess")
	cmd.Env = append(os.Environ(),
		appLockHelperEnv+"=1",
		appLockHelperDirEnv+"="+dataDir,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe returned error: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("StderrPipe returned error: %v", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe returned error: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		stderrBytes, _ := io.ReadAll(stderr)
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatalf("helper did not become ready: %v; stderr=%s", err, strings.TrimSpace(string(stderrBytes)))
	}
	if strings.TrimSpace(line) != "ready" {
		stderrBytes, _ := io.ReadAll(stderr)
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatalf("unexpected helper ready line %q; stderr=%s", strings.TrimSpace(line), strings.TrimSpace(string(stderrBytes)))
	}

	return &appLockHelper{
		cmd:   cmd,
		stdin: stdin,
	}
}

func (h *appLockHelper) stop(t *testing.T) {
	t.Helper()
	if h == nil || h.cmd == nil {
		return
	}
	if h.stdin != nil {
		_ = h.stdin.Close()
		h.stdin = nil
	}
	if err := h.cmd.Wait(); err != nil {
		t.Fatalf("helper process exited with error: %v", err)
	}
}
