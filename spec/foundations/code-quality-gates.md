# 代码质量门禁规范

本文件定义 gitcode-cli 的代码质量门禁。

目标不是重复编码规范，而是把“何时允许继续推进流程”写成可判定条件，避免人工或 AI 用模糊描述绕过交付要求。

## 职责

- 定义本地提交、PR 创建和合并前的质量门禁
- 定义什么证据缺失时不得宣称完成
- 统一人工与 AI 的交付标准

## 适用场景

- 任何准备提交的改动
- PR 描述与审查
- docs-only、代码变更、流程变更的交付检查

## 必须

- 在进入下一层门禁前通过上一层
- 按改动类型补测试、补文档、补审查记录
- 在无 CI 的情况下提供足够的本地验证证据
- 按状态机要求推进 issue 和 PR

## 禁止

- 把编码规范、测试指南或流程规范当作本文件替代品
- 跳过 blocker 检查直接合并
- 没有证据就声称“已完成”
- 把作者自检当作独立执行主体评审

## 同步要求

- 门禁变化时同步 `AGENTS.md`、`CLAUDE.md`，并检查独立 Skills 仓是否需要单独更新
- docs-only 规则变化时同步相关工作流说明

## 不负责什么

- 具体编码写法
- 具体测试案例设计
- 文档层级设计

## 1. 目标

代码质量门禁需要解决以下问题：

- 避免只有文档规范、没有执行标准
- 让多人和多 AI 客户端的交付标准一致
- 让本地检查、PR 审查和合并前确认形成闭环
- 防止 AI 通过“看起来像完成”的描述伪造流程闭环

## 2. 门禁分层

本仓库采用四级门禁：

1. 本地开发门禁
2. 远端 CI 验证（自动化证据补充层）
3. PR 门禁
4. 合并门禁

只有通过上一层门禁，才进入下一层。

远端 CI 验证由 AI 通过 `gc actions` 和 `gh` 核验，详细规范见 `spec/delivery/ci-workflows.md`。

## 3. 本地开发门禁

任何准备提交的改动，至少必须完成以下检查。

### 3.1 基础检查

```bash
go test ./...
go build -o ./gc ./cmd/gc
```

如果改动影响格式或存在格式风险，还应运行：

```bash
gofmt -w <files>
```

### 3.2 功能检查

如果改动影响具体命令行为，必须完成：

- 相关包级测试
- 至少一个真实命令验证

真实命令验证只能使用 `infra-test/*` 仓库。

### 3.3 安全检查

提交前必须确认：

- 无硬编码 token、password、secret
- 无敏感文件被误追踪
- 测试代码和文档不包含真实凭证

### 3.4 文档同步检查

行为变化后必须检查：

- `README.md`
- `docs/COMMANDS.md`
- `spec/*`
- `AGENTS.md`
- `CLAUDE.md`
- 独立 Skills 仓中的相关 Skill（如受影响，单独提交）

如果改动影响命令输出、确认行为、错误语义或 schema，还必须检查：

- `spec/foundations/agent-friendly-cli.md`
- `docs/AI-GUIDE.md`
- `docs/REGRESSION.md`

### 3.5 工作区卫生检查

提交前必须确认：

- 构建产物未误提交
- 临时文件未误提交
- `.gitignore` 已覆盖新增本地产物

### 3.6 状态与证据检查

进入提交流程前，必须确认：

- 关联 issue 已进入 `status/verified`
- 开发阶段已记录为 `status/in-progress`
- 已有验证记录
- 已有本地测试和真实命令验证证据

如果缺少上述任一项，不得宣称”开发完成”。

### 3.7 远端 CI 验证

本地门禁通过后，分支推送到 GitCode 并创建 PR 时，GitCode Actions 原生 CI 自动触发
（`on: pull_request`）。有对应 GitHub 镜像 PR 时，GitHub Actions 跨平台 CI 同时作为补充证据。

CI 通过的判定标准：

- GitCode 原生 `lint` / `security` / `test` / `build` / `package` Linux Job 通过
- 有 GitHub 镜像 PR 时，GitHub `test` / `build` 在 ubuntu / macOS / Windows 通过
- 有 GitHub 镜像 PR 时，GitHub `lint` / `secret-scan` / `docker` Linux Job 通过

CI 失败时不得进入 PR 门禁，除非失败原因已明确判定为环境/平台偶发问题（需在自检中记录）。

CI 触发与监控的完整流程见 `spec/delivery/ci-workflows.md`。

docs-only 改动可跳过 CI，但必须在自检中说明。

## 4. PR 门禁

进入 PR 前，必须满足以下要求。

### 4.1 关联与范围

- 改动有对应 issue
- 分支命名符合约定
- 变更范围清晰，不把无关内容混入同一个 PR

### 4.2 作者自检证据

PR 中必须有结构化作者自检，至少包含：

- 作者主体标识
- 根因或实现理由
- 改动内容
- 单元测试结果
- 构建结果
- CI 验证结果（run ID / URL + 各 Job 结论；docs-only 可注明跳过）
- 实际命令验证结果
- 安全审查结果
- 文档同步结果
- 风险点
- 未覆盖项

### 4.3 状态要求

PR 进入 `status/ready-for-review` 前必须满足：

- PR 已从 `status/draft` 进入 `status/self-checked`
- issue 已进入 `status/ready-for-review`
- 作者自检记录完整

