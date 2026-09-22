# 发布流程规范

本文件定义 gitcode-cli 的正式发布流程、版本约束、发布检查和发布后验证要求。

## 1. 目标

发布流程需要满足以下目标：

- 发布动作可复现
- 版本号和产物命名一致
- 发布说明与真实产物一致
- 发布前后都可验证
- 文档与安装方式同步更新

## 2. 适用范围

本规范适用于：

- 版本发布准备
- release notes 编写
- tag 创建
- release 产物上传
- 发布后验证

本规范定义正式发布的流程边界；GitCode 与 GitHub CI 的门禁细节以 `spec/delivery/ci-workflows.md` 和对应 workflow 文件为准。

## 3. 权威边界

- 正式发布流程以本文件为准
- 发布用命令和产物说明以本文件和 `docs/PACKAGING.md` 共同约束
- 历史性的 GitHub Actions 说明保留在根目录 `RELEASE.md`，但不能替代本规范

## 4. 版本规则

项目版本遵循语义化版本：

```text
vMAJOR.MINOR.PATCH[-PRERELEASE]
```

示例：

- `v1.0.0`
- `v1.0.1`
- `v1.1.0`
- `v2.0.0`
- `v1.0.0-beta.1`

发布时必须保证：

- git tag 与 release tag 一致
- 产物文件名中的版本与 tag 一致
- release notes 中的下载示例与实际版本一致
- 根 `VERSION` 与 nFPM、Python、npm metadata 一致；通过 `scripts/sync-package-version.sh <version> --check`

手动触发发布工作流时，`workflow_dispatch.inputs.version` 必须在第一阶段通过白名单校验后才能进入 tag、release、资产上传或配置替换步骤。允许的输入格式为：

```text
vMAJOR.MINOR.PATCH
vMAJOR.MINOR.PATCH-PRERELEASE
```

其中 `PRERELEASE` 只允许包版本兼容的 `alpha.N`、`beta.N`、`rc.N` 形式，`N` 为非负整数且不能带多余前导零，例如 `beta.1`、`rc.2`。

工作流不得把未验证的 `${{ inputs.version }}` 直接拼接进 shell 脚本文本中。应通过 `env` 传入 shell，再调用仓库脚本 `scripts/validate-release-version.sh` 完成校验和归一化。校验脚本可接受可选前缀 `v`，便于本地复用；正式 release workflow 只接受带 `v` 前缀的规范版本输入，并必须校验输入与派生 tag 完全一致，不能用未规范化的原始输入创建 tag 或 release。

正式 release workflow 必须串行执行。若目标 Release、PyPI 或 npm 版本已存在，重跑前必须比较既有资产的文件清单和 SHA-256；只有完全一致的制品可按幂等方式跳过，禁止覆盖或忽略任何差异制品。

## 5. 发布前置条件

正式发布前必须满足：

- 目标改动已合并到 `main`
- 当前主线处于可发布状态
- 本地构建与相关测试已完成
- 真实命令验证已完成
- 文档已同步
- 无未解决的 blocker 级问题
- Homebrew 链路就绪（首次发布前一次性配置）：
  - tap 仓 `gitcode-cli/homebrew-tap` 存在且默认分支为 `main`
  - tap 仓已配置 write deploy key（公钥）
  - 主仓 secret `HOMEBREW_TAP_DEPLOY_KEY` 已配置（对应私钥）
