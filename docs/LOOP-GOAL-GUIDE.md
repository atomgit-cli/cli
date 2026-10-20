# /loop 与 /goal 使用指南

本指南说明如何在 gitcode-cli 项目的 AI 开发流程中有效使用 Claude Code 的 `/loop` 和 `/goal` 命令。

**适用读者**：在本仓库中使用 Claude Code 进行开发的 AI 协作者及人工维护者。

**前提**：阅读本文前，应先理解项目的核心开发流程——参见 [spec/README.md](../spec/README.md) 和 [spec/workflows/development-workflow.md](../spec/workflows/development-workflow.md)。

---

## 1. /loop 与 /goal 概述

### 1.1 背景：AI 编程的自主化演进

AI 辅助编程经历了三个阶段：

1. **问答式**（2024-2025）：人问，AI 答，每次交互都需要人工发起
2. **任务式**（2025-2026）：AI 能完成多步骤任务，但仍需人工在每一步确认和推进
3. **自主式**（2026）：AI 能在无人干预的情况下持续工作，直到达成可验证的目标

`/loop` 和 `/goal` 是 Claude Code 在第三阶段提供的两种自主运行模式。它们解决同一个核心问题：**如何让 AI 在你不盯着屏幕的时候继续推进工作**。

两个命令都源于开源社区的实践。2026 年 4 月，澳大利亚开发者 Geoffrey Huntley 用三行 bash 脚本创建了"Ralph Loop"（以《辛普森一家》的 Ralph Wiggum 命名）：

```bash
while :; do
  cat PROMPT.md | claude-code --continue
done
```

这个极简的循环揭示了 AI 自主工作的可行性。短短 11 天内，OpenAI（Codex）、Anthropic（Claude Code）、Nous Research（Hermes）都发布了基于这一模式的官方实现。

### 1.2 /loop：定时循环执行

`/loop` 让 AI 按照固定时间间隔（或自主节奏）反复执行指定任务。

**工作方式**：

```
启动 → 执行任务 → 等待间隔 → 检查是否有新工作 → 执行 → 等待 → ...
                                                      ↓
                                          连续无事可做 → 自动收敛
```

**两种子模式**：

| 模式 | 用法 | 适用场景 |
|------|------|---------|
| **定时模式** | `/loop 5m <prompt>` | 已知节奏的周期性任务（如每 30 分钟检查新 Issue） |
| **自主模式** | `/loop`（无参数） | 让 AI 自己判断节奏，主动发现并推进未竟工作 |

**自主模式的行为逻辑**：

自主模式下的 AI 扮演"管家"角色——延续已有工作，而非发起新工作。它会：
- 重新阅读对话记录，找出未完成的任务
- 检查当前分支的 PR 状态（CI、review 意见、是否落后于 base）
- 处理能自动修复的问题（lint 错误、测试失败、合并冲突）
- 对不可逆操作（合并、删除、推送敏感数据）暂停并等待确认
- 连续三轮无事可做时自动收敛，避免空转

**核心设计理念**：信任来自延续，而非创新。自主模式的 AI 只推进已经在对话中明确的事情，不会自己发明新需求。

### 1.3 /goal：目标驱动冲刺

`/goal` 让 AI 持续工作直到达成一个**可衡量**的终点条件，由独立的评估器判定是否完成。

**工作方式**：

```
设定目标 → AI 工作一个回合 → 独立评估器判定 → 未完成（附原因）→ AI 继续 → ...
                                                  ↓
                                               已完成 → 自动停止
```

**关键设计**：**干活的模型不给自己打分**。评估器是一个独立的轻量模型（默认 Haiku），它只阅读对话文本来判断目标是否达成。这个"裁判与运动员分离"的设计防止 AI 自我欺骗——AI 不能自己说"我做完了"就算完。

**有效目标的三要素**：

```
/goal [做什么] until [可衡量的终点] without [约束条件]
```

| 要素 | 说明 | 示例 |
|------|------|------|
| 做什么 | 具体的工作内容 | "修复 auth 包的所有测试失败" |
| 可衡量的终点 | 可被评估器客观判定的条件 | `go test ./pkg/auth/... passes` |
| 约束条件 | 边界限制，防止越界 | "不修改 auth 以外的文件" |

