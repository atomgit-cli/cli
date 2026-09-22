# RFC-0001: AtomGit 品牌适配——ac/atomgit 命令名、并行分发与全渠道向后兼容

- **作者**：@aflyingto（Agent: opencode, operator: aflyingto）
- **创建日期**：2026-09-21
- **归属 SIG**：sig/cli-ux（主）；分发与发布工程无归属 SIG，由 sig/governance 兜底参与
- **追踪 issue**：#588

## 概述（Summary）

GitCode 品牌演进为 AtomGit。本提案为 GitCode CLI 引入新品牌命令名（`ac` / `atomgit`）与新 npm 包（`@atomgit-cli/cli`），并同步适配 deb/rpm/Homebrew/scoop/wheel/completions 分发渠道；**所有既有入口保持原样可用**：`gc` / `gitcode` 命令、`GC_*` / `GITCODE_*` 环境变量、`~/.config/gc/` 配置、`@gitcode-cli/cli` npm 包全部继续工作并并行发布。域名与 API host 不做任何切换。

## 动机（Motivation）

- 平台品牌演进为 AtomGit，CLI 用户接触面（命令名、包名、文案）需要跟进
- 存量用户不可破坏：npm 装机、CI 脚本、文档教程大量引用 `gc` / `gitcode` / `@gitcode-cli/cli`，硬切换会造成大面积断裂
- 双名/双包并行让用户自主选择迁移时机，不存在强制切换日

维护者已就 7 个关键决策拍板（见追踪 issue #588 决策记录），本 RFC 是其设计落地。

## 设计方案（Proposed Design）

### 1. 命令名解析

`pkg/cmd/root/root.go` 的 `resolveCommandName()` 解析顺序调整为：

```
ATOMGIT_CLI_COMMAND_NAME
  → GITCODE_CLI_COMMAND_NAME    （兼容保留）
  → 可执行文件基名（归一化后）
  → 平台默认：Windows = atomgit，其他 = ac
```

- `normalizedCommandName()` 增加对打包文件名（`ac-linux-arm64` 等）的归一化，与既有 `gc-*` 同规则
- 帮助文案按 `commandName` 动态生成（沿用既有 `rootLong(commandName)` 机制），新名下示例显示 `ac`，旧名下显示 `gc`
- 补全脚本生成两套：`gc.{bash,zsh,fish}` 与 `ac.{bash,zsh,fish}`（`make completions` 输出两组）

### 2. 环境变量兼容层

| 类别 | 顺序 |
|------|------|
| Token | `AG_TOKEN` > `ATOMGIT_TOKEN` > `GC_TOKEN` > `GITCODE_TOKEN` > auth.json |
| 其他（`API_DEBUG` / `BROWSER` / `CONFIG_DIR` / `DEBUG` / `EDITOR` / `GIT_PROTOCOL` / `PAGER` / `TIMEOUT`） | `AG_*` > `GC_*` > 默认值 |

- 旧变量不输出弃用警告（决策 2/4：无限期兼容，弃用时间表见未决问题）
- `cmdutil.ScanContentForSecrets` 将 `AG_TOKEN` / `ATOMGIT_TOKEN` 纳入提交前扫描

### 3. 配置目录

解析顺序：`AG_CONFIG_DIR` / `ATOMGIT_CONFIG_DIR` env > `~/.config/ac/`（若存在）> `~/.config/gc/`（若存在，**继续原地读写，不迁移**）> 全新安装创建 `~/.config/ac/`。

- **单活动目录原则**：任一时刻只激活一个目录，避免 split-brain
- 存量用户零感知：检测到 `~/.config/gc/` 即沿用，auth/config 全部有效
- 同时存在两个目录时优先 `ac`（用户显式创建 `~/.config/ac/` 视为主动迁移信号）

### 4. npm 三坐标并行（长期承诺）

