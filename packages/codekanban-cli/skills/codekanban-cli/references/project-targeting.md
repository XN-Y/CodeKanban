# Project Targeting Guide

Use this guide when the user refers to a CodeKanban project but does not give an exact `projectId`.

## Preferred targeting order

1. `--project-id` if already known
2. `--project-name` if the user names the project
3. local current working directory only when the server is local
4. explicit `--path` only when the filesystem path is known to be valid on the server

## Remote server rule

If `--base-url` points to a remote host, do not assume the local Codex machine path is valid on that host.
In remote mode:

- prefer `project list`
- then use `project resolve --project-name <name>`
- then run the real command with `--project-name` or `--project-id`

## Ambiguity handling

If multiple projects match the same name or path basename:

1. run `codekanban-cli project resolve --project-name <name>` first
2. if it reports multiple candidates, note the numbered matches
3. retry with `--project-index <n>` when the user already indicated which candidate to use
4. fall back to `project list` only when you need the wider server inventory

## Examples

### Example 1: user names a project

User says:

```text
Create a web session for project codekanban and pull git updates.
```

Recommended flow:

```bash
codekanban-cli project resolve --project-name codekanban
codekanban-cli web-session run --project-name codekanban --agent codex --text "Pull the latest git updates and summarize any conflicts." --strict-cwd
```

### Example 2: remote server with server-side path

```bash
codekanban-cli --base-url http://10.0.0.5:6112 web-session create --path /srv/projects/codekanban --agent codex
```

Use this only when you are sure `/srv/projects/codekanban` exists on the machine running CodeKanban.
