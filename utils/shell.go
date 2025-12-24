package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/google/shlex"
)

// splitShellCommand splits a shell command string into parts.
// On Windows, it handles quoted paths with backslashes correctly.
// On Unix, it uses shlex for proper shell parsing.
func splitShellCommand(command string) ([]string, error) {
	if runtime.GOOS == "windows" {
		return splitWindowsCommand(command)
	}
	return shlex.Split(command)
}

// splitWindowsCommand parses a Windows command line, handling quoted paths correctly.
func splitWindowsCommand(command string) ([]string, error) {
	var parts []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
		case c == ' ' && !inQuotes:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	if inQuotes {
		return nil, fmt.Errorf("unclosed quote in command: %s", command)
	}

	return parts, nil
}

// ResolveShellCommand selects an available shell command for the current host.
// override takes precedence. When override is empty, the configured shell plus
// platform defaults are probed in order until a binary is found.
func ResolveShellCommand(override string, cfg TerminalShellConfig) ([]string, error) {
	override = strings.TrimSpace(override)
	if override != "" {
		return parsePreferredShell(override)
	}

	candidates := buildShellCandidates(cfg)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no shell candidates configured for %s", runtime.GOOS)
	}

	var attempted []string
	for _, candidate := range candidates {
		parts, err := splitShellCommand(candidate)
		if err != nil {
			return nil, fmt.Errorf("invalid shell specification %q: %w", candidate, err)
		}
		if len(parts) == 0 {
			continue
		}
		if err := ensureExecutable(parts[0]); err != nil {
			attempted = append(attempted, parts[0])
			continue
		}
		return parts, nil
	}

	if len(attempted) > 0 {
		return nil, fmt.Errorf("no suitable shell found for %s (tried %s)", runtime.GOOS, strings.Join(attempted, ", "))
	}
	return nil, fmt.Errorf("no suitable shell found for %s", runtime.GOOS)
}

func parsePreferredShell(raw string) ([]string, error) {
	parts, err := splitShellCommand(raw)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid shell configuration: %q", raw)
	}
	if err := ensureExecutable(parts[0]); err != nil {
		return nil, fmt.Errorf("shell %q not found: %w", parts[0], err)
	}
	return parts, nil
}

func buildShellCandidates(cfg TerminalShellConfig) []string {
	var candidates []string
	appendCandidate := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, raw) {
				return
			}
		}
		candidates = append(candidates, raw)
	}

	switch runtime.GOOS {
	case "windows":
		appendCandidate(cfg.Windows)
		appendCandidate("pwsh.exe -NoLogo")
		appendCandidate("powershell.exe -NoLogo")
		appendCandidate("cmd.exe")
	case "darwin":
		appendCandidate(cfg.Darwin)
		appendCandidate("/bin/zsh")
		appendCandidate("/bin/bash")
		appendCandidate("/bin/sh")
	default:
		appendCandidate(cfg.Linux)
		appendCandidate("/bin/bash")
		appendCandidate("/bin/sh")
	}

	return candidates
}

func ensureExecutable(name string) error {
	_, err := exec.LookPath(name)
	return err
}

// ShellOption represents an available shell option for the UI
type ShellOption struct {
	ID          string `json:"id"`                    // Unique identifier (e.g., "pwsh", "cmd", "bash")
	Name        string `json:"name"`                  // Display name (e.g., "PowerShell 7", "CMD")
	Command     string `json:"command"`               // Full command (e.g., "pwsh.exe -NoLogo")
	Available   bool   `json:"available"`             // Whether the shell is available on this system
	Description string `json:"description"`           // Brief description
	Warning     string `json:"warning,omitempty"`     // Optional warning message (e.g., for limited features)
}

// AvailableShellsResponse contains available shells and current settings
type AvailableShellsResponse struct {
	Platform      string        `json:"platform"`      // Current platform: windows, darwin, linux
	CurrentShell  string        `json:"currentShell"`  // Currently configured shell
	DefaultShell  string        `json:"defaultShell"`  // Default/auto shell
	Options       []ShellOption `json:"options"`       // Available shell options
	CustomAllowed bool          `json:"customAllowed"` // Whether custom shell input is allowed
}