**目标的"可衡量性"决定了 `/goal` 的可靠性**：

| 目标质量 | 示例 | 评估器可靠性 |
|---------|------|------------|
| ✅ 优秀 | `go test ./... exits 0` | 高——输出是标准化的 PASS/FAIL |
| ✅ 良好 | `grep -r "TODO" 返回空` | 高——文本匹配，结果明确 |
| ⚠️ 一般 | `所有 review 意见已回复` | 中——依赖 AI 如实报告 |
| ❌ 不可靠 | `代码质量足够好` | 无法判定——没有客观标准 |

### 1.4 两个命令的核心差异

| 维度 | `/loop` | `/goal` |
|------|---------|---------|
| **驱动方式** | 时间间隔或自主节奏 | 目标条件（直到 X 达成） |
| **结束条件** | 手动停止，或连续无事可做 | 独立评估器判定达成 |
| **评估机制** | AI 自己判断（无独立裁判） | 独立评估器（裁判与运动员分离） |
| **核心优势** | 全流程覆盖，跨平台无盲区 | 单阶段精准冲刺，不浪费 token |
| **核心劣势** | 无独立评估，可能遗漏问题 | 评估器只看文本，看不到远端 API 真实响应 |
| **适合的任务形态** | 开放式的、跨阶段的、需要持续关注 | 封闭式的、单阶段的、有明确验收条件 |
| **持久性** | 会话级（重启丢失） | 会话级（重启丢失） |

**一句话区分**：`/loop` 是"你干活，我过会儿回来看"；`/goal` 是"不达目标不罢休，裁判说行才行"。

---

## 2. 与本项目 AI 开发流程的结合

### 2.1 本项目流程的核心特点

gitcode-cli 项目的开发流程（定义于 `spec/workflows/development-workflow.md`）具有以下核心特点，这些特点决定了 `/loop` 和 `/goal` 的使用边界：

**五大原则**：

1. **先定义状态，再执行动作**——每步操作都有对应的状态标签
2. **先提交证据，再推进状态**——没有证据的状态推进不被承认
3. **作者实现、自检、独立执行主体评审三者分离**——不能自己写、自己查、自己批准
4. **本地自动检查和独立执行主体语义审查分层**——机器能做和需要人（或 AI 独立实例）判断的分开
5. **所有推进动作可回放、可审计**——每步都留痕

**状态机（Issue）**：

```
status/triage → status/verified → status/in-progress → status/ready-for-review → status/merged
```

**状态机（PR）**：

```
status/draft → status/self-checked → status/ready-for-review → status/approved → status/merged
```

**关键门禁**：

| 门禁 | 类型 | 谁来执行 |
|------|------|---------|
| `go test ./...` + `go build` | 本地自动化 | AI 或人工 |
| 真实命令验证（`infra-test/*`） | 本地自动化 | AI 或人工 |
| CI（GitCode Actions + GitHub Actions） | 远端自动化 | 系统自动 + AI 监控 |
| 安全审查 | 本地半自动 | AI（grep + 规则检查） |
| 文档同步 | 本地手动 | AI 或人工 |
| 风险分级 | 本地半自动 | AI（脚本 + 策略判断） |
| 多角色评审（8 Agent） | 独立语义审查 | 独立 AI 执行主体 |
| 人工最终确认（`risk/high`） | 人工判断 | 人工 |

### 2.2 将项目阶段映射到 /goal 和 /loop

项目的每个阶段有不同的"可衡量性"和"跨平台依赖"，这决定了适合用哪个命令：

