#!/usr/bin/env node
// Standalone updater copied next to npm-bootstrap binaries.

"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const crypto = require("crypto");
const { spawnSync } = require("child_process");

// Derived from package.json so parallel npm coordinates (atomgit-cli,
// @atomgit-cli/cli, @gitcode-cli/cli) each bootstrap-update themselves.
// install.js rewrites this exact two-line marker to a literal
// `const PACKAGE = "<name>";` when copying the helper into the bin dir,
// because the copy runs outside the package tree (standalone, built-ins only).
const pkg = require("../package.json");
const PACKAGE = pkg.name;
const OFFICIAL_REGISTRY = "https://registry.npmjs.org";
const TTL_MS = 24 * 60 * 60 * 1000;
const LOCK_STALE_MS = 15 * 60 * 1000;
const UPDATE_ENV_ALLOWLIST = new Set([
  "ALL_PROXY", "APPDATA", "COMSPEC", "GC_CONFIG_DIR", "GC_STATE_DIR", "GC_UPDATE_MODE",
  "HOME", "HTTP_PROXY", "HTTPS_PROXY", "LANG", "LC_ALL", "LOCALAPPDATA", "NO_PROXY",
  "NODE_EXTRA_CA_CERTS", "PATH", "PATHEXT", "SSL_CERT_DIR", "SSL_CERT_FILE", "SYSTEMROOT", "TEMP", "TMP",
  "USERPROFILE", "WINDIR", "XDG_CONFIG_HOME", "XDG_STATE_HOME",
]);

function parseArgs(args) {
  const options = { background: false, check: false, force: false, json: false, manifest: "", parentPid: 0 };
  for (let index = 0; index < args.length; index += 1) {
    const arg = args[index];
    if (arg === "--background") options.background = true;
    else if (arg === "--check") options.check = true;
    else if (arg === "--force") options.force = true;
    else if (arg === "--json") options.json = true;
    else if (arg === "--manifest" && args[index + 1]) options.manifest = path.resolve(args[++index]);
    else if (arg === "--parent-pid" && args[index + 1]) options.parentPid = Number(args[++index]);
    else throw new Error(`unknown updater argument: ${arg}`);
  }
  if (!options.manifest) options.manifest = path.join(__dirname, ".gitcode-install.json");
  return options;
}

function readJSON(file, fallback = {}) {
  try {
    return JSON.parse(fs.readFileSync(file, "utf8"));
  } catch {
    return fallback;
  }
}

function writeJSON(file, value) {
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  const temp = `${file}.tmp-${process.pid}-${crypto.randomBytes(8).toString("hex")}`;
  fs.writeFileSync(temp, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600, flag: "wx" });
  try {
    fs.renameSync(temp, file);
  } catch (error) {
    if (!["EEXIST", "EPERM"].includes(error.code)) throw error;
    fs.unlinkSync(file);
    fs.renameSync(temp, file);
  }
}

function statePath(env = process.env) {
  let root = env.GC_STATE_DIR;
  if (!root && process.platform === "win32") {
    root = path.join(env.LOCALAPPDATA || path.join(os.homedir(), "AppData", "Local"), "gitcode-cli");
  }
  if (!root) root = path.join(env.XDG_STATE_HOME || path.join(os.homedir(), ".local", "state"), "gitcode-cli");
  return path.join(root, "update-state.json");
}

function updateMode(env = process.env) {
  const requested = String(env.GC_UPDATE_MODE || "").toLowerCase();
  if (["auto", "notify", "off"].includes(requested)) return requested;
  const configRoot = env.GC_CONFIG_DIR || path.join(os.homedir(), ".config", "gc");
  const config = readJSON(path.join(configRoot, "config.json"));
  const stored = (((config.hosts || {})["gitcode.com"] || {})["update.mode"] || "").toLowerCase();
  return ["auto", "notify", "off"].includes(stored) ? stored : "notify";
}

function updaterEnvironment(env = process.env) {
  const clean = { GC_NO_UPDATE_CHECK: "1" };
  for (const [key, value] of Object.entries(env)) {
    if (UPDATE_ENV_ALLOWLIST.has(key.toUpperCase())) clean[key] = value;
  }
  return clean;
}

