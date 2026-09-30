# 核心回归矩阵

本文档定义当前里程碑的最小真实命令回归集，对应脚本：

```bash
./scripts/regression-core.sh
go test -tags=system ./tests/system
./tests/system/run.sh --read
```

`scripts/regression-core.sh` 是历史最小冒烟回归入口。新增、补充或扩展真实命令行系统测试时，优先放入 `tests/system/`。

## 目标

- 覆盖核心用户路径
- 覆盖关键错误路径
- 让实际命令验证可重复执行，而不是每个 issue 临时补一组命令

## 前置条件

```bash
go build -o ./gc ./cmd/gc
./gc auth status
```

回归脚本不会读取、导出或管道传递真实 `GC_TOKEN` / `GITCODE_TOKEN`。请提前通过 `gc auth login` 或自行管理的环境完成认证，并只用 `gc auth status` 验证认证状态。

测试仓库限制：
- 只使用 `infra-test/gctest1` 或 `infra-test` 组织下仓库
- 不使用 `gitcode-cli/cli`
- 不使用个人仓库

## System Test Suite

`tests/system/` is the structured real-command test suite. It enforces the same repository boundary for read and write cases: every repository target must be `infra-test/*`.

The primary runner is Go testscript, guarded by the `system` build tag:

```bash
go test -tags=system ./tests/system
make system-test
```

Default read-only suite:

```bash
./tests/system/run.sh --read
```

Explicit write suite:

```bash
./tests/system/run.sh --write --write-repo infra-test/gctest1
GC_SYSTEM_WRITE=1 go test -tags=system ./tests/system -run TestWriteScripts
make system-test-write
```

Assignee write scenarios require an assignable username for the current
authentication:

```bash
GC_SYSTEM_WRITE=1 GC_SYSTEM_ASSIGNEE=<username> go test -tags=system ./tests/system -run TestWriteScripts
```

This scenario creates two temporary issues only in `infra-test/*`, verifies real
`issue create --assignee` and `issue edit --assignee` writes by reading them
back, and closes both temporary issues during cleanup.

Run `gc auth login` first. The system runner intentionally does not copy real
`GC_TOKEN` or `GITCODE_TOKEN` values into testscript because verbose testscript
output includes its environment.

PR write-path cases require a prepared test branch:

```bash
GC_SYSTEM_PR_HEAD=test-branch ./tests/system/run.sh --write --write-repo infra-test/gctest1
```

Cases live under:

- `tests/system/testdata/read/`
- `tests/system/testdata/write/`
- `tests/system/cases/read/`
- `tests/system/cases/write/`

The testscript suite provides contract helpers including `json-ok`,
`json-assert`, `stdout2env`, and `require-infra`. Use `json-assert` for key
field/type checks whenever a command supports `--json`.

## 默认回归集

`./scripts/regression-core.sh` 默认执行稳定的读路径、agent-friendly 契约检查和错误路径：

1. `auth status`
2. `auth token` 非交互保护（仅使用 fake token）
3. 无认证 `pr review --approve` 错误路径，期望退出码 `4`
4. `repo view`
5. `repo view --json`
6. `issue list --limit 1`
7. `issue list --json`
8. `issue view <list 返回的首个 issue>`
9. `issue view --json`
10. `pr list --json`
11. `pr list --paginate --per-page 1 --limit 1 --json`
12. `repo log --limit 1 --json`
13. `gc api repos/<repo>`
14. `release list --json`
15. `release delete <tag> --dry-run`
16. 非 Git 目录下的 `repo view` 错误路径，期望退出码 `2`
17. 不存在 commit 的 `commit view` 错误路径，期望退出码 `3`

说明：
- 脚本不会读取真实 token，不会执行 `auth login --with-token`，也不会执行 `auth logout`。
- 无认证错误路径会使用临时空 `GC_CONFIG_DIR` 和空环境变量执行单条命令，避免污染本地长期配置。
- `release delete` 的 dry-run 默认使用 `GC_REGRESSION_RELEASE_TAG`，未显式设置时默认取 `v0.0.1-test`。
- 真实删除请求当前被平台以 `405 Method Not Allowed` 拒绝，命令错误信息包含该 Release 的网页链接指引。回归集只覆盖 `--dry-run`，不执行真实删除（平台将来启用删除 API 后也应如此，避免删除 infra-test 资源）。

