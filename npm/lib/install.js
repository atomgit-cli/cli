// Bootstrap ("install" subcommand) for @gitcode-cli/cli.
//
// Copies the bundled platform binary to a global bin directory and, on
// Linux/macOS, installs shell completions. The registry-isolated npx bootstrap works without a prior
// `npm i -g`. Dependency-free (Node built-ins only). Pure helpers are
// exported separately so they can be unit-tested without touching the FS.

"use strict";

const fs = require("fs");
const path = require("path");
const os = require("os");
const crypto = require("crypto");
const { spawnSync } = require("child_process");
const { resolveBinaryName, isSupported } = require("./platform");
const { normalizePath, writeInstallMetadata } = require("./install-metadata");
const pkg = require("../package.json");

const PLATFORMS_DIR = path.join(__dirname, "..", "bin", "platforms");
const BOOTSTRAP_HELPER = path.join(__dirname, "bootstrap-update-helper.js");
const WINDOWS_UPDATE_USER_PATH = [
  "$ErrorActionPreference = 'Stop'",
  "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)",
  "$target = $env:GITCODE_CLI_TARGET_DIR",
  "if ([string]::IsNullOrWhiteSpace($target) -or -not [IO.Path]::IsPathRooted($target) -or $target.Contains(';') -or $target.Contains([char]0)) { throw 'invalid Windows PATH directory' }",
  "$sidObject = [Security.Principal.WindowsIdentity]::GetCurrent().User",
  "$sid = $sidObject.Value",
  "$stateDir = [IO.Path]::Combine($env:LOCALAPPDATA, 'gitcode-cli')",
  "[IO.Directory]::CreateDirectory($stateDir) | Out-Null",
  "$mutexIdPath = [IO.Path]::Combine($stateDir, 'path-mutex-id')",
  "if (-not [IO.File]::Exists($mutexIdPath)) {",
  "  $candidate = [Guid]::NewGuid().ToString('N')",
  "  $candidatePath = $mutexIdPath + '.' + [Guid]::NewGuid().ToString('N') + '.tmp'",
  "  [IO.File]::WriteAllText($candidatePath, $candidate, [Text.UTF8Encoding]::new($false))",
  "  try { [IO.File]::Move($candidatePath, $mutexIdPath) } catch [IO.IOException] { if (-not [IO.File]::Exists($mutexIdPath)) { throw } } finally { if ([IO.File]::Exists($candidatePath)) { [IO.File]::Delete($candidatePath) } }",
  "}",
  "$mutexId = [IO.File]::ReadAllText($mutexIdPath).Trim()",
  "if ($mutexId -notmatch '^[0-9a-f]{32}$') { throw 'invalid Windows PATH mutex identifier' }",
  "$mutexSecurity = [Security.AccessControl.MutexSecurity]::new()",
  "$mutexSecurity.SetAccessRuleProtection($true, $false)",
  "$mutexSecurity.AddAccessRule([Security.AccessControl.MutexAccessRule]::new($sidObject, [Security.AccessControl.MutexRights]::FullControl, [Security.AccessControl.AccessControlType]::Allow))",
  "$systemSid = [Security.Principal.SecurityIdentifier]::new([Security.Principal.WellKnownSidType]::LocalSystemSid, $null)",
  "$mutexSecurity.AddAccessRule([Security.AccessControl.MutexAccessRule]::new($systemSid, [Security.AccessControl.MutexRights]::FullControl, [Security.AccessControl.AccessControlType]::Allow))",
  "$createdNew = $false",
  "$mutex = [Threading.Mutex]::new($false, ('Global\\GitCodeCli.UserPath.' + $sid + '.' + $mutexId), [ref]$createdNew, $mutexSecurity)",
  "$mutexHeld = $false",
  "try {",
  "  try { $mutexHeld = $mutex.WaitOne(30000) } catch [Threading.AbandonedMutexException] { $mutexHeld = $true }",
  "  if (-not $mutexHeld) { throw 'timed out waiting for the user PATH update lock' }",
  "  $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)",
  "  if ($null -eq $key) { throw 'cannot open current user Environment registry key' }",
  "  try {",
  "    $hasPath = $key.GetValueNames() -contains 'Path'",
  "    $kind = if ($hasPath) { $key.GetValueKind('Path') } else { [Microsoft.Win32.RegistryValueKind]::ExpandString }",
  "    if ($kind -ne [Microsoft.Win32.RegistryValueKind]::String -and $kind -ne [Microsoft.Win32.RegistryValueKind]::ExpandString) { throw ('unsupported user PATH registry kind: ' + $kind) }",
  "    $current = if ($hasPath) { [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames) } else { '' }",
  "    function Normalize-GitCodePathEntry([string]$value) {",
  "      if ([string]::IsNullOrWhiteSpace($value)) { return $null }",
  "      $candidate = [Environment]::ExpandEnvironmentVariables($value.Trim().Trim([char]34))",
  "      try { return [IO.Path]::GetFullPath($candidate).TrimEnd([char]92).ToLowerInvariant() } catch { return $candidate.TrimEnd([char]92).ToLowerInvariant() }",
  "    }",
  "    $wanted = Normalize-GitCodePathEntry $target",
  "    $kept = [System.Collections.Generic.List[string]]::new()",
  "    if ($current.Length -gt 0) { foreach ($entry in $current.Split([char]59)) { if ((Normalize-GitCodePathEntry $entry) -ne $wanted) { $kept.Add($entry) } } }",
  "    $suffix = [string]::Join(';', $kept)",
  "    $next = if ($suffix.Length -gt 0) { $target + ';' + $suffix } else { $target }",
  "    $changed = $next -cne $current",
  "    if ($changed) { $key.SetValue('Path', $next, $kind) }",
  "  } finally { $key.Dispose() }",
  "  $broadcasted = $true",
  "  if ($changed) {",
  "    try {",
  "      Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public static class GitCodeEnvironmentBroadcast { [DllImport(\"user32.dll\", CharSet=CharSet.Unicode, SetLastError=true)] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint flags, uint timeout, out UIntPtr result); }'",
  "      $broadcastResult = [UIntPtr]::Zero",
  "      $broadcastStatus = [GitCodeEnvironmentBroadcast]::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero, 'Environment', 0x2, 5000, [ref]$broadcastResult)",
  "      $broadcasted = $broadcastStatus -ne [IntPtr]::Zero",
  "    } catch { $broadcasted = $false }",
  "  }",
  "} finally { if ($mutexHeld) { $mutex.ReleaseMutex() }; $mutex.Dispose() }",
  "@{ changed = [bool]$changed; kind = $kind.ToString(); broadcasted = [bool]$broadcasted } | ConvertTo-Json -Compress",
].join("; ");

