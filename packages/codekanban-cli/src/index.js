import { runCli, createHelpText } from './runtime.js';

export * from './core.js';
export { runCodeKanbanCli };

import { runCodeKanbanCliWithRuntime } from './core.js';

async function runCodeKanbanCli(argv, options = {}) {
  return await runCodeKanbanCliWithRuntime(argv, {
    runCli,
    createHelpText,
  }, options);
}
