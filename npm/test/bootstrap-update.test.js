"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawnSync } = require("child_process");
const pkgName = require("../package.json").name;
const {
  CHECK_ATTEMPTS, acquireLock, compareVersions, latestVersion, npmCommand, parseArgs, stableVersion, summarizeError,
  updateMode, updaterEnvironment, withNpmIsolation,
} = require("../lib/bootstrap-update-helper");

test("bootstrap updater accepts only stable versions", () => {
  assert.deepStrictEqual(stableVersion("1.2.3"), [1, 2, 3]);
  assert.strictEqual(stableVersion("1.2.3-beta.1"), null);
  assert.strictEqual(compareVersions("1.2.3", "1.2.4"), -1);
});

test("bootstrap updater parses detached update arguments", () => {
  const manifest = path.join(fs.mkdtempSync(path.join(os.tmpdir(), "gc-manifest-")), "install.json");
  assert.deepStrictEqual(
    parseArgs(["--background", "--force", "--parent-pid", "42", "--manifest", manifest]),
    { background: true, check: false, force: true, json: false, manifest, parentPid: 42 }
  );
});

test("bootstrap updater honors enterprise update mode", () => {
  assert.strictEqual(updateMode({ GC_CONFIG_DIR: fs.mkdtempSync(path.join(os.tmpdir(), "gc-config-")) }), "notify");
  assert.strictEqual(updateMode({ GC_UPDATE_MODE: "notify" }), "notify");
  assert.strictEqual(updateMode({ GC_UPDATE_MODE: "off" }), "off");
});

test("bootstrap updater uses a minimal child environment", () => {
  const clean = updaterEnvironment({ PATH: "/bin", GH_TOKEN: "secret", npm_config_registry: "mirror" });
  assert.strictEqual(clean.PATH, "/bin");
  assert.strictEqual(clean.GH_TOKEN, undefined);
  assert.strictEqual(clean.npm_config_registry, undefined);
});

test("bootstrap npm exec isolates config before the command separator", () => {
  const args = withNpmIsolation(["exec", "--yes", "--", "gitcode", "install"], "user.npmrc", "global.npmrc");
  const separator = args.indexOf("--");
  assert.ok(args.slice(0, separator).includes("--userconfig=user.npmrc"));
  if (pkgName.startsWith("@")) {
    assert.ok(args.slice(0, separator).includes(`--${pkgName.split("/")[0]}:registry=https://registry.npmjs.org`));
  } else {
    assert.ok(!args.some((a) => a.includes(":registry=")), "unscoped package names must not carry a scope-registry flag");
  }
  assert.ok(args.slice(0, separator).includes("--registry=https://registry.npmjs.org"));
  assert.deepStrictEqual(args.slice(separator + 1), ["gitcode", "install"]);
});

test("bootstrap updater falls back to PATH when recorded npm is stale", () => {
  const missing = path.join(os.tmpdir(), "missing-npm-cli.js");
  const command = npmCommand({ npm: missing });
  assert.ok(command.command);
  assert.notDeepStrictEqual(command.prefix, [missing]);
});

function flakyNpmStub(dir, failures, version = "0.0.2") {
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

test("bootstrap registry checks retry transient failures before giving up", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-retry-"));
  const { stub, attemptsFile } = flakyNpmStub(dir, CHECK_ATTEMPTS - 1);
  assert.strictEqual(latestVersion({ npm: stub }), "0.0.2");
  assert.strictEqual(Number(fs.readFileSync(attemptsFile, "utf8")), CHECK_ATTEMPTS);
});

test("bootstrap registry checks fail after exhausting the retry budget", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-retry-exhaust-"));
  const { stub, attemptsFile } = flakyNpmStub(dir, CHECK_ATTEMPTS);
  assert.throws(() => latestVersion({ npm: stub }), /ECONNRESET/);
  assert.strictEqual(Number(fs.readFileSync(attemptsFile, "utf8")), CHECK_ATTEMPTS);
});

test("bootstrap registry checks fail fast on deterministic errors", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-e404-"));
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
  assert.throws(() => latestVersion({ npm: stub }), /E404/);
  assert.strictEqual(Number(fs.readFileSync(attemptsFile, "utf8")), 1, "E404 must not be retried");
});

