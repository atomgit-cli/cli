"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const { spawnSync } = require("child_process");
const path = require("path");

// The guard script is a repository-level release tool invoked from the repo
// checkout by CI. Assembled npm-package staging copies (prepare-npm-package.sh)
// carry only npm/, so the script is absent there; skip instead of failing on
// a missing module.
const script = path.join(__dirname, "..", "..", "scripts", "npm-dist-tag-guard.mjs");
const skipOutsideRepo = fs.existsSync(script)
  ? false
  : "npm-dist-tag-guard.mjs is only present in the repository layout";

function runGuard(next, current) {
  const result = spawnSync(process.execPath, [script, next, current], { encoding: "utf8" });
  return { status: result.status, stderr: result.stderr };
}

test("allows forward and equal dist-tag moves", { skip: skipOutsideRepo }, () => {
  assert.strictEqual(runGuard("1.1.0", "1.0.0").status, 0);
  assert.strictEqual(runGuard("1.0.0", "1.0.0").status, 0);
  assert.strictEqual(runGuard("2.0.0", "1.9.9").status, 0);
});

test("refuses backwards dist-tag moves", { skip: skipOutsideRepo }, () => {
  const result = runGuard("1.0.0", "1.1.0");
  assert.strictEqual(result.status, 1);
  assert.match(result.stderr, /refusing to move npm dist-tag backwards from 1\.1\.0 to 1\.0\.0/);
});

test("ranks prereleases below their stable release", { skip: skipOutsideRepo }, () => {
  // Stable -> prerelease is a move backwards.
  const backwards = runGuard("1.0.0-rc.1", "1.0.0");
  assert.strictEqual(backwards.status, 1);
  assert.match(backwards.stderr, /refusing to move npm dist-tag backwards/);
  // Prerelease ladder ordering.
  assert.strictEqual(runGuard("1.0.0-rc.1", "1.0.0-beta.2").status, 0);
  assert.strictEqual(runGuard("1.0.0-beta.1", "1.0.0-alpha.9").status, 0);
  assert.strictEqual(runGuard("1.0.0-rc.1", "1.0.0-rc.2").status, 1);
});

test("rejects malformed versions and missing arguments", { skip: skipOutsideRepo }, () => {
  assert.strictEqual(runGuard("banana", "1.0.0").status, 1);
  assert.strictEqual(runGuard("1.0.0", "banana").status, 1);
  assert.strictEqual(runGuard("", "1.0.0").status, 2);
  assert.strictEqual(runGuard("1.0.0", "").status, 2);
});