### 4.4 文档完整性

如果改动影响命令、流程、发布、审查或 AI 协作规则，PR 中必须体现同步文档更新。

## 5. 合并门禁

PR 在合并前，必须满足：

- 无 blocker 级问题
- 变更范围与 issue 一致
- 文档已同步
- 必要测试已完成
- 平台限制已明确写入文档，而不是隐藏或回避
- 已有独立执行主体评审结论
- 作者主体标识与评审主体标识可区分
- PR 状态已进入 `status/approved`

在当前无统一远端流程门禁的前提下，测试和审查证据以本地验证、远端事实核验和独立执行主体评审作为主门禁。

### 5.1 风险分级

进入评审前，必须对改动执行最小风险分级：

```bash
python3 scripts/classify-change-risk.py --base origin/main
```

风险等级定义：

- `low`：文档、规范、模板、共享 skill、入口文档、文档辅助脚本
- `medium`：一般命令实现、测试补齐、非认证类行为修正
- `high`：认证、token、权限、删除或覆盖、确认逻辑、JSON/schema/退出码契约、发布安装链路

### 5.2 风险对应评审策略

- `low`：第二个 AI 代理可作为独立执行主体评审并自动推进
- `medium`：第二个 AI 代理可作为独立执行主体评审；仅在 blocker 或不确定时升级人工
- `high`：独立 AI 评审后仍需人工最终确认

## 6. 自动检查与人工审查边界

### 6.1 适合自动检查

- `go test ./...`
- `go build -o ./gc ./cmd/gc`
- 基础格式检查
- 常见敏感信息扫描
- 本地产物和临时文件检查
- 标签和状态是否齐全
- 自检模板字段是否齐全
- `scripts/validate-ai-record.py` 的模板结构检查和必填字段检查
- `scripts/classify-change-risk.py` 的风险分级
- `scripts/verify-remote-facts.py` 的远端事实核验

### 6.2 必须由独立执行主体审查

- 命令行为是否符合产品语义
- 错误提示是否清晰
- 非交互环境是否会阻塞等待输入
- JSON / schema 输出是否稳定且可消费
- API 兼容性是否真实有效
- 是否引入重复实现
- 文档是否准确描述当前限制
- AI 生成代码是否只是“看起来合理”

其中：

- `low` 和大多数 `medium` 风险改动，可由第二个 AI 代理完成独立执行主体审查
- `high` 风险改动或评审主体明确不确定时，必须升级人工最终确认

## 7. Blocker 判定

以下问题默认视为 blocker：

- 命令行为错误
- 参数解析回退或兼容性破坏
- 真实命令验证失败且无合理解释
- 敏感信息泄漏风险
- 缺少必要安全审查结论
- 文档与真实行为明显不一致
- 缺少 issue 验证记录
- 缺少作者自检记录
- 缺少作者主体标识或评审主体标识
- 把作者自检误当作独立执行主体评审
- 未处理的高优 review finding
- 涉及认证、凭证、权限或危险写路径的改动未进行安全审查
- 高风险改动缺少人工最终确认

以下问题通常不是 blocker，但应尽快修正：

- 轻微文案问题
- 非关键示例缺失
- 局部注释或表达不够清晰

## 8. docs-only 变更规则

如果 PR 是 docs-only 变更，可以不运行测试，但必须明确写出：

- 本次改动不涉及代码路径
- 未运行测试的原因
- 文档内容基于哪些现有实现或已验证行为
- 文档中未写入真实凭证、敏感信息或误导性的危险操作示例

即使是 docs-only，也仍应满足 issue / PR 状态与自检记录要求。

## 9. AI 协作补充规则

多人使用 Codex 和 Claude 时，质量门禁要求完全一致。

这意味着：

- `AGENTS.md` 和 `CLAUDE.md` 只能解释入口，不得降低门禁
- 独立 `gitcode-cli/skills` 仓中的 Skill 不能定义低于本文件的交付标准
- 用户本地 `.claude/skills/` 和 `.codex/skills/` 只是安装副本，不是项目规则源
- AI 生成的结果必须像人工提交一样接受同一套门禁
- AI 不能把缺少证据的“完成声明”视为通过门禁
- 作者与评审必须是不同执行主体

## 10. 当前执行基线

当前仓库的执行基线为：

1. 本地开发门禁必须执行
2. 远端 CI 验证由 AI 通过 `gc actions` / `gh` 核验，作为自动化证据补充（非代码改动可跳过）
3. PR 门禁必须留下结构化自检证据（含 CI 结果）
4. `low` 与大多数 `medium` 风险改动可走双 AI 闭环，但必须依赖独立执行主体评审、本地验证和 CI 结果
5. `high` 风险改动必须追加人工最终确认
6. CI 自动化已同时基于 GitCode Actions 与 GitHub Actions 落地，PR 提交时自动触发，详见 `spec/delivery/ci-workflows.md`

## 下一步去看哪里

- 如果你不确定哪些文档需要同步，继续看 [文档治理规范](../governance/docs-governance.md)
- 如果你还没完成流程动作，继续看 [开发工作流程](../workflows/development-workflow.md)
- 如果你要补齐 issue 和 PR 的状态机细节，继续看 [Issue 流程](../workflows/issue-workflow.md) 和 [PR 流程](../workflows/pr-workflow.md)

---

**最后更新**: 2026-06-02