test("a prerelease bootstrap current is reported as available", { timeout: 30000 }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-prerelease-"));
  const { stub } = flakyNpmStub(root, 0, "0.0.2");
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({
    distribution: "npm-bootstrap",
    version: "0.0.1-rc.1",
    targetDir: root,
    npm: stub,
    helper: path.join(root, "helper.js"),
  }));
  const result = spawnSync(process.execPath, [path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"), "--check", "--json", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env: { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: path.join(root, "state") },
  });
  assert.strictEqual(result.status, 0, result.stderr);
  const payload = JSON.parse(result.stdout);
  assert.strictEqual(payload.status, "available");
  assert.match(payload.message, /0\.0\.2 is available/);
  assert.match(payload.message, /prerelease/);
});

test("bootstrap background current check queues no summary and clears stale notices", { timeout: 30000 }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-current-"));
  const stateDir = path.join(root, "state");
  fs.mkdirSync(stateDir, { recursive: true });
  const { stub } = flakyNpmStub(root, 0, "0.0.1");
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({
    distribution: "npm-bootstrap",
    version: "0.0.1",
    targetDir: root,
    npm: stub,
    helper: path.join(root, "helper.js"),
  }));
  fs.writeFileSync(path.join(stateDir, "update-state.json"), JSON.stringify({
    summary: { message: "GitCode CLI 9.9.9 is available.", shown: false },
  }));
  const result = spawnSync(process.execPath, [path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"), "--background", "--force", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env: { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: stateDir },
  });
  assert.strictEqual(result.status, 0, result.stderr);
  const state = JSON.parse(fs.readFileSync(path.join(stateDir, "update-state.json"), "utf8"));
  assert.strictEqual(state.summary, undefined, "current results must not queue a daily notice");
  assert.ok(state.nextCheck > Date.now());
});

test("bootstrap state lands under the package/npm-bootstrap directory", { timeout: 30000 }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-scoped-"));
  const { stub } = flakyNpmStub(root, 0, "0.0.1");
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({
    distribution: "npm-bootstrap",
    version: "0.0.1",
    targetDir: root,
    npm: stub,
    helper: path.join(root, "helper.js"),
    package: pkgName,
  }));
  const result = spawnSync(process.execPath, [path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"), "--background", "--force", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env: {
      PATH: process.env.PATH,
      HOME: process.env.HOME,
      XDG_STATE_HOME: root,
      LOCALAPPDATA: path.join(root, "la"),
    },
  });
  assert.strictEqual(result.status, 0, result.stderr);
  // The untransformed source helper uses the checkout's package name, so
  // the state must land in <state-root>/<package>/npm-bootstrap/. On
  // Windows the LOCALAPPDATA branch of stateRoot wins over XDG_STATE_HOME.
  const base = process.platform === "win32"
    ? path.join(root, "la", "gitcode-cli")
    : path.join(root, "gitcode-cli");
  const stateFile = path.join(base, pkgName, "npm-bootstrap", "update-state.json");
  const state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
  assert.ok(state.nextCheck > Date.now());
});

test("an invalid manifest is recorded as a permanent failure", { timeout: 30000 }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-permanent-"));
  const stateDir = path.join(root, "state");
  fs.mkdirSync(stateDir, { recursive: true });
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({ distribution: "bogus" }));
  const helper = path.join(__dirname, "..", "lib", "bootstrap-update-helper.js");
  const first = spawnSync(process.execPath, [helper, "--background", "--force", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env: { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: stateDir },
  });
  assert.strictEqual(first.status, 1);
  const stateFile = path.join(stateDir, "update-state.json");
  const state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
  assert.strictEqual(state.permanentError, true);
  assert.strictEqual(state.failureStreak, 1);
  assert.ok(state.nextCheck > Date.now(), "backoff still applies to manual retries");
  assert.match(state.summary.message, /invalid npm-bootstrap install manifest/);
  assert.match(state.summary.message, /Background checks are paused/);
});

