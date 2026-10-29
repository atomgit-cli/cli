"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { commandCandidates, discoverGlobalInstall, normalizePath, pathConflict, pathEntries, windowsExecutableExtensions, writeInstallMetadata } = require("../lib/install-metadata");
const { isGlobalInstall, runPostinstall } = require("../lib/postinstall");

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
  assert.strictEqual(report.names.gitcode.shadowed, true);
  assert.strictEqual(report.names.gitcode.selected, path.join(oldBin, "gitcode.exe"));
  assert.strictEqual(report.names.gc.shadowed, false);
  assert.strictEqual(report.shadowed, true);
  assert.deepStrictEqual(commandCandidates("gitcode", env, true), [
    path.join(oldBin, "gitcode.exe"),
    path.join(npmBin, "gitcode.cmd"),
  ]);
});

test("reports a gc-only provider shadowing the npm global bin", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-path-gc-"));
  const oldBin = path.join(root, "brew");
  const npmBin = path.join(root, "npm");
  fs.mkdirSync(oldBin);
  fs.mkdirSync(npmBin);
  fs.writeFileSync(path.join(oldBin, "gc.exe"), "");
  fs.writeFileSync(path.join(npmBin, "gitcode.cmd"), "");
  const env = { PATH: `${oldBin};${npmBin}` };

  const report = pathConflict(npmBin, env, true);
  assert.strictEqual(report.names.gitcode.shadowed, false);
  assert.strictEqual(report.names.gc.shadowed, true);
  assert.strictEqual(report.names.gc.selected, path.join(oldBin, "gc.exe"));
  assert.strictEqual(report.shadowed, true);
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

test("runtime discovery recognizes unscoped global installs on every platform", () => {
  const runner = (command, args) =>
    ({ status: 0, stdout: args.includes("root") ? "/prefix/lib/node_modules\n" : "/prefix\n" });
  const options = {
    npm: { command: "node", prefix: ["npm-cli.js"], metadataPath: "npm-cli.js" },
    runner,
    platform: "linux",
    env: {},
  };
  const linux = discoverGlobalInstall("/prefix/lib/node_modules/atomgit-cli", options);
  assert.strictEqual(linux.global, true);
  assert.strictEqual(linux.distribution, "npm");

  const windows = discoverGlobalInstall("C:\\Users\\u\\AppData\\Roaming\\npm\\node_modules\\atomgit-cli", {
    ...options,
    platform: "win32",
    runner: (command, args) =>
      ({ status: 0, stdout: args.includes("root") ? "C:\\Users\\u\\AppData\\Roaming\\npm\\node_modules\r\n" : "C:\\Users\\u\\AppData\\Roaming\\npm\r\n" }),
  });
  assert.strictEqual(windows.global, true);
  assert.strictEqual(windows.distribution, "npm");

  const local = discoverGlobalInstall("/workspace/node_modules/atomgit-cli", options);
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

test("postinstall records pnpm global installs as pnpm, never as npm", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-postinstall-pnpm-"));
  const packageRoot = path.join(root, "lib", "node_modules", "@gitcode-cli", "cli");
  fs.mkdirSync(packageRoot, { recursive: true });
  const chunks = [];
  runPostinstall(
    { npm_config_global: "true", npm_config_user_agent: "pnpm/9.15.9 npm/? node/v20", npm_config_prefix: "/prefix" },
    { write: (chunk) => chunks.push(String(chunk)) },
    packageRoot
  );
  const metadata = JSON.parse(fs.readFileSync(path.join(packageRoot, ".gitcode-install.json"), "utf8"));
  assert.strictEqual(metadata.distribution, "pnpm");
  assert.strictEqual(metadata.global, true);
  assert.strictEqual(metadata.prefix, "");
  // The PATH-shadowing report speaks the npm-channel language; for a pnpm
  // install its paths and attribution are wrong, so nothing is printed.
  assert.strictEqual(chunks.join(""), "", "no npm-flavored report for the pnpm channel");
});

test("postinstall keeps recording npm global installs as npm", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-postinstall-npm-"));
  const packageRoot = path.join(root, "lib", "node_modules", "@gitcode-cli", "cli");
  fs.mkdirSync(packageRoot, { recursive: true });
  runPostinstall(
    { npm_config_global: "true", npm_config_user_agent: "npm/10.0.0 node/v20", npm_config_prefix: "/prefix", npm_execpath: "/usr/bin/npm" },
    { write: () => {} },
    packageRoot
  );
  const metadata = JSON.parse(fs.readFileSync(path.join(packageRoot, ".gitcode-install.json"), "utf8"));
  assert.strictEqual(metadata.distribution, "npm");
  assert.strictEqual(metadata.prefix, "/prefix");
});

test("postinstall warns per command name when a foreign provider shadows the npm bin", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-postinstall-shadow-"));
  const isWin = process.platform === "win32";
  const oldBin = path.join(root, "brew");
  const npmPrefix = path.join(root, "npm");
  const npmBinDir = isWin ? npmPrefix : path.join(npmPrefix, "bin");
  fs.mkdirSync(oldBin);
  fs.mkdirSync(npmBinDir, { recursive: true });
  fs.writeFileSync(path.join(oldBin, isWin ? "gc.exe" : "gc"), "");
  fs.writeFileSync(path.join(npmBinDir, isWin ? "gitcode.cmd" : "gitcode"), "");
  const packageRoot = path.join(root, "pkg");
  fs.mkdirSync(packageRoot);
  const chunks = [];
  runPostinstall(
    {
      npm_config_global: "true",
      npm_config_user_agent: "npm/10.0.0 node/v20",
      npm_config_prefix: npmPrefix,
      npm_execpath: "/usr/bin/npm",
      PATH: `${oldBin}${path.delimiter}${npmBinDir}`,
    },
    { write: (chunk) => chunks.push(String(chunk)) },
    packageRoot
  );
  const output = chunks.join("");
  assert.ok(output.includes("command:  gc\n"), "the shadowed gc name must be reported");
  assert.ok(!output.includes("command:  gitcode"), "the unshadowed gitcode name must stay silent");
  assert.ok(output.includes(path.join(npmBinDir, isWin ? "gc.cmd" : "gc")), "the direct npm entry for gc must be suggested");
});

