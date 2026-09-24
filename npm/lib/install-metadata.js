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
  const candidates = commandCandidates("gitcode", env, isWin);
  const selected = candidates[0] || "";
  const pathAPI = isWin ? path.win32 : path.posix;
  const selectedDir = selected ? normalizePath(pathAPI.dirname(selected), isWin) : "";
  return {
    expectedDir: expectedGlobalBin(prefix, isWin),
    candidates,
    selected,
    shadowed: Boolean(selected && selectedDir !== expectedDir),
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
  fs.writeFileSync(temp, `${JSON.stringify(data, null, 2)}\n`, { mode: 0o600, flag: "wx" });
  try {
    fs.renameSync(temp, target);
  } catch (error) {
    if (!["EEXIST", "EPERM"].includes(error.code)) throw error;
    fs.unlinkSync(target);
    fs.renameSync(temp, target);
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

function isPnpmEnvironment(env, packageRoot, isWin) {
  if (/^pnpm\//.test(String((env || {}).npm_config_user_agent || ""))) return true;
  // Fallback for direct invocation outside a pnpm shim: pnpm's global layout.
  // Windows paths keep backslashes through normalizePath, so unify them.
  const normalized = normalizePath(packageRoot, isWin).replace(/\\/g, "/");
  return /\/pnpm\/global\//.test(normalized);
}

function discoverGlobalInstall(packageRoot, options = {}) {
  const env = options.env || process.env;
  const isWin = (options.platform || process.platform) === "win32";
  if (isPnpmEnvironment(env, packageRoot, isWin)) {
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
  const actualRoot = normalizePath(pathAPI.resolve(packageRoot, "..", ".."), isWin);
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
