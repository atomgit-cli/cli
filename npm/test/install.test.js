// Unit tests for lib/install.js pure helpers (dir selection, completion
// targets, PATH membership). The bootstrap side effects (copy/symlink/spawn)
// are exercised end-to-end by the release workflow; here we lock the logic.
// Run: node --test (from npm/) or `npm test`.

"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const path = require("path");
const {
  chooseGlobalBinDir, commitTransaction, completionTarget, dirFirstOnPath, dirOnPath,
  ensureUsableInstallDir, formatErrorChain, helperPackageNameTransform, installHelp, parseInstallArgs,
  persistWindowsUserPath, prependWindowsUserPath, quotePowerShell, replacePath, rollbackTransaction,
  validateWindowsPathDirectory, windowsPathGuidance,
} = require("../lib/install");

test("copied update helper gets the npm coordinate injected and loads standalone", () => {
  const helperSrc = fs.readFileSync(path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"), "utf8");
  for (const name of ["atomgit-cli", "@atomgit-cli/cli", "@gitcode-cli/cli"]) {
    const rendered = helperPackageNameTransform(name)(helperSrc);
    assert.ok(!rendered.includes('require("../package.json")'), `${name}: relative require must be rewritten`);
    assert.ok(rendered.includes(`const PACKAGE = ${JSON.stringify(name)};`), `${name}: literal coordinate expected`);
    // The copy runs from the bin directory: loading it must not throw and
    // must expose the injected coordinate through its behavior.
    const dir = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-helper-copy-"));
    const copy = path.join(dir, "gitcode-update-helper.js");
    fs.writeFileSync(copy, rendered, { mode: 0o755 });
    const helper = require(copy);
    const args = helper.withNpmIsolation(["exec", "--yes", "--", "gitcode", "install"], "u.npmrc", "g.npmrc");
    assert.ok(args.includes("--registry=https://registry.npmjs.org"), `${name}: official registry must be pinned`);
    if (name.startsWith("@")) {
      assert.ok(args.some((a) => a === `--${name.split("/")[0]}:registry=https://registry.npmjs.org`), `${name}: scope flag expected`);
    } else {
      assert.ok(!args.some((a) => a.includes(":registry=")), `${name}: no scope flag for unscoped names`);
    }
  }
});

test("helper transform fails loudly when the marker is missing", () => {
  assert.throws(() => helperPackageNameTransform("atomgit-cli")("const PACKAGE = 'stale';"));
});

test("chooseGlobalBinDir fallback explains the failure and the --target-dir escape hatch", (t) => {
  if (process.platform === "win32") {
    t.skip("posix fallback path only");
    return;
  }
  const home = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-home-notdir-"));
  fs.writeFileSync(path.join(home, ".local"), "regular file");
  // Inject candidates so the test does not depend on /usr/local/bin
  // writability (CI runners can write it, which would bypass the fallback).
  const unwritable = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-nowrite-"));
  fs.chmodSync(unwritable, 0o555);
  try {
    assert.throws(
      () => chooseGlobalBinDir(home, false, [unwritable, path.join(home, ".local", "bin")]),
      (error) => /cannot create install directory/.test(error.message) &&
        /\.local/.test(error.message) &&
        /--target-dir/.test(error.message)
    );
  } finally {
    fs.chmodSync(unwritable, 0o755);
  }
});

test("ensureUsableInstallDir reports an unwritable target dir with guidance", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-dir-unusable-"));
  if (process.platform === "win32") {
    t.skip("chmod read-only bits are advisory on Windows");
    return;
  }
  const dir = path.join(root, "bin");
  fs.mkdirSync(dir);
  fs.chmodSync(dir, 0o555);
  try {
    assert.throws(
      () => ensureUsableInstallDir(dir),
      (error) => /install directory is not usable/.test(error.message) &&
        /--target-dir/.test(error.message)
    );
  } finally {
    fs.chmodSync(dir, 0o755);
  }
});

test("ensureUsableInstallDir rejects a path occupied by a regular file with guidance", () => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-dir-notdir-"));
  const target = path.join(root, "occupied");
  fs.writeFileSync(target, "file");
  assert.throws(
    () => ensureUsableInstallDir(target),
    (error) => /install directory is not usable/.test(error.message) &&
      /--target-dir/.test(error.message)
  );
});

test("ensureUsableInstallDir accepts and creates a writable target dir", () => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-dir-ok-"));
  const dir = path.join(root, "nested", "bin");
  ensureUsableInstallDir(dir);
  assert.strictEqual(fs.statSync(dir).isDirectory(), true);
  assert.strictEqual(fs.existsSync(path.join(dir, `.gc-install-probe-${process.pid}`)), false);
});