```
项目阶段               可衡量性   跨平台依赖   推荐命令
─────────────────────────────────────────────────────
Issue Triage              高          无        /goal ✅
问题验证 (Verified)       高          无        /goal ✅
创建分支                  高          无        /goal ✅
开发实现                  中          无        /goal ✅
本地测试 (go test)        高          无        /goal ✅
真实命令验证              高          无        /goal ✅
安全审查                  高          无        /goal ✅
文档同步                  中          无        /goal ⚠️
推送到远端                高         有         /loop ✅
CI 等待与诊断             中         有         /loop ✅
自检模板补齐              高          有        /loop ✅
风险分级                  中          无        /goal ⚠️
多角色评审                语义        无        手动 ⛔
评审意见修复              中         有         /loop ✅
合并                      高         有         手动 ⛔
```

**符号说明**：✅ 推荐 ⚠️ 可用但需注意 ⛔ 禁止

**关键规律**：
- **有明确二进制输出的阶段**（测试通过/失败、构建成功/失败）→ `/goal` 最可靠
- **涉及远端平台操作的阶段**（push、CI 监控、PR 状态查询）→ `/loop` 更可靠
- **需要独立语义判断的阶段**（代码评审、架构评估）→ 禁止自动化，必须手动
- **有不可逆后果的阶段**（合并、删除）→ 手动确认

### 2.3 /loop 的自主模式如何适配项目原则

当 `/loop` 以自主模式运行时，它的行为天然适配本项目的五大原则：

| 项目原则 | /loop 自主模式的行为 |
|---------|---------------------|
| 先定义状态再执行动作 | 每轮检查当前状态标签，从状态决定下一步 |
| 先提交证据再推进状态 | 补齐证据（测试结果、验证记录）后才更新标签 |
| 三者分离 | 完成自检后停止，不尝试自己做评审 |
| 自动检查与语义审查分层 | 自动检查全部执行，语义审查留给手动步骤 |
| 可回放可审计 | 每步操作在对话中留痕 |

**/loop 不会做的事**（硬约束）：
- 在 `main` 分支直接开发
- 跳过 `status/verified` 直接写代码
- 把作者自检当作独立评审通过
- 在 PR 未合入前关闭 issue
- 连续三轮无事可做后仍空转

### 2.4 /goal 的评估器如何适配项目门禁

`/goal` 的独立评估器在项目中能做什么、不能做什么：

**评估器能可靠判定的门禁（推荐用于 /goal）**：

| 门禁 | 评估器判断依据 | 可靠性 |
|------|--------------|--------|
| `go test ./...` | 对话中的 PASS/FAIL 输出 | 高 |
| `go build` | 对话中的构建成功/失败输出 | 高 |
| 安全扫描 | 对话中的 grep 结果 | 高 |
| 自检模板完整性 | 模板字段是否逐一填写 | 高 |
| 标签更新 | 对话中 `gc issue label` 的返回 | 中 |

**评估器无法可靠判定的门禁（不要用于 /goal）**：

| 门禁 | 原因 |
|------|------|
| CI 状态 | 评估器看不到 `gh run view` 的真实响应 |
| PR 合并状态 | 评估器看不到 GitCode 的远端状态 |
| 代码评审结果 | 需要语义判断，不是文本匹配 |
| 文档同步完整性 | 需要对比多个文件，超出评估器能力 |

---

## 3. 两个命令的定位

（本章为快速对比参考，详细原理见第 1 章。）

| 维度 | `/loop` | `/goal` |
|------|---------|---------|
| **驱动方式** | 时间间隔或自主节奏 | 目标条件（直到 X 达成） |
| **结束条件** | 手动停止，或连续无事可做 | 独立评估器（Haiku）判定目标达成 |
| **核心优势** | 全流程保姆，自主推进，跨平台无盲区 | 单阶段冲刺，有明确终点时不浪费 token |
| **评估机制** | 无独立评估器（AI 自己判断） | 独立轻量模型判定（不看真实 API 响应） |
| **持久性** | 会话级 | 会话级 |

**一句话区分**：`/loop` 是"你干活，我过会儿回来看"；`/goal` 是"目标不达成不许停"。

---

## 4. 项目双平台架构与命令选择

### 2.1 双平台分工

本项目托管在 GitCode，并同时使用 GitCode 原生 CI 与 GitHub 镜像跨平台 CI：

