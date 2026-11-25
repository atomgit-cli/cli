# CI 工作流规范

本文件定义 gitcode-cli 的远端 CI 工作流规范，包括 CI 定位、触发方式、Job 映射、AI 编排流程和证据纳入规则。

## 职责

- 定义远端 CI 在项目质量门禁体系中的定位
- 定义 AI 如何通过 `gc` 和 `gh` CLI 监控 CI 结果
- 定义 CI Job 与 `spec/foundations/code-quality-gates.md` 的映射关系
- 定义 CI 结果如何纳入 PR 自检证据

## 适用场景

- AI 协作者在本地验证完成后、进入自检前，核验远端 CI
- 跨平台（Linux/macOS/Windows）构建与测试验证
- PR 自检中引用 CI 结果作为自动化证据

## 必须

- GitCode 原生 CI 在 GitCode PR 提交到 `main`、或 `main` 收到合并/直推时自动触发，AI 通过 `gc actions` 查看结果
- GitHub 镜像 CI 在 GitHub PR 提交到 `main`、或镜像 `main` 收到同步推送时自动触发，AI 通过 `gh` 查看结果
- CI 结果必须纳入 PR 自检证据
- CI 失败时不得进入 `status/self-checked`
- CI Job 定义变更时同步本文件

## 禁止

- 把 CI 通过当作跳过本地验证的理由
- 把 CI 通过当作独立执行主体评审的替代
- 在 CI 运行中或失败时宣称"开发完成"
- 修改 CI 定义绕过本文件规定的门禁映射

## 同步要求

- CI Job 变化时同步本文件和 `spec/foundations/code-quality-gates.md`
- CI 触发流程变化时同步 `spec/workflows/ai-local-development-workflow.md`

## 不负责什么

- 本地构建与单元测试（由本地门禁负责）
- 独立执行主体语义审查（由评审流程负责）
- 真实命令验证（由 `infra-test/*` 负责）

---

## 1. CI 定位

### 1.1 在质量门禁体系中的层级

远端 CI 位于本地门禁和 PR 门禁之间，作为**自动化证据补充层**：

```
本地开发门禁（必须，不可跳过）
  → 推送分支 + 创建 PR
  → 远端 CI 验证（PR 与 main 推送自动触发，AI 通过 gc / gh CLI 查看结果）
  → PR 门禁（自检证据 + CI 结果）
  → 合并门禁（独立评审 + 人工确认）
```

CI 不替代任何现有门禁层，在 PR 提交与 `main` 合并/直推时自动运行跨平台验证，
使主干时点拥有独立的单测与构建证据，不再仅依赖 PR 时点。

### 1.2 运行平台

CI 同时运行在两个平台：

- **GitCode Actions**：GitCode 主仓 `gitcode.com/atomgit-cli/cli` 的原生 Linux CI，是 GitCode PR 的首要自动化证据
- **GitHub Actions**：GitHub 镜像仓 `github.com/atomgit-cli/cli` 的跨平台 CI，继续覆盖 Linux、macOS 和 Windows

GitCode 原生 CI 当前只覆盖 Linux；macOS 和 Windows 兼容性仍以 GitHub Actions 结果为准。

### 1.3 工具链

| 平台 | 自动触发 | 查看运行 | 查看 Job / 日志 |
|------|---------|---------|-----------------|
| GitCode Actions | GitCode PR 提交/更新到 `main`；`main` 合并/直推 | `gc actions run list -R atomgit-cli/cli --pr <pr> --json` | `gc actions job list/view/log` |
| GitHub Actions | GitHub PR 提交/更新到 `main`；镜像 `main` 同步推送 | `gh run list --workflow=ci.yml` | `gh run view --log` |

GitCode 平台操作固定使用 `gc`，GitHub 镜像仓操作使用 `gh`。

---

## 2. CI Job 定义

### 2.1 Job 概览

GitCode 原生工作流 `.gitcode/workflows/ci.yml` 对齐 GitHub CI 的 Linux 路径：

