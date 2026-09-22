"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const pkgName = require("../package.json").name;
const {
  compareVersions, npmCommand, parseArgs, stableVersion, updateMode, updaterEnvironment, withNpmIsolation,
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
  }
  assert.deepStrictEqual(args.slice(separator + 1), ["gitcode", "install"]);
});

test("bootstrap updater falls back to PATH when recorded npm is stale", () => {
  const missing = path.join(os.tmpdir(), "missing-npm-cli.js");
  const command = npmCommand({ npm: missing });
  assert.ok(command.command);
  assert.notDeepStrictEqual(command.prefix, [missing]);
});
