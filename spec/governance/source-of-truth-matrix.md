# 真相源矩阵

本文档定义 gitcode-cli 仓库中不同信息类型的真相源、适用边界和判定优先级。

## 职责

- 明确“哪类信息该去哪看”
- 防止把入口文档、阶段说明或外部使用说明误当成正式规则源
- 统一人工与 AI 在判定事实时的依据

## 适用场景

- 判断某份文档能否直接作为事实依据
- 识别仓库内开发与外部项目使用 `gc` 的边界
- 处理文档之间出现冲突或信息滞后时的优先级

## 真相源矩阵

| 信息类型 | 真相源 | 可否直接判事实 | 说明 |
|------|------|------|------|
| 项目正式规则 | `spec/` | 是 | 项目规则唯一正式来源 |
| 命令行为 | `docs/COMMANDS.md` | 是 | `gc` 命令行为唯一真相源 |
| SIG 元数据（SIG 列表、角色、范围、`scope_labels` 映射、Agent 登记与授权） | `spec/governance/sigs/*.yaml` | 是 | SIG 治理变更走 PR；`docs/SIGS.md`、label 描述均须从 YAML 派生 |
| RFC 流程规则 | `spec/workflows/rfc-workflow.md` | 是 | RFC 文件存放于 `docs/rfcs/`，但流程规则以 spec 为准 |
| 测试、门禁、评审规则 | `spec/foundations/*`、`spec/workflows/*` | 是 | 包括测试、状态机、门禁、评审边界 |
| 构建与打包规则 | `spec/delivery/*` | 是 | 当前本地构建与打包规则以此为准 |
| GitCode CLI Skills | [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills) | 有条件地是 | 定义可安装的命令与工作流 Skill，不得覆盖本仓库 `spec/` |
| 项目入口导航 | `README.md` | 否 | 入口文档，不是规则源 |
| AI 客户端入口 | `AGENTS.md`、`CLAUDE.md` | 否 | 入口文档，不是规则源 |
| 外部项目 AI 使用说明 | `docs/AI-GUIDE.md` | 否 | 只服务外部项目通过 AI 使用 `gc` |
| 项目阶段说明 | `issues-plan/PROGRESS.md` | 否 | 可能滞后，只能作阶段说明与背景参考 |
| 单个 issue / PR 的实时状态 | GitCode 远端 issue、PR、label、comment | 是 | 远端平台是实时事实源 |
| 是否已主干合入 | merged PR + `origin/main` | 是 | 不能只看 issue 状态、comment 或 release 文案 |
| GitCode 原生 CI 状态与结果 | GitCode Actions run（通过 `gc actions` 获取） | 是 | GitCode PR 的 Linux 自动化事实 |
| GitHub 镜像 CI 状态与结果 | GitHub Actions run（通过 `gh run view` 获取） | 是 | 跨平台自动化事实 |
| CI 工作流定义 | `.gitcode/workflows/ci.yml`、`.github/workflows/ci.yml` + `spec/delivery/ci-workflows.md` | 是 | workflow 定义行为，spec 定义平台边界 |
| GitCode API 端点/path/body | `api-doc/`（gc-api-doc submodule 副本）+ 远端 `gitcode-cli/gc-api-doc` 仓 | 有条件地是 | submodule 锁定特定 commit，需定期 `git submodule update --remote api-doc` sync；查证时若与远端实测不符，以远端平台实测为准（判定优先级第 3）|

## 判定优先级

当不同文档或信息源出现冲突时，按以下顺序理解：

1. `spec/` 定义项目正式规则
2. `docs/COMMANDS.md` 定义命令行为
3. GitCode 远端平台、merged PR 和 `origin/main` 定义实时事实
4. GitCode Actions 与 GitHub Actions CI 运行结果定义各自平台的自动化验证事实
5. 独立仓库 `gitcode-cli/skills` 定义可安装的 GitCode CLI Skills
6. `README.md`、`AGENTS.md`、`CLAUDE.md`、`issues-plan/PROGRESS.md` 仅作入口、导航或阶段说明

## 必须

- 不得把入口文档当成正式规则源
- 不得把 `issues-plan/PROGRESS.md` 当成实时状态真相源
- 不得把 `docs/AI-GUIDE.md` 当成 gitcode-cli 仓库内部开发流程规范
- 不得把已废弃的 `.ai/skills/` 或客户端本地 `.claude/skills/`、`.codex/skills/` 当成本仓库的规则源
- 判断交付完成度时，必须检查远端平台事实和 `origin/main`
- 查证 API 端点/path/body 时以 `api-doc/` submodule 为参考；若与远端实测行为不符，以远端平台实测为准（submodule 锁定 commit 可能滞后于平台）

## 下一步去看哪里

- 如果你在改文档边界，继续看 [docs-governance.md](./docs-governance.md)
- 如果你在改 AI 入口或 skill 分层，继续看 [ai-collaboration.md](./ai-collaboration.md)
- 如果你在执行仓库内 AI 开发流程，继续看 [../workflows/ai-local-development-workflow.md](../workflows/ai-local-development-workflow.md)