| Job | 运行环境 | 内容 | 对应质量门禁 |
|-----|---------|------|-------------|
| `lint` | codearts-hosted / ubuntu-latest / x64 / small | golangci-lint v2.12.2 | 代码规范检查（`coding-standards.md`） |
| `security` | codearts-hosted / ubuntu-latest / x64 / small | 浅克隆检测 + unshallow 后 gitleaks 全历史密钥扫描（`--redact=100` 脱敏输出） | 凭证泄漏扫描（`security.md`） |
| `test` | codearts-hosted / ubuntu-latest / x64 / small | release / package 版本校验 + `go test -v -race -coverprofile` + npm wrapper 单测 + Python 单测（wrapper、AI 记录校验器、远端事实核验）+ 覆盖率制品 | 发布输入脚本回归 + Go/Node/Python 单元测试 + 竞态检测 + 覆盖率（`testing-guide.md`） |
| `build` | codearts-hosted / ubuntu-latest / x64 / small | Linux `go build` + `gc version` + 二进制制品 | Linux 构建验证（`build-and-package.md`） |
| `package` | codearts-hosted / ubuntu-latest / x64 / small | 补全生成 + Linux 二进制 + wheel 构建、三入口冒烟与制品上传 | 打包与 wheel 入口验证 |

GitHub 工作流 `.github/workflows/ci.yml` 保留原有跨平台覆盖：

| Job | 运行环境 | 内容 |
|-----|---------|------|
| `lint` | ubuntu-latest | golangci-lint |
| `secret-scan` | ubuntu-latest | gitleaks 全历史密钥扫描（`fetch-depth: 0` + `--redact=100` 脱敏输出） |
| `test` | ubuntu-latest / macos-latest / windows-latest | release / package 版本校验 + 宿主机 setup 契约 + Go/Node 单元测试 + 竞态检测 + 真实旧 npm→本地新包升级、PATH 遮蔽与 bootstrap 烟测 + Python 单测（wrapper、AI 记录校验器、远端事实核验） + 生成资产 drift 检查（completions/ 先从新构建重新生成再一并校验）（ubuntu） + 覆盖率 |
| `build` | ubuntu-latest / macos-latest / windows-latest | 跨平台 `go build` + `gc version` |
| `docker` | ubuntu-latest | Docker 构建 + shell 补全 + wheel 入口冒烟 |

另有独立定时工作流 `.gitcode/workflows/artifact-cleanup.yml`，只负责按保留期清理
Actions 制品，不属于日常 CI Job，见 §2.4。

macOS 标签使用 `macos-latest`：`macos-14` 镜像已被 GitHub Actions 于 2026-11-02 官方退役
（GitHub actions/runner-images 仓库 15226 号公告，Intel 与 arm64 均不延期），继续使用会使
macOS 任务在队列阶段即被取消（2026-11-09 实测确认，迁移记录见 issue 639）。`macos-latest`
现指向 macOS 26：其新 dyld 会拒绝加载缺少 LC_UUID 加载命令的 Go 测试二进制（报
`dyld: missing LC_UUID load command` 后 abort），该链接器修复在 1.25.9 backport 与 1.26/1.27
线均含（golang/go 仓库 78056 号 issue）。

CI 与发布工具链统一钉受支持线（issue 640）：镜像仓 CI 与 GitCode CI 全部 job 钉
`go-version: '1.26'`（GitCode 侧 setup-go 的输入键为 `version`），两侧各有契约测试钉住
版本防静默回退；release 构建钉精确补丁版 `go-version: '1.26.10'`。本地开发下限仍为
1.22+（`go.mod` 语言版本 `go 1.22.0` 不变，见 build-and-package.md 与 CONTRIBUTING.md）。

GitCode 托管 runner 不提供 Docker daemon，因此 Docker 构建由 GitHub Linux `docker` Job 承担。GitCode
原生工作流生成 `coverage.out`、Linux 二进制和 wheel，并使用 run ID 隔离制品名称；它不复用 GitHub
仓的 `CODECOV_TOKEN`，Codecov 外部上报仍由 GitHub Linux `test` Job 承担。

### 2.2 Job 依赖关系

```
lint      security      test   （三者并行，security 无下游依赖）
                          ├──→ build
                          └──→ package
```

- `lint`、`security` 和 `test` 并行启动
- GitCode `build`、`package` 等待 `test` 通过后执行（`security` 无下游依赖）
- 任何 Job 失败即整体 CI 失败

### 2.3 与质量门禁的映射

| 质量门禁要求 | GitCode 原生 CI | GitHub 镜像 CI |
|-------------|----------------|----------------|
| `go test ./...` | Linux `test`（`-race`） | 3 OS `test`（`-race`） |
| release workflow 版本输入校验 | Linux `test` | 3 OS `test` |
| 宿主机 setup 安装、幂等、隔离与防覆盖契约 | 不覆盖 | 3 OS `test` 运行 `scripts/test-dev-setup.*` |
| `go build` | Linux `build` | 3 OS `build` |
| 格式/规范检查 | Linux `lint` | Linux `lint` |
| Docker / wheel 入口 | Linux `package` 覆盖 wheel；不覆盖 Docker | Linux `docker` |
| 凭证泄漏扫描 | Linux `security`（gitleaks 全历史） | Linux `secret-scan`（gitleaks 全历史） |
| 跨平台兼容 | 不覆盖 | ubuntu / macOS / Windows |
| npm 安装升级 | Linux wrapper 单测 | 三平台真实 global 升级 + bootstrap + doctor 冲突诊断 |