test("formatErrorChain renders nested AggregateError causes with their errno", () => {
  const leaf = new Error("EACCES: permission denied, rename 'gc.backup-t1' -> 'gc'");
  const inner = new AggregateError([leaf, new Error("restore denied")], "path replacement failed and rollback was incomplete");
  const outer = new AggregateError([inner], "installation failed and rollback was incomplete");
  const rendered = formatErrorChain(outer);
  assert.ok(rendered.includes("installation failed and rollback was incomplete"));
  assert.ok(rendered.includes("EACCES: permission denied, rename 'gc.backup-t1' -> 'gc'"));
  assert.ok(rendered.includes("restore denied"));
  assert.ok(rendered.split("\n").length >= 4);
});

test("install explains that a directory occupies the target and points to --target-dir", () => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-dir-target-"));
  const source = path.join(root, "source");
  const target = path.join(root, "gc");
  fs.writeFileSync(source, "new");
  fs.mkdirSync(target);
  assert.throws(
    () => replacePath(source, target, "dir-target"),
    (error) => /refusing non-regular install target/.test(error.message) &&
      /is a directory/.test(error.message) &&
      /--target-dir/.test(error.message)
  );
  assert.strictEqual(fs.statSync(target).isDirectory(), true);
});

test("chooseGlobalBinDir returns a writable, existing dir on posix (regardless of /usr/local/bin)", () => {
  // Deterministic: on hosted runners /usr/local/bin may be writable, so we
  // only assert the invariant — the returned dir exists and is writable —
  // rather than which specific dir is chosen.
  const home = "/tmp/gc-install-home-" + process.pid;
  const dir = chooseGlobalBinDir(home, false);
  assert.ok(fs.existsSync(dir), `${dir} should exist`);
  // Writable probe (the function already guarantees writability by design).
  const probe = path.join(dir, ".gc-test-probe");
  fs.writeFileSync(probe, "");
  fs.unlinkSync(probe);
});

test("chooseGlobalBinDir returns the Windows per-user dir on win32 (no FS writability dependency)", () => {
  const home = "/u/home";
  const dir = chooseGlobalBinDir(home, true);
  assert.strictEqual(dir, path.join(home, "AppData", "Local", "gitcode-cli", "bin"));
});

test("completionTarget maps each shell to a standard path", () => {
  const home = "/u/home";
  assert.strictEqual(completionTarget("bash", home), path.join(home, ".local", "share", "bash-completion", "completions", "gc"));
  assert.strictEqual(completionTarget("zsh", home), path.join(home, ".zsh", "completions", "_gc"));
  assert.strictEqual(completionTarget("fish", home), path.join(home, ".config", "fish", "completions", "gc.fish"));
  assert.strictEqual(completionTarget("powershell", home), null);
});

test("dirOnPath reflects the current PATH delimiter", () => {
  // The real PATH contains some entries; a synthetic dir not in it is false.
  assert.strictEqual(dirOnPath("/this/dir/is/not/on/path"), false);
});

test("dirOnPath is case-insensitive and separator-insensitive on Windows", () => {
  const env = { PATH: `C:\\Tools\\GitCode\\;C:\\Windows` };
  assert.strictEqual(dirOnPath("c:\\tools\\gitcode", env, true), true);
});

test("dirFirstOnPath requires the Windows install directory to have priority", () => {
  const dir = "C:\\Users\\u\\AppData\\Local\\gitcode-cli\\bin";
  assert.strictEqual(dirFirstOnPath(dir, { PATH: `${dir};C:\\Old` }, true), true);
  assert.strictEqual(dirFirstOnPath(dir, { PATH: `C:\\Old;${dir}` }, true), false);
});

test("prependWindowsUserPath prepends, de-duplicates, and expands variables for comparison", () => {
  const dir = "C:\\Users\\u\\AppData\\Local\\gitcode-cli\\bin";
  const env = { LOCALAPPDATA: "C:\\Users\\u\\AppData\\Local" };
  const current = `C:\\Old;%LOCALAPPDATA%\\gitcode-cli\\bin\\;${dir.toUpperCase()};C:\\Other`;
  assert.strictEqual(prependWindowsUserPath(dir, current, env), `${dir};C:\\Old;C:\\Other`);
  assert.strictEqual(prependWindowsUserPath(dir, "", env), dir);
  assert.throws(() => prependWindowsUserPath("C:\\bad;path", "", env), /invalid Windows PATH directory/);
});

test("prependWindowsUserPath preserves every non-target PATH segment verbatim", () => {
  const dir = "C:\\GitCode\\bin";
  const current = " C:\\Keep Spaces \\ ;;;C:\\Other\\;";
  assert.strictEqual(prependWindowsUserPath(dir, current, {}), `${dir};${current}`);
});

