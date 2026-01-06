package system

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// OpenTerminal launches a terminal window pointing to the provided directory.
func OpenTerminal(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("wt.exe"); err == nil {
			cmd = exec.Command("wt.exe", "-d", path)
		} else {
			cmd = exec.Command("cmd", "/c", "start", "cmd", "/k", fmt.Sprintf("cd /d %s", quoteCmdPath(path)))
		}
	case "darwin":
		script := fmt.Sprintf(`tell application "Terminal"
	do script "cd \"%s\""
	activate
end tell`, escapeAppleScript(path))
		cmd = exec.Command("osascript", "-e", script)
	case "linux":
		terminals := []struct {
			Name string
			Args []string
		}{
			{"gnome-terminal", []string{"--working-directory", path}},
			{"konsole", []string{"--workdir", path}},
			{"xterm", nil},
			{"x-terminal-emulator", nil},
		}

		for _, term := range terminals {
			if _, err := exec.LookPath(term.Name); err == nil {
				if len(term.Args) > 0 {
					cmd = exec.Command(term.Name, term.Args...)
				} else {
					cmd = exec.Command(term.Name)
					cmd.Dir = path
				}
				break
			}
		}

		if cmd == nil {
			return ErrNoTerminal
		}
	default:
		return ErrUnsupportedOS
	}

	return cmd.Start()
}

func escapeAppleScript(path string) string {
	escaped := strings.ReplaceAll(path, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return escaped
}

func quoteCmdPath(path string) string {
	return `"` + strings.ReplaceAll(path, `"`, `""`) + `"`
}
