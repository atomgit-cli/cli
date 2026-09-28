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
  // A 60s fetch timeout keeps npm's own fetch-retries inside the overall
  // install budget.
  assert.ok(args.includes("--fetch-timeout=60000"));
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

test("a foreground failure surfaces the full summary and never re-queues it", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-runupdate-foreground-"));
  const stateDir = path.join(dir, "state");
  const stateFile = path.join(stateDir, "update-state.json");
  const { stub } = flakyNpmStub(dir, 99, "0.0.2");
  const metadata = { global: true, distribution: "npm", prefix: dir, npm: stub };
  const previousStateDir = process.env.GC_STATE_DIR;
  process.env.GC_STATE_DIR = stateDir;
  let caught;
  try {
    try {
      runUpdate({ stateFile, metadata });
    } catch (error) {
      caught = error;
    }
    assert.ok(caught, "the update must throw");
    // The thrown error carries the composed summary (repair guidance
    // included) so the CLI entry can print the full message.
    assert.match(caught.summaryMessage, /^Automatic update failed: /);
    let state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.summary.shown, true, "a foreground failure is displayed now, not re-queued");

    // The same failure again in the foreground must not reset shown.
    try {
      runUpdate({ stateFile, metadata });
    } catch (error) {
      caught = error;
    }
    state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.summary.shown, true, "an identical fingerprint must stay shown");
    assert.match(caught.summaryMessage, /^Automatic update failed: /);

    // A transient failure must not lift a recorded permanent pause.
    writeJSON(stateFile, {
      ...state,
      permanentError: true,
      lastErrorFingerprint: "old-fingerprint",
      summary: { message: "Automatic update failed: earlier.", shown: true },
    });
    try {
      runUpdate({ stateFile, metadata });
    } catch {
      // expected
    }
    state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.permanentError, true, "the pause is sticky until a success clears it");
    assert.match(state.summary.message, /remain paused from an earlier unrepaired failure/);
  } finally {
    if (previousStateDir === undefined) delete process.env.GC_STATE_DIR;
    else process.env.GC_STATE_DIR = previousStateDir;
  }
});

test("a failed state write never turns a successful update into a failure", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-runupdate-statefail-"));
  const stateDir = path.join(dir, "state");
  fs.mkdirSync(stateDir, { recursive: true });
  const { stub } = flakyNpmStub(dir, 0, "0.0.1");
  // Make the state file unwritable-by-rename: a directory at the target
  // path makes writeJSON's rename fail with EISDIR/ENOTEMPTY.
  const stateFile = path.join(stateDir, "update-state.json");
  fs.mkdirSync(stateFile);
  const previousStateDir = process.env.GC_STATE_DIR;
  process.env.GC_STATE_DIR = stateDir;
  try {
    const result = runUpdate({
      stateFile,
      checkOnly: true,
      metadata: { global: true, distribution: "npm", prefix: dir, npm: stub },
    });
    // The check succeeded; the state failure must not surface as an error.
    assert.strictEqual(result.status, "current");
  } finally {
    if (previousStateDir === undefined) delete process.env.GC_STATE_DIR;
    else process.env.GC_STATE_DIR = previousStateDir;
  }
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
  assert.match(log, /package=\S+ channel=npm status=error detail=.*ECONNRESET/);
});

