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