test("pnpm layout detection also matches Windows global paths", () => {
  const runner = () => {
    throw new Error("npm must not be invoked for a pnpm install");
  };
  const metadata = discoverGlobalInstall("C:\\Users\\u\\AppData\\Local\\pnpm\\global\\5\\node_modules\\@gitcode-cli\\cli", {
    npm: { command: "node", prefix: ["npm-cli.js"], metadataPath: "npm-cli.js" },
    runner,
    platform: "win32",
    env: {},
  });
  assert.strictEqual(metadata.distribution, "pnpm");
  assert.strictEqual(metadata.global, true);
});

test("a project-level pnpm dependency is pnpm but not global", () => {
  const runner = () => {
    throw new Error("npm must not be invoked for a pnpm install");
  };
  const metadata = discoverGlobalInstall("/workspace/node_modules/@gitcode-cli/cli", {
    npm: { command: "node", prefix: ["npm-cli.js"], metadataPath: "npm-cli.js" },
    runner,
    platform: "linux",
    env: { npm_config_user_agent: "pnpm/9.15.9 npm/? node/v20" },
  });
  assert.strictEqual(metadata.distribution, "pnpm");
  assert.strictEqual(metadata.global, false);
});

test("an npm-global install under a pnpm user agent is confirmed via npm root -g", () => {
  // An npm global invoked from inside a pnpm script environment carries a
  // pnpm user agent; the layout check alone would misrecord it as pnpm.
  const runner = (_command, args) => {
    if (args.includes("root")) return { status: 0, stdout: "/global/node_modules\n" };
    if (args.includes("prefix")) return { status: 0, stdout: "/global\n" };
    throw new Error("unexpected npm invocation");
  };
  const metadata = discoverGlobalInstall("/global/node_modules/@gitcode-cli/cli", {
    npm: { command: "node", prefix: ["npm-cli.js"], metadataPath: "npm-cli.js" },
    runner,
    platform: "linux",
    env: { npm_config_user_agent: "pnpm/9.15.9 npm/? node/v20" },
  });
  assert.strictEqual(metadata.distribution, "npm");
  assert.strictEqual(metadata.global, true);
});

test("writeInstallMetadata cleans up its temp file when the rename fails", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-metadata-fail-"));
  const originalRenameSync = fs.renameSync;
  try {
    fs.renameSync = () => {
      const error = new Error("EACCES: permission denied");
      error.code = "EACCES";
      throw error;
    };
    assert.throws(() => writeInstallMetadata(root, { distribution: "npm" }), /EACCES/);
  } finally {
    fs.renameSync = originalRenameSync;
  }
  // Neither the metadata file nor a stranded .gitcode-install.json.tmp-*.
  assert.deepStrictEqual(fs.readdirSync(root), []);
});

test("writeInstallMetadata stays readable by other users in shared directories", () => {
  if (process.platform === "win32") {
    // Windows ignores the POSIX permission bits; the mode is advisory only.
    return;
  }
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-metadata-mode-"));
  writeInstallMetadata(root, { distribution: "npm" });
  const mode = fs.statSync(path.join(root, ".gitcode-install.json")).mode & 0o777;
  assert.strictEqual(mode, 0o644, "shared bin directories must keep the manifest world-readable");
});

