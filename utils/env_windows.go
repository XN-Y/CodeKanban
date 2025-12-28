//go:build windows

package utils

import (
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// GetFreshEnviron returns the current environment variables with PATH refreshed from Windows registry.
// This is useful for terminal managers that need to pick up newly installed tools.
func GetFreshEnviron() []string {
	// Start with current environment
	env := os.Environ()

	// Get fresh PATH from registry
	freshPath := GetFreshPath()
	if freshPath == "" {
		return env
	}

	// Replace PATH in the environment
	result := make([]string, 0, len(env))
	pathReplaced := false
	for _, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			result = append(result, "PATH="+freshPath)
			pathReplaced = true
		} else {
			result = append(result, e)
		}
	}

	if !pathReplaced {
		result = append(result, "PATH="+freshPath)
	}

	return result
}

// GetFreshPath reads and combines PATH from Windows registry (system + user).
// Returns the combined PATH or empty string on error.
func GetFreshPath() string {
	var paths []string

	// System PATH (HKEY_LOCAL_MACHINE)
	if sysPath := getRegistryPath(
		registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control\Session Manager\Environment`,
		"Path",
	); sysPath != "" {
		paths = append(paths, sysPath)
	}

	// User PATH (HKEY_CURRENT_USER)
	if userPath := getRegistryPath(
		registry.CURRENT_USER,
		`Environment`,
		"Path",
	); userPath != "" {
		paths = append(paths, userPath)
	}

	if len(paths) == 0 {
		return ""
	}

	return strings.Join(paths, ";")
}

// getRegistryPath reads a string value from the Windows registry.
func getRegistryPath(root registry.Key, keyPath, valueName string) string {
	key, err := registry.OpenKey(root, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()

	// Try to read as expandable string first (REG_EXPAND_SZ)
	value, valType, err := key.GetStringValue(valueName)
	if err != nil {
		return ""
	}

	// Expand environment variables if it's a REG_EXPAND_SZ type
	if valType == registry.EXPAND_SZ {
		value = os.ExpandEnv(value)
	}

	return value
}

// RefreshProcessEnviron updates the current process's PATH environment variable
// from the Windows registry. Call this if you want all future os.Environ() calls
// to include the updated PATH.
func RefreshProcessEnviron() error {
	freshPath := GetFreshPath()
	if freshPath == "" {
		return nil
	}
	return os.Setenv("PATH", freshPath)
}
