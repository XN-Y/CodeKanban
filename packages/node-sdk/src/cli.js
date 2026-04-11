import { CodeKanbanClient } from './client.js';
import { buildAgentLaunchSpec } from './command-builder.js';
import { createJsonOutput } from './utils.js';

function readFlagValue(argv, index, flag) {
  const value = argv[index + 1];
  if (value == null || value.startsWith('--')) {
    throw new Error(`${flag} requires a value`);
  }
  return value;
}

function parseIntegerFlag(value, fieldName) {
  if (value == null || value === '') {
    return undefined;
  }
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    throw new Error(`${fieldName} must be a number`);
  }
  return Math.trunc(parsed);
}

function parseJsonFlag(value, fieldName) {
  if (!value) {
    throw new Error(`${fieldName} is required`);
  }
  try {
    return JSON.parse(value);
  } catch (error) {
    throw new Error(`${fieldName} must be valid JSON`);
  }
}

function parseCliArgs(argv) {
  const positionals = [];
  const flags = {
    addDirs: [],
    attachmentIds: [],
    extraArgs: [],
    includeTerminal: true,
    includeAI: true,
    refresh: false,
    clearExisting: false,
    raw: false,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token.startsWith('--')) {
      positionals.push(token);
      continue;
    }

    switch (token) {
      case '--base-url':
        flags.baseURL = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--project-id':
        flags.projectId = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--path':
        flags.path = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--prompt':
        flags.prompt = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--text':
        flags.text = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--agent':
        flags.agent = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--model':
        flags.model = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--profile':
        flags.profile = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--sandbox':
        flags.sandbox = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--approval-policy':
        flags.approvalPolicy = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--add-dir':
        flags.addDirs.push(readFlagValue(argv, index, token));
        index += 1;
        break;
      case '--attachment-id':
        flags.attachmentIds.push(readFlagValue(argv, index, token));
        index += 1;
        break;
      case '--extra-arg':
        flags.extraArgs.push(readFlagValue(argv, index, token));
        index += 1;
        break;
      case '--title':
        flags.title = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--working-dir':
        flags.workingDir = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--worktree-id':
        flags.worktreeId = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--session-id':
        flags.sessionId = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--id':
        flags.id = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--tool-use-id':
        flags.toolUseId = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--reasoning-effort':
        flags.reasoningEffort = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--workflow-mode':
        flags.workflowMode = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--permission-level':
        flags.permissionLevel = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--permission-mode':
        flags.permissionMode = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--limit':
        flags.limit = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--before-cursor':
        flags.beforeCursor = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--mode':
        flags.mode = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--group-id':
        flags.groupId = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--file':
        flags.file = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--item-id':
        flags.itemId = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--answers-json':
        flags.answersJson = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--prev-session-id':
        flags.prevSessionId = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--next-session-id':
        flags.nextSessionId = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--idle-timeout-ms':
        flags.idleTimeoutMs = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--max-events':
        flags.maxEvents = readFlagValue(argv, index, token);
        index += 1;
        break;
      case '--refresh':
        flags.refresh = true;
        break;
      case '--clear-existing':
        flags.clearExisting = true;
        break;
      case '--raw':
        flags.raw = true;
        break;
      case '--no-terminal':
        flags.includeTerminal = false;
        break;
      case '--no-ai':
        flags.includeAI = false;
        break;
      default:
        throw new Error(`unknown flag: ${token}`);
    }
  }

  return {
    positionals,
    flags,
  };
}

function buildPermissions(flags) {
  const permissions = {};
  if (flags.sandbox) {
    permissions.sandbox = flags.sandbox;
  }
  if (flags.approvalPolicy) {
    permissions.approvalPolicy = flags.approvalPolicy;
  }
  if (flags.addDirs.length > 0) {
    permissions.addDirs = flags.addDirs;
  }
  return Object.keys(permissions).length > 0 ? permissions : undefined;
}

function sanitizeConnection(value) {
  if (!value || typeof value !== 'object') {
    return value;
  }
  const next = { ...value };
  delete next.connection;
  delete next.commandChannel;
  delete next.eventStream;
  delete next.socket;
  return next;
}

function printJson(stream, payload) {
  stream.write(createJsonOutput(payload));
}

function writeJsonLine(stream, payload) {
  stream.write(`${JSON.stringify(payload)}\n`);
}

async function withWebSessionCommandChannel(client, handler) {
  const channel = client.openWebSessionCommandChannel();
  try {
    await channel.waitForOpen();
    return await handler(channel);
  } finally {
    channel.close();
  }
}

