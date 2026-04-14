#!/usr/bin/env node
import packageJson from '../package.json' with { type: 'json' };
import { runCodeKanbanCli } from '../src/index.js';

const exitCode = await runCodeKanbanCli(process.argv.slice(2), {
  version: packageJson.version,
});
if (typeof exitCode === 'number') {
  process.exitCode = exitCode;
}