test("summarizeError collapses and bounds error text", () => {
  assert.strictEqual(summarizeError(new Error("boom")), "boom");
  assert.strictEqual(summarizeError(null), "unknown error");
  assert.strictEqual(summarizeError(new Error(" \n\t ")), "unknown error");
  assert.strictEqual(summarizeError({ code: "ECONNRESET" }), "ECONNRESET");
  assert.strictEqual(summarizeError(new Error(`x${"a".repeat(400)}`)).length, 203);
  // The cut must not leave an orphaned high surrogate (broken UTF-16).
  const bounded = summarizeError(new Error(`x${"😀".repeat(150)}`));
  assert.ok(bounded.endsWith("..."));
  const body = bounded.slice(0, -3);
  const last = body.charCodeAt(body.length - 1);
  assert.ok(!(last >= 0xd800 && last <= 0xdbff), "cut must not end on a lone high surrogate");
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
    // (Foreground runs: the background path is now TTL-gated under the lock,
    // and the backoff window from run 1 is still open.)
    assert.throws(() => runUpdate({ stateFile, metadata }), /ECONNRESET/);
    state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.failureStreak, 2);
    assert.ok(state.nextCheck > Date.now() + 100 * 60 * 1000, "second backoff ~2h");
    state.summary.shown = true;
    writeJSON(stateFile, state);
    assert.throws(() => runUpdate({ stateFile, metadata }), /ECONNRESET/);
    state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.failureStreak, 3);
    assert.strictEqual(state.summary.shown, true, "an identical fingerprint must not requeue the summary");

    // A different failure requeues with the new message. Foreground
    // failures mark the summary shown immediately (it was just printed).
    const angry = path.join(dir, "angry-npm-cli.js");
    fs.writeFileSync(angry, "process.stderr.write('registry BOOM from angry stub\\n'); process.exit(1);");
    assert.throws(
      () => runUpdate({ stateFile, metadata: { ...metadata, npm: angry } }),
      /BOOM/
    );
    state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    assert.strictEqual(state.summary.shown, true);
    assert.match(state.summary.message, /BOOM/);

    // A successful check clears the failure bookkeeping (checkOnly keeps
    // the notify-mode path without installing anything).
    const healthyDir = path.join(dir, "healthy");
    fs.mkdirSync(healthyDir, { recursive: true });
    const { stub: healthy } = flakyNpmStub(healthyDir, 0, "0.0.1");
    runUpdate({ stateFile, checkOnly: true, metadata: { ...metadata, npm: healthy } });
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
  // cobra accepts --flag=value; only a truthy value counts as set.
  assert.strictEqual(disabledForInvocation(["--no-interactive=true"], {}), true);
  assert.strictEqual(disabledForInvocation(["--no-update-check=1"], {}), true);
  assert.strictEqual(disabledForInvocation(["--no-interactive=false"], {}), false);
  // CI vendors that never set CI=true.
  assert.strictEqual(disabledForInvocation([], { BUILD_NUMBER: "42" }), true);
  assert.strictEqual(disabledForInvocation([], { TEAMCITY_VERSION: "2023.05" }), true);
  assert.strictEqual(disabledForInvocation([], { CI_NAME: "codeship" }), true);
  assert.strictEqual(disabledForInvocation([], { GITHUB_ACTIONS: "true" }), true);
  assert.strictEqual(disabledForInvocation([], { BUILD_NUMBER: "" }), false);
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
  // A garbage nextCheck counts as due: the next write heals the state.
  writeJSON(updateStatePath(env), { nextCheck: "garbage" });
  assert.strictEqual(shouldSchedule([], env, 1e15), true);
});