test("validateWindowsPathDirectory rejects values that cannot be one PATH entry", () => {
  assert.doesNotThrow(() => validateWindowsPathDirectory("C:\\GitCode\\bin"));
  assert.throws(() => validateWindowsPathDirectory("C:\\safe;C:\\attacker"), /invalid Windows PATH directory/);
  assert.throws(() => validateWindowsPathDirectory("C:\\safe\0attacker"), /invalid Windows PATH directory/);
});

test("parseInstallArgs supports target directory and Windows PATH opt-out", () => {
  assert.deepStrictEqual(parseInstallArgs([]), { targetDir: "", modifyPath: true });
  assert.strictEqual(parseInstallArgs(["--target-dir", "."]).targetDir, path.resolve("."));
  assert.deepStrictEqual(parseInstallArgs(["--no-modify-path"]), { targetDir: "", modifyPath: false });
  assert.throws(() => parseInstallArgs(["--target-dir"]), /requires a directory value/);
  assert.throws(() => parseInstallArgs(["--target-dir", "--no-modify-path"]), /requires a directory value/);
  assert.throws(() => parseInstallArgs(["--force"]), /unknown install argument/);
});

test("install help documents target directory and Windows PATH opt-out", () => {
  assert.match(installHelp(), /--target-dir <directory>/);
  assert.match(installHelp(), /--no-modify-path/);
});

test("PowerShell guidance escapes single quotes in target directories", () => {
  assert.strictEqual(quotePowerShell("C:\\Users\\O'Brien\\bin;"), "'C:\\Users\\O''Brien\\bin;'");
});

test("persistWindowsUserPath uses one raw-registry PowerShell transaction and a minimal environment", () => {
  const dir = "C:\\Users\\u\\AppData\\Local\\gitcode-cli\\bin";
  const calls = [];
  const runner = (executable, args, options) => {
    calls.push({ executable, args, options });
    return { status: 0, stdout: '{"changed":true,"kind":"ExpandString","broadcasted":true}', stderr: "" };
  };
  const env = {
    SystemRoot: "C:\\Windows",
    LOCALAPPDATA: "C:\\Users\\u\\AppData\\Local",
    TEMP: "C:\\Temp",
    GC_TOKEN: "must-not-leak",
    npm_config_registry: "https://untrusted.invalid",
  };
  const result = persistWindowsUserPath(dir, { env, runner, fileExists: () => true });
  assert.deepStrictEqual(result, {
    ok: true, changed: true, registryKind: "ExpandString", broadcasted: true,
  });
  assert.strictEqual(calls.length, 1);
  assert.strictEqual(calls[0].executable, "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe");
  assert.ok(!calls[0].args.join(" ").includes(dir), "target path must not be interpolated into PowerShell code");
  const script = calls[0].args.at(-1);
  assert.match(script, /DoNotExpandEnvironmentNames/);
  assert.match(script, /GetValueKind\('Path'\)/);
  assert.match(script, /SetValue\('Path', \$next, \$kind\)/);
  assert.match(script, /Mutex.*Global\\GitCodeCli\.UserPath/);
  assert.match(script, /path-mutex-id/);
  assert.match(script, /File\]::Move\(\$candidatePath, \$mutexIdPath\)/);
  assert.match(script, /MutexSecurity/);
  assert.match(script, /SetAccessRuleProtection\(\$true, \$false\)/);
  assert.match(script, /LocalSystemSid/);
  assert.match(script, /WaitOne\(30000\)/);
  assert.match(script, /ReleaseMutex/);
  assert.match(script, /SendMessageTimeout/);
  assert.match(script, /broadcastStatus/);
  assert.match(script, /catch \{ \$broadcasted = \$false \}/);
  assert.strictEqual(calls[0].options.env.GITCODE_CLI_TARGET_DIR, dir);
  assert.strictEqual(calls[0].options.env.GC_TOKEN, undefined);
  assert.strictEqual(calls[0].options.env.npm_config_registry, undefined);
});

test("persistWindowsUserPath reports transaction failures without claiming success", () => {
  const dir = "C:\\Users\\u\\AppData\\Local\\gitcode-cli\\bin";
  const env = { SystemRoot: "C:\\Windows" };
  const failure = persistWindowsUserPath(dir, {
    env,
    fileExists: () => true,
    runner: () => ({ status: 1, stdout: "", stderr: "registry denied\nextra" }),
  });
  assert.deepStrictEqual(failure, { ok: false, error: "更新当前用户 PATH失败：registry denied" });
});

