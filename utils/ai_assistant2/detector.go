package ai_assistant2

import (
	"path"
	"strings"

	"code-kanban/utils/ai_assistant2/types"
)

// DetectionRule defines how to detect a specific AI assistant from command
type DetectionRule struct {
	Type            types.AssistantType
	Patterns        []string // Command line patterns to match (case-insensitive)
	ExecutableNames []string // Executable names to match exactly (case-insensitive)
	Description     string
}

var defaultRules = []DetectionRule{
	{
		Type: types.AssistantTypeClaudeCode,
		Patterns: []string{
			"@anthropic-ai/claude-code",
			"claude-code/cli.js",
			"claude-code/bin/",
		},
		ExecutableNames: []string{
			"claude",
			"claude-code",
		},
		Description: "Detects Anthropic Claude Code CLI",
	},
	{
		Type: types.AssistantTypeCodex,
		Patterns: []string{
			"@openai/codex",
			"codex/bin/codex.js",
			"codex.js",
		},
		ExecutableNames: []string{
			"codex",
		},
		Description: "Detects OpenAI Codex CLI",
	},
	{
		Type: types.AssistantTypeQwenCode,
		Patterns: []string{
			"@qwen-code/qwen-code",
			"qwen-code/cli.js",
			"qwen-code/bin/",
		},
		Description: "Detects Qwen Code CLI",
	},
	{
		Type: types.AssistantTypeGemini,
		Patterns: []string{
			"@google/gemini-cli",
			"gemini-cli/dist/index.js",
			"gemini-cli/bin/",
		},
		Description: "Detects Google Gemini CLI",
	},
}

// Match checks if the command matches this rule
func (r *DetectionRule) Match(command string) bool {
	if command == "" {
		return false
	}

	normalizedCmd := normalizeCommand(command)

	for _, pattern := range r.Patterns {
		normalizedPattern := strings.ToLower(pattern)
		if strings.Contains(normalizedCmd, normalizedPattern) {
			return true
		}
	}

	if len(r.ExecutableNames) == 0 {
		return false
	}

	candidates := candidateExecutables(normalizedCmd)
	for _, candidate := range candidates {
		if r.matchesExecutable(candidate) {
			return true
		}
	}

	return false
}

func normalizeCommand(command string) string {
	normalized := strings.ToLower(strings.TrimSpace(command))
	return strings.ReplaceAll(normalized, "\\", "/")
}

func splitCommandTokens(command string) []string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		token := strings.Trim(field, `"'`)
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}

	return tokens
}

func candidateExecutables(command string) []string {
	tokens := splitCommandTokens(command)
	if len(tokens) == 0 {
		return nil
	}

	candidates := make([]string, 0, 4)
	appendCandidate := func(token string) {
		token = strings.TrimSpace(token)
		if token == "" {
			return
		}
		candidates = append(candidates, token)
	}

	appendCandidate(tokens[0])

	if isNodeRuntime(tokens[0]) && len(tokens) > 1 {
		appendCandidate(tokens[1])
	}

	if isShellExecutable(tokens[0]) {
		scriptTokens := extractShellCommandTokens(tokens)
		if len(scriptTokens) > 0 {
			appendCandidate(scriptTokens[0])
			if isNodeRuntime(scriptTokens[0]) && len(scriptTokens) > 1 {
				appendCandidate(scriptTokens[1])
			}
		}
	}

	return candidates
}

func extractShellCommandTokens(tokens []string) []string {
	if len(tokens) < 2 {
		return nil
	}

	for idx := 1; idx < len(tokens); idx++ {
		token := tokens[idx]
		if token == "-c" || token == "-lc" || token == "-cl" {
			if idx+1 >= len(tokens) {
				return nil
			}
			return splitCommandTokens(strings.Join(tokens[idx+1:], " "))
		}
	}

	return nil
}

func isNodeRuntime(token string) bool {
	base := path.Base(token)
	switch base {
	case "node", "node.exe":
		return true
	default:
		return false
	}
}

func isShellExecutable(token string) bool {
	base := path.Base(token)
	switch base {
	case "bash", "bash.exe", "sh", "sh.exe", "zsh", "zsh.exe", "fish", "fish.exe":
		return true
	default:
		return false
	}
}

func (r *DetectionRule) matchesExecutable(token string) bool {
	base := path.Base(token)
	for _, executable := range r.ExecutableNames {
		executable = strings.ToLower(strings.TrimSpace(executable))
		if executable == "" {
			continue
		}
		if token == executable || base == executable {
			return true
		}
	}
	return false
}

// AssistantDetector detects AI assistant type from command
type AssistantDetector struct {
	rules []DetectionRule
}

// NewAssistantDetector creates a new AI assistant detector
func NewAssistantDetector() *AssistantDetector {
	return &AssistantDetector{
		rules: defaultRules,
	}
}

// DetectFromCommand analyzes a command string and returns the AI assistant type
func (d *AssistantDetector) DetectFromCommand(command string) *types.AssistantInfo {
	if command == "" {
		return nil
	}

	for _, rule := range d.rules {
		if rule.Match(command) {
			return &types.AssistantInfo{
				Type:        rule.Type,
				Name:        string(rule.Type),
				DisplayName: rule.Type.DisplayName(),
				Command:     command,
				Detected:    true,
			}
		}
	}

	return nil
}

// IsAIAssistant checks if the command is running an AI assistant
func (d *AssistantDetector) IsAIAssistant(command string) bool {
	return d.DetectFromCommand(command) != nil
}

// GetType returns the AI assistant type from command
func (d *AssistantDetector) GetType(command string) types.AssistantType {
	info := d.DetectFromCommand(command)
	if info != nil {
		return info.Type
	}
	return types.AssistantTypeUnknown
}

// Default detector instance
var defaultDetector = NewAssistantDetector()

// DetectFromCommand uses the default detector to analyze a command
func DetectFromCommand(command string) *types.AssistantInfo {
	return defaultDetector.DetectFromCommand(command)
}

// IsAIAssistant uses the default detector to check if command is an AI assistant
func IsAIAssistant(command string) bool {
	return defaultDetector.IsAIAssistant(command)
}

// GetType uses the default detector to get the assistant type
func GetType(command string) types.AssistantType {
	return defaultDetector.GetType(command)
}
