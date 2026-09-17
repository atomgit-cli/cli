# SIG 治理规范

本文件定义 gitcode-cli 仓库中 Special Interest Group（SIG）的治理模型、生命周期、决策机制和与其他规范的关系。

## 职责

- 定义 SIG 的角色、生命周期和决策机制
- 明确 SIG 元数据的存放位置与格式
- 规范 SIG 与 issue / PR / label / discussion 的关联方式
- 约束 SIG 规则与其他 `spec/` 文档的边界

## 适用场景

- 提议新增、合并或归档 SIG
- 在 issue / PR 上使用 `sig/*` label 标识归属
- 决定某个议题应该由哪个 SIG 处理
- 评估 SIG 是否活跃、是否需要治理介入

## 必须

- SIG 元数据真相源为 `spec/governance/sigs/<sig-name>.yaml`，其他位置（docs 介绍、CLI 输出）都必须从该 YAML 派生
- SIG 治理变更（新增、合并、归档、调整范围）必须走 PR 流程，并标注 `sig/governance` label
- 在 issue / PR 上使用 `sig/*` label 时，label 名必须与 YAML 中的 `name` 字段完全一致
- SIG 内**日常议题**决策采用 lazy consensus：议题提出后**最短公示 ≥ 72 小时**内无人类明确反对即视为通过（Agent 表态不计入，见 §7）；SIG 内重大方向、跨 SIG 决策的决策方式见 §7，不适用本条
- SIG 例会、纪要、路线图等运行时产物使用 SIG 讨论通道承载（Phase 1 为带 `sig/<name>` label 的 issue；discussion 分类开通后迁移，见 §6.2），不进入 `spec/`

## 禁止

- 在 SIG YAML 之外另建"权威"SIG 列表（如 Wiki、独立网站）
- 通过 `gc` CLI 直接创建、编辑、归档 SIG（治理变更必须留痕并走 review）
- 让 SIG 规则覆盖或绕过 `spec/foundations/`、`spec/workflows/`、`spec/delivery/` 中的既有规则
- 让单个 SIG 独占某个目录的合并权限，阻塞跨 SIG 协作
- 把个人 contributor 直接绑定为 SIG 唯一决策人（至少保留 1 名 lead + 1 名 maintainer；单人维护豁免期例外见 §4.2）

## 同步要求

- 新增 SIG 时同步：创建 YAML → 创建对应 label → 更新 `docs/SIGS.md` → 在 SIG 讨论通道发出公告（Phase 1 为带 `sig/<name>` label 的 issue；discussion 分类开通后改为创建对应分类，见 §6.2）
- 归档 SIG 时同步：更新 YAML `status: archived` → 更新 `docs/SIGS.md`（标注 archived）→ 在 SIG 讨论通道发出归档公告；label 本身保留不改名（历史 issue/PR 的 `sig/*` 标注仍可查询；GitCode label API 无 description 字段，归档状态以 YAML 为准）
- SIG 范围（`scope` / `scope_labels` 字段）变化时同步检查 `docs/SIGS.md` 与相关模块的 README

## 不负责什么

- 命令实现细节（由 `spec/foundations/command-template.md` 负责）
- 测试与质量门禁（由 `spec/foundations/testing-guide.md`、`code-quality-gates.md` 负责）
- 发布流程（由 `spec/delivery/release-process.md` 负责）
- AI 协作边界（由 `spec/governance/ai-collaboration.md` 负责）

## 1. 目标

引入 SIG 的目标是：

- 为跨人协调密集的领域提供明确的归属与决策路径
- 降低单个 maintainer 在跨领域议题上的认知负担
- 让 issue / PR 的归属通过 label 显式化，便于统计与查询
- 为长期 roadmap（如 API 演进、CLI 体验一致性）提供稳定的责任主体

不引入 SIG 的领域：规则稳定、变更频率低、已有完善 `spec/` 兜底的领域（如 release、testing），过度治理反而增加开销。

## 2. 当前 SIG 列表

| 名称               | 状态         | 范围                                                              | 说明                                                      |
| ------------------ | ------------ | ----------------------------------------------------------------- | --------------------------------------------------------- |
| `sig/api`        | incubating | `api/`、`api-doc/`                                            | GitCode API 客户端、endpoint 对齐、api-doc submodule 同步 |
| `sig/cli-ux`     | incubating | `pkg/cmd/`、`pkg/cmdutil/`、`pkg/output/`、`pkg/iostreams/` | 命令交互、`--json` 输出、错误提示、退出码、帮助文案     |
| `sig/governance` | incubating | `spec/`、`docs/`、`AGENTS.md`、`CLAUDE.md`、`CONTRIBUTING.md` | 规范演进、文档治理、AI 协作边界、SIG 治理自身             |