function appendLog(message) {
  const file = path.join(path.dirname(statePath()), "update.log");
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  fs.appendFileSync(file, `${new Date().toISOString()} ${message}\n`, { mode: 0o600 });
}

function stableVersion(value) {
  const match = String(value || "").trim().match(/^v?(\d+)\.(\d+)\.(\d+)$/);
  return match ? match.slice(1).map(Number) : null;
}

function compareVersions(left, right) {
  const a = stableVersion(left);
  const b = stableVersion(right);
  if (!a || !b) return null;
  for (let index = 0; index < 3; index += 1) {
    if (a[index] !== b[index]) return a[index] < b[index] ? -1 : 1;
  }
  return 0;
}

function npmCommand(manifest) {
  if (manifest.npm && /\.m?js$/i.test(manifest.npm) && fs.existsSync(manifest.npm)) {
    return { command: process.execPath, prefix: [manifest.npm] };
  }
  if (manifest.npm && (!path.isAbsolute(manifest.npm) || fs.existsSync(manifest.npm)) && !/\.(?:cmd|bat)$/i.test(manifest.npm)) {
    return { command: manifest.npm, prefix: [] };
  }
  const candidates = [
    path.join(path.dirname(process.execPath), "node_modules", "npm", "bin", "npm-cli.js"),
    path.resolve(path.dirname(process.execPath), "..", "lib", "node_modules", "npm", "bin", "npm-cli.js"),
  ];
  const current = candidates.find((candidate) => fs.existsSync(candidate));
  if (current) return { command: process.execPath, prefix: [current] };
  throw new Error("npm CLI JavaScript runtime not found; reinstall Node.js or run npm install -g explicitly");
}

function withNpmIsolation(args, userConfig, globalConfig) {
  const isolation = [
    `--userconfig=${userConfig}`,
    `--globalconfig=${globalConfig}`,
    ...(PACKAGE.startsWith("@") ? [`--${PACKAGE.split("/")[0]}:registry=${OFFICIAL_REGISTRY}`] : []),
    `--registry=${OFFICIAL_REGISTRY}`,
  ];
  const separator = args.indexOf("--");
  if (separator < 0) return [...args, ...isolation];
  return [...args.slice(0, separator), ...isolation, ...args.slice(separator)];
}

function runNpm(manifest, args, timeout) {
  const npm = npmCommand(manifest);
  const workDir = fs.mkdtempSync(path.join(os.tmpdir(), "gitcode-npm-bootstrap-"));
  try {
    const userConfig = path.join(workDir, "user.npmrc");
    const globalConfig = path.join(workDir, "global.npmrc");
    fs.writeFileSync(userConfig, "", { flag: "wx", mode: 0o600 });
    fs.writeFileSync(globalConfig, "", { flag: "wx", mode: 0o600 });
    return spawnSync(npm.command, [...npm.prefix, ...withNpmIsolation(args, userConfig, globalConfig)], {
      encoding: "utf8",
      timeout,
      windowsHide: true,
      cwd: workDir,
      env: updaterEnvironment(),
    });
  } finally {
    fs.rmSync(workDir, { recursive: true, force: true });
  }
}

function latestVersion(manifest) {
  const result = runNpm(
    manifest,
    ["view", PACKAGE, "dist-tags.latest", "--json"],
    10000
  );
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error((result.stderr || "npm registry check failed").trim());
  const version = JSON.parse(result.stdout);
  if (!stableVersion(version)) throw new Error(`registry latest is not stable: ${version}`);
  return version;
}

function waitForParent(pid) {
  if (!pid) return;
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    try {
      process.kill(pid, 0);
      Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 100);
    } catch {
      return;
    }
  }
  throw new Error(`parent process ${pid} did not exit before update timeout`);
}

