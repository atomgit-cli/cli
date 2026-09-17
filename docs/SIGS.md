# Special Interest Groups (SIGs)

本文档面向贡献者介绍 gitcode-cli 项目的 SIG（Special Interest Group）机制、当前 SIG 列表、如何参与以及如何提议新 SIG。

正式治理规则以 [spec/governance/sig-governance.md](../spec/governance/sig-governance.md) 为准；本页只做导航与简介。

## 什么是 SIG

SIG 是 gitcode-cli 项目对某些**跨人协调密集的领域**设置的兴趣小组。每个 SIG：

- 有明确的**范围**（对应仓库内的目录或主题）
- 有明确的**责任主体**（lead + maintainer）
- 通过 **`sig/<name>` label** 标识相关 issue / PR
- 通过 **discussion 分类** 承载异步讨论与决策记录

SIG **不**替代 `spec/` 中的任何规则，也不替代现有的 issue / PR workflow。

## 当前 SIG 列表

| SIG | 范围 | 状态 | 注册的 Agent | Discussion |
|-----|------|------|--------------|-----------|
| **sig/api** ⚠️ | `api/`、`api-doc/` — GitCode API 客户端、endpoint 对接、api-doc 同步 | incubating | claude-code, trae-ai | SIG: API |
| **sig/cli-ux** ⚠️ | `pkg/cmd/`、`pkg/cmdutil/`、`pkg/output/`、`pkg/iostreams/` — 命令交互、`--json` 输出、错误提示、退出码 | incubating | claude-code, trae-ai | SIG: CLI UX |
| **sig/governance** ⚠️ | `spec/`、`docs/`、`AGENTS.md`、`CLAUDE.md`、`CONTRIBUTING.md` — 规范演进、文档治理、AI 协作边界 | incubating | claude-code, trae-ai | SIG: Governance |

> ⚠️ **单人维护豁免期**：当前仓库 maintainer 总人数 < 2，三个 SIG 的 lead 与 maintainer 均为同一人，处于 `spec/governance/sig-governance.md` §4.2 豁免条款覆盖范围。新增仓库 maintainer 后 30 天内将完成整改。
>
> 三个试点 SIG 当前均为 `incubating`（试运行）状态，按 sig-governance.md §3 的退出条件达标后转 `active`。

完整元数据见 [`spec/governance/sigs/`](../spec/governance/sigs/) 目录下对应的 `<sig-name>.yaml` 文件。

## 我该怎么用 SIG

### 作为贡献者

1. **提 issue 时**：如果你的问题清晰属于某个模块，在描述里提一下模块名即可；triager 会按模块打 `scope/*` label。日常 issue 不需要 `sig/*` label——只有当议题需要 SIG 治理介入（跨 SIG 协调、SIG 治理变更、SIG 路线图讨论）时才会额外打 `sig/*` label（规则详见 sig-governance.md §6.1）
2. **提 PR 时**：如果改动落在某个 SIG 范围，PR 描述里提一下相关 SIG；reviewer 会按需打 label
3. **参与讨论**：到对应 SIG 的 discussion 分类下发帖，比 issue 更适合开放性话题

### 作为 reviewer / maintainer

1. triage 新 issue 时，**默认只打 `scope/*` label**（对应 `spec/governance/sigs/*.yaml` 的 `scope` 字段映射）
2. 仅当 issue 需要 SIG 治理介入时（跨 SIG 协调、SIG 治理变更、活跃度统计、SIG 路线图讨论），才**额外**打 `sig/*` label；拿不准主 SIG 时先打一个，并在 comment 中提及其他相关 SIG
3. 跨 SIG 议题放到 `General` discussion，由 `sig/governance` 协调
4. **注意**：`gc pr edit --labels` 是替换语义，更新 PR 状态时 `--labels` 必须同时携带已打的 `sig/*` 与 `scope/*` label，否则会被静默移除

### 作为 Agent（AI 开发者）

本项目把 Agent 视为**一等开发者**。详细规则见 `spec/governance/sig-governance.md` §4.3，关键要点：