async function watchWebSession(client, flags, stdout) {
  const eventStream = client.openWebSessionEventStream({
    sessionId: flags.sessionId,
  });
  const maxEvents = parseIntegerFlag(flags.maxEvents, 'maxEvents');
  const idleTimeoutMs = parseIntegerFlag(flags.idleTimeoutMs, 'idleTimeoutMs');
  let eventCount = 0;
  let idleTimer = null;
  let closedByPolicy = false;

  const armIdleTimer = () => {
    if (idleTimer) {
      clearTimeout(idleTimer);
      idleTimer = null;
    }
    if (!Number.isFinite(idleTimeoutMs) || idleTimeoutMs == null || idleTimeoutMs <= 0) {
      return;
    }
    idleTimer = setTimeout(() => {
      closedByPolicy = true;
      eventStream.close();
    }, idleTimeoutMs);
  };

  try {
    await eventStream.waitForOpen();
    armIdleTimer();

    for await (const event of eventStream) {
      if (event.type === 'open' || event.type === 'close') {
        continue;
      }

      if (event.type === 'error' && event.errorType !== 'frame') {
        throw event.error instanceof Error ? event.error : new Error(event.message || 'event stream failed');
      }

      const payload = flags.raw && event.raw ? event.raw : event;
      writeJsonLine(stdout, payload);
      eventCount += 1;
      armIdleTimer();

      if (maxEvents != null && maxEvents > 0 && eventCount >= maxEvents) {
        closedByPolicy = true;
        eventStream.close();
      }
    }

    return 0;
  } finally {
    if (idleTimer) {
      clearTimeout(idleTimer);
    }
    if (!closedByPolicy) {
      eventStream.close();
    }
  }
}

