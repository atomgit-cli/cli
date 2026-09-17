# RFC（Request for Comments）

本目录承载 gitcode-cli 的**大型设计提案**。流程参考 [PyTorch RFC](https://github.com/pytorch/rfcs)，按本项目规模精简（仓内目录而非独立仓库）。

## 什么时候需要 RFC

| 改动类型 | 承载方式 |
|---------|---------|
| bug fix、小改进、文档修正 | 普通 PR |
| 中等功能 | `type/feature` issue |
| SIG 内重大方向 | SIG discussion 异步公示（sig-governance.md §7） |
| 跨 SIG / 全局性大设计 | **RFC（本目录）** |
| 治理规则变更 | `spec/` PR（不走 RFC） |

**判断标准**：当设计讨论"大到一个 issue / discussion 装不下"时使用 RFC——典型特征是新抽象、非平凡 tradeoff、影响多个 SIG。拿不准时先开 issue 询问，由 maintainer 判断是否需要升级为 RFC。

## 流程

1. 复制 `RFC-0000-template.md` 为 `RFC-NNNN-<short-name>.md`（`NNNN` = 当前目录已存在的最大编号 + 1，4 位数字补零）
2. 填写后提 PR（标题含 RFC 编号），按 [spec/workflows/pr-workflow.md](../../spec/workflows/pr-workflow.md) 的状态机推进
3. 评审与共识：归属 SIG lead 必须参与；涉及多个 SIG 时相关 SIG lead 均须表态；讨论在 RFC PR 中进行
4. 结果判定：
   - **PR merge = RFC 被接受**，文件即定稿（接受后的修改走新 PR）
   - **PR 关闭未 merge = 被拒绝**；如未来可能重启，在关闭 comment 中标注 `shelved` 及原因
5. 实施：被接受的 RFC 须开追踪 issue（描述中关联 RFC 编号）；**作者不强制自己实施**，其他人可认领

## 与 SIG / spec 的关系

- RFC 按**主题归属**打对应 `sig/*` label（如 API 客户端大版本打 `sig/api`）；本目录的流程维护由 `sig/governance` 负责
- RFC 是**设计提案**，被接受不等于规则生效：若涉及 `spec/` 规则变更，实施后仍须走 `spec/` PR 把规则写入规范
- RFC 的共识判定遵循 [spec/governance/sig-governance.md](../../spec/governance/sig-governance.md) §7：Agent 表态不计入 consensus 基数；Agent 可以作为作者提交 RFC，但接受与否由人决定

## 当前 RFC 列表

（暂无。RFC 被接受后在此登记一行：`RFC-NNNN | 标题 | 归属 SIG | 状态（accepted/shelved）| 追踪 issue`）
