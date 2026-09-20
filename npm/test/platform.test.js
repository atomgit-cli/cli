// Unit tests for lib/platform.js using Node's built-in test runner.
// Run: node --test (from the npm/ directory) or `npm test`.

"use strict";

const test = require("node:test");
const assert = require("node:assert");
const { resolveBinaryName, isSupported, ARCH_MAP, PLATFORM_MAP } = require("../lib/platform");

test("resolveBinaryName maps common combos", () => {
  assert.strictEqual(resolveBinaryName("linux", "x64"), "gc-linux-amd64");
  assert.strictEqual(resolveBinaryName("linux", "arm64"), "gc-linux-arm64");
  assert.strictEqual(resolveBinaryName("openharmony", "arm64"), "gc-linux-arm64");
  assert.strictEqual(resolveBinaryName("darwin", "x64"), "gc-darwin-amd64");
  assert.strictEqual(resolveBinaryName("darwin", "arm64"), "gc-darwin-arm64");
  assert.strictEqual(resolveBinaryName("win32", "x64"), "gc-windows-amd64.exe");
});

test("resolveBinaryName throws on unsupported platform", () => {
  assert.throws(() => resolveBinaryName("aix", "x64"), /unsupported platform\/arch/);
  assert.throws(() => resolveBinaryName("linux", "ia32"), /unsupported platform\/arch/);
  assert.throws(() => resolveBinaryName("linux", "arm"), /unsupported platform\/arch/);
  assert.throws(() => resolveBinaryName("win32", "arm64"), /unsupported platform\/arch/);
});

test("isSupported returns true for shipped combos and false otherwise", () => {
  for (const combo of [
    ["linux", "x64"],
    ["linux", "arm64"],
    ["openharmony", "arm64"],
    ["darwin", "x64"],
    ["darwin", "arm64"],
    ["win32", "x64"],
  ]) {
    assert.strictEqual(isSupported(combo[0], combo[1]), true, `${combo.join("/")}`);
  }
  assert.strictEqual(isSupported("aix", "x64"), false);
  assert.strictEqual(isSupported("linux", "ia32"), false);
  assert.strictEqual(isSupported("linux", "arm"), false);
  assert.strictEqual(isSupported("win32", "arm64"), false);
  assert.strictEqual(isSupported("openharmony", "x64"), false);
});

test("arch map normalizes x64 to amd64", () => {
  assert.strictEqual(ARCH_MAP.x64, "amd64");
  assert.strictEqual(ARCH_MAP.arm64, "arm64");
});

test("platform map covers linux/darwin/win32 and the openharmony alias", () => {
  assert.strictEqual(PLATFORM_MAP.linux, "linux");
  assert.strictEqual(PLATFORM_MAP.openharmony, "linux");
  assert.strictEqual(PLATFORM_MAP.darwin, "darwin");
  assert.strictEqual(PLATFORM_MAP.win32, "windows");
});
