#!/usr/bin/env node
// Wrapper entry for @gitcode-cli/cli.
//
// - Default: resolve the bundled platform binary and exec it with the
//   remaining args (so `gc version`, `gc issue list`, ... all work).
// - Special subcommand `install`: run the Node-side bootstrap (copy binary to
//   a global bin dir, install shell completions on Linux/macOS) so an explicit,
//   registry-isolated npx bootstrap works without a global npm install.
//
// This file must stay dependency-free (no require of external packages) so
// the npm package needs no install step beyond shipping the binaries.

"use strict";

const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");
const { resolveBinaryName } = require("../lib/platform");
const { ensureInstallMetadata, readInstallMetadata } = require("../lib/install-metadata");
const pkg = require("../package.json");
const {
  shouldSchedule,
  showFirstRunNotice,
  showPendingSummary,
  updaterEnvironment,
} = require("../lib/update");

// PLATFORMS_DIR can be overridden (e.g. for deterministic unit tests that
// point at an empty dir to exercise the ENOENT path). Defaults to the bundled
// platforms directory shipped with this package.
const PLATFORMS_DIR = process.env.GC_PLATFORMS_DIR || path.join(__dirname, "platforms");

function binPath() {
  const name = resolveBinaryName(process.platform, process.arch);
  return path.join(PLATFORMS_DIR, name);
}

function runBinary(args) {
  const p = binPath();
  // Ensure the exec bit survives tarball extraction on posix.
  if (process.platform !== "win32") {
    try {
      fs.chmodSync(p, 0o755);
    } catch {
      /* best-effort; ENOENT handled below */
    }
  }
  const child = spawn(p, args, {
    stdio: "inherit",
    env: {
      ...process.env,
      GITCODE_CLI_DISTRIBUTION: "npm",
      GITCODE_CLI_ENTRYPOINT: process.argv[1],
      GITCODE_CLI_BINARY: p,
      GITCODE_CLI_PACKAGE_ROOT: path.resolve(__dirname, ".."),
    },
  });
  child.on("error", (err) => {
    if (err.code === "ENOENT") {
      process.stderr.write(
        `gc binary not found at ${p}. ` +
          `Run "npx --yes --ignore-scripts --registry=https://registry.npmjs.org ` +
          `${pkg.name.startsWith("@") ? `--${pkg.name.split("/")[0]}:registry=https://registry.npmjs.org ` : ""}${pkg.name}@latest install" first.\n`
      );
      process.exit(127);
    }
    process.stderr.write(`failed to run gc: ${err.message}\n`);
    process.exit(1);
  });
  child.on("exit", (code, signal) => {
    if (signal) {
      // Forward the signal so shells report it correctly (do not force exit 1).
      try {
        process.kill(process.pid, signal);
      } catch {
        process.exit(1);
      }
      return;
    }
    finishUpdateLifecycle(args);
    process.exit(code == null ? 1 : code);
  });
}

function finishUpdateLifecycle(args) {
  const packageRoot = path.resolve(__dirname, "..");
  const metadata = readInstallMetadata(packageRoot);
  if (!metadata || !metadata.global || metadata.distribution !== "npm") return;
  try {
    showPendingSummary();
    showFirstRunNotice();
    if (!shouldSchedule(args)) return;
    const helper = path.join(packageRoot, "lib", "update-helper.js");
    const child = spawn(process.execPath, [helper, "--background"], {
      detached: true,
      stdio: "ignore",
      windowsHide: true,
      env: { ...updaterEnvironment(), GC_UPDATE_BACKGROUND: "1" },
    });
    child.unref();
  } catch (error) {
    process.stderr.write(`update check skipped: ${error.message}\n`);
  }
}

function main() {
  const args = process.argv.slice(2);
  ensureInstallMetadata(path.resolve(__dirname, ".."), pkg.version);

  // Intercept the install subcommand (lark-cli-style bootstrap). If the Go
  // CLI grows a real "gc install" later, prefer forwarding "--help"/unknown
  // flags through and reserve only the bare "install" first token.
  if (args[0] === "install") {
    const { runInstall } = require("../lib/install");
    runInstall(args.slice(1)).catch((err) => {
      process.stderr.write(`install failed: ${err && err.message ? err.message : err}\n`);
      process.exit(1);
    });
    return;
  }

  if (args[0] === "update") {
    const { main: runUpdate } = require("../lib/update-helper");
    process.exitCode = runUpdate(args.slice(1));
    return;
  }

  runBinary(args);
}

main();
