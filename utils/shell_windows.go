//go:build windows

package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// quotePathIfNeeded wraps a path in double quotes if it contains spaces
func quotePathIfNeeded(path string) string {
	if strings.Contains(path, " ") {
		return `"` + path + `"`
	}
	return path
}

// getGitBashPath attempts to find Git Bash executable via registry and common paths
func getGitBashPath() string {
	// Try registry first (64-bit and 32-bit locations)
	for _, key := range []string{
		`SOFTWARE\GitForWindows`,
		`SOFTWARE\Wow6432Node\GitForWindows`,
	} {
		if path := getGitBashFromRegistry(registry.LOCAL_MACHINE, key); path != "" {
			return path
		}
	}

	// Try common installation paths
	commonPaths := []string{
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
		`C:\Git\bin\bash.exe`,
	}

	// Also try user's home directory
	if home, err := os.UserHomeDir(); err == nil {
		commonPaths = append(commonPaths,
			filepath.Join(home, `AppData\Local\Programs\Git\bin\bash.exe`),
			filepath.Join(home, `scoop\apps\git\current\bin\bash.exe`),
		)
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fall back to PATH lookup
	if path, err := exec.LookPath("bash.exe"); err == nil {
		// Make sure it's Git Bash, not WSL bash
		if strings.Contains(strings.ToLower(path), "git") {
			return path
		}
	}

	return ""
}

func getGitBashFromRegistry(root registry.Key, keyPath string) string {
	key, err := registry.OpenKey(root, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()

	installPath, _, err := key.GetStringValue("InstallPath")
	if err != nil {
		return ""
	}

	bashPath := filepath.Join(installPath, "bin", "bash.exe")
	if _, err := os.Stat(bashPath); err == nil {
		return bashPath
	}

	return ""
}

// getPowerShell7Path attempts to find PowerShell 7 (pwsh.exe) via registry and common paths
func getPowerShell7Path() string {
	// Try App Paths registry key
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\pwsh.exe`,
		registry.QUERY_VALUE,
	)
	if err == nil {
		defer key.Close()
		if path, _, err := key.GetStringValue(""); err == nil && path != "" {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	// Try common installation paths
	commonPaths := []string{
		`C:\Program Files\PowerShell\7\pwsh.exe`,
		`C:\Program Files (x86)\PowerShell\7\pwsh.exe`,
	}

	// Also try user's home directory for winget/scoop installs
	if home, err := os.UserHomeDir(); err == nil {
		commonPaths = append(commonPaths,
			filepath.Join(home, `AppData\Local\Microsoft\WindowsApps\pwsh.exe`),
			filepath.Join(home, `scoop\apps\pwsh\current\pwsh.exe`),
		)
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fall back to PATH lookup
	if path, err := exec.LookPath("pwsh.exe"); err == nil {
		return path
	}

	return ""
}

// getWindowsPowerShellPath returns the path to Windows PowerShell 5.x
func getWindowsPowerShellPath() string {
	// Windows PowerShell is always installed at a known location
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}

	psPath := filepath.Join(systemRoot, `System32\WindowsPowerShell\v1.0\powershell.exe`)
	if _, err := os.Stat(psPath); err == nil {
		return psPath
	}

	// Fallback to PATH
	if path, err := exec.LookPath("powershell.exe"); err == nil {
		return path
	}

	return ""
}

// getCmdPath returns the path to cmd.exe
func getCmdPath() string {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}

	cmdPath := filepath.Join(systemRoot, `System32\cmd.exe`)
	if _, err := os.Stat(cmdPath); err == nil {
		return cmdPath
	}

	// Fallback to PATH
	if path, err := exec.LookPath("cmd.exe"); err == nil {
		return path
	}

	return ""
}

// getWSLPath returns the path to wsl.exe if available
func getWSLPath() string {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}

	wslPath := filepath.Join(systemRoot, `System32\wsl.exe`)
	if _, err := os.Stat(wslPath); err == nil {
		return wslPath
	}

	// Fallback to PATH
	if path, err := exec.LookPath("wsl.exe"); err == nil {
		return path
	}

	return ""
}

// getWindowsShellOptionsEnhanced returns shell options with enhanced detection via registry
func getWindowsShellOptionsEnhanced() []ShellOption {
	options := []ShellOption{}

	// PowerShell 7 (pwsh)
	if pwshPath := getPowerShell7Path(); pwshPath != "" {
		options = append(options, ShellOption{
			ID:          "pwsh",
			Name:        "PowerShell 7",
			Command:     quotePathIfNeeded(pwshPath) + " -NoLogo",
			Available:   true,
			Description: "Modern cross-platform PowerShell",
		})
	} else {
		options = append(options, ShellOption{
			ID:          "pwsh",
			Name:        "PowerShell 7",
			Command:     "pwsh.exe -NoLogo",
			Available:   false,
			Description: "Modern cross-platform PowerShell",
		})
	}

	// Windows PowerShell 5.x
	if psPath := getWindowsPowerShellPath(); psPath != "" {
		options = append(options, ShellOption{
			ID:          "powershell",
			Name:        "Windows PowerShell",
			Command:     quotePathIfNeeded(psPath) + " -NoLogo",
			Available:   true,
			Description: "Built-in Windows PowerShell 5.x",
		})
	} else {
		options = append(options, ShellOption{
			ID:          "powershell",
			Name:        "Windows PowerShell",
			Command:     "powershell.exe -NoLogo",
			Available:   false,
			Description: "Built-in Windows PowerShell 5.x",
		})
	}

	// CMD
	if cmdPath := getCmdPath(); cmdPath != "" {
		options = append(options, ShellOption{
			ID:          "cmd",
			Name:        "CMD",
			Command:     quotePathIfNeeded(cmdPath),
			Available:   true,
			Description: "Classic Windows Command Prompt",
		})
	} else {
		options = append(options, ShellOption{
			ID:          "cmd",
			Name:        "CMD",
			Command:     "cmd.exe",
			Available:   false,
			Description: "Classic Windows Command Prompt",
		})
	}

	// Git Bash
	gitBashWarning := "gitbash_ai_detection_warning" // Warning key for i18n translation
	if gitBashPath := getGitBashPath(); gitBashPath != "" {
		options = append(options, ShellOption{
			ID:          "gitbash",
			Name:        "Git Bash",
			Command:     quotePathIfNeeded(gitBashPath) + " --login -i",
			Available:   true,
			Description: "Git for Windows Bash shell",
			Warning:     gitBashWarning,
		})
	} else {
		options = append(options, ShellOption{
			ID:          "gitbash",
			Name:        "Git Bash",
			Command:     "bash.exe",
			Available:   false,
			Description: "Git for Windows Bash shell",
			Warning:     gitBashWarning,
		})
	}

	// WSL
	if wslPath := getWSLPath(); wslPath != "" {
		// Check if WSL is actually installed and configured
		if checkWSLInstalled() {
			options = append(options, ShellOption{
				ID:          "wsl",
				Name:        "WSL2 Bash",
				Command:     quotePathIfNeeded(wslPath) + " -e bash",
				Available:   true,
				Description: "Windows Subsystem for Linux",
			})
		} else {
			options = append(options, ShellOption{
				ID:          "wsl",
				Name:        "WSL2 Bash",
				Command:     "wsl.exe -e bash",
				Available:   false,
				Description: "Windows Subsystem for Linux (not configured)",
			})
		}
	} else {
		options = append(options, ShellOption{
			ID:          "wsl",
			Name:        "WSL2 Bash",
			Command:     "wsl.exe -e bash",
			Available:   false,
			Description: "Windows Subsystem for Linux",
		})
	}

	return options
}

// checkWSLInstalled checks if WSL has at least one distribution installed
func checkWSLInstalled() bool {
	cmd := exec.Command("wsl.exe", "--list", "--quiet")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	// Check if there's any output (distro names)
	return len(strings.TrimSpace(string(output))) > 0
}

// getUnixShellOptionsEnhanced is only available on Unix systems
// This stub exists to satisfy the compiler on Windows
func getUnixShellOptionsEnhanced() []ShellOption {
	return nil
}
