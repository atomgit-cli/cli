# Contributing to GitCode CLI

Thank you for your interest in contributing to GitCode CLI!

## Documentation

Before contributing, please read the following documentation:

### Core Documentation

| Document | Description |
|----------|-------------|
| [README.md](./README.md) | Project overview and installation guide |
| [CLAUDE.md](./CLAUDE.md) | AI-assisted development guide |
| [AGENTS.md](./AGENTS.md) | Codex and general agent entry |

### Development Specifications (spec/)

| Document | Description |
|----------|-------------|
| [编码规范](./spec/foundations/coding-standards.md) | Naming, file structure, error handling |
| [测试指南](./spec/foundations/testing-guide.md) | Unit tests, actual command testing |
| [命令开发模板](./spec/foundations/command-template.md) | Command development template |
| [安全规范](./spec/foundations/security.md) | Token management, sensitive info protection |

### Development Workflows (spec/workflows/)

| Document | Description |
|----------|-------------|
| [Issue 流程](./spec/workflows/issue-workflow.md) | Issue lifecycle management |
| [PR 流程](./spec/workflows/pr-workflow.md) | PR creation and merge |
| [评审流程](./spec/workflows/review-workflow.md) | Code review process |
| [测试流程](./spec/workflows/test-workflow.md) | Testing workflow |
| [开发流程](./spec/workflows/development-workflow.md) | End-to-end development workflow |
| [AI 本地开发流程](./spec/workflows/ai-local-development-workflow.md) | AI-assisted local development workflow |
| [构建打包规范](./spec/delivery/build-and-package.md) | Build and package |
| [Release 规范](./spec/delivery/release-process.md) | Release process |

### Other Documentation

| Document | Description |
|----------|-------------|
| [docs/COMMANDS.md](./docs/COMMANDS.md) | Command usage guide |
| [docs/PACKAGING.md](./docs/PACKAGING.md) | Packaging guide |
| [docs/AI-GUIDE.md](./docs/AI-GUIDE.md) | External-project AI usage guide for `gc` |
| [docs/AI-TEMPLATES.md](./docs/AI-TEMPLATES.md) | Internal gitcode-cli issue/PR comment templates |
| [source-of-truth-matrix.md](./spec/governance/source-of-truth-matrix.md) | What can and cannot be used as a fact source |
| [sig-governance.md](./spec/governance/sig-governance.md) | SIG governance: roles, lifecycle, decision rules |
| [rfc-workflow.md](./spec/workflows/rfc-workflow.md) | RFC process for large design proposals |

> **Important**: When modifying command-related code, you MUST sync updates to `docs/COMMANDS.md`.
>
> **Important**: When modifying development or review workflow rules, you MUST sync `spec/`, `AGENTS.md`, `CLAUDE.md`, related shared skills, and AI workflow templates.
>
> **Important**: Do not treat `issues-plan/PROGRESS.md` as the real-time source for single issue/PR status; use the remote platform, merged PRs, and `origin/main`.
>
> **Important**: When changing `docs/AI-TEMPLATES.md` or `docs/ai-templates/*.md`, run `make validate-ai-templates`.

## Workflow Expectations

- Development and review flow is state-based, not a loose checklist.
- Do not start coding on an issue before it reaches `status/verified`.
- Do not treat author self-check as an independent review.
- Do not close a fix issue before the related PR is merged to `main`.
- Use the reusable templates in `docs/AI-TEMPLATES.md` or `docs/ai-templates/*.md` for issue comments, PR self-check, and review outcomes.

## Development Setup

### Prerequisites

- Go 1.22+
- Make
- Docker (optional)
- GoReleaser (optional, for releases)

### Host Setup (no container)

The dev container below is convenient for IDE workflows, but local verification
does not require one. The host scripts install the core build, test, lint, and
secret-scanning baseline directly on the host. Packaging and release tooling
remains in the dev container and the documented release workflow:

```bash
# Linux / macOS
bash scripts/dev-setup.sh                    # install what is missing, then verify
bash scripts/dev-setup.sh --check            # report gaps only, install nothing
```

