# 流程状态更新 Checklist

本文档提供 Issue/PR 状态标签更新的简明 checklist，确保流程推进时状态标签同步更新。

## 重要说明

**`gc pr edit --labels` 是替换行为，不是追加。**

每次使用 `--labels` 更新状态时，必须携带 **完整四维标签**，否则其他维度标签会丢失。

四维标签要求：
- **status**: 状态标签（随流程推进变化）
- **type**: 类型标签（feature/bug/docs/refactor）
- **risk**: 风险标签（low/medium/high）
- **scope**: 范围标签（auth/repo/issue/pr/release/docs/testing）

此外还有**第五维（如有则必须保留）**：

- **sig**: 治理归属标签（`sig/<name>`，与 `spec/governance/sigs/<name>.yaml` 对应）。triage 时默认不打 `sig/*`，仅当需要 SIG 治理介入时打标（规则见 [sig-governance.md](../governance/sig-governance.md) §6.1）；一旦打上，后续每次 `--labels` 更新都必须携带，否则会被静默移除

## Issue 状态更新时机

| 步骤 | 操作命令 | 必须添加 | 必须移除 |
|------|----------|----------|----------|
| 创建 Issue 后 | `gc issue label <n> --add type/bug,status/triage,scope/xxx -R owner/repo` | `type/*`, `status/triage`, `scope/*` | - |
| 验证完成后 | `gc issue label <n> --add status/verified -R owner/repo` | `status/verified` | `status/triage` |
| 开始开发时 | `gc issue label <n> --add status/in-progress -R owner/repo` | `status/in-progress` | - |
| PR 合入后 | `gc issue label <n> --add status/merged -R owner/repo && gc issue close <n> -R owner/repo` | `status/merged` | `status/in-progress`, `status/triage` |

## PR 状态更新时机

**注意**：`gc pr edit --labels` 是替换操作，每次必须携带完整四维标签（已打 `sig/*` 时须一并携带）。

| 步骤 | 操作命令 | 状态变化 |
|------|----------|----------|
| 创建 PR 后 | `gc pr edit <n> -R owner/repo --labels status/draft,type/*,risk/*,scope/*` | 设置初始四维标签 |
| 自检完成后 | `gc pr edit <n> -R owner/repo --labels status/self-checked,type/*,risk/*,scope/*` | status: draft → self-checked |
| 进入评审前 | `gc pr edit <n> -R owner/repo --labels status/ready-for-review,type/*,risk/*,scope/*` | status: self-checked → ready-for-review |
| 评审通过后 | `gc pr edit <n> -R owner/repo --labels status/approved,type/*,risk/*,scope/*` | status: ready-for-review → approved |
| 合入主干后 | `gc pr edit <n> -R owner/repo --labels status/merged,type/*,risk/*,scope/*` | status: approved → merged |

## Label 维度要求

### Issue 必须包含

- **类型**: `type/bug`, `type/feature`, `type/docs`, `type/refactor`
- **状态**: `status/triage`, `status/verified`, `status/in-progress`, `status/merged`
- **范围**: 开放命名空间，按命令/模块领域命名（示例：`scope/auth`, `scope/repo`, `scope/issue`, `scope/pr`, `scope/release`, `scope/docs`, `scope/testing`, `scope/api`, `scope/actions`, `scope/ci` 等，可按需扩展）
- **风险**: `risk/low`, `risk/medium`, `risk/high` (可选，验证后添加)

### PR 必须包含

- **类型**: `type/bug`, `type/feature`, `type/docs`, `type/refactor`
- **状态**: `status/draft`, `status/self-checked`, `status/ready-for-review`, `status/approved`, `status/merged`
- **范围**: `scope/*` (与 Issue 对应)
- **风险**: `risk/low`, `risk/medium`, `risk/high` (自检时添加)

## 快速参考命令

```bash
# Issue 状态推进（issue label 是增量操作）
gc issue label 123 --add status/verified --remove status/triage -R owner/repo
gc issue label 123 --add status/in-progress -R owner/repo
gc issue label 123 --add status/merged --remove status/in-progress,status/triage -R owner/repo

# PR 状态推进（pr edit --labels 是替换操作，必须携带完整四维标签）
gc pr edit 456 -R owner/repo --labels status/draft,type/feature,risk/low,scope/pr
gc pr edit 456 -R owner/repo --labels status/self-checked,type/feature,risk/low,scope/pr
gc pr edit 456 -R owner/repo --labels status/ready-for-review,type/feature,risk/low,scope/pr
gc pr edit 456 -R owner/repo --labels status/approved,type/feature,risk/low,scope/pr
gc pr edit 456 -R owner/repo --labels status/merged,type/feature,risk/low,scope/pr
# 已打 sig/* 标签时，状态推进必须携带（替换语义，遗漏即丢失）：
gc pr edit 456 -R owner/repo --labels status/self-checked,type/feature,risk/low,scope/pr,sig/pr
```

---

**重要**: 状态标签更新是流程关键步骤，不得跳过或延迟执行。
