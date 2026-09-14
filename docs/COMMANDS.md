# GitCode CLI 命令使用指南

> 项目概述和功能介绍请参阅 [README.md](../README.md)，开发与规范入口请参阅 [spec/README.md](../spec/README.md)，打包发布请参阅 [PACKAGING.md](./PACKAGING.md)。

本文档提供 `gc` 命令行工具所有命令的实际使用示例。

## 前置准备

### 仓库参数格式

大多数接受仓库参数的命令现在统一支持以下格式：

```bash
owner/repo
https://gitcode.com/owner/repo
git@gitcode.com:owner/repo.git
```

SSH default for code transfer:
- Code download and sync paths use SSH by default.
- `repo clone owner/repo` defaults to `git@gitcode.com:owner/repo.git` unless `--git-protocol https` or a saved config explicitly selects HTTPS.
- SSH-based code transfer requires a local SSH key with access to `git@gitcode.com`.


说明：
- 未显式传 `-R` 的命令，仍按各自命令说明决定是否支持从当前 Git 仓库自动推断。
- 传入 HTTPS 或 SSH 仓库地址时，CLI 会统一解析出目标仓库，不再要求手工改写成 `owner/repo`。

### Windows PowerShell 命令名和 stdin

Windows PowerShell 预置 `gc` 作为 `Get-Content` 别名。若在 PowerShell 中使用 CLI，推荐写完整命令名 `gitcode`；也可以显式调用 `gc.exe`。

当中文或其他非 ASCII 正文需要通过 `--body-file -` / `--comment-file -` 从 stdin 传入时，推荐使用 UTF-8 文件：

```powershell
Set-Content -Path body.md -Value "中文正文" -Encoding UTF8
gitcode issue create -R owner/repo --title "标题" --body-file body.md
```

若必须直接管道传入，请先设置 UTF-8 输出编码：

```powershell
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
"中文正文" | gitcode issue create -R owner/repo --title "标题" --body-file -
```

CLI 只会在显式 stdin 文本 flag（当前包括 `--body-file -` 和 `--comment-file -`）上拦截疑似已被 Windows PowerShell 损坏成 `???` 的输入，并在 stderr 提示正确用法；如果确实需要原样传入连续问号，可设置 `GITCODE_CLI_ALLOW_LOSSY_STDIN=1`。

当前自动推断边界：
- 仅显式接入 `cmdutil.ResolveRepo(...)` 的命令支持缺省 `-R` 时从当前 Git 仓库推断目标仓库，主要覆盖 `issue` 相关命令、`repo view/log/branch view/stats`，`pr list/view/issues/comments/checkout/create/merge/reply/test`、`pr label/edit/close/ready`、`release list/view/create/delete/delete-asset/upload`、`commit view`、`commit comments (create/edit/list/list-by-sha/view)`、`commit diff/patch`、`label list/create/delete/edit`、`milestone list/view/create/delete`、`actions` 相关命令等。注意：自动推断现已覆盖写/破坏性命令（如 `pr merge`、`release delete`），缺省 `-R` 即作用于当前 Git 仓库。
- 仍需显式传目标仓库参数的命令，通常是语义上操作“另一个仓库”的命令，例如 `repo sync --target-repo` 这类显式目标仓库场景。


### Docker 使用

仓库提供了 Docker 镜像构建与运行支持：

```bash
make docker-build
make docker-run
docker compose up gc
```

认证 token 通过环境变量传入：`export GC_TOKEN=your_token && make docker-run`。
更多用法参见 [README.md](../README.md) 和 `docker-compose.yml`。

### Agent-Friendly CLI 能力

当前版本已开始收口面向 AI 代理和脚本的 CLI 契约：

- 只读命令和写路径结果支持 `--json`
- 删除类命令支持 `--dry-run`
- 非交互环境下删除命令未显式传 `--yes` 会直接失败，不再隐式等待输入
- 可通过 `gc schema` 查询命令树和单命令元数据（不含 `help` 和 `completion`）

当前已支持 `--json` 的只读命令：

- `auth status`
- `auth token`
- `commit comments list`
- `commit comments list-by-sha`
- `commit comments view`
- `commit view`
- `config get`
- `config list`
- `doctor install`
- `help`
- `issue comments`
- `issue list`
- `issue prs`
- `issue relations`
- `issue view`
- `label list`
- `milestone list`
- `milestone view`
- `pr comments`
- `pr diff`
- `pr issues`
- `pr list`
- `pr view`
- `precommit check`
- `release list`
- `release view`
- `repo branch view`
- `repo branch list`
- `repo branch-protection list`
- `repo pr-settings view`
- `actions run list`
- `actions run rerun`
- `actions run view`
- `actions run watch`
- `actions job list`
- `actions job view`
- `actions artifact list`
- `actions artifact view`
- `actions plugin list`
- `actions plugin view`
- `actions workflow list`
- `search repos`
- `search issues`
- `search users`
- `ssh-key list`
- `ssh-key view`
- `user view`
- `repo list`
- `repo log`
- `repo stats`
- `repo view`
- `version`
- `update --check`

当前已支持 `--json` 的写路径命令：

- `issue close`
- `issue comment`
- `issue create`
- `issue edit`
- `issue label`
- `issue reopen`
- `label create`
- `label delete`
- `label edit`
- `milestone create`
- `milestone edit`
- `pr close`
- `pr comment`
- `pr create`
- `pr edit`
- `pr label`
- `pr merge`
- `pr ready`
- `pr reopen`
- `pr review`
- `pr sync`
- `release create`
- `release edit`
- `release upload`
- `actions workflow run`
- `repo branch create`
- `repo edit`
- `repo pr-settings update`
- `ssh-key add`
- `ssh-key delete`
- `user edit`
- `repo create`
- `repo archive`
- `repo delete`
- `repo unarchive`
- `repo fork`
- `repo sync`
- `config set`
- `update`

其中 `issue list` 额外支持：

- `--format json|simple|table`
- `--time-format absolute|relative`
- `--template <go-template>`
- `--json` 与 `--format json` 等价，二者都应作为稳定机器可消费入口

`issue list` 的 `--format` 非法值应直接报用法错误，不应静默回退到默认格式。

`gc api` 可作为底层 API 调试入口，适合在 typed command 尚未覆盖时读取或调用 GitCode API；它输出远端原始响应，不额外包装 JSON。

### 退出码

`gc` 命令使用稳定的退出码语义，方便脚本和 AI 代理判断执行结果：

| 退出码 | 常量名 | 含义 | 典型场景 |
|--------|--------|------|----------|
| 0 | ExitSuccess | 命令执行成功 | 正常完成 |
| 1 | ExitError | 通用错误 | API 错误、网络错误 |
| 2 | ExitUsage | 参数用法错误 | 缺少必选参数、参数格式错误、非交互模式未传 `--yes` |
| 3 | ExitNotFound | 资源不存在 | issue/pr/repo 不存在 |
| 4 | ExitAuth | 认证错误 | 未登录或 token 无效 |
| 5 | ExitConflict | 资源冲突 | PR merge 冲突 |

### 全局 flag

| Flag | 说明 |
|------|------|
| `--no-interactive` | 禁用交互式提示（供 AI 代理/脚本/管道使用）。设置后所有确认提示立即失败并提示 `--yes`，等价于强制非 TTY 的确认行为。破坏性命令仍需显式传 `--yes` 才能执行。 |
| `--no-update-check` | 仅本次调用禁用 npm 自动更新检查；不修改持久化配置。 |

示例：
```bash
# 非交互模式：破坏性命令必须显式 --yes，否则退出码 2
gc --no-interactive pr merge 11 -R owner/repo          # 退出码 2，提示 rerun with --yes
gc --no-interactive --yes pr merge 11 -R owner/repo    # 执行合并

# 检查退出码
gc issue view abc  # 无效 issue 号
echo $?            # 输出 2 (ExitUsage)

gc issue view 999999  # 不存在的 issue
echo $?               # 输出 3 (ExitNotFound)

gc auth status  # 未认证时
echo $?         # 输出 4 (ExitAuth)
```

### 认证

```bash
# 方式一：设置环境变量（推荐，CI 从平台 Secret 注入；本地开发可写入 shell 配置）
# 值须来自 secret manager，勿在命令行或配置文件中硬编码真实 token
export GC_TOKEN="$GC_TOKEN_FROM_SECRETS"

# 永久生效（本地开发），添加到 shell 配置
echo 'export GC_TOKEN="$GC_TOKEN_FROM_SECRETS"' >> ~/.bashrc
source ~/.bashrc

# 方式二：非交互登录（从 secret manager 管道读入；禁止 echo/cat 明文 token，
# 会写入 shell history 或明文落盘）
<print-token-from-secret-manager> | gc auth login --with-token
```

### 测试仓库

本文档使用以下测试仓库：
- `infra-test/gctest1`

---

## 认证命令 (auth)

### auth login - 登录

```bash
# 交互式登录（推荐；在私有、未录制的终端执行）
gc auth login

# 从 stdin 读取 Token 登录（从 secret manager 管道读入；禁止 echo/cat 明文 token，
# 会写入 shell history 或明文落盘）
<print-token-from-secret-manager> | gc auth login --with-token

# 打开浏览器生成 Token 后继续登录
gc auth login --web
```

说明：
- 认证优先级：交互式登录或 `--web`（私有终端）> `GC_TOKEN` 环境变量（CI 从平台 Secret 注入）> 本地配置。禁止把 token 字面量放命令行（写入 shell history/进程列表）或明文文件；`--with-token` 须从 secret manager 管道读入。
- `auth login --web` 仅支持默认主机 `gitcode.com`，会打开 `https://gitcode.com/setting/token-classic/create` 新建访问令牌页面，然后继续在终端中读取你粘贴的 Token 完成登录。
- `--web` 与 `--with-token` 不能同时使用；两者分别代表浏览器辅助交互登录和从标准输入读取 Token。
- 自定义主机不支持 `--web`；请先核对目标主机，再使用 `auth login --hostname <host>` 在本地交互终端登录。
- 登录成功后 token 会写入本地配置；若同时设置了 `GC_TOKEN` 或 `GITCODE_TOKEN`，环境变量优先。
- 未显式传 `--with-token` 时需要交互式 TTY；非交互环境会直接报错，避免命令挂起等待输入。

### auth status - 查看认证状态

```bash
gc auth status

# 查看指定主机的持久化认证状态
gc auth status --hostname gitcode.com

# 显示完整 token（人工确认后临时查看）
gc auth status --show-token

# 输出 JSON
gc auth status --json
```

输出示例：
```
gitcode.com
  ✓ Logged in as username (GC_TOKEN)
  ✓ Git operations protocol: https
```

未认证或 token 无效/过期时，诊断信息输出到 stderr，退出码为 `4`（`ExitAuth`）。`--json` 模式下 JSON 仍写入 stdout（`logged_in:false` 或 `token_valid:false`），退出码同样为 `4`，便于 AI 代理仅凭退出码判断认证状态。

### auth token - 显示 Token

```bash
gc auth token

# 输出指定主机的已存储 token
gc auth token --hostname gitcode.com

# 输出 JSON
gc auth token --json
```

说明：
- `auth token` 输出当前实际生效的 token，解析顺序与 `auth status` 一致。
- 显式传 `--hostname` 时，会读取该主机已存储的 token，不再被通用环境变量覆盖。
- token 输出到 stdout，同时向 stderr 输出安全警告提醒不要共享此输出；禁止把该命令作为脚本 piping、日志采集或 AI 代理取 token 入口。
- `auth token` 和 `auth status --show-token` 必须在交互式 TTY 中按提示输入 hostname 确认；非交互环境一律拒绝输出完整 token，没有 `--yes` 绕过。

### auth logout - 登出

```bash
gc auth logout

# 非交互执行
gc auth logout --yes
```

说明：
- `auth logout` 会清理本地持久化认证信息。
- 若当前认证来自 `GC_TOKEN` 或 `GITCODE_TOKEN`，命令会提示你手动 `unset` 环境变量。
- 非交互场景中显式传 `--yes`。

### auth setup-git - 配置 git credential helper

```bash
# 为默认主机配置 git credential helper
gc auth setup-git

# 为指定主机配置
gc auth setup-git --hostname gitcode.com
```

说明：
- `auth setup-git` 将 gc 配置为 git 的 credential helper，使 git 操作（clone/pull/push）自动携带 GitCode 认证。
- 以 `--add` 追加方式写入 `git config --global credential."https://<host>".helper`，不覆盖已有 credential helper 配置。
- 幂等：重复运行不产生重复条目，已配置相同 helper 时直接跳过。
- 配置的 helper 指向 `gc auth git-credential`（credential 协议响应子命令，后续交付）；当前 setup-git 仅完成配置写入。
- `--hostname` 支持自定义主机，默认使用配置的默认主机（`gitcode.com`）。
- 纯客户端命令，不调用 GitCode API，不读取或输出 token。

---

## API 命令 (api)

### api - 调用 GitCode API

```bash
# 读取仓库 API 原始响应
gc api repos/infra-test/gctest1

# 读取 PR 文件列表
gc api repos/infra-test/gctest1/pulls/1/files

# 带查询参数的 API，包含 & 时请整体加引号
gc api 'repos/infra-test/gctest1/commits?path=README.md&sha=main'

# 指定 HTTP 方法和请求体文件
gc api repos/infra-test/gctest1/pulls/1 --method PATCH --input body.json

# 从 stdin 读取请求体
printf '{"title":"New title"}' | gc api repos/infra-test/gctest1/pulls/1 --method PATCH --input -

# 自定义请求头
gc api repos/infra-test/gctest1 --header 'Accept: application/json'
```

说明：
- endpoint 可写成 `repos/owner/repo` 或 `/api/v5/repos/owner/repo`；普通相对路径会自动补齐 `/api/v5/`。
- 认证沿用当前 `gc` 登录态或 `GC_TOKEN` / `GITCODE_TOKEN` 环境变量。
- 默认方法为 `GET`；传入 `--input` 但未指定 `--method` 时会自动使用 `POST`。
- 输出为远端原始响应 body，便于脚本继续交给 `python -m json.tool`、`jq` 或其他工具处理。

---

## 仓库命令 (repo)

### repo view - 查看仓库

```bash
# 查看仓库详情
gc repo view infra-test/gctest1
gc repo view

# 在浏览器中打开
gc repo view infra-test/gctest1 --web

# 输出 JSON
gc repo view infra-test/gctest1 --json
```

说明：
- 在当前 Git 仓库中执行时，`gc repo view` 可缺省仓库参数；CLI 会优先解析 `origin` remote，若不存在则回退到第一个 remote。

### repo branch view - 查看分支

```bash
# 查看分支详情
gc repo branch view main -R owner/repo

# 查看分支详情（当前仓库）
gc repo branch view main

# 输出 JSON
gc repo branch view main -R owner/repo --json
```

说明：
- `repo branch view` 显示指定分支的名称、保护状态和最新 commit 信息（ID、短 ID、标题、作者）。
- `--json` 输出分支对象，包含 `name`、`protected`、`commit.id`、`commit.short_id`、`commit.title`、`commit.message`、`commit.author.login`、`commit.committer.login`、`commit.created_at` 等字段。
- 分支不存在时返回明确错误。

### repo branch list - 列出分支

列出仓库中的所有分支。

```bash
# 列出分支
gc repo branch list -R owner/repo

# JSON 输出
gc repo branch list -R owner/repo --json
```

说明：
- `--json` 输出分支数组到 stdout。
- 默认分支标记 `(default)`。
- 退出码：`0` 成功；`1` 通用错误；`3` 仓库不存在（HTTP 404）。

### repo branch create - 创建分支

从指定引用（分支或 tag）创建新分支。默认从仓库默认分支创建。

```bash
# 从默认分支创建
gc repo branch create feature -R owner/repo

# 从指定 ref 创建
gc repo branch create feature --ref develop -R owner/repo

# 带描述
gc repo branch create feature --description "feature branch" -R owner/repo
```

说明：
- `--ref` 指定起点（分支名或 tag 名），默认为仓库默认分支。
- `--description` 设置分支描述。
- `--json` 输出创建后的分支对象。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 仓库不存在。

### repo branch-protection list - 列出保护分支规则

列出仓库的全部分支保护规则。

```bash
gc repo branch-protection list -R owner/repo

# JSON 输出
gc repo branch-protection list -R owner/repo --json
```

说明：
- `--json` 输出保护规则数组到 stdout。
- 退出码：`0` 成功；`1` 通用错误；`3` 仓库不存在（HTTP 404）；`4` 认证错误。

### repo branch-protection create - 创建保护分支规则

为分支或分支通配符创建保护规则。

```bash
gc repo branch-protection create --wildcard "main" --pusher admin --merger admin -R owner/repo
gc repo branch-protection create --wildcard "release/*" --pusher "admin;user1" -R owner/repo
```

说明：
- `--wildcard`（必填）：分支名或通配符（如 `main`、`release/*`）。
- `--pusher`：可推送角色/用户名（分号分隔）。角色：admin/develop/maintainer。
- `--merger`：可合并角色/用户名（同上）。
- `--pusher`/`--merger` 内容经 secret 扫描。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误（缺 wildcard）；`3` 仓库不存在；`4` 认证错误。

### repo branch-protection update - 更新保护分支规则

更新已存在的分支保护规则。

```bash
gc repo branch-protection update "main" --pusher develop -R owner/repo
```

