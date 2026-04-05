# `@codekanban/sdk`

Node SDK and CLI for CodeKanban project workflows.

## Features

- Resolve CodeKanban projects by `projectId` or local `path`
- Auto-create projects for new paths
- Select the main worktree automatically
- Launch terminal + agent workflows for `codex` or `claude`
- Support `standard`, `plan`, and `yolo` Codex profiles
- Read terminal session lists and AI session summaries
- Read AI conversations and continue existing terminal sessions

## CLI

```bash
node packages/node-sdk/bin/codekanban-sdk.js workflow start --base-url http://127.0.0.1:3000 --path D:\repo --profile plan --add-dir D:\shared --prompt "Inspect and plan the refactor"
node packages/node-sdk/bin/codekanban-sdk.js session list --base-url http://127.0.0.1:3000 --path D:\repo
node packages/node-sdk/bin/codekanban-sdk.js session conversation --base-url http://127.0.0.1:3000 --id <db-id>
node packages/node-sdk/bin/codekanban-sdk.js terminal continue --base-url http://127.0.0.1:3000 --session-id <terminal-session-id> --prompt "Continue from the previous plan"
```

## Library

```js
import { CodeKanbanClient } from '@codekanban/sdk';

const client = new CodeKanbanClient({ baseURL: 'http://127.0.0.1:3000' });

const result = await client.startWorkflow({
  path: 'D:/repo',
  prompt: 'Inspect the repository and produce a plan first.',
  profile: 'plan',
  permissions: {
    addDirs: ['D:/shared'],
  },
});
```
