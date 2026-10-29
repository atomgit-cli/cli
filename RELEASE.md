# GitCode CLI 版本发布指南

> 当前正式发布规范以 [spec/delivery/release-process.md](./spec/delivery/release-process.md) 为准。
>
> 本文档保留的是历史性的 GitHub Actions / GitHub Release 发布说明，用于解释早期自动发布设计和兼容背景，不能替代当前正式规范。

## 文档定位

本文档的职责是：

- 保留历史 GitHub Actions 自动发布方案说明
- 解释早期 GitHub Release / PyPI 发布设计

本文档不负责：

- 定义当前正式发布流程
- 定义当前 GitCode 环境下的发布门禁
- 替代 `spec/delivery/release-process.md`

如果你要执行当前版本发布，请先阅读：

- [spec/delivery/release-process.md](./spec/delivery/release-process.md)
- [spec/delivery/build-and-package.md](./spec/delivery/build-and-package.md)
- [docs/PACKAGING.md](./docs/PACKAGING.md)

## 当前边界说明

当前仓库仍未建立 GitCode CI 自动发布闭环，因此：

- 当前正式规则不以本文件为准
- 本文件中的 GitHub Actions 自动发布内容属于历史参考
- 若与 `spec/delivery/release-process.md` 冲突，以 `spec/` 规范为准

## 发布方式

**历史方案**：通过 GitHub Actions 自动发布。

### 发布触发条件

| 触发方式 | 说明 |
|---------|------|
| 推送 `v*` 标签 | 自动触发完整发布流程 |
| 手动触发 | 通过 GitHub Actions 界面手动触发 |

### 发布命令

```bash
# 1. 确保代码已合并到 main 分支
git checkout main
git pull

# 2. 创建并推送标签
git tag v0.3.4
git push origin v0.3.4

# 3. GitHub Actions 自动执行发布
```

---

## 发布产物

每次发布生成以下产物：

### 1. RPM/DEB 包（Linux）

构建后上传到 GitHub Artifacts，可在其他 workflow 中下载使用。

| 产物类型 | 架构 | 文件名 |
|---------|------|--------|
| RPM | x86_64 | `gc-{version}-1.x86_64.rpm` |
| RPM | aarch64 | `gc-{version}-1.aarch64.rpm` |
| DEB | amd64 | `gc_{version}_amd64.deb` |
| DEB | arm64 | `gc_{version}_arm64.deb` |

**在其他 workflow 中下载**：
```yaml
- name: Download packages
  uses: actions/download-artifact@v4
  with:
    name: packages
```

### 2. PyPI 包（跨平台）

发布到 PyPI，用户可通过 pip 安装。

| 包名 | 安装命令 |
|------|----------|
| `gitcode-cli` | `pip install gitcode-cli` |

**支持平台**：
- Linux (x86_64, arm64)
- macOS (x86_64, arm64)
- Windows (x86_64)

**安装方式**：
```bash
pip install gitcode-cli

# 或指定版本
pip install gitcode-cli==0.3.4
```

---

## 发布产物汇总表

| 产物类型 | 格式 | 架构 | 发布位置 |
|---------|------|------|---------|
| RPM | `.rpm` | x86_64, aarch64 | GitHub Release |
| DEB | `.deb` | amd64, arm64 | GitHub Release |
| Wheel | `.whl` | 跨平台 | GitHub Release |
| 源码包 | `.tar.gz` / `.zip` | all | GitHub Release |
| PyPI | wheel | 跨平台 | PyPI 官方源 |

---

## 前置配置

### PyPI Trusted Publishing

已在 GitHub 配置 Environments → pypi，使用 OIDC 认证，无需配置 API Token。

**配置步骤**（已配置）：
1. PyPI → Publishing settings → Add a new pending publisher
2. 填写 PyPI 项目名、Owner、Repository name、Workflow name
3. GitHub 仓库 → Settings → Environments → 创建 `pypi` environment

### 无需配置的 Secrets

| Secret | 说明 |
|--------|------|
| `GITHUB_TOKEN` | 自动提供 |
| PyPI Token | 使用 Trusted Publishing，无需 token |

---

## 发布流程

### 完整发布流程