说明：
- `<wildcard>`（必填位置参数）：要更新的分支通配符。
- `--pusher`/`--merger`：至少提供一个。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 仓库不存在；`4` 认证错误。

### repo branch-protection delete - 删除保护分支规则

删除已存在的分支保护规则。

```bash
gc repo branch-protection delete "main" -R owner/repo
```

说明：
- `<wildcard>`（必填位置参数）：要删除的分支通配符。
- `--yes`/`-y` 跳过确认（非 TTY 环境必须传 `--yes`）。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 规则不存在（HTTP 404）；`4` 认证错误。

### repo pr-settings view - 查看 PR 门禁设置

查看仓库的 Pull Request 合并门禁规则设置。

```bash
gc repo pr-settings view -R owner/repo

# JSON 输出
gc repo pr-settings view -R owner/repo --json
```

说明：
- 显示合并门禁（流水线必过/禁止自合入/强制合入/合并模式/删除源分支）、评审人（最小评审数/批准者/测试人）、Squash 设置、CLA 校验、轻量级 PR 等主要设置。`--json` 输出完整 24 项设置对象。
- `--json` 输出完整设置对象到 stdout。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 仓库不存在（HTTP 404）；`4` 认证错误。

### repo pr-settings update - 更新 PR 门禁设置

更新仓库的 PR 合并门禁规则。仅更新提供的字段。

```bash
# 要求流水线成功才能合并
gc repo pr-settings update --pipeline-required -R owner/repo

# 设置最小评审人数为 2
gc repo pr-settings update --reviewers 2 -R owner/repo

# 禁止合入自己的 PR
gc repo pr-settings update --no-self-merge -R owner/repo

# 设置合并模式为 fast-forward
gc repo pr-settings update --merge-method ff -R owner/repo

# JSON 输出
gc repo pr-settings update --pipeline-required -R owner/repo --json
```

说明：
- 支持 flags：`--reviewers`（最小评审人数，0 禁用）、`--pipeline-required`、`--no-self-merge`、`--force-merge`、`--delete-source-branch`、`--merge-method`（merge/rebase_merge/ff）。
- 至少提供一个字段。
- `--json` 输出更新后的完整设置对象。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 仓库不存在；`4` 认证错误。

### repo list - 列出仓库

```bash
# 列出自己的仓库
gc repo list

# 列出指定组织的仓库
gc repo list --owner infra-test

# 限制数量
gc repo list --limit 10

# 控制 API page size
gc repo list --per-page 50

# 只列出公开仓库
gc repo list --visibility public

# 输出 JSON
gc repo list --json

# 结构化输出（与 --json 等价）
gc repo list --format json

# 简洁输出
gc repo list --format simple

# 表格输出
gc repo list --format table
```

### repo log - 查看仓库提交日志

```bash
# 查看最近提交
gc repo log -R infra-test/gctest1
gc repo log

# 查看指定分支上触碰某个文件的提交
gc repo log -R infra-test/gctest1 --file README.md --branch main

# 限制数量并输出 JSON
gc repo log -R infra-test/gctest1 --file README.md --branch main --limit 5 --json

# 控制 API page size
gc repo log -R infra-test/gctest1 --per-page 50 --json
```

说明：
- `repo log` 支持 `-R/--repo`，也支持在当前 Git 仓库中缺省 `-R` 自动推断目标仓库。
- `--file` 对应提交 API 的文件路径过滤，`--branch` 可传分支、tag 或 commit SHA。
- `-L/--limit`（默认 30，客户端结果上限）与 `--per-page`（API page size，未指定时默认 `--limit`）；同时指定时本地按 `--limit` 截断。
- 文本输出会显示短 SHA、提交日期和提交信息首行；`--json` 输出远端提交对象数组。

### repo sync - 同步目录到目标仓库并创建 PR

```bash
# 将当前仓库 docs/api 同步到目标仓库的 mirror/api 目录
gc repo sync \
  --target-repo infra-test/target-repo \
  --source-dir docs/api \
  --target-dir mirror/api

# 指定 base 分支和 PR 标题
gc repo sync \
  --target-repo infra-test/target-repo \
  --source-dir pkg/contracts \
  --target-dir mirror/contracts \
  --base main \
  --title "sync: update contracts"

# 结构化输出
gc repo sync \
  --target-repo infra-test/target-repo \
  --source-dir docs/api \
  --target-dir mirror/api \
  --yes \
  --json
```

说明：
- 该命令必须在本地 Git 仓库内执行
- `--source-dir` 是当前仓库内要同步的目录
- `--target-dir` 是目标仓库中的子目录，不能是仓库根目录
- 命令会自动创建同步分支、提交、推送并创建目标 PR
- `repo sync` clones and pushes the target repository over SSH; ensure an SSH key with access to `git@gitcode.com` is configured.
- 推送同步分支并创建目标 PR 前默认需要确认；非交互场景中显式传 `--yes`
- 如果目标目录内容与源目录一致，命令会直接返回“无变更”

### repo edit - 更新仓库设置

更新 GitCode 仓库的设置。仅更新提供的字段。

```bash
# 更新描述
gc repo edit --description "New description" -R owner/repo

# 设为私有
gc repo edit --private -R owner/repo

# 设为公开
gc repo edit --public -R owner/repo

# 更新默认分支
gc repo edit --default-branch main -R owner/repo

# 重命名仓库
gc repo edit --name new-name -R owner/repo

# JSON 输出
gc repo edit --description "test" -R owner/repo --json
```

说明：

- 支持 flags：`--description`、`--homepage`、`--default-branch`、`--name`、`--private`、`--public`。
- `--private` 和 `--public` 互斥，不能同时使用。
- 至少提供一个字段，否则报参数错误。
- `--description` 内容经 secret 扫描。
- `--json` 输出更新后的仓库对象。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 仓库不存在（HTTP 404）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409，如重命名为已存在的名称）。

### repo create - 创建仓库

```bash
# 创建公开仓库
gc repo create my-repo --public

# 创建私有仓库
gc repo create my-repo --private

# 创建带描述的仓库
gc repo create my-repo --public --description "My project"

# 创建后输出 JSON
gc repo create my-repo --private --json
```

> **注意**: 在组织下创建仓库需要有组织的相应权限。
> `--json` 只在成功创建后输出仓库对象；不会混入文本提示。

### repo fork - Fork 仓库

```bash
# Fork 仓库到自己的账户
gc repo fork owner/repo

# Fork 并克隆到本地
gc repo fork owner/repo --clone

# Fork 后输出 JSON
gc repo fork owner/repo --json
```

说明：
- `repo fork` 现在会按传入的 `owner/repo` 执行 fork，不再使用硬编码仓库路径。
- `--clone` 会在 fork 成功后将 fork 出来的仓库克隆到当前目录。
- `--json` 只在 fork 成功后输出 fork 仓库对象；不能与 `--clone` 同时使用。

### repo clone - 克隆仓库

```bash
# 克隆仓库到当前目录
gc repo clone owner/repo

# 克隆到指定目录
gc repo clone owner/repo my-project

# 使用 SSH 克隆
gc repo clone owner/repo --git-protocol ssh

# 使用 HTTPS 克隆
gc repo clone owner/repo --git-protocol https

# 浅克隆
gc repo clone owner/repo --depth 1

# 克隆并切换到指定分支
gc repo clone owner/repo --branch develop

# 递归克隆子模块
gc repo clone owner/repo --recursive
```

说明：
- 仓库可以指定为 `owner/repo` 格式或完整 URL（如 `https://gitcode.com/owner/repo`）。
- 默认使用 SSH 协议（`git@gitcode.com:owner/repo.git`），除非 `--git-protocol https` 或已保存配置显式选择 HTTPS。
- SSH 克隆需要本地 SSH key 有权限访问 `git@gitcode.com`。
- `--depth` 创建浅克隆，减少下载量。
- `--recursive` 克隆后自动初始化并更新子模块。

### repo delete - 删除仓库

```bash
# 删除仓库（危险操作，需确认）
gc repo delete owner/repo

# 预演删除
gc repo delete owner/repo --dry-run

# 非交互执行
gc repo delete owner/repo --yes

# 输出 JSON
gc repo delete owner/repo --yes --json
```

说明：
- 默认会要求输入仓库名确认。
- 在非交互环境中，未显式传 `--yes` 会直接失败。

### repo archive - 归档仓库

```bash
# 归档仓库（危险操作，需确认）
gc repo archive owner/repo

# 非交互执行
gc repo archive owner/repo --yes

# 预演归档
gc repo archive owner/repo --dry-run

# 输出 JSON
gc repo archive owner/repo --yes --json
```

说明：
- 归档后仓库变为只读；可用 `gc repo unarchive` 恢复。
- 默认要求输入仓库名确认；非交互环境需 `--yes`。

### repo unarchive - 取消归档仓库

```bash
# 取消归档
gc repo unarchive owner/repo --yes

# 预演
gc repo unarchive owner/repo --dry-run

# 输出 JSON
gc repo unarchive owner/repo --yes --json
```

说明：
- 恢复归档仓库的写权限。
- 默认要求输入仓库名确认；非交互环境需 `--yes`。

### repo stats - 代码贡献统计

```bash
# 获取 main 分支代码贡献统计
gc repo stats --branch main -R infra-test/gctest1

# 按作者筛选
gc repo stats --branch main --author username -R infra-test/gctest1

# 仅显示个人统计
gc repo stats --branch main --only-self -R infra-test/gctest1

# 指定日期范围
gc repo stats --branch main --since 2024-01-01 --until 2024-12-31 -R infra-test/gctest1

# JSON 输出
gc repo stats --branch main -R infra-test/gctest1 --json
```

### repo set-default - 设置默认仓库

设置或查看当前 host 的默认仓库。其他命令在未提供 `-R/--repo` 且不在 git 仓库目录中时使用此值。纯客户端配置，无需 API。

```bash
# 从当前 git remote 设置默认仓库
gc repo set-default

# 显式指定默认仓库
gc repo set-default owner/repo

# 查看当前默认仓库
gc repo set-default --view

# JSON 输出
gc repo set-default --view --json

# 清除默认仓库
gc repo set-default --unset
```

说明：

- 无参数时从 git remote 推断仓库；传 `<owner>/<repo>` 显式指定。
- `--view` 显示当前默认仓库；`--json` 输出 `{"default_repo": "owner/repo"}`。
- `--unset` 清除默认仓库。
- 配置存储在 `~/.config/gc/config.json` 的 `default_repo` key（按 host 维度）。环境变量 `GC_DEFAULT_REPO` 覆盖配置。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误（如非法仓库格式或无 git 上下文）。

---

## Issue 命令 (issue)

### issue create - 创建 Issue

```bash
# 创建 Issue
gc issue create -R infra-test/gctest1 --title "Bug: Something wrong" --body "Description here"
gc issue create --title "Bug: Something wrong" --body "Description here"

# 从文件读取 body
gc issue create -R infra-test/gctest1 --title "Bug report" --body-file description.md

# 从 stdin 读取 body
echo "Description from stdin" | gc issue create -R infra-test/gctest1 --title "Bug report" --body-file -

# 创建 Issue 并添加标签
gc issue create -R infra-test/gctest1 --title "Feature request" --body "Description" --label enhancement

# 指定受理人
gc issue create -R infra-test/gctest1 --title "Task" --body "Description" --assignee username

# 使用模板路径创建
gc issue create -R infra-test/gctest1 --title "Feature request" --template-path .gitcode/ISSUE_TEMPLATE/feature.yaml

# 创建私有 Issue
gc issue create -R infra-test/gctest1 --title "Security report" --security-hole

# 传入高级字段（企业版）
gc issue create -R infra-test/gctest1 --title "Feature request" --issue-type "需求" --issue-severity "高"

# 通过 JSON 传入 custom_fields
gc issue create -R infra-test/gctest1 --title "Feature request" --custom-fields-json '[{"id":"field","value":"demo"}]'

# 从文件读取 custom_fields
gc issue create -R infra-test/gctest1 --title "Feature request" --custom-fields-file custom-fields.json

# 预演创建
gc issue create -R infra-test/gctest1 --title "Task" --body "Description" --dry-run

# 预演创建并输出 JSON
gc issue create -R infra-test/gctest1 --title "Task" --body "Description" --dry-run --json

# 创建后输出 JSON
gc issue create -R infra-test/gctest1 --title "Task" --body "Description" --json
```

说明：
- `issue create` 当前已支持 `--dry-run` 预演创建参数。
- `issue create --dry-run --json` 输出结构化预览，不执行真实创建。
- `--json` 只在成功创建并完成必要回读验证后输出 issue 对象；不会混入文本提示。
- 不含负责人和高级字段时，创建会继续走兼容的 repo 级 form 提交路径。
- `--assignee` 直接按 GitCode 文档要求提交用户名；多个用户名以英文逗号组合，并切换到 owner 级创建接口。
- 显式传入 `--template-path`、`--security-hole`、`--issue-type`、`--issue-severity`、`--custom-fields-json`、`--custom-fields-file` 时，会切换到 GitCode 文档化的 owner 级创建接口并透传高级字段。
- `--custom-fields-json` 与 `--custom-fields-file` 不能同时使用；两者都要求 JSON 顶层是 `object[]`。
- 模板路径支持仓库下的 `.gitcode`、`.github`、`.gitee` 目录；组织模板可能来自 `owner/.gitcode`，第一阶段仅支持显式传路径，不保证自动发现。
- 若 GitCode API 已创建 issue 但未实际应用 assignee，命令会返回失败并包含已创建 issue 的 URL，避免静默误判；自动化调用方不应盲目重试创建。
- 若负责人回读请求失败，命令同样返回包含已创建 issue URL 的验证错误；写操作可能已完成，调用方应先回读而不是直接重试创建。
- Windows PowerShell 中从 stdin 传中文正文时，建议使用 UTF-8 文件或先设置 `$OutputEncoding`，详见“Windows PowerShell 命令名和 stdin”。

### issue list - 列出 Issues

```bash
# 列出所有开放的 Issues
gc issue list -R infra-test/gctest1
gc issue list

# 只列出已关闭的 Issues
gc issue list -R infra-test/gctest1 --state closed

# 列出所有状态的 Issues
gc issue list -R infra-test/gctest1 --state all

# 按标签筛选
gc issue list -R infra-test/gctest1 --label bug,enhancement

# 限制数量
gc issue list -R infra-test/gctest1 --limit 20

# 按里程碑完整标题筛选
gc issue list -R infra-test/gctest1 --milestone "v1.0"

# 按里程碑编号筛选
gc issue list -R infra-test/gctest1 --milestone 123

# 按受理人筛选
gc issue list -R infra-test/gctest1 --assignee username

# 按创建者筛选
gc issue list -R infra-test/gctest1 --creator username

# 按更新时间排序
gc issue list -R infra-test/gctest1 --sort updated --direction desc

# 按创建时间筛选
gc issue list -R infra-test/gctest1 --created-after "2024-01-01"
gc issue list -R infra-test/gctest1 --created-before "2024-12-31"

# 按更新时间筛选
gc issue list -R infra-test/gctest1 --updated-after "2024-01-01"

# 关键字搜索
gc issue list -R infra-test/gctest1 --search "bug"

# 组合使用
gc issue list -R infra-test/gctest1 --state open --milestone "v1.0" --sort updated

# 输出 JSON
gc issue list -R infra-test/gctest1 --json

# 输出格式
gc issue list -R infra-test/gctest1 --format json
gc issue list -R infra-test/gctest1 --format simple
gc issue list -R infra-test/gctest1 --format table

# 时间格式
gc issue list -R infra-test/gctest1 --time-format absolute
gc issue list -R infra-test/gctest1 --time-format relative

# 自定义模板
gc issue list -R infra-test/gctest1 --template '{{range .}}#{{.Number}} {{.Title}}{{"\n"}}{{end}}'
```

说明：
- `--json` 继续作为兼容入口保留，适合脚本和代理调用。
- `--milestone` 接受完整里程碑标题或正的 32 位十进制编号；纯 ASCII 数字输入始终按编号解析，允许前导零，`0` 和溢出值会返回参数错误。
- 带符号或其他非数字字符的值按完整标题处理；不支持标题缩写或模糊匹配，因此纯数字标题无法通过该参数与编号消歧。
- `--format json` 与 `--json` 输出一致。
- `--time-format` 只影响文本展示中的时间格式，不改变 JSON 结构。
- `--template` 使用 Go template 渲染 issue 列表，当前与 `--json`、`--format` 互斥。
- 非法 `--format` 值会返回错误，不会静默降级为默认输出。
- `--since`、`--created-after`、`--created-before`、`--updated-after`、`--updated-before` 支持 `YYYY-MM-DD` 和 ISO 8601 时间。
- CLI 会在请求前自动规范化为 GitCode API 可接受的 RFC3339 时间戳。
- `-L/--limit`（默认 30，客户端结果上限）与 `--per-page`（API page size，未指定时默认 `--limit`）；同时指定时本地按 `--limit` 截断。`--page` 翻页。

### issue view - 查看 Issue

```bash
# 查看 Issue 详情
gc issue view 1 -R infra-test/gctest1
gc issue view 1

# 查看所有评论（自动翻页获取全部）
gc issue view 1 -R infra-test/gctest1 --comments

# 在浏览器中打开
gc issue view 1 -R infra-test/gctest1 --web

# 输出 JSON
gc issue view 1 -R infra-test/gctest1 --json

# 查看评论并输出 JSON
gc issue view 1 -R infra-test/gctest1 --comments --json

# 相对时间
gc issue view 1 -R infra-test/gctest1 --time-format relative
```

