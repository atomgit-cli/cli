# 使用 AI 操作 GitCode 指南

本指南帮助你通过 AI 助手（如 Claude Code）操作 GitCode 平台。

边界说明：

- 本文档只适用于“外部项目通过 AI 使用 `gitcode`（或可用时的 `gc`）操作 GitCode”
- 本文档不定义 gitcode-cli 仓库内部开发流程
- 在 gitcode-cli 仓库内部参与开发时，应以 `AGENTS.md`、`CLAUDE.md`、`spec/README.md` 和 `spec/workflows/*` 为准

## 1. 安装 GitCode CLI

主渠道建议：已安装 Node.js/npm 的 Windows、Linux、macOS、OpenHarmony（arm64）与 AI 环境优先使用 npm bootstrap；没有 Node.js 时再使用 Homebrew、DEB/RPM，wheel 只放入 pipx/venv。不要让多个全局渠道同时争用 `gc` / `gitcode`。

**npm（Windows 和跨平台 Node 环境推荐）:**

```bash
npx -y @gitcode-cli/cli@latest install
gitcode version
gitcode doctor install --json
```

上面的短命令适合可信的用户环境。CI、不可信项目目录或自定义 npm registry 环境请固定官方源并禁用 lifecycle scripts：

```bash
npx --yes --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@latest install
```

不要使用 `npm i @gitcode-cli/cli` 安装全局 CLI；它只添加当前项目依赖，不会替换 PATH 中已有的 `gitcode`。需要 npm global prefix 管理入口时，同样固定官方 registry 并禁用 lifecycle scripts。

若裸 `gitcode` 仍被旧 pip 入口遮蔽，Windows 先运行 `& "$(npm prefix -g)\gitcode.cmd" doctor install --json`，Linux/macOS 运行 `"$(npm prefix -g)/bin/gitcode" doctor install --json`；确认候选路径后再由用户选择调整 PATH、升级或卸载旧渠道。

npm 安装默认每天在命令结束后后台检查 stable 更新，发现新版本会在下一次启动提示 `gitcode update`，不会自动安装；需要自动应用时由用户执行 `gitcode config set update.mode auto`。CI、非交互或可复现任务应设置 `GC_NO_UPDATE_CHECK=1`，或执行 `gitcode config set update.mode off`。更新器不调用 pip/Homebrew/apt/dnf，也不修改 PATH。

**macOS (Homebrew):**

```bash
brew install gitcode-cli/homebrew-tap/gc
brew upgrade gc
gitcode doctor install --json
```

**Linux (DEB/RPM):**

```bash
# DEB (Debian/Ubuntu)
wget https://gitcode.com/gitcode-cli/cli/releases/download/v0.13.0/gc_0.13.0_amd64.deb
sudo dpkg -i gc_0.13.0_amd64.deb

# RPM (RHEL/CentOS/Fedora)
wget https://gitcode.com/gitcode-cli/cli/releases/download/v0.13.0/gc-0.13.0-1.x86_64.rpm
sudo rpm -i gc-0.13.0-1.x86_64.rpm
```

DEB/RPM packages install both `gc` and `gitcode`; on Linux they are equivalent.

**Wheel 包（跨平台、隔离安装）:**

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
- Windows 使用 `py -m pip install --user ...` 时，脚本会安装到 Python user scheme 的 `Scripts` 目录。请运行 `py -c "import os, sysconfig; print(sysconfig.get_path('scripts', os.name + '_user'))"` 获取准确路径，将其加入用户 `PATH` 后重新打开终端；配置前可直接运行 `py -m gc_cli version`。
- Windows PowerShell 预置 `gc` 作为 `Get-Content` 别名；如果 `gc version` 被解析为读取文件，请改用 `gitcode version`、`gc.exe version` 或 `py -m gc_cli version`。
- Windows PowerShell 中让 AI 直接执行命令时，优先使用 `gitcode`。中文或其他非 ASCII 正文需要传给 `--body-file -` / `--comment-file -` 时，优先写入 UTF-8 临时文件再传文件路径；若必须直接管道，先设置 `$OutputEncoding = [System.Text.UTF8Encoding]::new($false)`。

