# RFC（Request for Comments）

本目录承载 gitcode-cli 的**大型设计提案**。**流程规则（触发判断、编号、结果判定、共识约束）的真相源是 [spec/workflows/rfc-workflow.md](../../spec/workflows/rfc-workflow.md)**；本页只做导航与使用说明。

## 快速使用

1. 判断是否需要 RFC：跨 SIG / 全局性大设计（新抽象、非平凡 tradeoff）→ RFC；拿不准先开 issue 问 maintainer。完整对照表见 [rfc-workflow.md](../../spec/workflows/rfc-workflow.md) "触发判断"
2. 复制 `RFC-0000-template.md` 为 `RFC-NNNN-<short-name>.md`（编号 = 目录现有最大编号 + 1，4 位补零；以 merge 时目录现状为准，撞号须重命名）
3. 填写后提 PR（标题含 RFC 编号），按 [spec/workflows/pr-workflow.md](../../spec/workflows/pr-workflow.md) 状态机推进；归属 SIG lead 必须参与评审，无归属 SIG 的主题由 `sig/governance` 兜底
4. PR merge = RFC 被接受；关闭未 merge = 被拒绝（可能重启时在关闭 comment 标注 `shelved`，不落盘不登记）
5. 被接受的 RFC 开追踪 issue 实施；作者不强制自己实施

## 与 SIG / spec 的关系

- RFC 按**主题归属**打对应 `sig/*` label；本目录的流程维护由 `sig/governance` 负责
- RFC 是**设计提案**，被接受不等于规则生效：涉及 `spec/` 规则变更的，实施后仍须走 `spec/` PR
- Agent 可作为 RFC 作者（注明 `Agent: <agent-id>` 与 operator），但表态不计入 consensus、接受与否由人决定（详见 [sig-governance.md](../../spec/governance/sig-governance.md) §7）

## 当前 RFC 列表

（暂无。**仅 accepted 状态**的 RFC 在此登记一行：`RFC-NNNN | 标题 | 归属 SIG | accepted | 追踪 issue`；shelved 记录见对应关闭 PR 的 comment，不在此登记）