**你能做的**：
- 独立承担 SIG 范围内的开发任务（PR、文档、分析、issue 回答）
- 你的产出与人类 contributor 同等计入 SIG 活跃度，并纳入 `incubating → active` 生命周期评估（§3 归属计数包含 Agent 产出）
- 通过 `spec/governance/sigs/*.yaml` 的 `agents` 字段查询你在哪些 SIG 范围内被授权

**你必须遵守的边界**：
- 不能担任 `lead` 或 `maintainer`
- 不能做出 merge / approve / request-changes 等评审决策
- 不能未经人 review 直接修改 `spec/` 目录
- 不能任命或移除其他 SIG 角色
- 不能单独发起 SIG 生命周期变更

**产出标识**：
- **独立完成的 commit**：commit message 必须带 trailer：`Signed-off-by-Agent: <agent-id>`
- **与人协作的 commit**：只带 `Co-Authored-By`，不带 `Signed-off-by-Agent`（两种 trailer 的场景区分详见 sig-governance.md §4.3）
- PR / issue comment 描述中说明 `Agent: <agent-id>`

**责任链**：你的所有产出由你登记的 operator 承担完全责任。如果你不确定边界，请先与你的 operator 确认再行动。

## 如何提议新 SIG

1. 先开一个 issue，标题格式：`docs: propose sig/<name>`，并使用 `sig/governance` + `type/docs` label
2. 在 issue 中说明：
   - 拟定的 SIG 名称与范围（必须映射到仓库内的目录或主题）
   - 为什么现有 SIG 无法覆盖
   - 拟任的 lead 与 maintainer
   - 试运行期内的预期工作量（至少 ≥ 5 个 issue / PR 规模）
3. maintainer 评审通过后，提交 PR 添加 `spec/governance/sigs/<name>.yaml`，状态为 `incubating`
4. 试运行 ≥ 4 周且达标后，由 maintainer 转为 `active`

## 如何加入现有 SIG

加入条件与任命流程的正式规则见 sig-governance.md §4.4，简要路径：

1. 先在该 SIG 范围内提交 ≥ 2 个被合并的 PR，或持续参与该 SIG discussion
2. 在对应 SIG 的 discussion 分类下自荐
3. 由该 SIG lead 决定接纳为 maintainer，并走 PR 更新 YAML
4. lead 任命由仓库 maintainer 团队决定

## 常见问题

**Q：我的 issue 跨多个 SIG，怎么办？**
A：打多个 `sig/*` label 即可；description 里说明哪个 SIG 是主负责方。

**Q：SIG lead 不响应怎么办？**
A：按 sig-governance.md §4.4"SIG lead 缺位处理"执行：先在该 SIG discussion 分类下提醒；连续 ≥ 2 周无响应时，在 `sig/governance` discussion 提出，由仓库 maintainer 介入。

**Q：能用 `gc` 命令查 SIG 信息吗？**
A：Phase 1 试点期间不行。可以：
- 在仓库内直接看 `spec/governance/sigs/*.yaml`
- 用 `gc issue list --label sig/api` 这样的命令按 label 过滤

未来可能引入 `gc sig list / view` 只读命令，但**永远不会**引入 `gc sig create / edit / archive`——治理变更必须走 PR。

**Q：我有一个大的设计想法（新抽象、影响多个 SIG），该提到哪？**
A：先看 `docs/rfcs/README.md` 的对照表——"大到 issue / discussion 装不下"的设计走 RFC 流程；拿不准时先开 issue，由 maintainer 判断是否升级为 RFC。

**Q：SIG 决策可以推翻 `spec/` 中的规则吗？**
A：不能。SIG 决策必须在 `spec/` 框架内做出；如果要改 `spec/`，必须走 PR 流程修改对应规范文档。

## 相关链接

- [spec/governance/sig-governance.md](../spec/governance/sig-governance.md) — 正式治理规则
- [spec/governance/sigs/](../spec/governance/sigs/) — SIG 元数据 YAML
- [docs/rfcs/](./rfcs/) — 大型设计提案（RFC）目录
- [docs/COMMANDS.md](./COMMANDS.md) — 命令行为真相源
- [spec/README.md](../spec/README.md) — 规范入口