```
GitCode (gitcode.com/gitcode-cli/cli)     GitHub (github.com/gitcode-cli/cli)
───────────────────────────────           ───────────────────────────────
gc issue / gc pr / gc release             gh run list / gh run watch / gh run view --log
gc actions run / job
    ↑                                          ↑
  代码托管、PR、Issue、标签                    镜像跨平台 CI
  原生 Linux CI
```

详见 [spec/delivery/ci-workflows.md](../spec/delivery/ci-workflows.md)。

### 2.2 这对 `/loop` 和 `/goal` 意味着什么

**`/loop`** 天然支持双平台——它拥有完整的 bash 工具权限，可以调用 `gc actions` 和 `gh`，
并直接读取两者的真实输出。不存在"评估器盲区"问题。

**`/goal`** 的评估器（Haiku）只阅读对话文本，不能执行命令。如果 AI 工作报告"CI 通过了"但实际失败了，评估器无法发现。因此 `/goal` 更适合**终点可在本地验证**的阶段。

### 2.3 选择流程图

```
你要做的事跨多个平台（GitCode + GitHub）吗？
    ├── 是 → 用 /loop
    └── 否 → 终点是可机器验证的单一条件吗？
                ├── 是 → 用 /goal
                └── 否 → 用 /loop
```

---

## 5. `/goal` 场景指南

`/goal` 适合**单一阶段、有明确可验证终点**的任务。在本项目流程中，以下阶段天然匹配：

### 3.1 场景一：Issue Triage → Verified

```bash
/goal until issue #<number> is triaged AND verified:
  - 判断类型（bug/feature/docs/refactor），补标签
  - 复现问题或确认需求有效
  - 在 issue comment 中留下结构化验证记录
  - 标签更新为 status/verified
```

**为什么适合**：验证结果（复现成功/失败）是二元结论，评估器可从对话文本判定。

### 3.2 场景二：开发 + 本地验证

```bash
/goal until:
  - go test ./... passes (所有包)
  - go build -o ./gc ./cmd/gc succeeds
  - ./gc version 输出正常
  - 如涉及命令行为变更，至少一条真实命令验证通过
  without modifying main branch
```

**为什么适合**：`go test` 和 `go build` 的输出是标准化的，评估器可以可靠地判断 PASS/FAIL。

### 3.3 场景三：安全审查

```bash
/goal until security review passes:
  - git diff origin/main 中无硬编码 token/password/secret
  - 无真实凭证写入文档或测试
  - 涉及认证/配置/权限路径的改动已对照 spec/foundations/security.md 检查
  - 审查结论已写入自检记录
```

**为什么适合**：安全检查（grep 凭证模式）可自动化，结果明确。

### 3.4 场景四：作者自检补齐

```bash
/goal until PR self-check template is complete:
  - 根因或实现理由已填写
  - 修改范围已填写
  - 测试结果已填写
  - 实际命令验证结果已填写
  - 安全审查结果已填写
  - 文档同步结果已填写
  - 风险点已填写
  - 未覆盖项已填写
  - CI 证据（run ID + Job 状态）已填写
  label status/self-checked applied
```

**为什么适合**：自检模板有 9 个固定字段，评估器可逐项检查完整性。

### 3.5 不适合 `/goal` 的场景

| 场景 | 原因 |
|------|------|
| 多角色评审 | 需要独立执行主体语义判断，评估器无法替代 |
| 风险分级 | 需要运行脚本 + 人工判断策略 |
| "让代码更好" | 无法量化，评估器无法判定 |
| 跨 GitCode + GitHub 的完整 PR 流程 | 评估器看不到真实 API 响应 |

---

## 6. `/loop` 场景指南

`/loop` 适合**持续关注、跨阶段、跨平台**的推进任务。

### 4.1 场景一：PR 保姆模式（最常用）

在开发分支上启动，让 AI 自主推进 PR 从创建到 ready-for-review：

```bash
/loop
```

AI 的自主行为循环：