```powershell
# Windows (PowerShell)
powershell -ExecutionPolicy Bypass -File scripts\dev-setup.ps1
powershell -ExecutionPolicy Bypass -File scripts\dev-setup.ps1 -Check
```

After Make is available, the following convenience targets pick the right
script for the platform:

```bash
make dev-setup    # install host dependencies
make dev-doctor   # check only, exits non-zero when gaps remain
```

On a fresh host, invoke the platform script directly first. `make dev-setup`
cannot bootstrap Make itself because Make is needed to enter that target.

On Linux, install Go 1.22 or newer from
[go.dev/dl](https://go.dev/dl/) when it is not already available. The script
does not install a distribution `golang-go` package because supported
distribution releases may provide an older, unsupported Go version.

Both scripts are idempotent and install tooling only. They never read, write,
or print `GC_TOKEN` / `GITCODE_TOKEN`; the variables are removed from the
script process before package managers, compilers, or installed tools run.
Authentication stays a separate manual step.

Project-managed tools are isolated from shared user tool directories:

- Linux/macOS: `~/.local/share/gitcode-cli/dev-tools/bin`. The script does not
  edit shell profiles; add this directory to `PATH` before using managed tools
  in a later shell:
  `export PATH="$HOME/.local/share/gitcode-cli/dev-tools/bin:$PATH"`.
- Windows: `%LOCALAPPDATA%\gitcode-cli\dev-tools\bin`.

Neither script edits shell profiles or the user `PATH`. On Windows, add the
managed tools for the current shell with
`$env:Path = "$env:LOCALAPPDATA\gitcode-cli\dev-tools\bin;$env:Path"`.
Managed wrappers are never written over files outside the dedicated directory,
and the script refuses to replace an unrecognized wrapper inside it.

`--check` / `-Check` is offline and makes no persistent changes. It may return
non-zero on a clean host to report tools that the install mode would add.

#### Windows notes

Windows needs two fixups that `dev-setup.ps1` applies automatically. They are
listed here because they also bite when setting up by hand:

- GNU Make defaults to `cmd.exe` as `SHELL`, which breaks every shell-based
  recipe in the `Makefile`. The script writes a managed `make.cmd` wrapper that
  forces Git bash via an 8.3 short path.
- `python3` resolves to a WindowsApps stub that exits with code `9009` instead
  of running, so `make validate-ai-*` fails. The script creates a managed
  wrapper for the real Python interpreter.

The core setup also checks for a C compiler and runs a race-enabled Go test,
because the repository's standard test target uses `go test -race`.

Also build `./gc.exe` rather than `./gc` on Windows; PowerShell refuses to
execute an extensionless binary. When running `scripts/regression-core.sh`
under Git bash, point `GC_BIN` at the built executable.

If a download stalls behind a slow network, use the standard package manager
again or configure `GOPROXY`; do not pass project credentials to alternate
download scripts.

### Dev Container

If you use VS Code Dev Containers or GitHub Codespaces, the repository now includes
`.devcontainer/` for the local build, UT, packaging, and release-validation baseline.

The dev container provides:

- Go 1.22
- Python 3 + `build`, `wheel`, `setuptools`
- `nfpm` for DEB/RPM packaging
- `goreleaser` for snapshot release validation
- `rpm`, `make`, `git`, and standard shell tooling

For safety, the committed devcontainer config does not automatically forward local
`GC_TOKEN` or `GITCODE_TOKEN` into the container.

If you need real command verification, export the token manually only after you trust
the checked-out code:

```bash
export GC_TOKEN="your_gitcode_token"
```

After the container starts, the recommended minimum checks remain:

```bash
go test ./...
go build -o ./gc ./cmd/gc
./gc version
./scripts/regression-core.sh
```

Real command verification still requires a valid token and must only target
`infra-test/*` repositories.

### Building

```bash
# Clone the repository
git clone https://gitcode.com/gitcode-cli/cli.git
cd gitcode-cli

# Install dependencies
make deps

# Build
make build

# Run tests
make test

# Run
make run
```

### Development Commands

```bash
make build          # Build binary
make run            # Run application
make test           # Run tests
make test-coverage  # Run tests with coverage
make fmt            # Format code
make lint           # Run linter
make completions    # Generate shell completions
```

### Pre-commit

The repository now includes a lightweight [`.pre-commit-config.yaml`](./.pre-commit-config.yaml)
for local commit-time hygiene checks.

It is intentionally limited to fast, deterministic checks:

- trailing whitespace and EOF fixes
- YAML / JSON / TOML syntax checks
- merge-conflict marker detection
- large-file guard
- private-key detection
- `gofmt` on staged Go files

It does **not** replace the formal gates in `spec/` and does **not** run heavy checks such as:

- `go test ./...`
- `./scripts/regression-core.sh`
- `./scripts/package.sh`
- `make release-local`

Recommended usage:

```bash
pre-commit install
pre-commit run --files path/to/changed-file.go path/to/changed-file.md
```

Use `pre-commit run --all-files` only when you intentionally want a repository-wide
cleanup pass. The current repository still contains some historical files that would
trigger large unrelated formatting or EOF-only diffs in a full sweep.

If `pre-commit` is not installed on your machine, install it with your preferred package manager
or Python tooling first.

### Workspace Hygiene

Do not commit local build outputs, package artifacts, evaluation workspaces, or statistics files.

Common examples that must stay out of git:

```bash
build/
dist/
gc_linux_*
gc-darwin-*
*.deb
*.rpm
*.tar.gz
*.zip
*.egg-info/
issue-reviewer-workspace/
context/
statistics.json
project-management-statistics.json
```

The repository `.gitignore` already covers these paths. If a new local-only output appears repeatedly, update `.gitignore` in the same change.

## Packaging

### Binary

```bash
# Build for current platform
make build

# Build for all platforms
make build-all
```

### Docker

```bash
# Build Docker image
make docker

# Build with specific tag
make docker DOCKER_TAG=v1.0.0

# Run Docker container
make docker-run

# Multi-platform build
make docker-all
```

### Linux Packages (DEB/RPM)

```bash
# Install goreleaser
go install github.com/goreleaser/goreleaser/v2@latest

# Create snapshot release (local testing)
make release-local

# Check generated packages in dist/
```

### Homebrew

```bash
# Formula is generated by GoReleaser (see .goreleaser.yaml `brews:`)
# and pushed to gitcode-cli/homebrew-tap by the release workflow.
brew install gitcode-cli/homebrew-tap/gc
```

### Scoop (Windows)

```bash
# Manifest is in scoop/gc.json
# After release, users can install with:
scoop bucket add gitcode-com https://github.com/gitcode-com/scoop-bucket
scoop install gc
```

## Release Process

> 详细发布流程请参阅 [RELEASE.md](./RELEASE.md)。

### Quick Start

```bash
# 创建并推送标签触发自动发布
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions 自动执行：
# - 构建 RPM/DEB 包（x86_64, arm64）
# - 发布到 GitHub Release
# - 发布 PyPI 包 (pip install gitcode-cli)
```

### Required Secrets

| Secret | Description |
|--------|-------------|
| `GITHUB_TOKEN` | Automatically provided by GitHub |

> PyPI 使用 Trusted Publishing（OIDC），无需配置 API Token。

## Directory Structure

```
.
├── cmd/gc/              # Main application entry
├── pkg/                 # Public packages
│   ├── cmd/             # Command implementations
│   ├── cmdutil/         # Utilities for commands
│   ├── iostreams/       # I/O stream management
│   └── testutil/        # Testing utilities
├── internal/            # Private packages
├── api/                 # API client
├── git/                 # Git operations
├── completions/         # Shell completions
├── scripts/             # Package scripts
├── homebrew/            # Homebrew formula
├── scoop/               # Scoop manifest
├── .github/workflows/   # GitHub Actions
├── Dockerfile           # Docker configuration
├── docker-compose.yml   # Docker Compose
├── .goreleaser.yaml     # GoReleaser config
└── Makefile             # Build automation
```

## Code Style

- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Run `make fmt` before committing
- Run `make lint` to check for issues
- Write tests for new functionality

## Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Run linter: `make lint`
6. Commit with conventional commits
7. Push and create PR

## Commit Convention

We use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `test:` Test changes
- `refactor:` Code refactoring
- `chore:` Maintenance tasks

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
