# @gitcode-cli/cli

The open-source [GitCode CLI](https://gitcode.com/gitcode-cli/cli) distributed via npm with **bundled multi-platform binaries** (Linux x64/ARM64, macOS x64/ARM64, Windows x64). No separate binary download is needed after the bootstrap or global installation.

GitCode CLI (`gc` / `gitcode`) is the community-developed, MIT-licensed command line tool for GitCode — bringing repositories, issues, pull requests, releases and Actions back to the terminal for developers, scripts, and AI agents. It is an independent open-source project, not published by the GitCode platform team.

## Install

### One-line bootstrap (no global npm install)

```bash
npx -y @gitcode-cli/cli@latest install
```

The short command is intended for trusted user environments and inherits the current npm configuration. For CI, audits, untrusted project directories, custom registries, or a custom target directory, use the hardened form:

```bash
npx --yes --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@latest install
npx --yes --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@latest install --target-dir /custom/bin
```

Copies the platform binary to a global bin dir (`/usr/local/bin` if writable, else `~/.local/bin`). On Linux/macOS it also installs bash/zsh/fish completions.
If an older installation left a same-directory `gitcode -> gc` alias, bootstrap migrates that verified alias transactionally; links to any other target are never overwritten.
On Windows it installs both `gc.exe` and `gitcode.exe`, then prepends the install directory to the persistent user PATH. Pass `--no-modify-path` to opt out. An npx child process cannot refresh the already-running PowerShell process, so the installer prints explicit Chinese instructions for refreshing `$env:Path` immediately or closing all PowerShell/Windows Terminal windows before reopening. It never changes the machine PATH or removes another package manager's entry. Use `gitcode` in PowerShell because `gc` is the built-in `Get-Content` alias.

An explicit `--target-dir` replaces regular `gc`/`gitcode` files inside that directory. Do not point it at Python Scripts, an npm prefix, or another package manager-owned directory.

Do not use `npm i @gitcode-cli/cli` as a CLI installation command. It only adds a dependency to the current project's `node_modules` and does not replace an existing `gitcode` earlier on PATH.

### Global npm install

```bash
npm install -g --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@latest
gitcode version
```

## Installation diagnostics and updates

```bash
gitcode doctor install
gitcode doctor install --json
gitcode update --check
gitcode update
```

Global npm and npm-bootstrap installations default to `notify`: after a normal command, a detached helper checks the stable `latest` tag from the official npm registry at most once every 24 hours, then prompts on the next launch without installing. Run `gitcode update` explicitly, or opt in to automatic application with `update.mode auto`. Updates use a minimal child environment, `--ignore-scripts`, a process lock, health check, and rollback. Registry, permission, or offline failures never change the completed command's exit code.

```bash
gitcode config set update.mode auto
gitcode config set update.mode notify
gitcode config set update.mode off
GC_NO_UPDATE_CHECK=1 gitcode version
```

`CI=true` and `--no-interactive` disable background checks. The updater touches only `@gitcode-cli/cli`; it never invokes pip, Homebrew, apt, dnf, or rpm, and it never rewrites PATH. If another `gitcode` is earlier on PATH, npm's non-failing install check and `doctor install` report the exact candidates and remediation choices.

## Supported platforms

| OS | Arch |
| --- | --- |
| Linux | x64, arm64 |
| macOS | x64, arm64 |
| Windows | x64 |
| OpenHarmony | arm64 |

OpenHarmony (arm64) reuses the bundled `gc-linux-arm64` binary: the OpenHarmony standard system runs the Linux kernel and gc ships fully static binaries, so no separate download is needed. Node.js reports `process.platform === "openharmony"` there; official upstream support is experimental, so use a Node.js build for OpenHarmony.

Windows arm64 is not shipped by npm, wheel, or release archives yet; build from source with Go for that target. On other unsupported combinations, use only a channel that explicitly lists the OS/architecture: https://gitcode.com/gitcode-cli/cli/releases

## Links

- Repository: https://gitcode.com/gitcode-cli/cli
- Issues: https://gitcode.com/gitcode-cli/cli/issues
- Command reference: https://gitcode.com/gitcode-cli/cli/blob/main/docs/COMMANDS.md
- Feishu/Lark notifications: https://gitcode.com/gitcode-cli/cli/blob/main/docs/LARK.md

## License

MIT