## 可选写路径

`pr create` 属于有副作用的写路径，不默认执行。显式开启方式：

```bash
export GC_REGRESSION_WRITE=1
export GC_REGRESSION_PR_REPO=infra-test/some-test-repo
export GC_REGRESSION_PR_HEAD=feature-branch
export GC_REGRESSION_PR_BASE=main
./scripts/regression-core.sh
```

说明：
- 只有在明确提供目标仓库和 head 分支时才执行 `pr create`
- 这样可以避免默认污染测试仓库，同时仍把 `pr create` 纳入统一回归矩阵

如果本次改动涉及 issue 标签写路径、issue 状态流转或 PR 状态流转，再补以下真实命令验证：

```bash
# issue label / close
issue_number=$(./gc issue create -R infra-test/gctest1 --title "Regression" --body "write-path" | sed -n 's/.*#\([0-9][0-9]*\).*/\1/p')
./gc issue label "$issue_number" -R infra-test/gctest1 --add bug
./gc issue label "$issue_number" -R infra-test/gctest1 --remove bug
./gc issue label "$issue_number" -R infra-test/gctest1 --list
./gc issue view "$issue_number" -R infra-test/gctest1 --json
./gc issue close "$issue_number" -R infra-test/gctest1 --yes

# pr close
./gc pr create -R infra-test/gctest1 --head <test-branch> --base main --title "Regression PR" --body "write-path"
./gc pr close <pr-number> -R infra-test/gctest1 --yes
./gc pr view <pr-number> -R infra-test/gctest1 --json
```

如果本次改动涉及写路径 `--json`，再补以下测试仓库验证：

```bash
# issue create --json: 必须创建到 infra-test/*，并用返回 number 回读后关闭
./gc issue create -R infra-test/gctest1 --title "Regression JSON dry-run" --body "dry-run json" --dry-run --json | python3 -m json.tool
issue_json=$(./gc issue create -R infra-test/gctest1 --title "Regression JSON" --body "write-path json" --json)
issue_number=$(printf '%s\n' "$issue_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["number"])')
./gc issue view "$issue_number" -R infra-test/gctest1 --json
./gc issue close "$issue_number" -R infra-test/gctest1 --yes

# pr create --json: 仅在显式准备好测试 head 分支时执行，并回读 PR
./gc pr create -R infra-test/gctest1 --head <test-branch> --base main --title "Regression PR" --body "write-path json" --json
./gc pr view <pr-number> -R infra-test/gctest1 --json

# P1 写路径 JSON：只在准备好可清理测试资源时执行真实写入；否则至少执行 help/schema 和 no-write 错误路径验证
./gc issue edit <issue-number> -R infra-test/gctest1 --title "Regression JSON edit" --json | python3 -m json.tool
./gc release create <test-tag> -R infra-test/gctest1 --title "Regression JSON release" --notes "write-path json" --json | python3 -m json.tool
./gc pr merge <test-pr-number> -R infra-test/gctest1 --yes --json | python3 -m json.tool

# P2 写路径 JSON：fork/upload 容易产生远端残留，真实写入前必须确认测试资源可清理
./gc repo fork <owner/repo> --json | python3 -m json.tool
./gc release upload <test-tag> <file> -R infra-test/gctest1 --json | python3 -m json.tool
```

说明：
- 对写路径命令不能只看退出码，必须回读远端状态。
- 对写路径 `--json`，还必须确认 stdout 是可解析 JSON，且没有混入文本提示。
- `issue label --remove` 后，`--list` / `issue view --json` 不应再包含目标标签。
- `issue close` 后，`issue view --json` 的 `state` 应变为 `closed`。
- `pr close` 后，`pr view --json` 的 `state` 应变为 `closed`。

## Agent-Friendly 契约补充回归

除核心脚本已覆盖的检查外，当前里程碑还推荐以下补充检查：