test("persistWindowsUserPath reports idempotence from the same registry transaction", () => {
  const dir = "C:\\Users\\u\\AppData\\Local\\gitcode-cli\\bin";
  let calls = 0;
  const result = persistWindowsUserPath(dir, {
    env: { SystemRoot: "C:\\Windows" },
    fileExists: () => true,
    runner: () => {
      calls += 1;
      return { status: 0, stdout: '{"changed":false,"kind":"String","broadcasted":true}', stderr: "" };
    },
  });
  assert.deepStrictEqual(result, {
    ok: true, changed: false, registryKind: "String", broadcasted: true,
  });
  assert.strictEqual(calls, 1);
});

test("persistWindowsUserPath rejects unsafe directories before spawning PowerShell", () => {
  let called = false;
  const result = persistWindowsUserPath("C:\\safe;C:\\attacker", {
    env: { SystemRoot: "C:\\Windows" },
    runner: () => { called = true; },
  });
  assert.strictEqual(result.ok, false);
  assert.strictEqual(result.invalidDirectory, true);
  assert.strictEqual(called, false);
});

test("Windows PATH guidance is explicit and Chinese for current-shell refresh", () => {
  const dir = "C:\\Users\\u\\AppData\\Local\\gitcode-cli\\bin";
  const guidance = windowsPathGuidance(
    dir,
    { modifyPath: true },
    { ok: true, changed: true },
    { PATH: "C:\\Old" }
  );
  assert.match(guidance, /已自动将 .* 置于当前用户 PATH 前面/);
  assert.match(guidance, /当前 PowerShell\/Windows Terminal 窗口无法由安装器自动刷新 PATH/);
  assert.match(guidance, /\$env:Path = 'C:\\Users\\u\\AppData\\Local\\gitcode-cli\\bin;' \+ \$env:Path/);
  assert.match(guidance, /关闭全部 PowerShell\/Windows Terminal 窗口后重新打开/);
  assert.match(guidance, /gitcode version/);
  assert.match(guidance, /其他 pip\/npm 安装入口不会被自动删除/);
  assert.match(guidance, /gitcode doctor install/);
});

test("Windows PATH guidance explains opt-out and persistence failure in Chinese", () => {
  const dir = "C:\\Users\\u\\AppData\\Local\\gitcode-cli\\bin";
  const optOut = windowsPathGuidance(dir, { modifyPath: false }, { ok: true, changed: false }, { PATH: "" });
  assert.match(optOut, /已按 --no-modify-path 跳过持久 PATH 修改/);
  assert.match(optOut, /编辑账户的环境变量/);
  assert.doesNotMatch(optOut, /SetEnvironmentVariable/);

  const failure = windowsPathGuidance(dir, { modifyPath: true }, { ok: false, error: "拒绝访问" }, { PATH: "" });
  assert.match(failure, /警告：未能自动更新当前用户 PATH：拒绝访问/);
  assert.match(failure, /编辑账户的环境变量/);
  assert.doesNotMatch(failure, /SetEnvironmentVariable/);
});

test("Windows PATH guidance reports broadcast failure without claiming full propagation", () => {
  const dir = "C:\\Users\\u\\AppData\\Local\\gitcode-cli\\bin";
  const guidance = windowsPathGuidance(
    dir,
    { modifyPath: true },
    { ok: true, changed: true, broadcasted: false },
    { PATH: "C:\\Old" }
  );
  assert.match(guidance, /持久 User PATH 已写入/);
  assert.match(guidance, /未能通知桌面环境/);
  assert.match(guidance, /注销并重新登录 Windows/);
});

test("Windows PATH guidance never emits PATH commands for an unsafe directory", () => {
  const guidance = windowsPathGuidance(
    "C:\\safe;C:\\attacker",
    { modifyPath: true },
    { ok: false, error: "invalid Windows PATH directory", invalidDirectory: true },
    { PATH: "C:\\Old" }
  );
  assert.match(guidance, /未生成任何 PATH 修改命令/);
  assert.doesNotMatch(guidance, /SetEnvironmentVariable/);
  assert.doesNotMatch(guidance, /\$env:Path/);
  assert.match(guidance, /gitcode\.exe' version/);
});

test("install rollback restores only files touched by the current transaction", () => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-transaction-"));
  const source = path.join(root, "source");
  const first = path.join(root, "gc");
  const second = path.join(root, "gitcode");
  const untouched = path.join(root, "helper");
  fs.writeFileSync(source, "new");
  fs.writeFileSync(first, "first-current");
  fs.writeFileSync(second, "second-current");
  fs.writeFileSync(untouched, "untouched-current");
  fs.writeFileSync(`${untouched}.previous`, "stale-previous");

  const transaction = [replacePath(source, first, "test"), replacePath(source, second, "test")];
  rollbackTransaction(transaction);

  assert.strictEqual(fs.readFileSync(first, "utf8"), "first-current");
  assert.strictEqual(fs.readFileSync(second, "utf8"), "second-current");
  assert.strictEqual(fs.readFileSync(untouched, "utf8"), "untouched-current");
});

