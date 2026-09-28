"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const pkgName = require("../package.json").name;
const { spawnSync } = require("child_process");
const {
  acquireLock,
  checkLatest,
  compareVersions,
  currentVsLatest,
  disabledForInvocation,
  errorFingerprint,
  exactInstallArgs,
  failureBackoffMs,
  globalWrapper,
  npmCommand,
  permanentUpdateError,
  releaseLock,
  runUpdate,
  shouldSchedule,
  shouldOnlyNotify,
  stableVersion,
  summarizeError,
  TTL_MS,
  updateMode,
  updaterEnvironment,
  updateStatePath,
  withNpmIsolation,
  writeJSON,
} = require("../lib/update");
const { parseArgs } = require("../lib/update-helper");

function stateEnv() {
  return { GC_STATE_DIR: fs.mkdtempSync(path.join(os.tmpdir(), "gc-update-state-")) };
}

test("accepts only stable semantic versions and compares without downgrading", () => {
  assert.deepStrictEqual(stableVersion("v1.2.3"), [1, 2, 3]);
  assert.strictEqual(stableVersion("1.2.3-rc.1"), null);
  assert.strictEqual(compareVersions("1.2.3", "1.3.0"), -1);
  assert.strictEqual(compareVersions("2.0.0", "1.9.9"), 1);
  assert.strictEqual(compareVersions("1.2.3", "1.2.3"), 0);
});

test("notify mode affects background checks but not explicit updates", () => {
  assert.strictEqual(shouldOnlyNotify({ mode: "notify", background: true }), true);
  assert.strictEqual(shouldOnlyNotify({ mode: "notify", background: false }), false);
  assert.strictEqual(shouldOnlyNotify({ mode: "auto", checkOnly: true }), true);
});

test("exact updates and rollbacks stay inside the recorded npm prefix", () => {
  const args = exactInstallArgs({ prefix: "/isolated/prefix" }, "1.2.3");
  assert.deepStrictEqual(args.slice(args.indexOf("--prefix"), args.indexOf("--prefix") + 2), [
    "--prefix", "/isolated/prefix",
  ]);
  assert.ok(args.includes("--ignore-scripts"));
  assert.ok(args.includes(`${pkgName}@1.2.3`), `exact install must target the installed coordinate ${pkgName}`);
});

test("exact install arguments target the installed coordinate, not a sibling package", () => {
  const args = exactInstallArgs({ prefix: "/isolated/prefix" }, "1.2.3");
  assert.ok(args.includes(`${pkgName}@1.2.3`));
  assert.ok(!args.some((a) => a !== `${pkgName}@1.2.3` && /@gitcode-cli\/cli@|@atomgit-cli\/cli@|^atomgit-cli@|^gitcode-cli@/.test(a)));
});

test("npm calls isolate config and override scoped registries before exec arguments", () => {
  const args = withNpmIsolation(["exec", "--yes", "--", "gitcode", "install"], "user.npmrc", "global.npmrc");
  const separator = args.indexOf("--");
  assert.ok(args.slice(0, separator).includes("--userconfig=user.npmrc"));
  assert.ok(args.slice(0, separator).includes("--globalconfig=global.npmrc"));
  if (pkgName.startsWith("@")) {
    assert.ok(args.slice(0, separator).includes(`--${pkgName.split("/")[0]}:registry=https://registry.npmjs.org`));
  } else {
    assert.ok(!args.some((a) => a.includes(":registry=")), "unscoped package names must not carry a scope-registry flag");
  }
  assert.ok(args.slice(0, separator).includes("--registry=https://registry.npmjs.org"));
  assert.deepStrictEqual(args.slice(separator + 1), ["gitcode", "install"]);
});

(pkgName.startsWith("@") ? test : test.skip)("npm isolation overrides conflicting user and project scoped registries", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-npm-registry-isolation-"));
  const home = path.join(root, "home");
  const project = path.join(root, "project");
  fs.mkdirSync(home);
  fs.mkdirSync(project);
  const conflict = `${pkgName.split("/")[0]}:registry=https://untrusted.invalid\n`;
  fs.writeFileSync(path.join(home, ".npmrc"), conflict);
  fs.writeFileSync(path.join(project, ".npmrc"), conflict);
  const userConfig = path.join(root, "isolated-user.npmrc");
  const globalConfig = path.join(root, "isolated-global.npmrc");
  fs.writeFileSync(userConfig, "");
  fs.writeFileSync(globalConfig, "");
  const invocation = npmCommand({ npm: process.env.npm_execpath || "" });
  const result = spawnSync(
    invocation.command,
    [...invocation.prefix, ...withNpmIsolation(["config", "get", `${pkgName.split("/")[0]}:registry`], userConfig, globalConfig)],
    {
      cwd: project,
      encoding: "utf8",
      windowsHide: true,
      env: updaterEnvironment({ ...process.env, HOME: home, USERPROFILE: home }),
    }
  );
  assert.strictEqual(result.status, 0, result.stderr);
  assert.strictEqual(result.stdout.trim(), "https://registry.npmjs.org");
});