test("a permanent failure stops scheduled helper runs even when due", { timeout: 30000 }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-perm-gate-"));
  const stateDir = path.join(root, "state");
  fs.mkdirSync(stateDir, { recursive: true });
  const { stub } = flakyNpmStub(root, 99, "9.9.9");
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({
    distribution: "npm-bootstrap",
    version: "0.0.1",
    targetDir: root,
    npm: stub,
    helper: path.join(root, "helper.js"),
  }));
  const stateFile = path.join(stateDir, "update-state.json");
  // Due for a check (nextCheck in the past) but flagged permanent: the
  // scheduled run must short-circuit before touching npm or the state.
  fs.writeFileSync(stateFile, JSON.stringify({
    permanentError: true,
    failureStreak: 3,
    nextCheck: 1,
    summary: { message: "Automatic update failed: earlier.", shown: true },
  }));
  const helper = path.join(__dirname, "..", "lib", "bootstrap-update-helper.js");
  const result = spawnSync(process.execPath, [helper, "--background", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env: { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: stateDir },
  });
  assert.strictEqual(result.status, 0, result.stderr);
  assert.strictEqual(JSON.parse(fs.readFileSync(stateFile, "utf8")).nextCheck, 1, "state must be untouched");
  assert.strictEqual(fs.existsSync(path.join(root, "attempts")), false, "the flaky npm stub must not be invoked");
});

test("an unwritable update.log never turns a successful bootstrap check into a failure", { timeout: 30000 }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-log-fail-"));
  const stateDir = path.join(root, "state");
  fs.mkdirSync(stateDir, { recursive: true });
  // A directory at the log path makes appendFileSync fail (EISDIR); the
  // check must still succeed (mirrors the npm-channel test).
  fs.mkdirSync(path.join(stateDir, "update.log"));
  const { stub } = flakyNpmStub(root, 0, "0.0.1");
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({
    distribution: "npm-bootstrap",
    version: "0.0.1",
    targetDir: root,
    npm: stub,
    helper: path.join(root, "helper.js"),
  }));
  const result = spawnSync(process.execPath, [path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"), "--check", "--json", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env: { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: stateDir },
  });
  assert.strictEqual(result.status, 0, result.stderr);
  const payload = JSON.parse(result.stdout);
  assert.strictEqual(payload.status, "current");
});

test("bootstrap --json reports argument errors as JSON", () => {
  // parseArgs throws before options exists; the raw-argv --json detection
  // must keep the JSON contract (mirrors the npm channel).
  const result = spawnSync(process.execPath, [
    path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"),
    "--json", "--bogus-flag", "--manifest", "/nonexistent",
  ], { encoding: "utf8", timeout: 20000, env: { PATH: process.env.PATH, HOME: process.env.HOME } });
  assert.strictEqual(result.status, 1);
  const payload = JSON.parse(result.stdout);
  assert.strictEqual(payload.status, "error");
  assert.match(payload.message, /unknown updater argument/);
});

test("a background bootstrap update re-checks the TTL under the lock", { timeout: 30000 }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-cached-"));
  const stateDir = path.join(root, "state");
  fs.mkdirSync(stateDir, { recursive: true });
  const { stub, attemptsFile } = flakyNpmStub(root, 0, "9.9.9");
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({
    distribution: "npm-bootstrap",
    version: "0.0.1",
    targetDir: root,
    npm: stub,
    helper: path.join(root, "helper.js"),
  }));
  // Not due: a background helper spawned milliseconds after a foreground
  // check must not repeat it (mirrors the npm channel's under-lock recheck).
  fs.writeFileSync(path.join(stateDir, "update-state.json"), JSON.stringify({
    nextCheck: Date.now() + 60 * 60 * 1000,
  }));
  const result = spawnSync(process.execPath, [path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"), "--background", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env: { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: stateDir },
  });
  assert.strictEqual(result.status, 0, result.stderr);
  const state = JSON.parse(fs.readFileSync(path.join(stateDir, "update-state.json"), "utf8"));
  assert.strictEqual(state.status, undefined, "a cached run must not touch the state");
  assert.strictEqual(fs.existsSync(attemptsFile), false, "the npm stub must not be invoked");
});

