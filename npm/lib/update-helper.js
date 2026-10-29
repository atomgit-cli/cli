#!/usr/bin/env node

"use strict";

const { runUpdate } = require("./update");

// PowerShell reserves "gc" as Get-Content; show the name Windows users run.
const COMMAND = process.platform === "win32" ? "gitcode" : "gc";

const UPDATE_HELP = `Check for or apply an npm-channel update.

Usage: ${COMMAND} update [flags]

Flags:
  --check            Check for an update without installing it
  --json             Print a stable JSON object to stdout
  --background       Internal: run as the detached background updater
  --no-update-check  Accepted for CLI consistency (ignored here)
  --no-interactive   Accepted for CLI consistency (ignored here)
  -h, --help         Show this help

This wrapper command manages global npm installs (atomgit-cli,
@atomgit-cli/cli, @gitcode-cli/cli). npm-bootstrap installations are
handled by the ${COMMAND} binary itself. Other channels (pip, uv, Homebrew, deb, rpm)
stay user-controlled and are never invoked implicitly.
`;

// cobra/pflag accepts --flag and --flag=value for boolean flags; only a
// truthy value counts as set (mirrors update.js flagSet). A non-boolean
// value in the =form is rejected.
function booleanFlag(args, name) {
  return args.some((arg) => arg === name ||
    (arg.startsWith(`${name}=`) && ["1", "true", "yes"].includes(arg.slice(name.length + 1).trim().toLowerCase())));
}

function isBooleanFlagArg(arg, name) {
  if (arg === name) return true;
  if (!arg.startsWith(`${name}=`)) return false;
  return ["1", "true", "yes", "0", "false", "no", ""].includes(arg.slice(name.length + 1).trim().toLowerCase());
}

function parseArgs(args) {
  const options = { background: false, checkOnly: false, json: false, help: false };
  options.background = booleanFlag(args, "--background");
  options.checkOnly = booleanFlag(args, "--check");
  options.json = booleanFlag(args, "--json");
  for (const arg of args) {
    if (arg === "--help" || arg === "-h") options.help = true;
    else if (["--background", "--check", "--json"].some((name) => isBooleanFlagArg(arg, name))) {
      // Boolean flags, handled above (including =value forms).
    } else if (arg === "--no-update-check" || arg === "--no-interactive" ||
        arg.startsWith("--no-update-check=") || arg.startsWith("--no-interactive=")) {
      // Global flags: accepted for CLI consistency (including the
      // --flag=value forms cobra/pflag allows), no effect here.
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
      // parseArgs failures leave options unset: detect --json from the raw
      // argv so JSON consumers still get a JSON error object.
      const json = options ? options.json : args.includes("--json");
      if (json) {
        process.stdout.write(`${JSON.stringify({ status: "error", distribution: "npm", current: "", latest: "", message: error.summaryMessage || error.message })}\n`);
      } else {
        // Prefer the composed summary message: it carries the repair
        // guidance a permanent failure recorded, not just the raw error.
        process.stderr.write(`${error.summaryMessage || `update failed: ${error.message}`}\n`);
      }
    }
    return 1;
  }
}

if (require.main === module) process.exitCode = main();

module.exports = { UPDATE_HELP, main, parseArgs };