test("npm isolation overrides a conflicting default registry for every coordinate", () => {
  // Unscoped names have no scope-registry key; the global --registry flag is
  // their only defense against a hijacked user/project npmrc.
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-npm-registry-isolation-"));
  const home = path.join(root, "home");
  const project = path.join(root, "project");
  fs.mkdirSync(home);
  fs.mkdirSync(project);
  const conflict = "registry=https://untrusted.invalid\n";
  fs.writeFileSync(path.join(home, ".npmrc"), conflict);
  fs.writeFileSync(path.join(project, ".npmrc"), conflict);
  const userConfig = path.join(root, "isolated-user.npmrc");
  const globalConfig = path.join(root, "isolated-global.npmrc");
  fs.writeFileSync(userConfig, "");
  fs.writeFileSync(globalConfig, "");
  const invocation = npmCommand({ npm: process.env.npm_execpath || "" });
  const result = spawnSync(
    invocation.command,
    [...invocation.prefix, ...withNpmIsolation(["config", "get", "registry"], userConfig, globalConfig)],
    {
      cwd: project,
      encoding: "utf8",
      windowsHide: true,
      env: updaterEnvironment({ ...process.env, HOME: home, USERPROFILE: home }),
    }
  );
  assert.strictEqual(result.status, 0, result.stderr);
  assert.strictEqual(result.stdout.trim().replace(/\/$/, ""), "https://registry.npmjs.org");
});

test("latest-version checks query the installed coordinate on the official registry", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-npm-view-"));
  const argvFile = path.join(root, "argv.json");
  const stub = path.join(root, "npm-cli.js");
  fs.writeFileSync(stub, [
    "const fs = require('fs');",
    `fs.writeFileSync(${JSON.stringify(argvFile)}, JSON.stringify(process.argv.slice(2)));`,
    'process.stdout.write(\'"1.2.3"\\n\');',
    "",
  ].join("\n"));
  checkLatest({ npm: stub });
  const argv = JSON.parse(fs.readFileSync(argvFile, "utf8"));
  assert.ok(argv.includes("view"), `view command expected: ${JSON.stringify(argv)}`);
  assert.ok(argv.includes(pkgName), `registry query must target the installed coordinate ${pkgName}: ${JSON.stringify(argv)}`);
  assert.ok(argv.includes("dist-tags.latest"), `dist-tags.latest expected: ${JSON.stringify(argv)}`);
  assert.ok(argv.includes("--json"));
  assert.ok(argv.includes("--registry=https://registry.npmjs.org"));
  assert.strictEqual(checkLatest({ npm: stub }), "1.2.3");
});

test("global updater falls back when the recorded npm runtime is stale", () => {
  const command = npmCommand({ npm: path.join(os.tmpdir(), "missing-npm-cli.js") });
  assert.ok(command.command);
  assert.notDeepStrictEqual(command.prefix, [path.join(os.tmpdir(), "missing-npm-cli.js")]);
});

function flakyNpmStub(dir, failures, version = "1.2.3") {
  const attemptsFile = path.join(dir, "attempts");
  const stub = path.join(dir, "flaky-npm-cli.js");
  fs.writeFileSync(stub, [
    "const fs = require('fs');",
    `const attempts = fs.existsSync(${JSON.stringify(attemptsFile)}) ? Number(fs.readFileSync(${JSON.stringify(attemptsFile)}, "utf8")) : 0;`,
    `fs.writeFileSync(${JSON.stringify(attemptsFile)}, String(attempts + 1));`,
    `if (attempts < ${failures}) { process.stderr.write("network ECONNRESET from stub\\n"); process.exit(1); }`,
    `process.stdout.write(${JSON.stringify(`"${version}"\n`)});`,
    "",
  ].join("\n"));
  return { stub, attemptsFile };
}

