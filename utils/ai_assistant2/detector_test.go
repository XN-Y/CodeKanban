package ai_assistant2

import (
	"testing"

	"code-kanban/utils/ai_assistant2/types"
)

func TestDetectFromCommand(t *testing.T) {
	t.Parallel()

	detector := NewAssistantDetector()

	tests := []struct {
		name    string
		command string
		want    types.AssistantType
	}{
		{
			name:    "codex package path",
			command: "node /workspace/node_modules/@openai/codex/bin/codex.js",
			want:    types.AssistantTypeCodex,
		},
		{
			name:    "codex direct executable path",
			command: "/usr/local/bin/codex --model gpt-5",
			want:    types.AssistantTypeCodex,
		},
		{
			name:    "codex node wrapped executable",
			command: "node /usr/local/bin/codex 1",
			want:    types.AssistantTypeCodex,
		},
		{
			name:    "codex shell wrapped executable",
			command: `bash -lc "codex --model gpt-5"`,
			want:    types.AssistantTypeCodex,
		},
		{
			name:    "claude direct executable path",
			command: "/usr/local/bin/claude --print",
			want:    types.AssistantTypeClaudeCode,
		},
		{
			name:    "claude code direct executable path",
			command: "/usr/local/bin/claude-code --print",
			want:    types.AssistantTypeClaudeCode,
		},
		{
			name:    "claude node wrapped executable",
			command: "node /usr/local/bin/claude 1",
			want:    types.AssistantTypeClaudeCode,
		},
		{
			name:    "claude shell wrapped executable",
			command: `bash -lc "claude --dangerously-skip-permissions"`,
			want:    types.AssistantTypeClaudeCode,
		},
		{
			name:    "regular shell command is not ai assistant",
			command: `bash -lc "echo codex"`,
			want:    types.AssistantTypeUnknown,
		},
		{
			name:    "regular node command is not ai assistant",
			command: "node /usr/local/bin/serve 3000",
			want:    types.AssistantTypeUnknown,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			info := detector.DetectFromCommand(tt.command)
			if tt.want == types.AssistantTypeUnknown {
				if info != nil {
					t.Fatalf("expected nil detection, got %+v", info)
				}
				return
			}

			if info == nil {
				t.Fatalf("expected detection for %q", tt.command)
			}
			if info.Type != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, info.Type)
			}
		})
	}
}