> 下表范围为主要摘要，权威真相源以各 YAML 的 `scope`（目录范围）与 `scope_labels`（活跃度映射，见 §6.1）字段为准。三个试点 SIG 自 `incubating` 起步，按 §3 退出条件达标后转 `active`。

完整元数据见 `spec/governance/sigs/<sig-name>.yaml`。

## 3. SIG 生命周期

```
propose → incubating → active ⇄ degraded → archived
              └──────────────────────────→ archived（试点失败撤销）
```

> `propose` 是 issue 阶段的讨论状态，不进入 YAML 的 `status` 枚举——YAML 创建时即为 `incubating`（见 §5.2）。

| 状态           | 含义                                                                                          | 退出条件                                                                                          |
| -------------- | --------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `propose`    | issue 阶段，讨论必要性与范围                                                                  | maintainer 批准进入 incubating                                                                    |
| `incubating` | 试运行，YAML 与 `sig/<name>` label 已建并启用，处于归属数据观察期                          | ≥ 4 周内有 ≥ 5 个 issue / PR 归属该 SIG（`sig/<name>` label 或 `scope_labels` 映射，见下方统计口径），且无重大归属争议；试运行失败可直接转 `archived`（撤销） |
| `active`     | 正式运行                                                                                      | 持续活跃                                                                                          |
| `degraded`   | 连续 ≥ 8 周无任何关联活动（`sig/<name>` label 使用，或 `scope_labels` 映射的 issue/PR），或 lead 缺位 ≥ 2 周（对齐 §4.4），或豁免整改逾期（连续 2 个评估周期未整改，见 §4.2） | 补充 lead 或恢复活跃；连续 ≥ 8 周处于 degraded 且无恢复计划时，由 maintainer 决定转 archived |
| `archived`   | 已归档                                                                                        | 不可恢复，如需重建须重新 propose                                                                  |

> **活跃度统计口径**：为避免"为了显得活跃而打 `sig/*` label"的循环激励（见 §6.1 打标策略），活跃度评估采用**双口径**——`sig/<name>` label 使用数 **加上** 该 SIG YAML `scope_labels` 字段映射的 `scope/*` label 对应 issue/PR 数。`scope_labels` 未映射的 `scope/*` label **不计入**任何 SIG 的活跃度。两者任一非零即视为有关联活动。

> **评估主体与节奏**：生命周期状态由 `sig/governance` lead 组织的**月度生命周期评估**统一核查（试点期内按 §9 的试点节奏），评估基于 `gc issue list --label` 的 issue 数据；PR 侧按 label 过滤的 CLI 能力暂缺（`gc pr list` 无 label flag），经 GitCode 平台 Web 端 label 过滤或 `gc api` 查询获取（`gc sig prs` 只读命令为 Phase 3 候选）；评估结论（含状态变更建议）记录在带 `sig/governance` label 的 issue 中，状态变更本身仍须走 PR 修改 YAML。

## 4. SIG 角色

| 角色            | 职责                                                                 | 承担者                | 人数    |
| --------------- | -------------------------------------------------------------------- | --------------------- | ------- |
| `lead`        | SIG 决策、跨 SIG 协调、代表 SIG 参与 maintainer 会议                | 人                    | 1-2 人  |
| `maintainer`  | review SIG 范围内的 PR、参与决策                                     | 人                    | ≥ 1 人 |
| `agent`       | 执行 SIG 范围内的开发任务：提 PR、写文档、做分析、回答 issue         | AI 助手（Claude、Trae 等） | 不限    |
| `contributor` | 提交 issue / PR、参与讨论                                            | 人或 Agent            | 不限    |

> 本项目把 Agent 视为一等开发者。详见下方"§4.3 Agent 在 SIG 中的位置"。

### 4.1 角色规则

- `lead` 必须是仓库 maintainer
- `lead` 与 `maintainer` 不能全是同一人（避免单点；例外见 §4.2"豁免条款"）
- `agent` 不能担任 `lead` 或 `maintainer`（决策责任只能由人承担）
- 角色变更必须更新对应 YAML 并走 PR