test("registry checks retry transient failures before giving up", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-npm-retry-"));
  const { stub, attemptsFile } = flakyNpmStub(dir, 2);
  assert.strictEqual(checkLatest({ npm: stub }), "1.2.3");
  assert.strictEqual(Number(fs.readFileSync(attemptsFile, "utf8")), 3);
});

test("registry checks fail after exhausting the retry budget", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-npm-retry-exhaust-"));
  const { stub, attemptsFile } = flakyNpmStub(dir, 3);
  assert.throws(() => checkLatest({ npm: stub }), /ECONNRESET/);
  assert.strictEqual(Number(fs.readFileSync(attemptsFile, "utf8")), 3);
});

test("deterministic registry errors fail fast without retrying", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-npm-e404-"));
  const attemptsFile = path.join(dir, "attempts");
  const stub = path.join(dir, "e404-npm-cli.js");
  fs.writeFileSync(stub, [
    "const fs = require('fs');",
    `const n = fs.existsSync(${JSON.stringify(attemptsFile)}) ? Number(fs.readFileSync(${JSON.stringify(attemptsFile)}, "utf8")) : 0;`,
    `fs.writeFileSync(${JSON.stringify(attemptsFile)}, String(n + 1));`,
    'process.stderr.write("npm error code E404\\n");',
    "process.exit(1);",
    "",
  ].join("\n"));
  assert.throws(() => checkLatest({ npm: stub }), /E404/);
  assert.strictEqual(Number(fs.readFileSync(attemptsFile, "utf8")), 1, "E404 must not be retried");
});

test("a prerelease current version is an upgrade path, garbage is not", () => {
  assert.deepStrictEqual(currentVsLatest("0.14.1", "0.15.0"), { comparison: -1, prerelease: false });
  assert.deepStrictEqual(currentVsLatest("0.15.0", "0.15.0"), { comparison: 0, prerelease: false });
  // A manual @next install with a stable latest: available, not an error.
  assert.deepStrictEqual(currentVsLatest("0.14.1-rc.1", "0.15.0"), { comparison: -1, prerelease: true });
  assert.deepStrictEqual(currentVsLatest("v0.14.1-rc.1", "0.15.0"), { comparison: -1, prerelease: true });
  // Garbage versions and prerelease latests still fail loudly.
  assert.deepStrictEqual(currentVsLatest("banana", "0.15.0"), { comparison: null, prerelease: false });
  assert.deepStrictEqual(currentVsLatest("0.14.1", "0.15.0-rc.1"), { comparison: null, prerelease: false });
});

test("a failed rollback tells the user how to recover manually", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-runupdate-rollback-fail-"));
  const stateDir = path.join(dir, "state");
  const stub = path.join(dir, "install-fail-npm-cli.js");
  fs.writeFileSync(stub, [
    "const args = process.argv.slice(2);",
    'if (args.includes("view")) { process.stdout.write(\'"9.9.9"\\n\'); process.exit(0); }',
    'process.stderr.write("npm install EACCES from stub\\n"); process.exit(1);',
    "",
  ].join("\n"));
  const stateFile = path.join(stateDir, "update-state.json");
  const previousStateDir = process.env.GC_STATE_DIR;
  process.env.GC_STATE_DIR = stateDir;
  try {
    assert.throws(
      () => runUpdate({
        stateFile,
        mode: "auto",
        metadata: { global: true, distribution: "npm", prefix: dir, npm: stub },
      }),
      (error) => /rollback failed/.test(error.message) && /npm install -g/.test(error.message)
    );
  } finally {
    if (previousStateDir === undefined) delete process.env.GC_STATE_DIR;
    else process.env.GC_STATE_DIR = previousStateDir;
  }
  const state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
  assert.match(state.summary.message, /npm install -g/, "the failure summary must carry the manual recovery hint");
});

test("runUpdate failure summaries carry the real error for the next launch", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-runupdate-fail-"));
  const stateDir = path.join(dir, "state");
  const { stub } = flakyNpmStub(dir, Infinity);
  const stateFile = path.join(stateDir, "update-state.json");
  const previousStateDir = process.env.GC_STATE_DIR;
  process.env.GC_STATE_DIR = stateDir;
  try {
    assert.throws(
      () => runUpdate({
        stateFile,
        metadata: { global: true, distribution: "npm", prefix: dir, npm: stub },
      }),
      /ECONNRESET/
    );
  } finally {
    if (previousStateDir === undefined) delete process.env.GC_STATE_DIR;
    else process.env.GC_STATE_DIR = previousStateDir;
  }
  const state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
  assert.match(state.summary.message, /^Automatic update failed: /);
  assert.match(state.summary.message, /ECONNRESET/);
  assert.ok(!state.summary.message.includes("run gitcode update for details"), "dead-end notice must be gone");
  assert.strictEqual(state.summary.shown, true);
  const log = fs.readFileSync(path.join(stateDir, "update.log"), "utf8");
  assert.match(log, /status=error detail=.*ECONNRESET/);
});