CI **不覆盖**的质量门禁（仍需本地或人工执行）：

- 真实命令验证（`infra-test/*`）
- 安全审查（语义层面的敏感信息检查、`.gitleaksignore` 变更评审；模式匹配型凭证扫描已由 Secret Scan job 覆盖）
- 文档同步检查
- 工作区卫生检查
- 独立执行主体语义审查

### 2.4 制品保留与清理

GitCode Actions 的 `upload-artifact` 接受 `retention-days`，但它不是硬性过期：2026-09-14
把三个上传步设成 `retention-days: 7` 后，2026-11-13 仍残留 34 天内产出的 411 个制品
（约 1.07 GiB），直接打满命名空间 1 GiB 配额，`Package` Job 报
`Namespace artifact quota exceeded: ... currentUsed=1076816552, max=1073741824`
失败（issue 666）。2026-11-25 复核：API 已会为每个制品返回 `expires_at = created_at + 7 天`，
但历史制品并未按该窗口被清理，因此配额安全不能建立在平台过期语义上。保留策略由上传侧与兜底侧两段组成：

- **上传侧**：`.gitcode/workflows/ci.yml` 的 coverage / binary / wheel 三件套（实测约 8.9 MiB/次）
  一律 `retention-days: 2`，不依赖平台兑现过期语义。
- **兜底侧**：`.gitcode/workflows/artifact-cleanup.yml` 每 6 小时（`0 */6 * * *`）与手动
  触发时运行 `scripts/cleanup-artifacts.py`，用 `gc actions artifact list --paginate`
  + `gc actions artifact delete --yes` 删除 `created_at` 早于 `RETENTION_DAYS`
  （默认 2 天）的制品，删除失败即令 job 失败。清理频率不能退回每日：清理前那一刻的
  存量是「窗口 + 间隔」，每日清理在峰值流量下会重新逼近配额。

配额换算（2026-11-25 实测单次三件套 8.94 MiB；646 次运行历史：峰值 43 次/天、p90 20 次/天、
中位数 7 次/天）：峰值日产量约 384 MiB，2 天窗口 + 6 小时间隔 ≈ 2.25 天 ≈ 865 MiB（84% 配额）；
p90 约 403 MiB（39%），中位数约 141 MiB（14%）。同样峰值下每日清理为 3 天 ≈ 1.15 GiB（超配额），
7 天窗口约 3 GiB，因此窗口与频率必须成对调整。
`created_at` 在生产 API 中是 RFC 3339 字符串（`2026-11-25T10:25:46+08:00`），脚本同时兼容
旧毫秒时间戳，避免格式变化导致全部制品被判为“时间不可用”而永不清理。两处窗口（上传
`retention-days` 与清理 `RETENTION_DAYS`）以及 6 小时间隔上限由
`tests/contracts/cleanup_workflow_test.go` 钉住，任一侧漂移即契约失败。

清理工作流只在默认分支生效，触发面固定为 `schedule` + `workflow_dispatch`，权限固定
`repository: read` + `action: write`。令牌使用 runner 自动注入的 `ATOMGIT_TOKEN`：步骤内
`export GC_TOKEN="$ATOMGIT_TOKEN"`；**不得**改写 `secrets.ATOMGIT_TOKEN`，该键不存在，
映射后会以空 token 覆盖注入值。

**重跑同名冲突的处置口径**：`gc actions run rerun` 复用原 run ID，`coverage-linux-<run-id>`、
`gc-ubuntu-<run-id>`、`gc-wheel-<run-id>` 会与首轮残留同名冲突，因此重跑前先删除该 run 的残留制品：

```bash
gc actions artifact list -R atomgit-cli/cli --run <run-id> --json
gc actions artifact delete <artifact-id> -R atomgit-cli/cli --yes
gc actions run rerun <run-id> -R atomgit-cli/cli --yes
```

配额已满导致 `Package` 失败时，先手动触发清理再重跑：

```bash
gc actions workflow run artifact-cleanup.yml -R atomgit-cli/cli --ref main
gc actions artifact list -R atomgit-cli/cli --paginate --per-page 100 --json
```

---

## 3. AI 监控流程

### 3.1 触发机制

两个平台的 CI 均在以下情况下**自动触发**（无需人工或 AI 手动操作）：

