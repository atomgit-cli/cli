# 本地构建与打包规范

本文件定义 gitcode-cli 的本地构建、打包和产物校验规则。

本规范面向开发者和 AI 协作者，目标是让本地交付流程可重复、可验证、可审查。

## 1. 适用范围

本规范适用于以下场景：

- 本地构建 `gc` 二进制
- 本地构建多平台二进制
- 本地构建 DEB / RPM / PyPI 包
- 本地验证构建产物
- 提交前检查本地构建目录和产物边界

本规范不定义 CI 自动化规则。CI 相关内容在后续 `spec/delivery/ci-workflows.md` 中单独维护。

## 2. 权威边界

- 构建和打包的正式规范以本文件为准
- 用户如何使用打包产物，以 `docs/PACKAGING.md` 为准
- 当前仓库的脚本实现以 `Makefile` 和 `scripts/package.sh`、`scripts/build.sh`、`scripts/build-release.sh` 为准
- 若脚本行为与本规范不一致，应优先修正脚本或同步更新本规范，不允许长期漂移

## 3. 标准构建命令

### 3.1 本地开发构建

开发阶段的标准本地构建命令为：

```bash
go build -o ./gc ./cmd/gc
```

该命令用于：

- 日常开发验证
- 实际命令测试
- issue 修复后的快速回归

### 3.2 Makefile 构建

仓库已提供以下标准构建入口：

```bash
make build
make build-all
make clean
make install PREFIX="$HOME/.local"
```

使用约束如下：

- `make build` 用于当前平台构建
- `make build-all` 用于多平台二进制构建
- `make clean` 用于清理 `bin/`、`dist/` 和覆盖率产物
- `make install` 同时安装 `gc` 和 `gitcode`；`PREFIX` 默认 `/usr/local`，可显式覆盖，`make uninstall` 只删除该 `PREFIX/bin` 下的两个入口

### 3.3 快照式发布构建

需要验证发布产物时，可使用：

```bash
make release-local
make release-snapshot
```

这些命令用于本地快照验证，不等同于正式 release。

## 4. 标准打包方式

### 4.1 推荐脚本

本地打包优先使用：

```bash
./scripts/package.sh <version> [target]
```

例如：

```bash
./scripts/package.sh v0.3.4 release
./scripts/package.sh v0.3.4 linux
./scripts/package.sh v0.3.4 deb
./scripts/package.sh v0.3.4 rpm
./scripts/package.sh v0.3.4 pypi
```

原因：

- 该脚本已经承担版本同步、二进制构建和打包收口
- 比手工调用零散工具更接近当前仓库真实发布流程

### 4.2 允许的产物类型

当前仓库允许生成以下产物：

- 当前平台本地二进制：`./gc`
- 多平台二进制：`bin/` 或 `dist/` 中的对应产物
- DEB 包：`gc_*.deb`
- RPM 包：`gc-*.rpm`
- PyPI 包：`gitcode_cli-*.whl`、`gitcode_cli-*.tar.gz`
- npm 包：`@gitcode-cli/cli`（`npm/` 目录的 Node wrapper + 内置多平台二进制 `npm/bin/platforms/gc-{linux,darwin}-{amd64,arm64}` 与 `gc-windows-amd64.exe`，由 release `artifacts` job 组装、`npm` job 发布）
- npm tarball：`gitcode-cli-cli-*.tgz`（由 release `artifacts` job 从 GoReleaser 标准二进制组装，纳入统一 SHA-256 清单）

### 4.3 统一版本与二进制来源

- 仓库根 `VERSION` 是 nFPM、Python 与 npm package metadata 的版本真相源；发布准备使用 `scripts/sync-package-version.sh <version>` 同步，CI / release preflight 使用 `--check` 阻止漂移。
- GoReleaser 生成唯一标准二进制集合。DEB/RPM、wheel、Homebrew、npm 和 archive 必须消费该集合，禁止 npm publish job 再次独立交叉编译。
- `scripts/prepare-npm-package.sh` 只接收已验证的 GoReleaser `dist/` 产物，运行 npm 单测并生成 `.tgz`。
- npm tarball 和其中二进制随其他 Release 资产进入 checksum；registry 已存在同版本时必须比对 tarball SHA-256 才可幂等跳过。

## 5. 构建前置条件

执行本地构建或打包前，必须确认：

- Go 环境可用
- 依赖已安装
- 需要 DEB / RPM 打包时，`nfpm` 已安装
- 需要 PyPI 打包时，Python build 工具已安装
- 需要真实命令验证时，已设置 `GC_TOKEN`

常用准备命令：

```bash
go mod tidy
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.41.3
pip install --upgrade build wheel setuptools
```

也可以直接使用宿主机依赖安装脚本，安装日常构建、竞态测试、lint 和
secret scan 所需的核心验证依赖：