test("summarizeError collapses and bounds error text", () => {
  assert.strictEqual(summarizeError(new Error("boom")), "boom");
  assert.strictEqual(summarizeError(null), "unknown error");
  assert.strictEqual(summarizeError(new Error(" \n\t ")), "unknown error");
  assert.strictEqual(summarizeError({ code: "ECONNRESET" }), "ECONNRESET");
  assert.strictEqual(summarizeError(new Error(`x${"a".repeat(400)}`)).length, 203);
});

test("failureBackoffMs grows exponentially and caps at the daily TTL", () => {
  const hour = 60 * 60 * 1000;
  assert.strictEqual(failureBackoffMs(1), hour);
  assert.strictEqual(failureBackoffMs(2), 2 * hour);
  assert.strictEqual(failureBackoffMs(5), 16 * hour);
  assert.strictEqual(failureBackoffMs(6), TTL_MS);
  assert.strictEqual(failureBackoffMs(100), TTL_MS);
  assert.strictEqual(failureBackoffMs(0), hour);
});

test("permanentUpdateError classifies only retry-unrepairable failures", () => {
  assert.strictEqual(permanentUpdateError(new Error("invalid npm-bootstrap install manifest")), true);
  assert.strictEqual(permanentUpdateError(new Error("npm CLI JavaScript runtime not found; reinstall Node.js or run npm install -g explicitly")), true);
  // A bad dist-tag is maintainer-fixable: retrying must auto-recover.
  assert.strictEqual(permanentUpdateError(new Error("registry latest is not a stable semantic version: 9.9.9-rc.1")), false);
  assert.strictEqual(permanentUpdateError(new Error("network ECONNRESET")), false);
  assert.strictEqual(errorFingerprint(new Error("boom")), errorFingerprint(new Error("boom")));
  assert.notStrictEqual(errorFingerprint(new Error("boom")), errorFingerprint(new Error("other")));
});

test("failed checks back off progressively and deduplicate summaries by fingerprint", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-runupdate-backoff-"));
  const stateDir = path.join(dir, "state");
  const stateFile = path.join(stateDir, "update-state.json");
  const { stub } = flakyNpmStub(dir, 99, "0.0.2");
  const metadata = { global: true, distribution: "npm", prefix: dir, npm: stub };
  const previousStateDir = process.env.GC_STATE_DIR;
  process.env.GC_STATE_DIR = stateDir;
  try {
    const before = Date.now();
    assert.throws(() => runUpdate({ stateFile, background: true, metadata }), /ECONNRESET/);
    let state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.failureStreak, 1);
    const firstDelay = state.nextCheck - before;
    assert.ok(firstDelay >= 55 * 60 * 1000 && firstDelay <= 65 * 60 * 1000, `first backoff ~1h, got ${firstDelay}`);
    assert.strictEqual(state.summary.shown, false);
    assert.ok(state.lastErrorFingerprint, "fingerprint must be recorded");

    // The same failure again: streak grows, the seen summary is untouched.
    assert.throws(() => runUpdate({ stateFile, background: true, metadata }), /ECONNRESET/);
    state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.failureStreak, 2);
    assert.ok(state.nextCheck > Date.now() + 100 * 60 * 1000, "second backoff ~2h");
    state.summary.shown = true;
    writeJSON(stateFile, state);
    assert.throws(() => runUpdate({ stateFile, background: true, metadata }), /ECONNRESET/);
    state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.failureStreak, 3);
    assert.strictEqual(state.summary.shown, true, "an identical fingerprint must not requeue the summary");

    // A different failure requeues with the new message.
    const angry = path.join(dir, "angry-npm-cli.js");
    fs.writeFileSync(angry, "process.stderr.write('registry BOOM from angry stub\\n'); process.exit(1);");
    assert.throws(
      () => runUpdate({ stateFile, background: true, metadata: { ...metadata, npm: angry } }),
      /BOOM/
    );
    state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.summary.shown, false);
    assert.match(state.summary.message, /BOOM/);

    // A successful check clears the failure bookkeeping.
    const healthyDir = path.join(dir, "healthy");
    fs.mkdirSync(healthyDir, { recursive: true });
    const { stub: healthy } = flakyNpmStub(healthyDir, 0, "0.0.1");
    runUpdate({ stateFile, background: true, metadata: { ...metadata, npm: healthy } });
    state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.failureStreak, 0);
    assert.strictEqual(state.lastErrorFingerprint, undefined);
    assert.strictEqual(state.permanentError, false);
    assert.ok(state.nextCheck > Date.now() + 23 * 60 * 60 * 1000, "success restores the daily TTL");
  } finally {
    if (previousStateDir === undefined) delete process.env.GC_STATE_DIR;
    else process.env.GC_STATE_DIR = previousStateDir;
  }
});

