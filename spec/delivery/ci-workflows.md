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

- GitCode 原生 CI 在 GitCode PR 提交到 `main` 时自动触发，AI 通过 `gc actions` 查看结果
- GitHub 镜像 CI 在 GitHub PR 提交到 `main` 时自动触发，AI 通过 `gh` 查看结果
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
  → 远端 CI 验证（PR 自动触发，AI 通过 gc / gh CLI 查看结果）
  → PR 门禁（自检证据 + CI 结果）
  → 合并门禁（独立评审 + 人工确认）
```

CI 不替代任何现有门禁层，只在 PR 提交时自动运行跨平台验证。

### 1.2 运行平台

CI 同时运行在两个平台：

- **GitCode Actions**：GitCode 主仓 `gitcode.com/gitcode-cli/cli` 的原生 Linux CI，是 GitCode PR 的首要自动化证据
- **GitHub Actions**：GitHub 镜像仓 `github.com/gitcode-cli/cli` 的跨平台 CI，继续覆盖 Linux、macOS 和 Windows

GitCode 原生 CI 当前只覆盖 Linux；macOS 和 Windows 兼容性仍以 GitHub Actions 结果为准。

### 1.3 工具链

| 平台 | 自动触发 | 查看运行 | 查看 Job / 日志 |
|------|---------|---------|-----------------|
| GitCode Actions | GitCode PR 提交/更新到 `main` | `gc actions run list -R gitcode-cli/cli --pr <pr> --json` | `gc actions job list/view/log` |
| GitHub Actions | GitHub PR 提交/更新到 `main` | `gh run list --workflow=ci.yml` | `gh run view --log` |

GitCode 平台操作固定使用 `gc`，GitHub 镜像仓操作使用 `gh`。

---

## 2. CI Job 定义

### 2.1 Job 概览

GitCode 原生工作流 `.gitcode/workflows/ci.yml` 对齐 GitHub CI 的 Linux 路径：

| Job | 运行环境 | 内容 | 对应质量门禁 |
|-----|---------|------|-------------|
| `lint` | codearts-hosted / ubuntu-latest / x64 / small | golangci-lint v2.12.2 | 代码规范检查（`coding-standards.md`） |
| `security` | codearts-hosted / ubuntu-latest / x64 / small | 浅克隆检测 + unshallow 后 gitleaks 全历史密钥扫描（`--redact=100` 脱敏输出） | 凭证泄漏扫描（`security.md`） |
| `test` | codearts-hosted / ubuntu-latest / x64 / small | release / package 版本校验 + `go test -v -race -coverprofile` + npm wrapper 单测 + 覆盖率制品 | 发布输入脚本回归 + Go/Node 单元测试 + 竞态检测 + 覆盖率（`testing-guide.md`） |
| `build` | codearts-hosted / ubuntu-latest / x64 / small | Linux `go build` + `gc version` + 二进制制品 | Linux 构建验证（`build-and-package.md`） |
| `package` | codearts-hosted / ubuntu-latest / x64 / small | 补全生成 + Linux 二进制 + wheel 构建、三入口冒烟与制品上传 | 打包与 wheel 入口验证 |

GitHub 工作流 `.github/workflows/ci.yml` 保留原有跨平台覆盖：

| Job | 运行环境 | 内容 |
|-----|---------|------|
| `lint` | ubuntu-latest | golangci-lint |
| `secret-scan` | ubuntu-latest | gitleaks 全历史密钥扫描（`fetch-depth: 0` + `--redact=100` 脱敏输出） |
| `test` | ubuntu-latest / macos-14 / windows-latest | release / package 版本校验 + 宿主机 setup 契约 + Go/Node 单元测试 + 竞态检测 + 真实旧 npm→本地新包升级、PATH 遮蔽与 bootstrap 烟测 + 覆盖率 |
| `build` | ubuntu-latest / macos-14 / windows-latest | 跨平台 `go build` + `gc version` |
| `docker` | ubuntu-latest | Docker 构建 + shell 补全 + wheel 入口冒烟 |

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

---

## 3. AI 监控流程

### 3.1 触发机制

两个平台的 CI 均在以下情况下**自动触发**（无需人工或 AI 手动操作）：

1. 对应平台的 PR 提交到 `main` 分支
2. 对应 PR 的源分支推送新 commit

触发配置：

- GitCode：`.gitcode/workflows/ci.yml` 的 `on.pull_request.branches: [main]`
- GitHub：`.github/workflows/ci.yml` 的 `on.pull_request.branches: [main]`

GitCode 日常 CI 使用最小仓库只读权限 `permissions: repository: read`；GitHub 日常 CI 使用
`permissions: contents: read`。发布权限仍由 `.github/workflows/release.yml` 单独管理。

### 3.2 查看 CI 结果

```bash
# GitCode：查看 PR 关联的运行、详情和 Jobs
gc actions run list -R gitcode-cli/cli --pr <pr-number> --workflow "CI" --json
gc actions run view <run-id> -R gitcode-cli/cli --json
gc actions job list <run-id> -R gitcode-cli/cli --json

