# CodeKanban Project Session Profiles

- `standard`
  - `codex -s workspace-write -a on-request`
- `plan`
  - Same default sandbox and approval flags as `standard`
  - Injects a planning preamble before the user prompt
  - Supports extra `--add-dir`, `--sandbox`, and `--approval-policy`
- `yolo`
  - `codex --dangerously-bypass-approvals-and-sandbox`
  - Do not combine with structured sandbox, approval, or add-dir overrides

For `claude`, only `standard` is supported in v1.
