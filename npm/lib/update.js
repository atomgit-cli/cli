// Safe npm-channel update checks and exact-version upgrades.

"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const crypto = require("crypto");
const { spawnSync } = require("child_process");
const pkg = require("../package.json");
const { npmInvocation, readInstallMetadata, stateDir } = require("./install-metadata");

// The npm coordinate this copy runs under. Derived from package.json so the
// same wrapper ships under parallel coordinates (atomgit-cli,
// @atomgit-cli/cli, @gitcode-cli/cli) and each updates itself, not a sibling.
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

function updateStatePath(env = process.env) {
  return path.join(stateDir(env), "update-state.json");
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

function configDir(env = process.env) {
  if (env.GC_CONFIG_DIR) return env.GC_CONFIG_DIR;
  return path.join(require("os").homedir(), ".config", "gc");
}

function updaterEnvironment(env = process.env) {
  const clean = { GC_NO_UPDATE_CHECK: "1" };
  for (const [key, value] of Object.entries(env)) {
    if (UPDATE_ENV_ALLOWLIST.has(key.toUpperCase())) clean[key] = value;
  }
  return clean;
}

function appendLog(message, env = process.env) {
  const file = path.join(stateDir(env), "update.log");
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  fs.appendFileSync(file, `${new Date().toISOString()} ${message}\n`, { mode: 0o600 });
}

function updateMode(env = process.env) {
  const fromEnv = (env.GC_UPDATE_MODE || "").toLowerCase();
  if (["auto", "notify", "off"].includes(fromEnv)) return fromEnv;
  const config = readJSON(path.join(configDir(env), "config.json"));
  const configured = (((config.hosts || {})["gitcode.com"] || {})["update.mode"] || "").toLowerCase();
  return ["auto", "notify", "off"].includes(configured) ? configured : "notify";
}

function disabledForInvocation(args = [], env = process.env) {
  return (
    truthy(env.GC_NO_UPDATE_CHECK) ||
    truthy(env.CI) ||
    args.includes("--no-update-check") ||
    args.includes("--no-interactive") ||
    updateMode(env) === "off"
  );
}

function truthy(value) {
  return ["1", "true", "yes"].includes(String(value || "").trim().toLowerCase());
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

function npmCommand(metadata) {
  if (metadata && metadata.npm) {
    if (/\.m?js$/i.test(metadata.npm) && fs.existsSync(metadata.npm)) {
      return { command: process.execPath, prefix: [metadata.npm] };
    }
    if ((!path.isAbsolute(metadata.npm) || fs.existsSync(metadata.npm)) && !/\.(?:cmd|bat)$/i.test(metadata.npm)) {
      return { command: metadata.npm, prefix: [] };
    }
  }
  const current = npmInvocation();
  if (current.shell) {
    throw new Error("npm CLI JavaScript runtime not found; reinstall Node.js or run npm install -g explicitly");
  }
  return current;
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

function runNpm(metadata, args, timeout = 15000) {
  const npm = npmCommand(metadata);
  const workDir = fs.mkdtempSync(path.join(os.tmpdir(), "gitcode-npm-update-"));
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

function checkLatest(metadata) {
  const result = runNpm(
    metadata,
    ["view", PACKAGE, "dist-tags.latest", "--json"],
    10000
  );
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error((result.stderr || "npm registry check failed").trim());
  const latest = JSON.parse(result.stdout);
  if (!stableVersion(latest)) throw new Error(`registry latest is not a stable semantic version: ${latest}`);
  return latest;
}

function globalWrapper(metadata) {
  const modules = process.platform === "win32"
    ? path.join(metadata.prefix, "node_modules")
    : path.join(metadata.prefix, "lib", "node_modules");
  return path.join(modules, ...PACKAGE.split("/"), "bin", "gc.js");
}

function healthCheck(metadata, expectedVersion) {
  const wrapper = globalWrapper(metadata);
  const result = spawnSync(process.execPath, [wrapper, "version", "--json"], {
    encoding: "utf8",
    timeout: 10000,
    windowsHide: true,
    env: updaterEnvironment(),
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error((result.stderr || "updated CLI health check failed").trim());
  const version = JSON.parse(result.stdout).version;
  if (compareVersions(version, expectedVersion) !== 0) {
    throw new Error(`updated CLI reported ${version}, expected ${expectedVersion}`);
  }
}

function installExact(metadata, version) {
  const result = runNpm(
    metadata,
    exactInstallArgs(metadata, version),
    120000
  );
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error((result.stderr || `npm install exited ${result.status}`).trim());
}

function exactInstallArgs(metadata, version) {
  return [
    "install", "-g", `${PACKAGE}@${version}`, "--ignore-scripts", "--no-audit", "--no-fund",
    "--prefix", metadata.prefix,
  ];
}

function acquireLock(file, now = Date.now()) {
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  try {
    return fs.openSync(file, "wx", 0o600);
  } catch (error) {
    if (error.code !== "EEXIST") throw error;
    try {
      if (now - fs.statSync(file).mtimeMs > LOCK_STALE_MS) {
        fs.unlinkSync(file);
        return fs.openSync(file, "wx", 0o600);
      }
    } catch {
      // Another process owns or just released the lock.
    }
    return null;
  }
}

function releaseLock(file, descriptor) {
  if (descriptor == null) return;
  try {
    fs.closeSync(descriptor);
  } finally {
    try {
      fs.unlinkSync(file);
    } catch {
      // Best effort; stale locks are recovered on the next attempt.
    }
  }
}

function resultObject(status, latest, message) {
  return { status, distribution: "npm", current: pkg.version, latest: latest || "", message };
}

function shouldOnlyNotify(options) {
  return Boolean(options.checkOnly || (options.mode === "notify" && options.background));
}

function performUpdate(options = {}) {
  const packageRoot = options.packageRoot || path.resolve(__dirname, "..");
  const metadata = options.metadata || readInstallMetadata(packageRoot);
  if (!metadata || !metadata.global || metadata.distribution !== "npm" || !metadata.prefix) {
    throw new Error("automatic update is available only for a global npm installation");
  }
  const latest = checkLatest(metadata);
  const comparison = compareVersions(pkg.version, latest);
  if (comparison == null) throw new Error(`cannot compare versions ${pkg.version} and ${latest}`);
  if (comparison >= 0) return resultObject("current", latest, `GitCode CLI ${pkg.version} is current.`);
  if (shouldOnlyNotify(options)) {
    return resultObject("available", latest, `GitCode CLI ${latest} is available (current ${pkg.version}).`);
  }

  try {
    installExact(metadata, latest);
    healthCheck(metadata, latest);
    return resultObject("updated", latest, `Updated GitCode CLI ${pkg.version} -> ${latest}.`);
  } catch (error) {
    try {
      installExact(metadata, pkg.version);
      healthCheck(metadata, pkg.version);
      throw new Error(`${error.message}; restored ${pkg.version}`);
    } catch (rollbackError) {
      if (rollbackError.message.includes("restored")) throw rollbackError;
      throw new Error(`${error.message}; rollback failed: ${rollbackError.message}`);
    }
  }
}

function runUpdate(options = {}) {
  const stateFile = options.stateFile || updateStatePath();
  const lockFile = `${stateFile}.lock`;
  const descriptor = acquireLock(lockFile);
  if (descriptor == null) return resultObject("busy", "", "Another GitCode CLI update is already running.");
  try {
    const mode = options.mode || updateMode();
    const result = performUpdate({ ...options, mode });
    const now = Date.now();
    const state = readJSON(stateFile);
    state.lastChecked = new Date(now).toISOString();
    state.nextCheck = now + TTL_MS;
    state.summary = { message: result.message, shown: !options.background };
    writeJSON(stateFile, state);
    appendLog(`status=${result.status} current=${result.current} latest=${result.latest || "none"}`);
    return result;
  } catch (error) {
    const now = Date.now();
    const state = readJSON(stateFile);
    state.lastChecked = new Date(now).toISOString();
    state.nextCheck = now + TTL_MS;
    state.summary = { message: "Automatic update failed; run gitcode update for details.", shown: !options.background };
    writeJSON(stateFile, state);
    appendLog("status=error; run an explicit update for details");
    throw error;
  } finally {
    releaseLock(lockFile, descriptor);
  }
}

function shouldSchedule(args = [], env = process.env, now = Date.now()) {
  if (disabledForInvocation(args, env)) return false;
  const state = readJSON(updateStatePath(env));
  return !state.nextCheck || Number(state.nextCheck) <= now;
}

function showPendingSummary(stderr = process.stderr, env = process.env) {
  const file = updateStatePath(env);
  const lockFile = `${file}.lock`;
  const descriptor = acquireLock(lockFile);
  if (descriptor == null) return;
  try {
    const state = readJSON(file);
    if (!state.summary || state.summary.shown) return;
    stderr.write(`${state.summary.message}\n`);
    state.summary.shown = true;
    writeJSON(file, state);
  } finally {
    releaseLock(lockFile, descriptor);
  }
}

function showFirstRunNotice(stderr = process.stderr, env = process.env) {
  if (disabledForInvocation([], env)) return;
  const file = updateStatePath(env);
  const lockFile = `${file}.lock`;
  const descriptor = acquireLock(lockFile);
  if (descriptor == null) return;
  try {
    const state = readJSON(file);
    if (state.noticeShown) return;
    stderr.write(
      "GitCode CLI installed by npm checks daily for stable updates and notifies without installing them.\n" +
        'Run "gitcode update", opt in with "gitcode config set update.mode auto", or disable checks with update.mode off.\n'
    );
    state.noticeShown = true;
    writeJSON(file, state);
  } finally {
    releaseLock(lockFile, descriptor);
  }
}

module.exports = {
  PACKAGE,
  OFFICIAL_REGISTRY,
  TTL_MS,
  acquireLock,
  appendLog,
  checkLatest,
  compareVersions,
  disabledForInvocation,
  exactInstallArgs,
  globalWrapper,
  npmCommand,
  performUpdate,
  readJSON,
  releaseLock,
  runUpdate,
  shouldSchedule,
  shouldOnlyNotify,
  showFirstRunNotice,
  showPendingSummary,
  stableVersion,
  updateMode,
  updaterEnvironment,
  updateStatePath,
  withNpmIsolation,
  writeJSON,
};
