# GitCode CLI

[![AI 操作指南](https://img.shields.io/badge/📖_使用_AI_操作_GitCode_指南-点击查看-FF6B6B?style=for-the-badge)](./docs/AI-GUIDE.md)

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-latest-blue)](https://gitcode.com/gitcode-cli/cli/releases)

GitCode CLI 把仓库、Issue、PR、Release 和 Actions 带回终端，让开发者减少页面切换，也让脚本与 AI 获得结构化、可审计、带安全边界的 GitCode 执行入口。

[快速了解核心价值与应用场景，并在五分钟内开始使用](./docs/INTRODUCTION.md)。

## 文档导航

按角色建议从以下入口开始：

| 角色 | 入口 |
|------|------|
| 使用者 | [docs/README.md](./docs/README.md) |
| 开发者 | [spec/README.md](./spec/README.md) |
| Codex / 代理 | [AGENTS.md](./AGENTS.md) |
| Claude | [CLAUDE.md](./CLAUDE.md) |

主要文档：

- [产品介绍与快速上手](./docs/INTRODUCTION.md)
- [命令手册](./docs/COMMANDS.md)
- [认证说明](./docs/AUTH.md)
- [飞书/Lark 通知](./docs/LARK.md)
- [回归说明](./docs/REGRESSION.md)
- [打包说明](./docs/PACKAGING.md)
- [AI 操作指南（外部项目）](./docs/AI-GUIDE.md)
- [应用案例库](./Example/index.md)
- [开发规范](./spec/README.md)
- [真相源矩阵](./spec/governance/source-of-truth-matrix.md)
- [AI 本地开发流程](./spec/workflows/ai-local-development-workflow.md)
- [阶段说明](./issues-plan/PROGRESS.md)

## 安装

推荐只让一个全局安装渠道拥有 `gc` / `gitcode`。已安装 Node.js/npm 的 Windows、Linux、macOS、OpenHarmony（arm64）与 AI 环境优先使用下方 npm bootstrap；没有 Node.js 时再选择 Homebrew、DEB/RPM 或隔离的 Python wheel。CI 建议固定版本并校验 checksum。

### 推荐：npm 一行 bootstrap（跨平台）

已安装 Node.js/npm 时，安装或升级 CLI 的首选入口是：

```bash
npx -y @gitcode-cli/cli@latest install
gitcode version
```

短命令适合用户主目录等可信环境，会继承当前目录与用户的 npm 配置。CI、审计、不可信项目目录或自定义 npm registry 环境请使用完整加固命令：

```bash
npx --yes --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@latest install
```

CI、审计或版本复现可固定版本：

```bash
npx --yes --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@0.13.0 install
```

不要使用 `npm i @gitcode-cli/cli` 或 `npm install @gitcode-cli/cli` 安装全局 CLI；这两条命令只会把包加入当前项目的 `node_modules`，不会更新 PATH 中已有的 `gitcode`。需要由 npm global prefix 管理入口时，可改用：

```bash
npm install -g --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@latest
gitcode version
```

一行 bootstrap 会把平台二进制安装到全局 bin 目录；Linux/macOS 同时配置 bash/zsh/fish 补全，Windows 跳过 shell 补全。Windows 会在用户显式执行 `install` 后把安装目录置于持久 User PATH 前面并去重；如不希望修改，传入 `--no-modify-path`。由于 `npx` 子进程无法刷新已经打开的 PowerShell，安装完成后会用中文给出当前窗口立即生效的 `$env:Path` 命令和重开终端说明。它不会修改 Machine PATH、删除或重写其他 PATH 条目，也不会调用其他包管理器卸载软件。显式 `--target-dir` 会替换该目录内的同名常规文件，不要将它指向 Python Scripts、npm prefix 等由其他包管理器持有的目录。

Linux/macOS 上若存在历史同目录 `gitcode -> gc` 别名，会在安全校验和事务保护下自动迁移；无需先卸载 PyPI 包或手工删除链接。命令显式固定 generic 与 scoped 官方 registry，并禁用 npm lifecycle scripts，避免继承当前目录或用户 npm 配置中的非官方包来源。

安装或升级后可离线检查实际命令来源和全部 PATH 候选：

```bash
gitcode doctor install
gitcode doctor install --json
```

诊断只读取公开的可执行文件、PATH 和本地安装 manifest，不读取认证配置或 Token，也不会卸载其他渠道或修改 PATH。

### 从源码构建

**前置要求:**
- Go 1.22+

```bash
# 克隆仓库（需要 git clone 才能获取版本信息）
git clone https://gitcode.com/gitcode-cli/cli.git
cd cli

# 方式一：使用 go build（推荐）
go build -o gc ./cmd/gc
# 安装
mkdir -p ~/.local/bin
mv gc ~/.local/bin/

# 方式二：构建并安装 gc/gitcode（带完整版本标签）
make install PREFIX="$HOME/.local"

# 添加到 PATH
export PATH="$HOME/.local/bin:$PATH"
```

> **说明**:
> - `go build` 从 `debug.ReadBuildInfo()` 自动获取 git commit 和构建时间（需要 `git clone` 源码）。
> - `make build` 使用 `-ldflags` 注入完整版本标签（如 `v0.3.11-38-g1128f2b`）。

### Linux 包管理器

**DEB (Debian/Ubuntu):**

```bash
# 从 Releases 下载 .deb 包
wget https://gitcode.com/gitcode-cli/cli/releases/download/v0.13.0/gc_0.13.0_amd64.deb

# 安装
sudo dpkg -i gc_0.13.0_amd64.deb
```

DEB/RPM packages install both `gc` and `gitcode`; on Linux they are equivalent.

**RPM (RHEL/CentOS/Fedora):**

```bash
# 从 Releases 下载 .rpm 包
wget https://gitcode.com/gitcode-cli/cli/releases/download/v0.13.0/gc-0.13.0-1.x86_64.rpm

# 安装
sudo rpm -i gc-0.13.0-1.x86_64.rpm
```

DEB/RPM packages install both `gc` and `gitcode`; on Linux they are equivalent.

### Wheel 包（跨平台、隔离安装）

从 Release 归档下载 wheel 包安装，**内置全平台二进制**（Linux x64/ARM、macOS Intel/Apple Silicon、Windows x64）：

```bash
# 创建虚拟环境
python3 -m venv .venv
source .venv/bin/activate  # Linux/macOS
# .\.venv\Scripts\Activate.ps1  # Windows PowerShell

# 安装（一行命令）
pip install https://gitcode.com/gitcode-cli/cli/releases/download/v0.13.0/gitcode_cli-0.13.0-py3-none-any.whl

# Windows PowerShell 中推荐使用 gitcode，避免 gc 被内置 Get-Content 别名覆盖
gitcode version
```

说明：
- wheel 会同时安装 `gc` 和 `gitcode` 两个命令入口，功能相同。
- DEB/RPM 包也会同时安装 `gc` 和 `gitcode`；Linux 上二者功能相同。
- 不建议让 pip user install 与 DEB/RPM/Homebrew/npm 同时提供全局命令。已混用时先运行 `gitcode doctor install` 确认实际选中路径，再由用户明确选择调整 PATH、升级原渠道或卸载旧渠道；CLI 不会代替用户调用其他包管理器。
- Windows 使用 `py -m pip install --user ...` 时，脚本会安装到 Python user scheme 的 `Scripts` 目录。请运行 `py -c "import os, sysconfig; print(sysconfig.get_path('scripts', os.name + '_user'))"` 获取准确路径，将其加入用户 `PATH` 后重新打开终端；配置前可直接运行 `py -m gc_cli version`。
- Windows PowerShell 预置 `gc` 作为 `Get-Content` 别名；如果 `gc version` 被解析为读取文件，请改用 `gitcode version`、`gc.exe version` 或 `py -m gc_cli version`。
- Windows PowerShell 中通过 `--body-file -` / `--comment-file -` 管道传入中文或其他非 ASCII 正文时，推荐使用 UTF-8 文件；如果必须直接管道，先设置 `$OutputEncoding = [System.Text.UTF8Encoding]::new($false)`。CLI 会拦截疑似已被 PowerShell 损坏成 `???` 的输入并提示正确用法。

```powershell
Set-Content -Path body.md -Value "中文正文" -Encoding UTF8
gitcode issue create -R owner/repo --title "标题" --body-file body.md
```

### PyPI（备选）

> PyPI 可能晚于 GitCode Release 同步。固定目标版本可避免静默安装旧版本；目标版本暂不可用时，请使用上方 Release wheel。

```bash
# 创建虚拟环境
python3 -m venv .venv
source .venv/bin/activate  # Linux/macOS
# .\.venv\Scripts\Activate.ps1  # Windows PowerShell

# 固定版本安装，避免 PyPI 尚未同步时安装旧版本
python -m pip install -i https://pypi.org/simple/ gitcode-cli==0.13.0

# Windows PowerShell 中推荐使用 gitcode
gitcode version
```

### Linux 二进制文件

从 Release Assets 直接下载 Linux 二进制文件：

| 平台 | 文件 |
|------|------|
| Linux x64 | `gc_linux_amd64` |
| Linux ARM64 | `gc_linux_arm64` |

下载地址: https://gitcode.com/gitcode-cli/cli/releases

下载后赋予可执行权限，并放到 PATH 目录：

```bash
chmod +x gc_linux_amd64
mv gc_linux_amd64 ~/.local/bin/gc
ln -s gc ~/.local/bin/gitcode
gc version
```

需要与系统包管理器隔离时可使用上方 wheel；需要免安装直接部署时使用独立二进制。二者都应先确认平台架构受支持。

### Docker 镜像

> **用途说明**：本仓库 Docker 配置面向开发/构建环境。`docker compose up gc` 默认执行 `gc --help` 展示用法后退出（CLI 工具非长期服务）。交互式使用 `docker run -it --entrypoint bash gitcode/gc:dev`（进入 shell）或 `docker run -it gitcode/gc:dev <子命令>`（如 `auth login`）。

仓库已提供 `Dockerfile`、`docker-compose.yml` 和 Makefile 目标：

```bash
# 构建并运行
make docker-build
make docker-run

# 或使用 docker compose
docker compose up gc
```

认证 Token 通过环境变量传入。请在交互终端中静默读取，避免 Token 值进入 shell history：

```bash
read -rsp "GitCode token: " GC_TOKEN
export GC_TOKEN
make docker-run
unset GC_TOKEN
```

更多用法参见 Makefile 和 `docker-compose.yml`。

### Homebrew (macOS/Linux)

```bash
brew install gitcode-cli/homebrew-tap/gc
```

更新到最新版本：

```bash
brew upgrade gc
```

shell 补全（bash/zsh/fish）随安装自动配置，无需额外操作。
Homebrew 同时提供 `gc` 与 `gitcode`；升级后可运行 `gitcode doctor install` 检查是否仍被 pip/npm 旧入口遮蔽。

### npm 更新与 PATH 诊断

如果 `gitcode version` 仍显示旧版本，先直接调用 npm 安装目录中的新入口诊断，不必先卸载 pip 或手工删除旧入口：

```powershell
# Windows PowerShell
& "$(npm prefix -g)\gitcode.cmd" doctor install --json
```

```bash
# Linux/macOS
"$(npm prefix -g)/bin/gitcode" doctor install --json
```

包内置 Linux/macOS/Windows 多平台二进制。Windows bootstrap 同时安装 `gc.exe` 和 `gitcode.exe`，PowerShell 请使用 `gitcode`，避免内置 `gc`/`Get-Content` alias。
Linux/macOS bootstrap 会自动迁移历史安装遗留的同目录 `gitcode -> gc` 别名；该迁移参与安装事务，失败时恢复原链接，并继续拒绝指向其他位置的符号链接。

npm global 与 npm bootstrap 默认使用 `notify` 模式：命令本身立即执行，24 小时 TTL 到期后在后台检查 stable `latest`，发现新版后在下一次启动提示 `gitcode update`，不会自动安装，也不会改变刚完成命令的退出码。需要自动应用的用户可明确启用 `auto`；该模式具有跨进程锁、版本健康检查和失败回滚。

```bash
gitcode update --check       # 只检查
gitcode update               # 显式更新当前 npm 渠道
gitcode update --json

gitcode config set update.mode auto
gitcode config set update.mode notify
gitcode config set update.mode off

gitcode --no-update-check version   # 单次禁用
GC_NO_UPDATE_CHECK=1 gitcode version
```

`CI=true`、`--no-interactive`、`--no-update-check` 或 `GC_NO_UPDATE_CHECK=1` 会禁用后台检查。更新器仅从官方 `https://registry.npmjs.org` 获取精确 stable 版本，安装时禁用生命周期脚本，并以最小环境启动；它不会调用 pip、Homebrew、apt、dnf，也不会静默修改 PATH。npm 生命周期脚本被组织策略禁用时，可直接运行 npm prefix 中的 `gitcode` 再执行 `doctor install`；wrapper 会在首次直接运行时补建来源 metadata。

### 规划中的安装方式

以下安装方式正在开发中：

- [ ] Scoop (Windows)

## 快速开始

### 认证

以下示例使用安装包提供的 `gitcode` 入口；从源码构建或使用独立二进制时，请将命令名改为 `gc`。

**方式一：打开令牌页面并登录**

```bash
gitcode auth login --web
```

当前 `--web` 会打开 GitCode 的新建访问令牌页面，生成令牌后仍需回到终端粘贴。当前版本不会隐藏输入，因此必须由用户本人在私有、未录制且不由 AI 控制的本地终端中执行。

**方式二：交互式 Token 登录**

```bash
gitcode auth login
```

浏览器不可用时，可在同样受控的本地交互终端中输入 Token。不要把 Token 值直接写进命令、shell history 或配置脚本。CI 场景应通过平台 Secret 注入 `GC_TOKEN` 或 `GITCODE_TOKEN`。

当前版本认证优先级：

1. `GC_TOKEN`
2. `GITCODE_TOKEN`
3. 本地登录配置

说明：
- `gc auth login` 会将认证信息持久化到本地配置目录
- 如果设置了环境变量，环境变量始终覆盖本地配置
- `gc auth logout` 只清理本地配置，不会自动取消环境变量
- 详细规则见 [docs/AUTH.md](./docs/AUTH.md)

**获取 Token：**
1. 登录 [GitCode](https://gitcode.com)
2. 进入 个人设置 -> 访问令牌
3. 点击“新建访问令牌”，选择所需权限
4. 复制生成的 Token

**验证认证：**

```bash
# 查看认证状态
gitcode auth status
```

> 详细命令行为和完整示例请查看 [docs/COMMANDS.md](./docs/COMMANDS.md)。

## 输出格式

`gitcode` 的只读命令继续以文本输出为默认体验，同时为脚本和代理保留稳定的结构化入口。

```bash
# 结构化输出
gitcode issue list -R owner/repo --json
gitcode issue list -R owner/repo --format json
gitcode repo log -R owner/repo --file README.md --branch main --json
gitcode pr list -R owner/repo --paginate --per-page 100 --json

# 常规文本与表格
gitcode issue list -R owner/repo --format simple
gitcode issue list -R owner/repo --format table

# 时间格式切换
gitcode issue list -R owner/repo --time-format absolute
gitcode issue list -R owner/repo --time-format relative

# 自定义模板输出
gitcode issue list -R owner/repo --template '{{range .}}#{{.Number}} {{.Title}}{{"\n"}}{{end}}'

# typed command 尚未覆盖的 API，可用 gitcode api 读取原始响应
gitcode api repos/owner/repo
```

`issue view` 和 `pr view` 的文本详情展示也会保持稳定布局，而 `--json` 仍然是面向机器调用的首选入口。

## 常见任务入口

最常用的起步命令：

```bash
# 查看仓库
gitcode repo view

# 查看文件提交历史
gitcode repo log -R owner/repo --file README.md --branch main

# 创建 Issue
gitcode issue create --title "Bug report" --body "Description"

# 列出 Issues
gitcode issue list --state open

# 创建 PR
gitcode pr create --title "New feature" --base main

# 按提交信息反查 PR
gitcode pr list -R owner/repo --commit-message "fix login"

# 提交前检查 pre-commit 配置与本地环境
gitcode precommit check

# 查看流水线运行记录
gitcode actions run list -R owner/repo --status FAILED

# 查看流水线运行详情
gitcode actions run view <run-id> -R owner/repo

# 列出流水线运行的 jobs
gitcode actions job list <run-id> -R owner/repo

# 查看工作流 job 详情
gitcode actions job view <run-id> <job-id> -R owner/repo

# 下载 job 日志归档
gitcode actions job log <run-id> <job-id> -R owner/repo --output job-log.zip

# 列出仓库 artifacts
gitcode actions artifact list -R owner/repo

# 查看 artifact 详情
gitcode actions artifact view <artifact-id> -R owner/repo

# 下载 artifact
gitcode actions artifact download <artifact-id> -R owner/repo --output artifact.zip

# 删除 artifact
gitcode actions artifact delete <artifact-id> -R owner/repo --yes

# 校验 workflow YAML
gitcode actions yaml validate --file .gitcode/workflows/ci.yml -R owner/repo

# 调用 GitCode API 原始响应
gitcode api repos/owner/repo

# 查看认证状态
gitcode auth status
```

完整命令说明、参数细节、平台限制和更多示例，请直接查看：

- [docs/COMMANDS.md](./docs/COMMANDS.md)
- [docs/AUTH.md](./docs/AUTH.md)
- [docs/PACKAGING.md](./docs/PACKAGING.md)
- [docs/REGRESSION.md](./docs/REGRESSION.md)

## Shell 补全

```bash
# Bash
gc completion bash > /etc/bash_completion.d/gc
source ~/.bashrc

# Zsh
gc completion zsh > "${fpath[1]}/_gc"
source ~/.zshrc

# Fish
gc completion fish > ~/.config/fish/completions/gc.fish
source ~/.config/fish/config.fish
```

## 项目定位

当前仓库已经建立：

- 用户文档入口：[`docs/`](./docs/README.md)
- 正式规范入口：[`spec/`](./spec/README.md)
- Codex 入口：[`AGENTS.md`](./AGENTS.md)
- Claude 入口：[`CLAUDE.md`](./CLAUDE.md)

如果你要看完整规范、构建与发布规则、质量门禁和 AI 协作边界，请直接进入对应入口，不要仅依赖本 README。

补充说明：

- `docs/AI-GUIDE.md` 只服务外部项目通过 AI 使用 `gitcode`（或源码构建的 `gc`）
- gitcode-cli 仓库内部 AI 开发请看 `AGENTS.md`、`CLAUDE.md` 和 `spec/workflows/ai-local-development-workflow.md`
- `issues-plan/PROGRESS.md` 只作为阶段说明，不作为单个 issue / PR 的实时事实依据

## 开发

```bash
# 克隆仓库
git clone https://gitcode.com/gitcode-cli/cli.git
cd gitcode-cli

# 安装依赖
make deps

# 构建
make build

# 运行测试
make test

# 代码检查
make lint

# 运行
make run
```

## 贡献

欢迎贡献代码。开始前请查看 [贡献指南](./CONTRIBUTING.md) 和 [spec/README.md](./spec/README.md)。

## 许可证

[MIT License](./LICENSE)

## 致谢

本项目参考了 [GitHub CLI](https://github.com/cli/cli) 的设计与实现，感谢 GitHub 团队的开源贡献。

## 相关链接

- [GitCode](https://gitcode.com) - GitCode 平台
- [API 文档](https://docs.gitcode.com/docs/apis/) - GitCode API 参考
- [问题反馈](https://gitcode.com/gitcode-cli/cli/issues) - 提交 Bug 或建议
