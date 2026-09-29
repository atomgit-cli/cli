// Shared installation metadata and PATH diagnostics for the npm wrapper.

"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const crypto = require("crypto");
const { spawnSync } = require("child_process");

const METADATA_FILE = ".gitcode-install.json";

function normalizePath(value, isWin = process.platform === "win32") {
  const pathAPI = isWin ? path.win32 : path.posix;
  let normalized = pathAPI.resolve(value || "").replace(/[\\/]+$/, "");
  if (isWin) normalized = normalized.toLowerCase();
  return normalized;
}

function pathEntries(env = process.env, isWin = process.platform === "win32") {
  const delimiter = isWin ? ";" : ":";
  return (env.PATH || env.Path || "")
    .split(delimiter)
    .map((entry) => entry.trim().replace(/^"|"$/g, ""))
    .filter(Boolean);
}

function commandCandidates(name, env = process.env, isWin = process.platform === "win32") {
  const extensions = isWin ? [".exe", ".com", ".bat", ".cmd", ".ps1", ""] : [""];
  const candidates = [];
  const seen = new Set();
  for (const dir of pathEntries(env, isWin)) {
    for (const extension of extensions) {
      const candidate = path.join(dir, `${name}${extension}`);
      const key = normalizePath(candidate, isWin);
      if (seen.has(key)) continue;
      seen.add(key);
      try {
        if (fs.statSync(candidate).isFile()) candidates.push(candidate);
      } catch {
        // Missing and inaccessible PATH entries are diagnostics, not failures.
      }
    }
  }
  return candidates;
}

function expectedGlobalBin(prefix, isWin = process.platform === "win32") {
  return isWin ? prefix : path.join(prefix, "bin");
}

function pathConflict(prefix, env = process.env, isWin = process.platform === "win32") {
  const expectedDir = normalizePath(expectedGlobalBin(prefix, isWin), isWin);
  const pathAPI = isWin ? path.win32 : path.posix;
  // Both command names ship in the npm bin dir; either one being shadowed by
  // an earlier PATH entry (deb/brew/pip installs provide only "gc", other
  // tools may reserve "gitcode") hides the npm install from the user.
  const names = {};
  for (const name of ["gitcode", "gc"]) {
    const candidates = commandCandidates(name, env, isWin);
    const selected = candidates[0] || "";
    const selectedDir = selected ? normalizePath(pathAPI.dirname(selected), isWin) : "";
    names[name] = {
      candidates,
      selected,
      shadowed: Boolean(selected && selectedDir !== expectedDir),
    };
  }
  return {
    expectedDir: expectedGlobalBin(prefix, isWin),
    names,
    shadowed: names.gitcode.shadowed || names.gc.shadowed,
  };
}

