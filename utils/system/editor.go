package system

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/google/shlex"
)

type EditorKind string

const (
	EditorVSCode EditorKind = "vscode"
	EditorCursor EditorKind = "cursor"
	EditorTrae   EditorKind = "trae"
	EditorZed    EditorKind = "zed"
	EditorCustom EditorKind = "custom"
)

const customEditorPathPlaceholder = "${path}"

// OpenEditor attempts to open the provided path inside the requested editor.
// Supported editors are VSCode, Cursor, Trae, Zed and a custom command.
func OpenEditor(path string, editor string, customCommand string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}

	kind := normalizeEditorKind(editor)
	switch kind {
	case EditorVSCode, EditorCursor, EditorTrae, EditorZed:
		return launchKnownEditor(kind, path)
	case EditorCustom:
		return launchCustomEditor(customCommand, path)
	default:
		return ErrUnsupportedEditor
	}
}

func normalizeEditorKind(editor string) EditorKind {
	switch strings.ToLower(strings.TrimSpace(editor)) {
	case string(EditorVSCode):
		return EditorVSCode
	case string(EditorCursor):
		return EditorCursor
	case string(EditorTrae):
		return EditorTrae
	case string(EditorZed):
		return EditorZed
	case string(EditorCustom):
		return EditorCustom
	default:
		return EditorKind(strings.ToLower(strings.TrimSpace(editor)))
	}
}

func launchKnownEditor(kind EditorKind, path string) error {
	candidates := buildEditorCandidates(kind)
	if len(candidates) == 0 {
		return fmt.Errorf("%w: %s", ErrEditorCommandMissing, kind)
	}

	var (
		anyExecutable bool
		lastErr       error
	)

	for _, candidate := range candidates {
		found, err := tryLaunchCommand(candidate.command, append(candidate.args, path)...)
		if !found {
			continue
		}
		anyExecutable = true
		if err == nil {
			return nil
		}
		lastErr = err
	}

	if !anyExecutable {
		return fmt.Errorf("%w: %s", ErrEditorCommandMissing, kind)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("failed to open %s editor", kind)
}

func launchCustomEditor(commandTemplate string, path string) error {
	parts, err := buildCustomEditorCommandParts(commandTemplate, path)
	if err != nil {
		return err
	}

	_, err = tryLaunchCommand(parts[0], parts[1:]...)
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%w: %s", ErrEditorCommandMissing, parts[0])
	}
	return err
}

func buildCustomEditorCommandParts(commandTemplate string, path string) ([]string, error) {
	commandTemplate = strings.TrimSpace(commandTemplate)
	if commandTemplate == "" {
		return nil, ErrCustomEditorCommand
	}

	parts, err := shlex.Split(commandTemplate)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCustomEditorCommand, err)
	}
	if len(parts) == 0 {
		return nil, ErrCustomEditorCommand
	}

	hasPlaceholder := false
	for idx, token := range parts {
		replaced := strings.ReplaceAll(token, customEditorPathPlaceholder, path)
		if replaced != token {
			hasPlaceholder = true
			parts[idx] = replaced
		}
	}
	if !hasPlaceholder {
		parts = append(parts, path)
	}

	return parts, nil
}

type editorCommand struct {
	command string
	args    []string
}

func tryLaunchCommand(command string, args ...string) (bool, error) {
	if strings.TrimSpace(command) == "" {
		return false, nil
	}
	if _, err := exec.LookPath(command); err != nil {
		return false, err
	}
	cmd := exec.Command(command, args...)
	return true, cmd.Start()
}

func buildEditorCandidates(kind EditorKind) []editorCommand {
	var candidates []editorCommand
	seen := map[string]struct{}{}

	add := func(command string, args ...string) {
		command = strings.TrimSpace(command)
		if command == "" {
			return
		}
		key := command + "\x00" + strings.Join(args, "\x00")
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		candidates = append(candidates, editorCommand{
			command: command,
			args:    append([]string{}, args...),
		})
	}

	switch kind {
	case EditorVSCode:
		add("code", "-r")
		add("code.cmd", "-r")
		add("code.exe", "-r")
		add("Code.exe", "-r")
		add("code-insiders", "-r")
		add("code-insiders.exe", "-r")
	case EditorCursor:
		add("cursor")
		add("cursor.exe")
		add("Cursor.exe")
	case EditorTrae:
		add("trae")
		add("trae.exe")
		add("Trae.exe")
	case EditorZed:
		add("zed")
		add("zed.exe")
		add("Zed.exe")
	}

	if runtime.GOOS == "darwin" {
		appName := map[EditorKind]string{
			EditorVSCode: "Visual Studio Code",
			EditorCursor: "Cursor",
			EditorTrae:   "Trae",
			EditorZed:    "Zed",
		}[kind]
		if appName != "" {
			add("open", "-a", appName)
		}
	}

	return candidates
}
