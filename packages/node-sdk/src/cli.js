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

function parseCliArgs(argv) {
  const positionals = [];
  const flags = {
    addDirs: [],
    extraArgs: [],
    includeTerminal: true,
    includeAI: true,
    refresh: false,
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
      case '--agent':
        flags.agent = readFlagValue(argv, index, token);
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
      case '--task-id':
        flags.taskId = readFlagValue(argv, index, token);
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
      case '--refresh':
        flags.refresh = true;
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
  return next;
}

function printJson(stream, payload) {
  stream.write(createJsonOutput(payload));
}

export async function runCli(argv) {
  try {
    const { positionals, flags } = parseCliArgs(argv);
    const [scope, action] = positionals;
    if (!scope || !action) {
      throw new Error('usage: <workflow|session|terminal> <action> --base-url <url> [...]');
    }

    if (scope === 'workflow' && action === 'command') {
      const result = buildAgentLaunchSpec({
        agent: flags.agent,
        profile: flags.profile,
        permissions: buildPermissions(flags),
        extraArgs: flags.extraArgs,
        prompt: flags.prompt || 'Inspect the project and respond.',
      });
      printJson(process.stdout, sanitizeConnection(result));
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
        taskId: flags.taskId,
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
    } else {
      throw new Error(`unsupported command: ${scope} ${action}`);
    }

    printJson(process.stdout, sanitizeConnection(result));
    return 0;
  } catch (error) {
    const payload = {
      error: {
        name: error instanceof Error ? error.name : 'Error',
        message: error instanceof Error ? error.message : String(error),
      },
    };
    printJson(process.stderr, payload);
    return 1;
  }
}
