# Delivery Record: Issue #413
- Title: fix(api): itoa(0) 返回 "30" 导致 API 分页参数异常
- Type: bug
- Risk: risk/low
- Scope: scope/api
- Status: merged (via !478)

## State Transitions
| From | To | When | Evidence |
|------|----|------|----------|
| status/triage | status/verified | 2026-07-08 | 复现 `itoa(0)` 返回 `"30"`（`i <= 0` 分支回退默认值） |
| status/verified | merged | 2026-08-10 | 修复经 !478 合入 `origin/main`（`f6e85fe`） |

## 复现确认
- `itoa(0)` = `"30"` (expected `"0"`)  ❌
- `itoa(1)` = `"1"` ✓
- `itoa(30)` = `"30"` ✓
- `itoa(100)` = `"100"` ✓

## 影响范围
- `itoa` 定义于 `queries_repo.go:250`，生产调用 48 处：`queries_pr.go` 26、`queries_issue.go` 15、`queries_label_milestone.go` 7
- `itoa64`（`queries_release.go`，10 处调用）实现正确，不受影响
- 当 `PerPage=0` 时向 API 发送 `per_page=30`

## Key Artifacts
- 首个 PR: [#358](https://gitcode.com/gitcode-cli/cli/merge_requests/358)（因与主干冲突关闭，未合入；其中 `api/itoa_regression_test.go` 附加 3 个回归测试，覆盖 GetIssue / GetPullRequest / GetMilestone 路径构造，验证 0/1/999 边界）
- 修复合入: [!478](https://gitcode.com/gitcode-cli/cli/merge_requests/478)（`f6e85fe`，2026-08-10，9 文件：`itoa`/`itoa64` 定义删除，全部调用点替换为 `strconv.Itoa`/`strconv.FormatInt`；测试文件为纯替换，无新增用例）

## Gates Summary
| # | Gate | Result |
|---|------|--------|
| 1 | 问题分类 | ✅ type: bug |
| 2 | 复现验证 | ✅ `itoa(0)` 返回 `"30"` |
| 3 | 影响分析 | ✅ 48 处 `itoa` + 10 处 `itoa64` 调用点已审计 |
| 4 | 方案评审 | ✅ strconv 标准库替换，3 个回归测试（随 #358 提交） |
| 5 | 风险分级 | ✅ risk/low |
| 6 | 标签更新 | ✅ bug, status/verified |
| 7 | 验证 comment | ✅ [comment_179075535](https://gitcode.com/gitcode-cli/cli/issues/413#comment_179075535) |
| 8 | 主干合入 | ✅ !478 merged；主干 `api/` 无 `itoa`/`itoa64` 定义及调用残留 |

## 补录说明
- 本记录 2026-09-15 补录入库（原始验证 2026-07-08）
- 首个 PR #358 因与主干冲突关闭；修复经 `fix/issue-413-itoa-strconv` 分支以 !478 合入主干

ISSUE_NUM=413
