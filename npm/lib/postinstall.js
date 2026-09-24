// Non-failing global-install diagnostics. Never edits PATH or other packages.

"use strict";

const path = require("path");
const pkg = require("../package.json");
const { isPnpmEnvironment, pathConflict, writeInstallMetadata } = require("./install-metadata");

function isGlobalInstall(env = process.env) {
  return env.npm_config_global === "true" || env.npm_config_global === true;
}

function globalInstallReport(env = process.env, isWin = process.platform === "win32") {
  const prefix = env.npm_config_prefix || "";
  if (!prefix) return null;
  return { prefix, ...pathConflict(prefix, env, isWin) };
}

function runPostinstall(env = process.env, stderr = process.stderr, packageRoot = path.resolve(__dirname, "..")) {
  if (!isGlobalInstall(env)) return;
  // pnpm global installs must never be recorded as npm installs: the npm
  // updater would then run through pnpm with npm-only flags and fail.
  if (isPnpmEnvironment(env, packageRoot, process.platform === "win32")) {
    writeInstallMetadata(packageRoot, {
      schema: 1,
      distribution: "pnpm",
      global: true,
      version: pkg.version,
      prefix: "",
      npm: "",
    });
  } else {
    writeInstallMetadata(packageRoot, {
      distribution: "npm",
      global: true,
      version: pkg.version,
      prefix: env.npm_config_prefix || "",
      npm: env.npm_execpath || "",
    });
  }

  const report = globalInstallReport(env);
  if (!report) return;
  if (report.shadowed) {
    stderr.write(
      `\nGitCode CLI ${pkg.version} was installed by npm, but another command is first on PATH:\n` +
        `  selected: ${report.selected}\n` +
        `  npm bin:  ${report.expectedDir}\n` +
        `Run the npm entry directly, then inspect all installations:\n` +
        (process.platform === "win32"
          ? `  & "${path.join(report.expectedDir, "gitcode.cmd")}" doctor install\n`
          : `  "${path.join(report.expectedDir, "gitcode")}" doctor install\n`) +
        `No package was removed and PATH was not changed.\n\n`
    );
  }
  if (process.platform === "win32") {
    stderr.write(
      `Windows PowerShell reserves "gc" as Get-Content; use "gitcode" for GitCode CLI.\n`
    );
  }
}

if (require.main === module) {
  try {
    runPostinstall();
  } catch (error) {
    process.stderr.write(`GitCode CLI install check skipped: ${error.message}\n`);
  }
}

module.exports = { globalInstallReport, isGlobalInstall, runPostinstall };