**PyPI（备选）:**

> **注意**：PyPI 官方源可能有同步延迟。固定目标版本可避免静默安装旧版本；目标版本暂不可用时，请使用上方 Release wheel。

```bash
# 创建虚拟环境
python3 -m venv .venv
source .venv/bin/activate  # Linux/macOS
# .\.venv\Scripts\Activate.ps1  # Windows PowerShell

# 固定版本安装，避免 PyPI 尚未同步时安装旧版本
python -m pip install -i https://pypi.org/simple/ gitcode-cli==0.11.1

# Windows PowerShell 中推荐使用 gitcode
gitcode version
```

**从源码构建:**

```bash
git clone https://gitcode.com/gitcode-cli/cli.git
cd cli
go build -o gc ./cmd/gc
```

## 2. 认证配置

以下示例使用安装包提供的 `gitcode` 入口；从源码构建或使用独立二进制时，请将命令名改为 `gc`。

```bash
# 打开新建访问令牌页面，生成后回到终端粘贴
gitcode auth login --web

# 浏览器不可用时，在本地交互终端中输入 Token
gitcode auth login
```

当前 `--web` 不是完整的浏览器授权流程，终端粘贴的 Token 也不会隐藏。上述登录命令必须由用户本人在私有、未录制且不由 AI 控制的本地终端中执行。不要把 Token 值交给 AI，也不要把 Token 直接写进命令、shell history、Prompt 或配置脚本。CI 场景应通过平台 Secret 注入 `GC_TOKEN` 或 `GITCODE_TOKEN`。完整认证规则见 [AUTH.md](./AUTH.md)。