test("a current check queues no next-launch summary and clears stale notices", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-runupdate-current-"));
  const stateDir = path.join(dir, "state");
  const stateFile = path.join(stateDir, "update-state.json");
  writeJSON(stateFile, { summary: { message: "GitCode CLI 9.9.9 is available.", shown: false } });
  const { stub } = flakyNpmStub(dir, 0, "0.0.1");
  const previousStateDir = process.env.GC_STATE_DIR;
  process.env.GC_STATE_DIR = stateDir;
  try {
    const result = runUpdate({
      stateFile,
      background: true,
      metadata: { global: true, distribution: "npm", prefix: dir, npm: stub },
    });
    assert.strictEqual(result.status, "current");
  } finally {
    if (previousStateDir === undefined) delete process.env.GC_STATE_DIR;
    else process.env.GC_STATE_DIR = previousStateDir;
  }
  const state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
  assert.strictEqual(state.summary, undefined, "current results must not queue a daily notice");
  assert.ok(state.nextCheck > Date.now());
});

test("an available update still queues the next-launch summary", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-runupdate-available-"));
  const stateDir = path.join(dir, "state");
  const stateFile = path.join(stateDir, "update-state.json");
  const { stub } = flakyNpmStub(dir, 0, "9.9.9");
  const previousStateDir = process.env.GC_STATE_DIR;
  process.env.GC_STATE_DIR = stateDir;
  try {
    const result = runUpdate({
      stateFile,
      background: true,
      mode: "notify",
      metadata: { global: true, distribution: "npm", prefix: dir, npm: stub },
    });
    assert.strictEqual(result.status, "available");
  } finally {
    if (previousStateDir === undefined) delete process.env.GC_STATE_DIR;
    else process.env.GC_STATE_DIR = previousStateDir;
  }
  const state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
  assert.match(state.summary.message, /9\.9\.9 is available/);
  assert.strictEqual(state.summary.shown, false);
});

test("an unwritable update.log never turns a successful check into a failure", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-runupdate-log-fail-"));
  const stateDir = path.join(dir, "state");
  fs.mkdirSync(stateDir);
  fs.mkdirSync(path.join(stateDir, "update.log"), { recursive: false });
  const stateFile = path.join(stateDir, "update-state.json");
  const { stub } = flakyNpmStub(dir, 0, "0.0.1");
  const previousStateDir = process.env.GC_STATE_DIR;
  process.env.GC_STATE_DIR = stateDir;
  try {
    const result = runUpdate({
      stateFile,
      metadata: { global: true, distribution: "npm", prefix: dir, npm: stub },
    });
    assert.strictEqual(result.status, "current");
  } finally {
    if (previousStateDir === undefined) delete process.env.GC_STATE_DIR;
    else process.env.GC_STATE_DIR = previousStateDir;
  }
  const state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
  assert.strictEqual(state.summary, undefined, "a successful check must not be rewritten as a failure");
  assert.ok(state.nextCheck > Date.now());
});

test("global health checks execute the wrapper inside the recorded prefix", () => {
  const wrapper = globalWrapper({ prefix: "/isolated/prefix" });
  const expected = process.platform === "win32"
    ? path.join("/isolated/prefix", "node_modules", pkgName, "bin", "gc.js")
    : path.join("/isolated/prefix", "lib", "node_modules", pkgName, "bin", "gc.js");
  assert.strictEqual(wrapper, expected);
});