```bash
# 命令元数据
./gc schema
./gc schema "issue view"
./gc schema "issue list"

# 结构化输出
./gc repo view infra-test/gctest1 --json
./gc issue list -R infra-test/gctest1 --limit 1 --json
./gc issue list -R infra-test/gctest1 --format json
./gc issue list -R infra-test/gctest1 --format table
./gc issue list -R infra-test/gctest1 --format simple
./gc issue list -R infra-test/gctest1 --time-format absolute
./gc issue list -R infra-test/gctest1 --time-format relative
./gc issue list -R infra-test/gctest1 --template '{{range .}}{{.Number}}{{"\n"}}{{end}}'
./gc issue view 1 -R infra-test/gctest1 --json
./gc issue view 1 -R infra-test/gctest1
./gc pr list -R infra-test/gctest1 --json
./gc pr list -R infra-test/gctest1 --paginate --per-page 100 --limit 200 --json
./gc pr list -R infra-test/gctest1 --commit-message "fix login" --json
./gc pr view 1 -R infra-test/gctest1 --json
./gc pr view 1 -R infra-test/gctest1
./gc pr comments 1 -R infra-test/gctest1 --json
./gc issue prs 1 -R infra-test/gctest1 --json
./gc repo log -R infra-test/gctest1 --file README.md --branch main --limit 5 --json
./gc api repos/infra-test/gctest1
./gc api 'repos/infra-test/gctest1/commits?path=README.md&sha=main'
./gc repo stats -R infra-test/gctest1 --branch main --json
./gc milestone list -R infra-test/gctest1 --json
./gc milestone view <milestone-number> -R infra-test/gctest1 --json
./gc release list -R infra-test/gctest1 --json
./gc commit comments list -R infra-test/gctest1 --json
./gc commit comments list-by-sha <sha> -R infra-test/gctest1 --json
./gc commit comments view <comment-id> -R infra-test/gctest1 --json

# dry-run
./gc repo delete infra-test/gctest1 --dry-run
./gc label delete bug -R infra-test/gctest1 --dry-run
./gc milestone delete 1 -R infra-test/gctest1 --dry-run
./gc release delete v0.0.0 -R infra-test/gctest1 --dry-run
./gc issue create -R infra-test/gctest1 --title "Regression" --body "dry-run" --dry-run
```

说明：
- 上述 dry-run 命令不应执行真实写入。
- 非交互环境中未传 `--yes` 的删除、关闭、重开、PR 状态切换、合并、同步推送/建 PR 命令应直接失败，不应挂起等待输入。
- `issue list --format yaml` 应返回用法错误，不应静默回退到默认输出。
- 参数/用法错误应返回退出码 `2`；认证或权限错误应返回退出码 `4`；API 404 或 body 内嵌 `error_code: 404` 应返回退出码 `3`。
- `issue list --json` 与 `issue list --format json` 应保持等价。
- `issue list --time-format absolute|relative` 仅影响文本展示，不应改变 JSON 输出。
- `gc schema "issue list"` 应为 `format`、`time-format`、`state` 暴露稳定 `enum` 值。
- `issue list --template` 应输出模板结果，并与 `--json`、`--format` 的冲突保持稳定报错。
- `issue view` 和 `pr view` 的文本输出应保留稳定的详情布局，`--json` 继续输出结构化数据。
- `pr view --json` 应包含 `body`、`description`、`merged`、`merged_at`，并在远端详情接口统计为 0 时尽量通过 files/commits API 补齐统计。
- 已合并 PR 的 `pr view --json` 与 `pr list --state merged --json` 应返回 `merged=true`，即使远端原始字段为 `merged: null`；`state` 保留远端返回值，不由 CLI 改写。
- `gc api` 应输出远端原始响应，不额外混入文本提示。
- inline `--body`/`--comment`/`--description`/`--notes` 在内容含当前 `GC_TOKEN`/`GITCODE_TOKEN` 值时应被 `ScanContentForSecrets` 拒绝（系统测试 `secret-scan.txtar` 覆盖全部 22 条 inline 路径）。

## 推荐记录方式

在 PR 描述或 issue comment 中记录：

```markdown
## Regression

- [x] ./scripts/regression-core.sh
- [x] read-only checks passed
- [ ] write-path checks skipped
```