说明：
- `issue view` 的文本详情输出会使用更稳定的元信息排布，便于人工和代理阅读。
- `--time-format absolute|relative` 只影响文本详情和评论区中的时间展示，不改变 `--json` 结构。
- `--json` 路径保持结构化输出，不受文本排版变化影响。

### issue close - 关闭 Issue

```bash
# 关闭 Issue
gc issue close 1 -R infra-test/gctest1
gc issue close 1

# 非交互执行
gc issue close 1 -R infra-test/gctest1 --yes

# 输出 JSON
gc issue close 1 -R infra-test/gctest1 --yes --json
```

说明：
- `-R` 在当前 Git 仓库目录执行时可省略，命令会自动推断目标仓库。
- `issue close` 属于写操作，默认需要确认；非交互场景中显式传 `--yes`。
- 命令会在关闭请求后验证 Issue 状态，避免服务端未实际关闭时误报成功。

### issue edit - 编辑 Issue

```bash
# 修改标题
gc issue edit 1 --title "New title" -R infra-test/gctest1
gc issue edit 1 --title "New title"

# 修改描述
gc issue edit 1 --body "New description" -R infra-test/gctest1

# 从文件读取新描述
gc issue edit 1 --body-file new-description.md -R infra-test/gctest1

# 修改状态（close/reopen）
gc issue edit 1 --state close -R infra-test/gctest1
gc issue edit 1 --state reopen -R infra-test/gctest1

# 指派负责人
gc issue edit 1 --assignee username -R infra-test/gctest1
gc issue edit 1 --assignee user1 --assignee user2 -R infra-test/gctest1

# 设置标签
gc issue edit 1 --label bug,enhancement -R infra-test/gctest1

# 设置里程碑
gc issue edit 1 --milestone 5 -R infra-test/gctest1

# 设置为私有 Issue
gc issue edit 1 --security-hole -R infra-test/gctest1

# 组合使用
gc issue edit 1 --title "Bug fix" --assignee username --label bug --milestone 1 -R infra-test/gctest1

# 更新后输出 JSON
gc issue edit 1 --title "Bug fix" -R infra-test/gctest1 --json
```

说明：
- `issue edit --assignee` 按 GitCode 文档要求直接提交用户名；多个用户名以英文逗号组合。
- 若 GitCode API 未实际应用 assignee，命令会返回失败并包含已更新 issue 的 URL，避免自动化流程静默误判。
- 若负责人回读请求失败，命令返回包含已更新 issue URL 的验证错误；调用方可据此重新查询远端状态。
- `--json` 只在成功更新并完成必要回读验证后输出 issue 对象；不会混入文本提示。

### issue reopen - 重开 Issue

```bash
# 重开 Issue
gc issue reopen 1 -R infra-test/gctest1
gc issue reopen 1

# 非交互执行
gc issue reopen 1 -R infra-test/gctest1 --yes

# 输出 JSON
gc issue reopen 1 -R infra-test/gctest1 --yes --json
```

说明：
- `-R` 在当前 Git 仓库目录执行时可省略，命令会自动推断目标仓库。
- `issue reopen` 属于写操作，默认需要确认；非交互场景中显式传 `--yes`。
- 命令会在重开请求后验证 Issue 状态。

### issue comment - 添加评论

```bash
# 添加评论
gc issue comment 1 -R infra-test/gctest1 --body "This is a comment"
gc issue comment 1 --body "This is a comment"

# 从文件读取评论内容
gc issue comment 1 -R infra-test/gctest1 --body-file comment.txt

# 从 stdin 读取评论内容
echo "Comment from stdin" | gc issue comment 1 -R infra-test/gctest1 --body-file -

# 输出 JSON
gc issue comment 1 -R infra-test/gctest1 --body "This is a comment" --json
```

### issue comment edit - 编辑 Issue 评论

```bash
# 按参数编辑评论
gc issue comment edit 166061383 -R infra-test/gctest1 --body "Updated comment"

# 按 --id 编辑评论
gc issue comment edit --id 166061383 -R infra-test/gctest1 --body "Updated comment"

# 从文件读取新内容
gc issue comment edit 166061383 -R infra-test/gctest1 --body-file comment.md
```

### issue comments - 列出 Issue 评论

```bash
# 列出评论
gc issue comments 1 -R infra-test/gctest1
gc issue comments 1

# 查看所有评论（--limit 0 或不指定 --limit，自动翻页获取全部）
gc issue comments 1 -R infra-test/gctest1
gc issue comments 1 -R infra-test/gctest1 --limit 0

# 限制返回数量
gc issue comments 1 -R infra-test/gctest1 --limit 10

# 倒序排列
gc issue comments 1 -R infra-test/gctest1 --order desc

# 按更新时间筛选
gc issue comments 1 -R infra-test/gctest1 --since "2024-01-01T00:00:00+08:00"

# JSON 输出
gc issue comments 1 -R infra-test/gctest1 --json
```

### issue label - 管理 Issue 标签

```bash
# 添加标签
gc issue label 1 --add bug,enhancement -R infra-test/gctest1
gc issue label 1 --add bug,enhancement

# 移除标签
gc issue label 1 --remove bug -R infra-test/gctest1

# 先移除旧标签再添加新标签（单次原子更新，支持一条命令完成标签替换）
gc issue label 1 --remove status/triage --add status/verified,status/in-progress -R infra-test/gctest1

# 列出标签
gc issue label 1 --list -R infra-test/gctest1

# 输出 JSON
gc issue label 1 --add bug -R infra-test/gctest1 --json
gc issue label 1 --remove status/triage --add status/verified -R infra-test/gctest1 --json
gc issue label 1 --list -R infra-test/gctest1 --json
```

说明：

- 同时指定 `--remove` 和 `--add` 时，命令读取当前标签后以单次更新完成「移除旧标签 + 添加新标签」，不存在只移除未添加的中间态；`--add` 输入先解析校验，无有效标签名时返回参数错误且不触发任何远端写操作。
- 组合操作的 JSON 结果 `action` 为 `update`，`label` 为被移除的标签，`labels` 为更新后的标签集合。

### issue prs - 查看 Issue 关联的 PRs

```bash
# 查看 Issue 关联的 Pull Requests
gc issue prs 123 -R infra-test/gctest1
gc issue prs 123

# 获取增强信息（包含可合并状态）
gc issue prs 123 --mode 1 -R infra-test/gctest1

# JSON 输出
gc issue prs 123 -R infra-test/gctest1 --json
```

说明：
- `issue create/list/view/close/reopen/comment/comments/edit/label/prs` 在当前 Git 仓库中可缺省 `-R`，CLI 会优先解析 `origin` remote；若没有 `origin`，则回退到第一个 remote。
- 若当前目录不是 Git 仓库，或仓库没有可用 remote，会返回明确错误并提示改用 `-R owner/repo`。
- `--mode 1` 显示 Mergeable 状态：绿色 `can merge` 表示可合并，红色 `cannot merge` 表示不可合并，黄色 `unknown` 表示未计算或未知。

### issue relations - 查看仓库内 Issue / PR 关联表

```bash
# 查看仓库内所有 issue / PR 关联关系
gc issue relations -R infra-test/gctest1

# 输出 JSON 关系行
gc issue relations -R infra-test/gctest1 --json

# 只扫描开放 issue
gc issue relations -R infra-test/gctest1 --state open --limit 50
```

说明：
- 该命令会遍历仓库 issue，并获取每个 issue 关联的 PR。
- 文本输出按 PR 聚合，并同时显示关联 issue 的状态信息。
- `--json` 输出为关系行数组，每一行包含 `pr` 和 `issue` 两部分。

---

## Discussions 命令 (discussions)

只读访问 GitCode 组织级讨论（discuss）。当前覆盖组织讨论列表与详情，调用 GitCode v5 API（`GET /api/v5/orgs/{org}/discuss` 与 `/discuss/{number}`）。

### discussions list - 列出组织讨论

```bash
# 列出组织讨论
gc discussions list --org my-org

# 按标题/描述搜索
gc discussions list --org my-org --search "release plan"

# 按评论数排序（降序）
gc discussions list --org my-org --sort comment_size --direction desc

# 分页
gc discussions list --org my-org --page 2 --per-page 50

# 输出 JSON
gc discussions list --org my-org --json
```

说明：
- `--org`（必填）：组织 path。
- `--page` / `--per-page`：分页（`--per-page` 最大 100，默认 20）。
- `-L/--limit`：客户端结果上限（默认 30），在 `--per-page` 取回后本地截断。
- `--sort`：`created`（默认，创建时间）或 `comment_size`（评论数量）。
- `--direction`：`asc` 或 `desc`（默认降序）。
- `--search`：按标题与描述服务端过滤。
- `--json`：原样输出讨论数组。

### discussions view - 查看组织讨论

```bash
# 查看讨论
gc discussions view 42 --org my-org

# 输出 JSON
gc discussions view 42 --org my-org --json
```

说明：
- `--org`（必填）：组织 path。位置参数为讨论编号。
- `--json`：原样输出讨论对象（含 `md_content` 正文）。

### discussions project list - 列出仓库讨论

仓库（项目级）讨论，调用 GitCode v5 API（`GET /api/v5/repos/{owner}/{repo}/discuss`），参数与组织讨论一致。

```bash
# 列出仓库讨论
gc discussions project list -R owner/repo

# 搜索 / 排序 / 分页
gc discussions project list -R owner/repo --search "release" --sort comment_size --direction desc --page 2 --per-page 50

# 输出 JSON
gc discussions project list -R owner/repo --json
```

说明：
- `-R`（必填）：仓库（owner/repo），在当前 Git 仓库目录执行时可省略自动推断。
- `--page`/`--per-page`/`--sort`/`--direction`/`--search`：与 `discussions list` 同义。
- `-L/--limit`：客户端结果上限（默认 30），与 `discussions list` 同义。
- `--json`：原样输出讨论数组。

### discussions project view - 查看仓库讨论

```bash
gc discussions project view 42 -R owner/repo
gc discussions project view 42 -R owner/repo --json
```

说明：
- `-R`（必填）：仓库。位置参数为讨论编号。
- `--json`：原样输出讨论对象（含 `md_content` 正文）。

### discussions comments list - 列出组织讨论评论

组织讨论评论，调用 `GET /api/v5/orgs/{org}/discuss/{number}/comment`。

```bash
gc discussions comments list 42 --org my-org
gc discussions comments list 42 --org my-org --order hot_desc --json
```

说明：`--org`（必填）；位置参数为讨论编号；`--order`（time_asc/time_desc/hot_desc）；`--page`/`--per-page`；`--json`。

### discussions comments replies - 列出组织讨论评论回复

```bash
gc discussions comments replies 42 <comment-id> --org my-org
```

说明：两个位置参数（讨论编号 + comment-id）；`--org`（必填）；`--page`/`--per-page`；`--json`。

### discussions project comments list - 列出仓库讨论评论

仓库讨论评论，调用 `GET /api/v5/repos/{owner}/{repo}/discuss/{number}/comment`。

```bash
gc discussions project comments list 42 -R owner/repo
gc discussions project comments list 42 -R owner/repo --order time_desc --json
```

说明：`-R` 仓库（owner/repo），在当前 Git 仓库目录执行时可省略自动推断；位置参数为讨论编号；`--order`/`--page`/`--per-page`/`--json`。

### discussions project comments replies - 列出仓库讨论评论回复

```bash
gc discussions project comments replies 42 <comment-id> -R owner/repo
```

说明：两个位置参数（讨论编号 + comment-id）；`-R` 仓库，可省略自动推断；`--page`/`--per-page`/`--json`。

评论与回复共享同一输出结构（`id`/`author`/`content`/`md_content`/`like_total`/`reply_total`/`created_at` 等）；404（讨论或评论不存在）返回 exit 3。

---

## Pull Request 命令 (pr)

### pr create - 创建 PR

```bash
# 创建 PR（自动检测当前分支作为 head）
gc pr create -R infra-test/gctest1 --title "New feature" --body "Description"

# 从文件读取 PR 内容
gc pr create -R infra-test/gctest1 --title "New feature" --body-file description.md

# 从 stdin 读取 PR 内容
echo "Description from stdin" | gc pr create -R infra-test/gctest1 --title "New feature" --body-file -

# 指定 head 分支
gc pr create -R infra-test/gctest1 --head feature-branch --title "Feature" --body "Description"

# 指定基础分支
gc pr create -R infra-test/gctest1 --base main --title "Feature" --body "Description"

# 创建草稿 PR
gc pr create -R infra-test/gctest1 --title "WIP: Feature" --draft

# 创建 PR 并打标签
gc pr create -R infra-test/gctest1 --title "New feature" --body "Description" --labels bug,enhancement

# 创建跨仓库 PR（从 fork 到 upstream）；--fork 会自动把 head 规范化为 myfork/repo:feature-branch
gc pr create -R upstream/repo --fork myfork/repo --head feature-branch --title "Feature"

# 等价写法：直接用 owner/repo:branch 形式的 head，可省略 --fork
gc pr create -R upstream/repo --head myfork/repo:feature-branch --title "Feature"

# 从最后一次提交填充标题和内容
gc pr create -R infra-test/gctest1 --fill

# 创建成功后在浏览器中打开 PR
gc pr create -R infra-test/gctest1 --title "New feature" --body "Description" --web

# 创建后输出 JSON
gc pr create -R infra-test/gctest1 --head feature-branch --title "Feature" --body "Description" --json
```

> **说明**: `--head` 参数可选，未指定时自动检测当前 Git 分支。
> `-l, --labels` 支持创建 PR 时直接打标签，值为逗号分隔的标签名（如 `bug,enhancement`），与 `gc pr edit --labels` 格式一致。不指定时不打标签。
> `--body-file` 支持从文件读取 PR 内容；使用 `-` 可从 stdin 读取。`--body` 与 `--body-file` 不能同时使用。
> `--fill` 会使用最近一次 Git commit 的标题和正文补全未显式提供的 `--title` / `--body` / `--body-file`。
> `--web` 会在 PR 创建成功后打开新建 PR 页面。
> `--json` 只在成功创建后输出 PR 对象；不能与 `--web` 同时使用。
> 如果 GitCode 创建响应未返回 `body`，CLI 会在创建后尝试回读 PR；若回读仍未返回或无法确认远端 body，`--json` 会保持远端返回的空值并在 stderr 给出 warning，避免把本地提交内容伪装成远端事实。可用 `gitcode pr view <number> -R owner/repo --json` 再次核验。
> **跨仓库 PR**: 跨仓库（从 fork 到 upstream）的源仓库通过 `head="<fork_owner>/<fork_repo>:<branch>"` 表达（GitCode v5 实测支持，见 #495；早期 `owner:branch` 形式已被服务端弃用），而不是已废弃的 `fork_path` 表单字段——后者在 GitCode v5 会错误解析源甚至 403。使用 `--fork owner/repo` 时，CLI 会自动把 `--head` 规范化为 `<fork_owner>/<fork_repo>:<branch>`；若 `--head` 已是 `owner/repo:branch` 形式则原样保留，可省略 `--fork`。
> 当前分支解析已统一接入 `Factory.Branch`；若当前目录不是 Git 仓库或无法识别分支，会明确提示改用 `--head`。
> PR body 中含 `Closes #NNN`（或 `Fixes #NNN`/`Resolves #NNN`）时，PR merge 后 GitCode 自动关闭关联 issue；非修复型 PR 使用 `Refs #NNN` 仅引用不关闭。commit message 中的 `Closes` 不被 GitCode 识别为自动关闭。

### pr list - 列出 PRs

```bash
# 列出所有开放的 PRs
gc pr list -R infra-test/gctest1

# 只列出已关闭的 PRs
gc pr list -R infra-test/gctest1 --state closed

# 只列出已合并的 PRs
gc pr list -R infra-test/gctest1 --state merged

# 按 head / base 分支过滤
gc pr list -R infra-test/gctest1 --head feature/login --base main

# 按里程碑筛选
gc pr list -R infra-test/gctest1 --milestone "v1.0"

# 限制数量
gc pr list -R infra-test/gctest1 --limit 10

# 排序与分页
gc pr list -R infra-test/gctest1 --sort updated --direction desc --page 2

# 自动翻页获取多页结果
gc pr list -R infra-test/gctest1 --paginate --per-page 100

# 按 PR 提交信息过滤
gc pr list -R infra-test/gctest1 --commit-message "fix login"

# 自动翻页后按提交信息过滤，仍可用 --limit 做结果上限
gc pr list -R infra-test/gctest1 --paginate --per-page 100 --limit 200 --commit-message "fix login" --json

# 输出 JSON
gc pr list -R infra-test/gctest1 --json

# 表格输出
gc pr list -R infra-test/gctest1 --format table
```

说明：
- `--paginate` 会从第一页开始连续读取多页结果，直到远端返回不足一页；不能与 `--page` 同时使用。
- `--per-page` 控制单页大小，未显式传 `--limit` 时默认每页 100；显式传 `--limit` 时会在本地截断到指定数量。
- `--commit-message` 会读取每个候选 PR 的提交列表并按提交信息子串匹配，适合从提交标题反查关联 PR。
- `--json` 输出结构化数组写入 stdout；空结果输出 `[]`（不会输出 `null`）。
- GitCode API 可能对已合并 PR 返回 `merged: null`；CLI 会根据 `state=merged` 或非空 `merged_at` 将 JSON 中的 `merged` 归一化为 `true`。

