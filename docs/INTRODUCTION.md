# GitCode CLI：把 GitCode 变成开发者和 AI 都能直接调用的工程能力

当团队每天都在 GitCode 上查看 Issue、评审 PR、追踪流水线、整理发布时，真正消耗时间的往往不是某一次点击，而是重复查找、复制信息、切换页面，以及把同一套操作重新做一遍。

GitCode CLI 把这些工作带回终端：仓库、Issue、Pull Request、Commit、标签、里程碑、Release 和 Actions 都可以通过统一命令完成。对开发者，它减少上下文切换；对团队，它让操作可以复用和审计；对 AI 代理，它提供结构化、可发现、带安全边界的 GitCode 执行入口。本文统一使用跨平台入口 `gitcode`；通过 npm、PyPI、Homebrew、DEB/RPM 或 wheel 安装时都会同时提供 `gitcode` 和 `gc` 两个入口。

- 项目仓库：[gitcode-cli/cli](https://gitcode.com/gitcode-cli/cli)
- 安装渠道：[npm](https://www.npmjs.com/package/@gitcode-cli/cli)｜[PyPI](https://pypi.org/project/gitcode-cli/)｜[GitCode Release](https://gitcode.com/gitcode-cli/cli/releases)｜[GitHub Release](https://github.com/gitcode-cli/cli/releases)
- 完整命令手册：[docs/COMMANDS.md](https://gitcode.com/gitcode-cli/cli/blob/main/docs/COMMANDS.md)

## 五分钟开始使用

### 1. 安装

GitCode CLI 通过多个官方源分发：Linux/macOS 支持 x64/ARM64，Windows 当前支持 x64，OpenHarmony 支持 arm64（复用静态 Linux arm64 二进制）。已安装 Node.js/npm 时优先使用 npm bootstrap 作为 Windows、Linux、macOS、OpenHarmony 与 AI 环境的通用入口；没有 Node.js 时再选择 Homebrew、DEB/RPM 或隔离的 Python wheel。只让一个全局渠道拥有命令入口。

```bash
# 推荐：npm 一行 bootstrap（https://www.npmjs.com/package/@gitcode-cli/cli）
# 安装与升级到最新稳定版是同一条命令
npx -y @gitcode-cli/cli@latest install
gitcode version
```

短命令适合用户主目录等可信环境，会继承当前目录与用户的 npm 配置。CI、审计、不可信项目目录或自定义 npm registry 环境请使用完整加固命令：

```bash
npx --yes --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@latest install
```

不要使用 `npm i @gitcode-cli/cli` 安装全局 CLI；它只会添加当前项目依赖，不会更新 PATH 中已有的 `gitcode`。需要 npm global prefix 管理入口时，同样固定官方 registry 并禁用 lifecycle scripts。

Windows 上，npm bootstrap 会在用户显式执行 `install` 后，将安装器输出的默认用户级安装目录置于持久 User PATH 前面并去重；不修改 Machine PATH，也不会删除 pip、旧 npm 等其他渠道的入口。如不希望安装器修改 User PATH，可追加 `--no-modify-path`。`npx` 子进程无法刷新已经打开的 PowerShell，安装完成后请按中文提示中的绝对路径执行当前窗口 `$env:Path` 刷新命令，或关闭全部 PowerShell/Windows Terminal 后重新打开，再运行 `gitcode version`。

若仍显示旧版本，先运行 `gitcode doctor install` 检查实际命中的入口与 PATH 候选，再由你决定调整 PATH、升级原渠道或卸载旧渠道；CLI 不会代替用户调用其他包管理器。显式使用 `--target-dir` 会替换该目录内同名的常规 `gc` / `gitcode` 文件，不要将它指向 Python Scripts、npm prefix 等由其他包管理器持有的目录。完整行为见[命令手册的安装诊断、配置与更新章节](./COMMANDS.md#安装诊断配置与更新)。

npm 安装默认使用 `notify` 模式：普通命令最多每 24 小时在后台连接官方 npm registry 检查一次 stable 新版本，只提示、不自动安装。需要升级时显式运行 `gitcode update`；只有主动执行 `gitcode config set update.mode auto` 才会自动应用，也可用 `update.mode off` 禁用后台检查。详见[命令手册的更新策略](./COMMANDS.md#gitcode-update)。

其他官方渠道：

| 渠道 | 安装 / 升级 | 地址 |
|------|-------------|------|
| npm bootstrap（Windows、Node/AI 环境首选） | `npx -y @gitcode-cli/cli@latest install` | https://www.npmjs.com/package/@gitcode-cli/cli |
| PyPI（隔离安装） | `pipx install gitcode-cli` / `pipx upgrade gitcode-cli`（或仅在已激活 venv 中使用 pip） | https://pypi.org/project/gitcode-cli/ |
| Homebrew (macOS/Linux) | `brew install gitcode-cli/homebrew-tap/gc` / `brew upgrade gc` | [homebrew-tap](https://github.com/gitcode-cli/homebrew-tap) |
| GitCode Release | 从归档下载 wheel/DEB/RPM/二进制 | https://gitcode.com/gitcode-cli/cli/releases |
| GitHub Release | 同上制品镜像 | https://github.com/gitcode-cli/cli/releases |

> npm 全局安装备选：`npm install -g --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@latest`

安装后确认命令可用：

```bash
gitcode version
```

Windows PowerShell 已将 `gc` 用作 `Get-Content` 的别名，因此推荐使用 `gitcode`。npm/PyPI/Homebrew/DEB/RPM/wheel 安装时会同时提供 `gitcode` 和 `gc`；从源码构建或使用独立二进制时默认产物通常只有 `gc`。

### 2. 在私有终端登录并确认认证

```bash
gitcode auth login --web
```

输出示例：

```
Opening https://gitcode.com/setting/token-classic/create in your browser.
After generating a token in the browser, paste it below.

? Paste your authentication token: _
```

在桌面终端中 `--web` 会自动打开浏览器跳到令牌创建页；**在 WSL、SSH、无 GUI 的服务器等环境不会自动跳转**，需要手动复制输出的链接到浏览器打开，生成令牌后回到终端粘贴。

> ⚠️ Token 只在用户本人控制的私有、未录制本地终端中输入，当前版本不会隐藏输入。不要把 Token 交给 AI，也不要写进命令、脚本或聊天内容。

```bash
gitcode auth status   # 确认登录状态
```

认证来源、优先级和安全注意事项见[认证说明](https://gitcode.com/gitcode-cli/cli/blob/main/docs/AUTH.md)。

### 3. 给你的 AI 装上 GitCode 技能

GitCode CLI 的真正吸引力不在手敲命令，而在让 AI 在安全边界内替你完成端到端工作。把 [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills) 里的技能装进你的 AI 客户端（Claude / Codex 等）：

```bash
# 克隆技能仓库
git clone https://gitcode.com/gitcode-cli/skills.git
# 把需要的技能目录复制到 AI 客户端的 skills 目录
#   Claude:  ~/.claude/skills/<技能名>/SKILL.md
#   Codex:   ~/.codex/skills/<技能名>/SKILL.md
cp -R skills/gitcode-issue-triage ~/.claude/skills/
```

技能覆盖 Issue 创建/评审/分诊、PR 创建/评审/行内评审/反馈修复、Release 发布、安全检查、流水线分析、回归等端到端工作流。每个技能自包含，按需复制即可。

### 4. 直接给 AI 一个任务

装好技能后，你不必逐条敲命令——把任务交给 AI，它用 `gitcode` 在边界内完成查、读、写、发布：

> 「把这个仓库的 open issue 分诊一遍：打标签、标重复、给结论」
> 「评审 PR #N：读 diff、按项目规范做工程评审、提行内意见」
> 「准备并发布 vX.Y.Z：核 tag、生成 release notes、走发布流程」

AI 自动调用 `gitcode issue list/label`、`gitcode pr view/diff`、`gitcode release create/upload` 等，把结构化结果与证据回给你。你审结论，不用逐页点 GitCode。

### 5. 危险动作由你把关

AI 不会悄悄执行破坏性操作：

- 删除/合并/发布等破坏性命令默认有确认保护；非交互环境（脚本/管道）未显式 `--yes` 会直接失败，不隐式等待输入。
- issue/PR/comment/release 的正文提交前经 `cmdutil.ScanContentForSecrets` 扫描，疑似 `GC_TOKEN` 值会被拒绝，防止 token 泄漏到平台内容。
- 全局 `--no-interactive` 可主动声明非交互模式（供 AI/脚本），设置后所有确认立即失败、需 `--yes`。

### 想自己敲命令？

CLI 同样完整可用，命令树与参数见 [命令手册 COMMANDS.md](https://gitcode.com/gitcode-cli/cli/blob/main/docs/COMMANDS.md)。只读命令支持 `--json`，写命令支持 `--dry-run`，退出码 0-5 稳定语义；`gitcode help --json` 与 `gitcode schema` 可程序化发现命令。

常用入口：

- 所有命令和参数：[命令手册](https://gitcode.com/gitcode-cli/cli/blob/main/docs/COMMANDS.md)
- 登录与 Token 安全：[认证说明](https://gitcode.com/gitcode-cli/cli/blob/main/docs/AUTH.md)
- AI 操作建议：[AI 使用指南](https://gitcode.com/gitcode-cli/cli/blob/main/docs/AI-GUIDE.md)
- 安装、构建与平台说明：[项目 README](https://gitcode.com/gitcode-cli/cli)
- 可复制的业务场景：[应用案例库](https://gitcode.com/gitcode-cli/cli/tree/main/Example)
- AI 工作流 skills：[gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills)
- 问题反馈与功能建议：[Issues](https://gitcode.com/gitcode-cli/cli/issues)

## 真实样例：开箱即用

以下命令均可直接复制运行（以公开仓库 `gitcode-cli/cli` 为靶子）；AI 任务则把引用块里的话发给你的 AI 客户端，它用 `gitcode` 在边界内跑完，把结构化结果和证据回给你。

### 一行命令，立即出结果

```bash
# 当前有哪些待评审 PR？一条命令拿到结构化清单
gitcode pr list -R gitcode-cli/cli --state open --json

# 最近 CI 有没有挂？直接筛失败流水线
gitcode actions run list -R gitcode-cli/cli --status FAILED --json

# 不开浏览器看某 PR 的改动
gitcode pr view 440 -R gitcode-cli/cli --json

# 一条命令给 issue 打多个标签
gitcode issue label 497 --add feature,scope/actions -R gitcode-cli/cli

# 把 PR 的 diff 导出成 patch 评审
gitcode pr diff 440 -R gitcode-cli/cli > review.patch

# 列出某仓库所有 release 资产
gitcode release view v0.11.1 -R gitcode-cli/cli --json
```

### 把任务交给 AI，你审结论

**样例 1 — PR 一览**
> 「看下 gitcode-cli/cli 当前有哪些 open PR，每个 PR 的标题、状态、关联 issue，汇总成一张表，标注哪些 ready-for-review」

AI 跑 `gitcode pr list --state open --json`，整理成表回给你，你不用逐个点开页面。

**样例 2 — 定位失败 CI**
> 「gitcode-cli/cli 最近 CI 有没有失败？失败的是哪个 job、哪条 commit 触发的、失败原因是什么」

AI 跑 `gitcode actions run list --status FAILED` → `gitcode actions job list <run-id>` → 拉 job 日志，定位到具体 job + 触发 commit + 错误行，给出修复方向。

**样例 3 — Issue 分诊**
> 「把 gitcode-cli/cli 的 open issue 按 bug/feature 分两类，统计各自数量，标出 3 个最该优先处理的并说明理由」

AI 跑 `gitcode issue list --json --paginate`，分类统计，按影响/紧迫排序，给你一份待办清单。

**样例 4 — 工程评审 PR**
> 「评审 PR #440：读 diff，按项目 spec 规范做工程评审，给我 P0/P1/P2 结论」

AI 跑 `gitcode pr view/diff`，读 `spec/foundations/*`，跨代码/安全/测试/文档四角色出结构化评审，直接发到 PR 评论。

**样例 5 — 端到端发布**
> 「准备并发布 vX.Y.Z：写 release notes、核 tag、走 issue→PR→CI→merge→release 全流程」

AI 创建发布准备 PR → CI 通过 → 合入 → 触发 release workflow → 同步 GitCode Release，全程你只在危险动作处 `--yes` 确认，最后它把各平台验证结果汇总给你。

### <span style="color:#e60000;">危险动作不会悄悄执行</span>

AI 要删一个 release、合并一个 PR、或把含 `GC_TOKEN` 的正文提交到 issue？非交互环境没显式 `--yes` 直接失败；正文经 secret 扫描会被拒绝。<span style="color:#e60000;">你始终是最后一道闸——AI 干活，你签字</span>。

## 为什么值得使用

### 少离开终端，多完成一次完整交付

从发现问题到合并发布，常见动作都可以留在当前工作区中完成：

```bash
# 看仓库和待处理事项
gitcode repo view owner/repo
gitcode issue list -R owner/repo --state open
gitcode pr list -R owner/repo --state open

# 创建 Issue 和 PR
gitcode issue create -R owner/repo --title "修复登录超时" --body-file issue.md --json
gitcode pr create -R owner/repo --base main --title "fix: 修复登录超时" --body-file pr.md --json

# 评审与发布
gitcode pr diff 42 -R owner/repo
gitcode pr review 42 -R owner/repo --comment-file review.md
gitcode release create v1.0.0 -R owner/repo --title "v1.0.0" --notes-file CHANGELOG.md --json
```

命令参数、输出字段和平台限制以[完整命令手册](https://gitcode.com/gitcode-cli/cli/blob/main/docs/COMMANDS.md)为准。

### 自动化不必依赖脆弱的页面脚本

高频只读命令和主要写操作支持 `--json`。脚本可以解析稳定的标准输出，错误则通过标准错误和明确退出码返回。需要探索能力时，`gitcode schema` 可以直接给出命令树、参数和元数据；尚未封装成专用命令的接口，还可以通过 `gitcode api` 调用。

```bash
# 机器可读结果
gitcode issue list -R owner/repo --paginate --per-page 100 --json
gitcode pr view 42 -R owner/repo --json
gitcode actions run list -R owner/repo --status FAILED --json

# 让脚本或 AI 发现命令，而不是猜参数
gitcode schema
gitcode schema "pr create"

# 专用命令尚未覆盖时读取原始 API 响应
gitcode api repos/owner/repo
```

### AI 不只是“告诉你怎么做”，而是可以在边界内完成操作

网页适合人浏览，CLI 更适合 AI 执行。GitCode CLI 的结构化输出、命令元数据、非交互行为和确认机制，让 AI 可以完成“读取事实、分析、执行、核验”的闭环，同时避免因为等待交互输入而卡住。

例如，你可以直接对 AI 说：

> 查看 `owner/repo` 当前所有开放 PR，按风险排序，逐个总结改动和 CI 状态，只执行只读命令。

> 根据本地改动起草一个 Issue 和 PR 描述，先展示给我确认，再使用 GitCode CLI 提交。

> 找出 `main` 分支最近失败的 Actions 运行，定位失败 job，下载日志并给出根因判断。

面向 Codex、Claude 等 AI 客户端的可安装 skills 已独立维护在 [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills)。其中包括 Issue 创建与评审、PR 创建与评审、反馈修复、Release 发布、安全检查、流水线分析等端到端工作流。更完整的 AI 使用约定见[使用 AI 操作 GitCode 指南](https://gitcode.com/gitcode-cli/cli/blob/main/docs/AI-GUIDE.md)。

### 自动化有边界，危险动作不会悄悄发生

GitCode CLI 对删除等高风险操作提供 `--dry-run` 和确认保护。在非交互环境中，未明确确认的破坏性操作会直接失败，不会无限等待输入。对 AI 来说，这意味着“能执行”不等于“可自行授权”：只有用户明确批准后，AI 才应使用 `--yes` 跳过确认。

```bash
# 先预演，再由人确认是否执行
gitcode repo delete owner/repo --dry-run
gitcode release delete v1.0.0 -R owner/repo --dry-run
```

认证信息不应出现在聊天、Prompt、脚本参数、Issue 或 PR 正文中。登录必须由用户本人在私有、未录制且不由 AI 控制的本地终端完成，AI 只运行 `gitcode auth status` 确认认证是否可用。详细规则见[认证说明](https://gitcode.com/gitcode-cli/cli/blob/main/docs/AUTH.md)。

## 适合哪些场景

| 使用者 | 典型任务 | GitCode CLI 带来的价值 |
| --- | --- | --- |
| 日常开发者 | 查 Issue、创建 PR、查看 diff、回复评审 | 减少页面切换，让工作流留在代码旁边 |
| 项目维护者 | Issue 分诊、标签与里程碑、批量审查、版本发布 | 形成一致、可复用的项目治理动作 |
| 测试与发布人员 | 追踪变更、核对 Release、下载发布资产 | 用命令和 JSON 构建可重复的发布检查 |
| CI/CD 运维人员 | 查看 Actions run/job、下载日志和 Artifact、检查 Runner | 更快定位流水线和运行环境问题 |
| AI 编码代理 | 获取远端事实、提交 Issue/PR、评审、核验结果 | 获得可发现、结构化、受约束的执行接口 |
| 企业自动化平台 | 跨仓库统计、流水线巡检、标准化交付 | 以统一 CLI 代替零散 API 脚本和页面自动化 |

仓库中已经整理了可直接复用的真实场景，包括 Issue 到 PR 的完整链路、发布评审、CI 流水线定位、安全检查和 AI 全流程交付，见 [GitCode CLI 应用案例库](https://gitcode.com/gitcode-cli/cli/tree/main/Example)。

## 从今天的一件小事开始

不必先改造整套研发流程。装好后，直接给 AI 一个小任务：让它把某个仓库的待评审 PR 汇总一遍、找出最近一次失败流水线、或在只读模式下分析一个 Issue/PR。AI 用 `gitcode` 在边界内跑完，把结论和证据交给你——这是 GitCode CLI 想给你的协作体验。

当一个操作可以被命令准确表达，它就可以被保存、复用、审计，也可以安全地交给自动化和 AI。GitCode CLI 的价值，正是把 GitCode 上分散的协作动作，变成团队可以持续积累的工程能力。