1. **检查工作区状态**：有无未提交改动 → 提交
2. **检查本地门禁**：`go test ./...`、`go build` → 失败则修复
3. **检查远端同步**：分支是否已 push → push 到 GitCode 和 GitHub
4. **检查 CI 状态**：`gh run list --workflow=ci.yml --branch <branch>` → 失败则诊断修复
5. **检查 PR 状态**：PR 是否已创建、标签是否更新
6. **检查自检证据**：自检模板是否完整 → 补齐
7. **检查评审状态**：等待独立评审（不可逆操作暂停确认）
8. **无事可做**：静默等待

**关键约束**：
- 不会在 `main` 分支操作
- 不会跳过 `status/verified` 直接写代码
- 不会把作者自检当作独立评审
- 连续 3 轮无事可做会自动收敛

### 4.2 场景二：Issue 批量 Triage

```bash
/loop 30m 检查 GitCode 上 gitcode-cli/cli 的新 issue，对 status/triage 的 issue 执行分类：
  - 判断类型（bug/feature/docs/refactor）
  - 补充 scope 标签
  - bug 类型则补充复现信息请求
  - 更新为 status/triage 或 status/verified
```

### 4.3 场景三：CI 故障修复循环

PR 推送后在 CI 持续失败的场景：

```bash
/loop 5m 监控 gh run list --workflow=ci.yml --branch <branch>：
  - CI 全绿 → 记录 run ID，更新自检证据，停止
  - CI 失败 → 获取日志，诊断根因，修复，commit，push，回到监控
  - 如是环境/平台偶发问题 → 在自检中记录，继续推进
```

### 4.4 场景四：评审前准备冲刺

当开发完成但自检材料不齐时：

```bash
/loop 当前分支 <branch>，目标是将 PR 推进到 status/ready-for-review：
  1. 运行 go test ./... && go build ./...（本地门禁）
  2. 运行至少一条真实命令验证（infra-test/*）
  3. 执行安全审查
  4. 同步文档（COMMANDS.md 等）
  5. 运行风险分级脚本
  6. 收集 CI 证据（gh run list + run URL）
  7. 补齐作者自检模板 9 项
  8. 更新 PR 标签到 status/self-checked
  完成后停止，不要合并，等待独立评审。
```

### 4.5 场景五：评审意见响应

PR 收到 `status/changes-requested` 后：

```bash
/loop 处理当前 PR 的 review 意见：
  - 获取最新 review comments
  - 逐条分析并修复
  - 修复后运行测试验证
  - commit + push
  - 回复 review 线程
  - 全部解决后更新标签
```

### 4.6 `/loop` 的行为边界（本项目硬约束）

根据 `spec/workflows/development-workflow.md`，`/loop` 在以下情况**必须暂停并请求确认**：

| 操作 | 策略 |
|------|------|
| 合并 PR（`gc pr merge`） | 必须确认，`risk/high` 需人工最终确认 |
| 删除分支/数据 | 必须确认 |
| 修改 CI 配置 | 必须确认 |
| 关闭 issue（未合并时） | 禁止 |
| 把作者自检当独立评审 | 禁止 |
| 在 main 分支开发 | 禁止 |

---

## 7. 完整开发流程中的命令接力

以下展示从 Issue 到 Merge 的完整流程中，`/goal` 和 `/loop` 如何配合使用：

```
阶段                     推荐命令              预估耗时
─────────────────────────────────────────────────────────
Issue Triage          →  /goal (验证)           2-5 min
Issue Verified        →  /goal (开发+测试)      10-30 min
安全审查              →  /goal (安全扫描)        2-5 min
文档同步 + 风险分级   →  /goal (自检补齐)        5-10 min
推送 + CI 等待        →  /loop (CI 保姆)         5-20 min
多角色评审            →  手动（TeamCreate + Agent）10-20 min
评审意见修复          →  /loop (评审响应)        5-15 min
合并                  →  手动确认                 1 min
```

### 5.1 快速模式（信任度高的小改动）

```bash
# 一条命令覆盖开发到 ready-for-review
/loop 基于 issue #<number>，从当前状态推进到 status/ready-for-review，
严格遵循 spec/workflows/development-workflow.md 的状态机，不跳过任何门禁。
```

### 5.2 精细模式（高风险或复杂改动）