test("install commit removes transaction-scoped backups", () => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-commit-"));
  const source = path.join(root, "source");
  const target = path.join(root, "gc");
  fs.writeFileSync(source, "new");
  fs.writeFileSync(target, "old");
  const record = replacePath(source, target, "commit");
  commitTransaction([record]);
  assert.strictEqual(fs.readFileSync(target, "utf8"), "new");
  assert.strictEqual(fs.existsSync(record.backup), false);
});

test("replacePath applies the helper transform at the wiring level", () => {
  // Guards the P0 wiring itself: without the transform option, the copied
  // helper keeps require("../package.json") and cannot load from a bin dir.
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-helper-wiring-"));
  const copied = path.join(root, "gitcode-update-helper.js");
  const record = replacePath(
    path.join(__dirname, "..", "lib", "bootstrap-update-helper.js"),
    copied,
    "helper-wiring",
    { transform: helperPackageNameTransform(require("../package.json").name) }
  );
  commitTransaction([record]);
  const helper = require(copied);
  assert.ok(helper.withNpmIsolation(["exec", "--", "gitcode", "install"], "u.npmrc", "g.npmrc")
    .includes("--registry=https://registry.npmjs.org"));
});

function createFileSymlinkOrSkip(t, target, link) {
  try {
    fs.symlinkSync(target, link, "file");
  } catch (error) {
    if (process.platform === "win32" && ["EPERM", "EACCES"].includes(error.code)) {
      t.skip("creating symlinks requires Windows Developer Mode or elevated privileges");
      return false;
    }
    throw error;
  }
  return true;
}

test("install transaction migrates a same-directory gitcode alias and restores it on rollback", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-alias-"));
  const source = path.join(root, "source");
  const target = path.join(root, "gc");
  const alias = path.join(root, "gitcode");
  fs.writeFileSync(source, "new");
  fs.writeFileSync(target, "old");
  if (!createFileSymlinkOrSkip(t, "gc", alias)) return;

  const transaction = [
    replacePath(source, target, "alias-rollback"),
    replacePath(source, alias, "alias-rollback", { allowedSymlinkTarget: target }),
  ];
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), false);
  assert.strictEqual(fs.readFileSync(alias, "utf8"), "new");

  rollbackTransaction(transaction);
  assert.strictEqual(fs.readFileSync(target, "utf8"), "old");
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), true);
  assert.strictEqual(fs.readlinkSync(alias), "gc");
  assert.strictEqual(fs.readFileSync(alias, "utf8"), "old");
  for (const record of transaction) assert.strictEqual(fs.existsSync(record.backup), false);
});

test("install commit removes the backup of a migrated same-directory alias", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-alias-commit-"));
  const source = path.join(root, "source");
  const target = path.join(root, "gc");
  const alias = path.join(root, "gitcode");
  fs.writeFileSync(source, "new");
  fs.writeFileSync(target, "old");
  if (!createFileSymlinkOrSkip(t, "gc", alias)) return;

  const record = replacePath(source, alias, "alias-commit", { allowedSymlinkTarget: target });
  commitTransaction([record]);
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), false);
  assert.strictEqual(fs.readFileSync(alias, "utf8"), "new");
  assert.strictEqual(fs.existsSync(record.backup), false);
});

test("install rejects aliases that point anywhere except the sibling gc binary", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-alias-reject-"));
  const source = path.join(root, "source");
  const target = path.join(root, "gc");
  const other = path.join(root, "other");
  const alias = path.join(root, "gitcode");
  fs.writeFileSync(source, "new");
  fs.writeFileSync(target, "old");
  fs.writeFileSync(other, "unrelated");
  if (!createFileSymlinkOrSkip(t, "other", alias)) return;

  assert.throws(
    () => replacePath(source, alias, "alias-reject", { allowedSymlinkTarget: target }),
    /refusing non-regular install target/
  );
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), true);
  assert.strictEqual(fs.readlinkSync(alias), "other");
  assert.strictEqual(fs.readFileSync(other, "utf8"), "unrelated");
});