test("isPnpmEnvironment recognizes custom PNPM_HOME via the resolved store layout", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-pnpm-custom-"));
  // A custom-named pnpm global root: no "pnpm" in the path, no user agent —
  // only the resolved store layout (global/<ver>/.pnpm/<pkg>/...) says pnpm.
  const storePkg = path.join(root, "jsbins", "global", "5", ".pnpm", "@gitcode-cli+cli@1.0.0", "node_modules", "@gitcode-cli", "cli");
  fs.mkdirSync(storePkg, { recursive: true });
  const { isPnpmEnvironment } = require("../lib/install-metadata");
  assert.strictEqual(isPnpmEnvironment({}, storePkg, false), true, "custom PNPM_HOME store layout must be recognized");
  // A project-level pnpm dependency is also pnpm-managed.
  const projectPkg = path.join(root, "proj", "node_modules", ".pnpm", "@gitcode-cli+cli@1.0.0", "node_modules", "@gitcode-cli", "cli");
  fs.mkdirSync(projectPkg, { recursive: true });
  assert.strictEqual(isPnpmEnvironment({}, projectPkg, false), true);
  // An ordinary npm tree is not.
  const npmPkg = path.join(root, "lib", "node_modules", "@gitcode-cli", "cli");
  fs.mkdirSync(npmPkg, { recursive: true });
  assert.strictEqual(isPnpmEnvironment({}, npmPkg, false), false);
});

test("discovery caches to the state dir when the package root is read-only", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "gc-discovery-cache-"));
  const stateDir = fs.mkdtempSync(path.join(os.tmpdir(), "gc-discovery-state-"));
  const packageRoot = path.join(root, "lib", "node_modules", "@gitcode-cli", "cli");
  fs.mkdirSync(packageRoot, { recursive: true });
  const { ensureInstallMetadata } = require("../lib/install-metadata");
  let spawns = 0;
  const runner = () => {
    spawns += 1;
    return { status: 0, stdout: "/prefix\n" };
  };
  const options = { runner, env: { GC_STATE_DIR: stateDir } };
  const first = ensureInstallMetadata(packageRoot, "1.0.0", options);
  assert.ok(first, "first discovery succeeds");
  assert.strictEqual(spawns, 2, "two npm invocations (root -g + prefix -g)");
  // The cache exists under the state dir.
  assert.ok(fs.existsSync(path.join(stateDir, "discovery-cache.json")));
  // The second call reads the cache: no further spawns.
  const second = ensureInstallMetadata(packageRoot, "1.0.0", options);
  assert.strictEqual(second.distribution, first.distribution);
  assert.strictEqual(spawns, 2, "the cached discovery skips the subprocesses");
  // A version change invalidates the cache (the package root metadata
  // was written on the first call, so delete it to force the cache path).
  fs.unlinkSync(path.join(packageRoot, ".gitcode-install.json"));
  const third = ensureInstallMetadata(packageRoot, "2.0.0", options);
  assert.ok(third);
  assert.strictEqual(spawns, 4, "a version change re-runs discovery");
});

test("pathEntries keeps quoted Windows separators intact", () => {
  // Mirrors Go's filepath.SplitList: separators inside quotes do not split,
  // and quotes are stripped from the entries.
  assert.deepStrictEqual(
    pathEntries({ PATH: '"C:\\a;b";C:\\c' }, true),
    ["C:\\a;b", "C:\\c"]
  );
  assert.deepStrictEqual(
    pathEntries({ PATH: 'C:\\plain;C:\\other' }, true),
    ["C:\\plain", "C:\\other"]
  );
  assert.deepStrictEqual(pathEntries({ PATH: "/a:/b" }, false), ["/a", "/b"]);
});

test("windowsExecutableExtensions follows PATHEXT with documented fallback", () => {
  // Default order matches cmd.exe's documented PATHEXT, .ps1 is always
  // appended (npm writes ps1 shims), extensionless stays last.
  assert.deepStrictEqual(windowsExecutableExtensions({}), [".com", ".exe", ".bat", ".cmd", ".ps1", ""]);
  assert.deepStrictEqual(
    windowsExecutableExtensions({ PATHEXT: ".XYZ;.FOO" }),
    [".xyz", ".foo", ".ps1", ""]
  );
  // Duplicates and missing leading dots are normalized.
  assert.deepStrictEqual(
    windowsExecutableExtensions({ PATHEXT: ".exe;EXE;.foo;.foo" }),
    [".exe", ".foo", ".ps1", ""]
  );
});