test("disables updates for explicit opt-out, CI, non-interactive, and off mode", () => {
  assert.strictEqual(disabledForInvocation([], { GC_NO_UPDATE_CHECK: "1" }), true);
  assert.strictEqual(disabledForInvocation([], { CI: "true" }), true);
  assert.strictEqual(disabledForInvocation([], { CI: "1" }), true);
  assert.strictEqual(disabledForInvocation([], { CI: " YES " }), true);
  assert.strictEqual(disabledForInvocation(["--no-interactive"], {}), true);
  assert.strictEqual(disabledForInvocation(["--no-update-check"], {}), true);
  assert.strictEqual(disabledForInvocation([], { GC_UPDATE_MODE: "off" }), true);
  assert.strictEqual(disabledForInvocation([], { GC_UPDATE_MODE: "notify" }), false);
});

test("mode defaults to notify and honors the environment", () => {
  assert.strictEqual(updateMode({ GC_CONFIG_DIR: fs.mkdtempSync(path.join(os.tmpdir(), "gc-config-")) }), "notify");
  assert.strictEqual(updateMode({ GC_UPDATE_MODE: "notify" }), "notify");
});

test("TTL prevents a background check until it expires", () => {
  const env = stateEnv();
  assert.strictEqual(shouldSchedule([], env, 100), true);
  writeJSON(updateStatePath(env), { nextCheck: 200 });
  assert.strictEqual(shouldSchedule([], env, 100), false);
  assert.strictEqual(shouldSchedule([], env, 201), true);
});

test("cross-process lock permits only one owner", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-update-lock-"));
  const lock = path.join(dir, "update.lock");
  const first = acquireLock(lock);
  assert.notStrictEqual(first, null);
  assert.strictEqual(acquireLock(lock), null);
  releaseLock(lock, first);
  const second = acquireLock(lock);
  assert.notStrictEqual(second, null);
  releaseLock(lock, second);
});

test("writeJSON cleans up its temp file when the rename fails", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-writejson-fail-"));
  const target = path.join(root, "update-state.json");
  const originalRenameSync = fs.renameSync;
  try {
    fs.renameSync = () => {
      const error = new Error("EBUSY: resource busy or locked");
      error.code = "EBUSY";
      throw error;
    };
    assert.throws(() => writeJSON(target, { nextCheck: 1 }), /EBUSY/);
  } finally {
    fs.renameSync = originalRenameSync;
  }
  // Neither the target nor a stranded update-state.json.tmp-* remains.
  assert.deepStrictEqual(fs.readdirSync(root), []);
});

test("update state paths are scoped per package and channel unless overridden", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-state-path-"));
  const env = { XDG_STATE_HOME: root, LOCALAPPDATA: path.join(root, "la") };
  const base = process.platform === "win32"
    ? path.join(root, "la", "gitcode-cli")
    : path.join(root, "gitcode-cli");
  assert.strictEqual(
    updateStatePath(env),
    path.join(base, pkgName, "npm", "update-state.json")
  );
  // An explicit GC_STATE_DIR keeps the legacy flat shared file.
  assert.strictEqual(
    updateStatePath({ GC_STATE_DIR: root }),
    path.join(root, "update-state.json")
  );
});

test("update helper parser accepts check/json/background only", () => {
  assert.deepStrictEqual(parseArgs(["--check", "--json"]), {
    background: false,
    checkOnly: true,
    json: true,
    help: false,
  });
  assert.throws(() => parseArgs(["--channel", "next"]), /unknown update argument/);
});

test("updater child environment strips GitCode and npm credentials", () => {
  const env = updaterEnvironment({
    PATH: "/bin",
    GC_TOKEN: "gitcode-secret",
    GITCODE_TOKEN: "legacy-secret",
    NPM_TOKEN: "npm-secret",
    NODE_AUTH_TOKEN: "node-secret",
    GITHUB_TOKEN: "github-secret",
    AWS_SECRET_ACCESS_KEY: "cloud-secret",
    npm_config_registry: "https://untrusted.example",
    SAFE_VALUE: "not-required",
  });
  assert.strictEqual(env.PATH, "/bin");
  assert.strictEqual(env.GC_NO_UPDATE_CHECK, "1");
  for (const key of [
    "GC_TOKEN", "GITCODE_TOKEN", "NPM_TOKEN", "NODE_AUTH_TOKEN", "GITHUB_TOKEN",
    "AWS_SECRET_ACCESS_KEY", "npm_config_registry", "SAFE_VALUE",
  ]) {
    assert.strictEqual(env[key], undefined);
  }
});