### 4.2 豁免条款（单人维护期）

> 本条款是 §4.1 中"`lead` 与 `maintainer` 不能全是同一人"规则的**例外**。

**触发条件**：当仓库**人类 maintainer** 总人数 < 2 人时，允许同一 SIG 的 `leads` 与 `maintainers` 列表完全重叠（即同一人同时担任 lead 与唯一 maintainer）。该状态属于过渡期安排。**Agent 不计入 maintainer 总数**——即使 SIG 元数据中登记了多个 Agent，豁免状态不受影响。

> **maintainer 总人数的判定来源**：Phase 1 以 GitCode 仓库远端成员列表（仓库设置页可见的 maintainer 角色）为准；Phase 2 maintainer 注册表落地后以其为准。

**豁免有效期**：自触发条件成立起，至整改完成或触发条件消失（仓库人类 maintainer 总人数 ≥ 2）后的整改期限届满为止。**整改窗口内** lead 与 maintainer 的重叠**不视为违反** §4.1。

**标注义务**（以下位置都必须显式标注，构成封闭集合；不在本列表内的文档可自行决定是否补充）：

1. **YAML 文件顶部注释**：必须以 `# note: 单人维护豁免期内` 作为前缀，可在前缀后追加补充说明。标准格式：
   ```
   # note: 单人维护豁免期内（<原因简述>）
   ```
2. **`docs/SIGS.md` 当前 SIG 列表**：在 SIG 列表表格的 SIG 名后加 `⚠️` 标记，并在表格下方以引用块（`>`）形式说明豁免状态与整改义务。

**整改义务**：

- **触发时点**：仓库新增 maintainer 的 PR merge 之日
- **责任人**：`sig/governance` lead 负责跟踪
- **整改期限**：自触发时点起 **30 天内**
- **整改终态**：**全部**处于豁免状态的 SIG 均脱离豁免（新 maintainer 补充进 SIG 的 `maintainers` 列表，或调整 lead 人选）；仅部分 SIG 完成整改时，其余 SIG 的豁免注释保留至各自完成整改
- **整改完成标志**：以对应 SIG 的 YAML 中移除豁免注释、`docs/SIGS.md` 中移除对应 ⚠️ 标记与豁免说明块为准
- **逾期后果**：`sig/governance` lead 须在月度生命周期评估（§3）中将逾期 SIG 列入风险清单并升级到 maintainer 会议；**连续 2 个评估周期**未整改的 SIG，触发 §3 的 `degraded` 评估

**Review 标准约束**：豁免状态本身不构成降低 PR review 标准的理由；SIG 内 PR 仍需遵循 `spec/workflows/review-workflow.md` 的独立评审原则。

### 4.3 Agent 在 SIG 中的位置

本项目把 Agent 视为**一等开发者**，与 `AGENTS.md`、`CLAUDE.md`、`spec/governance/ai-collaboration.md` 的定位保持一致。本节定义 Agent 在 SIG 治理中的正式位置。

**与其他规范的关系**：本节定义 Agent 在 **SIG 治理语境下**的特定边界（角色、commit 标识、决策权限、审计等）。通用 AI 协作边界（AI 客户端入口、Skills 仓管理、文档分层）由 [ai-collaboration.md](./ai-collaboration.md) 定义。两者互补，不冲突。

**核心原则**：Agent 是执行者，人是责任承担者。Agent 可以独立承担 SIG 范围内的开发任务，但所有"涉及责任"的决策（merge、approve、角色任命、SIG 生命周期变更）永远由人做出。

**地位**：

- Agent 是 SIG 的一等 `agent` 角色成员，其贡献（PR、文档、分析、issue 回答）与人类 contributor 同等计入 SIG 活跃度
- Agent 不被视为"外部工具"，而是 SIG 治理框架内的正式参与者
- Agent 的活跃度纳入 §3 生命周期评估（`incubating → active` 的归属计数包含 Agent 产出，与人类 contributor 同等对待）

**责任**：

- 每个 Agent 在 SIG 元数据的 `agents` 字段中必须显式登记，并绑定一个**人类 operator**（必须是仓库 maintainer）
- Agent 的所有产出由其 operator 承担**完全责任**，包括但不限于代码正确性、合规性、安全性
- operator 变更必须走 PR 更新对应 SIG 的 YAML

**边界**（Agent **不得**做的事）：

