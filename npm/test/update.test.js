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
  disabledForInvocation,
  exactInstallArgs,
  globalWrapper,
  npmCommand,
  releaseLock,
  shouldSchedule,
  shouldOnlyNotify,
  stableVersion,
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

test("update helper parser accepts check/json/background only", () => {
  assert.deepStrictEqual(parseArgs(["--check", "--json"]), {
    background: false,
    checkOnly: true,
    json: true,
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