function bundledBinaryPath() {
  return path.join(PLATFORMS_DIR, resolveBinaryName(process.platform, process.arch));
}

/**
 * Choose a global bin dir. Prefer a no-sudo writable location; fall back to a
 * per-user dir under the home directory. Pure (no FS side effects beyond the
 * write probe on the candidate dir).
 */
function chooseGlobalBinDir(home, isWin) {
  if (isWin) {
    return path.join(home, "AppData", "Local", "gitcode-cli", "bin");
  }
  for (const dir of ["/usr/local/bin", path.join(home, ".local", "bin")]) {
    try {
      fs.mkdirSync(dir, { recursive: true });
      const probe = path.join(dir, ".gc-write-probe");
      fs.writeFileSync(probe, "");
      fs.unlinkSync(probe);
      return dir;
    } catch {
      // not writable; try next
    }
  }
  const dir = path.join(home, ".local", "bin");
  fs.mkdirSync(dir, { recursive: true });
  return dir;
}

// Completion target dirs per shell (user-writable, auto-loaded where possible).
// Pure: derives the path from the shell + home.
function completionTarget(shell, home) {
  switch (shell) {
    case "bash":
      return path.join(home, ".local", "share", "bash-completion", "completions", "gc");
    case "zsh":
      return path.join(home, ".zsh", "completions", "_gc");
    case "fish":
      return path.join(home, ".config", "fish", "completions", "gc.fish");
    default:
      return null;
  }
}

function ensureExec(file) {
  if (process.platform === "win32") return;
  try {
    fs.chmodSync(file, 0o755);
  } catch {
    /* best-effort */
  }
}

function pathExists(file) {
  try {
    fs.lstatSync(file);
    return true;
  } catch (error) {
    if (error.code === "ENOENT") return false;
    throw error;
  }
}

