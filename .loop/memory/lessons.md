# 经验教训（团队共享）

跨会话积累的操作经验，供所有 AI 协作者和人工维护者参考。

## Loop 运行

1. **后台 loop 用 Cron 模式** — ScheduleWakeup 在用户活跃时不触发，Cron 固定时钟可靠
2. **一条全流程 loop 优于多条接力** — triage 线和 verified 线没有独立存在价值
3. **spec 门禁表是 /loop 的基石** — spec 全则 loop 全，spec 漏则 loop 漏

## Agent 可靠性

4. **多角色评审 Agent ~50% 静默失败率** — 2 分钟无响应则手动补齐评审
5. **风险评估脚本误判** — `classify-change-risk.py` 扫描全部累积 diff，小改动也被标 risk/high
6. **建议增加 `--head-only` 模式** — 仅扫描当前分支 diff

## 环境

7. **GitHub 代理阻断** — `HTTP_PROXY` 环境变量导致 `gh` 返回 EOF，需 `unset` 后使用
8. **GitCode PR 405** — `gc pr merge --yes` 可能返回 405，fallback: `git merge` + `git push`
9. **CI 触发方式** — 项目 CI 由 `pull_request` 与 `push`（限 `main`）事件触发；`main` 合并/镜像同步推送会自动跑 CI，非 main 分支推送仍需 `gh workflow run` 手动 dispatch

## 门禁

10. **docs-only 不能全跳** — 仍需 pre-commit + 安全审查 + 2 角色评审（docs+security），见 spec 5.3
11. **实际命令测试必须用 infra-test/\*** — 不得用生产仓库测试

## git 操作

12. **`git commit --amend` 前置条件** — pre-commit hook 失败后不能直接 amend，需先 `git add` 修复文件再 commit

## Bash + Python 交互

13. **`"$VAR"` 在 bash heredoc 内截断 Python 代码** — bash 在 `"..."` 内将 `"` 解析为字符串结束符。Python 路径引用必须用单引号 `'$VAR'`。此 bug 导致 loop token 注入 100% 失败，从第一天起就存在，直到 6/27 修复。见 #373。

14. **内联 Python 只适合 5 行以内** — heredoc 内的 `python3 -c` 面临三重陷阱：bash 引号截断、tab/空格混用、跨块变量不可见。超过 5 行就应提取为独立 `.py` 文件。见 dfb8b20 重构（260→60 行脚本）。

## 脚本可靠性

15. **`sleep N` 不是竞态修复** — 管道关闭即文件完整，`readlines()` 本身阻塞到 EOF。固定延迟在慢 FS/大输出下不可靠，正常场景下浪费时间。见 #383。

16. **统计正则必须匹配实际数据格式** — `count-deliveries.sh` 的 `(\d+)k` 只匹配整数 k 值，`4.7M`、`1.5M(1.5M cache)` 全部漏掉。应解析目标列而非全文扫描，支持 k/M 双后缀 + 小数。

17. **硬编码日期随时间退化** — `--since=2026-06-26` 一周后就开始漏数据。用 `datetime.now() - timedelta(days=60)` 动态计算。

## Loop 运维

18. **stale PID 文件阻断 cron** — 进程被 kill 后 `trap cleanup` 可能未执行（SIGKILL），PID 文件残留导致下次 cron tick 误判 SKIP。应在启动时做 liveness 检查（`kill -0`）——已有此逻辑，但需确认 trap 覆盖 SIGKILL 场景。

19. **已修复的 issue 要及时关闭** — #373 #375 在 dfb8b20 合入 main 后仍 open 数天，污染 issue 列表。修复代码合入后应立即验证并关闭对应 issue。

---

**最后更新**: 2026-06-27