### pr view - 查看 PR

```bash
# 查看 PR 详情
gc pr view 1 -R infra-test/gctest1

# 查看评论
gc pr view 1 -R infra-test/gctest1 --comments

# 在浏览器中打开
gc pr view 1 -R infra-test/gctest1 --web

# 输出 JSON
gc pr view 1 -R infra-test/gctest1 --json

# 查看评论并输出 JSON
gc pr view 1 -R infra-test/gctest1 --comments --json

# 相对时间
gc pr view 1 -R infra-test/gctest1 --time-format relative
```

说明：
- `pr view` 的文本详情输出会使用更稳定的元信息排布，便于人工和代理阅读。
- 文本输出包含里程碑信息（如果 PR 关联了里程碑）。
- `--time-format absolute|relative` 只影响文本详情和评论区中的时间展示，不改变 `--json` 结构。
- 如果 PR 详情 API 返回的 `additions`、`deletions`、`changed_files` 或 `commits` 为 0，CLI 会尝试通过 PR files/commits API 补齐统计；补齐失败时会给出 warning，但不阻断查看。补齐失败时返回非零退出码（按底层 API 错误映射：401→4、404→3、409→5、其他→1），PR 详情仍输出到 stdout。多个补齐源同时失败时，退出码按首个失败 API 的状态码映射。
- `--json` 路径保持结构化输出，milestone、body、description、merged、merged_at 等字段会自动包含在 JSON 中；其中 `body` 与 `description` 会基于远端返回互相补齐。GitCode API 对已合并 PR 返回 `merged: null` 时，CLI 会根据 `state=merged` 或非空 `merged_at` 将 `merged` 归一化为 `true`。`--comments --json` 在评论获取失败时仍写入 `{"pull_request": ..., "comments": null}` 并返回非零退出码，与文本模式"渲染已有数据 + 信号不完整"的行为一致。

### pr issues - 查看 PR 关联的 Issues

```bash
# 查看 PR 关联的 Issues
gc pr issues 123 -R owner/repo

# 查看 PR 关联的 Issues（当前仓库）
gc pr issues 123

# 输出 JSON
gc pr issues 123 -R owner/repo --json
```

说明：
- `pr issues` 列出指定 PR 关联的 Issue 列表。
- `--json` 输出 Issue 对象数组，包含 `id`、`number`、`title`、`body`、`state`、`html_url`、`user`、`labels`、`milestone`、`created_at` 等字段；无关联 Issue 时输出 `[]`，不会混入文本提示。
- PR 不存在时返回明确错误；无关联 Issue 时文本输出提示"No linked issues found for PR #\<number\>"。

### pr comments - 查看 PR 评论

```bash
# 查看 PR 评论列表
gc pr comments 1 -R infra-test/gctest1

# 限制评论数量
gc pr comments 1 --limit 5 -R infra-test/gctest1

# JSON 输出
gc pr comments 1 -R infra-test/gctest1 --json
```

评论列表会显示 `Discussion ID`，可直接用于 `gc pr reply --discussion`。
`--json` 输出评论对象数组；无评论时输出 `[]`，不会混入文本提示。
可通过 `gc pr comment resolve` 和 `gc pr comment unresolve` 标记评论讨论的解决状态。
inline comment 会显示文件路径和所在行号（新版本文件的行号）。

### pr comment - 添加 PR 评论

```bash
# 添加普通评论
gc pr comment 123 --body "This looks good" -R owner/repo

# 从文件读取评论内容
gc pr comment 123 --body-file comment.txt -R owner/repo

# 从 stdin 读取评论内容
echo "Comment from stdin" | gc pr comment 123 --body-file - -R owner/repo

# 添加行内评论 - 先获取文件路径
gc pr diff 123 -R owner/repo                        # 查看变更文件获取文件路径
gc pr comment 123 --body "代码逻辑正确" --path api/auth.go --position 42 -R owner/repo

# 输出 JSON
gc pr comment 123 --body "This looks good" --json
```

添加评论到 PR。支持普通评论和行内评论（inline comment）。

**行内评论注意事项**：
- 需要同时提供 `--path`（文件路径）和 `--position`（新版本文件中的行号）
- 文件路径必须是 diff 中显示的实际文件名（如 `test-cross-pr.txt`）
- `--position` 是目标代码在**新版本文件**中的行号（diff 新增侧/右侧的行号），不是 diff hunk 内的偏移量；可用 `gc pr diff <number>` 查看新增侧行号
- 可通过 `gc pr diff <number>` 查看变更文件列表来获取文件路径
- 如果文件名错误，会返回错误：`diff failed to be generated due to invalid params under position param`

### pr comment edit - 编辑 PR 评论

```bash
# 编辑评论
gc pr comment edit 123 --body "Updated comment" -R owner/repo
```

编辑已存在的 PR 评论。评论 ID 可通过 `gc pr comments <number>` 查看。

### pr comment delete - 删除 PR 评论

```bash
# 删除评论
gc pr comment delete 123 -R owner/repo

# 跳过确认直接删除
gc pr comment delete 123 -R owner/repo --yes
```

删除 PR 评论。默认需要输入评论 ID 确认；使用 `--yes` 跳过确认。

### pr comment resolve - 标记 PR 评论为已解决

```bash
# 标记评论讨论为已解决
gc pr comment resolve 1 d1 -R owner/repo
```

标记 PR 评论讨论为已解决状态。Discussion ID 可通过 `gc pr comments` 查看。

### pr comment unresolve - 标记 PR 评论为未解决

```bash
# 标记评论讨论为未解决
gc pr comment unresolve 1 d1 -R owner/repo
```

标记 PR 评论讨论为未解决状态。

### pr reply - 回复 PR 评论

```bash
# 回复评论讨论
gc pr reply 1 --discussion <discussion_id> --body "回复内容" -R infra-test/gctest1

# 使用简写
gc pr reply 1 -d <discussion_id> -b "回复内容" -R owner/repo
```

### pr diff - 查看 PR 差异

```bash
# 查看 PR 差异
gc pr diff 1 -R infra-test/gctest1

# 输出 JSON
gc pr diff 1 -R infra-test/gctest1 --json
```

### pr checkout - 检出 PR 分支

```bash
# 检出 PR 到本地分支
gc pr checkout 1 -R infra-test/gctest1
```

### pr merge - 合并 PR

```bash
# 合并 PR（默认合并提交）
gc pr merge 1 -R infra-test/gctest1

# 非交互执行
gc pr merge 1 -R infra-test/gctest1 --yes

# Squash 合并
gc pr merge 1 -R infra-test/gctest1 --method squash

# Rebase 合并
gc pr merge 1 -R infra-test/gctest1 --method rebase

# 合并后删除源分支
gc pr merge 1 -R infra-test/gctest1 --delete-branch --yes

# 合并后输出 JSON
gc pr merge 1 -R infra-test/gctest1 --yes --json
```

说明：
- `pr merge` 属于高风险写操作，默认需要确认。
- 非交互场景中显式传 `--yes`。
- `--delete-branch` 会在合并成功后调用远端分支删除接口；删除失败时命令返回失败。
- `--json` 只在合并和可选删除分支都完成后输出结构化结果，包含顶层 `number`、`merged`、`pull_request` 和可选 `deleted_branch`；不会混入文本提示。

### pr close - 关闭 PR

```bash
# 关闭 PR
gc pr close 1 -R infra-test/gctest1

# 非交互执行
gc pr close 1 -R infra-test/gctest1 --yes

# 输出 JSON
gc pr close 1 -R infra-test/gctest1 --yes --json
```

说明：
- `pr close` 属于写操作，默认需要确认；非交互场景中显式传 `--yes`。

### pr reopen - 重开 PR

```bash
# 重开 PR
gc pr reopen 1 -R infra-test/gctest1

# 非交互执行
gc pr reopen 1 -R infra-test/gctest1 --yes

# 输出 JSON
gc pr reopen 1 -R infra-test/gctest1 --json
```

说明：
- `pr reopen` 属于写操作，默认需要确认；非交互场景中显式传 `--yes`。

### pr ready - 标记就绪状态

```bash
# 标记为就绪（取消草稿）
gc pr ready 1 -R infra-test/gctest1

# 标记为草稿
gc pr ready 1 -R infra-test/gctest1 --wip

# 非交互执行
gc pr ready 1 -R infra-test/gctest1 --ready --yes

# 输出 JSON
gc pr ready 1 -R infra-test/gctest1 --json
```

说明：
- `pr ready` 会修改 PR 草稿/就绪状态，默认需要确认；非交互场景中显式传 `--yes`。

### pr review - 评审 PR

```bash
# 评论 PR
gc pr review 1 --comment "评审意见" -R infra-test/gctest1

# 从文件读取评论
gc pr review 1 --comment-file review-notes.md -R infra-test/gctest1

# 从 stdin 读取评论
echo "评审意见" | gc pr review 1 --comment-file - -R infra-test/gctest1

# 批准 PR
gc pr review 1 --approve -R infra-test/gctest1

# 批准 PR 并附带评论
gc pr review 1 --approve --comment "LGTM" -R infra-test/gctest1

# 批准 PR 并从文件读取评论
gc pr review 1 --approve --comment-file self-check.md -R infra-test/gctest1

# 请求修改 PR
gc pr review 1 --request -R infra-test/gctest1

# 请求修改 PR 并附带评论
gc pr review 1 --request --comment "请修复错误处理" -R infra-test/gctest1

# 强制通过审批（管理员权限）
gc pr review 1 --approve --force -R infra-test/gctest1
```

说明：
- `--approve` 现在走 GitCode 实际可用的 `/pulls/:number/review` endpoint，不再命中错误的 `/reviews` 路径。
- `--approve --comment` 会先提交普通评论，再执行批准动作。
- `--request` 由于 GitCode API 不原生支持 "REQUEST_CHANGES" 事件，会以 `[REQUEST CHANGES]` 前缀提交评论来标记请求修改，输出中会提示降级行为。
- `--request --comment` 会将 `[REQUEST CHANGES]` 前缀与评论内容合并后提交。
- `--comment-file` 支持从文件读取多行评论，使用 `-` 可从 stdin 读取。
- `--comment` 与 `--comment-file` 互斥，不能同时使用。
- Windows PowerShell 中从 stdin 传中文评论时，建议使用 UTF-8 文件或先设置 `$OutputEncoding`，详见"Windows PowerShell 命令名和 stdin"。

> **权限说明**: `--approve` 需要 GitCode 平台的”审批权限”，与 `gc pr merge` 的”合并权限”是两套独立权限体系。
> - 有合并权限的用户不一定有审批权限
> - PR 作者或不在审批人范围内的用户可能收到 403 Forbidden
> - 如遇权限错误，请使用 `--comment` 留下评审记录，或联系仓库管理员授予审批权限
> - `--force` 仅限管理员使用，用于强制通过审批门禁

### pr edit - 编辑 PR

```bash
# 修改标题
gc pr edit 1 --title "新标题" -R infra-test/gctest1

# 修改描述
gc pr edit 1 --body "新描述" -R infra-test/gctest1

# 设置草稿状态
gc pr edit 1 --draft true -R infra-test/gctest1

# 取消草稿状态
gc pr edit 1 --draft false -R infra-test/gctest1

# 添加标签
gc pr edit 1 --labels bug,enhancement -R infra-test/gctest1

# 显式追加和移除标签（保留其余标签）
gc pr edit 1 --add-label status/approved --remove-label status/review -R infra-test/gctest1

# 替换全部标签（交互确认；自动化场景需显式添加 --yes）
gc pr edit 1 --replace-labels status/approved,risk/low -R infra-test/gctest1

# 清空全部标签
gc pr edit 1 --replace-labels "" -R infra-test/gctest1 --yes

# 设置里程碑
gc pr edit 1 --milestone 5 -R infra-test/gctest1

# JSON 输出
gc pr edit 1 --title "新标题" -R infra-test/gctest1 --json

# 添加评审人
gc pr edit 1 --add-reviewer dev1,dev2 -R infra-test/gctest1

# 移除评审人
gc pr edit 1 --remove-reviewer dev1 -R infra-test/gctest1

# 添加审查人
gc pr edit 1 --add-assignee reviewer1 -R infra-test/gctest1

# 添加测试人
gc pr edit 1 --add-tester tester1 -R infra-test/gctest1
```

说明：
- `--labels` 为兼容参数，语义是追加标签，不会删除 PR 上已有的其他标签；新脚本建议使用含义更明确的 `--add-label`。
- `--add-label` 和 `--remove-label` 可同时使用，均会保留未指定的现有标签，并对最终标签列表去重。
- `--replace-labels` 会替换全部标签，不能与 `--labels`、`--add-label` 或 `--remove-label` 同时使用。
- 全量替换默认要求输入 `replace-labels` 确认；非交互环境必须显式传入 `--yes`，空值可用于清空全部标签。
- `--add-assignee`/`--remove-assignee`：添加/移除审查人（POST/DELETE /pulls/{number}/assignees）。
- `--add-reviewer`/`--remove-reviewer`：添加/移除评审人（POST/DELETE /pulls/{number}/reviewers）。
- `--add-tester`/`--remove-tester`：添加/移除测试人（POST/DELETE /pulls/{number}/testers）。
- 以上 6 个 flag 可与 `--title`/`--body` 等基础编辑 flag 同时使用。
- `--json` 会在更新成功后读取并输出完整的最终 PR 状态，`id`、`number`、`labels` 等字段可直接供脚本和 AI 代理判断结果。

### pr label - 管理 PR 标签

```bash
# 添加标签
gc pr label 123 --add bug,enhancement -R owner/repo

# 移除标签
gc pr label 123 --remove bug -R owner/repo

# 列出 PR 标签
gc pr label 123 --list -R owner/repo

# JSON 输出
gc pr label 123 --list -R owner/repo --json
```

说明：
- `pr label` 支持添加（`--add`）、移除（`--remove`）、列出（`--list`）PR 标签。
- `--add` 接受逗号分隔的标签名列表，`--remove` 一次移除一个标签。
- `--json` 输出结构化标签数据，适合脚本和 AI 代理调用。
- `-R` 在当前 Git 仓库目录执行时可省略，命令会自动推断目标仓库。

### pr test - 触发 PR 测试

```bash
# 触发测试
gc pr test 1 -R infra-test/gctest1

# 强制通过测试（管理员权限）
gc pr test 1 --force -R infra-test/gctest1
```

### pr sync - 同步 PR 到另一个仓库

```bash
# 同步 PR 到目标仓库
gc pr sync --source-pr owner/source-repo#123 --target-repo owner/target-repo

# 指定目标分支
gc pr sync --source-pr owner/source-repo#123 \
  --target-repo owner/target-repo \
  --base release/v1.0

# 自定义标题和内容
gc pr sync --source-pr owner/source-repo#123 \
  --target-repo owner/target-repo \
  --title "[sync] Fix login bug" \
  --body "从 owner/source-repo#123 同步"

# 创建草稿 PR
gc pr sync --source-pr owner/source-repo#123 \
  --target-repo owner/target-repo \
  --draft

# 结构化输出
gc pr sync --source-pr owner/source-repo#123 \
  --target-repo owner/target-repo \
  --yes \
  --json
```

说明：
- `--source-pr` 支持两种格式：`owner/repo#number` 或完整 GitCode URL，例如 `https://gitcode.com/owner/repo/merge_requests/123`
- 命令会按原顺序逐个 cherry-pick 源 PR 的所有 commits 到目标仓库，保留提交边界
- `pr sync` clones, fetches, and pushes repositories over SSH; ensure an SSH key with access to `git@gitcode.com` is configured.
- 推送同步分支并创建目标 PR 前默认需要确认；非交互场景中显式传 `--yes`
- 新 PR 标题默认格式：`[sync] {源 PR 标题}`
- 新 PR 内容默认继承源 PR 内容并追加同步来源信息
- 如遇 cherry-pick 冲突，命令会报错并提示手动处理

---

## Release 命令 (release)

### release create - 创建 Release

```bash
# 创建 Release（建议包含 --notes 参数）
gc release create v1.0.0 -R infra-test/gctest1 --title "Version 1.0.0" --notes "Release notes"

# 从文件读取 Release Notes
gc release create v1.0.0 -R infra-test/gctest1 --title "Version 1.0.0" --notes-file CHANGELOG.md

# 创建预发布 Release
gc release create v1.0.0-beta -R infra-test/gctest1 --title "v1.0.0 Beta" --notes "Beta release" --prerelease

# 指定目标分支
gc release create v1.0.0 -R infra-test/gctest1 --title "v1.0.0" --notes "Release" --target main

# 创建后输出 JSON
gc release create v1.0.0 -R infra-test/gctest1 --title "v1.0.0" --notes "Release" --json
```

> **注意**: `--notes` 和 `--notes-file` 参数不能同时使用。
> `--draft` 当前不受 GitCode release create API 支持；CLI 会在发起远端创建前返回错误，避免创建出非草稿 Release。
> `--prerelease` 会使用 GitCode 的 `release_status=pre` 创建预发布，并在创建后回读确认状态。
> `--json` 只在成功创建后输出 release 对象；不会混入文本提示。

### release list - 列出 Releases

```bash
# 列出所有 Releases
gc release list -R infra-test/gctest1

# 输出 JSON
gc release list -R infra-test/gctest1 --json
```