export async function runCli(argv, options = {}) {
  const stdout = options.stdout || process.stdout;
  const stderr = options.stderr || process.stderr;

  try {
    const { positionals, flags } = parseCliArgs(argv);
    const [scope, action] = positionals;
    if (!scope || !action) {
      throw new Error('usage: <workflow|session|terminal|web-session> <action> --base-url <url> [...]');
    }

    if (scope === 'workflow' && action === 'command') {
      const result = buildAgentLaunchSpec({
        agent: flags.agent,
        profile: flags.profile,
        permissions: buildPermissions(flags),
        extraArgs: flags.extraArgs,
        prompt: flags.prompt || 'Inspect the project and respond.',
      });
      printJson(stdout, sanitizeConnection(result));
      return 0;
    }

    if (!flags.baseURL) {
      throw new Error('--base-url is required');
    }

    const client = new CodeKanbanClient({ baseURL: flags.baseURL });
    let result;

    if (scope === 'workflow' && action === 'start') {
      result = await client.startWorkflow({
        projectId: flags.projectId,
        path: flags.path,
        worktreeId: flags.worktreeId,
        prompt: flags.prompt,
        agent: flags.agent,
        profile: flags.profile,
        permissions: buildPermissions(flags),
        extraArgs: flags.extraArgs,
        title: flags.title,
        workingDir: flags.workingDir,
      });
    } else if (scope === 'session' && action === 'list') {
      result = await client.listSessions({
        projectId: flags.projectId,
        path: flags.path,
        includeTerminal: flags.includeTerminal,
        includeAI: flags.includeAI,
      });
    } else if (scope === 'session' && action === 'conversation') {
      result = await client.getAISessionConversation({
        id: flags.id,
        sessionId: flags.sessionId,
        refresh: flags.refresh,
      });
    } else if (scope === 'session' && action === 'tool-result') {
      result = await client.getAISessionToolResult({
        id: flags.id,
        sessionId: flags.sessionId,
        toolUseId: flags.toolUseId,
      });
    } else if (scope === 'terminal' && action === 'continue') {
      result = await client.continueTerminalSession({
        projectId: flags.projectId,
        path: flags.path,
        sessionId: flags.sessionId,
        prompt: flags.prompt,
      });
    } else if (scope === 'web-session' && action === 'list') {
      result = await client.listWebSessions({
        projectId: flags.projectId,
        path: flags.path,
      });
    } else if (scope === 'web-session' && action === 'create') {
      result = await client.createWebSession({
        projectId: flags.projectId,
        path: flags.path,
        worktreeId: flags.worktreeId,
        agent: flags.agent,
        model: flags.model,
        reasoningEffort: flags.reasoningEffort,
        workflowMode: flags.workflowMode,
        permissionLevel: flags.permissionLevel,
        permissionMode: flags.permissionMode,
        title: flags.title,
      });
    } else if (scope === 'web-session' && action === 'connect') {
      result = await withWebSessionCommandChannel(client, channel => channel.connect(flags.sessionId));
    } else if (scope === 'web-session' && action === 'snapshot') {
      result = await client.getWebSessionSnapshot({
        projectId: flags.projectId,
        path: flags.path,
        sessionId: flags.sessionId,
        limit: parseIntegerFlag(flags.limit, 'limit'),
      });
    } else if (scope === 'web-session' && action === 'history') {
      result = await client.getWebSessionHistory({
        projectId: flags.projectId,
        path: flags.path,
        sessionId: flags.sessionId,
        beforeCursor: flags.beforeCursor,
        limit: parseIntegerFlag(flags.limit, 'limit'),
      });
    } else if (scope === 'web-session' && action === 'sync') {
      result = await client.syncWebSession({
        projectId: flags.projectId,
        path: flags.path,
        sessionId: flags.sessionId,
        mode: flags.mode,
        clearExisting: flags.clearExisting,
      });
    } else if (scope === 'web-session' && action === 'archive') {
      result = await client.archiveWebSession({
        projectId: flags.projectId,
        path: flags.path,
        sessionId: flags.sessionId,
      });
    } else if (scope === 'web-session' && action === 'unarchive') {
      result = await client.unarchiveWebSession({
        projectId: flags.projectId,
        path: flags.path,
        sessionId: flags.sessionId,
      });
    } else if (scope === 'web-session' && action === 'rename') {
      result = await client.renameWebSession({
        projectId: flags.projectId,
        path: flags.path,
        sessionId: flags.sessionId,
        title: flags.title,
      });
    } else if (scope === 'web-session' && action === 'close') {
      result = await client.closeWebSession({
        projectId: flags.projectId,
        path: flags.path,
        sessionId: flags.sessionId,
      });
    } else if (scope === 'web-session' && action === 'delete') {
      result = await client.deleteWebSession({
        projectId: flags.projectId,
        path: flags.path,
        sessionId: flags.sessionId,
      });
    } else if (scope === 'web-session' && action === 'runtime-config') {
      result = await client.getWebSessionRuntimeConfig();
    } else if (scope === 'web-session' && action === 'command-group') {
      result = await client.getWebSessionCommandGroup({
        projectId: flags.projectId,
        path: flags.path,
        sessionId: flags.sessionId,
        groupId: flags.groupId,
      });
    } else if (scope === 'web-session' && action === 'attach') {
      result = await client.uploadWebSessionAttachment({
        projectId: flags.projectId,
        path: flags.path,
        filePath: flags.file,
      });
    } else if (scope === 'web-session' && action === 'send') {
      result = await withWebSessionCommandChannel(client, channel =>
        channel.sendMessage(flags.sessionId, {
          text: flags.text || flags.prompt,
          attachmentIds: flags.attachmentIds,
        }),
      );
    } else if (scope === 'web-session' && action === 'approve') {
      result = await withWebSessionCommandChannel(client, channel => channel.approve(flags.sessionId));
    } else if (scope === 'web-session' && action === 'reject') {
      result = await withWebSessionCommandChannel(client, channel => channel.reject(flags.sessionId));
    } else if (scope === 'web-session' && action === 'user-input') {
      result = await withWebSessionCommandChannel(client, channel =>
        channel.answerUserInput(flags.sessionId, {
          itemId: flags.itemId,
          answers: parseJsonFlag(flags.answersJson, 'answersJson'),
        }),
      );
    } else if (scope === 'web-session' && action === 'set-model') {
      result = await withWebSessionCommandChannel(client, channel =>
        channel.updateModel(flags.sessionId, { model: flags.model }),
      );
    } else if (scope === 'web-session' && action === 'set-reasoning') {
      result = await withWebSessionCommandChannel(client, channel =>
        channel.updateReasoningEffort(flags.sessionId, {
          reasoningEffort: flags.reasoningEffort,
        }),
      );
    } else if (scope === 'web-session' && action === 'set-workflow') {
      result = await withWebSessionCommandChannel(client, channel =>
        channel.updateWorkflowMode(flags.sessionId, {
          workflowMode: flags.workflowMode,
        }),
      );
    } else if (scope === 'web-session' && action === 'set-permission') {
      result = await withWebSessionCommandChannel(client, channel =>
        channel.updatePermissionLevel(flags.sessionId, {
          permissionLevel: flags.permissionLevel,
        }),
      );
    } else if (scope === 'web-session' && action === 'set-agent') {
      result = await withWebSessionCommandChannel(client, channel =>
        channel.updateAgent(flags.sessionId, {
          agent: flags.agent,
        }),
      );
    } else if (scope === 'web-session' && action === 'move') {
      result = await withWebSessionCommandChannel(client, channel =>
        channel.move(flags.sessionId, {
          prevSessionId: flags.prevSessionId,
          nextSessionId: flags.nextSessionId,
        }),
      );
    } else if (scope === 'web-session' && action === 'watch') {
      return await watchWebSession(client, flags, stdout);
    } else {
      throw new Error(`unsupported command: ${scope} ${action}`);
    }

    printJson(stdout, sanitizeConnection(result));
    return 0;
  } catch (error) {
    const payload = {
      error: {
        name: error instanceof Error ? error.name : 'Error',
        message: error instanceof Error ? error.message : String(error),
      },
    };
    printJson(stderr, payload);
    return 1;
  }
}
