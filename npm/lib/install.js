// Bootstrap ("install" subcommand) for the npm-distributed CLI wrapper.
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
const { commandCandidates, normalizePath, writeInstallMetadata } = require("./install-metadata");
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
  "@{ changed = [bool]$changed; kind = $kind.ToString(); broadcasted = [bool]$broadcasted; previous = [string]$current } | ConvertTo-Json -Compress",
].join("; ");

function bundledBinaryPath() {
  return path.join(PLATFORMS_DIR, resolveBinaryName(process.platform, process.arch));
}

/**
 * Choose a global bin dir. Prefer a no-sudo writable location; fall back to a
 * per-user dir under the home directory. Pure (no FS side effects beyond the
 * write probe on the candidate dir).
 */
function chooseGlobalBinDir(home, isWin, posixCandidates, brewCellarPath = "/usr/local/Cellar") {
  if (isWin) {
    return path.join(home, "AppData", "Local", "gitcode-cli", "bin");
  }
  let candidates = posixCandidates || ["/usr/local/bin", path.join(home, ".local", "bin")];
  // On Intel Macs /usr/local is Homebrew's domain: writing there upsets
  // brew doctor and blocks a future "brew install gc". Skip the candidate
  // when the Homebrew layout is present.
  try {
    fs.statSync(brewCellarPath);
    const filtered = candidates.filter((dir) => dir !== "/usr/local/bin");
    if (filtered.length) candidates = filtered;
  } catch {
    // No Homebrew layout; the candidates stand.
  }
  for (const dir of candidates) {
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
  try {
    fs.mkdirSync(dir, { recursive: true });
  } catch (error) {
    throw new Error(
      `cannot create install directory ${dir}: ${error.message}\n` +
        `choose another directory with --target-dir <dir>`
    );
  }
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

// Builds the content transform used when copying the bootstrap update helper
// into the bin directory. The helper must stay standalone (Node built-ins
// only): the dynamic `require("../package.json")` from the source tree is
// rewritten to a literal so the copy resolves outside the package tree.
function helperPackageNameTransform(name) {
  const marker = 'const pkg = require("../package.json");\nconst PACKAGE = pkg.name;';
  return (content) => {
    if (!content.includes(marker)) {
      throw new Error("bootstrap update helper package-name marker not found");
    }
    return content.replace(marker, `const PACKAGE = ${JSON.stringify(name)};`);
  };
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
    // ELOOP (self/mutual symlink loops) falls through to the refusal path
    // like ENOENT/EINVAL so users get the actionable refusal message
    // instead of a raw errno.
    if (["ENOENT", "EINVAL", "ELOOP"].includes(error.code)) return false;
    throw error;
  }
}

// npm coordinates owned by this project. A bin symlink resolving into one of
// these package trees is a leftover of a classic `npm install -g` channel and
// is safe to migrate to the bootstrap layout.
// Org-transfer note: if the bare name "gitcode-cli" is ever added here, the
// path-level check below cannot distinguish a pre-existing third-party
// install from a new official one and would silently adopt the third-party
// entry. At transfer time, adoption must additionally read the resolved
// target's package.json "name" and verify it.
const OWN_NPM_PACKAGES = ["@atomgit-cli/cli", "@gitcode-cli/cli", "atomgit-cli"];
const THIRD_PARTY_NPM_PACKAGE = "gitcode-cli";

function resolvesIntoOwnNpmPackage(linkPath) {
  let resolved;
  try {
    resolved = fs.realpathSync(linkPath);
  } catch (error) {
    if (error.code !== "ENOENT") {
      // ELOOP/EACCES/ENOTDIR and friends: fall through to the refusal path,
      // which renders an actionable message instead of a raw errno.
      return false;
    }
    // A broken link still counts when its text points into one of our own
    // package trees (leftover after the package was uninstalled). The text is
    // resolved lexically here; a stale link has no live target to protect, so
    // over-matching on crafted text is acceptable by design.
    try {
      resolved = path.resolve(path.dirname(linkPath), fs.readlinkSync(linkPath));
    } catch {
      return false;
    }
  }
  const normalized = resolved.split(path.sep).join("/");
  return OWN_NPM_PACKAGES.some((name) => {
    const marker = `/node_modules/${name}/`;
    const index = normalized.indexOf(marker);
    // Only a top-level global layout counts (`<prefix>/node_modules/<pkg>/`):
    // the matched node_modules must be the outermost one, so links inside the
    // nested dependency trees of other packages are never hijacked.
    return index >= 0 && !normalized.slice(0, index).includes("/node_modules/");
  });
}

// pnpm's global layout nests our own npm package under .../pnpm/.../node_modules/,
// which resolvesIntoOwnNpmPackage would treat as a classic npm-global leftover
// and silently migrate. Check the raw link text and the resolved path for a
// pnpm marker before any adoption decision and refuse instead, keeping the
// pnpm channel intact (mirrors the Homebrew/pipx symlink refusals).
function pnpmChannelSymlinkError(dst) {
  let raw;
  try {
    raw = fs.readlinkSync(dst);
  } catch {
    return null;
  }
  let resolved = "";
  try {
    resolved = fs.realpathSync(dst);
  } catch {
    // Broken link: the raw text still carries the pnpm marker.
  }
  const surface = `${raw}\n${resolved}`.split(path.sep).join("/").replace(/\\/g, "/");
  // Two markers: the default global layout (.../pnpm/global/...) and the
  // store layout (/.pnpm/<name>@<version>/...) — the store marker survives
  // custom PNPM_HOME naming. Keep in sync with isPnpmEnvironment's layout
  // marker in install-metadata.js.
  if (!surface.includes("/pnpm/global/") && !surface.includes("/.pnpm/")) return null;
  let coordinate = pkg.name;
  for (const name of OWN_NPM_PACKAGES) {
    if (surface.includes(`/node_modules/${name}/`)) {
      coordinate = name;
      break;
    }
  }
  const detail = resolved && resolved !== raw ? ` (resolves to ${resolved})` : "";
  return new Error(
    `refusing non-regular install target: ${dst} is a symlink -> ${raw}${detail}\n` +
      `this symlink belongs to a pnpm global installation; run "pnpm remove -g ${coordinate}" first, ` +
      `or keep pnpm and skip the npm bootstrap install`
  );
}

// yarn v1 global bin links resolve into ~/.config/yarn/global/node_modules/
// <own coordinate>/... — the same shape as a classic npm install, so the
// adoption check below would silently migrate (and destroy) the yarn
// channel entry. Refuse first, mirroring the pnpm defense.
function yarnChannelSymlinkError(dst) {
  let raw;
  try {
    raw = fs.readlinkSync(dst);
  } catch {
    return null;
  }
  let resolved = "";
  try {
    resolved = fs.realpathSync(dst);
  } catch {
    // Broken link: the raw text still carries the yarn marker.
  }
  const surface = `${raw}\n${resolved}`.split(path.sep).join("/").replace(/\\/g, "/");
  if (!surface.includes("/yarn/global/")) return null;
  let coordinate = pkg.name;
  for (const name of OWN_NPM_PACKAGES) {
    if (surface.includes(`/node_modules/${name}/`)) {
      coordinate = name;
      break;
    }
  }
  const detail = resolved && resolved !== raw ? ` (resolves to ${resolved})` : "";
  return new Error(
    `refusing non-regular install target: ${dst} is a symlink -> ${raw}${detail}\n` +
      `this symlink belongs to a yarn global installation; run "yarn global remove ${coordinate}" first, ` +
      `or keep yarn and skip the npm bootstrap install`
  );
}

// Channel-specific guidance for foreign symlinks, matched against both the
// raw link text and the resolved path.
const FOREIGN_SYMLINK_CHANNEL_HINTS = [
  { marker: "/Cellar/", guidance: 'this symlink belongs to a Homebrew installation; run "brew uninstall gc" first, or keep Homebrew and skip the npm bootstrap install' },
  { marker: "/opt/homebrew/", guidance: 'this symlink belongs to a Homebrew installation; run "brew uninstall gc" first, or keep Homebrew and skip the npm bootstrap install' },
  { marker: "/uv/tools/", guidance: 'this symlink belongs to a uv-managed tool; run "uv tool uninstall gitcode-cli" first, or keep uv and skip the npm bootstrap install' },
  { marker: "/yarn/global/", guidance: 'this symlink belongs to a yarn global installation; run "yarn global remove @gitcode-cli/cli" first, or keep yarn and skip the npm bootstrap install' },
  { marker: "/pipx/venvs/", guidance: 'this symlink belongs to a pipx installation; run "pipx uninstall gitcode-cli" first, or remove the symlink' },
];

function nonRegularTargetError(dst) {
  try {
    const raw = fs.readlinkSync(dst);
    let resolved = "";
    try {
      resolved = fs.realpathSync(dst);
    } catch {
      // Keep the raw link text only (broken link).
    }
    let detail = `: ${dst} is a symlink -> ${raw}`;
    if (resolved && resolved !== raw) detail += ` (resolves to ${resolved})`;
    // Windows readlink/path results use backslashes; normalize for matching.
    const surface = `${raw}\n${resolved}`.split(path.sep).join("/").replace(/\\/g, "/");
    let guidance;
    if (surface.includes(`/node_modules/${THIRD_PARTY_NPM_PACKAGE}/`)) {
      guidance = `the third-party npm package "${THIRD_PARTY_NPM_PACKAGE}" is not AtomGit CLI; ` +
        `run "npm uninstall -g ${THIRD_PARTY_NPM_PACKAGE}" (check "npm prefix -g"), or remove the symlink`;
    } else {
      const channelHint = FOREIGN_SYMLINK_CHANNEL_HINTS.find((hint) => surface.includes(hint.marker));
      guidance = channelHint
        ? channelHint.guidance
        : "remove the symlink and run install again, or install to another directory with --target-dir";
    }
    return new Error(`refusing non-regular install target${detail}\n${guidance}`);
  } catch {
    // Non-symlink non-regular target (e.g. a directory): keep the plain form.
    return new Error(`refusing non-regular install target: ${dst}`);
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
    if (stat.isSymbolicLink()) {
      const pnpmError = pnpmChannelSymlinkError(dst);
      if (pnpmError) throw pnpmError;
      const yarnError = yarnChannelSymlinkError(dst);
      if (yarnError) throw yarnError;
      if (!isAllowedAliasSymlink(dst, options.allowedSymlinkTarget) && !resolvesIntoOwnNpmPackage(dst)) {
        throw nonRegularTargetError(dst);
      }
      moveOriginal = true;
    } else if (!stat.isFile()) {
      const kind = stat.isDirectory() ? "a directory" : "not a regular file";
      throw new Error(
        `refusing non-regular install target: ${dst} is ${kind}; ` +
          `remove it or choose another directory with --target-dir <dir>`
      );
    } else {
      // Refuse foreign-channel regular files (e.g. pip console scripts):
      // replacing would destroy the other channel's entry point on commit.
      const hint = foreignChannelHint(dst);
      if (hint) throw foreignChannelTargetError(dst, hint);
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
  if (options.transform) {
    fs.writeFileSync(temp, options.transform(fs.readFileSync(src, "utf8")), { flag: "wx", mode: 0o755 });
  } else {
    fs.copyFileSync(src, temp, fs.constants.COPYFILE_EXCL);
  }
  ensureExec(temp);
  let backupReady = false;
  try {
    if (hadOriginal) {
      if (moveOriginal) {
        fs.renameSync(dst, backup);
        backupReady = true;
        const pnpmError = pnpmChannelSymlinkError(backup);
        if (pnpmError) throw pnpmError;
        const yarnError = yarnChannelSymlinkError(backup);
        if (yarnError) throw yarnError;
        if (!isAllowedAliasSymlink(backup, options.allowedSymlinkTarget) && !resolvesIntoOwnNpmPackage(backup)) {
          throw nonRegularTargetError(backup);
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
  return spawnSync(bin, args, {
    encoding: "utf8",
    // A blocked child (AV scanning the fresh binary, hung shell) must not
    // hang the whole install indefinitely.
    timeout: 30000,
    env: {
      ...process.env,
      // The installer's own probes must not run the update lifecycle: the
      // spawned binary reads the adjacent bootstrap manifest and would
      // consume any pending summary into discarded stderr, mark the
      // first-run notice shown, and spawn background updaters mid-install
      // (up to four per install). The binary pointer targets the bundled
      // platforms copy — no manifest sits next to it, so the lifecycle
      // exits early even before the env guard applies (AfterCommand shows
      // the summary before checking the disabled flags).
      GC_NO_UPDATE_CHECK: "1",
      GITCODE_CLI_BINARY: bundledBinaryPath(),
    },
  });
}

// A failing command sometimes reports on stdout with an empty stderr; fall
// back to the first non-empty stdout line before the generic message.
function commandFailureDetail(result, fallback) {
  const stderr = String(result.stderr || "").trim();
  if (stderr) return stderr;
  const stdoutLine = String(result.stdout || "").split(/\r?\n/).map((line) => line.trim()).find(Boolean);
  return stdoutLine || fallback;
}

// Replace a completion file atomically and never write through a symlink:
// an existing `_gc` symlink could point anywhere in the user's tree, and a
// plain overwrite would clobber the target outside our directory.
function writeCompletionFile(target, content) {
  try {
    const stat = fs.lstatSync(target);
    if (stat.isSymbolicLink()) {
      return { skipped: true };
    }
  } catch {
    // Target does not exist yet.
  }
  const temp = `${target}.tmp-${process.pid}-${crypto.randomBytes(4).toString("hex")}`;
  fs.writeFileSync(temp, content, { mode: 0o644, flag: "wx" });
  try {
    renameReplace(temp, target);
  } catch (error) {
    try {
      fs.unlinkSync(temp);
    } catch {
      // Preserve the original error.
    }
    throw error;
  }
  return { skipped: false };
}

function installCompletions(bin, home) {
  const installed = [];
  const skipped = [];
  for (const shell of ["bash", "zsh", "fish"]) {
    const res = runGc(bin, ["completion", shell]);
    if (res.status !== 0 || !res.stdout) {
      skipped.push(`${shell}: completion command failed`);
      continue;
    }
    const target = completionTarget(shell, home);
    if (!target) {
      skipped.push(`${shell}: no completion target`);
      continue;
    }
    try {
      fs.mkdirSync(path.dirname(target), { recursive: true });
      const written = writeCompletionFile(target, res.stdout);
      if (written.skipped) {
        skipped.push(`${shell}: ${target} is a symlink`);
        continue;
      }
      installed.push(`${shell}: ${target}`);
    } catch {
      skipped.push(`${shell}: ${target} not writable`);
    }
  }
  return { installed, skipped };
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

// Test-only model of the PowerShell PATH update: the production logic is the
// WINDOWS_UPDATE_USER_PATH script above (run via persistWindowsUserPath).
// No production caller — it exists so tests can lock the PowerShell behavior
// without spawning powershell.exe. When that script changes, update this
// model or the tests silently drift.
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

// Shadowing report for a fresh Windows install at `dir`: which command names
// resolve to an earlier provider on the merged PATH. New Windows processes
// concatenate System PATH before User PATH, and the installer only prepends
// `dir` to the User PATH — so a provider inside the old User PATH
// (`userPathValue`) is overtaken in new windows, while a provider anywhere
// else (System PATH, shell profile injection) keeps winning regardless.
// `userPathValue === null` means the old User PATH is unknown.
function windowsPathShadowing(dir, env, userPathValue) {
  const shadows = [];
  const wanted = normalizePath(dir, true);
  const userEntries = new Set();
  if (typeof userPathValue === "string") {
    for (const entry of userPathValue.split(";")) {
      const unquoted = entry.trim().replace(/^"|"$/g, "");
      if (!unquoted) continue;
      userEntries.add(normalizePath(expandWindowsEnvironment(unquoted, env), true));
    }
  }
  for (const name of ["gitcode", "gc"]) {
    const provider = commandCandidates(name, env, true)[0] || "";
    if (!provider) continue;
    const providerDir = normalizePath(path.win32.dirname(provider), true);
    if (providerDir === wanted) continue;
    shadows.push({
      name,
      provider,
      scope: typeof userPathValue === "string"
        ? (userEntries.has(providerDir) ? "user" : "system")
        : "unknown",
    });
  }
  return shadows;
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
    // The script itself waits up to 30s on the global PATH mutex; 90s
    // bounds a hung PowerShell without cutting a legitimate slow wait.
    timeout: 90000,
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
      previousUserPath: typeof parsed.previous === "string" ? parsed.previous : null,
    };
  } catch (error) {
    return { ok: false, error: `解析 Windows PATH 更新结果失败：${error.message}` };
  }
}

function parseInstallArgs(args) {
  const options = { targetDir: "", modifyPath: true };
  for (let index = 0; index < args.length; index += 1) {
    const arg = args[index];
    if (arg === "--target-dir") {
      const value = args[index + 1];
      index += 1;
      // In the space-separated form a value starting with "--" is almost
      // certainly a missed flag (known or not); single-dash directory names
      // are still expressible as ./-name or --target-dir=-name.
      if (!value || value.startsWith("--")) {
        throw new Error(
          "--target-dir requires a directory value (use ./-name or --target-dir=-name for dash-prefixed names)"
        );
      }
      options.targetDir = path.resolve(value);
      continue;
    }
    if (arg.startsWith("--target-dir=")) {
      const value = arg.slice("--target-dir=".length);
      if (!value) {
        throw new Error("--target-dir requires a directory value");
      }
      options.targetDir = path.resolve(value);
      continue;
    }
    if (arg === "--no-modify-path") {
      options.modifyPath = false;
      continue;
    }
    throw new Error(`unknown install argument: ${arg}`);
  }
  return options;
}

function quotePowerShell(value) {
  return `'${String(value).replace(/'/g, "''")}'`;
}

function installHelp() {
  // Both bin names route to this wrapper; show the one the platform's
  // users actually type (PowerShell reserves "gc" as Get-Content).
  const invoked = process.platform === "win32" ? "gitcode" : "gc";
  return [
    "Install bundled gc and gitcode binaries outside the npm package directory.",
    "",
    "Usage:",
    `  ${invoked} install [--target-dir <directory>] [--no-modify-path]`,
    "",
    "Flags:",
    "  --target-dir <directory>  Install into an explicit directory (also --target-dir=<directory>)",
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
  // Deeper shadowing analysis on the merged PATH: an earlier provider keeps
  // winning in new windows unless it lives inside the old User PATH.
  const userPathValue = result.ok && options.modifyPath && typeof result.previousUserPath === "string"
    ? result.previousUserPath
    : null;
  for (const shadow of windowsPathShadowing(dir, env, userPathValue)) {
    if (shadow.scope === "user") {
      lines.push(`  注意：${shadow.provider} 先于本安装目录提供 "${shadow.name}"。`);
      lines.push("  该提供者位于用户 PATH，重新打开 PowerShell/Windows Terminal 窗口后本安装将优先生效。");
      lines.push("  若同一提供者同时位于系统 PATH，重开后仍由其优先生效；请运行 gitcode doctor install 确认全部来源。");
    } else if (shadow.scope === "system") {
      lines.push(`  警告：PATH 中 "${shadow.name}" 解析到 ${shadow.provider}，重开窗口也无法解决（该提供者位于系统 PATH 或 shell 配置，合并顺序在用户 PATH 之前）。`);
      lines.push("  请卸载旧提供者，或由管理员将本安装目录加入系统 PATH 并置于其前；运行 gitcode doctor install 查看全部来源。");
    } else {
      lines.push(`  警告：${shadow.provider} 先于本安装目录提供 "${shadow.name}"。`);
      lines.push("  若该提供者来自系统 PATH，重开窗口无法解决：请运行 gitcode doctor install 确认来源，再卸载旧提供者或调整 PATH 顺序。");
    }
  }
  lines.push("  其他 pip/npm 安装入口不会被自动删除；如需清理，请先运行 gitcode doctor install 确认来源。");
  return `${lines.join("\n")}\n`;
}

// Detect a foreign-channel regular file occupying an install target (e.g. a
// pip console script). Returns "" when nothing recognizable is found.
function foreignChannelHint(file, isWin = process.platform === "win32") {
  let content = "";
  try {
    const fd = fs.openSync(file, "r");
    try {
      // 1 MiB covers distlib's launcher stubs (~100 KB+); the embedded
      // shebang sits far past any 4 KB window.
      const buf = Buffer.alloc(1 << 20);
      const bytes = fs.readSync(fd, buf, 0, buf.length, 0);
      content = buf.toString("utf8", 0, bytes);
    } finally {
      fs.closeSync(fd);
    }
  } catch {
    return "";
  }
  const firstLine = content.split("\n", 1)[0];
  // Python shebangs may carry interpreter options (python3 -E, env -S
  // python3); only options are accepted after the interpreter, never a
  // script path (the file itself is the script).
  if (/^#!\s*(?:(?:\S*\/)?env(?:\s+-\S+)*\s+python|\S*python)([0-9.]*)?(?:\s+-\S+)*\s*$/.test(firstLine)) {
    return 'python script (likely a pip console script); run "pip uninstall gitcode-cli" first to keep the pip channel';
  }
  // Windows pip console shims are PE binaries with no shebang line at the
  // top: distlib appends "#!<python.exe path>\r\n" well into the stub, in
  // front of the zip payload. Anchor on the embedded shebang so ordinary
  // PE binaries never match.
  if (isWin && content.startsWith("MZ") && /#![^\r\n]{0,400}pythonw?\.exe/i.test(content)) {
    return 'pip console executable (PE shim); run "pip uninstall gitcode-cli" first to keep the pip channel';
  }
  return "";
}

// Refusal error for a foreign-channel regular file occupying an install
// target (e.g. a pip console script). The replacement would be transactional,
// but the backup is deleted on commit, so replacing would permanently destroy
// the other channel's entry point.
function foreignChannelTargetError(dst, hint) {
  return new Error(
    `refusing to replace foreign install target: ${dst} appears to be a ${hint}\n` +
      `install to another directory with --target-dir <dir> to keep both channels`
  );
}

// First directory on PATH providing the given command name (resolved through
// symlinks; broken links are skipped).
function firstProviderOnPath(name, env = process.env) {
  const dirs = (env.PATH || "").split(path.delimiter).map((d) => d.trim()).filter(Boolean);
  for (const dir of dirs) {
    const candidate = path.join(dir, name);
    try {
      if (fs.statSync(candidate).isFile()) return candidate;
    } catch {
      // keep scanning
    }
  }
  return "";
}

// Fail early with actionable guidance when the install dir cannot be created
// or written (mkdir on an existing dir skips write checks, so probe too).
function ensureUsableInstallDir(dir) {
  try {
    fs.mkdirSync(dir, { recursive: true });
    const probe = path.join(dir, `.gc-install-probe-${process.pid}`);
    fs.writeFileSync(probe, "");
    fs.unlinkSync(probe);
  } catch (error) {
    throw new Error(
      `install directory is not usable: ${dir} (${error.message})\n` +
        `check the path and permissions, or choose another directory with --target-dir <dir>`
    );
  }
}

// Cross-process install lock for the bin directory. Without it two
// concurrent installs with different transaction IDs can interleave: A
// renames the target to its backup, B sees a missing target and installs
// fresh, then A's late rollback restores the old version over B's winner.
// The lock name intentionally matches no leftover prefix — a crashed
// install leaves it behind and stale reclaim (not the sweep) is the
// recovery path.
const INSTALL_LOCK_STALE_MS = 10 * 60 * 1000;

function acquireInstallLock(dir, now = Date.now()) {
  const lock = path.join(dir, ".gc-install-lock");
  const claim = () => {
    fs.writeFileSync(lock, `${process.pid} ${new Date(now).toISOString()}\n`, { flag: "wx", mode: 0o644 });
    return lock;
  };
  try {
    return claim();
  } catch (error) {
    if (error.code !== "EEXIST") throw error;
    let mtimeMs = 0;
    try {
      mtimeMs = fs.statSync(lock).mtimeMs;
    } catch {
      return claim(); // the holder released between our failure and stat
    }
    if (now - mtimeMs < INSTALL_LOCK_STALE_MS) {
      throw new Error(
        `another gc install appears to be running in ${dir} (lock: ${lock}); ` +
          `retry in a moment or remove the lock if you are certain none is running`
      );
    }
    // Atomic reclaim: renaming the stale lock to a private name lets exactly
    // one of two racing reclaimers win — the loser's rename fails with
    // ENOENT (the winner already retired it) and its claim then hits the
    // winner's fresh lock. Unlinking the shared path instead would delete
    // each other's fresh locks and let both believe they hold it.
    let stoleFreshLock = false;
    try {
      const retired = `${lock}.retired-${process.pid}-${crypto.randomBytes(4).toString("hex")}`;
      fs.renameSync(lock, retired);
      // Re-verify staleness on the retired file: a fresh install's claim
      // may have landed between the check above and this rename, and
      // renaming a live lock away would silently break its holder.
      if (now - fs.statSync(retired).mtimeMs <= INSTALL_LOCK_STALE_MS) {
        try {
          fs.renameSync(retired, lock);
        } catch {
          // Someone claimed the path while we held their lock aside.
        }
        stoleFreshLock = true;
      } else {
        try {
          fs.unlinkSync(retired);
        } catch {
          // Best effort; a stranded .retired-* file is inert debris.
        }
      }
    } catch {
      // Raced with another reclaimer and lost; fall through to the claim,
      // which fails with EEXIST on their fresh lock below.
    }
    if (stoleFreshLock) {
      throw new Error(
        `another gc install appears to be running in ${dir} (lock: ${lock}); ` +
          `retry in a moment or remove the lock if you are certain none is running`
      );
    }
    try {
      return claim();
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
      // Both reclaimers raced: the other one won the recreated lock.
      throw new Error(
        `another gc install appears to be running in ${dir} (lock: ${lock}); ` +
          `retry in a moment or remove the lock if you are certain none is running`
      );
    }
  }
}

function releaseInstallLock(lock) {
  try {
    // Identity check via the pid token written at claim time: if this lock
    // was reclaimed as stale while we were stalled, the path now holds
    // another install's fresh lock and must not be removed.
    const holder = fs.readFileSync(lock, "utf8").trim().split(" ")[0];
    if (holder !== String(process.pid)) return;
  } catch {
    // The lock file is already gone; nothing to release.
    return;
  }
  try {
    fs.unlinkSync(lock);
  } catch {
    // Best effort: a stranded lock self-heals after INSTALL_LOCK_STALE_MS.
  }
}

// Files an interrupted (SIGKILL/Ctrl-C) bootstrap install can leave behind in
// the bin dir. Only regular files are swept, and only by mtime age: a symlink
// backup keeps its original mtime through rename, so age is unreliable there
// and active concurrent transactions must never be disturbed.
const LEFTOVER_PREFIXES = ["gc", "gc.exe", "gitcode", "gitcode.exe", "gitcode-update-helper.js", ".gitcode-install.json"];
const LEFTOVER_AGE_MS = 24 * 60 * 60 * 1000;

function isTransactionLeftoverName(name) {
  if (name.startsWith(".gc-install-probe-")) return true;
  if (name === ".gc-write-probe") return true;
  return LEFTOVER_PREFIXES.some((prefix) =>
    name.startsWith(`${prefix}.backup-`) || name.startsWith(`${prefix}.tmp-`));
}

function sweepTransactionLeftovers(dir, now = Date.now()) {
  let entries;
  try {
    entries = fs.readdirSync(dir);
  } catch {
    return 0;
  }
  let removed = 0;
  for (const entry of entries) {
    if (!isTransactionLeftoverName(entry)) continue;
    const file = path.join(dir, entry);
    try {
      const stat = fs.lstatSync(file);
      if (!stat.isFile() || now - stat.mtimeMs < LEFTOVER_AGE_MS) continue;
      fs.unlinkSync(file);
      removed += 1;
    } catch {
      // best-effort sweep; a failed unlink must not block installation
    }
  }
  return removed;
}

// Render an error chain (AggregateError causes) so users see the underlying
// errno and paths instead of only the aggregate message (issue #592).
function formatErrorChain(error) {
  const lines = [];
  const walk = (current, depth) => {
    const prefix = depth === 0 ? "" : "  ".repeat(depth) + "- ";
    lines.push(`${prefix}${current && current.message ? current.message : current}`);
    if (current && Array.isArray(current.errors)) {
      for (const inner of current.errors) walk(inner, depth + 1);
    }
  };
  walk(error, 0);
  return lines.join("\n");
}

async function runInstall(args = []) {
  if (args.includes("-h") || args.includes("--help")) {
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
        `https://gitcode.com/atomgit-cli/cli/releases`
    );
  }

  const src = bundledBinaryPath();
  if (!fs.existsSync(src)) {
    throw new Error(
      `bundled binary missing at ${src}. The npm package may be incomplete; ` +
        `reinstall ${pkg.name}.`
    );
  }
  ensureExec(src);

  const dir = options.targetDir || chooseGlobalBinDir(home, isWin);
  ensureUsableInstallDir(dir);
  // Hold the bin-directory lock across the sweep, the transaction, and any
  // rollback so concurrent installs cannot interleave (see acquireInstallLock).
  const installLock = acquireInstallLock(dir);
  const dst = path.join(dir, isWin ? "gc.exe" : "gc");
  const alias = path.join(dir, isWin ? "gitcode.exe" : "gitcode");
  const helper = path.join(dir, "gitcode-update-helper.js");
  const aliasOptions = isWin ? {} : { allowedSymlinkTarget: dst };
  const transactionID = `${process.pid}-${crypto.randomBytes(8).toString("hex")}`;
  const transaction = [];
  let versionLine;
  try {
    const swept = sweepTransactionLeftovers(dir);
    if (swept > 0) {
      process.stdout.write(`Swept ${swept} leftover file(s) from a previously interrupted install in ${dir}\n`);
    }
    transaction.push(replacePath(src, dst, transactionID));
    transaction.push(replacePath(src, alias, transactionID, aliasOptions));
    // The bootstrap update helper runs from the bin directory, outside the
    // package tree, so `require("../package.json")` cannot resolve there.
    // Inject the npm coordinate as a literal while copying.
    transaction.push(replacePath(BOOTSTRAP_HELPER, helper, transactionID, {
      transform: helperPackageNameTransform(pkg.name),
    }));
    if (sha256(src) !== sha256(dst) || sha256(src) !== sha256(alias)) {
      throw new Error("installed binary checksum verification failed");
    }

    const v = runGc(dst, ["version"]);
    if (v.status !== 0) {
      throw new Error(`installed binary health check failed: ${commandFailureDetail(v, "no error output")}`);
    }
    versionLine = (v.stdout || "").split("\n")[0] || "(gc version failed)";
    writeInstallMetadata(dir, {
      distribution: "npm-bootstrap",
      version: pkg.version,
      // Same coordinate injected into the helper copy: the Go side and the
      // helper both derive <state>/gitcode-cli/<package>/npm-bootstrap/
      // from it, so they must agree.
      package: pkg.name,
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
  } finally {
    releaseInstallLock(installLock);
  }

  // Completions (posix only; Windows shell completion differs).
  const completions = isWin ? { installed: [], skipped: [] } : installCompletions(dst, home);
  const windowsPathResult = isWin && options.modifyPath
    ? persistWindowsUserPath(dir)
    : { ok: true, changed: false };

  process.stdout.write(`Installed gc and gitcode to ${dir}\n`);
  process.stdout.write(`  ${versionLine}\n`);
  if (completions.installed.length) {
    process.stdout.write(`Shell completions installed:\n`);
    for (const c of completions.installed) process.stdout.write(`  ${c}\n`);
  } else if (isWin) {
    process.stdout.write(`Shell completions: skipped on Windows. Run "gc completion bash|powershell" manually if needed.\n`);
  } else {
    process.stdout.write(`Shell completions: none installed. Run "gc completion bash|zsh|fish" manually.\n`);
  }
  // Attribution for the skips: a failed completion command is a different
  // problem than an unwritable target.
  for (const s of completions.skipped) {
    process.stdout.write(`  skipped: ${s}\n`);
  }

  // PATH registration and guidance.
  if (isWin) {
    process.stdout.write(windowsPathGuidance(dir, options, windowsPathResult));
  } else {
    // Shadowing check: another provider earlier on PATH would win over the
    // fresh install (mirrors the npm-channel postinstall warning).
    for (const name of ["gc", "gitcode"]) {
      const provider = firstProviderOnPath(name);
      if (provider && path.resolve(path.dirname(provider)) !== path.resolve(dir)) {
        process.stdout.write(
          `Warning: PATH resolves "${name}" to ${provider}; the new installation is at ${dir}.\n` +
            `  Move ${dir} earlier on PATH, or remove the other provider (run "gc doctor install" for details).\n`
        );
      }
    }
    if (dir === path.join(home, ".local", "bin")) {
      if (!dirOnPath(dir)) {
        process.stdout.write(
          `\nAdd ${dir} to your PATH:\n` +
            `  echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc  # or ~/.zshrc\n`
        );
      }
    }
  }
  process.stdout.write(`\nRun "${isWin ? "gitcode" : "gc"} --help" to get started.\n`);
}

module.exports = {
  runInstall, chooseGlobalBinDir, commitTransaction, completionTarget, dirFirstOnPath, dirOnPath,
  ensureUsableInstallDir, acquireInstallLock, releaseInstallLock, writeCompletionFile,
  firstProviderOnPath, foreignChannelHint, foreignChannelTargetError, formatErrorChain,
  helperPackageNameTransform, installHelp, isTransactionLeftoverName, parseInstallArgs, persistWindowsUserPath,
  prependWindowsUserPath, pnpmChannelSymlinkError, quotePowerShell, replacePath, rollbackTransaction,
  sweepTransactionLeftovers, validateWindowsPathDirectory, windowsPathGuidance, windowsPathShadowing,
  yarnChannelSymlinkError,
};