- 不能担任 `lead` 或 `maintainer` 角色
- 不能做出 merge / approve / request-changes 等评审决策
- 不能修改 `spec/` 目录的内容而不经人 review（Agent 可以提交修改 `spec/` 的 PR，但 merge 决策由人做出）
- 不能任命或移除其他 SIG 角色（包括其他 Agent）
- 不能单独发起 SIG 生命周期变更（propose / activate / archive）——必须由其 operator 或其他人 maintainer 发起

**标识**：

- Agent 产出的 commit 必须在 commit message 中包含 trailer：`Signed-off-by-Agent: <agent-id>`（`<agent-id>` 与 YAML 中登记的 `id` 一致）
- Agent 提交的 PR / issue comment 应在描述中说明 `Agent: <agent-id>`，便于追溯
- 仓库 CI 可在未来引入 commit trailer 校验脚本（Phase 1 暂不强制）

**`Signed-off-by-Agent` 与 `Co-Authored-By` 的区分**：

项目现有规范（[spec/workflows/pr-workflow.md](../workflows/pr-workflow.md) §4 示例）使用 `Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>` 标识 AI 协作。两者适用于**不同场景**，可共存于同一 commit：

| Trailer | 适用场景 | 责任归属 |
|---------|---------|---------|
| `Co-Authored-By: <AI> <email>` | **人机协作**：人写主体，AI 补全/审校/改进；或 AI 起草但人深度参与修改 | 人与 AI 共同承担 |
| `Signed-off-by-Agent: <agent-id>` | **Agent 独立完成**：Agent 写主体，人只做 review/merge；或 Agent 完全自主完成 | Agent 的 operator 完全承担 |

**使用规则**：

- 人机协作共同完成一个 commit → 只用 `Co-Authored-By`
- Agent 独立完成一个 commit → 只用 `Signed-off-by-Agent`
- Agent 起草 + 人深度修改 → 只用 `Co-Authored-By`（视为协作）
- 两者可同时存在于同一 PR 的不同 commit 上（如 AI 独立写一个 commit，人改写另一个 commit）

**与既有审计 trailer 的关系**：仓库既有 commit 惯例使用 `Created-by` / `Commit-by` / `Merged-by` 等审计 trailer（git 历史事实惯例，暂未在 spec 成文）。本节定义的 `Signed-off-by-Agent` / `Co-Authored-By` 与它们**正交**，按既有惯例叠加使用，不互斥、不替代。

**审计**：

- SIG lead 应定期（建议每月）review 本 SIG 范围内 Agent 的产出质量与边界遵守情况
- 发现 Agent 越界（如未经 review 修改 spec、未带 trailer 提交）时，由 operator 负责纠正；情节严重的可由仓库 maintainer 暂停该 Agent 在 SIG 内的活动

### 4.4 加入 SIG 与角色任命

本节定义各角色的进入与退出路径。所有角色变更均须走 PR 更新对应 YAML（见 §4.1）。

**contributor → SIG maintainer**：

1. 先在该 SIG 范围内提交 ≥ 2 个被合并的 PR（登记 Agent 的产出可计入其 operator 的资历，见 §4.3），或持续参与该 SIG 讨论通道
2. 在对应 SIG 的讨论通道下自荐（Phase 1 为带 `sig/<name>` label 的 issue；discussion 分类开通后为对应分类，见 §6.2）
3. 由该 SIG lead 决定接纳为 maintainer，并通过 PR 更新 YAML 的 `maintainers` 字段

**SIG lead 任命**：

- 由仓库 maintainer 团队决定，并通过 PR 更新 YAML 的 `leads` 字段
- 候选人应已担任该 SIG maintainer，或有同等的跨 SIG 协调经验

**Agent 登记与移除**：

- 由其拟任 operator 提出，走 PR 更新对应 YAML 的 `agents` 字段
- Agent 移除同样走 PR；operator 离职或不再担任仓库 maintainer 时，其名下 Agent 须一并移除或更换 operator

**SIG lead 缺位处理**：

1. 先在该 SIG 讨论通道下提醒（Phase 1 为带 `sig/<name>` label 的 issue）
2. 连续 ≥ 2 周无响应时，在 `sig/governance` 讨论通道提出（Phase 1 为带 `sig/governance` label 的 issue），由仓库 maintainer 介入
3. 介入结果可能是补充 lead、指定代理 lead，或触发 §3 的 `degraded` 评估

