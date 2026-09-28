"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawnSync } = require("child_process");
const pkgName = require("../package.json").name;
const {
  CHECK_ATTEMPTS, compareVersions, latestVersion, npmCommand, parseArgs, stableVersion, summarizeError,
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

function flakyNpmStub(dir, failures) {
  const attemptsFile = path.join(dir, "attempts");
  const stub = path.join(dir, "flaky-npm-cli.js");
  fs.writeFileSync(stub, [
    "const fs = require('fs');",
    `const attempts = fs.existsSync(${JSON.stringify(attemptsFile)}) ? Number(fs.readFileSync(${JSON.stringify(attemptsFile)}, "utf8")) : 0;`,
    `fs.writeFileSync(${JSON.stringify(attemptsFile)}, String(attempts + 1));`,
    `if (attempts < ${failures}) { process.stderr.write("network ECONNRESET from stub\\n"); process.exit(1); }`,
    'process.stdout.write(\'"0.0.2"\\n\');',
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
  const result = spawnSync(process.execPath, [path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"), "--background", "--manifest", manifestFile], {
    encoding: "utf8",
    timeout: 20000,
    env: { PATH: process.env.PATH, HOME: process.env.HOME, GC_STATE_DIR: stateDir },
  });
  assert.strictEqual(result.status, 1, result.stderr);
  const state = JSON.parse(fs.readFileSync(path.join(stateDir, "update-state.json"), "utf8"));
  assert.match(state.summary.message, /^Automatic update failed: /);
  assert.match(state.summary.message, /ECONNRESET/);
  assert.ok(!state.summary.message.includes("run gitcode update for details"), "dead-end notice must be gone");
  assert.strictEqual(state.summary.shown, false);
  const log = fs.readFileSync(path.join(stateDir, "update.log"), "utf8");
  assert.match(log, /status=error detail=.*ECONNRESET/);
});

test("bootstrap summarizeError collapses and bounds error text", () => {
  assert.strictEqual(summarizeError(new Error("boom")), "boom");
  assert.strictEqual(summarizeError(null), "unknown error");
  const long = new Error(`x${"a".repeat(400)}`);
  assert.strictEqual(summarizeError(long).length, 203);
  assert.ok(summarizeError(new Error("line1\nline2")).includes("line1 line2"));
});