```bash
# Linux / macOS
bash scripts/dev-setup.sh

# Windows
powershell -ExecutionPolicy Bypass -File scripts\dev-setup.ps1

# Make 已可用后的跨平台核心验证入口
make dev-setup
make dev-doctor
```

说明：

- 脚本只安装工具链，不读取也不打印 `GC_TOKEN` / `GITCODE_TOKEN`
- 脚本运行前从子进程环境移除 `GC_TOKEN` / `GITCODE_TOKEN`
- `--check` / `-Check` 离线检查且不做持久化修改，存在缺口时返回非零退出码
- 全新宿主机应先直接运行平台脚本，因为此时可能还没有 Make
- Linux 缺少 Go 时须从 `https://go.dev/dl/` 安装 Go 1.22+；脚本不安装发行版中可能过旧的 `golang-go`
- `.devcontainer/` 提供隔离、可复现的完整环境；宿主机脚本提供无需容器的本地验证路径
- 宿主机脚本不安装打包/发布工具；`nfpm`、`goreleaser` 和 Python build 工具继续按本节手工命令或 `.devcontainer/` 管理
- 两个平台脚本都不自动修改 shell profile 或用户 `PATH`；后续 shell 使用受管工具前，必须按脚本输出手工加入项目专属受管工具目录

## 6. 本地构建验证

### 6.1 最小验证

任何涉及命令行为的改动，在提交前至少完成：

```bash
go build -o ./gc ./cmd/gc
./gc version
```

### 6.2 功能验证

如果改动影响具体命令，必须继续做：

- 相关包级单元测试
- `go test ./...`
- 至少一个真实命令验证

真实命令验证只能使用 `infra-test/*` 仓库。

### 6.3 打包验证

如果改动影响构建、打包、版本信息或发布产物，必须继续验证：

- 构建脚本可执行
- 目标产物实际生成
- 产物命名与文档描述一致
- 至少抽样验证一个安装或运行路径

## 7. 产物边界

构建和打包产物默认属于本地产物，不应直接提交到仓库。

常见本地产物包括：

- `gc`
- `bin/`
- `dist/`
- `build/`
- `*.deb`
- `*.rpm`
- `*.tar.gz`
- `*.zip`
- `*.egg-info/`

提交前必须确认：

- 本地产物未被误提交
- `.gitignore` 已覆盖新增产物类型
- 本地临时目录未进入版本控制

### 7.1 Homebrew formula

Homebrew formula (`gc.rb`) 是发布产物，不属于本地产物：

- 由 GoReleaser 在 release workflow `artifacts` job 生成到 `dist/homebrew/gc.rb`（含 version/sha256/URL/install/test，来自 `.goreleaser.yaml` `brews:` 段）
- 由 `brew` job 用 deploy key 推送到 tap 仓 `atomgit-cli/homebrew-tap`（见 [release-process.md](./release-process.md) §6.5）
- 本地 `homebrew/gc.rb` 仅作参考副本，实际 formula 以 GoReleaser 生成结果为准（DSL 风格可能与本副本不同）

> **技术债务（保留 brews 的决策依据）**：GoReleaser `brews:` 段自 v2.16 起标记废弃，但仍可生成 formula（`goreleaser release` 兼容，仅 `goreleaser check` 报错）。正式 release workflow 固定 GoReleaser v2.17.1。GoReleaser 推荐迁移到 `homebrew_casks`，但 cask 假定签名公证、依赖 macOS app bundle 特性、Linux 支持有限，**不适用于跨平台未签名 CLI**——gc 是 CLI 二进制，需 macOS + Linux 双平台 `brew install`，formula 是 Homebrew 对 CLI 工具的标准做法（如 gh/jq/ripgrep）。故保留 `brews` 生成 formula，跟踪 GoReleaser 后续是否提供 CLI 友好替代；若未来大版本移除 `brews`，再评估自定义 formula 生成（`after.hooks` + 脚本）。不阻塞当前发布。

## 8. 文档同步要求

当构建、打包流程或产物命名发生变化时，必须同步检查：

- `docs/PACKAGING.md`
- `README.md`
- `spec/delivery/build-and-package.md`
- `spec/delivery/release-process.md`
- `AGENTS.md`
- `CLAUDE.md`
- 相关 AI skills

## 9. 禁止事项

以下行为不允许出现：

- 在未验证产物实际可用前更新发布说明
- 在主工作区堆积大量构建产物后直接提交
- 将本地测试凭证写入构建脚本或文档
- 在未同步文档的情况下修改打包产物命名
- 使用未纳入仓库规范的个人脚本替代标准流程

## 10. 当前执行基线

当前仓库的构建与打包基线如下：

1. 开发验证优先使用 `go build -o ./gc ./cmd/gc`
2. 多平台构建优先使用 `make build-all`
3. 本地打包优先使用 `scripts/package.sh`
4. 发布前的构建检查必须结合 `go test ./...` 和真实命令验证
5. 构建与打包规则变更后必须同步更新用户文档和 AI 协作文档