function healthCheck(manifest, expected) {
  const entry = path.join(manifest.targetDir, process.platform === "win32" ? "gitcode.exe" : "gitcode");
  const result = spawnSync(entry, ["version", "--json"], {
    encoding: "utf8",
    timeout: 10000,
    windowsHide: true,
    env: updaterEnvironment(),
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error((result.stderr || "bootstrap health check failed").trim());
  const actual = JSON.parse(result.stdout).version;
  if (compareVersions(actual, expected) !== 0) throw new Error(`updated CLI reported ${actual}, expected ${expected}`);
}

function installLatest(manifest, latest) {
  const args = [
    "exec",
    "--yes",
    `--package=${PACKAGE}@${latest}`,
    "--ignore-scripts",
    "--",
    "gitcode",
    "install",
    "--target-dir",
    manifest.targetDir,
  ];
  const result = runNpm(manifest, args, 120000);
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error((result.stderr || `npm exec exited ${result.status}`).trim());
  healthCheck(manifest, latest);
}

function acquireLock(file) {
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  try {
    return fs.openSync(file, "wx", 0o600);
  } catch (error) {
    if (error.code === "EEXIST") {
      try {
        if (Date.now() - fs.statSync(file).mtimeMs > LOCK_STALE_MS) {
          fs.unlinkSync(file);
          return fs.openSync(file, "wx", 0o600);
        }
      } catch {
        // Another process owns or just released the lock.
      }
      return null;
    }
    throw error;
  }
}

function run(options) {
  const manifest = readJSON(options.manifest);
  if (manifest.distribution !== "npm-bootstrap" || !manifest.targetDir || !manifest.version) {
    throw new Error("invalid npm-bootstrap install manifest");
  }
  const stateFile = statePath();
  const state = readJSON(stateFile);
  if (!options.force && !options.check && state.nextCheck && Number(state.nextCheck) > Date.now()) {
    return { status: "cached", distribution: "npm-bootstrap", current: manifest.version, latest: "", message: "Update check is not due yet." };
  }
  const lock = `${stateFile}.lock`;
  const descriptor = acquireLock(lock);
  if (descriptor == null) return { status: "busy", distribution: "npm-bootstrap", current: manifest.version, latest: "", message: "Another update is running." };
  try {
    const latest = latestVersion(manifest);
    const comparison = compareVersions(manifest.version, latest);
    let result;
    if (comparison == null) throw new Error(`cannot compare ${manifest.version} and ${latest}`);
    if (comparison >= 0) {
      result = { status: "current", distribution: "npm-bootstrap", current: manifest.version, latest, message: `GitCode CLI ${manifest.version} is current.` };
    } else if (options.check || (updateMode() === "notify" && !options.force)) {
      result = { status: "available", distribution: "npm-bootstrap", current: manifest.version, latest, message: `GitCode CLI ${latest} is available.` };
    } else if (updateMode() === "off" && !options.force) {
      result = { status: "disabled", distribution: "npm-bootstrap", current: manifest.version, latest, message: "Automatic updates are disabled." };
    } else {
      waitForParent(options.parentPid);
      installLatest(manifest, latest);
      result = { status: "updated", distribution: "npm-bootstrap", current: manifest.version, latest, message: `Updated GitCode CLI ${manifest.version} -> ${latest}.` };
    }
    const now = Date.now();
    state.lastChecked = new Date(now).toISOString();
    state.nextCheck = now + TTL_MS;
    state.summary = { message: result.message, shown: !options.background };
    writeJSON(stateFile, state);
    appendLog(`status=${result.status} current=${result.current} latest=${result.latest || "none"}`);
    return result;
  } finally {
    fs.closeSync(descriptor);
    try {
      fs.unlinkSync(lock);
    } catch {
      // Stale bootstrap locks are visible and can be removed by the user.
    }
  }
}

function main(args = process.argv.slice(2)) {
  let options;
  try {
    options = parseArgs(args);
    const result = run(options);
    if (!options.background) process.stdout.write(options.json ? `${JSON.stringify(result)}\n` : `${result.message}\n`);
    return 0;
  } catch (error) {
    if (options && options.background) {
      const file = statePath();
      const state = readJSON(file);
      state.nextCheck = Date.now() + TTL_MS;
      state.summary = { message: "Automatic update failed; run gitcode update for details.", shown: false };
      writeJSON(file, state);
      appendLog("status=error; run an explicit update for details");
    }
    if (!options || !options.background) {
      if (options && options.json) process.stdout.write(`${JSON.stringify({ status: "error", distribution: "npm-bootstrap", current: "", latest: "", message: error.message })}\n`);
      else process.stderr.write(`update failed: ${error.message}\n`);
    }
    return 1;
  }
}

if (require.main === module) process.exitCode = main();

module.exports = {
  compareVersions, main, npmCommand, parseArgs, stableVersion, updateMode, updaterEnvironment, withNpmIsolation,
};
