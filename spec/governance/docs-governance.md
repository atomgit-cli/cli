# 文档治理规范

本文件定义 gitcode-cli 仓库的文档体系、AI 协作文档分层、唯一真相源和分阶段实施方案。

## 职责

定义文档分层、唯一真相源、AI 协作入口边界和变更后的同步规则。

## 适用场景

- 修改命令行为说明
- 修改 AI 入口文档或 skills
- 调整项目规范、README、docs、issues-plan 之间的边界

## 必须

- 以 `spec/` 作为项目规则唯一来源
- 明确每类信息的唯一真相源
- 行为变化后检查相关入口文档和 skills

## 禁止

- 只改代码不检查文档同步
- 在 AI 文档中定义与 `spec/` 冲突的规则
- 把已废弃的 `.ai/skills/` 或客户端本地 `.claude/skills/`、`.codex/skills/` 当成项目 Skill 真相源

## 同步要求

- 命令、流程、发布、审查、AI 协作变化后都要检查对应入口文档和 skills
- 状态变化后同步 `issues-plan/PROGRESS.md`

## 不负责什么

- 命令实现细节
- 代码风格
- PR blocker 判定

## 1. 目标

本仓库的文档治理目标是：

- 为用户、开发者、维护者和 AI 协作者提供清晰入口
- 降低 README、docs、spec、skills 之间的重复
- 明确每类信息的唯一真相源
- 明确 Claude 与 Codex 并存时的协作边界
- 让文档同步成为正式交付要求，而不是个人习惯

## 2. 文档分层

### 2.1 README.md

`README.md` 是项目总入口，负责：

- 项目简介
- 安装与快速开始
- 文档导航
- 开发入口
- AI 协作入口

`README.md` 不应承载完整命令手册、完整开发流程或历史规划细节。

### 2.2 docs/

`docs/` 是用户文档层，负责：

- 命令行为说明
- 用户操作说明
- 安装、打包、发布的使用说明
- SIG 与 RFC 的贡献者导航（`docs/SIGS.md`、`docs/rfcs/README.md`，规则本体在 `spec/governance/sig-governance.md` 与 `spec/workflows/rfc-workflow.md`）

其中 `docs/COMMANDS.md` 是命令行为唯一真相源。
`docs/AI-GUIDE.md` 只服务外部项目通过 AI 使用 `gc`，不定义 gitcode-cli 仓库内部开发流程。

### 2.3 spec/

`spec/` 是项目正式规范层，负责：

- 开发流程
- 编码规范
- 测试规范
- 安全规范
- 构建与打包规范
- 发布流程规范
- 代码质量门禁
- 文档治理规范

`spec/` 是项目规则唯一来源。其他文档不得定义与 `spec/` 冲突的规则。

### 2.4 issues-plan/

`issues-plan/` 是规划与状态层，负责：

- 阶段计划
- 历史规划
- 当前进度状态

其中 `issues-plan/PROGRESS.md` 是阶段说明文档，不是远端 issue / PR 实时状态真相源。

### 2.5 AI 协作入口层

AI 协作入口层由以下文件组成：

- `AGENTS.md`
- `CLAUDE.md`

这两个文件负责告诉不同 AI 客户端如何进入本仓库的规范体系，不负责重新定义项目规则。

### 2.6 AI skill 层

GitCode CLI Skills 采用独立仓库治理：

- [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills)：Skill 真相源、版本管理和分发入口
- `~/.claude/skills/`：Claude 用户本地安装目录
- `~/.codex/skills/`：Codex 用户本地安装目录

本仓库不追踪 Skill 副本，也不提供从仓内目录生成客户端适配层的同步脚本。
项目开发规则仍由 `spec/` 定义，独立 Skills 仓不得覆盖本仓库规则。

## 3. 唯一真相源

本仓库的唯一真相源定义如下：

- 项目总入口：`README.md`
- 命令行为：`docs/COMMANDS.md`
- 项目正式规范：`spec/`
- SIG 元数据：`spec/governance/sigs/*.yaml`
- 真相源边界说明：`spec/governance/source-of-truth-matrix.md`
- 项目阶段说明：`issues-plan/PROGRESS.md`
- GitCode CLI Skills 真相源：[gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills)
- Codex 项目级入口：`AGENTS.md`
- Claude 项目级入口：`CLAUDE.md`

### 3.1 权威关系

权威关系按以下顺序生效：

1. `spec/` 定义项目正式规则
2. `docs/COMMANDS.md` 定义命令行为
3. `spec/governance/source-of-truth-matrix.md` 定义不同信息类型的事实边界
4. `AGENTS.md` / `CLAUDE.md` 定义 AI 入口
5. 独立仓库 `gitcode-cli/skills` 定义可安装的命令与工作流 Skills

`AGENTS.md`、`CLAUDE.md` 和独立 Skills 仓不得定义与 `spec/` 冲突的项目规则。
`issues-plan/PROGRESS.md` 只能作为阶段说明，不得作为单个 issue / PR 实时状态真相源。

## 4. 多 AI 协作规则

本仓库允许多人协作，并允许同时使用 Codex 和 Claude。

在此模式下，必须遵守以下规则：

- 任何开发流程、测试要求、安全要求都以 `spec/` 为准
- `AGENTS.md` 负责 Codex 侧入口说明
- `CLAUDE.md` 负责 Claude 侧入口说明
- GitCode CLI Skills 从独立仓库安装
- 用户本地 Skill 安装目录不得提交到本仓库

AI 协作文档属于正式项目文档，行为变更后必须纳入同步检查。

## 5. skill 体系设计

独立 Skills 仓中的 Skill 分为两类：

- 核心命令 Skill：描述认证、仓库、Issue、PR、Release 等命令族
- 工作流 Skill：描述 Issue、PR、评审、发布、安全检查等端到端任务