test("install rejects a lexical sibling alias that resolves through an external directory symlink", (t) => {
  if (process.platform === "win32") {
    t.skip("nested symlink traversal is a POSIX migration boundary");
    return;
  }
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-alias-traversal-"));
  const external = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-alias-external-"));
  const source = path.join(root, "source");
  const target = path.join(root, "gc");
  const hop = path.join(root, "hop");
  const alias = path.join(root, "gitcode");
  const externalInner = path.join(external, "inner");
  const externalTarget = path.join(external, "gc");
  fs.writeFileSync(source, "new");
  fs.writeFileSync(target, "allowed");
  fs.mkdirSync(externalInner);
  fs.writeFileSync(externalTarget, "external");
  fs.symlinkSync(externalInner, hop, "dir");
  fs.symlinkSync("hop/../gc", alias, "file");
  assert.strictEqual(fs.readFileSync(alias, "utf8"), "external");

  assert.throws(
    () => replacePath(source, alias, "alias-traversal", { allowedSymlinkTarget: target }),
    /refusing non-regular install target/
  );
  assert.strictEqual(fs.readlinkSync(alias), "hop/../gc");
  assert.strictEqual(fs.readFileSync(externalTarget, "utf8"), "external");
  assert.strictEqual(fs.readFileSync(target, "utf8"), "allowed");
});

test("install still rejects a symlink at the primary gc target", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-primary-reject-"));
  const source = path.join(root, "source");
  const other = path.join(root, "other");
  const target = path.join(root, "gc");
  fs.writeFileSync(source, "new");
  fs.writeFileSync(other, "old");
  if (!createFileSymlinkOrSkip(t, "other", target)) return;

  assert.throws(() => replacePath(source, target, "primary-reject"), /refusing non-regular install target/);
  assert.strictEqual(fs.lstatSync(target).isSymbolicLink(), true);
  assert.strictEqual(fs.readFileSync(other, "utf8"), "old");
});

test("install adopts a gitcode symlink left by a classic npm-global install and restores it on rollback", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-npm-global-"));
  const packageBin = path.join(root, "lib", "node_modules", "@gitcode-cli", "cli", "bin", "gc.js");
  fs.mkdirSync(path.dirname(packageBin), { recursive: true });
  fs.writeFileSync(packageBin, "old");
  const binDir = path.join(root, "bin");
  fs.mkdirSync(binDir);
  const source = path.join(root, "source");
  const alias = path.join(binDir, "gitcode");
  fs.writeFileSync(source, "new");
  if (!createFileSymlinkOrSkip(t, "../lib/node_modules/@gitcode-cli/cli/bin/gc.js", alias)) return;

  const record = replacePath(source, alias, "npm-global-alias");
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), false);
  assert.strictEqual(fs.readFileSync(alias, "utf8"), "new");
  assert.strictEqual(fs.lstatSync(record.backup).isSymbolicLink(), true);
  assert.strictEqual(fs.readFileSync(record.backup, "utf8"), "old");

  rollbackTransaction([record]);
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), true);
  // Windows readlink returns backslash-separated targets; normalize both sides.
  assert.strictEqual(fs.readlinkSync(alias).split(path.sep).join("/"), "../lib/node_modules/@gitcode-cli/cli/bin/gc.js");
  assert.strictEqual(fs.readFileSync(alias, "utf8"), "old");
  assert.strictEqual(fs.existsSync(record.backup), false);
});

test("install adopts npm-global symlinks for every own npm coordinate", (t) => {
  for (const coordinate of ["@atomgit-cli/cli", "@gitcode-cli/cli", "atomgit-cli"]) {
    const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-npm-coord-"));
    const packageBin = path.join(root, "node_modules", ...coordinate.split("/"), "bin", "gc.js");
    fs.mkdirSync(path.dirname(packageBin), { recursive: true });
    fs.writeFileSync(packageBin, "old");
    const binDir = path.join(root, "bin");
    fs.mkdirSync(binDir);
    const source = path.join(root, "source");
    const alias = path.join(binDir, "gitcode");
    fs.writeFileSync(source, "new");
    if (!createFileSymlinkOrSkip(t, path.relative(binDir, packageBin), alias)) return;

    const record = replacePath(source, alias, `npm-coord-${coordinate.replace(/\//g, "-")}`);
    assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), false, coordinate);
    assert.strictEqual(fs.readFileSync(alias, "utf8"), "new", coordinate);
    commitTransaction([record]);
    assert.strictEqual(fs.existsSync(record.backup), false, coordinate);
  }
});

test("install adopts an npm-global symlink at the primary gc target", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-npm-primary-"));
  const packageBin = path.join(root, "node_modules", "atomgit-cli", "bin", "gc.js");
  fs.mkdirSync(path.dirname(packageBin), { recursive: true });
  fs.writeFileSync(packageBin, "old");
  const binDir = path.join(root, "bin");
  fs.mkdirSync(binDir);
  const source = path.join(root, "source");
  const target = path.join(binDir, "gc");
  fs.writeFileSync(source, "new");
  if (!createFileSymlinkOrSkip(t, path.relative(binDir, packageBin), target)) return;

  const record = replacePath(source, target, "npm-global-primary");
  assert.strictEqual(fs.lstatSync(target).isSymbolicLink(), false);
  assert.strictEqual(fs.readFileSync(target, "utf8"), "new");
  commitTransaction([record]);
});

