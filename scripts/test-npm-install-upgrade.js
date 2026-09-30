#!/usr/bin/env node
// Cross-platform real-install smoke: published old npm -> local package.

"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawnSync } = require("child_process");
const { resolveBinaryName } = require("../npm/lib/platform");

const OLD_VERSION = "0.10.3";
const TEST_VERSION = "9.9.9";
const root = path.resolve(__dirname, "..");
const temp = fs.mkdtempSync(path.join(os.tmpdir(), "gc-npm-upgrade-"));
const packageDir = path.join(temp, "package");
const prefix = path.join(temp, "prefix");
const oldBin = path.join(temp, "old-python-bin");
const bootstrap = path.join(temp, "bootstrap");
const npmCLI = process.env.npm_execpath;

if (!npmCLI) throw new Error("run through npm so npm_execpath identifies the trusted npm CLI");

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd || root,
    encoding: "utf8",
    timeout: options.timeout || 120000,
    windowsHide: true,
    env: { ...process.env, CI: "true", GC_NO_UPDATE_CHECK: "1", ...options.env },
    shell: options.shell || false,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(
      `${command} ${args.join(" ")} exited ${result.status}\n${result.stdout || ""}\n${result.stderr || ""}`
    );
  }
  return result;
}

function npm(args, options = {}) {
  return run(process.execPath, [npmCLI, ...args], options);
}

function npmBinDir() {
  return process.platform === "win32" ? prefix : path.join(prefix, "bin");
}

function npmEntrypoint() {
  return path.join(npmBinDir(), process.platform === "win32" ? "gitcode.cmd" : "gitcode");
}

function runEntrypoint(entrypoint, args, options = {}) {
  if (process.platform === "win32" && /\.cmd$/i.test(entrypoint)) {
    const name = require("../npm/package.json").name;
    const wrapper = path.join(prefix, "node_modules", ...name.split("/"), "bin", "gc.js");
    return run(process.execPath, [wrapper, ...args], options);
  }
  return run(entrypoint, args, options);
}

function writeShadow() {
  fs.mkdirSync(oldBin, { recursive: true });
  const target = path.join(oldBin, process.platform === "win32" ? "gitcode.cmd" : "gitcode");
  const content = process.platform === "win32" ? "@echo shadow-0.8.0\r\n" : "#!/bin/sh\necho shadow-0.8.0\n";
  fs.writeFileSync(target, content, { mode: 0o755 });
  return target;
}

function buildPackage() {
  fs.cpSync(path.join(root, "npm"), packageDir, { recursive: true });
  const platforms = path.join(packageDir, "bin", "platforms");
  fs.rmSync(platforms, { recursive: true, force: true });
  fs.mkdirSync(platforms, { recursive: true });
  const packageJSONPath = path.join(packageDir, "package.json");
  const packageJSON = JSON.parse(fs.readFileSync(packageJSONPath, "utf8"));
  packageJSON.version = TEST_VERSION;
  fs.writeFileSync(packageJSONPath, `${JSON.stringify(packageJSON, null, 2)}\n`);
  const binary = path.join(platforms, resolveBinaryName(process.platform, process.arch));
  run(
    "go",
    [
      "build",
      "-ldflags",
      `-X main.version=${TEST_VERSION} -X main.commit=ci-upgrade -X main.date=ci`,
      "-o",
      binary,
      "./cmd/gc",
    ],
    { cwd: root }
  );
  if (process.platform !== "win32") fs.chmodSync(binary, 0o755);
  const packed = npm(["pack", "--json", "--pack-destination", temp], { cwd: packageDir });
  const result = JSON.parse(packed.stdout);
  const record = Array.isArray(result) ? result[0] : result[packageJSON.name] || Object.values(result)[0];
  if (!record || !record.filename) throw new Error(`unexpected npm pack output: ${packed.stdout}`);
  return path.join(temp, record.filename);
}

function assertVersion(entrypoint, expected, env) {
  const result = runEntrypoint(entrypoint, ["version", "--json"], { env });
  const actual = JSON.parse(result.stdout).version;
  if (actual !== expected) throw new Error(`${entrypoint} reported ${actual}, expected ${expected}`);
}