## 5. SIG 元数据

### 5.1 文件位置

```
spec/governance/sigs/
├── api.yaml
├── cli-ux.yaml
└── governance.yaml
```

### 5.2 字段定义

字段定义以本节为准；`spec/governance/sigs/_schema.yaml`（JSON Schema 校验）待 Phase 2 补充。

最小必需字段：

- `name`：kebab-case，与 label 一致，不带 `sig/` 前缀
- `display_name`：人类可读名称
- `leads`：GitCode 用户名列表（带 `@`）
- `maintainers`：GitCode 用户名列表（带 `@`）
- `scope`：目录或文件路径列表
- `labels`：与该 SIG 关联的 label 列表，至少包含 `sig/<name>`
- `status`：`incubating | active | degraded | archived`
- `created_at`：ISO 日期（`YYYY-MM-DD`）

可选字段：

- `scope_labels`：该 SIG 活跃度统计映射的 `scope/*` label 列表（见 §3 统计口径与 §6.1）。**建议填写**；未填写时该 SIG 的活跃度只统计 `sig/<name>` label 口径。映射的 label 必须真实存在于仓库 label 列表，变更走 PR
- `charter`：SIG 使命与边界描述（多行字符串）
- `meeting`：例会信息（cadence、notes）
- `discussion_category`：对应 discussion 分类名（**Phase 1 未开通**：平台 repo 级 discussion 分类创建能力未验证，本字段为规划值，见 §6.2）
- `agents`：登记参与本 SIG 的 Agent 列表（详见 §4.3），每项包含以下子字段：
  - `id`：Agent 标识（kebab-case，如 `claude-code`、`trae-ai`），全局唯一，会作为 commit trailer 中的 `<agent-id>`
  - `operator`：该 Agent 的人类责任人，必须是仓库 maintainer（带 `@`）
  - `scopes`：该 Agent 被授权的目录/文件路径列表（应为 `scope` 字段的子集；为空列表或不设置表示授权整个 SIG 范围）
  - `since`：登记日期（ISO 格式，`YYYY-MM-DD`）

### 5.3 校验

- **Phase 1**：PR review 时由 reviewer 人工核对 §5.2 的字段要求；可本地运行 `scripts/validate-sig-yamls.sh` 辅助核对（该脚本为可选辅助工具，其校验项以"机器可判的必须项"为限，脚本无法覆盖的规则由人工兜底，详见脚本头部注释）
- **Phase 2**：将脚本接入 CI（校验失败即 PR 失败）。接入涉及 `spec/delivery/ci-workflows.md` 变更，须单独走 PR，不在本规范 Phase 1 交付范围内

## 6. SIG 与现有机制的关联

### 6.1 与 label 的关系

- 每个 active / incubating SIG 必须有一个 `sig/<name>` label
- label 与对应 YAML 的关联通过**命名约定**（`sig/<name>` ↔ `sigs/<name>.yaml`）与 `docs/SIGS.md` 导航发现；GitCode label API 仅支持 name/color（创建与更新均无 description 字段），不在 label 上承载链接
- 一个 issue / PR 可以打多个 `sig/*` label（跨领域），但应有 1 个主 SIG

**`sig/*` 与 `scope/*` 的关系**：

`sig/*` 与 `scope/*` 是**正交关系**，分别表达不同维度：

- **`scope/*`** 标识**技术模块归属**（"这个 issue 涉及哪个模块"），用于 issue triage 默认打标，遵循 [spec/workflows/status-label-checklist.md](../workflows/status-label-checklist.md) 的四维标签要求
- **`sig/*`** 标识**治理归属**（"这个 issue 需要哪个 SIG 介入"），仅在需要 SIG 治理介入时打标

使用规则：

- triage 时**默认只打 `scope/*`**；只有当 issue 需要 SIG 治理介入时才**额外**打 `sig/*`
- 需要打 `sig/*` 的典型场景：
  - 跨 SIG 议题（需要多个 SIG lead 协调）
  - SIG 治理变更（如调整 SIG 范围、lead 变更）
  - SIG 活跃度统计与生命周期评估
  - SIG 路线图讨论
- 两者可共存于同一 issue（如 `scope/api` + `sig/api`）；当 issue 仅涉及技术模块而无需 SIG 治理介入时，只打 `scope/*` 即可
- 不强制每个 issue 都打 `sig/*`——大部分日常 issue 只需 `scope/*`