```bash
# Step 1
/goal until issue #<number> verified with reproduction evidence

# Step 2
/goal until go test ./... && go build ./... passes, real cmd verified

# Step 3
/goal until security review done AND self-check template complete

# Step 4: 推送 + CI
/loop 5m 监控 CI，失败则修复，全绿后记录 run ID 到自检

# Step 5: 手动多角色评审（TeamCreate + 8 Agent）

# Step 6: 响应评审
/loop 处理 review 意见，修复后 push，全部解决后停止
```

---

## 8. CI 相关操作的正确方式

### 6.1 `/loop` 中的 CI 监控逻辑

当 `/loop` 检测到已 push 并创建 GitCode PR 后，先核验原生 CI：

```bash
gc actions run list -R gitcode-cli/cli --pr <pr-number> --workflow "CI" --json
gc actions run view <run-id> -R gitcode-cli/cli --json
gc actions job list <run-id> -R gitcode-cli/cli --json

# 如果失败，下载对应 Job 日志归档
gc actions job log <run-id> <job-id> -R gitcode-cli/cli --output job-log.zip
```

存在 GitHub 镜像 PR 时，再核验跨平台 CI：

```bash
# 获取最新 CI run
gh run list --workflow=ci.yml --branch <branch> --limit 1 \
  --json databaseId,status,conclusion

# 如果还在运行，等待完成
gh run watch <run-id>

# 查看结论
gh run view <run-id> --json conclusion --jq '.conclusion'

# 如果失败，获取日志
gh run view <run-id> --log --job=<job-id> 2>&1 | head -200
```

### 6.2 `/goal` 中的 CI 验证

```bash
# ⚠️ /goal 不能直接验证 CI，但可以这样写终点：
/goal until:
  - GitCode Actions run ID 已记录到自检
  - GitCode 原生 CI status 为 COMPLETED（从 gc actions 输出确认）
  - 有 GitHub 镜像 PR 时，GitHub CI conclusion 为 success（从 gh run view 输出确认）
  - 所有 Job 状态已填入自检模板
```

关键提示：`/goal` 的评估器看不到 `gc actions` 或 `gh` 的真实输出，它只能看 AI 是否**声称**已检查。
因此 CI 监控优先用 `/loop`。

---

## 9. 多角色评审：为什么必须手动

本项目的多角色评审要求 **8 个独立 Agent** 分两轮执行（详见 [spec/workflows/review-workflow.md](../spec/workflows/review-workflow.md)）：

```
第一轮（必须）:  代码审查 + 安全审查 + 测试审查 + 文档审查
第二轮（按需）:  架构一致性 + API 契约 + 边界条件 + 用户体验
```

评审的本质是**独立执行主体的语义判断**，不能自动化。用 `/loop` 或 `/goal` 跳过评审直接合并是**严重违规**。

正确的做法：在 `/loop` 或 `/goal` 完成自检后，**手动**创建评审团队：

```bash
# 创建评审团队
TeamCreate

# 第一轮并行评审
Agent(subagent_type="general-purpose", description="代码审查")
Agent(subagent_type="general-purpose", description="安全审查")
Agent(subagent_type="general-purpose", description="测试审查")
Agent(subagent_type="general-purpose", description="文档审查")

# 收集结论，如有问题修复后第二轮
# 全部通过后 → approved → merge
```

---

## 10. 常见问题

### Q1: `/loop` 会不会在 main 分支上开发？

不会。项目规则（`spec/workflows/development-workflow.md`）禁止在 `main` 直接开发，`/loop` 遵循此约束。如果检测到当前在 `main` 分支，会先创建功能分支。

### Q2: `/goal` 会不会误判"已完成"而跳过门禁？

有可能。`/goal` 的评估器只能读对话文本，如果 AI 工作报告"已完成"但实际未完成，评估器无法发现。因此：
- 可本地验证的门禁（`go test`、`go build`）→ `/goal` 可靠
- 涉及远端 API 的门禁（CI 状态、PR 合并）→ 用 `/loop` 更可靠

### Q3: `/loop` 会不会自己合并 PR？