- npm 链路就绪（首次发布前一次性配置，**三坐标逐一配置**）：
  - npm 组织 `gitcode-cli` 与 `atomgit-cli` 已创建，发布者为成员；裸名 `atomgit-cli` 与 `@atomgit-cli/cli` 的 Trusted Publisher 同样绑定
  - 在 npmjs.com 为 `@gitcode-cli/cli`、`@atomgit-cli/cli`、`atomgit-cli` 三个包分别配置 **Trusted Publisher**（GitHub Actions）：org `atomgit-cli`、repo `cli`、workflow filename `release.yml`、allowed `npm publish`。npm CLI ≥ 11.5.1 + Node ≥ 22.14 自动经 OIDC 发布，**无需 NPM_TOKEN**；Classic token 已于 2025-11 移除，granular token 无法创建 org 下首个新包，故采用 OIDC（npm 官方推荐）
  - `npm/package.json` 的 `repository.url` 必须为 `https://github.com/atomgit-cli/cli.git`（OIDC 校验 repository.url 与 GitHub 仓一致；旧 `gitcode-cli` 路径仅重定向兼容）

## 6. 标准发布流程

当前仓库的标准发布流程如下：

1. 切换到最新 `main`
2. 运行测试与本地构建验证
3. 在 `docs/releases/vX.Y.Z.md` 准备并评审 release notes
4. 运行 `scripts/sync-package-version.sh X.Y.Z` 同步根 `VERSION` 与所有 package metadata，并同步文档版本号
5. 使用标准脚本在本地验证发布产物
6. 将发布准备 PR 合入 GitCode 与 GitHub 的 `main`，确认 tree hash 一致
7. 触发 GitHub release workflow，创建 tag、GitHub Release、正式产物并发布 PyPI，推送 Homebrew formula 到 homebrew tap（brew job），并以三坐标并行发布 npm 包 `@gitcode-cli/cli`、`@atomgit-cli/cli`、`atomgit-cli`（npm job，内置多平台二进制 + Node wrapper + `install` 子命令；发布后校验三坐标 dist-tag 版本一致）
8. 通过 SSH 将同一 tag 推送到 GitCode，并把同一批正式产物上传到 GitCode Release
9. 对两个平台、PyPI、Homebrew 与 npm 安装入口执行发布后验证

### 6.1 获取最新主线

```bash
git checkout main
git pull origin main
```

### 6.2 最低发布前验证

```bash
go test ./...
go build -o ./gc ./cmd/gc
./gc version
```

如发布涉及命令行为变更，还必须执行真实命令验证。

### 6.3 构建发布产物

优先使用：

```bash
./scripts/package.sh <version> release
```

这一步用于本地发布前验证。制品中的版本、commit SHA 或来源不可追溯时不得上传；正式发布制品必须来自合入后 `main` 的标准 release workflow。

### 6.4 同步文档版本号

`README.md` 与 `docs/PACKAGING.md` 含版本号绑定的下载 URL 与示例，必须随发版同步，否则滞后（见 #314）。使用专用脚本自动探测当前版本串并替换为目标版本，幂等，替换后校验无残留：

```bash
./scripts/sync-docs-version.sh vX.Y.Z
```

脚本支持 `--dry-run` 预览。同步后提交到 `main`，再创建 tag，使 tag 指向的提交包含正确的文档版本引用。下载 URL 指向的 release 产物在 release 创建与上传完成后（§6.5）即生效。

### 6.5 创建正式 tag、GitHub Release 与 PyPI 包

发布准备改动合入两个远端的 `main` 且 tree hash 一致后，触发标准 workflow：

```bash
gh workflow run release.yml -R gitcode-cli/cli -f version=vX.Y.Z
gh run watch <run-id> -R gitcode-cli/cli
```

workflow 必须先在只读权限下校验 `docs/releases/vX.Y.Z.md`、执行 GoReleaser snapshot、nFPM、wheel/sdist 和入口冒烟；全部预检通过后，独立的最小写权限 job 才能创建指向当前 `main` 的 tag。正式制品从该 tag 在只读 job 中构建并生成覆盖全部资产的 SHA-256 清单，再由独立 job 发布 GitHub Release。PyPI job 只下载已验证制品并执行 Trusted Publishing，不参与构建，也不修改 Release。已有 tag 仅在其 commit 与当前 workflow HEAD 完全一致时允许复用。