治理边界是：

- 本仓库维护产品代码、命令文档和项目开发规范
- `gitcode-cli/skills` 维护可安装 Skills
- Skill 不得依赖本仓库私有或不稳定的内部路径
- 用户本地安装目录仅作为运行时副本，不作为项目规则真相源
- 命令或流程变化影响 Skill 时，应在独立仓库建立对应 Issue / PR，不在本仓库复制修改

## 6. 文档同步规则

### 6.1 命令行为变化

命令行为变化时，必须检查并同步：

- `README.md`
- `docs/COMMANDS.md`
- `AGENTS.md`
- `CLAUDE.md`
- 独立 Skills 仓中的相关 Skill（如受影响，单独提交）

### 6.2 开发流程变化

开发流程变化时，必须检查并同步：

- `spec/*`
- `AGENTS.md`
- `CLAUDE.md`
- 独立 Skills 仓中的相关工作流 Skill（如受影响，单独提交）

### 6.3 审查流程变化

审查流程变化时，必须检查并同步：

- `spec/workflows/review-workflow.md`
- `AGENTS.md`
- `CLAUDE.md`
- 独立 Skills 仓中的审查相关 Skill（如受影响，单独提交）

### 6.4 构建、打包、发布规则变化

构建、打包、发布规则变化时，必须检查并同步：

- `spec/delivery/build-and-package.md`
- `spec/delivery/release-process.md`
- `AGENTS.md`
- `CLAUDE.md`
- 独立 Skills 仓中的发布相关 Skill（如受影响，单独提交）

### 6.5 状态变化

阶段计划或当前状态变化时，必须检查并同步：

- `issues-plan/PROGRESS.md`
- `issues-plan/README.md`

## 7. 规范补齐计划

当前 `spec/` 目录已具备开发、测试、安全、build、release、quality、workflow 和 CI 基础规范，规范补齐计划已完成。

## 8. 分阶段实施方案

### 阶段 1：治理基线

目标：先建立规则和边界。

交付物：

- `spec/governance/docs-governance.md`
- `spec/delivery/build-and-package.md`
- `spec/delivery/release-process.md`
- `spec/foundations/code-quality-gates.md`
- 更新 `spec/README.md`

验收标准：

- 明确各目录职责
- 明确唯一真相源
- 明确文档同步规则
- 明确后续规范补齐顺序

### 阶段 2：入口收口

目标：让用户、开发者、Codex、Claude 都知道先看哪里。

交付物：

- 调整 `README.md`
- 新增 `docs/README.md`
- 调整 `AGENTS.md`
- 调整 `CLAUDE.md`

验收标准：

- `README.md` 成为总入口
- `docs/` 和 `spec/` 的边界清晰
- AI 协作入口清晰

### 阶段 3：AI skill 体系重构（历史）

该阶段曾在本仓库建立共享源和客户端适配层，现已被独立 Skills 仓替代。

交付物：

- 新增 `.ai/README.md`
- 新增 `.ai/skills/`
- 新增 `.codex/skills/`
- 梳理 `.claude/skills/`

这些目录只用于解释历史演进，不再是当前交付物或验收标准。

### 阶段 4：内容去重与迁移

目标：减少重复和漂移。

交付物：

- 压缩 `README.md`
- 收口 `docs/COMMANDS.md`
- 清理 `spec/` 与 AI 文档的重复
- 明确 `issues-plan/` 的历史与当前边界

验收标准：

- README 不再承载重复命令细节
- 命令行为只在 `docs/COMMANDS.md` 定义
- AI 文档不再复制通用规范

### 阶段 5：分发与同步工具（历史）

该阶段曾通过仓内同步脚本复用 Skill，现已由独立仓库的版本管理和安装说明替代。

仓内同步脚本和适配层不再保留。

### 阶段 6：CI 自动化 ✅

目标：在 GitCode 主仓与 GitHub 镜像仓 CI 环境落地自动化约束。

交付物：

- `spec/delivery/ci-workflows.md` ✅
- `.gitcode/workflows/ci.yml` ✅
- `.github/workflows/ci.yml` ✅
- `.github/workflows/release.yml` ✅
- 质量门禁的 CI 映射 ✅

验收标准：

- CI 规范基于 GitCode Actions 与 GitHub Actions 真实环境，而不是纸面设计 ✅
- 本地门禁和 CI 门禁保持一致 ✅
- AI 通过 `gc actions` / `gh` 查询运行与日志 ✅

### 阶段 7：Skills 独立仓迁移 ✅

目标：消除仓内 Skill 副本与客户端适配层漂移。

交付物：

- 独立仓库 [gitcode-cli/skills](https://gitcode.com/gitcode-cli/skills)
- `docs/AI-GUIDE.md` 安装说明
- 用户本地 Claude / Codex Skill 安装目录

验收标准：

- 本仓库不追踪 `.ai/skills/`、`.claude/skills/` 或 `.codex/skills/`
- 本仓库不保留 Skill 分发副本或同步生成脚本
- 现行规范统一指向独立 Skills 仓

## 9. 当前执行原则

当前 7 阶段整改已全部完成。

当前优先级固定为：

1. 保持 `foundations/`、`workflows/`、`delivery/`、`governance/` 的边界稳定
2. 同步更新入口文档；需要更新 Skill 时在独立仓库单独交付
3. CI 规范已基于 GitCode Actions 与 GitHub Actions 真实环境落地，AI 通过 `gc actions` / `gh` 查询运行与日志

## 下一步去看哪里

- 如果你在修改命令行为，继续看 [docs/COMMANDS.md](../../docs/COMMANDS.md)
- 如果你在准备提交，继续看 [代码质量门禁规范](../foundations/code-quality-gates.md)