不会自动合并。合并是不可逆操作，`/loop` 在合并前会暂停并请求确认。`risk/high` 改动还需要人工最终确认。

### Q4: 如何停止正在运行的 `/loop` 或 `/goal`？

- 在对话中回复"停止"
- `/goal clear` 取消目标
- 连续多轮无事可做时，`/loop` 会自动收敛

### Q5: 可以在同一个会话中混合使用两个命令吗？

可以。推荐模式是用 `/goal` 处理每个有明确终点的阶段，用 `/loop` 处理需要持续关注的跨阶段任务。参见第 7 节的接力流程。

### Q6: `gc` 和 `gh` 混用会有什么问题？

不会有问题，它们是独立的 CLI 工具操作不同平台。但注意：
- `gc pr create` 在 GitCode 创建 PR，会自动触发 `.gitcode/workflows/ci.yml` 的原生 Linux CI
- `gc actions` 查询 GitCode 原生 CI；`gh` 查询 GitHub 镜像 CI
- GitHub Actions 在镜像 PR 提交/更新、以及镜像 `main` 收到同步推送时自动触发（`on: pull_request, push`）

---

## 11. 反模式（不要这样做）

| 反模式 | 为什么错误 | 正确做法 |
|--------|-----------|---------|
| `/goal until PR merged` | 评估器无法验证远端合并状态，且跳过了评审 | 分阶段接力，合并前手动确认 |
| `/loop` 中跳过自检直接请求评审 | 违反状态机，自检是评审的前置条件 | 让 `/loop` 补齐自检后再手动触发评审 |
| `/goal until all review passed` | 评估器不能替代独立执行主体判断 | 评审必须手动触发（TeamCreate + Agent） |
| 在 `main` 分支启动 `/loop` | 项目规范禁止在 main 直接开发 | 先创建功能分支 |
| 只查询一个 CI 平台 | 会遗漏 GitCode 原生 Linux 或 GitHub 跨平台结果 | 按 `spec/delivery/ci-workflows.md` 核验对应平台 |

---

## 12. 快速参考卡片

```
┌─────────────────────────────────────────────────────────┐
│                   /loop vs /goal 速查                    │
├─────────────────────────────────────────────────────────┤
│  用 /goal 当你：                                        │
│    □ 只有一个明确阶段要做                               │
│    □ 终点可以在本地机器验证（go test / go build）        │
│    □ 不涉及 GitCode + GitHub 跨平台判断                 │
│    □ 想要"不达目标不罢休"的确定感                       │
│                                                         │
│  用 /loop 当你：                                        │
│    □ 需要持续关注多个阶段                               │
│    □ 涉及 CI 监控（gh）和 GitCode 操作（gc）            │
│    □ 需要 PR 保姆式全流程推进                           │
│    □ 想要"放手让 AI 干，你回来验收"                     │
│                                                         │
│  不要用任何一个当你：                                   │
│    □ 需要多角色独立评审 → 手动 TeamCreate + Agent       │
│    □ 需要人工最终确认（risk/high）                      │
│    □ 没有明确 Issue → 先走 spec 流程                    │
└─────────────────────────────────────────────────────────┘
```

---

## 13. 相关文档

| 文档 | 说明 |
|------|------|
| [spec/workflows/development-workflow.md](../spec/workflows/development-workflow.md) | 完整开发流程状态机 |
| [spec/workflows/pr-workflow.md](../spec/workflows/pr-workflow.md) | PR 生命周期与自检要求 |
| [spec/workflows/review-workflow.md](../spec/workflows/review-workflow.md) | 多角色评审规范 |
| [spec/workflows/ai-local-development-workflow.md](../spec/workflows/ai-local-development-workflow.md) | AI 本地开发闭环编排 |
| [spec/delivery/ci-workflows.md](../spec/delivery/ci-workflows.md) | CI 工作流与 `gc actions` / `gh` 用法 |
| [spec/governance/docs-governance.md](../spec/governance/docs-governance.md) | 文档分层与治理规范 |
| [docs/COMMANDS.md](./COMMANDS.md) | 命令行为手册 |

---

**最后更新**: 2026-06-24
