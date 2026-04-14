# CodeKanban Codex skills

This directory is the single source of truth for the Codex skills shipped with `@codekanban/cli`.

Available skills in this package:

- `codekanban-cli`: the primary discoverable skill for operating CodeKanban through the installable `codekanban-cli` command

Key entrypoints:

- Skill source: `packages/codekanban-cli/skills/codekanban-cli/SKILL.md`
- Agent metadata: `packages/codekanban-cli/skills/codekanban-cli/agents/openai.yaml`

Before using the skill:

1. Install `codekanban-cli`
2. Copy the `skills/` directory contents into the target user's Codex skills directory
3. Restart Codex so the new skill is discovered