1. 对应平台的 PR 提交到 `main` 分支
2. 对应 PR 的源分支推送新 commit
3. `main` 分支收到合并提交或直推（push 触发，为合入尖端的每次变更留下独立 CI 证据）

触发配置：

- GitCode：`.gitcode/workflows/ci.yml` 的 `on.pull_request.branches: [main]` 与 `on.push.branches: [main]`
- GitHub：`.github/workflows/ci.yml` 的 `on.pull_request.branches: [main]` 与 `on.push.branches: [main]`

GitCode 日常 CI 使用最小仓库只读权限 `permissions: repository: read`；GitHub 日常 CI 使用
`permissions: contents: read`。发布权限仍由 `.github/workflows/release.yml` 单独管理。

### 3.2 查看 CI 结果

```bash
# GitCode：查看 PR 关联的运行、详情和 Jobs
gc actions run list -R atomgit-cli/cli --pr <pr-number> --workflow "CI" --json
gc actions run view <run-id> -R atomgit-cli/cli --json
gc actions job list <run-id> -R atomgit-cli/cli --json

# GitHub：查看镜像 PR 分支的最新 CI 运行
gh run list --workflow=ci.yml --branch <pr-branch> --limit 1

# GitHub：实时等待最新 CI 完成
gh run watch $(gh run list --workflow=ci.yml --branch <pr-branch> --limit 1 --json databaseId --jq '.[0].databaseId')

# GitHub：查看 CI 结论
gh run view <run-id> --json conclusion --jq '.conclusion'

# GitCode：查看 main 分支 push 触发的运行记录（issue 618 验收口径）
gc actions run list -R atomgit-cli/cli --branch main --event Push --json

# GitHub：查看 main 分支 push 触发的运行记录
gh run list --workflow=ci.yml --branch main --limit 5
```

`gc actions run list --json` 返回的 `workflow_run_id` 是后续 `run view` 和 `job list` 的 `<run-id>`。
GitCode CLI 当前不提供 watch 子命令；需要等待时，按合理间隔重复读取 `run list` 或 `run view`，不得高频轮询。

### 3.3 失败处理

CI 失败时，AI 必须：

1. 获取失败 Job 的详细日志：
   - GitCode：先用 `gc actions job list <run-id> -R atomgit-cli/cli --json` 取 `<job-id>`，再执行
     `gc actions job log <run-id> <job-id> -R atomgit-cli/cli --output job-log.zip`
   - GitHub：`gh run view <run-id> --log --job=<job-id>`
2. 分析根因（代码问题 vs 环境问题 vs 偶发问题）
3. 修复后重新推送并重新触发 CI
4. 在 PR 自检中记录 CI 失败与修复过程

如果 CI 失败原因是环境/平台偶发问题（非代码问题），可在自检中明确说明，仍可继续推进流程。

---

## 4. CI 证据纳入自检

### 4.1 最小 CI 证据

PR 作者自检中至少包含：

- 平台（GitCode / GitHub）
- CI run ID 或 run URL
- GitCode PR 对应的 head SHA
- CI 结论（GitCode 使用 `COMPLETED` / `FAILED` 等状态，GitHub 使用 `success` / `failure`）
- 各 Job 状态摘要
- 如 CI 失败，失败原因和修复记录

### 4.2 自检模板中的 CI 条目

```markdown
## CI 验证

- GitCode Actions:
  - PR head SHA: `<sha>`
  - Run ID: `<run-id>`
  - 结论: COMPLETED
  - lint: ✅
  - security: ✅
  - test: ✅
  - build: ✅
  - package: ✅
- GitHub Actions:
  - Run URL: https://github.com/atomgit-cli/cli/actions/runs/<run-id>
  - 结论: success
  - test/build (ubuntu, macOS, Windows): ✅
  - lint/secret-scan/docker (ubuntu): ✅
```

### 4.3 CI 未执行的处理

如果因以下原因未执行 CI，必须在自检中明确说明：

- docs-only 改动（写明"不涉及代码路径，已跳过 CI"）
- GitCode Actions 暂不可用（写明 `gc actions` 返回的具体错误）
- GitHub 镜像仓不可达（写明具体错误）
- 其他合理原因（需明确记录）

---

## 5. 约束与边界

### 5.1 CI 通过 ≠ 可以合并

CI 通过只表示自动化检查无问题。以下事项仍需独立完成：

- 真实命令验证（`infra-test/*`）
- 安全审查
- 文档同步
- 独立执行主体评审
- 高风险改动的人工最终确认

### 5.2 CI 不定义新门禁