说明：
- `--limit`（默认 30）返回最新 N 个 release：CLI 向 API 请求 `direction=desc`（最新优先），客户端再按 `published_at`（缺失时回退 `created_at`）降序排序。
- 文本输出中只有最新一个正式 release 会标记为 `(latest)`。
- 其他正式 release 会标记为 `(published)`；草稿和预发布仍分别显示 `(draft)`、`(pre-release)`。

### release view - 查看 Release

```bash
# 查看 Release 详情
gc release view v1.0.0 -R infra-test/gctest1

# 在浏览器中打开
gc release view v1.0.0 -R infra-test/gctest1 --web

# 输出 JSON
gc release view v1.0.0 -R infra-test/gctest1 --json
```

说明：
- 当 GitCode API 未返回资产大小时，文本输出会显示 `unknown size`，避免把未知值误写成 `0 bytes`。

### release upload - 上传资产

```bash
# 上传单个文件
gc release upload v1.0.0 app.zip -R infra-test/gctest1

# 上传多个文件
gc release upload v1.0.0 app.zip checksum.txt -R infra-test/gctest1

# 上传后输出 JSON
gc release upload v1.0.0 app.zip -R infra-test/gctest1 --json
```

说明：
- `--label` 参数当前不受 GitCode release upload API 支持；CLI 现在会直接报错，不再静默忽略。
- `--json` 只在所有文件上传完成后输出上传结果数组；每项包含 `name`、`path`、`size` 和 `content_type`，不会混入文本提示。

### release download - 下载资产

```bash
# 下载 latest release 的资产到当前目录（默认跳过 source archive）
gc release download -R infra-test/gctest1

# 下载指定 release 的资产到当前目录
gc release download v1.0.0 -R infra-test/gctest1

# 下载到指定目录
gc release download v1.0.0 -R infra-test/gctest1 -o ./downloads/

# 下载指定文件
gc release download v1.0.0 app.zip -R infra-test/gctest1

# 下载全部资产（含 source archive）
gc release download v1.0.0 -R infra-test/gctest1 --all
```

说明：
- 默认过滤 source archive（`.zip`/`.tar.gz` 源码包），只下载二进制/ wheel 等发布资产。
- 需要完整下载（包含 source archive）时显式传 `--all`。

### release edit - 编辑 Release

```bash
# 修改标题
gc release edit v1.0.0 --title "New title" -R infra-test/gctest1

# 修改说明
gc release edit v1.0.0 --notes "New release notes" -R infra-test/gctest1

# 从文件读取说明
gc release edit v1.0.0 --notes-file RELEASE_NOTES.md -R infra-test/gctest1

# 标记为预发布
gc release edit v1.0.0 --prerelease true -R infra-test/gctest1

# 标记为正式发布
gc release edit v1.0.0 --prerelease false -R infra-test/gctest1

# JSON 输出
gc release edit v1.0.0 --title "New title" -R infra-test/gctest1 --json
```

说明：
- `--draft` 和 `--target` 参数当前不受 GitCode release edit API 支持，使用时会输出警告但继续执行其他修改。
- `--prerelease true` 将 release 标记为预发布状态（release_status=pre）。
- `--prerelease false` 将 release 标记为正式发布状态（release_status=latest）。
- 若只修改标题或只修改说明，API 会保留未修改字段的原始值。
- 支持包含斜杠的 tag 名称（如 `release/v1.0.0`）。

### release delete - 预演删除 Release（平台暂不支持实际删除）

```bash
# 预演删除
gc release delete v1.0.0 -R infra-test/gctest1 --dry-run
```

说明：
- GitCode 官方 OpenAPI 当前没有 Release 删除接口，实际删除请求会返回 `405 Method Not Allowed`。
- `--dry-run` 仅预览目标和参数，不执行删除；需要删除时请使用仓库 Release 页面。

### release delete-asset - 删除 Release 附件

```bash
# 删除指定 release 的附件
gc release delete-asset v1.0.0 12345 -R infra-test/gctest1

# 跳过确认
gc release delete-asset v1.0.0 12345 -R infra-test/gctest1 --yes

# 预演删除
gc release delete-asset v1.0.0 12345 -R infra-test/gctest1 --dry-run
```

说明：
- 删除指定 release 的附件（asset），参数为 `<tag> <asset-id>`。
- `asset-id` 可从 `gc release view <tag> --json` 的 `assets[].id` 获取。
- 破坏性操作：非交互环境需 `--yes` 跳过确认，否则要求输入 asset-id 确认。
- `--dry-run` 仅预览，不执行删除。

---

## Commit 命令 (commit)

### commit view - 查看提交

```bash
# 查看提交详情
gc commit view abc123 -R infra-test/gctest1

# 显示变更文件
gc commit view abc123 -R infra-test/gctest1 --show-diff

# 输出 JSON 格式
gc commit view abc123 -R infra-test/gctest1 --json

# 在浏览器打开
gc commit view abc123 -R infra-test/gctest1 --web
```

### commit diff - 获取提交差异

```bash
# 获取提交 diff
gc commit diff abc123 -R infra-test/gctest1
```

### commit patch - 获取提交补丁

```bash
# 获取提交 patch
gc commit patch abc123 -R infra-test/gctest1
```

### commit comments create - 创建提交评论

```bash
# 创建评论
gc commit comments create abc123 --body "Nice work!" -R infra-test/gctest1
```

### commit comments view - 查看提交评论

```bash
# 查看指定评论
gc commit comments view 123 -R infra-test/gctest1

# JSON 输出
gc commit comments view 123 -R infra-test/gctest1 --json
```

### commit comments edit - 编辑提交评论

```bash
# 编辑评论
gc commit comments edit 123 --body "Updated comment" -R infra-test/gctest1
```

### commit comments list - 列出仓库所有评论

```bash
# 列出所有评论
gc commit comments list -R infra-test/gctest1

# 分页
gc commit comments list -R infra-test/gctest1 --page 1 --per-page 50

# JSON 输出
gc commit comments list -R infra-test/gctest1 --json
```

### commit comments list-by-sha - 列出指定提交的评论

```bash
# 列出某提交的所有评论
gc commit comments list-by-sha abc123 -R infra-test/gctest1

# JSON 输出
gc commit comments list-by-sha abc123 -R infra-test/gctest1 --json
```

说明：
- `commit comments list` 和 `commit comments list-by-sha` 的 `--json` 输出评论对象数组；无评论时输出 `[]`。

---

## 标签命令 (label)

### label list - 列出标签

```bash
# 列出所有标签
gc label list -R infra-test/gctest1

# 分页查询
gc label list -R infra-test/gctest1 --limit 50 --page 2

# 结构化输出
gc label list -R infra-test/gctest1 --json
```

说明：
- `label list` 支持 `-L/--limit`（默认 30，客户端结果上限）和 `--per-page`（API page size，未指定时默认 `--limit`）；`--page` 翻页。`--limit` 与 `--per-page` 同时指定时，本地按 `--limit` 截断。

### label create - 创建标签

```bash
# 创建标签
gc label create "bug" -R infra-test/gctest1 --color "#ff0000" --description "Bug report"

# 输出 JSON
gc label create "bug" -R infra-test/gctest1 --color "#ff0000" --description "Bug report" --json
```

### label edit - 更新标签

```bash
# 重命名标签
gc label edit bug --new-name "bug-fix" -R infra-test/gctest1

# 更新标签颜色
gc label edit bug --color "#ff0000" -R infra-test/gctest1

# 输出 JSON
gc label edit bug --color "#ff0000" -R infra-test/gctest1 --json
```

说明：
- `label edit` 更新标签的名称和/或颜色；至少需要 `--new-name` 或 `--color` 之一。
- `--new-name` 更新标签名称，`--color` 更新颜色（hex 格式，如 `#ff0000`）。
- GitCode API 的 PATCH 端点不支持更新标签 description，故本命令不提供 `--description`。
- 支持 `--json` 输出更新后的标签。

### label delete - 删除标签

```bash
# 删除标签
gc label delete bug -R infra-test/gctest1

# 预演删除
gc label delete bug -R infra-test/gctest1 --dry-run

# 非交互执行
gc label delete bug -R infra-test/gctest1 --yes

# 输出 JSON
gc label delete bug -R infra-test/gctest1 --yes --json
```

---

## 里程碑命令 (milestone)

### milestone list - 列出里程碑

```bash
# 列出所有里程碑
gc milestone list -R infra-test/gctest1

# 分页查询
gc milestone list -R infra-test/gctest1 --limit 50 --page 2

# 结构化输出
gc milestone list -R infra-test/gctest1 --json
```

说明：
- `milestone list` 当前不提供 `--state`，因为 GitCode milestones list API 还没有对应筛选参数。`--limit`（默认 30）和 `--per-page`（API page size，默认 `--limit`）、`--page` 已支持。

### milestone create - 创建里程碑

```bash
# 创建里程碑
gc milestone create "v1.0" -R infra-test/gctest1 --description "First release"

# 输出 JSON
gc milestone create "v1.0" -R infra-test/gctest1 --description "First release" --json
```

### milestone view - 查看里程碑

```bash
# 查看里程碑详情（包含关联 issues）
gc milestone view 1 -R infra-test/gctest1

# JSON 输出（包含 issues 数组和计数）
gc milestone view 1 -R infra-test/gctest1 --json

# 只查看里程碑元数据，不显示 issues
gc milestone view 1 -R infra-test/gctest1 --issues=false
```

说明：
- `milestone view` 默认显示里程碑关联的 issues，按状态分组（Closed/Open）。
- `--json` 输出包含 `issues` 数组、`total_issues`、`closed_issues`、`open_issues` 字段；计数按里程碑完整标题查询关联 Issue 后计算，避免依赖服务端可能滞后的聚合字段。
- `--issues=false` 不显示关联 issues；与 `--json` 同用时仍会查询关联 Issue 以保证计数准确。
- `--json` 不能与 `--web` 同时使用。

### milestone edit - 编辑里程碑

```bash
# 编辑里程碑标题
gc milestone edit 1 --title "New Title" -R infra-test/gctest1

# 编辑里程碑描述
gc milestone edit 1 --description "Updated description" -R infra-test/gctest1

# 从文件读取描述
gc milestone edit 1 --description-file milestone-desc.md -R infra-test/gctest1

# 关闭里程碑
gc milestone edit 1 --state closed -R infra-test/gctest1

# 重新打开里程碑
gc milestone edit 1 --state open -R infra-test/gctest1

# 编辑截止日期
gc milestone edit 1 --due-date "2024-12-31" -R infra-test/gctest1

# JSON 输出
gc milestone edit 1 --title "New Title" --json -R infra-test/gctest1

# 同时编辑多个字段
gc milestone edit 1 --title "v2.0" --description "Next release" --due-date "2025-01-31" -R infra-test/gctest1
```

说明：
- `milestone edit` 支持编辑标题、描述、状态和截止日期。
- `--state` 支持 `open` 和 `closed` 两个值。
- `--description-file` 从文件读取描述内容，支持多行文本。
- `--json` 输出更新后的里程碑对象。
- 至少需要提供一个编辑选项（`--title`, `--description`, `--description-file`, `--state`, `--due-date`）。

### milestone delete - 删除里程碑

```bash
# 删除里程碑
gc milestone delete 1 -R infra-test/gctest1

# 预演删除
gc milestone delete 1 -R infra-test/gctest1 --dry-run

# 非交互执行
gc milestone delete 1 -R infra-test/gctest1 --yes
```

---

## Pre-commit 命令 (precommit)

`precommit` 命令组用于在提交或推送代码前检查仓库的 pre-commit 配置与本地环境，确保 Git 能正常拉起 pre-commit 与 pre-push 检查。跨平台支持 Windows、Linux（x86/arm）、macOS。

### precommit check - 检查 pre-commit 配置与环境

检查流程：

1. 检测仓库根是否存在 `.pre-commit-config.yaml`（或 `.yml`）。无配置时视为"无需检查"，退出码 `0`。
2. 检测本地 `pre-commit` 工具是否安装。
3. 执行 `pre-commit validate-config`，验证配置格式与最低版本要求。
4. 分别检测 git pre-commit 与 pre-push hook 是否已初始化；任一缺失时环境未就绪。
5. 可选：使用 `--run` 分别执行 pre-commit 与 pre-push 两个 stage 的全文件检查。

环境缺失时，在交互式终端（stdin 为 TTY）下会自动安装并初始化两种 hook；在非交互环境下需显式传 `--yes` 才会修改环境，否则仅诊断并报错。`--no-install` 表示只诊断、绝不修改环境。

```bash
# 检查环境是否就绪
gc precommit check

# 检查并实际拉起 pre-commit 检查
gc precommit check --run

# 仅诊断，不安装/初始化（不修改环境）
gc precommit check --no-install

# 非交互环境允许自动安装/初始化
gc precommit check --yes

# 机器可消费输出
gc precommit check --json
```

说明：

- 支持 `--json`：输出写入 stdout。兼容字段 `hook_installed` 保持原语义，表示 pre-commit hook 是否已安装；新增聚合字段 `hooks_installed` 仅在 pre-commit 与 pre-push 均已安装时为 `true`，独立状态字段为 `pre_commit_hook_installed`、`pre_push_hook_installed`。`--run` 的聚合结果仍由 `run_result`、`run_output` 表示，分阶段结果由 `pre_commit_run_result`、`pre_push_run_result` 表示。其余字段为 `config_found`、`tool_installed`、`tool_version`、`actions_taken`、`ok`、`reason`、`install_failure_categories`。即使自动安装失败（退出码 `1`），`--json` 仍会输出结构化结果体（`reason=install_failed`），不会只剩退出码与 stderr 文本。
- `reason` 是稳定、机器可读的结果分类，便于脚本/agent 直接分支，取值：
  - `no_config`：仓库未配置 pre-commit（`ok=true`，属正常跳过）。
  - `tool_missing`：`pre-commit` 工具未安装（且未尝试 / 未授权安装）。
  - `config_invalid`：当前 `pre-commit` 无法加载仓库配置，例如版本低于配置声明的 `minimum_pre_commit_version` 或配置格式无效。
  - `hook_missing`：git pre-commit 或 pre-push hook 未初始化。
  - `run_failed`：环境就绪但 pre-commit 或 pre-push stage 检查失败。
  - `install_failed`：已授权自动安装，但未能产出可用的 pre-commit 工具或完整 hooks；包括工具安装失败、hook 初始化命令失败，以及命令成功但复检仍缺少 hook。可结合 `tool_installed`、`hooks_installed`、`pre_commit_hook_installed`、`pre_push_hook_installed` 定位失败阶段；工具安装的具体失败类型见 `install_failure_categories`。
  - `not_in_repo`：当前目录不在 git 仓库内。
  - 环境完全就绪（且 `--run` 通过或未请求）时 `reason` 省略（为空）。
- `install_failure_categories` 仅在 `reason=install_failed` 时出现，为机器可读的失败类型数组（按首次出现顺序去重）：`permission`（权限不足）/ `network`（网络失败）/ `toolchain`（缺少 Python/pip 工具链）。无法归类的失败不计入该数组（可能为空）。
- 仓库声明 `minimum_pre_commit_version: 3.2.0`，以支持与 Git hook 同名的 `pre-commit`/`pre-push` stage；检查命令会先执行 `pre-commit validate-config`，旧版本或无效配置不会被报告为 ready。
- Git 配置了自定义 `core.hooksPath` 时，`pre-commit install` 可能拒绝自动初始化；此时返回 `reason=install_failed`，请按错误提示处理或移除该配置后重试。
- `--no-install` 与 `--yes` 互斥；hooks 本身运行失败时报"pre-commit checks failed"（区别于"环境未就绪"）。
- 退出码：`0` 就绪或无配置；`1` 环境未就绪 / 检查失败 / 非 Git 仓库 / 非交互且未授权修改环境；`2` 用法错误。
- 自动安装按工具可用性择优：`pipx` → `python3 -m pip install --user` → `python -m pip install --user`；都不可用时给出各平台手动安装指引。安装失败时按错误类型给出针对性指引（权限不足 / 网络失败 / 工具链缺失）。
- 不在 PATH 目录间复制二进制，始终在项目内调用。

---

## Actions 命令 (actions)

`actions` 命令组用于检视 GitCode Actions 流水线（pipeline）运行记录与工作流作业（workflow jobs），以只读为主（`run stop`/`run rerun`/`run retry`/`artifact delete`/`workflow run` 等写操作带确认门保护），通过 Actions v8 API（`/api/v8/...`）访问。与其它命令默认使用的 v5 不同，Actions 走独立的 v8 路径。

### actions run list - 列出流水线运行记录

列出仓库的流水线运行记录，支持按状态、事件、分支、触发人、流水线、创建日期和提交 SHA 等过滤。
除提交 SHA 外的过滤在服务端应用；提交 SHA 在客户端按每条记录的 `head_sha` 过滤。

