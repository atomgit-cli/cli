# docs 文档入口

`docs/` 是 gitcode-cli 的用户文档目录。

如果你是：

- 使用者：从本目录开始
- 开发者或 AI 协作者：先看 [spec/README.md](../spec/README.md)

## 权威边界

本目录的职责是：

- 解释用户如何使用 `gitcode`（或源码构建的 `gc`）
- 提供命令行为说明
- 提供认证、回归、打包等使用文档

本目录不是项目规则源。

项目正式规则以 [spec/README.md](../spec/README.md) 和 `spec/` 目录为准。

其中：

- [COMMANDS.md](./COMMANDS.md) 是命令行为唯一真相源

## 推荐阅读顺序

1. [INTRODUCTION.md](./INTRODUCTION.md)
2. [COMMANDS.md](./COMMANDS.md)
3. [AUTH.md](./AUTH.md)
4. [LARK.md](./LARK.md)
5. [REGRESSION.md](./REGRESSION.md)
6. [PACKAGING.md](./PACKAGING.md)
7. [AI-GUIDE.md](./AI-GUIDE.md)
8. [LOOP_ENGINEERING_MAINLINE_ANALYSIS.md](./LOOP_ENGINEERING_MAINLINE_ANALYSIS.md)
9. [AI-TEMPLATES.md](./AI-TEMPLATES.md)
10. `docs/ai-templates/*.md`
11. [应用案例库](../Example/index.md)

说明：

- [AI-GUIDE.md](./AI-GUIDE.md) 只服务“外部项目用 AI 操作 GitCode”
- 本仓库内部开发流程请看 `AGENTS.md`、`CLAUDE.md` 和 `spec/workflows/*`
- [AI-TEMPLATES.md](./AI-TEMPLATES.md) 提供 gitcode-cli 仓库内部协作模板，不是项目规则源

## 当前包含内容

| 文档 | 说明 |
|------|------|
| [INTRODUCTION.md](./INTRODUCTION.md) | GitCode CLI 产品介绍、核心价值、应用场景和快速上手指南 |
| [COMMANDS.md](./COMMANDS.md) | 命令行为说明和示例 |
| [AUTH.md](./AUTH.md) | 认证来源和行为说明 |
| [LARK.md](./LARK.md) | 飞书/Lark 通知（lark-cli 集成）安装、配置与发送指南 |
| [REGRESSION.md](./REGRESSION.md) | 核心回归矩阵说明 |
| [PACKAGING.md](./PACKAGING.md) | 本地打包与发布使用说明 |
| [AI-GUIDE.md](./AI-GUIDE.md) | 外部项目使用 AI 操作 GitCode 的场景指南 |
| [SIGS.md](./SIGS.md) | SIG（Special Interest Group）机制介绍与参与指南（规则见 `spec/governance/sig-governance.md`） |
| [rfcs/](./rfcs/) | 大型设计提案（RFC）目录与模板（流程规则见 `spec/workflows/rfc-workflow.md`） |
| [LOOP_ENGINEERING_MAINLINE_ANALYSIS.md](./LOOP_ENGINEERING_MAINLINE_ANALYSIS.md) | 当前主干 Loop Engineering 基因分析与演进计划 |
| [AI-TEMPLATES.md](./AI-TEMPLATES.md) | gitcode-cli 仓库内部协作的固定模板 |
| [LOOP-GOAL-GUIDE.md](./LOOP-GOAL-GUIDE.md) | Claude Code /loop 与 /goal 命令在本项目开发流程中的使用指南 |
| `docs/ai-templates/*.md` | 可直接复用的 AI 评论与检查模板文件 |
| [Example/](../Example/index.md) | GitCode CLI 在业务场景中的应用案例和可复制 prompt |