CI 是现有质量门禁的自动化实现，不得引入高于 `spec/foundations/code-quality-gates.md` 的额外要求。

### 5.3 CI 配置变更

修改 `.gitcode/workflows/ci.yml` 或 `.github/workflows/ci.yml` 的行为等同于修改构建/测试门禁，必须：

- 同步更新本文件中的 Job 描述和映射表
- 在 PR 中说明变更理由
- 变更后至少在对应平台成功运行一次 CI 作为自验证

### 5.4 CI 工作流契约回归

`tests/contracts/ci_workflow_test.go` 使用 YAML 解析器校验日常 CI 的语义契约，包括：

- 只包含 `lint`、`security`、`test`、`build`、`package` 五个 Job（GitCode 侧）
- test 的 `-race` 标志（与镜像侧对等）
- security 的 gitleaks `--redact=100` 全量脱敏输出（与镜像侧对等，命中内容不得进入公开 CI 日志）与浅克隆 unshallow 补全（失败须令 job 失败，等价于镜像侧 `fetch-depth: 0` 的全历史保证）
- `build` 和 `package` 依赖 `test`，且不依赖 Docker daemon
- `package` 使用官方 `setup-python` 并覆盖 wheel 三入口冒烟
- `test` 使用官方 `setup-node` 并执行 dependency-free npm wrapper 单测；GitHub 三平台额外安装已发布旧版本到隔离 prefix，再用本地 tarball 升级，不得污染 runner 全局 prefix
- coverage、binary、wheel 制品通过运行时 `ATOMGIT_RUN_ID` 和 step output 唯一化
- 双端 CI 的 `main` 触发面（`pull_request` 与 `push` 均限 `main`）被钉扎，任一侧静默丢失触发器即契约失败（`TestMirrorWorkflowPinsMainTriggers` 对 `.github/workflows/ci.yml` 校验同一触发契约）

`tests/contracts/cleanup_workflow_test.go` 钉住制品清理工作流的不变量：

- 触发面只有 `schedule`（单条五段 cron，且清理间隔不超过 6 小时）与 `workflow_dispatch`，不得挂 `push` / `pull_request`
- 权限固定 `repository: read` + `action: write`，不得扩权
- `RETENTION_DAYS` 与 `ci.yml` 三个上传步的 `retention-days` 完全一致（任一侧漂移即失败）
- 清理步骤必须 `export GC_TOKEN="$ATOMGIT_TOKEN"`，且不得改写不存在的 `secrets.ATOMGIT_TOKEN`

`tests/contracts/mirror_workflow_test.go` 另对 GitHub 镜像工作流钉结构不变量：

- 五 Job 集合（lint/secret-scan/test/build/docker）
- test 与 build 的三平台矩阵（ubuntu/macOS/Windows——镜像仓是后两者兼容性的唯一证据源），且 test 矩阵须 `fail-fast: false`（失败平台不得静默取消其余平台运行）
- build/docker 依赖 test
- test 的 `-race` 标志
- secret-scan 的 `fetch-depth: 0` 全历史扫描与 gitleaks `--redact=100` 全量脱敏输出（命中内容不得进入公开 CI 日志）
- workflow 级 `contents: read` 权限，且任何 job 级权限不得超过 `contents: read`
- 全部 action SHA 固定

Job 内步骤序列、env 细节、codecov/npm 冒烟内部与 docker 构建参数为显式声明的未覆盖维度（行为细节由 CI 运行本身覆盖）。

该契约测试随 `go test ./...` 在本地和两个 CI 平台执行。修改任一 CI 工作流（`.gitcode/workflows/ci.yml` 或 `.github/workflows/ci.yml`）的触发面或结构不变量时，必须同步更新契约测试；
不得通过删除或放宽断言绕过已记录的托管 runner 约束。

---

## 6. Release CI

`.github/workflows/release.yml` 用于版本发布构建，不属于日常开发 CI。

触发方式：AI 在发布流程中通过 `gh workflow run release.yml -f version=vX.Y.Z` 触发。
preflight 阶段执行 `go test ./...`，使发布在 main CI 缺失或失败时仍有单测门禁。

Release CI 规范详见 `spec/delivery/release-process.md`。

---

## 下一步去看哪里

- CI 不通过的修复流程：看 [测试流程](../workflows/test-workflow.md)
- CI 结果如何影响合并：看 [代码质量门禁规范](../foundations/code-quality-gates.md)
- Release CI 详情：看 [发布流程规范](./release-process.md)
- AI 如何编排 CI：看 [AI 本地开发流程](../workflows/ai-local-development-workflow.md)

---

**最后更新**: 2026-11-25
