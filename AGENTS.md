# AGENTS.md

本文件是 Codex 和其他通用代理在 gitcode-cli 仓库中的项目级入口文档，同时提供高层架构概览，帮助代理快速理解代码结构。

如果任务涉及代码、文档、流程、评审或发布，请先阅读：

1. [spec/README.md](./spec/README.md)
2. [docs/README.md](./docs/README.md)
3. [README.md](./README.md)

## 1. 入口职责

`AGENTS.md` 的职责是：

- 为 Codex 提供仓库级入口
- 指向正式规范、用户文档和后续 skill 分层
- 约束代理不得绕过项目正式规则
- 提供高层架构概览，便于任务定位

`AGENTS.md` 不是项目规则源。

项目正式规则以 [spec/README.md](./spec/README.md) 和 `spec/` 目录中的规范文档为准。

## 2. 项目概览

**GitCode CLI**（命令名 `gc`，Windows PowerShell 下等价于 `gitcode`）是用 Go 实现的 GitCode 平台命令行工具，参考 GitHub CLI 的设计，为人类终端交互与 AI 代理/脚本自动化提供统一入口。

- 入口：`cmd/gc/main.go` → `pkg/cmd/root/root.go`
- 框架：`github.com/spf13/cobra`
- API：GitCode REST API v5（默认 host `api.gitcode.com`）
- 模块路径：`gitcode.com/gitcode-cli/cli`
- Go 版本：1.21+（README 标注 1.22+）

### 2.1 顶层目录

| 目录 | 职责 |
|------|------|
| `cmd/gc/` | 程序入口，注入版本信息并执行 root 命令 |
| `api/` | GitCode API 客户端、查询函数、HTTP 重试与时间处理 |
| `pkg/cmd/` | 各命令实现（按领域分子目录：`auth`、`repo`、`issue`、`pr`、`commit`、`label`、`milestone`、`release`、`precommit`、`api`、`schema`、`version`、`help`、`root`） |
| `pkg/cmdutil/` | 命令共享工厂（`Factory`）、认证、确认、错误、输出与仓库解析 |
| `pkg/config/` | 配置与认证持久化（`~/.config/gc/`） |
| `pkg/iostreams/` | 终端 IO、颜色与 TTY 检测 |
| `pkg/output/` | 文本/表格/JSON 输出格式化 |
| `pkg/precommit/` | pre-commit 检测、安装与运行 |
| `pkg/browser/` | 浏览器打开 |
| `pkg/testutil/` | 测试工具（roundtrip 等） |
| `git/` | 本地 git 仓库检测与当前仓库/分支解析 |
| `completions/` | shell 补全脚本（bash/zsh/fish） |
| `scripts/` | 构建、打包、回归、风险分级、AI 记录校验等脚本 |
| `spec/` | 项目正式规范唯一来源 |
| `docs/` | 用户文档 |
| `issues-plan/` | 阶段说明（可能滞后，非事实源） |
| `gc_cli/` | Python wheel 包装层，分发内置全平台二进制 |
| `npm/` | npm 包包装层（`@gitcode-cli/cli`），Node wrapper + 内置多平台二进制 + `install` / 自动更新 / PATH 诊断引导 |
| `api-doc/` | gc-api-doc submodule 副本（GitCode API 端点/path/body 权威真相源，§5.9.B 查证用；锁定 commit，需定期 `git submodule update --remote api-doc` sync）|

### 2.2 命令结构约定

每条命令遵循 `pkg/cmd/<domain>/<command>.go` + `<command>_test.go` 模式，子命令各自独立目录。命令通过 `cmdutil.Factory` 注入 `IOStreams`、`HttpClient`、`Config`、`BaseRepo`、`Branch`，参见 `spec/foundations/command-template.md`。

### 2.3 API 客户端

