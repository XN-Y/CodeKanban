//go:build ignore

package main

import (
	"fmt"
	"os/exec"

	"github.com/google/shlex"
)

func checkShellAvailable(command string) bool {
	parts, err := shlex.Split(command)
	if err != nil || len(parts) == 0 {
		return false
	}
	_, err = exec.LookPath(parts[0])
	return err == nil
}

func main() {
	shells := []string{
		"pwsh.exe -NoLogo",
		"powershell.exe -NoLogo",
		"cmd.exe",
		"bash.exe",
		"wsl.exe -e bash",
	}

	for _, shell := range shells {
		available := checkShellAvailable(shell)
		parts, _ := shlex.Split(shell)
		path, err := exec.LookPath(parts[0])
		fmt.Printf("%s: available=%v, path=%v, err=%v\n", shell, available, path, err)
	}
}
