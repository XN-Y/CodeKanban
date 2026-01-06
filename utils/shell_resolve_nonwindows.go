//go:build !windows

package utils

func resolveWindowsShellBinary(name string) string {
	return ""
}
