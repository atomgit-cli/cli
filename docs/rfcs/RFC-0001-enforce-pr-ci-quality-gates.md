# RFC-0001: Enforce PR CI quality gates for AI-assisted delivery

- **作者**：@aflyingto（`Agent: Codex`；operator：@aflyingto）
- **创建日期**：2026-10-30
- **归属 SIG**：sig/governance
- **追踪 issue**：#632（候选追踪；RFC 被接受后转为正式实施追踪）

## 概述（Summary）

为 GitCode CLI 的 PR 交付链路增加一组可失败、可回滚、可审计的 CI 硬门禁。门禁覆盖覆盖率防回退、真实命令只读验证、跨平台构建证据、新代码严格 lint 和 PR 流程契约，同时继续保留独立执行主体评审与 `risk/high` 人工最终确认。

该 RFC 只提出设计。任何 `spec/` 规则变更或 CI workflow 变更必须在 RFC 接受后通过独立 PR 落地。

## 动机（Motivation）

当前项目已经建立了较强的多层质量体系：

- 本地门禁要求 `go test ./...`、`go build`、pre-commit、核心回归和系统测试
- GitCode Actions 覆盖 Linux lint、security、test、build 和 package
- GitHub Actions 覆盖 Linux、macOS、Windows 测试与构建
- CI 契约测试保护 workflow 的关键结构不变量
- 多角色独立 Agent 评审与 `risk/high` 人工确认

但当前仍有几类问题无法在普通 PR 中被机器稳定拦截：

1. **覆盖率要求没有失败门禁**。规范要求新功能覆盖率不低于 70%、核心模块不低于 80%，但两个 CI 只生成和上传覆盖率，未比较阈值。
2. **真实命令验证不在 CI 中**。`tests/system` 使用 `system` build tag，普通 `go test ./...` 不执行；真实 API 与命令输出回归只能依赖本地自觉。
3. **跨平台验证不是强约束**。当前规则是“有 GitHub 镜像 PR 时验证”，导致一个 GitCode PR 可以只有 Linux 证据就进入评审。
4. **静态检查存在人为降级**。CI 显式禁用 `errcheck` 和 `staticcheck`。
5. **PR 证据链缺少机器校验**。作者自检、标签、CI run/head SHA、文档同步等关键信息仍依赖流程自觉，AI 可以提交看起来完整但缺少证据的 PR。

不处理的代价是：AI 交付规模继续扩大后，本地流程可被跳过，CI 只证明“测试运行过”而不是“质量没有回退”；真实平台行为、覆盖率回归和跨平台兼容问题会推迟到评审或发布阶段才暴露。

## 设计方案（Proposed Design）

### 1. Coverage regression gate

新增共享脚本 `scripts/check-coverage.sh`，在 GitCode 与 GitHub 的 `test` job 中调用。

输入：

```text
go test -coverprofile=coverage.out ./...
```

脚本执行：

1. 运行 `go tool cover -func=coverage.out`
2. 解析总体覆盖率与每个有可执行语句的包
3. 忽略没有可执行语句的聚合包
4. 与 `coverage-baseline.json` 比较
5. 修改包低于 baseline、总体低于硬下限、核心包低于 80% 时失败

初始基线以当前实测数据生成；后续测试改进通过 PR 同步更新 baseline。硬下限建议从当前总体覆盖率附近的整数值开始，例如 `70%`，随后逐步提高。

CI 将调用脚本，而不是内联重复解析逻辑：

```bash
bash scripts/check-coverage.sh
```

两个 CI 的调用点继续由 `tests/contracts/ci_workflow_test.go` 和 `tests/contracts/mirror_workflow_test.go` 钉住，防止后续静默删除。

### 2. Read-only system test job

在 GitCode CI 增加 `system-test-read` job，只运行真实 GitCode 命令的只读路径：

```bash
go build -o ./gc ./cmd/gc
./scripts/regression-core.sh
go test -tags=system ./tests/system -run TestReadScripts
```

依赖：

- 一个低权限 CI bot 账号
- 一个只能读取 `infra-test/*` 的最小权限 Token
- Token 通过 CI Secret 注入，不在仓库中保存
- Job 不设置 `GC_SYSTEM_WRITE`
- 默认不输出详细命令参数；失败时只输出必要错误

写路径继续人工或独立环境 opt-in，不进入普通 PR CI。

### 3. Cross-platform evidence gate

对非 docs-only 的代码 PR，macOS/Windows 验证不能只是“有镜像 PR 时补充”。

优先级：

1. 如果 GitCode Actions 支持 Windows/macOS runner，直接在 GitCode CI 的 `test`/`build` 中使用三平台矩阵。
2. 如果暂不支持，则要求非 docs-only PR 必须存在对应 GitHub 镜像 PR。
3. GitHub run 必须与当前 GitCode PR head SHA 一致。
4. 任一平台的 `test` 或 `build` 失败，PR 不得进入 `status/ready-for-review`。

该证据先由 AI 通过 `gc actions` / `gh` 收集并写入作者自检；后续可用 PR 契约 job 自动核验。

### 4. New-code strict lint