`brew` job 在 publish 后推送 Homebrew formula 到 tap 仓，是发布完整性的组成部分：失败即整个 release workflow 失败，阻断 §6.6 GitCode 同步，需人工修复（deploy key / secret / tap 仓）后重跑 workflow（publish 幂等跳过，brew 重新推送），或手动补推 formula 与 GitCode 同步。它使用 deploy key 而非 GITHUB_TOKEN，只读权限即可。

`artifacts` job 从同一批 GoReleaser 标准产物提取 5 个平台二进制（linux/darwin × amd64/arm64 + windows amd64），调用 `scripts/prepare-npm-package.sh` 按**三坐标**（`@gitcode-cli/cli` / `@atomgit-cli/cli` / `atomgit-cli`，坐标 allowlist 硬校验）各组装一个 npm tarball——每个坐标副本以自身包名运行 wrapper 单测（动态坐标代码路径按坐标实测）——并把全部 `.tgz` 纳入统一 Release SHA-256 清单。禁止 `npm` job 独立重编译。

`npm` job 只下载并校验 `release-assets`（三坐标 tarball 齐全且入清单），再按坐标逐一以 **OIDC Trusted Publishing** 执行 `npm publish <verified.tgz> --access public`（`permissions: id-token: write`，无 `NPM_TOKEN`，自动 provenance）。任一坐标目标版本已存在时，必须 `npm pack` 下载 registry tarball 并与 Release tarball 比较 SHA-256；只有完全一致才可跳过，差异必须阻断发布。发布完成后必须校验三坐标同名 dist-tag 均指向目标版本（长期并行不变量，docs/PACKAGING.md）。

稳定版本发布到 npm `latest` dist-tag；带 prerelease 后缀的版本必须显式使用 `next`，不得污染自动更新读取的 stable `latest`。

若正式 workflow 已创建 GitHub Release、但 npm job 在 registry 写入前单独失败，可在修复并合入 `main` 后使用同一个受信任的 `release.yml` 执行仅 npm 恢复：

```bash
gh workflow run release.yml -R gitcode-cli/cli \
  -f version=vX.Y.Z \
  -f npm_recovery=true
```

恢复 job 必须要求 `main` 中存在经 PR 评审的 `docs/releases/vX.Y.Z.npm-recovery.json`，以精确 tag commit、tag tree 与 npm tarball SHA-256 把恢复目标绑定到受审事实。清单 schema v2 携带 `packages` 映射（每坐标一项：`file` + `sha256`，坐标限三坐标 allowlist）；v1 单包清单（`package` + `sha256`，仅 `@gitcode-cli/cli`）继续可恢复。恢复 job 须验证当前根 `VERSION`、tag 内 `VERSION`、tag tree 存在于当前 `main` 历史（允许双镜像产生 tree 相同但 commit 不同的 merge）、GitHub Release 非草稿、每个 Release tarball 的 SHA-256 与清单校验行、包内 name（等于恢复坐标）/version/repository（新旧 repository.url 均接受，重定向等价），以及内置 Linux 二进制的 version/commit。它不得重建制品、移动 tag 或重发其他渠道；发布前必须拒绝对应坐标 dist-tag 回退，目标 npm 版本已存在时仍须比较 registry tarball 与 Release tarball 并校验 dist-tag；清单覆盖全部三坐标时，恢复后校验三坐标 dist-tag 一致。该恢复路径继续使用 `release.yml` 的 OIDC Trusted Publisher 身份，不使用长期 `NPM_TOKEN`。

### 6.6 同步 GitCode tag、Release 与正式制品

GitHub workflow 全部成功后，通过 SSH 将同一 tag 推送到 GitCode，并下载 GitHub Release 的正式制品：

```bash
git fetch github tag vX.Y.Z
git push origin refs/tags/vX.Y.Z
gh release download vX.Y.Z -R gitcode-cli/cli --dir dist/github-release
cd dist/github-release
sha256sum -c gc_X.Y.Z_checksums.txt
cd ../..
```

使用受跟踪的同一份 release notes 创建 GitCode Release，再上传下载得到的同一批制品：