```bash
# 列出最近的运行记录
gc actions run list -R owner/repo

# 按状态过滤
gc actions run list -R owner/repo --status FAILED

# 按事件与分支过滤
gc actions run list -R owner/repo --event Push --branch main

# 按触发人过滤
gc actions run list -R owner/repo --executor dev

# 按流水线名称或 id 过滤
gc actions run list -R owner/repo --workflow "CI"
gc actions run list -R owner/repo --workflow-id wf-1

# 按 PR 编号过滤
gc actions run list -R owner/repo --pr 42

# 按创建日期过滤（UTC 当日 00:00:00 起）
gc actions run list -R owner/repo --created 2026-08-01

# 按完整 commit SHA 过滤，可与其他过滤项组合
gc actions run list -R owner/repo \
  --commit 0123456789abcdef0123456789abcdef01234567 --status FAILED --branch main

# 抓取全部分页
gc actions run list -R owner/repo --paginate --per-page 100

# 表格输出
gc actions run list -R owner/repo --format table

# 机器可消费输出
gc actions run list -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，字段直接映射 Actions v8 API 响应（`workflow_runs` 数组，每项含 `workflow_run_id`、`workflow_id`、`workflow_name`、`file_path`、`title`、`status`、`event`、`run_number`、`head_branch`、`head_sha`、`actor`、`start_time`、`end_time`、`pause_time`）；空结果输出 `[]`。
- `--status` 取值：`COMPLETED`/`RUNNING`/`FAILED`/`CANCELED`/`IGNORED`/`PAUSED`/`SUSPEND`；`--event` 取值：`MR`/`Push`/`Manual`。枚举为元数据（schema/help 发现用），不在本地强制校验，非法值原样透传给 API。
- `--created` 仅接受 `YYYY-MM-DD`，转换为 UTC 当日 00:00:00 的 Unix 秒并通过 Actions v8 API 的 `startTime` 参数过滤。
- `--commit` 在客户端对 API 返回记录的完整 `head_sha` 做不区分大小写的精确匹配；它可与服务端过滤项组合。
- 认证复用标准 Bearer header（`GC_TOKEN`/`GITCODE_TOKEN` 或本地配置），不通过 `access_token` query 参数暴露 token。
- 分页：`--limit`/`-L`（默认 30，映射为 `per_page`）、`--page`、`--paginate`（抓取全部分页至 `--limit`）、`--per-page`（API 页大小）。`--paginate` 与 `--page` 互斥。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如 `--paginate` 与 `--page` 同用、`--limit`/`--per-page` 为负）；`3` 资源不存在（HTTP 404，如仓库不存在）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

### actions run stop - 停止运行中的流水线

停止一条正在运行的流水线（run）。`<run-id>` 取 `gc actions run list` 返回的 `workflow_run_id`。

```bash
# 停止运行中的 run（交互确认）
gc actions run stop <run-id> -R owner/repo

# 跳过确认（脚本场景）
gc actions run stop <run-id> -R owner/repo --yes

# JSON 输出
gc actions run stop <run-id> -R owner/repo --yes --json
```

说明：

- 调用 `POST /api/v8/repos/{owner}/{repo}/actions/runs/{run_id}/stop`。
- 服务端幂等：对已经结束（非 RUNNING）的 run 调用同样返回成功。
- 确认门（spec/foundations/agent-friendly-cli.md §4 破坏性命令确认）：默认需交互输入 `stop pipeline run <run-id>` 确认；`--yes` 跳过确认；非交互环境未携带 `--yes` 时立即失败（退出码 2）。
- 支持 `--json`：输出 `{"run_id","owner","repo","action"}` 结构到 stdout。
- 认证复用标准 Bearer header（`GC_TOKEN`/`GITCODE_TOKEN` 或本地配置），不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误（如缺少 `<run-id>`、非交互未携带 `--yes`、确认输入不匹配）；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

### actions run view - 查看流水线运行详情

查看单条流水线运行记录的详情，含运行元信息与其下 stages（阶段）/jobs（任务）概览。`<run-id>` 取 `gc actions run list` 返回的 `workflow_run_id`。

```bash
# 查看一条流水线运行详情
gc actions run view <run-id> -R owner/repo

# 忠实 JSON 输出（保留 API 全部字段，含 stage/job/step 深层执行字段）
gc actions run view <run-id> -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，**原样透传 API 响应**（字段名直接映射 Actions v8 API，保留 `stages[].jobs[].steps[]` 及 `pre`/`post`/`parallel`/`condition`/`inputs`/`env` 等深层/可空字段，不被本地类型裁剪）。人类可读视图（默认）只摘要显示运行元信息 + stages/jobs 状态与计数，steps 明细见 `--json`。
- 认证复用标准 Bearer header（`GC_TOKEN`/`GITCODE_TOKEN` 或本地配置），不通过 `access_token` query 参数暴露 token。
- 时间字段（`started`/`ended`/`paused` 及 stage/job 时间）由 API 的毫秒时间戳格式化为 RFC3339（UTC）；`--json` 保留原始毫秒整数值。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如缺少 `<run-id>`）；`3` 资源不存在（HTTP 404，如 run 不存在或仓库不存在）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

### actions run watch - 监控流水线运行直到完成

轮询监控一条流水线运行（run）的状态，直到到达终态（`COMPLETED`/`FAILED`/`CANCELED`/`IGNORED`/`PAUSED`/`SUSPEND`）。`<run-id>` 取 `gc actions run list` 返回的 `workflow_run_id`。

```bash
# 监控 run 直到完成
gc actions run watch <run-id> -R owner/repo

# 指定轮询间隔（秒）
gc actions run watch <run-id> -R owner/repo --interval 10

# 紧凑模式（仅展示失败 step）
gc actions run watch <run-id> -R owner/repo --compact

# run 失败时返回非零退出码（CI 脚本用）
gc actions run watch <run-id> -R owner/repo --exit-status

# JSON 输出（单次快照）
gc actions run watch <run-id> -R owner/repo --json
```

说明：

- TTY 环境：按 `--interval`（默认 3 秒）定时刷新屏幕，展示 run 概要 + stages/jobs/steps 状态；到达终态后停止。
- 非 TTY 环境（管道输出、`--no-interactive`）：输出单次快照后立即退出，不阻塞等待（符合 agent-friendly 契约）。
- `--compact`：仅展示包含失败 job 的 stage 和失败 step。
- `--exit-status`：run 终态为 `FAILED`/`CANCELED` 时返回退出码 `1`，用于 CI 自动化。
- 支持 `--json`：输出写入 stdout，原样透传 API 响应（同 `run view --json`）。
- 认证复用标准 Bearer header（`GC_TOKEN`/`GITCODE_TOKEN` 或本地配置），不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功（含 `--exit-status` 时 run 成功完成）；`1` 通用错误或 `--exit-status` 时 run 失败；`2` 参数错误（如缺少 `<run-id>` 或 `--interval < 1`）；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误（HTTP 401/403）。

### actions run rerun - 重跑整条流水线

重新执行指定流水线的全部任务（对齐 `gh run rerun`）。`<run-id>` 取 `gc actions run list` 返回的 `workflow_run_id`。

```bash
# 重跑 run（交互确认）
gc actions run rerun <run-id> -R owner/repo

# 跳过确认（脚本场景）
gc actions run rerun <run-id> -R owner/repo --yes

# JSON 输出
gc actions run rerun <run-id> -R owner/repo --yes --json
```

说明：

- 调用 `POST /api/v8/repos/{owner}/{repo}/actions/runs/{run_id}/rerun`。
- 仅处于完成状态的 run 可重跑；对 RUNNING 中的 run 调用会返回冲突错误（HTTP 409，服务端业务校验）。
- 重跑为同 run 原地重跑，可结合 `gc actions run watch <run-id>` 继续跟踪。
- 确认门（spec/foundations/agent-friendly-cli.md §4 状态变更写操作保护）：默认需交互输入 `rerun pipeline run <run-id>` 确认；`--yes` 跳过确认；非交互环境未携带 `--yes` 时立即失败（退出码 2）。
- 支持 `--json`：输出 `{"run_id","owner","repo","action"}` 结构到 stdout。
- 认证复用标准 Bearer header（`GC_TOKEN`/`GITCODE_TOKEN` 或本地配置），不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误（如缺少 `<run-id>`、非交互未携带 `--yes`、确认输入不匹配）；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409，如对运行中的 run 重跑）。

### actions job list - 列出工作流运行的 jobs

列出一条流水线运行（run）下的工作流作业（jobs）。`<run-id>` 取 `gc actions run list` 返回的 `workflow_run_id`。

```bash
# 列出 run 的 jobs
gc actions job list <run-id> -R owner/repo

# 表格输出
gc actions job list <run-id> -R owner/repo --format table

# 机器可消费输出
gc actions job list <run-id> -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，为数组，每项映射 `WorkflowRunJob`（`id`、`name`、`identifier`、`status`、`sequence`、`job_type`、`resource`、`condition`、`is_select`、`depends_on`、`start_time`、`end_time`、`execute_cost_time`、`exec_id`、`last_dispatch_id`、`steps`）；空结果输出 `[]`。深层可空执行字段（`category`/`async`/`timeout`/`message`/`max_parallel` 等）不建模（多为 null，列表视图用不到；完整明细见 `actions run view` 或后续 `job view`）。
- 认证复用标准 Bearer header（`GC_TOKEN`/`GITCODE_TOKEN` 或本地配置），不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如缺少 `<run-id>`）；`3` 资源不存在（HTTP 404，如 run 不存在或仓库不存在）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

### actions job view - 查看工作流 job 详情

查看单个工作流作业（job）的详情，含其 steps。需同时提供 `<run-id>`（`gc actions run list` 的 `workflow_run_id`）与 `<job-id>`（`gc actions job list` 的 `id`），对应 API 路径 `runs/{run_id}/jobs/{job_id}`。

```bash
# 查看 job 详情
gc actions job view <run-id> <job-id> -R owner/repo

# 忠实 JSON 输出（保留 API 全部字段，含 steps 的 inputs/env 等深层字段）
gc actions job view <run-id> <job-id> -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，**原样透传 API 响应**（字段名直接映射 Actions v8 API，保留 `steps[]` 及 `inputs`/`env`/`runtime_attribution` 等深层/可空字段，不被本地类型裁剪）。人类可读视图（默认）摘要显示 job 元信息 + steps 列表（名称/task/status/时间），steps 深层明细见 `--json`。
- 认证复用标准 Bearer header（`GC_TOKEN`/`GITCODE_TOKEN` 或本地配置），不通过 `access_token` query 参数暴露 token。
- 时间字段（`started`/`ended` 及 step 时间）由 API 的毫秒时间戳格式化为 RFC3339（UTC）；`--json` 保留原始毫秒整数值。`execute_cost_time` 单位未在 API 文档标明，人类视图不渲染（避免误显），`--json` 保留原始值。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如缺少 `<run-id>`/`<job-id>`）；`3` 资源不存在（HTTP 404，如 job 不存在或仓库不存在）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

### actions job log - 下载工作流 job 日志

下载单个工作流作业（job）的日志。需同时提供 `<run-id>`（`gc actions run list` 的 `workflow_run_id`）与 `<job-id>`（`gc actions job list` 的 `id`），对应 API 路径 `runs/{run_id}/jobs/{job_id}/download_log`。

```bash
# 保存 job 日志归档并解压
gc actions job log <run-id> <job-id> -R owner/repo --output job-log.zip
unzip job-log.zip

# 重定向到文件（非 TTY / 管道）
gc actions job log <run-id> <job-id> -R owner/repo > job-log.zip
```

说明：

- 端点返回 **ZIP 归档**（二进制，含各 step 日志如 `0_Login to CodeArts.log`/`1_codecheck.log`），非纯文本、非 JSON；故无 `--json`。
- 输出：`--output/-o FILE` 写文件（推荐，便于 `unzip`）；无 `--output` 时写原始字节到 stdout。**在交互式终端（TTY）且未给 `--output` 时拒绝写入**（避免二进制刷乱终端），提示用 `--output` 或重定向；管道/重定向（非 TTY）正常写原始字节。
- 认证复用标准 Bearer header（`GC_TOKEN`/`GITCODE_TOKEN` 或本地配置）；API 文档虽标 `access_token` 为 required，但 GitCode 接受 Bearer，不通过 query 暴露 token。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如缺少 `<run-id>`/`<job-id>`，或 TTY 未给 `--output`/重定向）；`3` 资源不存在（HTTP 404，如 job 不存在或仓库不存在）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

### actions artifact list - 列出仓库 Artifacts

分页查询仓库下的制品（artifacts），支持按名称模糊过滤与排序。`--run <run-id>` 切换到特定 Run 的制品列表（API 路径 `/actions/runs/{run_id}/artifacts`）。过滤在服务端应用。

```bash
# 列出仓库 artifacts
gc actions artifact list -R owner/repo

# 列出特定 Run 的 artifacts
gc actions artifact list -R owner/repo --run <run-id>

# 按名称过滤（模糊匹配）
gc actions artifact list -R owner/repo --name build

# 按创建时间排序
gc actions artifact list -R owner/repo --sort created --direction desc

# 抓取全部分页
gc actions artifact list -R owner/repo --paginate --per-page 100

# 表格输出
gc actions artifact list -R owner/repo --format table

# 机器可消费输出
gc actions artifact list -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，为数组，每项映射 `Artifact`（`id`、`name`、`size_bytes`、`workflow_id`、`workflow_run_id`、`digest`、`expires_at`、`created_at`、`updated_at`；时间为字符串型毫秒时间戳）；空结果输出 `[]`。
- `--sort` 取值 `created`；`--direction` 取值 `asc`/`desc`。枚举为元数据（schema/help 发现用），不在本地强制校验，非法值原样透传给 API。
- 认证复用标准 Bearer header（`GC_TOKEN`/`GITCODE_TOKEN` 或本地配置），不通过 `access_token` query 参数暴露 token。
- 分页：`--limit`/`-L`（默认 30，映射为 `per_page`）、`--page`、`--paginate`（抓取全部分页至 `--limit`）、`--per-page`。`--paginate` 与 `--page` 互斥。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如 `--paginate` 与 `--page` 同用、`--limit`/`--per-page` 为负）；`3` 资源不存在（HTTP 404，如仓库不存在）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

### actions artifact view - 查看 Artifact 详情

查看单个制品（artifact）的详情。`<artifact-id>` 取 `gc actions artifact list` 返回的 `id`。

```bash
# 查看 artifact 详情
gc actions artifact view <artifact-id> -R owner/repo

# 忠实 JSON 输出（保留 API 全部字段）
gc actions artifact view <artifact-id> -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，**原样透传 API 响应**（字段名直接映射 Actions v8 API，时间字段为字符串型毫秒时间戳）。人类可读视图（默认）显示 artifact 元信息（name/id/size 人类可读 B/KiB/MiB/GiB/digest/workflow_run_id/created/updated/expires，时间 ms→RFC3339 UTC）。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如缺少 `<artifact-id>`，或 artifact_id 格式不合法 → HTTP 400 参数类型错误）；`3` 资源不存在（HTTP 404，如 artifact 不存在或仓库不存在）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。注意：该端点对格式不合法的 artifact_id 返回 HTTP 400（exit 2），而非 404。

### actions artifact download - 下载 Artifact

下载单个制品（artifact）为 ZIP 归档。`<artifact-id>` 取 `gc actions artifact list` 返回的 `id`。端点返回 302 跳转到预签名下载 URL，HTTP 客户端自动跟随，最终获取 ZIP 字节。

```bash
# 保存 artifact 到文件
gc actions artifact download <artifact-id> -R owner/repo --output artifact.zip

# 重定向到文件（非 TTY / 管道）
gc actions artifact download <artifact-id> -R owner/repo > artifact.zip
```

说明：

- 端点返回 **ZIP 归档**（二进制），非 JSON；故无 `--json`。
- 输出：`--output/-o FILE` 写文件（推荐）；无 `--output` 时写原始字节到 stdout。**在交互式终端（TTY）且未给 `--output` 时拒绝写入**（避免二进制刷乱终端），提示用 `--output` 或重定向；管道/重定向（非 TTY）正常写原始字节。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如缺少 `<artifact-id>`，或 TTY 未给 `--output`/重定向，或 artifact_id 格式不合法 → HTTP 400）；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

### actions artifact delete - 删除 Artifact

删除单个制品（artifact）。`<artifact-id>` 取 `gc actions artifact list` 返回的 `id`。这是**破坏性操作**，默认有确认保护。

```bash
# 交互式删除（需输入 artifact id 确认）
gc actions artifact delete <artifact-id> -R owner/repo

# 非交互式（需 --yes）
gc actions artifact delete <artifact-id> -R owner/repo --yes

# 预览删除（不实际删除）
gc actions artifact delete <artifact-id> -R owner/repo --dry-run

# JSON 输出
gc actions artifact delete <artifact-id> -R owner/repo --yes --json
```

说明：

- 支持 `--json`：输出 `{artifact_id, owner, repo, action}`（action 为 `deleted` 或 `dry_run`）。
- 确认保护：交互式终端（TTY）需输入 artifact id 确认；非 TTY 环境须传 `--yes` 跳过，否则立即失败（exit 2）。`--dry-run` 跳过确认，仅预览。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如缺少 `<artifact-id>`，或非 TTY 未给 `--yes`，或 artifact_id 格式不合法 → HTTP 400）；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

---

### actions workflow list - 列举仓库流水线

列出仓库中的 Actions 流水线（workflow）文件。

```bash
gc actions workflow list -R owner/repo

# JSON 输出
gc actions workflow list -R owner/repo --json
```

说明：
- 调用 GET /api/v8/repos/{owner}/{repo}/actions/workflows。
- 响应含 `workflow_id`、`file_path`、`name`、`state` 字段。
- `--json` 输出 workflow 数组到 stdout。
- `--limit`/`--page` 分页。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 仓库不存在；`4` 认证错误。

### actions workflow run - 手动触发流水线

手动触发 workflow_dispatch 运行。workflow_id 可为 ID 或带 yml/yaml 后缀的文件名。

