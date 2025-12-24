//go:build !windows && !darwin && !linux

package utils

// Stubs for platforms other than Windows, macOS, and Linux

func getWindowsShellOptionsEnhanced() []ShellOption {
	return nil
}

func getUnixShellOptionsEnhanced() []ShellOption {
	return nil
}
