//go:build windows

package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveShellCommand_WindowsDoesNotRequirePATH(t *testing.T) {
	t.Setenv("PATH", "")

	parts, err := ResolveShellCommand("", TerminalShellConfig{Windows: "cmd.exe"})
	if err != nil {
		t.Fatalf("ResolveShellCommand(cmd.exe) failed: %v", err)
	}
	if len(parts) == 0 {
		t.Fatalf("ResolveShellCommand(cmd.exe) returned empty parts")
	}
	if !strings.EqualFold(filepath.Base(parts[0]), "cmd.exe") {
		t.Fatalf("expected cmd.exe, got %q", parts[0])
	}
	if _, err := os.Stat(parts[0]); err != nil {
		t.Fatalf("resolved cmd.exe path does not exist: %q: %v", parts[0], err)
	}

	parts, err = ResolveShellCommand("", TerminalShellConfig{Windows: "powershell.exe -NoLogo"})
	if err != nil {
		t.Fatalf("ResolveShellCommand(powershell.exe) failed: %v", err)
	}
	if len(parts) < 1 {
		t.Fatalf("ResolveShellCommand(powershell.exe) returned empty parts")
	}
	if !strings.EqualFold(filepath.Base(parts[0]), "powershell.exe") {
		t.Fatalf("expected powershell.exe, got %q", parts[0])
	}
	if _, err := os.Stat(parts[0]); err != nil {
		t.Fatalf("resolved powershell.exe path does not exist: %q: %v", parts[0], err)
	}
}