function writeInstallMetadata(packageRoot, values) {
  const target = path.join(packageRoot, METADATA_FILE);
  const temp = `${target}.tmp-${process.pid}-${crypto.randomBytes(8).toString("hex")}`;
  const data = { schema: 1, ...values };
  try {
    const stat = fs.lstatSync(target);
    if (!stat.isFile() || stat.isSymbolicLink()) {
      throw new Error(`refusing non-regular install metadata target: ${target}`);
    }
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  // 0644: the bootstrap manifest lands in shared bin directories (e.g.
  // /usr/local/bin), where a root install with 0600 would hide it from
  // every other user and silently disable their update notifications.
  // The content carries no secrets (schema, coordinate, version, paths).
  fs.writeFileSync(temp, `${JSON.stringify(data, null, 2)}\n`, { mode: 0o644, flag: "wx" });
  try {
    try {
      fs.renameSync(temp, target);
    } catch (error) {
      if (!["EEXIST", "EPERM"].includes(error.code)) throw error;
      fs.unlinkSync(target);
      fs.renameSync(temp, target);
    }
  } catch (error) {
    // Never strand the temp file: a .gitcode-install.json.tmp-* residue in
    // the bin directory used to match no leftover prefix on either side
    // (JS sweep and doctor leftovers) and survived forever.
    try {
      fs.unlinkSync(temp);
    } catch {
      // Preserve the original error.
    }
    throw error;
  }
  return target;
}

function readInstallMetadata(packageRoot) {
  try {
    return JSON.parse(fs.readFileSync(path.join(packageRoot, METADATA_FILE), "utf8"));
  } catch {
    return null;
  }
}

function npmInvocation(execPath = process.execPath, env = process.env, platform = process.platform) {
  const candidates = [
    env.npm_execpath,
    path.join(path.dirname(execPath), "node_modules", "npm", "bin", "npm-cli.js"),
    path.resolve(path.dirname(execPath), "..", "lib", "node_modules", "npm", "bin", "npm-cli.js"),
  ].filter(Boolean);
  for (const candidate of candidates) {
    if (fs.existsSync(candidate) && /\.m?js$/i.test(candidate)) {
      return { command: execPath, prefix: [candidate], metadataPath: candidate };
    }
  }
  const command = platform === "win32" ? "npm.cmd" : "npm";
  return { command, prefix: [], metadataPath: command, shell: platform === "win32" };
}

function isPnpmEnvironment(env, packageRoot, isWin, options = {}) {
  const ua = /^pnpm\//.test(String((env || {}).npm_config_user_agent || ""));
  // Direct invocation outside a pnpm shim falls back to pnpm's global
  // layout. Windows paths keep backslashes through normalizePath, so unify
  // them. The realpath fallback catches custom PNPM_HOME naming: the store
  // layout (/<global-dir>/<ver>/.pnpm/<pkg>@<v>/node_modules/...) only
  // appears after resolution. Keep the marker set in sync with
  // pnpmChannelSymlinkError in install.js.
  const normalized = normalizePath(packageRoot, isWin).replace(/\\/g, "/");
  if (/\/pnpm\/global\//.test(normalized)) return true;
  if (hasPnpmStoreLayout(packageRoot)) return true;
  if (!ua) return false;
  // The UA says pnpm but the package is not in pnpm's layout: an npm-global
  // install invoked from inside a pnpm script environment would otherwise be
  // misrecorded as pnpm (empty prefix, updates refused). Confirm against
  // `npm root -g` before trusting the UA.
  const runner = options.runner || spawnSync;
  const invoke = options.npm || npmInvocation(options.execPath, env, isWin ? "win32" : "linux");
  let result;
  try {
    result = runner(invoke.command, [...invoke.prefix, "root", "-g"], {
      encoding: "utf8",
      timeout: 5000,
      windowsHide: true,
      shell: Boolean(invoke.shell),
    });
  } catch {
    return true; // cannot confirm; keep the UA signal
  }
  if (!result || result.status !== 0) return true;
  const globalRoot = normalizePath(String(result.stdout || "").trim(), isWin).replace(/\\/g, "/");
  if (!globalRoot) return true;
  return !(normalized === globalRoot || normalized.startsWith(`${globalRoot}/`));
}

// True when the resolved package path sits inside pnpm's store layout. A
// `<...>/global/<ver>/.pnpm/` segment means the pnpm *global* store; a bare
// `/.pnpm/` without the global segment is a project-level install (still
// pnpm-managed, but the caller's global field stays false).
function hasPnpmStoreLayout(packageRoot) {
  let resolved = "";
  try {
    resolved = fs.realpathSync(packageRoot);
  } catch {
    return false;
  }
  const normalized = String(resolved).split(path.sep).join("/").replace(/\\/g, "/").toLowerCase();
  return /\/global\/[^/]+\/\.pnpm\//.test(normalized) || /\/\.pnpm\/[^/]+@[^/]+\/node_modules\//.test(normalized);
}

function discoverGlobalInstall(packageRoot, options = {}) {
  const env = options.env || process.env;
  const isWin = (options.platform || process.platform) === "win32";
  if (isPnpmEnvironment(env, packageRoot, isWin, options)) {
    // pnpm manages its own global layout; the npm-based discovery below
    // (root -g comparison) would misclassify it, and an npm-channel update
    // would install a parallel npm copy.
    const layoutGlobal = /\/pnpm\/global\//.test(normalizePath(packageRoot, isWin).replace(/\\/g, "/"));
    return {
      schema: 1,
      distribution: "pnpm",
      global: layoutGlobal || String(env.npm_config_global) === "true",
      version: options.version || "",
      prefix: "",
      npm: "",
    };
  }
  const invoke = options.npm || npmInvocation(options.execPath, env, options.platform);
  const runner = options.runner || spawnSync;
  const run = (args) =>
    runner(invoke.command, [...invoke.prefix, ...args], {
      encoding: "utf8",
      timeout: 5000,
      windowsHide: true,
      shell: Boolean(invoke.shell),
    });
  const root = run(["root", "-g"]);
  const prefix = run(["prefix", "-g"]);
  if (root.status !== 0 || prefix.status !== 0) return null;
  const pathAPI = isWin ? path.win32 : path.posix;
  const expectedRoot = normalizePath(root.stdout.trim(), isWin);
  // The container that must equal `npm root -g` sits one level above the
  // package for unscoped names (atomgit-cli) and two levels above for scoped
  // ones (@gitcode-cli/cli). Resolving a fixed two levels misclassifies the
  // recommended unscoped coordinate as npm-local after its first self-update
  // (the updater installs with --ignore-scripts, wiping recorded metadata).
  const resolvedPackage = pathAPI.resolve(packageRoot);
  const container = pathAPI.dirname(resolvedPackage);
  const actualRoot = normalizePath(
    pathAPI.basename(container).startsWith("@") ? pathAPI.dirname(container) : container,
    isWin
  );
  return {
    schema: 1,
    distribution: expectedRoot === actualRoot ? "npm" : "npm-local",
    global: expectedRoot === actualRoot,
    version: options.version || "",
    prefix: prefix.stdout.trim(),
    npm: invoke.metadataPath,
  };
}

function ensureInstallMetadata(packageRoot, version, options = {}) {
  const existing = readInstallMetadata(packageRoot);
  if (existing) return existing;
  const discovered = discoverGlobalInstall(packageRoot, { ...options, version });
  if (!discovered) return null;
  try {
    writeInstallMetadata(packageRoot, discovered);
  } catch {
    // Discovery still applies to this process when the package is read-only.
  }
  return discovered;
}

function stateDir(env = process.env, platform = process.platform) {
  if (env.GC_STATE_DIR) return env.GC_STATE_DIR;
  if (platform === "win32") {
    return path.join(env.LOCALAPPDATA || path.join(os.homedir(), "AppData", "Local"), "gitcode-cli");
  }
  return path.join(env.XDG_STATE_HOME || path.join(os.homedir(), ".local", "state"), "gitcode-cli");
}

module.exports = {
  METADATA_FILE,
  commandCandidates,
  discoverGlobalInstall,
  ensureInstallMetadata,
  expectedGlobalBin,
  isPnpmEnvironment,
  normalizePath,
  npmInvocation,
  pathConflict,
  pathEntries,
  readInstallMetadata,
  stateDir,
  writeInstallMetadata,
};
