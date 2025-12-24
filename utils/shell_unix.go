//go:build darwin || linux

package utils

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// shellInfo holds information about a shell to detect
type shellInfo struct {
	ID          string
	Name        string
	Description string
	Paths       []string // Possible paths to check
	LoginArgs   string   // Arguments to add for login shell (optional)
}

// getRegisteredShells reads /etc/shells to get system-registered shells
func getRegisteredShells() map[string]bool {
	shells := make(map[string]bool)

	file, err := os.Open("/etc/shells")
	if err != nil {
		return shells
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			shells[line] = true
		}
	}

	return shells
}

// findShellPath finds the first available path for a shell
func findShellPath(paths []string) string {
	for _, p := range paths {
		// Expand ~ to home directory
		if strings.HasPrefix(p, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				p = filepath.Join(home, p[2:])
			}
		}

		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

// getDarwinShellOptionsEnhanced returns shell options with enhanced detection for macOS
func getDarwinShellOptionsEnhanced() []ShellOption {
	registeredShells := getRegisteredShells()

	// Define shells to detect with their possible paths
	// Order: system paths, Homebrew Apple Silicon, Homebrew Intel, MacPorts
	shellDefs := []shellInfo{
		{
			ID:          "zsh",
			Name:        "Zsh",
			Description: "Default macOS shell",
			Paths: []string{
				"/bin/zsh",
				"/opt/homebrew/bin/zsh",
				"/usr/local/bin/zsh",
			},
		},
		{
			ID:          "bash",
			Name:        "Bash",
			Description: "Bourne Again Shell",
			Paths: []string{
				"/bin/bash",
				"/opt/homebrew/bin/bash", // Homebrew bash (newer version)
				"/usr/local/bin/bash",
			},
		},
		{
			ID:          "fish",
			Name:        "Fish",
			Description: "Friendly Interactive Shell",
			Paths: []string{
				"/opt/homebrew/bin/fish",
				"/usr/local/bin/fish",
				"/opt/local/bin/fish", // MacPorts
			},
		},
		{
			ID:          "nu",
			Name:        "Nushell",
			Description: "Modern shell with structured data",
			Paths: []string{
				"/opt/homebrew/bin/nu",
				"/usr/local/bin/nu",
				"~/.cargo/bin/nu",
			},
		},
		{
			ID:          "elvish",
			Name:        "Elvish",
			Description: "Expressive programming language and shell",
			Paths: []string{
				"/opt/homebrew/bin/elvish",
				"/usr/local/bin/elvish",
			},
		},
		{
			ID:          "sh",
			Name:        "sh",
			Description: "POSIX shell",
			Paths: []string{
				"/bin/sh",
			},
		},
	}

	options := make([]ShellOption, 0, len(shellDefs))

	for _, def := range shellDefs {
		opt := ShellOption{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Available:   false,
		}

		if path := findShellPath(def.Paths); path != "" {
			opt.Command = path
			opt.Available = true

			// Verify it's in /etc/shells (optional validation)
			if _, registered := registeredShells[path]; registered {
				// Shell is properly registered
			}
		} else {
			// Use first path as default command for display
			opt.Command = def.Paths[0]
		}

		options = append(options, opt)
	}

	return options
}

// getLinuxShellOptionsEnhanced returns shell options with enhanced detection for Linux
func getLinuxShellOptionsEnhanced() []ShellOption {
	registeredShells := getRegisteredShells()

	// Define shells to detect with their possible paths
	shellDefs := []shellInfo{
		{
			ID:          "bash",
			Name:        "Bash",
			Description: "Bourne Again Shell",
			Paths: []string{
				"/bin/bash",
				"/usr/bin/bash",
			},
		},
		{
			ID:          "zsh",
			Name:        "Zsh",
			Description: "Z Shell",
			Paths: []string{
				"/bin/zsh",
				"/usr/bin/zsh",
				"/usr/local/bin/zsh",
			},
		},
		{
			ID:          "fish",
			Name:        "Fish",
			Description: "Friendly Interactive Shell",
			Paths: []string{
				"/usr/bin/fish",
				"/bin/fish",
				"/usr/local/bin/fish",
				"~/.local/bin/fish",
			},
		},
		{
			ID:          "nu",
			Name:        "Nushell",
			Description: "Modern shell with structured data",
			Paths: []string{
				"/usr/bin/nu",
				"/usr/local/bin/nu",
				"~/.cargo/bin/nu",
				"~/.local/bin/nu",
			},
		},
		{
			ID:          "elvish",
			Name:        "Elvish",
			Description: "Expressive programming language and shell",
			Paths: []string{
				"/usr/bin/elvish",
				"/usr/local/bin/elvish",
				"~/.local/bin/elvish",
			},
		},
		{
			ID:          "dash",
			Name:        "Dash",
			Description: "Debian Almquist Shell (fast, POSIX)",
			Paths: []string{
				"/bin/dash",
				"/usr/bin/dash",
			},
		},
		{
			ID:          "sh",
			Name:        "sh",
			Description: "POSIX shell",
			Paths: []string{
				"/bin/sh",
				"/usr/bin/sh",
			},
		},
	}

	options := make([]ShellOption, 0, len(shellDefs))

	for _, def := range shellDefs {
		opt := ShellOption{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Available:   false,
		}

		if path := findShellPath(def.Paths); path != "" {
			opt.Command = path
			opt.Available = true

			// Check if registered (informational)
			if _, registered := registeredShells[path]; registered {
				// Shell is properly registered in /etc/shells
			}
		} else {
			// Use first path as default command for display
			opt.Command = def.Paths[0]
		}

		options = append(options, opt)
	}

	return options
}

// getUnixShellOptionsEnhanced dispatches to the appropriate platform-specific function
func getUnixShellOptionsEnhanced() []ShellOption {
	switch runtime.GOOS {
	case "darwin":
		return getDarwinShellOptionsEnhanced()
	case "linux":
		return getLinuxShellOptionsEnhanced()
	default:
		return nil
	}
}

// getUserDefaultShell returns the user's default shell from environment or passwd
func getUserDefaultShell() string {
	// Try SHELL environment variable first
	if shell := os.Getenv("SHELL"); shell != "" {
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}

	// Fall back to getent/id command
	if path, err := exec.LookPath("getent"); err == nil {
		cmd := exec.Command(path, "passwd", os.Getenv("USER"))
		if output, err := cmd.Output(); err == nil {
			parts := strings.Split(string(output), ":")
			if len(parts) >= 7 {
				shell := strings.TrimSpace(parts[6])
				if _, err := os.Stat(shell); err == nil {
					return shell
				}
			}
		}
	}

	return ""
}