| | `atomgit-cli`（裸名，**推荐**） | `@atomgit-cli/cli`（新 scoped） | `@gitcode-cli/cli`（旧 scoped） |
|---|---|---|---|
| bin 入口 | `gc` / `gitcode`（实施 PR-2 前与旧包一致，其后含 `ac` / `atomgit` 超集） | 同左 | `gc` / `gitcode`（不变） |
| 版本号 | 三坐标同版本号同步演进 | 同左 | 照常演进 |
| 内容 | 同一组 goreleaser 二进制 | 同上 | 同上 |

- **长期并行承诺，不设弃用时间表**（决策 5 为长期承诺，非过渡措施）：每次正式发布必须以同一版本号发布全部三坐标（内容一致），发布流水线校验三坐标版本一致后方可判定发布成功；无任何坐标的 deprecation 计划
- npm 裸名 `gitcode-cli` 被第三方占用（2026-05 起，非本项目），不可达且已在 README 标注勿装——"四坐标"目标实际为三坐标
- release 流水线**一次构建**，按坐标模板化 package.json 组装三个 npm tarball（差异仅 name/bin/描述），全部纳入统一 checksum，全部发布（对齐既有"npm tarball 必须由 release artifacts job 组装"的规范；流水线须含坐标 allowlist 校验，防模板化笔误）
- 捆绑二进制文件名保持 `gc-linux-*` 等不变（避免 goreleaser 归档、checksum、platform.js 映射全链变动）；npm wrapper 按被调用的 bin 名设置 `ATOMGIT_CLI_COMMAND_NAME` 传递自名
- update checker / PATH 诊断按各自包名与入口适配（PR !564 已落地动态坐标）

### 5. 打包渠道同步适配（每渠道同时提供新旧名）

- **goreleaser**：主二进制仍为 `gc`；归档内追加 `ac` / `atomgit` 副本（同内容）
- **deb/rpm（nfpm）**：同一包安装 `/usr/bin/gc`、`/usr/bin/gitcode`、`/usr/bin/ac`、`/usr/bin/atomgit` 四个入口
- **Homebrew**：既有 tap formula 增加 `ac` / `atomgit` bin；新 tap `atomgit-cli/homebrew-tap` 提供 `ac` formula
- **scoop**：manifest 增加 `ac` / `atomgit` 入口
- **Python wheel**：PyPI 包名 `atomgit-cli` 与旧包并行发布，console scripts 提供全部四个入口；`gc_cli/wrapper.py` 按调用名自适配
- **Docker**：镜像标签增加 atomgit 命名别名，原标签保留

### 6. 仓库与模块路径

- **仓库已 rename**（2026-09-21 维护者操作）：`gitcode-cli/cli` → `atomgit-cli/cli`，`gc-api-doc` 随迁 `atomgit-cli/gc-api-doc`。PR / issue / CI 历史随仓库对象完整保留；git 层旧路径（SSH/HTTPS clone/fetch）经平台重定向仍可用，**API 层不重定向**——所有 `-R gitcode-cli/cli` 调用须改用 `-R atomgit-cli/cli`
- 仓内引用更新（实施 PR-5 范围）：`.gitmodules`、CI workflows、scripts、docs、AGENTS/CLAUDE 中的 `gitcode.com/gitcode-cli/*` 路径改为 `atomgit-cli/*`；Go module path 除外（见下）
- **Go module path 保持 `gitcode.com/gitcode-cli/cli` 不变**：CLI 应用不是可导入库，module path 仅是标识符；改写 500+ 文件 import 是纯噪声高风险动作。旧路径重定向失效（如旧 org 路径被重新注册）不影响本地构建
- 风险与对策：旧路径重定向的持久性不可控——文档与脚本一律改用新路径，不依赖重定向；本地/CI remote 同步更新
- GitHub 镜像 org 已同步改名（2026-09-21，`github.com/gitcode-cli` → `github.com/atomgit-cli`，homebrew-tap 随迁；镜像同步链路经 rename 后 push 验证仍工作）。剩余工作：仓内 23 处 `github.com/gitcode-cli` 引用（release 下载 URL、homebrew formula、workflows、scripts）更新，属实施 PR-5/6 范围；GitHub 旧路径自动重定向在旧 org 名被重新注册后失效，不作为长期依赖