test("a background update re-checks the TTL under the lock and stays cached", { timeout: 30000 }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-runupdate-cached-"));
  const stateDir = path.join(dir, "state");
  const stateFile = path.join(stateDir, "update-state.json");
  const { stub, attemptsFile } = flakyNpmStub(dir, 0, "9.9.9");
  // Not due: a second wrapper spawned milliseconds after the first must not
  // repeat the check.
  writeJSON(stateFile, { nextCheck: Date.now() + 60 * 60 * 1000 });
  const previousStateDir = process.env.GC_STATE_DIR;
  process.env.GC_STATE_DIR = stateDir;
  try {
    const result = runUpdate({
      stateFile,
      background: true,
      metadata: { global: true, distribution: "npm", prefix: dir, npm: stub },
    });
    assert.strictEqual(result.status, "cached");
    assert.strictEqual(fs.existsSync(attemptsFile), false, "the npm stub must not be invoked");
    // Explicit updates are never TTL-gated.
    const explicit = runUpdate({
      stateFile,
      checkOnly: true,
      metadata: { global: true, distribution: "npm", prefix: dir, npm: stub },
    });
    assert.strictEqual(explicit.status, "available");
  } finally {
    if (previousStateDir === undefined) delete process.env.GC_STATE_DIR;
    else process.env.GC_STATE_DIR = previousStateDir;
  }
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

test("the losing reclaimer of a stale lock does not double-hold", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-update-lock-race-"));
  const lock = path.join(dir, "update.lock");
  const stale = new Date(Date.now() - 16 * 60 * 1000);
  fs.writeFileSync(lock, "");
  fs.utimesSync(lock, stale, stale);
  const originalRenameSync = fs.renameSync;
  try {
    // Simulate the losing reclaimer: the winner retired the stale lock
    // between our stat and our rename.
    fs.renameSync = () => {
      const error = new Error("ENOENT: no such file or directory");
      error.code = "ENOENT";
      throw error;
    };
    assert.strictEqual(acquireLock(lock), null, "the loser must not hold the lock");
  } finally {
    fs.renameSync = originalRenameSync;
  }
  // With the race gone the stale lock is still reclaimable.
  const reclaimed = acquireLock(lock);
  assert.notStrictEqual(reclaimed, null);
  releaseLock(lock, reclaimed);
});

test("a reclaimer that steals a fresh lock restores it and yields", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-update-lock-steal-"));
  const lock = path.join(dir, "update.lock");
  const stale = new Date(Date.now() - 16 * 60 * 1000);
  fs.writeFileSync(lock, "");
  fs.utimesSync(lock, stale, stale);
  const originalStatSync = fs.statSync;
  try {
    // Simulate the stolen-lock case: another process claimed a fresh lock
    // between our staleness check and our rename, so the file we renamed
    // away is live, not stale.
    fs.statSync = (p) => {
      if (String(p).includes(".retired-")) {
        return { mtimeMs: Date.now() };
      }
      return originalStatSync(p);
    };
    assert.strictEqual(acquireLock(lock), null, "the thief must yield");
  } finally {
    fs.statSync = originalStatSync;
  }
  // The live lock was restored to its original path.
  assert.strictEqual(fs.existsSync(lock), true);
  assert.deepStrictEqual(fs.readdirSync(dir).filter((n) => n.startsWith("update.lock")), ["update.lock"]);
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

test("writeJSON retries a transient rename failure without destroying the target", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-writejson-retry-"));
  const target = path.join(root, "update-state.json");
  fs.writeFileSync(target, '{"mine":true}');
  const originalRenameSync = fs.renameSync;
  let calls = 0;
  try {
    fs.renameSync = (from, to) => {
      calls += 1;
      if (calls === 1) {
        const error = new Error("EPERM: operation not permitted");
        error.code = "EPERM";
        throw error;
      }
      return originalRenameSync(from, to);
    };
    writeJSON(target, { nextCheck: 1 });
  } finally {
    fs.renameSync = originalRenameSync;
  }
  assert.strictEqual(calls, 2, "the second attempt must succeed without unlinking");
  assert.strictEqual(JSON.parse(fs.readFileSync(target, "utf8")).nextCheck, 1);
});

test("releaseLock never removes a reclaimed lock it no longer holds", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-releaselock-"));
  const lock = path.join(root, "update-state.json.lock");
  const first = acquireLock(lock);
  assert.notStrictEqual(first, null);
  // Simulate a stale reclaim while we hold the fd: retire our lock, let
  // another process claim a fresh one at the same path.
  fs.renameSync(lock, `${lock}.retired-x`);
  fs.writeFileSync(lock, "", { flag: "wx" });
  releaseLock(lock, first);
  // The fresh holder's lock survives our release.
  assert.strictEqual(fs.existsSync(lock), true);
  // A normal release still removes our own lock.
  const second = acquireLock(`${lock}.new`);
  releaseLock(`${lock}.new`, second);
  assert.strictEqual(fs.existsSync(`${lock}.new`), false);
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