// Each completion file must carry its own command name in its generated
// header. The patterns anchor the name — padded header line for bash/fish,
// exact line for zsh — so one command's header can never satisfy the
// other's check.
function assertCompletionFiles(home) {
  const checks = [
    [".local/share/bash-completion/completions/gc", /^# bash completion V2 for gc( |$)/],
    [".local/share/bash-completion/completions/gitcode", /^# bash completion V2 for gitcode( |$)/],
    [".zsh/completions/_gc", /^#compdef gc$/],
    [".zsh/completions/_gitcode", /^#compdef gitcode$/],
    [".config/fish/completions/gc.fish", /^# fish completion for gc( |$)/],
    [".config/fish/completions/gitcode.fish", /^# fish completion for gitcode( |$)/],
  ];
  for (const [relative, pattern] of checks) {
    const target = path.join(home, relative);
    if (!fs.existsSync(target)) throw new Error(`completion file missing after install: ${target}`);
    const firstLine = fs.readFileSync(target, "utf8").split("\n", 1)[0];
    if (!pattern.test(firstLine)) {
      throw new Error(`completion header mismatch at ${target}:\n${firstLine}`);
    }
  }
}

function main() {
  const tarball = buildPackage();
  fs.mkdirSync(prefix, { recursive: true });
  npm(["install", "-g", `${require("../npm/package.json").name}@${OLD_VERSION}`, "--prefix", prefix, "--no-audit", "--no-fund"]);
  assertVersion(npmEntrypoint(), OLD_VERSION);

  writeShadow();
  const migrationPath = `${oldBin}${path.delimiter}${npmBinDir()}${path.delimiter}${process.env.PATH || ""}`;
  const install = npm(
    [
      "install",
      "-g",
      tarball,
      "--prefix",
      prefix,
      "--foreground-scripts",
      "--dangerously-allow-all-scripts",
      "--no-audit",
      "--no-fund",
    ],
    { env: { PATH: migrationPath, Path: migrationPath } }
  );
  const installOutput = `${install.stdout}\n${install.stderr}`;
  if (!installOutput.includes("another command is first on PATH")) {
    throw new Error(`postinstall did not report the shadowing command:\n${installOutput}`);
  }
  assertVersion(npmEntrypoint(), TEST_VERSION, { PATH: migrationPath, Path: migrationPath });

  const doctor = runEntrypoint(npmEntrypoint(), ["doctor", "install", "--json"], {
    env: { PATH: migrationPath, Path: migrationPath },
  });
  const report = JSON.parse(doctor.stdout);
  if (report.distribution !== "npm" || !report.conflicts.some((item) => item.includes("npm global bin"))) {
    throw new Error(`doctor did not diagnose npm shadowing: ${doctor.stdout}`);
  }

  // Regression (#608): a user-exported GITCODE_CLI_COMMAND_NAME must not
  // leak into the gc-named completion files written by the bootstrap
  // install (the CLI resolves the command name env-first, overriding
  // argv[0]). Install under an isolated HOME with the name exported as
  // the opposite command and verify every file matches its own name.
  const completionHome = fs.mkdtempSync(path.join(os.tmpdir(), "gc-completion-home-"));
  // Empty strings neutralize any runner-exported XDG/BASH_COMPLETION vars so
  // the assertions below can pin the default HOME-relative target paths. The
  // install dir leads PATH so the just-installed binaries own both command
  // names — on a machine with an existing gc install the shadow check would
  // otherwise (correctly) skip every completion file.
  const completionEnv = process.platform === "win32"
    ? {}
    : {
        HOME: completionHome,
        GITCODE_CLI_COMMAND_NAME: "gitcode",
        XDG_DATA_HOME: "",
        XDG_CONFIG_HOME: "",
        BASH_COMPLETION_USER_DIR: "",
        PATH: `${bootstrap}${path.delimiter}${process.env.PATH || ""}`,
        Path: `${bootstrap}${path.delimiter}${process.env.Path || process.env.PATH || ""}`,
      };
  run(process.execPath, [path.join(packageDir, "bin", "gc.js"), "install", "--target-dir", bootstrap], {
    env: {
      PATH: process.env.PATH || "",
      Path: process.env.Path || process.env.PATH || "",
      ...completionEnv,
    },
  });
  const bootstrapEntry = path.join(bootstrap, process.platform === "win32" ? "gitcode.exe" : "gitcode");
  assertVersion(bootstrapEntry, TEST_VERSION);
  const manifest = JSON.parse(fs.readFileSync(path.join(bootstrap, ".gitcode-install.json"), "utf8"));
  if (manifest.distribution !== "npm-bootstrap" || !manifest.sha256 || !fs.existsSync(manifest.helper)) {
    throw new Error(`invalid bootstrap manifest: ${JSON.stringify(manifest)}`);
  }
  if (process.platform !== "win32") assertCompletionFiles(completionHome);

  process.stdout.write(`npm upgrade smoke passed: ${OLD_VERSION} -> ${TEST_VERSION} on ${process.platform}/${process.arch}\n`);
}

main();
