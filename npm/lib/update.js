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
// Locks older than 15 minutes are reclaimed. The worst-case hold is the
// check budget (63s) plus a failed install rolled back with health checks
// (~723s total); keep any timeout adjustment comfortably under this bound.
const LOCK_STALE_MS = 15 * 60 * 1000;
const CHECK_ATTEMPTS = 3;
const CHECK_TIMEOUT_MS = 20000;
const CHECK_RETRY_DELAY_MS = 1000;
const INSTALL_TIMEOUT_MS = 300000;
const UPDATE_ENV_ALLOWLIST = new Set([
  "ALL_PROXY", "APPDATA", "COMSPEC", "GC_CONFIG_DIR", "GC_STATE_DIR", "GC_UPDATE_MODE",
  "HOME", "HTTP_PROXY", "HTTPS_PROXY", "LANG", "LC_ALL", "LOCALAPPDATA", "NO_PROXY",
  "NODE_EXTRA_CA_CERTS", "PATH", "PATHEXT", "SSL_CERT_DIR", "SSL_CERT_FILE", "SYSTEMROOT", "TEMP", "TMP",
  "USERPROFILE", "WINDIR", "XDG_CONFIG_HOME", "XDG_STATE_HOME",
]);

// Scoped per package coordinate and channel: npm-global and npm-bootstrap
// installs (and parallel coordinates) each keep their own nextCheck and
// summary instead of pushing each other's schedule or cross-showing
// summaries. An explicit GC_STATE_DIR keeps the legacy flat file as a
// deliberate user (and test) override.
function updateStatePath(env = process.env) {
  if (env.GC_STATE_DIR) return path.join(env.GC_STATE_DIR, "update-state.json");
  return path.join(stateDir(env), PACKAGE, "npm", "update-state.json");
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
    try {
      fs.renameSync(temp, file);
    } catch (error) {
      if (!["EEXIST", "EPERM"].includes(error.code)) throw error;
      // Retry once without unlinking: a concurrent writer may have just
      // replaced the target, and the plain retry (or a transient lock)
      // avoids destroying their write. Mirrors lib/bootstrap-update-helper.js.
      try {
        fs.renameSync(temp, file);
        return;
      } catch {
        // Fall through to the replace below (Windows rename-over-existing).
      }
      fs.unlinkSync(file);
      fs.renameSync(temp, file);
    }
  } catch (error) {
    // Never strand the temp file: an update-state.json.tmp-* residue
    // matches no sweep prefix and would outlive the failed write.
    try {
      fs.unlinkSync(temp);
    } catch {
      // Preserve the original error.
    }
    throw error;
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

// Appends to the shared update.log at the state root. The `file` override
// exists so callers (and tests) that inject a custom state file can keep
// the log beside it instead of writing to the production state directory.
function appendLog(message, env = process.env, file) {
  try {
    const target = file || path.join(stateDir(env), "update.log");
    fs.mkdirSync(path.dirname(target), { recursive: true, mode: 0o700 });
    fs.appendFileSync(target, `${new Date().toISOString()} ${message}\n`, { mode: 0o600 });
  } catch {
    // Logging is best-effort: an unwritable update.log (root-owned file,
    // read-only directory) must never turn a successful update into a
    // reported failure or replace the real result summary.
  }
}

// Collapses an error into one bounded line so failure summaries stay readable
// on the next launch while still carrying the real cause (registry resets,
// timeouts, spawn failures) instead of a generic dead-end notice.
function summarizeError(error) {
  const raw = (error && (error.message || error.code)) || error;
  let text = String(raw == null ? "" : raw).replace(/\s+/g, " ").trim() || "unknown error";
  if (text.length > 200) {
    let cut = text.slice(0, 200);
    // Do not leave an orphaned high surrogate at the cut boundary.
    const last = cut.charCodeAt(cut.length - 1);
    if (last >= 0xd800 && last <= 0xdbff) cut = cut.slice(0, -1);
    text = `${cut}...`;
  }
  return text;
}

// Permanent update failures: retrying cannot fix them (the on-disk install
// manifest or the npm runtime is broken), so background checks stop and the
// summary carries the repair action. Registry oddities (an unstable
// dist-tag) stay retryable: the maintainer fixing the tag must auto-recover
// notifications.
const PERMANENT_ERROR_PATTERNS = [
  /invalid npm-bootstrap install manifest/,
  /npm CLI JavaScript runtime not found/,
];

function permanentUpdateError(error) {
  const text = summarizeError(error);
  return PERMANENT_ERROR_PATTERNS.some((pattern) => pattern.test(text));
}

// Failure backoff: 1h, 2h, 4h, ... capped at the daily TTL, so a
// persistently failing environment (offline, blocking proxy) retries
// progressively instead of sitting blind for 24h after the first failure.
function failureBackoffMs(streak) {
  const step = Number(streak) > 0 ? Number(streak) : 1;
  return Math.min(TTL_MS, 60 * 60 * 1000 * 2 ** (step - 1));
}

// Stable identity of a failure for summary deduplication: the same error
// resurfacing on the next backoff attempt must not requeue (and reprint)
// what the user already saw.
function errorFingerprint(error) {
  return crypto.createHash("sha256").update(summarizeError(error)).digest("hex").slice(0, 16);
}

function sleepSync(ms) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

// A failing command sometimes reports on stdout with an empty stderr; fall
// back to the first non-empty stdout line before the generic message.
// Mirrors lib/install.js and lib/bootstrap-update-helper.js.
function commandFailureDetail(result, fallback) {
  const stderr = String(result.stderr || "").trim();
  if (stderr) return stderr;
  const stdoutLine = String(result.stdout || "").split(/\r?\n/).map((line) => line.trim()).find(Boolean);
  return stdoutLine || fallback;
}

function updateMode(env = process.env) {
  const fromEnv = (env.GC_UPDATE_MODE || "").toLowerCase();
  if (["auto", "notify", "off"].includes(fromEnv)) return fromEnv;
  const config = readJSON(path.join(configDir(env), "config.json"));
  const configured = (((config.hosts || {})["gitcode.com"] || {})["update.mode"] || "").toLowerCase();
  return ["auto", "notify", "off"].includes(configured) ? configured : "notify";
}

function truthy(value) {
  return ["1", "true", "yes"].includes(String(value || "").trim().toLowerCase());
}

// cobra/pflag accepts --flag and --flag=value for boolean flags while the
// wrapper sees raw argv; --flag=false must not count as set.
function flagSet(args, name) {
  return args.some((arg) => arg === name ||
    (arg.startsWith(`${name}=`) && truthy(arg.slice(name.length + 1))));
}

// CI detection covers the vendors that do not set CI=true (Jenkins,
// TeamCity, CodeShip): the variants carry version or build strings, so
// their mere presence is the signal. Mirrors the Go ciEnvironment.
function ciEnvironment(env = process.env) {
  if (truthy(env.CI)) return true;
  return Boolean(env.GITHUB_ACTIONS || env.BUILD_NUMBER || env.CI_NAME || env.TEAMCITY_VERSION);
}

function disabledForInvocation(args = [], env = process.env) {
  return (
    truthy(env.GC_NO_UPDATE_CHECK) ||
    ciEnvironment(env) ||
    flagSet(args, "--no-update-check") ||
    flagSet(args, "--no-interactive") ||
    updateMode(env) === "off"
  );
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
  let lastError;
  for (let attempt = 1; attempt <= CHECK_ATTEMPTS; attempt += 1) {
    const result = runNpm(
      metadata,
      ["view", PACKAGE, "dist-tags.latest", "--json"],
      CHECK_TIMEOUT_MS
    );
    if (!result.error && result.status === 0) {
      const latest = JSON.parse(result.stdout);
      if (!stableVersion(latest)) throw new Error(`registry latest is not a stable semantic version: ${latest}`);
      return latest;
    }
    lastError = result.error || new Error((result.stderr || "npm registry check failed").trim());
    // Deterministic registry errors (package deleted or renamed, auth or
    // forbidden): retrying cannot fix them, so fail fast instead of burning
    // the attempt budget on every scheduled check.
    if (DETERMINISTIC_NPM_ERROR.test(String(result.stderr || ""))) break;
    if (attempt < CHECK_ATTEMPTS) sleepSync(CHECK_RETRY_DELAY_MS * attempt);
  }
  throw lastError;
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
    // The first run after an --ignore-scripts install must discover global
    // metadata (two npm invocations of 5s each) before spawning the binary;
    // 10s turned that into false-negative rollbacks on slow machines.
    timeout: 30000,
    windowsHide: true,
    env: updaterEnvironment(),
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(commandFailureDetail(result, "updated CLI health check failed"));
  const version = JSON.parse(result.stdout).version;
  if (compareVersions(version, expectedVersion) !== 0) {
    throw new Error(`updated CLI reported ${version}, expected ${expectedVersion}`);
  }
}

function installExact(metadata, version) {
  const result = runNpm(
    metadata,
    exactInstallArgs(metadata, version),
    INSTALL_TIMEOUT_MS
  );
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error((result.stderr || `npm install exited ${result.status}`).trim());
}

function exactInstallArgs(metadata, version) {
  return [
    "install", "-g", `${PACKAGE}@${version}`, "--ignore-scripts", "--no-audit", "--no-fund",
    // A 60s fetch timeout lets npm's own fetch-retries finish inside the
    // 300s overall budget; with the npm default (300s) a single stalled
    // request consumes the whole budget and the retries never run.
    "--fetch-timeout=60000",
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
        // Atomic reclaim: renaming the stale lock to a private name lets
        // exactly one of two racing reclaimers win. The loser's rename
        // fails with ENOENT (the winner already retired it) and its claim
        // then hits the winner's fresh lock (EEXIST) — unlinking the
        // shared path instead would delete each other's fresh locks and
        // let both believe they hold it.
        const retired = `${file}.retired-${process.pid}-${crypto.randomBytes(4).toString("hex")}`;
        fs.renameSync(file, retired);
        // Re-verify staleness on the retired file: a fresh claim may have
        // landed between the check above and this rename, and renaming a
        // live lock away would silently break its holder. Restore and
        // yield in that case.
        if (now - fs.statSync(retired).mtimeMs <= LOCK_STALE_MS) {
          try {
            fs.renameSync(retired, file);
          } catch {
            // Someone claimed the path while we held their lock aside.
          }
          return null;
        }
        try {
          fs.unlinkSync(retired);
        } catch {
          // Best effort; a stranded .retired-* file is inert debris.
        }
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
    // Identity check: if this lock was reclaimed as stale while we were
    // stalled, the path now holds someone else's fresh lock — removing it
    // would let a third process double-hold. Only unlink what we opened.
    const held = fs.fstatSync(descriptor);
    const current = fs.statSync(file);
    if (held.ino !== current.ino) return;
  } catch {
    // The lock file is already gone; nothing to release.
    return;
  } finally {
    try {
      fs.closeSync(descriptor);
    } catch {
      // Best effort.
    }
  }
  try {
    fs.unlinkSync(file);
  } catch {
    // Best effort; stale locks are recovered on the next attempt.
  }
}

function resultObject(status, latest, message) {
  return { status, distribution: "npm", current: pkg.version, latest: latest || "", message };
}

function shouldOnlyNotify(options) {
  return Boolean(options.checkOnly || (options.mode === "notify" && options.background));
}

// npm error codes that retrying cannot fix (404 package deleted/renamed,
// 401/403 auth). Mirrors lib/bootstrap-update-helper.js.
const DETERMINISTIC_NPM_ERROR = /code E(40[01345]|42[89])/;

function isPrereleaseVersion(value) {
  return /^v?\d+\.\d+\.\d+-/.test(String(value || "").trim());
}

// Compares the installed version against the stable latest. A prerelease
// current (a manual @next install) with a stable latest is a legitimate
// upgrade path: report it as available instead of an unactionable
// "cannot compare" that would keep failing on every scheduled check. A
// garbage current version is not a prerelease and still fails loudly.
// Mirrors lib/bootstrap-update-helper.js.
function currentVsLatest(current, latest) {
  const comparison = compareVersions(current, latest);
  if (comparison != null) return { comparison, prerelease: false };
  if (isPrereleaseVersion(current) && stableVersion(latest) != null) {
    return { comparison: -1, prerelease: true };
  }
  return { comparison: null, prerelease: false };
}

function performUpdate(options = {}) {
  const packageRoot = options.packageRoot || path.resolve(__dirname, "..");
  const metadata = options.metadata || readInstallMetadata(packageRoot);
  if (!metadata || !metadata.global || metadata.distribution !== "npm" || !metadata.prefix) {
    if (metadata && metadata.distribution === "pnpm") {
      if (metadata.global) {
        throw new Error(`this installation is managed by pnpm; update it with "pnpm add -g ${pkg.name}@latest"`);
      }
      throw new Error(
        'this pnpm-managed dependency is not a global install; update it with "pnpm update" in the owning project'
      );
    }
    if (metadata && metadata.distribution === "npm-local") {
      throw new Error(
        `automatic update requires a global npm install; run "npm install -g ${pkg.name}" to switch, ` +
          "or update through the channel that owns this installation"
      );
    }
    throw new Error("automatic update is available only for a global npm installation");
  }
  const latest = checkLatest(metadata);
  const { comparison, prerelease } = currentVsLatest(pkg.version, latest);
  if (comparison == null) throw new Error(`cannot compare versions ${pkg.version} and ${latest}`);
  if (comparison >= 0) return resultObject("current", latest, `GitCode CLI ${pkg.version} is current.`);
  if (shouldOnlyNotify(options)) {
    const note = prerelease ? ", a prerelease" : "";
    return resultObject("available", latest, `GitCode CLI ${latest} is available (current ${pkg.version}${note}); run "gitcode update" to install it.`);
  }

  try {
    installExact(metadata, latest);
    healthCheck(metadata, latest);
    return resultObject("updated", latest, `Updated GitCode CLI ${pkg.version} -> ${latest}.`);
  } catch (error) {
    try {
      installExact(metadata, pkg.version);
      healthCheck(metadata, pkg.version);
      const restored = new Error(`${error.message}; restored ${pkg.version}`);
      // Explicit marker: a rollback failure whose text happens to contain
      // "restored" must not be mistaken for a successful restore.
      restored.restored = true;
      throw restored;
    } catch (rollbackError) {
      if (rollbackError.restored) throw rollbackError;
      throw new Error(
        `${error.message}; rollback failed: ${rollbackError.message}; ` +
          `recover manually with "npm install -g ${PACKAGE}@${pkg.version}"`
      );
    }
  }
}

function runUpdate(options = {}) {
  const stateFile = options.stateFile || updateStatePath();
  const lockFile = `${stateFile}.lock`;
  const descriptor = acquireLock(lockFile);
  if (descriptor == null) return resultObject("busy", "", "Another GitCode CLI update is already running.");
  try {
    // Re-check under the lock: two wrappers spawned milliseconds apart both
    // pass shouldSchedule, and the loser must not repeat the check the
    // winner just finished. Explicit updates are never TTL-gated.
    if (options.background) {
      const pending = readJSON(stateFile);
      if (pending.permanentError || (pending.nextCheck && Number(pending.nextCheck) > Date.now())) {
        return resultObject("cached", "", "Update check is not due yet.");
      }
    }
    const mode = options.mode || updateMode();
    const result = performUpdate({ ...options, mode });
    const now = Date.now();
    const state = readJSON(stateFile);
    state.lastChecked = new Date(now).toISOString();
    state.nextCheck = now + TTL_MS;
    // A recovered environment clears the failure bookkeeping so the next
    // failure starts the backoff from scratch.
    state.failureStreak = 0;
    delete state.lastErrorFingerprint;
    state.permanentError = false;
    // A "current" result carries no action for the user: queueing it would
    // print "X is current." once a day in the default notify mode. Stale
    // "available" summaries are cleared so users never see outdated notices.
    if (result.status === "current") delete state.summary;
    else state.summary = { message: result.message, shown: !options.background };
    try {
      writeJSON(stateFile, state);
    } catch (writeError) {
      // The update itself succeeded: a failed state write must not turn it
      // into a reported failure or feed the backoff accounting.
      appendLog(`package=${PACKAGE} channel=npm status=state-write-failed detail="${summarizeError(writeError)}"`, process.env, options.logFile);
    }
    appendLog(`package=${PACKAGE} channel=npm status=${result.status} current=${result.current} latest=${result.latest || "none"}`, process.env, options.logFile);
    return result;
  } catch (error) {
    const now = Date.now();
    const state = readJSON(stateFile);
    state.lastChecked = new Date(now).toISOString();
    const streak = (Number(state.failureStreak) || 0) + 1;
    state.failureStreak = streak;
    state.nextCheck = now + failureBackoffMs(streak);
    const fingerprint = errorFingerprint(error);
    const permanent = permanentUpdateError(error);
    // Sticky: a transient failure must not lift the pause a permanent one
    // recorded — only a successful update proves the environment repaired.
    state.permanentError = Boolean(state.permanentError) || permanent;
    if (state.lastErrorFingerprint !== fingerprint) {
      let message = `Automatic update failed: ${summarizeError(error)}.`;
      if (permanent) {
        message += /npm-bootstrap install manifest/.test(summarizeError(error))
          ? " Background checks are paused: rerun the npm bootstrap install (npx --yes --package=<coordinate>@latest gitcode install) to repair the install manifest."
          : ' Background checks are paused: reinstall Node.js, then run "gitcode update" to resume background checks.';
      } else if (state.permanentError) {
        // The pause is sticky from an earlier unrepaired failure; do not
        // attribute it to the transient error at hand.
        message += ' Background checks remain paused from an earlier unrepaired failure; repair it (rerun the bootstrap install or reinstall Node.js), then run "gitcode update".';
      }
      state.summary = { message, shown: !options.background };
      state.lastErrorFingerprint = fingerprint;
      // Foreground failures surface the full message (including any repair
      // guidance) through the thrown error; attach it for the CLI entry.
      error.summaryMessage = message;
    } else if (!options.background && state.summary) {
      // The same failure just ran in the foreground and is being printed:
      // mark it shown (no next-launch replay) and reuse the composed
      // message so the display stays consistent.
      state.summary.shown = true;
      error.summaryMessage = state.summary.message;
    }
    try {
      writeJSON(stateFile, state);
    } catch (writeError) {
      // Preserve the original failure; the state loss is logged best-effort.
      appendLog(`package=${PACKAGE} channel=npm status=state-write-failed detail="${summarizeError(writeError)}"`, process.env, options.logFile);
    }
    appendLog(`package=${PACKAGE} channel=npm status=error detail="${summarizeError(error)}" streak=${streak}${state.permanentError ? " permanent" : ""}`, process.env, options.logFile);
    throw error;
  } finally {
    releaseLock(lockFile, descriptor);
  }
}

function shouldSchedule(args = [], env = process.env, now = Date.now()) {
  if (disabledForInvocation(args, env)) return false;
  const state = readJSON(updateStatePath(env));
  // A permanent failure stops background scheduling until the user repairs
  // the install; explicit "gitcode update" still works.
  if (state.permanentError) return false;
  // A garbage nextCheck (NaN) counts as due: the next write heals it.
  const next = Number(state.nextCheck);
  return !state.nextCheck || !Number.isFinite(next) || next <= now;
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

function showFirstRunNotice(args = [], stderr = process.stderr, env = process.env) {
  if (disabledForInvocation(args, env)) return;
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
  currentVsLatest,
  disabledForInvocation,
  errorFingerprint,
  exactInstallArgs,
  failureBackoffMs,
  globalWrapper,
  npmCommand,
  performUpdate,
  permanentUpdateError,
  readJSON,
  releaseLock,
  runUpdate,
  shouldSchedule,
  shouldOnlyNotify,
  showFirstRunNotice,
  showPendingSummary,
  stableVersion,
  summarizeError,
  updateMode,
  updaterEnvironment,
  updateStatePath,
  withNpmIsolation,
  writeJSON,
};
