# AI 本地开发流程

本文档编排 gitcode-cli 仓库内 AI 协作者执行本地开发任务时的标准闭环。

本文件不新增项目规则；它把现有 `spec/`、命令文档、模板和 AI 协作资产串成一条可执行流程。

## 职责

- 定义 AI 接到开发任务后的标准读取顺序
- 明确哪些事实看本地文档，哪些事实看远端平台
- 统一从 issue 验证到本地验证到 PR 自检的执行链路

## 适用场景

- 修复 bug
- 开发功能
- 文档或流程修改
- 仓库内部 AI 协作开发

## 不适用场景

- 外部项目通过 AI 使用 `gc` 操作 GitCode
- 外部项目安装或使用 `gitcode-cli/skills`
- 不涉及本仓库开发的纯使用咨询

## 先看什么

AI 接到任务后，按以下顺序建立上下文：

1. `AGENTS.md` 或 `CLAUDE.md`
2. [spec/README.md](../README.md)
3. 与任务直接相关的 `spec/foundations/*`、`spec/workflows/*`、`spec/delivery/*`
4. 如涉及命令行为，继续看 [../../docs/COMMANDS.md](../../docs/COMMANDS.md)
5. 如需 GitCode CLI Skill，进入独立仓库 [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills)

## 事实判定边界

- 项目规则：看 `spec/`
- 命令行为：看 `docs/COMMANDS.md`
- 单个 issue / PR 当前状态：看远端 GitCode 平台
- 是否已主干合入：看 merged PR 和 `origin/main`
- GitCode 原生 CI 状态与结果：通过 `gc actions run/job` 获取
- GitHub 镜像 CI 状态与结果：通过 `gh run view` 获取
- 阶段背景或收口说明：可参考 `issues-plan/PROGRESS.md`
- 外部项目 AI 使用说明：`docs/AI-GUIDE.md`，不适用于本仓库内部开发规则

## 标准执行流程

```text
接收任务
→ 读取正式规范与事实源
→ 核验 issue / PR 当前状态
→ 验证问题或确认需求
→ 创建非 main 分支
→ 开发实现
→ 本地测试与构建
→ 真实命令验证
→ 推送分支 + GitCode 原生 CI 自动验证（PR 触发）
→ 有 GitHub 镜像 PR 时补跨平台 CI 验证
→ 安全审查
→ 文档同步
→ 风险分级
→ 补 issue 进度与 PR 自检
→ ready-for-review
→ 独立执行主体评审
→ merge
```

## 任务清单维护

AI 执行交付时，须维护一份与门禁清单（见 [development-workflow.md §5.3](./development-workflow.md) 的 In-Progress 阶段门禁表）对齐的细粒度任务清单，并实时反映进度：

- 拆分粒度对齐到单个门禁或子任务，不把多个门禁合并为一项
- 任一时刻仅 1 项 in-progress；完成即翻为 completed，再推下一项，不批量标记多项完成（此处 in-progress/completed 指任务清单项状态，区别于 issue/PR 的 `status/*` 标签）
- 每项完成时在对话中留下证据（命令或结果摘要）

目的：让交付进度可审计、可回放，避免批量跳步或“看起来像完成”。清单的具体工具由客户端决定，上述粒度与实时性要求适用于所有 AI 客户端（Codex、Claude、人工）。

## 详细步骤

### 1. 核验事实

开始修改前，必须先确认：

- 当前 issue / PR 的远端状态
- 是否已有 merged PR
- `origin/main` 是否已包含对应改动
- 如果 issue 已关闭，是否仍缺少 merged PR 或主干代码

建议使用最小远端事实核验脚本：

```bash
python3 scripts/verify-remote-facts.py --repo owner/repo --issue <issue> --pr <pr> --head-sha <sha>
```

如果已传入 `--pr`，脚本会优先从 PR 标题和正文中自动提取 `#123` 这类 issue 引用；在常见 `Refs #123` 场景下可减少显式传 `--issue`。

不得把 `issues-plan/PROGRESS.md` 当成上述事实的唯一依据。

若 issue 已关闭，但不存在 merged PR 或 `origin/main` 不包含对应改动，必须明确判定为“未完成主干合入”。

### 2. 验证问题

未完成验证，不得开始写代码。

最小要求：

- 复现问题或确认需求有效
- 留下结构化验证记录
- 进入 `status/verified`

### 3. 创建开发分支

只有在完成验证后，才允许创建非 `main` 开发分支并开始修改实现文件。

### 4. 本地开发

开发过程中必须遵守：

- 命令行为以 `docs/COMMANDS.md` 为准
- 质量门禁以 `spec/foundations/code-quality-gates.md` 为准
- 共享 AI 约束不得覆盖 `spec/`

### 5. 本地验证

最小验证命令集：

```bash
go test ./...
go build -o ./gc ./cmd/gc
./gc version
./scripts/regression-core.sh
go test -tags=system ./tests/system
```

`./scripts/regression-core.sh` 是核心冒烟回归脚本；`go test -tags=system ./tests/system` 是结构化真实命令系统测试（只读路径，需本地认证）。两者均要求 `infra-test/*` 仓库且不得读取或打印真实 token。

如果改动影响具体命令行为，还必须补至少一个真实命令验证。