```bash
# 通过 ID 触发
gc actions workflow run wf-1 -R owner/repo --ref main

# 通过文件名触发
gc actions workflow run ci.yml -R owner/repo --ref main

# 带自定义参数
gc actions workflow run ci.yml -R owner/repo --ref main -f key1=value1 -f key2=value2

# JSON 输出（返回 workflow_run_id）
gc actions workflow run ci.yml -R owner/repo --ref main --json
```

说明：
- 调用 POST /api/v8/repos/{owner}/{repo}/actions/workflows/{workflow_id}/dispatches。
- `--ref`（必填）：分支名或 tag 名。
- `-f`/`--field`：自定义参数 key=value，可多次传入。值经 secret 扫描。
- `--json` 输出 `{workflow_id, workflow_run_id}`。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误（缺 --ref）；`3` 仓库/workflow 不存在；`4` 认证错误。

### actions yaml validate - 校验 Workflow YAML

调用 GitCode Actions v8 官方接口（`POST /api/v8/repos/{owner}/{repo}/actions/workflows/validate`）校验 workflow YAML 配置语法。YAML 内容按 API 要求 base64 编码后放入 `base64_content` 请求字段。

```bash
# 校验 workflow 文件
gc actions yaml validate --file .gitcode/workflows/ci.yml -R owner/repo

# 从 stdin 读取 YAML
cat ci.yml | gc actions yaml validate --file - -R owner/repo

# JSON 输出
gc actions yaml validate --file ci.yml -R owner/repo --json
```

说明：

- `--file`（必填）：要校验的 YAML 文件路径；传 `-` 表示从 stdin 读取。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- `--json` 原样透传 API 响应（`valid`、`diagnostics[].range.start/end.line/column`、`severity`、`message`）。
- 退出码：`0` 校验通过（`valid=true`）；`1` 校验未通过（`valid=false`）或其它 API 错误；`2` 参数错误（缺少 `--file` 等）；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

---

### actions runner-group list - 列出组织 Runner Group

列出指定组织下的所有 Runner Group。

```bash
# 列出组织的 runner groups
gc actions runner-group list --org my-org

# 按关键字过滤
gc actions runner-group list --org my-org --keyword prod

# 获取所有页
gc actions runner-group list --org my-org --paginate --per-page 100

# JSON 输出
gc actions runner-group list --org my-org --json
```

说明：

- 支持 `--json`：输出写入 stdout，字段名直接映射 Actions v8 API（id/name/runner_group_name/namespace_id/creator/create_time/runner_count/namespace_type/share_all）。
- `--org`（必填）：组织 path（如 `my-org`）。
- `--keyword`：关键字过滤（服务端过滤）。
- 分页：`--page` 指定页码；`--paginate` 自动获取所有页（不能与 `--page` 同用）；`--per-page` 控制 API 页大小；`--limit` 截断总数。
- 空结果输出 `[]`（JSON）或 `No runner groups found`（文本）。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如缺少 `--org`，或 `--paginate` 与 `--page` 同用，或 `--limit`/`--per-page` 为负）；`3` 资源不存在（HTTP 404，如组织不存在）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

---

### actions runner-group view - 查看 Runner Group 详情

查看指定组织下单个 Runner Group 的详细信息。`<runner-group-id>` 取 `gc actions runner-group list` 返回的 `id`。

```bash
# 查看 runner group 详情
gc actions runner-group view <runner-group-id> --org my-org

# 忠实 JSON 输出（保留 API 全部字段）
gc actions runner-group view <runner-group-id> --org my-org --json
```

说明：

- 支持 `--json`：输出写入 stdout，**原样透传 API 响应**（字段名直接映射 Actions v8 API：runner_group_id/runner_group_name/share_all/share_all_public_repos/explicit_shared_repo_count/created_at/updated_at）。人类可读视图显示 ID/名称/分享状态/分享仓库数/创建时间/更新时间。
- `--org`（必填）：组织 path（如 `my-org`）。
- `<runner-group-id>`（位置参数，必填）：Runner Group ID。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误（其它 API 错误）；`2` 参数错误（如缺少 `<runner-group-id>` 或 `--org`）；`3` 资源不存在（HTTP 404，如 runner group 或组织不存在）；`4` 认证/权限错误（HTTP 401/403）；`5` 资源冲突（HTTP 409）。

---

### actions runner-group runner list - 列出 Runner Group 下的主机 Runner

列出指定 Runner Group 下的所有主机 Runner。

```bash
# 列出 runner group 下的主机 runner
gc actions runner-group runner list <runner-group-id> --org my-org

# 按关键字过滤
gc actions runner-group runner list <runner-group-id> --org my-org --keyword prod

# JSON 输出
gc actions runner-group runner list <runner-group-id> --org my-org --json
```

说明：

- 支持 `--json`：输出写入 stdout，字段名直接映射 Actions v8 API（id/runner_group_id/runner_name/name/work_dir/labels[ label_name/label_value/label_color ]）。
- `<runner-group-id>`（位置参数，必填）：Runner Group ID，取 `gc actions runner-group list` 返回的 `id`。
- `--org`（必填）：组织 path。
- `--keyword`：关键字过滤（服务端过滤）。
- 分页：`--page`/`--paginate`/`--per-page`/`--limit`。
- 空结果输出 `[]`（JSON）或 `No runners found`（文本）。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误（如缺少 `<runner-group-id>` 或 `--org`，或 runner group 不存在 → HTTP 400）；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误；`5` 资源冲突。

---

### actions runner-group runner-set list - 列出 Runner Group 下的 K8S Runner Set

列出指定 Runner Group 下的所有 K8S Runner Set。

```bash
# 列出 K8S runner sets
gc actions runner-group runner-set list <runner-group-id> --org my-org

# 按关键字过滤
gc actions runner-group runner-set list <runner-group-id> --org my-org --keyword prod

# JSON 输出
gc actions runner-group runner-set list <runner-group-id> --org my-org --json
```

说明：

- 支持 `--json`：输出写入 stdout，字段名直接映射 Actions v8 API（id/runner_group_id/name/status/required_labels[ label_name/label_value/label_color ]）。
- `<runner-group-id>`（位置参数，必填）：Runner Group ID。
- `--org`（必填）：组织 path。
- `--keyword`：关键字过滤（服务端过滤）。
- 分页：`--page`/`--paginate`/`--per-page`/`--limit`。
- 空结果输出 `[]`（JSON）或 `No runner sets found`（文本）。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误（如缺少 `<runner-group-id>` 或 `--org`，或 runner group 不存在 → HTTP 400）；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误；`5` 资源冲突。

---

### actions runner-group shared-namespace list - 列出 Runner Group 的共享命名空间

列出有权访问指定 Runner Group 的仓库/命名空间列表。

```bash
# 列出共享命名空间
gc actions runner-group shared-namespace list <runner-group-id> --org my-org

# 按关键字过滤
gc actions runner-group shared-namespace list <runner-group-id> --org my-org --keyword prod

# JSON 输出
gc actions runner-group shared-namespace list <runner-group-id> --org my-org --json
```

说明：

- 支持 `--json`：输出写入 stdout，字段名直接映射 Actions v8 API（id/runner_group_id/from_namespace_id/to_namespace_id/type/create_time/update_time）。
- `<runner-group-id>`（位置参数，必填）：Runner Group ID。
- `--org`（必填）：组织 path。
- `--keyword`：关键字过滤（服务端过滤）。
- 分页：`--page`/`--paginate`/`--per-page`/`--limit`。
- 空结果输出 `[]`（JSON）或 `No shared namespaces found`（文本）。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误；`5` 资源冲突。

---

### actions runner list - 列出仓库主机 Runner

列出指定仓库下的所有主机 Runner。

```bash
# 列出仓库的主机 runner
gc actions runner list -R owner/repo

# 按关键字过滤
gc actions runner list -R owner/repo --keyword prod

# JSON 输出
gc actions runner list -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，字段名直接映射 Actions v8 API（id/runner_group_id/runner_name/name/work_dir/labels[ label_name/label_value/label_color ]）。
- `-R`：仓库（可选，缺省时从当前 git 仓库解析）（owner/repo）。
- `--keyword`：关键字过滤（服务端过滤）。
- 分页：`--page`/`--paginate`/`--per-page`/`--limit`。
- 空结果输出 `[]`（JSON）或 `No runners found`（文本）。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误；`5` 资源冲突。

---

### actions runner-set list - 列出仓库 K8S Runner Set

列出指定仓库下的所有 K8S Runner Set。

```bash
# 列出仓库的 K8S runner sets
gc actions runner-set list -R owner/repo

# 按关键字过滤
gc actions runner-set list -R owner/repo --keyword prod

# JSON 输出
gc actions runner-set list -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，字段名直接映射 Actions v8 API（id/runner_group_id/name/status/required_labels[ label_name/label_value/label_color ]）。
- `-R`：仓库（owner/repo，可选，缺省时从当前 git 仓库解析）。
- `--keyword`：关键字过滤（服务端过滤）。
- 分页：`--page`/`--paginate`/`--per-page`/`--limit`。
- 空结果输出 `[]`（JSON）或 `No runner sets found`（文本）。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误；`5` 资源冲突。

---

### actions runner shared-runners - 列出仓库的共享主机 Runner

列出分享给指定仓库的所有主机 Runner（来自组织 Runner Group 共享）。

```bash
# 列出共享主机 runner
gc actions runner shared-runners -R owner/repo

# 按关键字过滤
gc actions runner shared-runners -R owner/repo --keyword prod

# JSON 输出
gc actions runner shared-runners -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，字段名直接映射 Actions v8 API（id/runner_group_id/runner_name/name/work_dir/labels[ label_name/label_value/label_color ]）。
- `-R`：仓库（owner/repo，可选，缺省时从当前 git 仓库解析）。
- `--keyword`：关键字过滤（服务端过滤）。
- 分页：`--page`/`--paginate`/`--per-page`/`--limit`。
- 空结果输出 `[]`（JSON）或 `No shared runners found`（文本）。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误；`5` 资源冲突。

---

### actions runner-set shared-runner-sets - 列出仓库的共享 K8S Runner Set

列出分享给指定仓库的所有 K8S Runner Set（来自组织 Runner Group 共享）。

```bash
# 列出共享 K8S runner sets
gc actions runner-set shared-runner-sets -R owner/repo

# 按关键字过滤
gc actions runner-set shared-runner-sets -R owner/repo --keyword prod

# JSON 输出
gc actions runner-set shared-runner-sets -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，字段名直接映射 Actions v8 API（id/runner_group_id/name/status/required_labels[ label_name/label_value/label_color ]）。
- `-R`：仓库（owner/repo，可选，缺省时从当前 git 仓库解析）。
- `--keyword`：关键字过滤（服务端过滤）。
- 分页：`--page`/`--paginate`/`--per-page`/`--limit`。
- 空结果输出 `[]`（JSON）或 `No shared runner sets found`（文本）。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误；`5` 资源冲突。

---

### actions plugin list - 列出官方 Actions 插件

列出指定仓库可用的官方 GitCode Actions 插件。

```bash
# 列出插件
gc actions plugin list -R owner/repo

# 获取单页
gc actions plugin list -R owner/repo --page 1 --per-page 20

# 输出为 JSON
gc actions plugin list -R owner/repo --json
```

说明：

- 支持 `--json`：输出写入 stdout，原样保留完整 API 字段。
- `-R`：仓库（owner/repo，可选，缺省时从当前 git 仓库解析，推断失败回退 gitcode-cli/cli）。
- `--files <path>`：将输出写入文件（UTF-8 字节，避免 PowerShell 重定向乱码）。
- 默认自动获取全部分页（`--limit 0` = 不限制）；`--limit N` 限制总数，`--page N` 指定单页，`--paginate` 显式全量。
- 端点：`GET https://web-api.gitcode.com/api/v2/projects/{project}/actions/plugins/all`。
- 请求头含 `Referer: https://gitcode.com/`，满足 Web API 同站校验。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 空结果输出 `[]`（JSON）或 `No actions plugins found`（文本）。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误；`5` 资源冲突。

---

### actions plugin view - 查看插件详情和 README

查看指定官方 GitCode Actions 插件的元信息、版本及 Markdown 使用说明。`view` 有别名 `show`。

```bash
# 查看插件 README
gc actions plugin view checkout -R owner/repo

# 使用 show 别名
gc actions plugin show checkout -R owner/repo

# 输出为 JSON（保留完整 API 字段）
gc actions plugin view checkout -R owner/repo --json

# 写入文件（避免 shell 编码问题）
gc actions plugin view checkout -R owner/repo --json --files detail.json
```

说明：

- 支持 `--json`：输出写入 stdout，原样输出完整 API 响应。
- `-R`：仓库（owner/repo，可选，缺省时从当前 git 仓库解析，推断失败回退 gitcode-cli/cli）。
- `--files <path>`：将输出写入文件（UTF-8 字节，避免 PowerShell 重定向乱码）。
- 位置参数：插件名称（省略时打印 help）。
- 文本模式默认输出 `vision_content[]` 中各版本的 README。
- 端点：`GET https://web-api.gitcode.com/api/v2/projects/{project}/actions/plugins/detail?name={plugin_name}`。
- 请求头含 `Referer: https://gitcode.com/`，满足 Web API 同站校验。
- 认证复用标准 Bearer header，不通过 `access_token` query 参数暴露 token。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`3` 资源不存在（HTTP 404）；`4` 认证/权限错误；`5` 资源冲突。

---

## 飞书/Lark 命令 (lark)

`gc lark` 通过官方 `lark-cli` 工具与飞书/Lark 集成。gc 以子进程方式委托 `lark-cli`，飞书 OAuth 凭证由 lark-cli 存入操作系统 keychain，gc 不接触飞书令牌。

### 前置要求

- 已安装 Node.js（`npm`/`npx`）。lark-cli 缺失时首次运行 `gc lark *` 子命令会返回明确错误并提示 `gc lark install`。
- 已完成飞书 OAuth 登录：`gc lark install` 安装 lark-cli 后，运行 `lark-cli config init` 与 `lark-cli auth login --recommend`（交互式，需浏览器）。

### lark send - 发送飞书消息

向飞书群、用户或自己发送消息，委托 `lark-cli im +messages-send`。

```bash
# 通知自己（bot → 你，无需群、无外部群限制）
gc lark send --to-self --text "deploy done"

# 发送纯文本到指定群
gc lark send --chat-id oc_xxx --text "deploy done"

# 给指定用户发单聊（按 open_id）
gc lark send --user-id ou_xxx --as bot --text "hi"

# 发送 markdown
gc lark send --chat-id oc_xxx --markdown "## Release\n- v1.2.3 shipped"

# 从文件读取正文（- 表示 stdin）
echo "ci passed" | gc lark send --to-self --body-file -

# 以机器人身份发送到群
gc lark send --chat-id oc_xxx --as bot --text "hello"

# 预览将执行的 lark-cli 调用，不实际发送
gc lark send --to-self --text "hi" --dry-run

# JSON 输出（供 AI 代理/脚本消费）
gc lark send --to-self --text "hi" --json
```

说明：
- 目标三选一（互斥）：`--chat-id <oc_xxx>`（群）、`--user-id <ou_xxx>`（单聊）、`--to-self`（给自己）。三者皆空时回落到默认群（见下）。推荐"每人用自己 bot 给自己发通知"的场景用 `--to-self`：自动从 `lark-cli auth status` 解析当前用户 open_id，默认以 bot 身份发送，不受群成员/外部群限制。
- 默认群（仅在未传目标时生效）解析优先级：环境变量 `GC_LARK_DEFAULT_CHAT_ID` > `~/.config/gc/lark.json` 的 `default_chat_id`。
- 内容来源四选一：`--text` / `--markdown` / `--file <路径>` / `--body-file <路径|->`。`--file` 把文件作为附件转发给 lark-cli。
- 消息正文（`--text`/`--markdown`/`--body-file`）在发送前经 `cmdutil.ScanContentForSecrets` 扫描，若包含当前 `GC_TOKEN`/`GITCODE_TOKEN` 值则拒绝发送。`--file` 附件为二进制路径，不由 gc 扫描内容。
- `--json` 成功写入 stdout、退出码 0；失败写入 stderr 并非零退出。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`4` 认证/权限错误（lark-cli 未登录或令牌失效）。

### lark auth status - 查看登录状态

```bash
gc lark auth status          # 友好输出
gc lark auth status --json   # 透传 lark-cli 的结构化登录状态
```

### lark install - 安装 lark-cli

```bash
gc lark install              # 运行 npx @larksuite/cli@latest install
```

安装后若当前 shell 的 PATH 未刷新，可设置环境变量 `GC_LARK_CLI_BIN` 指向 lark-cli 二进制路径。

### lark doctor - 健康检查

```bash
gc lark doctor               # 检测 lark-cli 安装 + 登录就绪 + 默认群配置
gc lark doctor --json
```

### lark config - 管理默认群

```bash
# 设置默认群
gc lark config set --default-chat oc_xxx

# 查看生效的默认群（受 GC_LARK_DEFAULT_CHAT_ID 覆盖）
gc lark config get
gc lark config get --json
```

默认群持久化在 `~/.config/gc/lark.json`（权限 0600），与 gc 的 host 配置 `config.json` 分离。环境变量 `GC_LARK_DEFAULT_CHAT_ID` 优先于持久化值。

---

## 安装诊断、配置与更新

