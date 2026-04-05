---
name: codekanban-project-session
description: Launch AI work inside a specific CodeKanban project or filesystem path, and inspect CodeKanban terminal sessions and AI sessions. Use when the user wants to start `codex` or `claude` in a project path, run `plan` or `yolo` startup profiles, add extra `codex` permissions such as `--add-dir`, custom sandbox, or approval policy, continue an existing terminal session, or read CodeKanban AI conversation history by project, path, terminal session, or AI session ID. This skill is for CodeKanban business sessions, not raw local agent transcript files.
---

# CodeKanban Project Session

## Overview

Use this skill to operate on **CodeKanban business sessions** for a project, not raw `codex resume` or local transcript files. Prefer the bundled runner script so session lookups and workflow starts go through the repository SDK instead of re-implementing HTTP or WebSocket logic in the prompt.

## Quick Start

Run the bundled script:

```bash
node skills/codekanban-project-session/scripts/run-codekanban-session.mjs <command>
```

Map user requests to one of these commands:

- Start work in a project or path:

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

- `start`
  - Use when the user wants to launch `codex` or `claude` in a specific project or path.
  - Accept either `--project-id` or `--path`.
  - If only a path is provided, let the SDK resolve or auto-create the CodeKanban project.
- `session list`
  - Use when the user asks for recent terminal sessions, AI sessions, or project conversation history.
  - Return the JSON summary in natural language, but keep IDs available for follow-up actions.
- `session conversation`
  - Use only when the user identifies a specific AI session or asks to inspect the full conversation.
  - Prefer database `--id`; use `--session-id` when that is the only identifier available.
- `terminal continue`
  - Use when the user wants to send another prompt into an already-running CodeKanban terminal session.

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

## Output Handling

- The runner prints structured JSON.
- Summarize success for the user in plain language.
- Preserve IDs in your reply when the user may continue with the same terminal or AI session.
- If the command fails, surface the JSON error message and suggest the smallest corrective step.

## Resources

### `scripts/run-codekanban-session.mjs`

Run the repository SDK CLI from a stable relative path.

### `references/profile-mapping.md`

Use this file when you need a quick reminder of how `standard`, `plan`, and `yolo` map to SDK behavior.