- `api/client.go`：`Client` 封装 HTTP 调用、token 注入、host 解析
- `api/http_client.go`：HTTP 客户端工厂（`ParseTimeoutFromEnv`、`IsDebugEnabled`、`NewHTTPClientWithRetry`）
- `api/retry.go`：HTTP 重试中间件（`RetryConfig`、`DefaultRetryConfig`、`RetryMiddleware`）
- `api/queries_*.go`：按领域分组的查询函数（issue、pr、repo、commit、release、label_milestone、user）
- `api/flexible.go`：灵活字段解析
- `api/time.go`：时间解析与格式化
- 官方 OpenAPI 参考仓：`api-doc/` submodule（`git submodule update --init` 获取；锁定 commit，需定期 `git submodule update --remote api-doc` sync），端点/path/body 权威真相源；仓不可达时 fallback 临时 clone `git@gitcode.com:gitcode-cli/gc-api-doc.git`。修复涉及 API 调用时先查证，见 [开发工作流程 §5.9.B](./spec/workflows/development-workflow.md#59-横切编排资产)

### 2.4 文档分层

| 文档 | 角色 |
|------|------|
| `spec/` | 项目正式规范唯一来源（编码、测试、安全、门禁、流程、交付、治理，含 SIG 治理与 RFC 流程） |
| `spec/governance/sigs/*.yaml` | SIG 元数据唯一真相源（角色、范围、scope_labels 映射、Agent 登记） |
| `docs/COMMANDS.md` | 命令行为唯一真相源 |
| `docs/AUTH.md`、`docs/PACKAGING.md`、`docs/REGRESSION.md` | 认证、打包、回归说明 |
| `docs/SIGS.md`、`docs/rfcs/` | SIG 与 RFC 的贡献者导航（规则本体在 `spec/governance/sig-governance.md`、`spec/workflows/rfc-workflow.md`） |
| `docs/AI-GUIDE.md` | 外部项目通过 AI 使用 `gc` 的说明，不定义本仓库内部流程 |
| `README.md`、`AGENTS.md`、`CLAUDE.md` | 入口导航，非规则源 |
| `issues-plan/PROGRESS.md` | 阶段说明，可能滞后 |
| `gitcode-cli/skills` | 独立的 GitCode CLI Skills 真相源和分发仓库 |
| `.ai/skills/` | 已废弃的仓内共享源路径，不再使用或追踪 |
| `.claude/skills/`、`.codex/skills/` | 客户端本地运行时目录，不纳入本仓库追踪 |

冲突时优先级见 [spec/governance/source-of-truth-matrix.md](./spec/governance/source-of-truth-matrix.md)：`spec/` > `docs/COMMANDS.md` > 远端平台事实/`origin/main` > CI 运行结果 > 独立 Skills 仓 > 入口文档。

## 3. 构建与命令

### 3.1 标准 Build

```bash
# 日常开发验证（推荐）
go build -o ./gc ./cmd/gc

# Makefile 构建（注入完整版本 ldflags）
make build          # 当前平台，产物 bin/gc
make build-all      # linux/darwin/windows 多平台
make clean          # 清理 bin/、dist/、coverage
```

版本信息：`go build` 从 `debug.ReadBuildInfo()` 自动取 git commit/time；`make build` 通过 `-ldflags` 注入 `main.version`、`main.commit`、`main.date`。

**macOS 注意**：`Makefile` 的 `LDFLAGS` 始终使用 `-s -w`，未做 macOS 适配。`-s` 会剥离符号表导致 macOS dyld 找不到 `LC_UUID`，使 `bin/gc version` 崩溃。macOS 构建请使用 `scripts/build.sh` 或 CI 工作流（见 commit `3b66a04`），它们在 Darwin 上只用 `-w`。

### 3.2 测试与检查

```bash
make test              # go test -v -race -coverprofile=coverage.out ./...
make test-coverage     # 生成 HTML 覆盖率报告
make lint              # golangci-lint run ./...
make fmt               # go fmt ./...
make check             # test + lint
```

### 3.3 打包与发布

```bash
./scripts/package.sh <version> [release|linux|deb|rpm|pypi]
make release-local     # goreleaser 本地快照
make release           # goreleaser 正式发布（需 tag）
make completions       # 生成 shell 补全
```

产物边界：`gc`、`bin/`、`dist/`、`build/`、`*.deb`、`*.rpm`、`*.whl`、`*.egg-info/` 均为本地产物，不得提交。版本号遵循语义化版本 `vMAJOR.MINOR.PATCH[-PRERELEASE]`。

包版本以根 `VERSION` 为基线，通过 `scripts/sync-package-version.sh` 同步 nFPM、Python 与 npm metadata。正式 npm tarball 必须由 release `artifacts` job 从 GoReleaser 标准二进制组装并纳入统一 checksum，`npm` publish job 不得独立重编译。

### 3.4 Docker

```bash
make docker-build
make docker-run        # 需 export GC_TOKEN=... 后再执行
docker compose up gc
```

### 3.5 开发辅助

全新宿主机先直接运行平台脚本（此时可能还没有 Make）：

```bash
# Linux / macOS：安装核心验证依赖
bash scripts/dev-setup.sh
```

```powershell
# Windows PowerShell：安装核心验证依赖
powershell -ExecutionPolicy Bypass -File scripts\dev-setup.ps1
```

Make 已可用后，可使用统一入口：

```bash
make dev-setup         # 宿主机安装核心验证依赖（无需容器，跨平台）
make dev-doctor        # 只检查依赖，不安装；有缺口时非零退出
make deps              # go mod download + tidy
make update-deps       # go mod tidy + go get -u ./...
make dev               # go run ./cmd/gc

# 校验工具
make validate-ai-templates                           # 校验 docs/ai-templates/*.md
make validate-ai-record FILE=... KIND=...            # 校验单条 AI 记录
make classify-change-risk BASE=origin/main           # 改动风险分级
make verify-remote-facts REPO=owner/repo [ISSUE=1] [PR=2] [HEAD_SHA=<sha>]
```

宿主机脚本不安装打包/发布工具；`nfpm`、`goreleaser`、Python build 等完整
打包依赖继续使用 `.devcontainer/` 或发布流程中记录的手工安装路径。

### 3.6 远端 CI（GitCode 原生 + GitHub 镜像）

CI 同时运行在 **GitCode Actions**（主仓 `gitcode.com/gitcode-cli/cli`）和 **GitHub Actions**
（镜像仓 `github.com/gitcode-cli/cli`）。GitCode 原生工作流定义在 `.gitcode/workflows/ci.yml`，
对齐 GitHub CI 的 Linux 路径；GitHub 工作流定义在 `.github/workflows/ci.yml`，继续承担
Linux/macOS/Windows 跨平台验证。正式规范见 [spec/delivery/ci-workflows.md](./spec/delivery/ci-workflows.md)。

**触发**：PR 提交或更新到 `main` 时自动触发（`on: pull_request: branches: [main]`），无需手动操作。

**Job 映射**：

| Job | GitCode Actions | GitHub Actions | 对应门禁 |
|-----|-----------------|----------------|---------|
| `lint` | Ubuntu latest | Ubuntu | 编码规范 |
| `security` / `secret-scan` | Ubuntu latest（security） | Ubuntu（secret-scan） | gitleaks 凭证泄漏扫描 |
| `test` | Ubuntu latest | Ubuntu / macOS / Windows | 单元测试 + 竞态 + 覆盖率 |
| `build` | Ubuntu latest | Ubuntu / macOS / Windows | Linux / 跨平台构建 |
| `package` / `docker` | Ubuntu latest（package） | Ubuntu（docker） | 补全 + wheel 入口冒烟 / Docker |

依赖：`lint`、`security` 与 `test` 并行 → GitCode `build`、`package` 和 GitHub `build`、`docker`
等待 `test` 通过后执行；任一 Job 失败即整体失败。

**AI 通过 `gc` / `gh` 监控**：

```bash
# GitCode 主仓原生 CI
gc actions run list -R gitcode-cli/cli --pr <pr-number> --workflow "CI" --json
gc actions run view <run-id> -R gitcode-cli/cli --json
gc actions job list <run-id> -R gitcode-cli/cli --json

# GitHub 镜像仓跨平台 CI
gh run list --workflow=ci.yml --branch <pr-branch> --limit 1
gh run watch $(gh run list --workflow=ci.yml --branch <pr-branch> --limit 1 --json databaseId --jq '.[0].databaseId')
gh run view <run-id> --json conclusion --jq '.conclusion'
gh run view <run-id> --log --job=<job-id>
```

**证据纳入**：PR 自检须分别记录已执行平台的 CI run ID/URL、PR head SHA、结论和各 Job
状态摘要；失败时记录原因与修复过程。CI 不覆盖真实命令验证、安全审查、文档同步、独立执行主体
评审。docs-only 改动可跳过 CI，但须在自检中说明。

**Release CI**：`.github/workflows/release.yml` 由 AI 在发布流程中通过 `gh workflow run release.yml -f version=vX.Y.Z` 触发，详见 [spec/delivery/release-process.md](./spec/delivery/release-process.md)。

## 4. 代码风格

正式规则见 [spec/foundations/coding-standards.md](./spec/foundations/coding-standards.md)，要点：

- 使用 `gofmt`，行宽上限 120，单函数不超过 50 行，嵌套不超过 3 层，参数不超过 5 个
- 包名小写简短，不使用下划线或驼峰；导出名称用 PascalCase，内部用 camelCase；单方法接口以 `-er` 结尾
- import 分组：标准库 → 第三方 → 内部包（`gitcode.com/gitcode-cli/cli/...`），组间空行
- 文件内容顺序：`package` → `import` → 常量 → 类型 → 构造函数 → 公开方法 → 私有方法 → 辅助函数
- 错误：简单错误用 `errors.New`，包装用 `fmt.Errorf("failed to ...: %w", err)`；错误信息小写开头、无句号、描述发生了什么
- 导出函数/类型必须注释，注释以名称开头
- 禁止循环依赖、滥用全局变量、不符合 Go 习惯的命名

命令实现必须复用 `cmdutil` 共享能力（统一输出、认证、确认、错误类型），不应每条命令重复发明 I/O 与错误处理逻辑。

## 5. 测试

正式规则见 [spec/foundations/testing-guide.md](./spec/foundations/testing-guide.md) 与 [spec/workflows/test-workflow.md](./spec/workflows/test-workflow.md)，要点：

- 测试文件与源文件同目录，命名 `<source>_test.go`；函数以 `Test` 开头，推荐表格驱动测试
- 覆盖率：新功能 ≥ 70%，核心模块 ≥ 80%
- 命令行为变更必须至少做一个真实命令验证，且**只能使用 `infra-test/*` 仓库**（首选 `infra-test/gctest1`）
- 禁止使用个人仓库、其他组织仓库或 `gitcode-cli/cli` 自身测试
- 优先执行核心回归脚本：`./scripts/regression-core.sh`
- 真实命令验证前需先完成认证（如人工 `gc auth login` 或自行管理的环境变量），并用 `./gc auth status` 验证；脚本和 AI 代理不得读取、打印或转存真实 token
- Mock/Stub 通过接口实现，参考 `pkg/testutil/roundtrip.go`

回归矩阵与最小稳定回归集见 [docs/REGRESSION.md](./docs/REGRESSION.md)。

## 6. 安全

正式规则见 [spec/foundations/security.md](./spec/foundations/security.md)，要点：

- 禁止硬编码 token/密钥；通过环境变量或本地配置管理认证
- Token 优先级：`GC_TOKEN` > `GITCODE_TOKEN` > 本地配置（`~/.config/gc/auth.json`）；环境变量始终覆盖本地配置
- AI 代理不得调用 `gitcode auth token`、`gitcode auth status --show-token`、读取 `~/.config/gc/auth.json` 或打印 `GC_TOKEN` / `GITCODE_TOKEN`；完整 token 只能由人工在交互式 TTY 中输入 hostname 确认后显示
- 禁止提交：`*.pem`、`*.key`、`*.p12`、`*.pfx`、`id_rsa*`、`id_ed*`、`.env*`、`*.secret`、`credentials.json`、`token.txt`、`*.token`、`secrets.y*ml`
- 提交前自检：无硬编码凭证、无敏感文件被追踪、测试与文档不含真实凭证
- 不得在 issue/PR/comment/discussion/commit/release 的内容中含敏感信息（token 值、密钥、私钥、安全漏洞细节、PoC、攻击细节）；gc 在 `--body`/`--body-file`/`--comment`/`--comment-file`/`--description`/`--description-file`/`--notes`/`--notes-file` 提交前会扫描当前 `GC_TOKEN`/`GITCODE_TOKEN` 值（`cmdutil.ScanContentForSecrets`），但 AI 代理仍须自觉避免任何敏感信息进入提交内容
- `git add` 必须逐个精确指定文件，禁止 `git add -A` / `git add .`；提交前必须 `git status` + `git diff --staged` 复核暂存区只含本次任务相关文件（详见 [spec/foundations/security.md](./spec/foundations/security.md) 本地防线与提交纪律）
- 禁止 `git commit --no-verify` / `git push --no-verify`；新 clone 后必须执行 `pre-commit install --hook-type pre-commit --hook-type pre-push`（钩子不随 clone 自动安装）；gitleaks 误报须登记 `.gitleaksignore`，不得绕过钩子
- CI/CD 使用 Secrets；PyPI 发布使用 Trusted Publishing（OIDC）
- 安全漏洞通过提交私密 Issue 报告（标记为私有），不在公开 Issue/PR/comment 中披露漏洞细节、PoC 或攻击细节

### 6.1 Agent-Friendly CLI 契约

正式规则见 [spec/foundations/agent-friendly-cli.md](./spec/foundations/agent-friendly-cli.md)：

- 高频只读命令优先支持 `--json`；JSON 只写 stdout，字段名直接映射 API/领域模型
- 非 TTY 环境不得隐式阻塞等待输入；无法继续时立即报错并提示 `--yes`
- 破坏性命令默认有确认保护，`--yes` 跳过；非 TTY 且未显式跳过时必须立即失败
- 全局 `--no-interactive` flag 可主动声明非交互模式（供 AI 代理/脚本/管道），设置后所有 `CanPrompt` 返回 false，确认立即失败需 `--yes`
- 退出码：`0` 成功、`1` 通用错误、`2` 参数错误、`3` 资源不存在、`4` 认证/权限错误、`5` 资源冲突
- `--help` 也是代理发现接口，须含用途、示例、关键 flag、是否支持 `--json`、非交互限制

## 7. 配置

### 7.1 认证配置

- 环境变量：`GC_TOKEN`（推荐）、`GITCODE_TOKEN`
- 本地配置：`gc auth login`（交互）或 `gc auth login --with-token`（stdin）
- 配置目录：`~/.config/gc/`，认证文件 `auth.json`
- 查看：`gc auth status`

### 7.2 CLI 配置

`pkg/config/config.go` 定义配置接口，允许的 key：`browser`、`editor`、`pager`、`update.mode`。配置按 host 维度存储，`ConfigEntry` 标注来源（`environment`/`config`/`default`）。`update.mode` 仅控制 npm global / npm-bootstrap 的 `auto`、`notify`、`off` 更新策略；不得借此调用其他包管理器。

### 7.3 API 行为配置

- 默认 host：`api.gitcode.com`（`api.gitcode.com`/`gitcode.com` 互转见 `apiHostForGitCodeHost`）
- 默认 API 版本：`v5`
- HTTP 超时：`api.ParseTimeoutFromEnv()`
- 重试：`api.DefaultRetryConfig()`，`api.IsDebugEnabled()` 时输出 `[api]` 日志到 stderr
- 命令名解析：环境变量 `GITCODE_CLI_COMMAND_NAME` → 可执行文件名 → 平台默认（Windows 为 `gitcode`，其他为 `gc`）

### 7.4 工作区

当前位于 git worktree 中，主分支为 `main`，不得在 `main` 直接开发。分支命名与 PR/Issue 状态机见 [spec/workflows/development-workflow.md](./spec/workflows/development-workflow.md)。

## 8. 必读文档

先读 [spec/README.md](./spec/README.md)，再按任务进入对应规范，不要机械顺序通读全部文档。

常用任务入口：

- 改命令行为：`spec/workflows/development-workflow.md`、`spec/governance/docs-governance.md`、`spec/foundations/code-quality-gates.md`
- 改 agent / script 可消费性：`spec/foundations/agent-friendly-cli.md`、`spec/foundations/code-quality-gates.md`
- 改 API / auth / config：`spec/foundations/coding-standards.md`、`spec/foundations/security.md`、`spec/foundations/testing-guide.md`
- 补测试或做真实命令验证：`spec/foundations/testing-guide.md`、`spec/workflows/test-workflow.md`
- 提交 PR / 做 review：`spec/workflows/pr-workflow.md`、`spec/workflows/review-workflow.md`
- 改 SIG 治理 / 发起大型设计提案（RFC）：`spec/governance/sig-governance.md`、`spec/workflows/rfc-workflow.md`
- 改构建 / 打包 / 发布：`spec/delivery/build-and-package.md`、`spec/delivery/release-process.md`

具体流程任务再进入：

- [Issue 流程](./spec/workflows/issue-workflow.md)
- [PR 流程](./spec/workflows/pr-workflow.md)
- [评审流程](./spec/workflows/review-workflow.md)
- [测试流程](./spec/workflows/test-workflow.md)

## 9. 核心执行规则

代理在本仓库中必须遵守：

- 项目命令固定为 `gc`
- 项目正式规范以 `spec/` 为准
- 命令行为以 [docs/COMMANDS.md](./docs/COMMANDS.md) 为准
- GitCode CLI Skills 以独立仓库 [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills) 为准，本仓库不维护客户端 Skill 副本
- 项目阶段说明可参考 [issues-plan/PROGRESS.md](./issues-plan/PROGRESS.md)，但该文档可能滞后，不作为单个 issue / PR 实时状态真相源
- 流程推进以 `spec/workflows/*` 定义的状态机为准，不能只把 checklist 当完成标准
- 判断"某个 issue / 功能是否已合入主干"时，必须以 merged PR 和 `origin/main` 为准，不能只依据 issue 状态、issue comment、release 文案或功能分支存在与否
- 如果 issue 已关闭但没有 merged PR 或 `origin/main` 不包含对应代码，必须明确判定为"未完成主干合入"
- 创建 PR 时 body 必须含 `Closes #XXX`（或 `Fixes #XXX`/`Resolves #XXX`）关联对应 issue（修复型 PR）；非修复型 PR（重构、文档等）使用 `Refs #XXX` 仅引用不关闭；commit message 的 `Closes` 不被 GitCode 识别为自动关闭，PR body 的 `Closes #XXX` 会在 merge 后自动关闭 issue（见 [spec/workflows/pr-workflow.md](./spec/workflows/pr-workflow.md) §5）
- **GitCode 对 PR body 里的裸 `#NNN`（即无 `Closes`/`Fixes`/`Resolves`/`Refs` 等关键字前缀的 `#NNN`）也触发自动关闭**：PR body 自检/描述/未覆盖项文本不得出现裸 `#NNN`（除 `Closes #XXX` 行外），如需引用其他 issue 用文字描述（如"后续 run view 交付"）
- 外部项目使用 AI 操作 GitCode 的说明以 `docs/AI-GUIDE.md` 为准，但该文档不定义本仓库内部开发流程
- 代码或流程变化后必须同步检查相关文档
- 实际命令测试只能使用 `infra-test/*`
- 涉及 GitCode API 调用的代码改动（新增/修改 `api/` 包函数、端点实测报错排查）必须先查 `api-doc/` submodule（`git submodule update --init` 后本地可读；锁定 commit 可能滞后，需定期 `git submodule update --remote api-doc` sync）确认端点/path/body，不类比 GitHub CLI、不猜语义、不绕过（如改用 curl+token 猜测）；详见 [开发工作流程 §5.9.B](./spec/workflows/development-workflow.md#59-横切编排资产)
- 不得在 `main` 直接开发
- 不得提交构建产物、评估输出或真实凭证
- 不得在缺少验证记录、自检证据或独立执行主体评审的情况下宣称"已完成"
- 遇到规范未覆盖的情况，必须先向用户确认，不得自由发挥
- 发现规范之间有冲突，必须先向用户报告，以用户确认为准
- MEMORY.md（如存在）是会话级记忆摘要，如果与 spec/ 冲突，以 spec/ 为准
- 遇到网络问题（TLS timeout / handshake failed 等）优先多次重试标准工具（gh/git）；多次重试仍失败必须报告用户由人工解决，不得自行绕过（如改用 curl+token、切换下载源、换工具等）；严禁提取 `gh auth token` 走 curl/脚本调 API（token 会泄露到进程列表/命令历史/日志，gh 已封装认证）

## 10. Codex 入口边界

当前仓库内的 Codex 项目级入口是：

- `AGENTS.md`

GitCode CLI Skills 已全部迁移到独立仓库 [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills)。
`.ai/skills/` 已废弃；`.codex/skills/` 和 `.claude/skills/` 仅可作为不入库的本地运行时目录。

Codex 应先以 `spec/` 和本文件为主要入口。

## 11. 常用入口

- 用户文档入口：[docs/README.md](./docs/README.md)
- 命令手册：[docs/COMMANDS.md](./docs/COMMANDS.md)
- 认证说明：[docs/AUTH.md](./docs/AUTH.md)
- 回归说明：[docs/REGRESSION.md](./docs/REGRESSION.md)
- Docker 安装和使用：[README.md](./README.md)
- 打包说明：[docs/PACKAGING.md](./docs/PACKAGING.md)
- 发布说明：[RELEASE.md](./RELEASE.md)
- 贡献说明：[CONTRIBUTING.md](./CONTRIBUTING.md)
- Claude 入口：[CLAUDE.md](./CLAUDE.md)