### 7. 品牌文案

- 用户可见文案（help、version、错误提示、安装引导）以 "AtomGit CLI" 为主表述，首次输出附一句"`gc` / `gitcode` 旧名继续可用"
- **域名、API host、web URL 一律不变**（决策 1）：`api.gitcode.com` / `web-api.gitcode.com` / `gitcode.com` 链接保持原样
- `docs/COMMANDS.md` 等文档示例分阶段改为 `ac` 为主，标注旧名别名

## 备选方案（Alternatives Considered）

| 方案 | 弃用原因 |
|------|----------|
| 硬切换 v1.0.0（移除旧名） | 破坏全部存量用户，违背决策 2/4/5 |
| Go module path 与全量 import 重写 | 500+ 文件机械改动、diff 噪声与回归风险，无用户价值 |
| 旧 npm 包标记 deprecated 指向新包 | 违背决策 5（并行支持） |
| 域名/host 同步切换 | 平台侧保持原域名；留待未来单独 RFC |

## 缺点与风险（Drawbacks）

- 双名/双包/双环境变量的长期维护成本：每个新增环境变量须双份（`AG_*` + `GC_*`）
- 捆绑二进制内部文件名仍为 `gc-*`，与新品牌不一致（内部细节，不影响用户交互面）
- 配置目录双位置判定规则需要清晰文档，误建双目录的用户需重新认证一次
- 仓库 rename 后旧路径依赖平台重定向，其持久性不可控；API 层无重定向需全量改用新 slug

## 回滚策略（Rollout / Rollback）

- 兼容性设计使回滚代价低：移除新名入口与新包发布即回到现状；旧名、旧包、旧配置路径全程未动
- 新包单侧故障（发布失败、镜像问题）：停发 `@atomgit-cli/cli`，不影响 `@gitcode-cli/cli` 正常演进
- 实施按 6 个 PR 分阶段合入，每阶段独立可回滚

## 未决问题（Unresolved Questions）

- 是否提供显式迁移命令（`ac config migrate`：`~/.config/gc` → `~/.config/ac`）
- `GC_*` 旧环境变量的弃用时间表（建议：至少到 v1 前不设限，届时另行决策；**npm 三坐标不在此列——长期并行是承诺，不是待决事项**）
- Homebrew 双 tap 的最终形态（单 formula 双名 vs 双 tap 并行）
- 平台域名未来若切换，需另起 RFC 处理 host、web URL 与配置迁移

## 实施计划（Implementation Plan）

| # | PR | 范围 | 依赖 |
|---|-----|------|------|
| 1 | 命令名解析 + env 兼容层 | `root.go`、`pkg/config`、`cmdutil`（含 secrets 扫描）+ 单测 | — |
| 2 | npm 双包 | package 模板、install/update/PATH 诊断、wrapper 自名传递 + npm 测试 | 1 |
| 3 | 打包渠道 | goreleaser、nfpm、scoop、homebrew、Makefile、completions | 1 |
| 4 | Python wheel | PyPI `atomgit-cli`、entry points、wrapper 适配 | 1 |
| 5 | 文档与规范 | README、COMMANDS、AUTH、PACKAGING、spec、AGENTS/CLAUDE、Docker 文档 | 1–4 |
| 6 | release 工程 | release workflow 双包发布、统一 checksum、GitHub 镜像与 tap 引用更新、CI workflow + spec/delivery 更新 | 2–4 |

回归：`./scripts/regression-core.sh` + 全渠道冒烟（双 npm 包安装、deb/rpm、brew、scoop、wheel、双命令名补全加载）。