test("an explicit --force update after a foreground check still installs (Go update chain)", { timeout: 30000 }, () => {
  // The Go `gitcode update` chain: CheckNow (--check, unconditionally writes
  // nextCheck = +24h) then StartDetached(manifest, force=true) spawning
  // --background --force. The under-lock TTL recheck must not swallow that
  // forced spawn on the nextCheck the check itself just wrote (the regression
  // the fifth review found: the install silently never happened).
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-force-chain-"));
  const stateDir = path.join(root, "state");
  fs.mkdirSync(stateDir, { recursive: true });
  const { stub, attemptsFile } = flakyNpmStub(root, 0, "9.9.9");
  // A fake installed entry for the post-install health check. POSIX only:
  // on Windows the health check fails after the install attempt, which
  // still proves the gate passed and the install path ran.
  if (process.platform !== "win32") {
    fs.writeFileSync(path.join(root, "gitcode"), "#!/bin/sh\necho '{\"version\":\"9.9.9\"}'\n", { mode: 0o755 });
  }
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({
    distribution: "npm-bootstrap",
    version: "0.0.1",
    targetDir: root,
    npm: stub,
    helper: path.join(root, "helper.js"),
  }));
  const helperPath = path.join(__dirname, "..", "lib", "bootstrap-update-helper.js");
  const env = { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: stateDir };

  // STEP 1 — the foreground check: "available" and nextCheck = +24h.
  const check = spawnSync(process.execPath, [helperPath, "--check", "--json", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env,
  });
  assert.strictEqual(check.status, 0, check.stderr);
  assert.strictEqual(JSON.parse(check.stdout).status, "available");

  // STEP 2 — the chain's forced background spawn must run the install.
  const forced = spawnSync(process.execPath, [helperPath, "--background", "--force", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env,
  });
  const attempts = Number(fs.readFileSync(attemptsFile, "utf8"));
  // STEP 1: one view. STEP 2 (fixed): view + exec. A swallowed spawn stops
  // at one invocation.
  assert.ok(attempts >= 3, `the forced update must run the install (attempts: ${attempts})`);
  const state = JSON.parse(fs.readFileSync(path.join(stateDir, "update-state.json"), "utf8"));
  if (process.platform === "win32") {
    assert.strictEqual(forced.status, 1, "the health check fails without a runnable gitcode.exe");
    // The failure is the health check's entry spawn itself (result.error is
    // thrown verbatim), so either message shape proves the install path ran.
    assert.match(state.summary.message, /gitcode\.exe ENOENT|health check/, "the install path must have run to the health check");
  } else {
    assert.strictEqual(forced.status, 0, forced.stderr);
    assert.match(state.summary.message, /Updated GitCode CLI 0\.0\.1 -> 9\.9\.9/);
    assert.strictEqual(state.summary.shown, false, "the update summary is queued for the next launch");
  }
});

test("bootstrap stale-lock reclamation stays single-owner under a reclaim race", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-lock-race-"));
  const stateDir = path.join(root, "state");
  fs.mkdirSync(stateDir, { recursive: true });
  const lockFile = path.join(stateDir, "update-state.json.lock");
  const stale = new Date(Date.now() - 16 * 60 * 1000);
  fs.writeFileSync(lockFile, "");
  fs.utimesSync(lockFile, stale, stale);
  const originalRenameSync = fs.renameSync;
  try {
    // The losing reclaimer: the winner retired the stale lock first.
    fs.renameSync = () => {
      const error = new Error("ENOENT: no such file or directory");
      error.code = "ENOENT";
      throw error;
    };
    assert.strictEqual(acquireLock(lockFile), null, "the loser must not hold the lock");
  } finally {
    fs.renameSync = originalRenameSync;
  }
  const reclaimed = acquireLock(lockFile);
  assert.notStrictEqual(reclaimed, null);
  fs.closeSync(reclaimed);
  fs.unlinkSync(lockFile);
});

test("bootstrap failure state writes respect the cross-process lock", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-lock-fail-"));
  const stateDir = path.join(root, "state");
  fs.mkdirSync(stateDir, { recursive: true });
  const stateFile = path.join(stateDir, "update-state.json");
  const original = JSON.stringify({ nextCheck: 123, noticeShown: true, summary: { message: "pending", shown: false } });
  fs.writeFileSync(stateFile, original);
  const lock = fs.openSync(`${stateFile}.lock`, "wx", 0o600);
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({ distribution: "bogus" }));
  try {
    const result = spawnSync(process.execPath, [path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"), "--background", "--manifest", manifestFile], {
      encoding: "utf8",
      timeout: 20000,
      env: { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: stateDir },
    });
    assert.strictEqual(result.status, 1, result.stderr);
    assert.strictEqual(fs.readFileSync(stateFile, "utf8"), original, "state must stay untouched while another updater holds the lock");
    const log = fs.readFileSync(path.join(stateDir, "update.log"), "utf8");
    assert.match(log, /state locked; summary skipped/);
  } finally {
    fs.closeSync(lock);
    fs.unlinkSync(`${stateFile}.lock`);
  }
});

