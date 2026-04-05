#!/usr/bin/env node
import { runCli } from '../../../packages/node-sdk/src/cli.js';

const exitCode = await runCli(process.argv.slice(2));
if (typeof exitCode === 'number') {
  process.exitCode = exitCode;
}