function isAllowedAliasSymlink(dst, allowedTarget) {
  if (!allowedTarget) return false;
  try {
    const targetStat = fs.lstatSync(allowedTarget, { bigint: true });
    if (!targetStat.isFile() || targetStat.isSymbolicLink()) return false;
    const linkTargetStat = fs.statSync(dst, { bigint: true });
    return linkTargetStat.isFile() &&
      linkTargetStat.dev === targetStat.dev &&
      linkTargetStat.ino === targetStat.ino;
  } catch (error) {
    if (["ENOENT", "EINVAL"].includes(error.code)) return false;
    throw error;
  }
}

function replacePath(src, dst, transactionID, options = {}) {
  const temp = `${dst}.tmp-${process.pid}-${crypto.randomBytes(8).toString("hex")}`;
  const backup = `${dst}.backup-${transactionID}`;
  const sourceStat = fs.lstatSync(src);
  if (!sourceStat.isFile() || sourceStat.isSymbolicLink()) {
    throw new Error(`refusing non-regular install source: ${src}`);
  }
  let hadOriginal = false;
  let moveOriginal = false;
  try {
    const stat = fs.lstatSync(dst);
    if (stat.isSymbolicLink() && isAllowedAliasSymlink(dst, options.allowedSymlinkTarget)) {
      moveOriginal = true;
    } else if (!stat.isFile() || stat.isSymbolicLink()) {
      throw new Error(`refusing non-regular install target: ${dst}`);
    }
    hadOriginal = true;
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  try {
    fs.lstatSync(backup);
    throw new Error(`refusing existing transaction backup: ${backup}`);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  fs.copyFileSync(src, temp, fs.constants.COPYFILE_EXCL);
  ensureExec(temp);
  let backupReady = false;
  try {
    if (hadOriginal) {
      if (moveOriginal) {
        fs.renameSync(dst, backup);
        backupReady = true;
        if (!isAllowedAliasSymlink(backup, options.allowedSymlinkTarget)) {
          throw new Error(`refusing non-regular install target: ${dst}`);
        }
      } else {
        fs.copyFileSync(dst, backup, fs.constants.COPYFILE_EXCL);
        backupReady = true;
      }
    }
    renameReplace(temp, dst);
    return { dst, backup, hadOriginal };
  } catch (error) {
    let restoreError;
    try {
      if (backupReady) {
        if (!pathExists(backup)) {
          restoreError = new Error(`transaction backup missing: ${backup}`);
          restoreError.code = "EROLLBACK";
        } else {
          renameReplace(backup, dst);
        }
      }
    } catch (caught) {
      // Keep the transaction backup for manual recovery.
      restoreError = caught;
    }
    try {
      fs.unlinkSync(temp);
    } catch {
      // Preserve the original error.
    }
    if (restoreError) {
      throw new AggregateError(
        [error, restoreError],
        "path replacement failed and rollback was incomplete",
        { cause: error }
      );
    }
    throw error;
  }
}

function renameReplace(src, dst) {
  try {
    fs.renameSync(src, dst);
  } catch (error) {
    if (!["EEXIST", "EPERM"].includes(error.code)) throw error;
    fs.unlinkSync(dst);
    fs.renameSync(src, dst);
  }
}

function rollbackTransaction(records) {
  const failures = [];
  for (const record of [...records].reverse()) {
    if (record.hadOriginal) {
      try {
        if (!pathExists(record.backup)) {
          const error = new Error(`transaction backup missing: ${record.backup}`);
          error.code = "EROLLBACK";
          throw error;
        }
        renameReplace(record.backup, record.dst);
      } catch (error) {
        failures.push(error);
      }
      continue;
    }
    try {
      fs.unlinkSync(record.dst);
    } catch (error) {
      if (error.code !== "ENOENT") failures.push(error);
    }
  }
  if (failures.length) {
    throw new AggregateError(failures, `failed to restore ${failures.length} install path(s)`);
  }
}

function commitTransaction(records) {
  for (const record of records) {
    try {
      fs.unlinkSync(record.backup);
    } catch {
      // The install is already committed; an orphaned unique backup is safer
      // than rolling back a healthy installation because cleanup failed.
    }
  }
}

function sha256(file) {
  return crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
}

function runGc(bin, args) {
  return spawnSync(bin, args, { encoding: "utf8" });
}

function installCompletions(bin, home) {
  const installed = [];
  for (const shell of ["bash", "zsh", "fish"]) {
    const res = runGc(bin, ["completion", shell]);
    if (res.status !== 0 || !res.stdout) continue;
    const target = completionTarget(shell, home);
    if (!target) continue;
    try {
      fs.mkdirSync(path.dirname(target), { recursive: true });
      fs.writeFileSync(target, res.stdout, { mode: 0o644 });
      installed.push(`${shell}: ${target}`);
    } catch {
      /* skip unwritable */
    }
  }
  return installed;
}

// Whether the per-user fallback dir is on PATH (pure).
function dirOnPath(dir, env = process.env, isWin = process.platform === "win32") {
  const wanted = normalizePath(dir, isWin);
  const delimiter = isWin ? ";" : ":";
  return (env.PATH || env.Path || "")
    .split(delimiter)
    .filter(Boolean)
    .some((entry) => normalizePath(entry.replace(/^"|"$/g, ""), isWin) === wanted);
}

function dirFirstOnPath(dir, env = process.env, isWin = process.platform === "win32") {
  const delimiter = isWin ? ";" : ":";
  const first = (env.PATH || env.Path || "")
    .split(delimiter)
    .map((entry) => entry.trim().replace(/^"|"$/g, ""))
    .find(Boolean);
  return Boolean(first && normalizePath(first, isWin) === normalizePath(dir, isWin));
}

function environmentValue(env, name) {
  const wanted = name.toLowerCase();
  const key = Object.keys(env).find((candidate) => candidate.toLowerCase() === wanted);
  return key ? env[key] : "";
}

function expandWindowsEnvironment(value, env) {
  return value.replace(/%([^%]+)%/g, (match, name) => environmentValue(env, name) || match);
}

function validateWindowsPathDirectory(dir) {
  if (!path.win32.isAbsolute(dir) || dir.includes(";") || dir.includes("\0")) {
    throw new Error(`invalid Windows PATH directory: ${dir}`);
  }
}

function prependWindowsUserPath(dir, current = "", env = process.env) {
  validateWindowsPathDirectory(dir);
  const wanted = normalizePath(dir, true);
  const raw = String(current);
  const entries = raw
    .split(";")
    .filter((entry) => {
      const unquoted = entry.trim().replace(/^"|"$/g, "");
      if (!unquoted) return true;
      return normalizePath(expandWindowsEnvironment(unquoted, env), true) !== wanted;
    });
  if (!raw || entries.length === 0) return dir;
  return `${dir};${entries.join(";")}`;
}

function windowsPowerShellExecutable(env) {
  const systemRoot = environmentValue(env, "SystemRoot") || environmentValue(env, "WINDIR");
  if (!systemRoot || !path.win32.isAbsolute(systemRoot)) {
    throw new Error("Windows SystemRoot is unavailable");
  }
  return path.win32.join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe");
}

function windowsPowerShellEnv(env, extra = {}) {
  const allowed = [
    "SystemRoot", "WINDIR", "SystemDrive", "TEMP", "TMP", "USERPROFILE",
    "HOMEDRIVE", "HOMEPATH", "LOCALAPPDATA", "APPDATA", "ProgramData", "ProgramFiles",
  ];
  const childEnv = {};
  for (const name of allowed) {
    const value = environmentValue(env, name);
    if (value) childEnv[name] = value;
  }
  return { ...childEnv, ...extra };
}

function powerShellFailure(action, result) {
  const detail = String(result.error?.message || result.stderr || `exit code ${result.status}`)
    .trim()
    .split(/\r?\n/, 1)[0];
  return `${action}失败${detail ? `：${detail}` : ""}`;
}

function persistWindowsUserPath(dir, options = {}) {
  const env = options.env || process.env;
  const runner = options.runner || spawnSync;
  const fileExists = options.fileExists || fs.existsSync;
  try {
    validateWindowsPathDirectory(dir);
  } catch (error) {
    return { ok: false, error: error.message, invalidDirectory: true };
  }
  let executable;
  try {
    executable = windowsPowerShellExecutable(env);
  } catch (error) {
    return { ok: false, error: error.message };
  }
  if (!fileExists(executable)) {
    return { ok: false, error: `找不到 Windows PowerShell：${executable}` };
  }
  const result = runner(executable, ["-NoLogo", "-NoProfile", "-NonInteractive", "-Command", WINDOWS_UPDATE_USER_PATH], {
    encoding: "utf8",
    windowsHide: true,
    env: windowsPowerShellEnv(env, { GITCODE_CLI_TARGET_DIR: dir }),
  });
  if (result.error || result.status !== 0) {
    return { ok: false, error: powerShellFailure("更新当前用户 PATH", result) };
  }
  try {
    const parsed = JSON.parse(String(result.stdout || "").trim());
    if (typeof parsed.changed !== "boolean" || typeof parsed.broadcasted !== "boolean" ||
        !["String", "ExpandString"].includes(parsed.kind)) {
      throw new Error("unexpected PowerShell result");
    }
    return {
      ok: true,
      changed: parsed.changed,
      registryKind: parsed.kind,
      broadcasted: parsed.broadcasted,
    };
  } catch (error) {
    return { ok: false, error: `解析 Windows PATH 更新结果失败：${error.message}` };
  }
}

function parseInstallArgs(args) {
  const options = { targetDir: "", modifyPath: true };
  for (let index = 0; index < args.length; index += 1) {
    if (args[index] === "--target-dir") {
      if (!args[index + 1] || args[index + 1].startsWith("-")) {
        throw new Error("--target-dir requires a directory value");
      }
      options.targetDir = path.resolve(args[index + 1]);
      index += 1;
      continue;
    }
    if (args[index] === "--no-modify-path") {
      options.modifyPath = false;
      continue;
    }
    throw new Error(`unknown install argument: ${args[index]}`);
  }
  return options;
}

function quotePowerShell(value) {
  return `'${String(value).replace(/'/g, "''")}'`;
}

function installHelp() {
  return [
    "Install bundled gc and gitcode binaries outside the npm package directory.",
    "",
    "Usage:",
    "  gitcode install [--target-dir <directory>] [--no-modify-path]",
    "",
    "Flags:",
    "  --target-dir <directory>  Install into an explicit directory",
    "  --no-modify-path           Do not update the Windows user PATH",
    "  -h, --help                Show this help",
    "",
  ].join("\n");
}

function windowsPathGuidance(dir, options, result, env = process.env) {
  const lines = ["", "Windows PATH 配置："];
  try {
    validateWindowsPathDirectory(dir);
  } catch {
    lines.push("  警告：目标目录包含不能安全加入 Windows PATH 的字符，未生成任何 PATH 修改命令。");
    lines.push("  请重新安装到不含分号或空字符的绝对目录，或直接使用已安装程序：");
    lines.push(`    & ${quotePowerShell(path.join(dir, "gitcode.exe"))} version`);
    lines.push("  其他 pip/npm 安装入口不会被自动删除；如需清理，请先运行 gitcode doctor install 确认来源。");
    return `${lines.join("\n")}\n`;
  }
  if (!options.modifyPath) {
    lines.push("  已按 --no-modify-path 跳过持久 PATH 修改。");
  } else if (result.ok && result.changed) {
    lines.push(`  已自动将 ${dir} 置于当前用户 PATH 前面。`);
  } else if (result.ok) {
    lines.push(`  ${dir} 已位于当前用户 PATH 前面。`);
  } else {
    lines.push(`  警告：未能自动更新当前用户 PATH：${result.error}`);
  }

  if (!options.modifyPath || !result.ok) {
    lines.push(`  如需持久生效，请在 Windows“编辑账户的环境变量”中将 ${dir} 移到用户 Path 第一位。`);
  }
  if (result.ok && result.changed && result.broadcasted === false) {
    lines.push("  警告：持久 User PATH 已写入，但未能通知桌面环境；请注销并重新登录 Windows 后再验证。");
  }
  if (!dirFirstOnPath(dir, env, true)) {
    lines.push("  注意：当前 PowerShell/Windows Terminal 窗口无法由安装器自动刷新 PATH。");
    lines.push("  请复制执行下面的命令，让当前窗口立即使用新版本：");
    lines.push(`    $env:Path = ${quotePowerShell(`${dir};`)} + $env:Path`);
    lines.push("  然后验证：");
    lines.push("    gitcode version");
    lines.push("  或者关闭全部 PowerShell/Windows Terminal 窗口后重新打开，再运行 gitcode version。");
  } else {
    lines.push("  请运行 gitcode version 验证当前版本。");
  }
  lines.push("  其他 pip/npm 安装入口不会被自动删除；如需清理，请先运行 gitcode doctor install 确认来源。");
  return `${lines.join("\n")}\n`;
}

async function runInstall(args = []) {
  if (args.length === 1 && ["-h", "--help"].includes(args[0])) {
    process.stdout.write(installHelp());
    return;
  }
  const options = parseInstallArgs(args);
  const home = os.homedir();
  const isWin = process.platform === "win32";

  if (!isSupported(process.platform, process.arch)) {
    const guidance = process.platform === "win32" && process.arch === "arm64"
      ? "Windows arm64 is not shipped yet; build from source with Go."
      : "Use a release channel that explicitly lists this OS and architecture.";
    throw new Error(
      `no bundled binary for ${process.platform}/${process.arch}. ${guidance} ` +
        `https://gitcode.com/gitcode-cli/cli/releases`
    );
  }

  const src = bundledBinaryPath();
  if (!fs.existsSync(src)) {
    throw new Error(
      `bundled binary missing at ${src}. The npm package may be incomplete; ` +
        `reinstall ${require("../package.json").name}.`
    );
  }
  ensureExec(src);

  const dir = options.targetDir || chooseGlobalBinDir(home, isWin);
  fs.mkdirSync(dir, { recursive: true });

  const dst = path.join(dir, isWin ? "gc.exe" : "gc");
  const alias = path.join(dir, isWin ? "gitcode.exe" : "gitcode");
  const helper = path.join(dir, "gitcode-update-helper.js");
  const aliasOptions = isWin ? {} : { allowedSymlinkTarget: dst };
  const transactionID = `${process.pid}-${crypto.randomBytes(8).toString("hex")}`;
  const transaction = [];
  let versionLine;
  try {
    transaction.push(replacePath(src, dst, transactionID));
    transaction.push(replacePath(src, alias, transactionID, aliasOptions));
    transaction.push(replacePath(BOOTSTRAP_HELPER, helper, transactionID));
    if (sha256(src) !== sha256(dst) || sha256(src) !== sha256(alias)) {
      throw new Error("installed binary checksum verification failed");
    }

    const v = runGc(dst, ["version"]);
    if (v.status !== 0) {
      throw new Error(`installed binary health check failed: ${(v.stderr || "unknown error").trim()}`);
    }
    versionLine = (v.stdout || "").split("\n")[0] || "(gc version failed)";
    writeInstallMetadata(dir, {
      distribution: "npm-bootstrap",
      version: pkg.version,
      targetDir: dir,
      node: process.execPath,
      npm: process.env.npm_execpath || "",
      helper,
      sha256: sha256(src),
    });
    commitTransaction(transaction);
  } catch (error) {
    try {
      rollbackTransaction(transaction);
    } catch (rollbackError) {
      throw new AggregateError(
        [error, rollbackError],
        "installation failed and rollback was incomplete",
        { cause: error }
      );
    }
    throw error;
  }

  // Completions (posix only; Windows shell completion differs).
  const completions = isWin ? [] : installCompletions(dst, home);
  const windowsPathResult = isWin && options.modifyPath
    ? persistWindowsUserPath(dir)
    : { ok: true, changed: false };

  process.stdout.write(`Installed gc and gitcode to ${dir}\n`);
  process.stdout.write(`  ${versionLine}\n`);
  if (completions.length) {
    process.stdout.write(`Shell completions installed:\n`);
    for (const c of completions) process.stdout.write(`  ${c}\n`);
  } else if (isWin) {
    process.stdout.write(`Shell completions: skipped on Windows. Run "gc completion bash|powershell" manually if needed.\n`);
  } else {
    process.stdout.write(`Shell completions: skipped (none writable). Run "gc completion bash|zsh|fish" manually.\n`);
  }

  // PATH registration and guidance.
  if (isWin) {
    process.stdout.write(windowsPathGuidance(dir, options, windowsPathResult));
  } else if (dir === path.join(home, ".local", "bin")) {
    if (!dirOnPath(dir)) {
      process.stdout.write(
        `\nAdd ${dir} to your PATH:\n` +
          `  echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc  # or ~/.zshrc\n`
      );
    }
  }
  process.stdout.write(`\nRun "${isWin ? "gitcode" : "gc"} --help" to get started.\n`);
}

module.exports = {
  runInstall, chooseGlobalBinDir, commitTransaction, completionTarget, dirFirstOnPath, dirOnPath,
  installHelp, parseInstallArgs, persistWindowsUserPath, prependWindowsUserPath,
  quotePowerShell, replacePath, rollbackTransaction, validateWindowsPathDirectory, windowsPathGuidance,
};
