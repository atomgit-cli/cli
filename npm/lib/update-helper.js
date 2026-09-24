#!/usr/bin/env node

"use strict";

const { runUpdate } = require("./update");

const UPDATE_HELP = `Check for or apply an npm-channel update.

Usage: gc update [flags]

Flags:
  --check            Check for an update without installing it
  --json             Print a stable JSON object to stdout
  --background       Internal: run as the detached background updater
  --no-update-check  Accepted for CLI consistency (ignored here)
  --no-interactive   Accepted for CLI consistency (ignored here)
  -h, --help         Show this help

This wrapper command manages global npm installs (atomgit-cli,
@atomgit-cli/cli, @gitcode-cli/cli). npm-bootstrap installations are
handled by the gc binary itself. Other channels (pip, Homebrew, deb, rpm)
stay user-controlled and are never invoked implicitly.
`;

function parseArgs(args) {
  const options = { background: false, checkOnly: false, json: false, help: false };
  for (const arg of args) {
    if (arg === "--background") options.background = true;
    else if (arg === "--check") options.checkOnly = true;
    else if (arg === "--json") options.json = true;
    else if (arg === "--help" || arg === "-h") options.help = true;
    else if (arg === "--no-update-check" || arg === "--no-interactive") {
      // Global flags: accepted for CLI consistency, no effect here.
    } else throw new Error(`unknown update argument: ${arg}`);
  }
  return options;
}

function main(args = process.argv.slice(2)) {
  let options;
  try {
    options = parseArgs(args);
    if (options.help) {
      if (!options.background) process.stdout.write(UPDATE_HELP);
      return 0;
    }
    const result = runUpdate(options);
    if (!options.background) {
      process.stdout.write(options.json ? `${JSON.stringify(result)}\n` : `${result.message}\n`);
    }
    return 0;
  } catch (error) {
    if (!options || !options.background) {
      if (options && options.json) {
        process.stdout.write(`${JSON.stringify({ status: "error", distribution: "npm", current: "", latest: "", message: error.message })}\n`);
      } else {
        process.stderr.write(`update failed: ${error.message}\n`);
      }
    }
    return 1;
  }
}

if (require.main === module) process.exitCode = main();

module.exports = { UPDATE_HELP, main, parseArgs };