// GetAvailableShells returns available shell options for the current platform
func GetAvailableShells(cfg TerminalShellConfig) AvailableShellsResponse {
	resp := AvailableShellsResponse{
		Platform:      runtime.GOOS,
		CustomAllowed: true,
	}

	// Get current configured shell based on platform
	switch runtime.GOOS {
	case "windows":
		resp.CurrentShell = cfg.Windows
		resp.Options = getWindowsShellOptions()
	case "darwin":
		resp.CurrentShell = cfg.Darwin
		resp.Options = getDarwinShellOptions()
	default:
		resp.CurrentShell = cfg.Linux
		resp.Options = getLinuxShellOptions()
	}

	// Determine the default/auto shell (first available)
	for _, opt := range resp.Options {
		if opt.Available {
			resp.DefaultShell = opt.Command
			break
		}
	}

	return resp
}

func getWindowsShellOptions() []ShellOption {
	// Use enhanced detection via registry and known paths
	if options := getWindowsShellOptionsEnhanced(); len(options) > 0 {
		return options
	}

	// Fallback to simple PATH-based detection
	options := []ShellOption{
		{
			ID:          "pwsh",
			Name:        "PowerShell 7",
			Command:     "pwsh.exe -NoLogo",
			Description: "Modern cross-platform PowerShell",
		},
		{
			ID:          "powershell",
			Name:        "Windows PowerShell",
			Command:     "powershell.exe -NoLogo",
			Description: "Built-in Windows PowerShell 5.x",
		},
		{
			ID:          "cmd",
			Name:        "CMD",
			Command:     "cmd.exe",
			Description: "Classic Windows Command Prompt",
		},
		{
			ID:          "gitbash",
			Name:        "Git Bash",
			Command:     "bash.exe",
			Description: "Git for Windows Bash shell",
		},
		{
			ID:          "wsl",
			Name:        "WSL2 Bash",
			Command:     "wsl.exe -e bash",
			Description: "Windows Subsystem for Linux",
		},
	}

	// Check availability
	for i := range options {
		options[i].Available = checkShellAvailable(options[i].Command)
	}

	return options
}

func getDarwinShellOptions() []ShellOption {
	// Use enhanced detection via multiple paths
	if options := getUnixShellOptionsEnhanced(); len(options) > 0 {
		return options
	}

	// Fallback to simple detection
	options := []ShellOption{
		{
			ID:          "zsh",
			Name:        "Zsh",
			Command:     "/bin/zsh",
			Description: "Default macOS shell",
		},
		{
			ID:          "bash",
			Name:        "Bash",
			Command:     "/bin/bash",
			Description: "Bourne Again Shell",
		},
		{
			ID:          "fish",
			Name:        "Fish",
			Command:     "/usr/local/bin/fish",
			Description: "Friendly Interactive Shell",
		},
		{
			ID:          "sh",
			Name:        "sh",
			Command:     "/bin/sh",
			Description: "POSIX shell",
		},
	}

	// Check availability
	for i := range options {
		options[i].Available = checkShellAvailable(options[i].Command)
	}

	// Also check Homebrew fish location
	if !options[2].Available {
		if checkShellAvailable("/opt/homebrew/bin/fish") {
			options[2].Command = "/opt/homebrew/bin/fish"
			options[2].Available = true
		}
	}

	return options
}

func getLinuxShellOptions() []ShellOption {
	// Use enhanced detection via multiple paths
	if options := getUnixShellOptionsEnhanced(); len(options) > 0 {
		return options
	}

	// Fallback to simple detection
	options := []ShellOption{
		{
			ID:          "bash",
			Name:        "Bash",
			Command:     "/bin/bash",
			Description: "Bourne Again Shell",
		},
		{
			ID:          "zsh",
			Name:        "Zsh",
			Command:     "/bin/zsh",
			Description: "Z Shell",
		},
		{
			ID:          "fish",
			Name:        "Fish",
			Command:     "/usr/bin/fish",
			Description: "Friendly Interactive Shell",
		},
		{
			ID:          "sh",
			Name:        "sh",
			Command:     "/bin/sh",
			Description: "POSIX shell",
		},
	}

	// Check availability
	for i := range options {
		options[i].Available = checkShellAvailable(options[i].Command)
	}

	// Check alternative zsh location
	if !options[1].Available {
		if checkShellAvailable("/usr/bin/zsh") {
			options[1].Command = "/usr/bin/zsh"
			options[1].Available = true
		}
	}

	return options
}

func checkShellAvailable(command string) bool {
	parts, err := splitShellCommand(command)
	if err != nil || len(parts) == 0 {
		return false
	}
	return ensureExecutable(parts[0]) == nil
}

// ValidateShellCommand checks if a shell command is valid and available
func ValidateShellCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil // Empty means use auto/default
	}
	_, err := parsePreferredShell(command)
	return err
}
