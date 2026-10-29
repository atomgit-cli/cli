# AI 协作规范

## 职责

定义 Codex、Claude、本仓库规则与独立 Skills 仓之间的关系。

## 适用场景

- 调整 `AGENTS.md` 或 `CLAUDE.md`
- 调整 GitCode CLI Skill 的来源、安装或使用边界
- 设计多 AI 协作边界

## 必须

- 以 `spec/` 作为项目正式规则源
- 以 `AGENTS.md` 和 `CLAUDE.md` 作为各自客户端入口
- 以独立仓库 [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills) 作为 GitCode CLI Skills 真相源
- 废弃 `.ai/skills/` 仓内共享源；`.claude/skills/` 与 `.codex/skills/` 仅作为不入库的本地运行时目录
- 让作者主体与独立评审主体保持分离

## 禁止

- 在 AI 入口文档中定义与 `spec/` 冲突的项目规则
- 把任一客户端的适配层当作跨 AI 的唯一来源
- 在本仓库重新维护独立 Skills 仓的副本或同步生成脚本
- 让可分发 Skill 依赖本仓库私有 `spec/` 或 `docs/` 路径
- 让同一执行主体同时扮演作者和独立评审

## 同步要求

- AI 入口变化时同步 `AGENTS.md`、`CLAUDE.md`
- 本仓库命令或流程变化影响 Skill 时，在独立 `gitcode-cli/skills` 仓单独提交同步改动
- Skill 安装方式变化时同步 `docs/AI-GUIDE.md`

## 不负责什么

- 命令实现细节
- 一般编码规范
- 本地构建与发布流程
- SIG 治理语境下的 Agent 角色与行为边界（由 [sig-governance.md](./sig-governance.md) §4.3 负责）

## 权威关系

按以下顺序理解：

1. `spec/` 定义项目正式规则
2. `AGENTS.md` / `CLAUDE.md` 定义不同 AI 客户端如何进入规则体系
3. `spec/governance/source-of-truth-matrix.md` 定义哪些文档可用于事实判定
4. 独立仓库 `gitcode-cli/skills` 定义可安装的 GitCode CLI Skills
5. 用户本地 Skill 目录只承载安装副本，不定义项目规则

## 当前结构

- Codex 入口：[../../AGENTS.md](../../AGENTS.md)
- Claude 入口：[../../CLAUDE.md](../../CLAUDE.md)
- Skills 真相源：[gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills)
- 文档治理：[docs-governance.md](./docs-governance.md)

## 下一步去看哪里

- 如果你在改技能分发，进入独立仓库 [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills)
- 如果你在改同步边界，继续看 [docs-governance.md](./docs-governance.md)
- 如果你在执行仓库内 AI 本地开发，继续看 [../workflows/ai-local-development-workflow.md](../workflows/ai-local-development-workflow.md)
