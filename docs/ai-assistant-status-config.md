# AI Assistant Status Monitoring Configuration

## Configuration Options

In `config.yaml`, under the `terminal` section, you can configure which AI assistants should have status tracking enabled:

```yaml
terminal:
  shell:
    windows: "pwsh.exe -NoLogo"
    linux: "/bin/bash"
    darwin: "/bin/zsh"
  idleTimeout: "0s"
  maxSessionsPerProject: 12
  allowedRoots: []
  encoding: "utf-8"
  scrollbackBytes: 262144

  # AI Assistant Status Tracking Configuration
  aiAssistantStatus:
    claudeCode: true   # Claude Code - 状态监测准确，推荐启用
    codex: false       # OpenAI Codex - 存在问题（光标操纵导致误判），默认禁用
    qwenCode: true     # Qwen Code - 状态监测准确，推荐启用
    gemini: false      # Google Gemini - 未充分测试，默认禁用
    cursor: false      # Cursor - 未充分测试，默认禁用
    copilot: false     # GitHub Copilot - 未充分测试，默认禁用
```

## AI Assistant Types

| Type | Status Tracking | Default | Notes |
|------|----------------|---------|-------|
| Claude Code | ✅ Accurate | Enabled | 状态监测准确，默认启用 |
| **OpenAI Codex** | ⚠️ **Issues** | **Disabled** | **存在问题：光标操纵导致的误判** |
| Qwen Code | ✅ Accurate | Enabled | 状态监测准确，默认启用 |
| Gemini | ❓ Untested | Disabled | 未充分测试，默认禁用 |
| Cursor | ❓ Untested | Disabled | 未充分测试，默认禁用 |
| Copilot | ❓ Untested | Disabled | 未充分测试，默认禁用 |

## Why Codex is Disabled by Default

OpenAI Codex uses ANSI cursor positioning and partial line updates (spotlight animation effect) which causes false positive detection of task completion.

**Symptoms:**
- Status changes from "Thinking" to "Waiting Input" prematurely
- Happens when Codex's spotlight animation plays over text
- Each animation frame may not contain the "esc to interrupt" status text

**Solution Options:**
1. Keep status tracking disabled for Codex (current default)
2. Use Codex CLI's `--json` mode for structured event output (future enhancement)
3. Use ACP (Agent Client Protocol) integration (advanced)

## How to Enable/Disable

To enable Codex status tracking (if you want to try despite the issues):

```yaml
terminal:
  aiAssistantStatus:
    codex: true
```

To disable Claude Code status tracking:

```yaml
terminal:
  aiAssistantStatus:
    claudeCode: false
```

## Technical Implementation

The status tracking configuration is checked when activating the StatusTracker for each terminal session. If disabled, the tracker will not monitor state changes, but type detection will still work.

**Location in Code:**
- Configuration: `utils/app_config.go` - `AIAssistantStatusConfig`
- Status Tracker: `utils/ai_assistant2/status_tracker.go` - `Activate()` method
- Session Creation: `service/terminal/session.go` - `NewSession()`

## Related Documentation

- [Terminal Status Detection](./bugfix-terminal-projectid-corruption.md)
- [AI Assistant Type Detection](../utils/ai_assistant2/)