test("bootstrap failure summaries carry the real error instead of a dead-end notice", { timeout: 30000 }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-bg-fail-"));
  const stateDir = path.join(root, "state");
  const { stub } = flakyNpmStub(root, Infinity);
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({
    distribution: "npm-bootstrap",
    version: "0.0.1",
    targetDir: root,
    npm: stub,
    helper: path.join(root, "helper.js"),
  }));
  const childEnv = { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: stateDir };
  for (const key of ["TEMP", "TMP", "SYSTEMROOT"]) {
    if (process.env[key]) childEnv[key] = process.env[key];
  }
  const result = spawnSync(process.execPath, [path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"), "--background", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env: childEnv,
  });
  assert.strictEqual(result.status, 1, result.stderr);
  const state = JSON.parse(fs.readFileSync(path.join(stateDir, "update-state.json"), "utf8"));
  assert.match(state.summary.message, /^Automatic update failed: /);
  assert.match(state.summary.message, /ECONNRESET/);
  assert.ok(!state.summary.message.includes("run gitcode update for details"), "dead-end notice must be gone");
  assert.strictEqual(state.summary.shown, false);
  const log = fs.readFileSync(path.join(stateDir, "update.log"), "utf8");
  assert.match(log, /package=\S+ channel=npm-bootstrap status=error detail=.*ECONNRESET/);
});

test("bootstrap summarizeError collapses and bounds error text", () => {
  assert.strictEqual(summarizeError(new Error("boom")), "boom");
  assert.strictEqual(summarizeError(null), "unknown error");
  assert.strictEqual(summarizeError(new Error(" \n\t ")), "unknown error");
  assert.strictEqual(summarizeError({ code: "ECONNRESET" }), "ECONNRESET");
  // The cut must not leave an orphaned high surrogate (broken UTF-16).
  const bounded = summarizeError(new Error(`x${"😀".repeat(150)}`));
  assert.ok(bounded.endsWith("..."));
  const body = bounded.slice(0, -3);
  const last = body.charCodeAt(body.length - 1);
  assert.ok(!(last >= 0xd800 && last <= 0xdbff), "cut must not end on a lone high surrogate");
  const long = new Error(`x${"a".repeat(400)}`);
  assert.strictEqual(summarizeError(long).length, 203);
  assert.ok(summarizeError(new Error("line1\nline2")).includes("line1 line2"));
});

test("bootstrap updater parseArgs accepts =value forms for all its flags", () => {
  assert.deepStrictEqual(parseArgs(["--check=true", "--json=1", "--manifest=/tmp/m.json", "--parent-pid=42"]), {
    background: false, check: true, force: false, json: true, manifest: "/tmp/m.json", parentPid: 42,
  });
  assert.deepStrictEqual(parseArgs(["--force=false", "--manifest=/tmp/m.json"]), {
    background: false, check: false, force: false, json: false, manifest: "/tmp/m.json", parentPid: 0,
  });
  assert.deepStrictEqual(parseArgs(["--background=1", "--manifest=/tmp/m.json"]), {
    background: true, check: false, force: false, json: false, manifest: "/tmp/m.json", parentPid: 0,
  });
  assert.throws(() => parseArgs(["--check=banana"]), /unknown updater argument/);
});

test("a transient bootstrap failure over an existing pause keeps the remain-paused guidance", { timeout: 30000 }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-bootstrap-sticky-"));
  const stateDir = path.join(root, "state");
  fs.mkdirSync(stateDir, { recursive: true });
  // An earlier permanent failure paused the checks; a new transient error
  // must not lift the pause and must say the pause remains.
  fs.writeFileSync(path.join(stateDir, "update-state.json"), JSON.stringify({
    permanentError: true,
    lastErrorFingerprint: "old-fingerprint",
    summary: { message: "Automatic update failed: earlier.", shown: true },
  }));
  const { stub } = flakyNpmStub(root, 99, "0.0.2");
  const manifestFile = path.join(root, "install.json");
  fs.writeFileSync(manifestFile, JSON.stringify({
    distribution: "npm-bootstrap",
    version: "0.0.1",
    targetDir: root,
    npm: stub,
    helper: path.join(root, "helper.js"),
  }));
  const result = spawnSync(process.execPath, [path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"), "--background", "--force", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env: { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: stateDir },
  });
  assert.strictEqual(result.status, 1);
  const state = JSON.parse(fs.readFileSync(path.join(stateDir, "update-state.json"), "utf8"));
  assert.strictEqual(state.permanentError, true, "the pause is sticky");
  assert.match(state.summary.message, /remain paused from an earlier unrepaired failure/);
});
