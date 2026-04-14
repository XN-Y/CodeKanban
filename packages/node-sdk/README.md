# `@codekanban/sdk`

Node SDK for CodeKanban workflows, terminal sessions, and web sessions.

Use `@codekanban/sdk` when you want to integrate with CodeKanban from JavaScript. For command-line usage and Codex skill packaging, use `@codekanban/cli`.

## Features

- Resolve CodeKanban projects by `projectId` or local `path`
- Auto-create projects for new paths
- Select the main worktree automatically
- Launch coding workflows for `codex` or `claude`
- Support `standard`, `plan`, and `yolo` Codex profiles
- Read terminal session lists and AI session summaries
- Read AI conversations and continue existing terminal sessions
- Control CodeKanban `web-session` runs over HTTP and WebSocket
- Read `web-session` snapshots, history, runtime config, command groups, and archived sessions
- Send `web-session` messages, approvals, user-input answers, model/workflow/permission updates, and move operations
- Receive normalized `web-session` events through `on/off`, `waitFor`, or `for await`
- Poll a `web-session` for derived actionable state
- Wait for a `web-session` to reach a pause/actionable state
- Run a short web-session loop with automatic user-input answers and latest-plan execution

## Install

```bash
npm install @codekanban/sdk
```

## Library

```js
import { CodeKanbanClient } from '@codekanban/sdk';

const client = new CodeKanbanClient({ baseURL: 'http://127.0.0.1:3007' });

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
const client = new CodeKanbanClient({ baseURL: 'http://127.0.0.1:3007' });

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
  settleMs: 2000,
});

console.log(doneState.lastAssistantMessage?.text);
```

### Waiting For Pause States

```js
const pause = await client.waitForWebSessionPause({
  projectId: session.projectId,
  sessionId: session.id,
  intervalMs: 1500,
  timeoutMs: 120000,
  settleMs: 2000,
});

console.log(pause.reason);
```

`waitForWebSessionPause()` returns when the session stops making forward progress and needs outside judgment, for example:

- `done`
- `error`
- `approval`
- `user_input`
- `execute_plan`

### Minimal Web Session Loop

```js
const result = await client.runWebSessionUntilDone({
  projectId: session.projectId,
  sessionId: session.id,
  intervalMs: 1500,
  timeoutMs: 120000,
  settleMs: 2000,
});

console.log(result.stopReason, result.finalState?.phase);
```

`runWebSessionUntilDone()` automatically:

- answers ordinary structured user-input prompts with the default `prefer-second-or-text` strategy
- executes the latest plan when the session reaches an execute-plan pause
- returns control on `needs_approval`, `needs_user_input`, `needs_execute_plan`, `done`, `error`, `until`, or `timeout`

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
