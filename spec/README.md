# 规范入口

`spec/` 是 gitcode-cli 的正式规范目录，也是 Codex、Claude 和人工维护者在本仓库中执行开发任务时的唯一规则源。

如果你要修改代码、文档、流程、发布规则或 AI 协作规则，先从本目录开始。

## 进入开发前的最小阅读集

首次进入本仓库，至少先读以下 4 篇：

1. [开发工作流程](./workflows/development-workflow.md)
2. [编码规范](./foundations/coding-standards.md)
3. [测试指南](./foundations/testing-guide.md)
4. [代码质量门禁规范](./foundations/code-quality-gates.md)

如果改动涉及命令行为、文档同步或 AI 协作边界，再补读：

5. [文档治理规范](./governance/docs-governance.md)

## 按任务选择入口

| 你正在做什么 | 先读这些文档 |
|------|------|
| 改命令行为、参数、用户可见输出 | [开发工作流程](./workflows/development-workflow.md)、[文档治理规范](./governance/docs-governance.md)、[代码质量门禁规范](./foundations/code-quality-gates.md) |
| 改 API、认证、配置、错误处理 | [编码规范](./foundations/coding-standards.md)、[安全规范](./foundations/security.md)、[测试指南](./foundations/testing-guide.md) |
| 补测试、修回归、做真实命令验证 | [测试指南](./foundations/testing-guide.md)、[测试流程](./workflows/test-workflow.md) |
| 提交 PR、补 issue comment、补 review comment | [PR 流程](./workflows/pr-workflow.md)、[评审流程](./workflows/review-workflow.md)、[代码质量门禁规范](./foundations/code-quality-gates.md) |
| 改构建、打包、发布 | [本地构建与打包规范](./delivery/build-and-package.md)、[发布流程规范](./delivery/release-process.md) |
| 改 CI 工作流、远端验证 | [CI 工作流规范](./delivery/ci-workflows.md)、[代码质量门禁规范](./foundations/code-quality-gates.md) |
| 改文档、skills、AGENTS、CLAUDE | [文档治理规范](./governance/docs-governance.md)、[代码质量门禁规范](./foundations/code-quality-gates.md) |

## 当前结构

```
spec/
├── README.md                # 本文件：规范入口
├── foundations/             # 基础规则
│   ├── coding-standards.md
│   ├── testing-guide.md
│   ├── security.md
│   ├── code-quality-gates.md
│   ├── agent-friendly-cli.md
│   └── command-template.md
├── workflows/               # 操作流程
│   ├── development-workflow.md
│   ├── ai-local-development-workflow.md
│   ├── issue-workflow.md
│   ├── pr-workflow.md
│   ├── review-workflow.md
│   └── test-workflow.md
├── delivery/                # 构建与交付
│   ├── build-and-package.md
│   ├── release-process.md
│   └── ci-workflows.md
└── governance/              # 治理与协作边界
    ├── docs-governance.md
    ├── source-of-truth-matrix.md
    ├── ai-collaboration.md
    ├── sig-governance.md
    └── sigs/                # SIG 元数据（api / cli-ux / governance）
```

## 权威边界

本目录定义的规则具有以下边界：

- `spec/` 是项目正式规范唯一来源
- `docs/COMMANDS.md` 定义命令行为
- `README.md` 是项目总入口，不是规范源
- `AGENTS.md` 和 `CLAUDE.md` 是 AI 协作入口，不是项目规则源

如果其他文档与本目录冲突，以 `spec/` 为准。

## 任何改动都要检查的同步点

- 命令行为变化：`docs/COMMANDS.md`、`README.md`、相关 AI skills
- 流程变化：`spec/*`、`AGENTS.md`、`CLAUDE.md`
- 文档或 AI 协作规则变化：`docs-governance.md`、相关 skills、入口文档
- 阶段说明变化：`issues-plan/PROGRESS.md`

## 核心规范

