# CodeKanban CLI offline bundle

This bundle installs the `codekanban-cli` Codex skill system without requiring the original repository.

Included artifacts:

- `npm/codekanban-cli-__CLI_VERSION__.tgz`
- `skills/`
- `install-windows.cmd`
- `install-cli-unix.sh`
- `install-skills-unix.sh`
- `install-unix.sh`

Included Codex skill:

- `codekanban-cli`

## Default service URL

If you do not provide a base URL, `codekanban-cli` defaults to:

```text
http://127.0.0.1:3007
```

## First-time initialization

If auth is enabled on the server, save a token once:

```bash
printf '%s' '<PASSWORD>' | codekanban-cli auth save-token --password-stdin
```

If the server is not running on the default address, set it explicitly:

```bash
codekanban-cli --base-url http://192.168.1.50:3007 session list --path /repo
```

or:

```bash
export CODEKANBAN_BASE_URL=http://192.168.1.50:3007
```

## Why Unix install is split into two steps

- Global npm installation can require `sudo` or another privileged prefix.
- Codex skills must be installed into the real user's Codex directory.
- Installing skills with `sudo` can place them under root's `~/.codex/skills`, which is the wrong account for normal Codex usage.

After installing the CLI and skills, restart Codex.