不要求历史代码一次性清除全部 `errcheck`/`staticcheck` 问题，而是新增 `lint-new` job：

```bash
golangci-lint run \
  --new-from-rev=origin/main \
  --enable=errcheck \
  --enable=staticcheck
```

只检查当前 PR 新增或修改的代码。稳定运行后，逐步移除主 lint job 的 `--disable=errcheck --disable=staticcheck`。

### 5. PR contract job

新增 `pr-contract` job，机器校验：

- 作者主体标识存在且区别于评审主体
- PR body 含本地测试、构建、CI 证据字段
- `docs-only` 例外声明完整
- 关联 issue 引用合法，且不存在误关闭其他 issue 的裸 `#NNN`
- 风险标签与评审策略匹配
- 命令行为变更同步了 `docs/COMMANDS.md`
- CI run 的 head SHA 与 PR head SHA 一致

可复用 `scripts/validate-ai-record.py`，并扩展只读的 PR 元数据核验。

### 6. Observability and retry

- coverage、binary、wheel artifact 名中加入 `head_sha`
- 双平台 test 矩阵设置 `fail-fast: false`
- 对已知网络类偶发失败提供有限重试
- 失败时记录失败 job、步骤和重试次数，供 AI 或人工归因

### 7. Nightly checks

以下检查不作为每个 PR 的阻塞项，放入 nightly 或按需触发：

- `govulncheck ./...`
- 针对 JSON 解析、模板解析和包名解析的短时 fuzz
- 每日 `goreleaser release --snapshot --clean --skip=publish`
- 依赖锁文件一致性检查

## 备选方案（Alternatives Considered）

1. **立即启用全局严格覆盖率下限**
   优点是实现简单；缺点是大量历史包低于 70%，会产生大范围阻塞，或迫使用大量无意义测试提高指标。因此选择 baseline 防回退方案。

2. **把 write-path 系统测试也放入 PR CI**
   优点是覆盖最完整；缺点是会真实写远端资源、需要更高权限，且并发 PR 容易互相干扰。因此只进只读路径。

3. **立即全量启用 `errcheck` 和 `staticcheck`**
   优点是一次清理干净；缺点是历史问题会导致本 RFC 实施前的大规模修复，偏离“先保护新增代码”的目标。

4. **依赖 GitHub Codecov 做覆盖率门禁**
   优点是使用现成服务；缺点是 GitCode 原生 CI 仍无本地门禁，且门槛规则不能与仓库基线版本化。因此保留 Codecov 上传，同时增加仓库内脚本门禁。

5. **不做任何变化**
   优点是零实施成本；缺点是真实命令验证和覆盖率回退仍可被静默绕过，AI 交付质量依赖代理自觉而非机器证据。

## 缺点与风险（Drawbacks）

- CI 运行时间增加，尤其 system-test-read 和 coverage 解析会延长反馈周期
- CI bot Token 是新的凭证面，必须严格控制权限和日志输出
- baseline 文件可能引发“为更新 baseline 而更新”的博弈，需要评审约定
- `--new-from-rev` 的 PR 合并、rebase 或 GitHub/GitCode diff 语义需要验证
- 跨平台强约束会增加 PR 准备成本；镜像仓同步失败时可能成为阻塞点
- 新门禁可能产生环境偶发失败，需要区分“代码失败”与“平台失败”

## 回滚策略（Rollout / Rollback）

分三阶段上线：

1. **Soft mode**：新增 job 运行但不阻断 merge，连续观察两周
2. **Mandatory mode**：确认误报率可接受后转为失败门禁
3. **Strict mode**：提高 baseline，并逐步移除 legacy lint 豁免

回滚方式：

- 单次失败：允许记录平台偶发原因后有限重试
- 系统性误报：将对应 job 降回 soft mode
- 凭证风险：立即轮换并撤销 CI bot Token
- 设计失效：关闭 RFC 追踪 issue，删除新增 job 和脚本，保留原有 CI 行为

## 未决问题（Unresolved Questions）

- GitCode Actions 是否具备 macOS/Windows runner 或等价矩阵能力
- CI bot 的 Token scope 在平台侧具体如何最小化
- GitHub 镜像 PR 应由人工创建、自动同步还是通过 GitCode workflow 创建
- baseline 文件按包、按目录还是按文件维护最合适
- `pr-contract` 的平台 API 是否稳定支持读取 PR body、labels 与 CI 状态
- 哪些网络错误允许自动重试，以及最大重试次数

## 实施计划（Implementation Plan）

1. PR A：新增 coverage baseline 与共享检查脚本，并更新 CI 契约测试
2. PR B：新增只读 system-test job 与最小权限 CI bot 配置
3. PR C：跨平台证据规则落地，先写入流程文档，再由 PR contract 校验
4. PR D：新增 `lint-new` job，验证 `--new-from-rev`
5. PR E：新增 PR contract job，并更新 AI 本地开发流程文档
6. PR F：soft mode 数据观察与误报修复
7. PR G：所有新增门禁转为 mandatory，并同步 `spec/` 正式规则

RFC 接受不自动实施；实施由维护者或认领者按上述顺序推进，作者不强制自己实施。