test("install adopts a broken npm-global symlink into our own package tree", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-npm-broken-"));
  const binDir = path.join(root, "bin");
  fs.mkdirSync(binDir);
  const source = path.join(root, "source");
  const alias = path.join(binDir, "gitcode");
  fs.writeFileSync(source, "new");
  if (!createFileSymlinkOrSkip(t, "../lib/node_modules/@gitcode-cli/cli/bin/gc.js", alias)) return;

  const record = replacePath(source, alias, "npm-global-broken");
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), false);
  assert.strictEqual(fs.readFileSync(alias, "utf8"), "new");
  commitTransaction([record]);
});

test("install rejects a symlink into the third-party gitcode-cli package with uninstall guidance", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-third-party-"));
  const packageBin = path.join(root, "lib", "node_modules", "gitcode-cli", "bin", "gc.js");
  fs.mkdirSync(path.dirname(packageBin), { recursive: true });
  fs.writeFileSync(packageBin, "unrelated");
  const binDir = path.join(root, "bin");
  fs.mkdirSync(binDir);
  const source = path.join(root, "source");
  const alias = path.join(binDir, "gitcode");
  fs.writeFileSync(source, "new");
  if (!createFileSymlinkOrSkip(t, "../lib/node_modules/gitcode-cli/bin/gc.js", alias)) return;

  assert.throws(
    () => replacePath(source, alias, "third-party-reject"),
    (error) => {
      // Windows renders the link target with backslashes; normalize before matching.
      const message = error.message.split(path.sep).join("/");
      return /refusing non-regular install target/.test(message) &&
        /node_modules\/gitcode-cli/.test(message) &&
        /npm uninstall -g gitcode-cli/.test(message);
    }
  );
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), true);
  assert.strictEqual(fs.readFileSync(packageBin, "utf8"), "unrelated");
});

test("install rejects a foreign symlink with the link target and --target-dir guidance", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-foreign-"));
  const source = path.join(root, "source");
  const other = path.join(root, "other");
  const alias = path.join(root, "gitcode");
  fs.writeFileSync(source, "new");
  fs.writeFileSync(other, "unrelated");
  if (!createFileSymlinkOrSkip(t, "other", alias)) return;

  assert.throws(
    () => replacePath(source, alias, "foreign-reject"),
    (error) => /refusing non-regular install target/.test(error.message) &&
      /symlink -> other/.test(error.message) &&
      /--target-dir/.test(error.message)
  );
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), true);
  assert.strictEqual(fs.readFileSync(other, "utf8"), "unrelated");
});

test("install reports an actionable refusal for a symlink loop at the primary target", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-loop-"));
  const binDir = path.join(root, "bin");
  fs.mkdirSync(binDir);
  const source = path.join(root, "source");
  const target = path.join(binDir, "gc");
  fs.writeFileSync(source, "new");
  if (!createFileSymlinkOrSkip(t, "gitcode", target)) return;
  if (!createFileSymlinkOrSkip(t, "gc", path.join(binDir, "gitcode"))) return;

  assert.throws(
    () => replacePath(source, target, "loop-reject"),
    (error) => /refusing non-regular install target/.test(error.message) &&
      !/ELOOP/.test(error.message)
  );
  assert.strictEqual(fs.lstatSync(target).isSymbolicLink(), true);
});

test("install refuses symlinks inside the nested dependency tree of another package", (t) => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-nested-"));
  const packageBin = path.join(root, "project", "node_modules", "pkg-a", "node_modules", "@gitcode-cli", "cli", "bin", "gc.js");
  fs.mkdirSync(path.dirname(packageBin), { recursive: true });
  fs.writeFileSync(packageBin, "old");
  const binDir = path.join(root, "bin");
  fs.mkdirSync(binDir);
  const source = path.join(root, "source");
  const alias = path.join(binDir, "gitcode");
  fs.writeFileSync(source, "new");
  if (!createFileSymlinkOrSkip(t, path.relative(binDir, packageBin), alias)) return;

  assert.throws(() => replacePath(source, alias, "nested-reject"), /refusing non-regular install target/);
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), true);
  assert.strictEqual(fs.readFileSync(packageBin, "utf8"), "old");
});