**`scope_labels` 映射的维护**：

- 每个可选 `scope_labels` 字段显式声明"哪些 `scope/*` label 的活跃度计入本 SIG"（如 `sig/api` 映射 `scope/api`、`scope/http`）；`sig/<name>` 与 `scope/<name>` 同名不是隐式映射，必须在 `scope_labels` 中显式列出
- 一个 `scope/*` label 原则上映射到至多 1 个 SIG（避免双计）；确需跨 SIG 统计时须在两个 SIG 的 YAML 中同时登记并在 charter 说明
- 未被任何 SIG 的 `scope_labels` 映射的 `scope/*` label 不计入任何 SIG 的活跃度（§3 统计口径）
- 映射变更走 PR，评审时须核对新增映射的 label 真实存在

### 6.2 与 discussion 的关系

**Phase 1 现状**：GitCode 平台 repo 级 discussion 分类创建能力**未验证**（`gc` 当前只提供 discussion 的部分读取与删除操作：org 级 `gc discussions list/view/delete`、repo 级只读 `gc discussions project list/view`，无 create、无分类管理；本仓库 discussion 尚无内容）。因此 Phase 1 试点期间：

- SIG 的讨论通道**降级为 issue 承载**：带 `sig/<name>` label 的 issue 即该 SIG 的讨论与决策记录载体
- YAML 的 `discussion_category` 字段保留为规划值，不生效

**启用条件与迁移**：当平台 repo 级 discussion 分类能力经 `sig/governance` 验证可用后，由 `sig/governance` lead 走 PR 修订本节，为每个 active SIG 创建对应分类（如 `SIG: API`），并将存量 issue 讨论的链接迁移登记到分类公告中；在此之前不得在规范或文档中将 discussion 描述为**已可用**的承载。

- SIG 例会纪要、路线图讨论、决策记录放 SIG 讨论通道（Phase 1 为 issue）
- 跨 SIG 议题放带 `sig/governance` label 的 issue（discussion 开通后为 `General` 分类），由 `sig/governance` 协调

### 6.3 与 issue / PR workflow 的关系

- 现状不变（以下为**核心主干**，完整状态机以 [spec/workflows/issue-workflow.md](../workflows/issue-workflow.md) 与 [spec/workflows/pr-workflow.md](../workflows/pr-workflow.md) 为准，label 语义详见 [status-label-checklist.md](../workflows/status-label-checklist.md)）：
  - **Issue 主干**：`status/triage` → `status/verified` → `status/in-progress` → `status/ready-for-review` → `status/merged`
  - **PR 主干**：`status/draft` → `status/self-checked` → `status/ready-for-review` → `status/approved` → `status/merged`
- SIG 介入点：triage 阶段由对应 SIG 决定是否接收，以及由谁跟进
- SIG 不替代 `spec/workflows/issue-workflow.md` 与 `spec/workflows/pr-workflow.md`
- **PR 状态更新时的 label 保留**：`gc pr edit --labels` 是**替换语义**（见 [status-label-checklist.md](../workflows/status-label-checklist.md) "重要说明"）。更新 PR 状态标签时，`--labels` 参数必须同时携带已打的 `sig/*` 与 `scope/*` label，否则它们会被静默移除

### 6.4 与文档治理的关系

- SIG 治理变更必须同步 `docs/SIGS.md`
- SIG 治理文档自身的演进遵循 `spec/governance/docs-governance.md`

## 7. 决策机制

| 范围                                  | 决策方式                                                   | 记录位置               |
| ------------------------------------- | ---------------------------------------------------------- | ---------------------- |
| SIG 内日常议题                        | lazy consensus（公示 ≥ 72h 无人类反对即通过，见上方"必须"） | issue / PR comment     |
| SIG 内重大方向（如 API 客户端大版本） | SIG lead 拍板 + 异步公示 ≥ 3 天                           | SIG 讨论通道（§6.2）   |
| 跨 SIG 议题                           | 涉及的 SIG lead 协商，无法达成一致时升级到 maintainer 会议 | `sig/governance` 讨论通道 |
| 跨 SIG / 全局性大设计（新抽象、非平凡 tradeoff） | RFC 流程（[spec/workflows/rfc-workflow.md](../workflows/rfc-workflow.md)），归属 SIG lead 参与 + PR review merge | RFC PR + `docs/rfcs/RFC-NNNN-*.md` |
| SIG 新增 / 合并 / 归档                | maintainer PR review 通过                                  | PR + YAML 变更         |