真实命令测试只能使用 `infra-test/*`。

### 5bis. 远端 CI 验证

本地验证通过后，推送分支到 GitCode 并创建 PR。GitCode 原生 CI 在 PR 提交时自动触发
（`on: pull_request`），AI 通过 `gc actions` 核验结果：

```bash
# 查看 GitCode PR 的 CI 运行、详情和 Jobs
gc actions run list -R atomgit-cli/cli --pr <pr-number> --workflow "CI" --json
gc actions run view <run-id> -R atomgit-cli/cli --json
gc actions job list <run-id> -R atomgit-cli/cli --json
```

如果存在对应的 GitHub 镜像 PR，再通过 `gh` 核验跨平台 CI：

```bash
gh run list --workflow=ci.yml --branch <pr-branch> --limit 1
gh run watch $(gh run list --workflow=ci.yml --branch <pr-branch> --limit 1 --json databaseId --jq '.[0].databaseId')
CONCLUSION=$(gh run view <run-id> --json conclusion --jq '.conclusion')
echo "CI conclusion: $CONCLUSION"
```

CI 通过标准：

- GitCode 原生 Job（lint / test / build / package）全部通过
- 有 GitHub 镜像 PR 时，GitHub 所有 Job 通过，且 `test` / `build` 覆盖 ubuntu / macOS / Windows

CI 失败处理：

- GitCode：通过 `gc actions job log <run-id> <job-id> -R atomgit-cli/cli --output job-log.zip` 获取失败日志
- GitHub：通过 `gh run view <run-id> --log --job=<job-id>` 获取失败日志
- 分析根因并修复，修复后重新推送并重新触发 CI
- 如是环境/平台偶发问题（非代码问题），在自检中记录后仍可继续

docs-only 改动可跳过 CI，但必须在自检中说明。

详细规范见 `spec/delivery/ci-workflows.md`。

### 6. AI 友好 CLI 约束

AI 或脚本消费 `gc` 时，应优先使用：

- 读取类命令：`--json`
- 命令发现：`gc schema`
- 高风险写操作：先 `--dry-run`
- 删除或确认类命令：非交互场景中显式传 `--yes`

### 7. 安全审查

进入 ready-for-review 前，必须完成最小安全审查。

至少检查：

- 无硬编码 token、password、secret
- 文档、测试和示例中未误写真实凭证
- 涉及认证、配置、权限、网络调用、删除或覆盖行为时，已对照 `spec/foundations/security.md` 检查
- 如存在安全影响，已在作者自检和评审中明确记录

### 8. 文档同步

按改动类型同步相关文档：

- 命令行为变化：`docs/COMMANDS.md`，必要时 `README.md`
- 流程或门禁变化：`spec/*`、`AGENTS.md`、`CLAUDE.md`
- agent-friendly CLI 变化：`spec/foundations/agent-friendly-cli.md`、`docs/REGRESSION.md`
- 构建打包变化：`spec/delivery/*`、`docs/PACKAGING.md`
- AI 协作边界变化：`spec/governance/*`、`AGENTS.md`、`CLAUDE.md`
- Skill 行为需要变化：在独立 `gitcode-cli/skills` 仓单独建立 Issue / PR

### 9. 风险分级

提交评审前，必须执行最小风险分级：

```bash
python3 scripts/classify-change-risk.py --base origin/main
```

结果用于决定评审策略：

- `low`：第二个 AI 代理可作为独立执行主体评审并自动推进
- `medium`：第二个 AI 代理可作为独立执行主体评审；仅在 blocker 或不确定时升级人工
- `high`：独立 AI 评审后仍需人工最终确认

### 10. 证据留存

提交评审前，至少应留存以下证据：

- issue 验证记录
- issue 开发进度记录
- 本地测试结果
- 构建结果
- CI 验证结果（run ID / URL + 各 Job 结论）
- 真实命令验证结果
- 安全审查结果
- 文档同步结果
- 风险分级结果
- PR 作者自检
- 作者主体标识与评审主体标识

模板见：

- [../../docs/AI-TEMPLATES.md](../../docs/AI-TEMPLATES.md)
- `docs/ai-templates/*.md`

如使用本地文件准备 issue / PR 评论，建议在提交前运行最小机器校验：

```bash
python3 scripts/validate-ai-record.py --mode record --kind pr-self-check /path/to/pr-self-check.md
```

## 禁止事项

- 不得在 `main` 直接开发
- 不得跳过 `status/verified`
- 不得缺少证据就宣称完成
- 不得把作者自检当独立评审
- 不得让作者与评审者是同一执行主体
- 不得把 `issues-plan/PROGRESS.md` 当成实时事实源
- 不得把 `docs/AI-GUIDE.md` 当成仓库内部开发规则
- 不得在 PR body 自检/描述文本裸写 `#NNN`（GitCode 对裸 `#NNN` 也触发自动关闭，会误关未交付 issue；仅在 `Closes #XXX` 行使用 `#NNN`，其余用文字描述）

## 下一步去看哪里

- Issue 级动作：看 [issue-workflow.md](./issue-workflow.md)
- PR 级动作：看 [pr-workflow.md](./pr-workflow.md)
- 独立评审：看 [review-workflow.md](./review-workflow.md)
- 本地验证：看 [test-workflow.md](./test-workflow.md)