# GitHub：查看镜像 PR 分支的最新 CI 运行
gh run list --workflow=ci.yml --branch <pr-branch> --limit 1

# GitHub：实时等待最新 CI 完成
gh run watch $(gh run list --workflow=ci.yml --branch <pr-branch> --limit 1 --json databaseId --jq '.[0].databaseId')

# GitHub：查看 CI 结论
gh run view <run-id> --json conclusion --jq '.conclusion'
```

`gc actions run list --json` 返回的 `workflow_run_id` 是后续 `run view` 和 `job list` 的 `<run-id>`。
GitCode CLI 当前不提供 watch 子命令；需要等待时，按合理间隔重复读取 `run list` 或 `run view`，不得高频轮询。

### 3.3 失败处理

CI 失败时，AI 必须：

1. 获取失败 Job 的详细日志：
   - GitCode：先用 `gc actions job list <run-id> -R gitcode-cli/cli --json` 取 `<job-id>`，再执行
     `gc actions job log <run-id> <job-id> -R gitcode-cli/cli --output job-log.zip`
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
  - Run URL: https://github.com/gitcode-cli/cli/actions/runs/<run-id>
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

### 5.4 GitCode 工作流契约回归

`tests/contracts/ci_workflow_test.go` 使用 YAML 解析器校验 GitCode 日常 CI 的语义契约，包括：

- 只包含 `lint`、`security`、`test`、`build`、`package` 五个 Job
- `build` 和 `package` 依赖 `test`，且不依赖 Docker daemon
- `package` 使用官方 `setup-python` 并覆盖 wheel 三入口冒烟
- `test` 使用官方 `setup-node` 并执行 dependency-free npm wrapper 单测；GitHub 三平台额外安装已发布旧版本到隔离 prefix，再用本地 tarball 升级，不得污染 runner 全局 prefix
- coverage、binary、wheel 制品通过运行时 `ATOMGIT_RUN_ID` 和 step output 唯一化

该契约测试随 `go test ./...` 在本地和两个 CI 平台执行。修改 `.gitcode/workflows/ci.yml` 时必须同步更新契约测试；
不得通过删除或放宽断言绕过已记录的托管 runner 约束。

---

## 6. Release CI

`.github/workflows/release.yml` 用于版本发布构建，不属于日常开发 CI。

触发方式：AI 在发布流程中通过 `gh workflow run release.yml -f version=vX.Y.Z` 触发。

Release CI 规范详见 `spec/delivery/release-process.md`。

---

## 下一步去看哪里

- CI 不通过的修复流程：看 [测试流程](../workflows/test-workflow.md)
- CI 结果如何影响合并：看 [代码质量门禁规范](../foundations/code-quality-gates.md)
- Release CI 详情：看 [发布流程规范](./release-process.md)
- AI 如何编排 CI：看 [AI 本地开发流程](../workflows/ai-local-development-workflow.md)

---

**最后更新**: 2026-09-15
