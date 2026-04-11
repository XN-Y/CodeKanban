---
name: codekanban-project-session
description: Operate CodeKanban `web session` and `terminal session` flows, with `web session` as the default interactive path. Use when the user wants to create, inspect, control, watch, or continue a CodeKanban programming-assistant workflow in a project or filesystem path. Prefer `web session` for structured interaction, planning, approvals, user input, and follow-up execution. Use terminal/workflow startup only when the user explicitly wants a PTY-style terminal flow, raw terminal continuation, or startup profiles such as `plan` / `yolo`. This skill is for CodeKanban session control, not raw local agent transcript files.
---

# CodeKanban Web And Terminal Sessions

## Overview

Use this skill to operate on **CodeKanban web sessions and terminal sessions** for a project, not raw `codex resume` or local transcript files. Prefer the bundled runner script so session lookups, workflow starts, and web-session control all go through the repository SDK instead of re-implementing HTTP or WebSocket logic in the prompt.

Default rule:

- Prefer `web-session` for interactive programming-assistant work.
- Treat `workflow start` and `terminal continue` as secondary paths for explicit PTY / terminal requests.

## Quick Start

Run the bundled script:

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs <command>
```

Map user requests to one of these commands, preferring `web-session` unless the user explicitly asks for terminal-style behavior:

- Create a web session:

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs web-session create --base-url <BASE_URL> --path <PROJECT_PATH> --agent codex --workflow-mode plan --permission-level elevated --title "Planning session"
```

- Send input to a web session:

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs web-session send --base-url <BASE_URL> --session-id <WEB_SESSION_ID> --text "Continue from the previous plan"
```

- Watch web session events:

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs web-session watch --base-url <BASE_URL> --session-id <WEB_SESSION_ID> --max-events 20
```

- Start a coding workflow in a project or path:

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs workflow start --base-url <BASE_URL> --path <PROJECT_PATH> --prompt "<PROMPT>"
```

- List CodeKanban sessions for a project or path:

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs session list --base-url <BASE_URL> --path <PROJECT_PATH>
```

- Read a full AI conversation:

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs session conversation --base-url <BASE_URL> --id <AI_SESSION_DB_ID>
```

- Continue an existing terminal session:

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs terminal continue --base-url <BASE_URL> --session-id <TERMINAL_SESSION_ID> --prompt "<PROMPT>"
```

## Workflow Mapping

Translate user intent into these operations:

- `session list`
  - Use when the user asks for recent terminal sessions, AI sessions, or project conversation history.
  - Return the JSON summary in natural language, but keep IDs available for follow-up actions.
- `session conversation`
  - Use only when the user identifies a specific AI session or asks to inspect the full conversation.
  - Prefer database `--id`; use `--session-id` when that is the only identifier available.
- `web-session create`
  - Default choice for new interactive coding-assistant work in CodeKanban.
  - Prefer this for structured control, snapshots, approvals, user-input handling, event streaming, plan execution, and low-cost follow-up polling.
- `web-session snapshot` / `web-session history`
  - Use when the user asks for the current state, recent messages, or older paginated history of a web session.
- `web-session send`
  - Use when the user wants to send another message to a running or idle web session.
- `web-session approve` / `web-session reject`
  - Use when the user wants to answer a pending approval prompt.
- `web-session user-input`
  - Use when the user wants to answer structured `request_user_input` prompts.
- `web-session set-model` / `set-reasoning` / `set-workflow` / `set-permission` / `set-agent`
  - Use when the user explicitly asks to change web-session runtime configuration before the next message.
- `web-session sync`
  - Use when the user wants to force-refresh a session from Codex thread state.
- `web-session archive` / `unarchive` / `delete`
  - Use for lifecycle operations on saved web sessions.
- `web-session watch`
  - Use when the user wants ongoing events, state changes, or approval/user-input notifications from a web session.
- `start`
  - Use only when the user explicitly wants to launch `codex` or `claude` through a PTY-style startup flow.
  - Accept either `--project-id` or `--path`.
  - If only a path is provided, let the SDK resolve or auto-create the CodeKanban project.
  - This is a supplement to `web-session`, not the default interactive path.
- `terminal continue`
  - Use only when the user explicitly wants to continue an already-running terminal session.
  - This is a supplement to `web-session`, not the default interactive path.

## Profile and Permission Rules

For `codex`, support three startup profiles:

- `--profile standard`
  - Normal write-capable startup.
- `--profile plan`
  - Start in planning mode.
  - The SDK injects a planning preamble before the user's prompt.
  - This profile must support extra permission parameters.
- `--profile yolo`
  - Full bypass mode.
  - Use only when the user explicitly asks for an unsafe unrestricted launch.

When the user asks for extra permissions during `plan` startup, map them to:

- `--add-dir <PATH>` for extra writable directories
- `--sandbox <read-only|workspace-write|danger-full-access>`
- `--approval-policy <untrusted|on-request|never>`

Examples:

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs workflow start --base-url <BASE_URL> --path <PROJECT_PATH> --profile plan --add-dir D:\shared --approval-policy never --prompt "Inspect and write a plan first"
```

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs workflow start --base-url <BASE_URL> --path <PROJECT_PATH> --profile yolo --prompt "Implement directly"
```

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs web-session set-workflow --base-url <BASE_URL> --session-id <WEB_SESSION_ID> --workflow-mode plan
```

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs web-session user-input --base-url <BASE_URL> --session-id <WEB_SESSION_ID> --item-id <ITEM_ID> --answers-json '{"scope":["full repo"]}'
```

## Output Handling

- The runner prints structured JSON.
- `web-session watch` prints NDJSON, one event per line.
- Summarize success for the user in plain language.
- Preserve IDs in your reply when the user may continue with the same terminal or AI session.
- Preserve web-session IDs, approval item IDs, and user-input item IDs when follow-up actions are likely.
- If the command fails, surface the JSON error message and suggest the smallest corrective step.

## Selection Rule

- If the user wants interactive programming-assistant work and did not explicitly ask for terminal / PTY behavior, choose `web-session`.
- If the user needs approvals, structured user input, plan execution, polling, or event watching, choose `web-session`.
- Use `workflow start` only when a raw terminal-style startup flow is explicitly the point.
- Use `terminal continue` only for explicit continuation of an existing terminal session.

## Resources

### `scripts/run-codekanban-session.mjs`

Run the repository SDK CLI from a stable relative path.

### `references/profile-mapping.md`

Use this file when you need a quick reminder of how `standard`, `plan`, and `yolo` map to SDK behavior.