**Agent 与 consensus**：Agent 不参与 lazy consensus 的"无反对即通过"判定——只有人类的明确赞同或沉默计入 consensus。Agent 的"同意"或"反对"通过其 operator 的表态生效；Agent 自身在 issue / PR 中的表态视为参考意见，不计入 consensus 判定基数。

**Agent 与 RFC**：登记 Agent 可以作为作者提交 RFC（`docs/rfcs/`），但 RFC 的接受与否由人决定（归属 SIG lead + PR review merge），与 §4.3 边界一致。

## 8. 与既有规范的边界

- 本规范不修改 `spec/foundations/coding-standards.md`、`testing-guide.md`、`security.md` 的任何要求
- 本规范不修改 `spec/workflows/` 中的状态机
- 本规范不修改 `spec/delivery/` 中的构建、发布、CI 规则
- 如 SIG 治理与上述规范冲突，以上述规范为准；确需调整时通过 PR 修订对应规范，而非通过 SIG 决议绕过

## 9. 试运行计划（Phase 1 试点）

- **试点范围**：`sig/api`、`sig/cli-ux`、`sig/governance` 三个 SIG
- **试点周期**：自本规范 merge 起 **4 周**（与 §3 incubating 最小观察窗对齐；第 2 周做一次中期检查点）
- **观察指标**：
  - 每个 SIG 在试运行期内的归属 issue/PR 数量（`sig/*` label + `scope_labels` 映射双口径，见 §3）
  - 是否出现归属争议（一个 issue 被多人反复改 SIG label）
  - 是否有 contributor 主动通过 SIG 归属讨论问题
  - 口径说明：豁免期内 lead 本人及其登记 Agent 的产出会计入活跃度，评估时应**单列**这部分数据作为参考基线。该口径是退出决策的**评估参考项**而非额外门槛——是否暂缓转 `active` 由 maintainer 在退出决策中结合单主体占比判断（如单主体占比 100% 时倾向延期一个观察窗）
- **失败判定标准**（满足任一即判定试点失败）：
  - 试点期内出现 ≥ 2 次无法在 1 周内解决的归属争议
  - ≥ 2 个 SIG 在观察窗结束时活跃度为零
  - 发现规范条款与既有 `spec/` 规则产生实际冲突
- **回退动作清单**（按失败程度递进，均走 PR）：
  - 收缩：调整相关 SIG 的 `scope` / `scope_labels`，消除争议范围
  - 撤销单个 SIG：YAML `status: archived`（走 §3 incubating → archived 路径），label 保留供历史查询（描述加 `[deprecated]`）
  - 整体回退：归档全部试点 SIG，本规范标记为 `experimental` 冻结，待条件成熟重新 propose
- **退出决策**：试点结束后由 maintainer 评估，决定是否扩展到 5 个 SIG、是否引入 `gc sig` 命令、是否引入例会机制

## 10. 与 `gc` CLI 的关系（约束）

- Phase 1：**不实现** `gc sig` 命令，SIG 信息通过 label 与 SIG 讨论通道（§6.2）直接使用
- Phase 3（可选）：仅允许 `gc sig list` / `gc sig view` / `gc sig issues` / `gc sig prs` 等**只读**命令
- 任何情况下不允许 `gc sig create` / `gc sig edit` / `gc sig archive`，SIG 生命周期变更必须通过 PR 修改 YAML
- 未来 `gc sig list --json` / `gc sig view <name> --json` 等只读命令的 JSON 输出应包含 `agents` 字段（列出该 SIG 注册的 Agent 及其 operator），便于 AI 代理/脚本查询自身被授权的范围

## 11. 常见反模式

- 把 SIG 当成分蛋糕工具，为每个目录强行指定 SIG
- 把 SIG lead 当成审批瓶颈，所有跨目录 PR 都要 SIG lead 同意
- 把 SIG 会议当成决策唯一来源，绕过 issue / PR 异步讨论
- 把 SIG label 当成 status label 使用，混用 `sig/api` 与 `status/in-progress`

## 12. 参考

- Kubernetes SIG 治理模型：https://github.com/kubernetes/community/blob/master/governance.md
- Python SIG 模型：https://www.python.org/community/sigs/
- 本仓库既有规范：`spec/README.md`、`spec/governance/docs-governance.md`、`spec/governance/ai-collaboration.md`
