"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { commandCandidates, discoverGlobalInstall, normalizePath, pathConflict } = require("../lib/install-metadata");
const { isGlobalInstall } = require("../lib/postinstall");

test("normalizes Windows paths case-insensitively and trims separators", () => {
  assert.strictEqual(normalizePath("C:\\Users\\WPF\\npm\\", true), normalizePath("c:\\users\\wpf\\npm", true));
});

test("finds a command shadowing the npm global bin", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-path-test-"));
  const oldBin = path.join(root, "python");
  const npmBin = path.join(root, "npm");
  fs.mkdirSync(oldBin);
  fs.mkdirSync(npmBin);
  fs.writeFileSync(path.join(oldBin, "gitcode.exe"), "");
  fs.writeFileSync(path.join(npmBin, "gitcode.cmd"), "");
  const env = { PATH: `${oldBin};${npmBin}` };

  const report = pathConflict(npmBin, env, true);
  assert.strictEqual(report.shadowed, true);
  assert.strictEqual(report.selected, path.join(oldBin, "gitcode.exe"));
  assert.deepStrictEqual(commandCandidates("gitcode", env, true), [
    path.join(oldBin, "gitcode.exe"),
    path.join(npmBin, "gitcode.cmd"),
  ]);
});

test("recognizes only explicit global npm lifecycle installs", () => {
  assert.strictEqual(isGlobalInstall({ npm_config_global: "true" }), true);
  assert.strictEqual(isGlobalInstall({ npm_config_global: "false" }), false);
  assert.strictEqual(isGlobalInstall({}), false);
});

test("runtime discovery distinguishes global and project-local npm packages", () => {
  const calls = [];
  const runner = (_command, args) => {
    calls.push(args);
    return { status: 0, stdout: args.includes("root") ? "/prefix/lib/node_modules\n" : "/prefix\n" };
  };
  const global = discoverGlobalInstall("/prefix/lib/node_modules/@gitcode-cli/cli", {
    npm: { command: "node", prefix: ["npm-cli.js"], metadataPath: "npm-cli.js" },
    runner,
    platform: "linux",
    version: "1.2.3",
    env: {},
  });
  assert.strictEqual(global.global, true);
  assert.strictEqual(global.distribution, "npm");
  assert.strictEqual(global.prefix, "/prefix");
  assert.strictEqual(calls.length, 2);

  const local = discoverGlobalInstall("/workspace/node_modules/@gitcode-cli/cli", {
    npm: { command: "node", prefix: ["npm-cli.js"], metadataPath: "npm-cli.js" },
    runner,
    platform: "linux",
    env: {},
  });
  assert.strictEqual(local.global, false);
  assert.strictEqual(local.distribution, "npm-local");
});

test("runtime discovery recognizes pnpm installs without invoking npm", () => {
  const runner = () => {
    throw new Error("npm must not be invoked for a pnpm install");
  };
  const metadata = discoverGlobalInstall("/home/u/.local/share/pnpm/global/5/node_modules/@gitcode-cli/cli", {
    npm: { command: "node", prefix: ["npm-cli.js"], metadataPath: "npm-cli.js" },
    runner,
    platform: "linux",
    version: "1.2.3",
    env: { npm_config_user_agent: "pnpm/9.12.0 npm/? node/v20.0.0 linux x64" },
  });
  assert.strictEqual(metadata.distribution, "pnpm");
  assert.strictEqual(metadata.global, true);
  assert.strictEqual(metadata.version, "1.2.3");

  const byLayout = discoverGlobalInstall("/home/u/.local/share/pnpm/global/5/node_modules/@gitcode-cli/cli", {
    npm: { command: "node", prefix: ["npm-cli.js"], metadataPath: "npm-cli.js" },
    runner,
    platform: "linux",
    env: {},
  });
  assert.strictEqual(byLayout.distribution, "pnpm");
});
