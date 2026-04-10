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
- Control CodeKanban `web session` runs over HTTP and WebSocket
- Read `web session` snapshots, history, runtime config, command groups, and archived sessions
- Send `web session` messages, approvals, user-input answers, model/workflow/permission updates, and move operations
- Receive normalized `web session` events through `on/off`, `waitFor`, or `for await`
- Poll a `web session` for a derived actionable state instead of reconstructing history logic yourself
- Execute the latest plan or answer the active structured prompt with one SDK call

## CLI

```bash
node packages/node-sdk/bin/codekanban-sdk.js workflow start --base-url http://127.0.0.1:3000 --path D:\repo --profile plan --add-dir D:\shared --prompt "Inspect and plan the refactor"
node packages/node-sdk/bin/codekanban-sdk.js session list --base-url http://127.0.0.1:3000 --path D:\repo
node packages/node-sdk/bin/codekanban-sdk.js session conversation --base-url http://127.0.0.1:3000 --id <db-id>
node packages/node-sdk/bin/codekanban-sdk.js terminal continue --base-url http://127.0.0.1:3000 --session-id <terminal-session-id> --prompt "Continue from the previous plan"
```

### Web Session CLI

```bash
node packages/node-sdk/bin/codekanban-sdk.js web-session create --base-url http://127.0.0.1:3000 --path D:\repo --agent codex --workflow-mode plan --permission-level elevated --title "Planning session"
node packages/node-sdk/bin/codekanban-sdk.js web-session snapshot --base-url http://127.0.0.1:3000 --project-id <project-id> --session-id <session-id> --limit 80
node packages/node-sdk/bin/codekanban-sdk.js web-session send --base-url http://127.0.0.1:3000 --session-id <session-id> --text "Continue from the last plan"
node packages/node-sdk/bin/codekanban-sdk.js web-session approve --base-url http://127.0.0.1:3000 --session-id <session-id>
node packages/node-sdk/bin/codekanban-sdk.js web-session user-input --base-url http://127.0.0.1:3000 --session-id <session-id> --item-id <item-id> --answers-json '{"scope":["full repo"]}'
node packages/node-sdk/bin/codekanban-sdk.js web-session watch --base-url http://127.0.0.1:3000 --session-id <session-id> --max-events 20
```

`web-session watch` writes NDJSON. Add `--raw` to print raw short-key protocol frames.

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

### Web Session HTTP

```js
const client = new CodeKanbanClient({ baseURL: 'http://127.0.0.1:3000' });

const session = await client.createWebSession({
  path: 'D:/repo',
  agent: 'codex',
  workflowMode: 'plan',
  permissionLevel: 'elevated',
  title: 'Planning session',
});

const snapshot = await client.getWebSessionSnapshot({
  projectId: session.projectId,
  sessionId: session.id,
});
```

### Polling-Friendly Web Session State

```js
const state = await client.getWebSessionState({
  projectId: session.projectId,
  sessionId: session.id,
});

if (state.phase === 'running') {
  return;
}

if (state.nextAction?.type === 'answer_user_input') {
  await client.answerPendingUserInput({
    projectId: session.projectId,
    sessionId: session.id,
    answers: {
      scope: ['full repo'],
    },
  });
}

if (state.nextAction?.type === 'execute_plan') {
  await client.executeLatestPlan({
    projectId: session.projectId,
    sessionId: session.id,
  });
}
```

`getWebSessionState()` derives:

- `phase`
- `canSend`
- `needsAction`
- `nextAction`
- `pendingApproval`
- `pendingUserInput`
- `latestPlan`
- `lastAssistantMessage`
- `snapshot`

### Waiting With Polling

```js
const doneState = await client.waitForWebSessionState({
  projectId: session.projectId,
  sessionId: session.id,
  until: 'done',
  intervalMs: 5000,
  timeoutMs: 120000,
});

console.log(doneState.lastAssistantMessage?.text);
```

### Web Session Command Channel

```js
const channel = client.openWebSessionCommandChannel();
await channel.waitForOpen();

await channel.sendMessage(session.id, {
  text: 'Continue from the latest review.',
});

await channel.updateWorkflowMode(session.id, {
  workflowMode: 'plan',
});

channel.close();
```

### Web Session Event Stream

```js
const events = client.openWebSessionEventStream({ sessionId: session.id });
await events.waitForOpen();

events.on('snapshot', event => {
  console.log('snapshot', event.snapshot.session.title);
});

const nextApproval = events.waitFor(event => event.type === 'historyItem' && event.item.detail?.type === 'approval_request');

for await (const event of events) {
  if (event.type === 'historyItem') {
    console.log(event.item.itemType, event.item.text);
  }
}

await nextApproval;
events.close();
```
