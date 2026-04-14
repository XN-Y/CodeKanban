# `@codekanban/cli`

Installable CLI for operating CodeKanban workflows, sessions, and web sessions without keeping this repository on the target machine.

`codekanban-cli` is the only public command surface. It builds on `@codekanban/sdk`, bundles that dependency for offline distribution, and ships its Codex skill sources from `skills/` inside this package.

## Default behavior

- Default server URL: `http://127.0.0.1:3007`
- Base URL resolution order:
  1. `--base-url`
  2. `CODEKANBAN_BASE_URL` or `BASE_URL`
  3. Saved user session file
  4. `http://127.0.0.1:3007`
- Token/password resolution order:
  1. Explicit args such as `--token` or `--password`
  2. `--token-file` / `--token-stdin` / `--password-file` / `--password-stdin`
  3. `CODEKANBAN_TOKEN` / `TOKEN` / `CODEKANBAN_PASSWORD` / `PASSWORD`
  4. Saved user session file for the matching base URL
- Saved session file:
  - Windows: `%APPDATA%\codekanban-cli\session.json`
  - macOS/Linux: `$XDG_CONFIG_HOME/codekanban-cli/session.json` or `~/.config/codekanban-cli/session.json`

## First-time setup

If your CodeKanban server does not use password protection, you can start running commands immediately:

```bash
codekanban-cli project list
```

If password protection is enabled, save a token once and reuse it later:

```bash
printf '%s' 'your-password' | codekanban-cli auth save-token --password-stdin
```

You can also save a token directly:

```bash
codekanban-cli auth save-token --token-file ./token.txt --base-url http://127.0.0.1:3007
```

`auth save-token` validates the credential against the live server before writing:

- `base_url`
- `access_token`
- `username`
- `saved_at`

## Project-aware commands

List projects known to the target CodeKanban server:

```bash
codekanban-cli project list
```

Resolve a project by name:

```bash
codekanban-cli project resolve --project-name codekanban
```

If multiple candidates match, pick one directly without copying a project ID first:

```bash
codekanban-cli project resolve --project-name codekanban --project-index 2
```

Run a command directly against a project name:

```bash
codekanban-cli web-session create --project-name codekanban --agent codex --title "Planning session"
codekanban-cli session list --project-name codekanban
```

If the service is local and you omit `--project-id`, `--project-name`, and `--path`, project-scoped commands fall back to the current working directory.
If the service is remote, prefer `--project-id` or `--project-name`. In remote mode, `--path` means a server-side path on the machine running CodeKanban.

## Common commands

```bash
codekanban-cli --help
codekanban-cli auth status
codekanban-cli project list
codekanban-cli session list --project-name codekanban
codekanban-cli web-session state --project-name codekanban --session-id <session-id>
codekanban-cli web-session answer-pending --project-name codekanban --session-id <session-id>
codekanban-cli web-session execute-plan --project-name codekanban --session-id <session-id>
codekanban-cli web-session wait --project-name codekanban --session-id <session-id> --until done --settle-ms 2000
codekanban-cli file read --project-name codekanban --file notes/123.md
codekanban-cli web-session run --project-name codekanban --agent codex --text "Create a concise plan first, then implement it." --strict-cwd
```

If the service is not running on the default address, use either:

```bash
codekanban-cli --base-url http://192.168.1.50:3007 project list
```

or:

```bash
export CODEKANBAN_BASE_URL=http://192.168.1.50:3007
codekanban-cli project list
```

## Available Codex skill

- `codekanban-cli`: the shipped and discoverable Codex skill for operating CodeKanban through the installable CLI

## Codex skills in this repo

- Packaged skill source: `packages/codekanban-cli/skills/codekanban-cli`
- Skill directory index: `packages/codekanban-cli/skills/README.md`
- Release bundle builder: `packages/codekanban-cli/scripts/build-release-bundle.py`
- Offline bundle install docs: `packages/codekanban-cli/release/README-install.md`
