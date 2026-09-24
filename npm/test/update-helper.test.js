"use strict";

// Tests for lib/update-helper.js (the `gc update` wrapper entry) and the
// performUpdate refusal guidance for non-npm-global channels.

const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { UPDATE_HELP, main, parseArgs } = require("../lib/update-helper");
const { performUpdate } = require("../lib/update");

test("update parseArgs accepts help and CLI-consistency flags", () => {
  assert.deepStrictEqual(parseArgs([]), { background: false, checkOnly: false, json: false, help: false });
  for (const flag of ["--help", "-h"]) {
    assert.strictEqual(parseArgs([flag]).help, true, flag);
  }
  for (const flag of ["--no-update-check", "--no-interactive"]) {
    const options = parseArgs([flag]);
    assert.strictEqual(options.background, false, flag);
    assert.strictEqual(options.checkOnly, false, flag);
    assert.strictEqual(options.help, false, flag);
  }
  assert.strictEqual(parseArgs(["--check", "--json", "--background"]).checkOnly, true);
  assert.throws(() => parseArgs(["--force"]), /unknown update argument: --force/);
});

test("gc update --help prints the wrapper help and exits zero", () => {
  const chunks = [];
  const original = process.stdout.write;
  process.stdout.write = (chunk) => {
    chunks.push(String(chunk));
    return true;
  };
  try {
    assert.strictEqual(main(["--help"]), 0);
  } finally {
    process.stdout.write = original;
  }
  const output = chunks.join("");
  assert.ok(output.includes("Usage: gc update"), output);
  assert.ok(output.includes("--check"), output);
  assert.ok(output.includes("--no-interactive"), output);
  assert.strictEqual(output, UPDATE_HELP);
});

test("gc update --help --background stays silent for the detached path", () => {
  const chunks = [];
  const original = process.stdout.write;
  process.stdout.write = (chunk) => {
    chunks.push(String(chunk));
    return true;
  };
  try {
    assert.strictEqual(main(["--help", "--background"]), 0);
  } finally {
    process.stdout.write = original;
  }
  assert.strictEqual(chunks.length, 0);
});

test("performUpdate names pnpm with its own upgrade command", () => {
  assert.throws(
    () => performUpdate({ metadata: { distribution: "pnpm", global: true } }),
    (error) => /managed by pnpm/.test(error.message) && /pnpm add -g /.test(error.message)
  );
});

test("performUpdate points npm-local installs at the global switch", () => {
  assert.throws(
    () => performUpdate({ metadata: { distribution: "npm-local", global: false } }),
    (error) => /requires a global npm install/.test(error.message) && /npm install -g /.test(error.message)
  );
});

test("performUpdate keeps the generic refusal when no metadata exists", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-no-metadata-"));
  assert.throws(
    () => performUpdate({ packageRoot: root }),
    /automatic update is available only for a global npm installation/
  );
});

test("performUpdate distinguishes project-level pnpm dependencies from global installs", () => {
  assert.throws(
    () => performUpdate({ metadata: { distribution: "pnpm", global: false } }),
    (error) => /pnpm-managed dependency/.test(error.message) && /pnpm update/.test(error.message)
  );
});