### doctor install - 诊断安装来源和 PATH 冲突

```bash
gc doctor install
gc doctor install --json
```

- 完全离线，无需认证；不读取或打印 Token。
- 输出当前 `version` / `commit` / `built`、`distribution`、wrapper `entrypoint`、实际 `binary`，以及 `gc` / `gitcode` 在 PATH 中的全部 `candidates` 与 `selected`。
- `distribution` 可为 `npm`、`npm-bootstrap`、`pypi`、`deb`、`rpm`、`homebrew`、`system-package` 或 `archive-or-source`。
- Windows 会报告 PowerShell 内置 `gc`/`Get-Content` alias 风险，并建议使用 `gitcode`，不会建议全局删除系统 alias。
- 只给出 `conflicts` 和 `recommendations`；不会修改 PATH、shell profile、认证配置，也不会调用其他包管理器卸载软件。
- `--json` 只向 stdout 写一个稳定 JSON 对象，适合安装器、CI 与 AI 代理消费。

npm bootstrap 的 Node wrapper 另提供 `gitcode install [--target-dir <directory>] [--no-modify-path]`；`--target-dir` 只在用户显式指定时覆盖默认的用户级安装目录，可用 `gitcode install --help` 查看。

- Windows 在用户显式执行 `install` 后，默认将安装目录置于持久 User PATH 前面并删除同目录重复项；只修改当前用户，不修改 Machine PATH、不提权、不删除或重写其他 PATH 条目，也不调用其他包管理器卸载软件。
- 显式 `--target-dir` 会替换该目录内已有的同名常规文件；不得将其指向 Python Scripts、npm prefix 等由其他包管理器持有的目录。缺少参数值或把下一 flag 当成目录时立即报错。
- `--no-modify-path` 显式跳过 Windows 持久 PATH 修改，并输出中文 Windows 用户环境变量设置步骤；Linux/macOS 接受该参数但原有 PATH/profile 行为不变。
- `npx` 子进程无法修改已经运行的父 PowerShell 环境。当前窗口尚未优先包含目标目录时，安装完成输出必须用中文显性给出 `$env:Path` 刷新命令、关闭全部 PowerShell/Windows Terminal 后重开的替代方式，以及 `gitcode version` 验证步骤。
- Windows 持久化失败不会伪装为 PATH 配置成功；安装器保留已通过校验的二进制，输出中文失败原因、Windows 用户环境变量设置步骤和当前窗口刷新命令。持久写入后的系统广播失败必须单独警告并提示注销后重新登录。
- 目标目录含分号、空字符或不是绝对 Windows 路径时，不得生成任何 PATH 修改命令，只能提示更换目录或直接运行已安装程序。

- Linux/macOS 安装时，如果历史版本留下的 `gitcode` 符号链接解析后精确指向同目录常规 `gc` 文件，bootstrap 会在同一安装事务中自动迁移该别名，无需用户先删除。
- `gc` 主程序目标上的符号链接、指向其他位置的 `gitcode` 链接和其他非常规目标一律拒绝覆盖；Windows 不迁移符号链接。
- 别名迁移保留事务唯一备份；后续二进制校验、健康检查或 metadata 写入失败时，旧 `gc` 与原始链接会按逆序原样回滚。

<a id="gitcode-update"></a>

### update - 检查或更新当前安装渠道

```bash
gc update --check
gc update --check --json
gc update
gc update --json
```

- npm global wrapper 只从官方 `https://registry.npmjs.org` 更新精确包 `@gitcode-cli/cli@<stable latest>`；不会更新其他全局 npm 包，并使用 `--ignore-scripts` 禁止更新包生命周期脚本。
- npm bootstrap 使用安装 manifest 和独立 helper，在当前进程退出后原子替换 `gc` / `gitcode`，下一次启动生效。
- stable 版本不会自动进入 prerelease，也不会降级。
- 更新有 24 小时 TTL、跨进程锁、`version --json` 健康检查与失败回滚；后台失败不会改变刚完成业务命令的退出码，摘要在下次启动写入 stderr 一次。
- updater 子进程使用最小环境白名单，不继承 GitCode/npm/GitHub/云平台凭证或用户 npm registry 配置；仅保留 PATH、系统目录、状态/配置目录、代理和 CA 等运行所需变量。
- `--check` 只查询 stable `latest`，不安装。
- 非 npm 渠道只返回对应包管理器的手工升级说明，不会调用 pip、Homebrew、apt、dnf 或 rpm。
- `--json` 字段为 `status`、`distribution`、`current`、`latest`、`message`；JSON 只写 stdout。

### config get/set - 更新策略配置

```bash
gc config get update.mode
gc config get update.mode --json
gc config set update.mode auto
gc config set update.mode notify
gc config set update.mode off
gc config set update.mode off --json
```

支持的 key：`browser`、`editor`、`pager`、`update.mode`、`default_repo`。前三项保存普通字符串；`update.mode` 仅接受以下枚举值：

- `notify`：默认值；后台检查 stable 新版本，在下一次启动提示 `gitcode update`，不自动安装。
- `auto`：用户明确启用后，后台检查并应用 stable 更新。
- `off`：不联网检查，不自动更新。
- 配置保存在 `~/.config/gc/config.json` 的 `gitcode.com` host 下；`GC_UPDATE_MODE` 优先于文件配置。
- 首次 npm 命令会在 stderr 说明默认策略和退出方式。
- `CI=true`、`--no-interactive`、`--no-update-check`、`GC_NO_UPDATE_CHECK=1` 均禁用隐式后台检查、联网与自动应用；显式 `update` / `update --check` 仍由用户主动控制。

### config list - 列出所有配置

列出全部配置项的当前值及来源（environment/config/default）。纯客户端实现。

```bash
# 列出所有配置
gc config list

# JSON 输出
gc config list --json
```

说明：

- 支持的 key：`browser`、`default_repo`、`editor`、`pager`、`update.mode`。
- 来源标注：`(environment)` = 环境变量覆盖；`(config)` = 配置文件设置；`(default)` = 默认值或未设置。
- `update.mode` 默认值为 `notify`。
- `--json` 输出 `[{key, value, source}]` 数组到 stdout。
- 配置目录：`~/.config/gc/`，环境变量 `GC_CONFIG_DIR` 可覆盖。

### config clear-cache - 清除缓存

清除 CLI 缓存目录中的临时文件（API 缓存、补全脚本等），不影响认证和配置文件。

```bash
gc config clear-cache
```

说明：

- 缓存目录：`~/.config/gc/cache/`（如存在）。受 `GC_CONFIG_DIR` 环境变量影响，与 `config list` 一致。
- 无缓存时输出 "No cache to clear."。
- `auth.json`、`config.json`、`lark.json` 不受影响。

## user 命令 (user)

### user view - 查看用户资料

查看 GitCode 用户资料。无参数时查看当前认证用户，传 `<username>` 查看指定用户。

```bash
# 查看当前用户
gc user view

# 查看指定用户
gc user view <username>

# JSON 输出
gc user view --json
```

说明：

- 无参数时调用 GET /api/v5/user（当前认证用户），传 username 时调用 GET /api/v5/users/{username}。
- `--json` 输出完整用户对象到 stdout。
- 退出码：`0` 成功；`1` 通用错误；`3` 用户不存在（HTTP 404）；`4` 认证/权限错误（HTTP 401/403）。

### user edit - 编辑用户资料

更新当前认证用户的资料。仅更新提供的字段。

```bash
# 更新名称
gc user edit --name "New Name"

# 更新简介和公司
gc user edit --bio "New bio" --company "Acme Inc"
```

说明：

- 支持 flags：`--name`、`--bio`、`--email`、`--company`、`--location`、`--website`。
- 至少提供一个字段，否则报参数错误。
- `--bio` 内容经 secret 扫描（防止误提交 token）。
- `--json` 输出更新后的用户对象。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误（未提供任何字段）；`4` 认证/权限错误（HTTP 401/403）。

## search 命令 (search)

### search repos - 搜索仓库

搜索 GitCode 仓库。

```bash
# 搜索仓库
gc search repos "gitcode"

# JSON 输出
gc search repos "cli tool" --json
```

说明：
- `--json` 输出结果数组到 stdout。
- `--limit`/`--page` 分页。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`4` 认证错误。

### search issues - 搜索 Issues

搜索 GitCode Issues。

```bash
# 搜索 Issues
gc search issues "bug report"

# 限定仓库 + 状态
gc search issues "feature" --repo owner/repo --state open
```

说明：
- `--repo` 限定搜索范围到指定仓库（owner/repo）。
- `--state` 按状态过滤（open/closed）。
- `--limit`/`--page` 分页。
- `--json` 输出结果数组。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`4` 认证错误。

### search users - 搜索用户

搜索 GitCode 用户。

```bash
# 搜索用户
gc search users "developer"

# JSON 输出
gc search users "admin" --json
```

说明：
- `--json` 输出用户对象数组。
- `--limit`/`--page` 分页。
- 退出码：`0` 成功；`1` 通用错误；`2` 参数错误；`4` 认证错误。

## SSH 公钥命令 (ssh-key)

`ssh-key` 管理当前认证用户在 GitCode 注册的 SSH 公钥。所有子命令均要求认证，不读取默认
`~/.ssh` 路径；添加公钥时必须显式传入文件。
文本模式会移除服务端字段中的终端控制字符；`--json` 保留字段值，并由 JSON 编码器转义控制字符。

### ssh-key list - 列出公钥

```bash
gc ssh-key list
gc ssh-key list --limit 50 --page 2
gc ssh-key list --json
```

说明：

- `--limit` 默认 30；`--per-page` 控制 API page size，最大 100；`--page` 从 1 开始。
- 文本输出字段为 ID、标题和创建时间；`--json` 输出完整公钥对象数组。

### ssh-key add - 添加公钥

```bash
gc ssh-key add --title laptop --key ~/.ssh/id_ed25519.pub
gc ssh-key add --title laptop --key key.pub --json
```

说明：

- `--title` 与 `--key` 必填；`--key` 是公钥文件路径，不接受私钥或隐式默认路径。发送前会校验为单行 OpenSSH 公钥格式。
- 文件按用户文本编码读取并去除首尾空白；空文件返回参数错误。
- `--json` 输出服务端返回的公钥对象。

### ssh-key view - 查看公钥

```bash
gc ssh-key view 123
gc ssh-key view 123 --json
```

说明：公钥 ID 必须为正整数。客户端兼容 GitCode 详情接口返回单对象或单元素数组两种响应形状。

### ssh-key delete - 删除公钥

```bash
gc ssh-key delete 123
gc ssh-key delete 123 --dry-run
gc ssh-key delete 123 --dry-run --json
gc ssh-key delete 123 --yes --json
```

说明：

- 删除操作默认要求输入公钥 ID 确认；非 TTY 或 `--no-interactive` 环境必须显式传 `--yes`。
- `--dry-run` 只预览目标，不调用删除 API，也不要求确认。
- `--json` 输出 `{id, action}`；实际删除时 `action` 为 `deleted`，与 `--dry-run` 组合时为 `would-delete`。
- 退出码：`0` 成功；`2` 参数或确认错误；`3` 公钥不存在；`4` 认证/权限错误。

## 其他命令

### browse - 在浏览器中打开 GitCode 资源

在默认浏览器中打开 GitCode 仓库页面。纯客户端实现，无需 API 调用。

```bash
# 打开仓库主页
gc browse -R owner/repo

# 打开 issue
gc browse 42 -R owner/repo

# 打开 pull request
gc browse 42 --pr -R owner/repo

# 打开指定页面（如 releases、issues、wiki）
gc browse releases -R owner/repo

# 打开分支页面
gc browse --branch feature -R owner/repo

# 打开 commit 页面
gc browse --commit abc123 -R owner/repo

# 仅打印 URL 不打开浏览器（脚本/AI 代理用）
gc browse --no-browser -R owner/repo
```

说明：

- 无参数时打开仓库主页；传数字时打开 issue/PR 页面（`--pr` 指定 PR）；传路径时打开对应页面。
- `--branch` 和 `--commit` 优先级高于位置参数。
- TTY 环境调用 `pkg/browser` 打开浏览器；非 TTY 环境（管道、`--no-interactive`）或 `--no-browser` 仅输出 URL 到 stdout，不阻塞。
- 退出码：`0` 成功；`2` 参数错误；`1` 其他错误。

### version - 显示版本

```bash
gc version

# 输出 JSON
gc version --json
```

Command-name note:
- When the CLI is launched as `gitcode` or `gitcode.exe`, `version`, `help`, `help --json`, `schema`, and shell completion output use `gitcode` as the command name.
- When the CLI is launched as `python -m gc_cli`, output uses `gitcode` as the command name.
- When the CLI is launched as `gc` or `gc.exe`, output continues to use `gc`.
- DEB/RPM packages install `/usr/bin/gitcode` as an alias of `/usr/bin/gc`; on Linux both commands are equivalent.

### help - 帮助

```bash
# 显示帮助
gc help

# 显示命令帮助
gc help issue
gc help issue create

# 搜索命令（按关键词搜索）
gc help --search pr
gc help --search issue

# 列出所有主题
gc help --topics

# 按主题过滤命令
gc help --topic pull-requests
gc help --topic issues
```

说明：
- `--search` 按关键词搜索命令名称、路径、描述和别名
- `--topics` 列出所有已定义的主题分类
- `--topic` 显示指定主题下的所有命令

```bash
# 输出 JSON
gc help --json
```

### schema - 命令元数据

```bash
# 输出完整命令树
gc schema

# 输出单个命令的元数据
gc schema "issue view"
```

说明：
- 对带预定义取值的 flag，schema 会在 `enum` 字段中暴露合法值，例如 `format`、`time-format`、`method`、部分 `state`/`sort`/`direction` flag。

---

## 常用选项

| 选项 | 说明 |
|------|------|
| `-R, --repo owner/repo` | 指定仓库 |
| `--help` | 显示帮助 |
| `--limit N` | 限制结果数量 |
| `--web` | 在浏览器中打开 |
| `--json` | 输出结构化 JSON |
| `--dry-run` | 预演写操作而不执行 |
| `--no-interactive` | 禁用所有交互提示；破坏性操作需配合 `--yes` |

---

## 环境变量

| 变量 | 说明 |
|------|------|
| `GC_TOKEN` | 认证 Token |
| `GITCODE_TOKEN` | 备用 Token |
| `GC_HOST` | 默认 GitCode 主机（默认：gitcode.com）；必须是受信 hostname-only 值；已接入共享 host-aware 认证入口的业务命令会映射到对应 API 主机，且非默认 host 只使用该 host 的本地登录 token |
| `GC_TIMEOUT` | API 请求超时时间（默认：30s）；支持 Duration 格式如 `60s`、`2m`，或纯秒数如 `120` |
| `GC_DEBUG` | 启用 API 调试日志，输出重试、Rate Limit 等信息到 stderr |
| `GC_API_DEBUG` | 同 `GC_DEBUG`，启用 API 调试日志 |
| `NO_COLOR` | 禁用颜色输出 |
| `GC_LARK_CLI_BIN` | 覆盖 lark-cli 二进制路径，优先于 PATH 查找（用于安装后 PATH 未刷新的场景） |
| `GC_LARK_DEFAULT_CHAT_ID` | 覆盖 `gc lark send` 的默认飞书群 id，优先于 `~/.config/gc/lark.json` |
| `GC_UPDATE_MODE` | 覆盖 npm 更新模式：`auto`、`notify` 或 `off` |
| `GC_NO_UPDATE_CHECK` | 设为 `1` / `true` 时禁用本次调用的 npm 自动更新检查 |
| `GC_STATE_DIR` | 覆盖 npm 更新锁、TTL、日志摘要状态目录，主要用于测试或受控环境 |

---

## 已知限制

以下功能受 GitCode API 限制，可能无法正常工作：

| 功能 | 限制说明 |
|------|----------|
| `repo fork` | 仓库路径已按用户输入解析，但 GitCode API 在部分仓库上仍可能返回 `400 Bad Request` |
| `milestone create/view` | 返回 400 错误，API 可能不支持 |
| `release delete` | GitCode 官方 OpenAPI 当前未提供 Release 删除接口；命令会返回平台的 `405 Method Not Allowed`，请在仓库 Release 页面删除 |

---

## 文档维护规范

**重要**：每次修改命令相关代码时，必须同步更新本文档！

### 同步更新要求

| 代码改动类型 | 需要更新的文档 |
|------------|--------------|
| 新增命令 | docs/COMMANDS.md、README.md |
| 新增子命令 | docs/COMMANDS.md |
| 修改命令参数/flags | docs/COMMANDS.md、README.md |
| 修改命令行为 | docs/COMMANDS.md |
| 删除命令 | docs/COMMANDS.md、README.md |

### 更新检查清单

开发完成后，确认以下检查项：

- [ ] 新命令已添加到 docs/COMMANDS.md
- [ ] README.md 命令概览已更新（如有新命令）
- [ ] 命令示例已验证可执行
- [ ] 参数说明与代码实现一致
- [ ] 已知限制表已更新（如有新的 API 限制）

### 常见问题

**Q: 如何确认文档与代码一致？**
```bash
# 查看所有命令
gc help

# 查看具体命令帮助
gc pr --help
gc issue --help
```

**Q: 文档更新顺序？**
1. 先更新 docs/COMMANDS.md（完整文档）
2. 再更新 README.md（概览文档）

---

**最后更新**: 2026-06-26