test("install judges npm-global adoption by the resolved path, not the link text", (t) => {
  if (process.platform === "win32") {
    t.skip("directory symlinks require Windows Developer Mode or elevated privileges");
    return;
  }
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-authority-"));
  const external = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-authority-external-"));
  fs.mkdirSync(path.join(root, "lib"));
  fs.mkdirSync(path.join(external, "libs", "@gitcode-cli", "cli", "bin"), { recursive: true });
  const externalBin = path.join(external, "libs", "@gitcode-cli", "cli", "bin", "gc.js");
  fs.writeFileSync(externalBin, "external");
  fs.symlinkSync(path.join(external, "libs"), path.join(root, "lib", "node_modules"), "dir");
  const binDir = path.join(root, "bin");
  fs.mkdirSync(binDir);
  const source = path.join(root, "source");
  const alias = path.join(binDir, "gitcode");
  fs.writeFileSync(source, "new");
  if (!createFileSymlinkOrSkip(t, "../lib/node_modules/@gitcode-cli/cli/bin/gc.js", alias)) return;

  assert.throws(() => replacePath(source, alias, "authority-reject"), /refusing non-regular install target/);
  assert.strictEqual(fs.lstatSync(alias).isSymbolicLink(), true);
  assert.strictEqual(fs.readFileSync(externalBin, "utf8"), "external");
});

test("rollback continues restoring remaining paths after one restore fails", () => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-rollback-failure-"));
  const source = path.join(root, "source");
  const target = path.join(root, "gc");
  const blocked = path.join(root, "blocked-directory");
  fs.writeFileSync(source, "new");
  fs.writeFileSync(target, "old");
  fs.mkdirSync(blocked);
  const good = replacePath(source, target, "failure-test");

  assert.throws(
    () => rollbackTransaction([good, { dst: blocked, backup: `${blocked}.backup`, hadOriginal: false }]),
    (error) => error instanceof AggregateError && error.errors.length === 1
  );
  assert.strictEqual(fs.readFileSync(target, "utf8"), "old");
});

test("rollback reports a missing backup and preserves the current target", () => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-missing-backup-"));
  const source = path.join(root, "source");
  const target = path.join(root, "gc");
  fs.writeFileSync(source, "new");
  fs.writeFileSync(target, "old");
  const record = replacePath(source, target, "missing-backup");
  fs.unlinkSync(record.backup);

  assert.throws(
    () => rollbackTransaction([record]),
    (error) => error instanceof AggregateError &&
      error.errors.length === 1 &&
      error.errors[0].code === "EROLLBACK"
  );
  assert.strictEqual(fs.readFileSync(target, "utf8"), "new");
});

test("replacePath reports both the replacement and internal restore failures", () => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-internal-restore-"));
  const source = path.join(root, "source");
  const target = path.join(root, "gc");
  const backup = `${target}.backup-internal-restore`;
  fs.writeFileSync(source, "new");
  fs.writeFileSync(target, "old");
  const originalRenameSync = fs.renameSync;
  fs.renameSync = (src, dst) => {
    const error = new Error(src === backup ? "restore denied" : "replacement denied");
    error.code = "EIO";
    throw error;
  };
  try {
    assert.throws(
      () => replacePath(source, target, "internal-restore"),
      (error) => error instanceof AggregateError &&
        error.errors.length === 2 &&
        error.errors[0].message === "replacement denied" &&
        error.errors[1].message === "restore denied"
    );
  } finally {
    fs.renameSync = originalRenameSync;
  }
  assert.strictEqual(fs.readFileSync(target, "utf8"), "old");
  assert.strictEqual(fs.readFileSync(backup, "utf8"), "old");
});

test("replacePath reports a backup that disappears before internal restore", () => {
  const root = fs.mkdtempSync(path.join(require("os").tmpdir(), "gc-install-lost-backup-"));
  const source = path.join(root, "source");
  const target = path.join(root, "gc");
  const backup = `${target}.backup-lost-backup`;
  fs.writeFileSync(source, "new");
  fs.writeFileSync(target, "old");
  const originalRenameSync = fs.renameSync;
  let replacementAttempts = 0;
  fs.renameSync = (src, dst) => {
    if (src.startsWith(`${target}.tmp-`)) {
      replacementAttempts += 1;
      const error = new Error(replacementAttempts === 1 ? "replace collision" : "replacement failed");
      error.code = replacementAttempts === 1 ? "EPERM" : "EIO";
      if (replacementAttempts === 2) fs.unlinkSync(backup);
      throw error;
    }
    return originalRenameSync(src, dst);
  };
  try {
    assert.throws(
      () => replacePath(source, target, "lost-backup"),
      (error) => error instanceof AggregateError &&
        error.errors.length === 2 &&
        error.errors[0].message === "replacement failed" &&
        error.errors[1].code === "EROLLBACK"
    );
  } finally {
    fs.renameSync = originalRenameSync;
  }
  assert.strictEqual(fs.existsSync(target), false);
  assert.strictEqual(fs.existsSync(backup), false);
});