**获取 Token：**
1. 登录 [GitCode](https://gitcode.com)
2. 进入 个人设置 -> 访问令牌
3. 新建访问令牌并复制

## 3. 验证安装

```bash
gitcode version
gitcode doctor install --json
gitcode auth status
```

## 4. 安装 GitCode CLI Skills

面向外部项目的通用 Skills 只在独立仓库 [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills) 维护和分发，本仓库不保留 Skill 副本。

该仓库将 skills 分为两类：

- 核心命令 skills：覆盖认证、仓库、Issue、PR、Review、Release、Commit、标签与里程碑等命令族
- 工作流 skills：覆盖 Issue 创建与评审、PR 创建与评审、评审反馈修复、Release 发布、安全检查、流水线分析等端到端任务

安装时克隆 skills 仓库，并将需要的独立 skill 目录复制到 AI 客户端的 skills 目录。每个 skill 都可以独立安装：

```bash
git clone git@gitcode.com:gitcode-cli/skills.git
cd skills
git checkout 6f0d8b0eb2bc3fb6f4c112a90432f3fcbf491d75

# 启用前审阅 skill 及其引用文件；目标已存在时停止，避免静默覆盖
git show --stat --oneline HEAD
git show HEAD:gitcode-pr-review/SKILL.md
test ! -e ~/.codex/skills/gitcode-pr-review/SKILL.md || { echo "target skill already exists" >&2; exit 1; }

# Codex 示例
mkdir -p ~/.codex/skills/gitcode-pr-review
cp gitcode-pr-review/SKILL.md ~/.codex/skills/gitcode-pr-review/SKILL.md

# Claude 示例
test ! -e ~/.claude/skills/gitcode-pr-review/SKILL.md || { echo "target skill already exists" >&2; exit 1; }
mkdir -p ~/.claude/skills/gitcode-pr-review
cp gitcode-pr-review/SKILL.md ~/.claude/skills/gitcode-pr-review/SKILL.md
```

Windows PowerShell：

```powershell
git clone git@gitcode.com:gitcode-cli/skills.git
Set-Location skills
$SkillCommit = "6f0d8b0eb2bc3fb6f4c112a90432f3fcbf491d75"
git checkout $SkillCommit
git show --stat --oneline HEAD
git show HEAD:gitcode-pr-review/SKILL.md
$Target = "$env:USERPROFILE\.codex\skills\gitcode-pr-review\SKILL.md"
if (Test-Path $Target) { throw "target skill already exists: $Target" }
New-Item -ItemType Directory -Force "$env:USERPROFILE\.codex\skills\gitcode-pr-review" | Out-Null
Copy-Item gitcode-pr-review\SKILL.md $Target
```

示例固定到评审时核验过的 commit；升级前应重新审阅 `SKILL.md` 及其引用文件。完整 skill 清单、适用场景和安装说明见 [GitCode CLI Skills README](https://gitcode.com/gitcode-cli/skills)。安装后，AI 即可按对应工作流使用 `gitcode` 命令操作 GitCode。

## 5. 面向 AI 的使用建议

为了让 AI 和脚本更稳定地消费 `gitcode`，优先使用以下模式：

```bash
# 读取类命令优先使用 JSON
gitcode repo view owner/repo --json
gitcode repo log -R owner/repo --file README.md --branch main --json
gitcode issue list -R owner/repo --json
gitcode pr view 123 -R owner/repo --json
gitcode pr list -R owner/repo --paginate --per-page 100 --json
gitcode pr list -R owner/repo --commit-message "fix login" --json
gitcode pr comments 123 -R owner/repo --json
gitcode issue prs 123 -R owner/repo --json
gitcode repo stats -R owner/repo --branch main --json
gitcode milestone view 1 -R owner/repo --json
gitcode commit comments list -R owner/repo --json
gitcode commit comments list-by-sha <sha> -R owner/repo --json
gitcode commit comments view <id> -R owner/repo --json

# 探索命令结构优先使用 schema
gitcode schema
gitcode schema "issue view"

# typed command 尚未覆盖时，可使用 gitcode api 读取 GitCode API 原始响应
gitcode api repos/owner/repo
gitcode api 'repos/owner/repo/commits?path=README.md&sha=main'

# 高风险删除命令先 dry-run，再决定是否执行
gitcode repo delete owner/repo --dry-run
gitcode release delete v1.0.0 -R owner/repo --dry-run

# 高频写路径需要解析结果时使用 JSON
gitcode issue create -R owner/repo --title "Bug" --body "..." --json
gitcode pr create -R owner/repo --head feature-branch --title "Feature" --body "..." --json
gitcode issue edit 123 -R owner/repo --title "Updated" --json
# 仅在用户针对本次合并明确授权后，才允许跳过交互确认
gitcode pr merge 123 -R owner/repo --yes --json
gitcode repo fork owner/repo --json
gitcode release create v1.0.0 -R owner/repo --title "v1.0.0" --notes "..." --json
gitcode release upload v1.0.0 app.zip -R owner/repo --json
```

说明：

- 删除、关闭、重开、状态切换、合并、同步推送/建 PR 等高风险写操作在非交互环境中不会再隐式等待输入；如果未显式传 `--yes`，会直接失败。
- AI 代理/脚本可显式传 `--no-interactive` 主动声明非交互模式，等价于强制非 TTY 的确认行为；破坏性命令仍需 `--yes` 才能执行。
- 当前默认文本输出仍保留；代理和脚本应优先使用 `--json`。
- `repo log --json` 适合按文件和分支追踪提交历史；`pr list --paginate` 适合跨页扫描，`--commit-message` 适合从提交信息反查 PR。
- PR JSON 中的 `merged` 已按 `state` / `merged_at` 归一化；代理可直接用该布尔值判断是否已合并，无需处理 GitCode API 的 `merged: null`。
- `gitcode api` 输出远端原始响应，适合 typed command 尚未覆盖的 API；使用含 `&` 的查询参数时建议整体加引号。
- 写路径 `--json` 只在操作成功后输出结构化结果；执行失败时不要从 stdout 解析半成品结果。
- `pr create --json` 会尽量回读新建 PR 以补齐创建响应缺失的正文；如果远端仍未返回 body，会在 stderr 给 warning，并保持 JSON 中的远端事实为空，脚本可再运行 `gitcode pr view <number> -R owner/repo --json` 核验。
- 当前基础退出码语义：`0` 成功，`1` 通用错误，`2` 参数/用法错误，`3` 资源不存在，`4` 认证/权限错误，`5` 资源冲突。

## 6. 使用 RTK 优化 Token 消耗（可选）

[RTK（Rust Token Killer）](https://github.com/rtk-ai/rtk) 是一个轻量级 CLI 代理工具，可在 CLI 命令的输出到达 LLM 上下文之前进行智能过滤与压缩，减少 60-90% 的 Token 消耗。

### 安装 RTK

```bash
# 从 GitHub 安装 RTK
cargo install rtk
# 或下载预编译二进制: https://github.com/rtk-ai/rtk/releases
```

### 配置 gc 过滤器

```bash
# 复制参考配置
mkdir -p ~/.config/rtk
cp contrib/rtk/config.toml ~/.config/rtk/config.toml

# 编辑配置以自定义过滤规则
$EDITOR ~/.config/rtk/config.toml
```

### 在 AI 工具中启用

```bash
# Claude Code
rtk init -g

# Hook 会自动拦截 gc 命令输出:
# gc pr list -R owner/repo → rtk gc pr list -R owner/repo
```

### 配置示例

参考配置文件位于 `contrib/rtk/config.toml`，预置了以下 gc 命令的过滤规则：

| 命令 | 策略 | 效果 |
|------|------|------|
| `gc pr list` / `gc issue list` | table-compact | 仅保留 number/state/title 列，限制 20 行 |
| `gc pr view` / `gc issue view` | strip-ansi-and-condense | 剥离颜色，仅保留标题/状态/正文 |
| `gc repo list` | table-compact | 仅保留 name/visibility/description |
| `gc release list` | table-compact | 仅保留 tag/title/date |
| `gc auth status` / `gc version` | one-line | 单行输出 |

### 效果对比

```
# 标准输出（~120 tokens）
$ gc pr list -R owner/repo
Showing 15 of 15 pull requests in owner/repo (filtered)
#1  open    Fix login bug                    bugfix/login ...
#2  merged  Add dark mode support           feature/dark-mode ...
...

# RTK 压缩后（~40 tokens，节省 ~67%）
#1 open Fix login bug
#2 merged Add dark mode support
...
```

### 注意事项

- RTK 是可选的外部工具，不影响 `gc` 的默认行为
- RTK 未安装时，`gc` 命令完全不受影响
- 错误输出默认透传，不会被过滤
- [参考配置文件](../contrib/rtk/config.toml) 可根据需要自定义

## 7. 在规范化仓库中的协作提醒

如果目标仓库本身已经定义了开发规范，AI 应直接遵守目标仓库自己的正式规则、状态机和证据门禁，而不是套用本文档。

## 8. 可参考的固定模板

外部项目如需固定模板，应由目标项目自己定义。

如果你只是参考 gitcode-cli 仓库的内部模板结构，请查看：

- [AI-TEMPLATES.md](./AI-TEMPLATES.md)

但这些模板不应被默认视为外部项目的正式流程模板。

## 完成后的使用方式

安装完成后，直接告诉 AI 你想做什么：

```
查看 owner/repo 仓库的所有 Issue
创建一个 PR，标题是"新增功能"
发布 v1.0.0 版本
```

AI 会自动使用 `gitcode` 命令执行操作。

## 更多信息

- [命令详细文档](./COMMANDS.md)
- [GitCode CLI 仓库](https://gitcode.com/gitcode-cli/cli)

---

**最后更新**: 2026-07-22
