"use strict";

const test = require("node:test");
const assert = require("node:assert");
const { spawnSync } = require("child_process");
const path = require("path");

const script = path.join(__dirname, "..", "..", "scripts", "npm-dist-tag-guard.mjs");

function runGuard(next, current) {
  const result = spawnSync(process.execPath, [script, next, current], { encoding: "utf8" });
  return { status: result.status, stderr: result.stderr };
}

test("allows forward and equal dist-tag moves", () => {
  assert.strictEqual(runGuard("1.1.0", "1.0.0").status, 0);
  assert.strictEqual(runGuard("1.0.0", "1.0.0").status, 0);
  assert.strictEqual(runGuard("2.0.0", "1.9.9").status, 0);
});

test("refuses backwards dist-tag moves", () => {
  const result = runGuard("1.0.0", "1.1.0");
  assert.strictEqual(result.status, 1);
  assert.match(result.stderr, /refusing to move npm dist-tag backwards from 1\.1\.0 to 1\.0\.0/);
});

test("ranks prereleases below their stable release", () => {
  // Stable -> prerelease is a move backwards.
  const backwards = runGuard("1.0.0-rc.1", "1.0.0");
  assert.strictEqual(backwards.status, 1);
  assert.match(backwards.stderr, /refusing to move npm dist-tag backwards/);
  // Prerelease ladder ordering.
  assert.strictEqual(runGuard("1.0.0-rc.1", "1.0.0-beta.2").status, 0);
  assert.strictEqual(runGuard("1.0.0-beta.1", "1.0.0-alpha.9").status, 0);
  assert.strictEqual(runGuard("1.0.0-rc.1", "1.0.0-rc.2").status, 1);
});

test("rejects malformed versions and missing arguments", () => {
  assert.strictEqual(runGuard("banana", "1.0.0").status, 1);
  assert.strictEqual(runGuard("1.0.0", "banana").status, 1);
  assert.strictEqual(runGuard("", "1.0.0").status, 2);
  assert.strictEqual(runGuard("1.0.0", "").status, 2);
});
