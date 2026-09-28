// Unit tests for bin/gc.js. Deterministic: points the wrapper at an empty
// platforms dir (via GC_PLATFORMS_DIR) so the ENOENT path is exercised
// regardless of whether the release workflow built the real binaries.
// Run: node --test (from npm/) or `npm test`.

"use strict";

const test = require("node:test");
const assert = require("node:assert");
const { execFileSync } = require("child_process");
const pkg = require("../package.json");
const fs = require("fs");
const os = require("os");
const path = require("path");

const WRAPPER = path.join(__dirname, "..", "bin", "gc.js");

function runWrapper(args, env) {
  try {
    const out = execFileSync(process.execPath, [WRAPPER, ...args], {
      encoding: "utf8",
      env: { ...process.env, ...env },
      timeout: 30000,
    });
    return { status: 0, stdout: out, stderr: "" };
  } catch (err) {
    return {
      status: err.status == null ? 1 : err.status,
      stdout: err.stdout || "",
      stderr: err.stderr || "",
    };
  }
}

test("wrapper exits 127 with a clear message when the binary is missing (ENOENT)", () => {
  // Point the wrapper at a guaranteed-empty temp dir so the ENOENT path is
  // deterministic whether or not real binaries are bundled in this checkout.
  const empty = fs.mkdtempSync(path.join(os.tmpdir(), "gc-platforms-empty-"));
  const r = runWrapper(["version"], { GC_PLATFORMS_DIR: empty });
  assert.strictEqual(r.status, 127);
  assert.match(r.stderr, /gc binary not found/);
  assert.ok(
    r.stderr.includes(
      "npx --yes --ignore-scripts --registry=https://registry.npmjs.org " +
        (pkg.name.startsWith("@") ? `--${pkg.name.split("/")[0]}:registry=https://registry.npmjs.org ` : "") +
        `${pkg.name}@latest install`
    )
  );
});

test("wrapper passes the discovered distribution to the binary with npm fallback", { skip: process.platform === "win32" }, () => {
  const { resolveBinaryName } = require("../lib/platform");
  const source = path.join(__dirname, "..");
  // Copy the wrapper's package tree (bin/, lib/, package.json) into a temp
  // root so metadata discovery writes there instead of this checkout.
  const makeTree = () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-wrapper-dist-"));
    fs.mkdirSync(path.join(root, "bin"), { recursive: true });
    fs.mkdirSync(path.join(root, "lib"), { recursive: true });
    fs.copyFileSync(path.join(source, "bin", "gc.js"), path.join(root, "bin", "gc.js"));
    for (const file of fs.readdirSync(path.join(source, "lib"))) {
      if (file.endsWith(".js")) fs.copyFileSync(path.join(source, "lib", file), path.join(root, "lib", file));
    }
    fs.copyFileSync(path.join(source, "package.json"), path.join(root, "package.json"));
    // The wrapper resolves the platform binary at bin/platforms/<name>.
    const platforms = path.join(root, "bin", "platforms");
    fs.mkdirSync(platforms, { recursive: true });
    fs.writeFileSync(path.join(platforms, resolveBinaryName(process.platform, process.arch)),
      "#!/bin/sh\necho \"$GITCODE_CLI_DISTRIBUTION\"\n", { mode: 0o755 });
    return path.join(root, "bin", "gc.js");
  };
  const distributionOf = (wrapper, env) => {
    try {
      return execFileSync(process.execPath, [wrapper, "distcheck"], {
        encoding: "utf8",
        env: { ...process.env, ...env },
        timeout: 30000,
      }).trim();
    } catch (err) {
      return `<exit ${err.status == null ? "?" : err.status}: ${(err.stderr || "").trim()}>`;
    }
  };

  // A checkout outside npm root -g is a project-local install.
  assert.strictEqual(distributionOf(makeTree(), {}), "npm-local");

  // pnpm user agent short-circuits discovery to the pnpm channel.
  assert.strictEqual(
    distributionOf(makeTree(), { npm_config_user_agent: "pnpm/9.15.9 npm/? node/v20" }),
    "pnpm"
  );

  // When discovery cannot run npm at all, the wrapper falls back to npm.
  const brokenNpm = fs.mkdtempSync(path.join(os.tmpdir(), "gc-wrapper-failnpm-"));
  const failJs = path.join(brokenNpm, "npm-cli.js");
  fs.writeFileSync(failJs, "process.exit(1);\n");
  assert.strictEqual(distributionOf(makeTree(), { npm_execpath: failJs }), "npm");
});