| 文档 | 说明 |
|------|------|
| [文档治理规范](./governance/docs-governance.md) | 文档分层、唯一真相源、AI 协作分层、分阶段实施方案 |
| [真相源矩阵](./governance/source-of-truth-matrix.md) | 各类信息的事实来源、边界和判定优先级 |
| [AI 协作规范](./governance/ai-collaboration.md) | Codex / Claude 入口关系、共享源与适配层边界 |
| [SIG 治理规范](./governance/sig-governance.md) | SIG 角色与生命周期、决策机制、Agent 在 SIG 中的位置、sig/* label 与 scope/* 的关系 |
| [开发工作流程](./workflows/development-workflow.md) | 完整流程、分支规范、禁止行为、检查清单 |
| [RFC 工作流程](./workflows/rfc-workflow.md) | 大型设计提案（RFC）的触发判断、编号、流程与结果判定 |
| [AI 本地开发流程](./workflows/ai-local-development-workflow.md) | 仓库内 AI 从接任务到本地验证到评审前的编排流程 |
| [本地构建与打包规范](./delivery/build-and-package.md) | 标准构建命令、打包方式、产物边界和验证要求 |
| [发布流程规范](./delivery/release-process.md) | 版本规则、发布步骤、release notes 和发布后验证 |
| [CI 工作流规范](./delivery/ci-workflows.md) | 远端 CI 定位、AI 触发方式、Job 映射、证据纳入规则 |
| [代码质量门禁规范](./foundations/code-quality-gates.md) | 本地门禁、PR 门禁、合并门禁和 blocker 判定 |
| [编码规范](./foundations/coding-standards.md) | 命名规范、文件结构、错误处理、代码风格 |
| [测试指南](./foundations/testing-guide.md) | 单元测试、实际命令测试、测试仓库限制 |
| [Agent-Friendly CLI 规范](./foundations/agent-friendly-cli.md) | 结构化输出、非交互行为、错误契约和命令发现能力 |
| [命令开发模板](./foundations/command-template.md) | 新命令开发模板、API 客户端用法、输出处理 |
| [安全规范](./foundations/security.md) | Token 管理、敏感信息保护、安全审查 |

## 操作流程

| 文档 | 说明 |
|------|------|
| [Issue 流程](./workflows/issue-workflow.md) | Issue 创建、标签、验证、关闭 |
| [PR 流程](./workflows/pr-workflow.md) | 分支创建、代码提交、PR 创建与合并 |
| [评审流程](./workflows/review-workflow.md) | Issue 评论、PR 审查评论 |
| [测试流程](./workflows/test-workflow.md) | 单元测试、实际命令测试 |
| [AI 本地开发流程](./workflows/ai-local-development-workflow.md) | AI 协作者执行本地开发闭环的编排流程 |

## AI 使用建议

对 Codex 和 Claude 来说，推荐执行顺序是：

1. 先从本页判断任务类型
2. 再进入对应规范正文
3. 开始修改前回到 [代码质量门禁规范](./foundations/code-quality-gates.md) 确认交付标准
4. 判断某类信息能否直接作为事实依据时，先看 [真相源矩阵](./governance/source-of-truth-matrix.md)
5. 仓库内 AI 执行本地开发闭环时，继续看 [AI 本地开发流程](./workflows/ai-local-development-workflow.md)
6. 命令行为或协作规则变化时，回到 [文档治理规范](./governance/docs-governance.md) 检查同步范围

## 相关文档

| 文档 | 位置 | 说明 |
|------|------|------|
| README.md | 根目录 | 项目总入口 |
| AGENTS.md | 根目录 | Codex 项目级入口 |
| CLAUDE.md | 根目录 | Claude 项目级入口 |
| docs/COMMANDS.md | docs/ | 命令行为说明 |
| docs/PACKAGING.md | docs/ | 打包发布使用说明 |
| issues-plan/PROGRESS.md | issues-plan/ | 项目阶段说明 |

**最后更新**: 2026-09-17
