# RFC 工作流程

本文件定义 gitcode-cli 仓库中 RFC（Request for Comments，大型设计提案）的触发条件、流程规则与结果判定。RFC 文件存放于 `docs/rfcs/`，其流程规则的真相源为本文件。

## 职责

- 定义何时需要 RFC、何时用 issue / 普通 PR / SIG 讨论通道
- 定义 RFC 的编号、提交流程与结果判定
- 约束 RFC 与 `spec/`、SIG 治理的关系

## 适用场景

- 提案新抽象、非平凡 tradeoff、影响多个 SIG 的全局性设计
- 判断一个议题应该走 RFC 还是普通 issue / PR

## 必须

### 触发判断

| 改动类型 | 承载方式 |
|---------|---------|
| bug fix、小改进、文档修正 | 普通 PR |
| 中等功能 | `type/feature` issue |
| SIG 内重大方向 | SIG 讨论通道异步公示（[sig-governance.md](../governance/sig-governance.md) §7） |
| 跨 SIG / 全局性大设计 | **RFC（`docs/rfcs/`）** |
| 治理规则变更 | `spec/` PR（不走 RFC） |

**判断标准**：当设计讨论"大到一个 issue / 讨论通道装不下"时使用 RFC——典型特征是新抽象、非平凡 tradeoff、影响多个 SIG。拿不准时先开 issue 询问，由 maintainer 判断是否需要升级为 RFC。

### 流程

1. 复制 `docs/rfcs/RFC-0000-template.md` 为 `RFC-NNNN-<short-name>.md`（`NNNN` = 目录中已存在的最大编号 + 1，4 位数字补零；编号以 merge 时目录现状为准，撞号时作者须重命名）
2. 填写后提 PR（标题含 RFC 编号），按 [pr-workflow.md](./pr-workflow.md) 的状态机推进
3. 评审与共识：归属 SIG lead 必须参与；涉及多个 SIG 时相关 SIG lead 均须表态；讨论在 RFC PR 中进行；**无归属 SIG 的主题（如 testing、build、release）由 `sig/governance` 兜底参与**
4. 结果判定：
   - **PR merge = RFC 被接受**，文件即定稿（接受后的修改走新 PR）
   - **PR 关闭未 merge = 被拒绝**；如未来可能重启，在关闭 comment 中标注 `shelved` 及原因（shelved RFC 不落盘文件，登记表不记录；重启时另起编号）
5. 实施：被接受的 RFC 须开追踪 issue（描述中关联 RFC 编号）；**作者不强制自己实施**，其他人可认领

### 共识与 Agent

- RFC 的共识判定遵循 [sig-governance.md](../governance/sig-governance.md) §7：Agent 表态不计入 consensus 基数
- 登记 Agent 可以作为作者提交 RFC（描述中注明 `Agent: <agent-id>` 与 operator），但 RFC 的接受与否由人决定（归属 SIG lead + PR review merge）

## 禁止

- 把 RFC 当作规则生效通道：RFC 是**设计提案**，被接受不等于规则生效；若涉及 `spec/` 规则变更，实施后仍须走 `spec/` PR 把规则写入规范
- 通过 RFC 绕过 [source-of-truth-matrix.md](../governance/source-of-truth-matrix.md) 定义的真相源边界
- 在 `docs/rfcs/` 中登记未走完流程的提案（登记表只登记 accepted 状态的 RFC）

## 不负责什么

- RFC 文件的模板内容（由 `docs/rfcs/RFC-0000-template.md` 承载）
- SIG 治理本身（由 [sig-governance.md](../governance/sig-governance.md) 负责）
- PR 评审状态机（由 [pr-workflow.md](./pr-workflow.md) 负责）

## 参考

- PyTorch RFC 流程：https://github.com/pytorch/rfcs
- 本仓库 RFC 目录导航：`docs/rfcs/README.md`