```bash
gc release create vX.Y.Z -R gitcode-cli/cli \
  --title "GitCode CLI vX.Y.Z" \
  --notes-file docs/releases/vX.Y.Z.md \
  --target main \
  --json
gc release upload vX.Y.Z dist/github-release/* -R gitcode-cli/cli --json
```

上传 GitCode 前必须先通过完整校验和验证。禁止在 GitCode 侧重新构建另一套正式制品。不得提取 `gh` 或 `gc` 保存的 token 再交给脚本或 `curl`；认证必须由 CLI 自身封装。

## 7. Release Notes 规则

release notes 必须满足：

- 描述本次更新内容
- 明确修复的 issue 或功能范围
- 提供完整安装方式
- 下载链接必须是完整路径
- 版本号必须与实际产物一致

不允许：

- 只写文件名，不写完整下载地址
- 使用与实际版本不一致的安装命令
- 在未验证产物存在前写入下载示例

## 8. 发布后验证

正式发布后至少完成以下验证：

- release 页面存在且信息正确
- 发布产物名称正确
- 下载链接可访问
- 至少抽样验证一个安装路径
- `gc version` 可正常输出版本信息
- GitCode 与 GitHub tag 指向同一 commit，两个 `main` 的 tree hash 一致
- GitCode 与 GitHub Release 的同名资产校验和一致
- PyPI 返回目标版本，且 wheel 内置二进制的版本和 commit SHA 可追溯
- Homebrew tap 仓 `gc.rb` 已更新到目标版本，`brew install gitcode-cli/homebrew-tap/gc` 可安装且 `gc version` 输出正确
- Homebrew 的 `gc` 与 `gitcode` 两个入口均解析到同一版本
- npm 三坐标均返回目标版本（`npm view @gitcode-cli/cli version`、`npm view @atomgit-cli/cli version`、`npm view atomgit-cli version`），各自 registry tarball 与 Release `.tgz` SHA-256 一致
- 固定 generic/scoped 官方 registry 且使用 `--ignore-scripts` 的 `npm install -g`（三坐标至少抽验推荐坐标 `atomgit-cli` 与一个 scoped 坐标）与 `npx --yes --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@latest install` 均提供 `gc` / `gitcode`，`version --json` 的版本与 commit SHA 正确，`doctor install --json` 识别对应 distribution；不得把只添加当前项目依赖的裸 `npm i` 写成 CLI 安装入口
- Windows、Linux、macOS 至少执行旧 npm 版本到目标版本的真实升级烟测；Windows 同时覆盖旧 pip launcher 位于 PATH 前方的诊断

若发布包含 DEB / RPM / wheel，建议至少各抽样验证一种常用安装路径。

## 9. 文档同步要求

发布流程、版本策略或安装方式变化时，必须同步检查：

- `README.md`
- `docs/PACKAGING.md`
- `docs/COMMANDS.md`
- `spec/delivery/release-process.md`
- `spec/delivery/build-and-package.md`
- `AGENTS.md`
- `CLAUDE.md`
- 相关 AI skills

## 10. 禁止事项

以下行为不允许出现：

- 未验证产物即创建正式 release
- release notes 中使用错误版本号
- 发布说明仍引用旧文件名或旧下载路径
- 用个人临时脚本替代仓库标准流程而不更新文档
- 把平台不支持的自动发布能力写成既成事实

## 11. 当前执行基线

当前发布执行基线为：

1. 本地 Windows 与 WSL Linux 门禁、真实命令验证全部通过
2. GitCode 原生 CI 与 GitHub 镜像跨平台 CI 对发布提交全部通过
3. GitHub release workflow 从已同步的 `main` 创建 tag、正式制品、GitHub Release 与 PyPI 包
4. GitCode 复用同一 tag、release notes 与正式制品，不在平台侧重复构建
5. 人工核对双端 tag、main tree、同名资产校验和及安装结果