```
┌─────────────────────────────────────────────────────────────┐
│                      推送 v* 标签                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    GitHub Actions 触发                       │
└─────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    ▼                   ▼
           ┌─────────────┐       ┌─────────────┐
           │    Build    │       │    PyPI     │
           │  (GoReleaser)│       │   发布      │
           └─────────────┘       └─────────────┘
                    │                   │
                    ▼                   ▼
           ┌─────────────┐       ┌─────────────┐
           │ - Artifacts │       │ PyPI        │
           │   (RPM/DEB) │       │ gitcode-cli │
           │ - Release   │       │             │
           │   (源码包)  │       │             │
           └─────────────┘       └─────────────┘
```

### 发布步骤详解

```bash
# 步骤 1: 确保在 main 分支
git checkout main
git pull origin main

# 步骤 2: 运行测试确保代码质量
make test
make lint

# 步骤 3: 更新版本号（如果有版本文件）
# - pyproject.toml
# - gc_cli/__init__.py

# 步骤 4: 提交版本更新
git add .
git commit -m "chore: prepare for release v0.3.4"

# 步骤 5: 创建标签
git tag -a v0.3.4 -m "Release v0.3.4"

# 步骤 6: 推送标签触发发布
git push origin main --tags

# 步骤 7: 监控 GitHub Actions 执行状态

# 步骤 8: 发布完成后验证
# - 检查 GitHub Release 页面
# - 检查 PyPI: https://pypi.org/project/gitcode-cli/
# - 检查 Artifacts 中的 RPM/DEB 包
```

---

## 发布后验证

### 验证 PyPI 包

```bash
# 创建虚拟环境测试
python -m venv test-env
source test-env/bin/activate
pip install gitcode-cli==0.3.4
gc version
```

### 验证 RPM/DEB 包

从 GitHub Actions Artifacts 下载后：

```bash
# RPM
sudo rpm -i gc-0.3.4-1.x86_64.rpm
gc version

# DEB
sudo dpkg -i gc_0.3.4_amd64.deb
gc version
```

---

## 版本命名规范

遵循 [语义化版本](https://semver.org/) 规范：

```
vMAJOR.MINOR.PATCH[-PRERELEASE]

示例：
v1.0.0         # 正式版本
v1.0.0-beta.1  # 预发布版本
v1.0.1         # Bug 修复版本
v1.1.0         # 新功能版本
v2.0.0         # 重大更新版本
```

---

## 发布检查清单

发布前必须确认以下事项：

### 代码质量
- [ ] 所有测试通过 (`make test`)
- [ ] Lint 检查通过 (`make lint`)
- [ ] 代码覆盖率达标
- [ ] 无已知严重 Bug

### 文档更新
- [ ] Release notes 已更新
- [ ] README.md 版本信息已更新（如需要）

### 发布后验证
- [ ] GitHub Release 页面正确
- [ ] PyPI 包可安装运行
- [ ] Artifacts 中的 RPM/DEB 包存在

---

## Release Notes 模板

创建 Release 时，使用以下模板确保安装命令完整：

```markdown
## 更新内容

### 新功能
- 功能描述

### Bug 修复
- 修复描述

### 修复的 Issue
- Fixes #XX

## 安装方式

### Linux 二进制文件
\`\`\`bash
# AMD64
wget https://gitcode.com/atomgit-cli/cli/releases/download/v{VERSION}/gc_linux_amd64
chmod +x gc_linux_amd64
sudo mv gc_linux_amd64 /usr/local/bin/gc

# ARM64
wget https://gitcode.com/atomgit-cli/cli/releases/download/v{VERSION}/gc_linux_arm64
chmod +x gc_linux_arm64
sudo mv gc_linux_arm64 /usr/local/bin/gc
\`\`\`

### PyPI（推荐）
\`\`\`bash
# 方式一：从 PyPI 直接安装
pip install gitcode-cli

# 方式二：从 Release 下载安装
wget https://gitcode.com/atomgit-cli/cli/releases/download/v{VERSION}/gitcode_cli-{VERSION}-py3-none-any.whl
pip install gitcode_cli-{VERSION}-py3-none-any.whl
\`\`\`

## 发布说明入口

参见当前 release notes 和 [spec/delivery/release-process.md](./spec/delivery/release-process.md)
```

**重要**：所有下载链接必须使用完整路径格式：
```
https://gitcode.com/atomgit-cli/cli/releases/download/v{VERSION}/{FILENAME}
```

---

## 相关文档

- [CONTRIBUTING.md](./CONTRIBUTING.md) - 贡献指南
- [docs/PACKAGING.md](./docs/PACKAGING.md) - 本地打包指南

---

**最后更新**: 2026-03-24
