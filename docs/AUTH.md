# 认证模型

本文档定义当前版本 GitCode CLI 的认证来源优先级和持久化语义。

## 当前模型

当前版本支持：

1. 环境变量认证
2. 本地配置持久化认证
3. 单主机单活动用户模型
4. 通用偏好配置持久化

当前版本不支持：

- 多账户切换的完整产品化体验
- 操作系统 keyring 集成

## 认证来源优先级

同一主机下，活动 token 的解析顺序为：

1. `GC_TOKEN`
2. `GITCODE_TOKEN`
3. 本地配置文件中的已登录 token

也就是说：

- 默认主机 `gitcode.com` 下，只要设置了环境变量，环境变量始终覆盖本地存储
- `auth logout` 只会清理本地存储，不会替你取消环境变量

## 持久化边界

`auth login` 成功后会将以下信息写入本地配置目录：

- 主机名
- 用户名
- token
- Git protocol

默认位置：

```bash
~/.config/gc/auth.json
```

可通过 `GC_CONFIG_DIR` 覆盖配置目录。

凭证文件落盘遵循安全约束（见 `spec/foundations/security.md`）：文件权限 `0600`、目录权限 `0700`、写入时拒绝符号链接（防止凭证重定向攻击）。若 `auth.json` 是符号链接（如某些 dotfiles 管理方案），`auth login` 将拒绝写入并报错，请删除符号链接后重试。

通用偏好配置写入同一配置目录下的：

```bash
~/.config/gc/config.json
```

当前通用配置用于保存 `editor`、`browser`、`pager` 等非敏感偏好。认证 token 仍只写入 `auth.json`。
通用配置会拒绝 `token`、`password`、`secret` 等未列入允许列表的键。

## 主机传播语义

- `GC_HOST` 或登录配置中的默认 host 会决定已接入共享 host-aware 认证入口的业务命令所使用的 GitCode 主机。
- host 必须是受信的 hostname-only 值，例如 `gitcode.com` 或 `enterprise.example.com`；不能包含 URL scheme、路径、端口、用户名或空白字符。
- 业务 API 请求会把 GitCode Web 主机映射为对应 API 主机，例如 `gitcode.com` 映射到 `api.gitcode.com`，`enterprise.example.com` 映射到 `api.enterprise.example.com`。
- 默认 `gitcode.com` 下，环境变量 token 仍优先于本地 token。
- 非默认 host 下，业务命令只使用该 host 本地登录保存的 token，不会把通用 `GC_TOKEN` / `GITCODE_TOKEN` 转发到自定义 host。
- 如果需要使用非默认 host，请先使用 `gc auth login --hostname <host>` 为该 host 建立本地认证。
- 显式支持 `--hostname` 的 auth 子命令仍按该参数读取目标主机认证信息。

## 各命令统一语义

### auth login

- 校验 token 有效性
- 成功后写入本地配置（主机名、用户名、token、Git protocol）
- 若同时存在环境变量，后续命令仍优先使用环境变量
- 登录目标为默认主机 `gitcode.com` 且设置了 `GC_TOKEN`/`GITCODE_TOKEN` 时（仅检测变量已设置且非空，与活动 token 解析语义一致；token 值不进入任何输出）：
  - 交互模式：保存前会先提示"本次登录在该环境不会生效"并询问是否仍保存（默认否，直接回车即跳过保存；确认接受后同样输出下述指引）
  - 非交互模式：保存后向 stderr 输出根治指引——两条可直接复制执行的方案（可逆停用配置文件中的旧定义 + 当前终端 unset + 新开终端验证；或更新环境变量的值后验证），命令按 `$SHELL` 推断配置文件路径、按平台适配 `sed`（GNU/BSD）或 PowerShell 语法
  - 修改配置文件的命令为可逆形式（注释而非删除；Windows 用户级环境变量为删除并已注明不可逆），会话级命令避免 token 进入 shell history（Windows `Read-Host` 输入会回显，文案已提示）
- 未设置环境变量时，登录成功不再输出环境变量相关提示
- 自定义主机登录不受通用环境变量影响，不输出上述警告
- 未显式传 `--with-token` 时，需要交互式 TTY；非交互环境会直接报错
- `--web` (`-w`): 仅支持默认主机 `gitcode.com`，打开 `https://gitcode.com/setting/token-classic/create` 新建访问令牌页面，再继续在终端中完成登录
- `--web` 与 `--with-token` 是互斥的认证模式，不能同时使用
- 自定义主机不支持 `--web`；请先核对目标主机，再使用 `gc auth login --hostname <host>` 在本地交互终端登录
- `--git-protocol` (`-p`): 选择 Git 操作使用的协议，支持 `https`（默认）和 `ssh`

### auth status

- 按统一优先级解析当前活动 token
- 显示 token 来源：`GC_TOKEN`、`GITCODE_TOKEN` 或 `config`
- token 失效或过期且来源为环境变量时，不再建议单纯 re-login（环境变量会继续覆盖新登录），改为向 stderr 输出与 `auth login` 同风格的可复制处理方案：方案 A 含完整登录步骤并停用环境变量定义，方案 B 更新环境变量的值；两条方案均以 `gc auth status` 验证收尾，方案 A 的验证在新开终端执行（仅文本输出模式；`--json` 模式不输出该指引，stdout 保持纯 JSON，退出码仍为 `4`）
- token 来源为本地配置时，维持 "To re-authenticate, run: gc auth login" 提示不变
- 显式传 `--hostname` 时，按目标主机读取本地已存储 token，不再被通用环境变量覆盖
- 传 `--show-token` 时会输出完整 token，仅建议由人工在受控终端中临时使用，禁止作为脚本取 token 入口
- `--show-token` 必须在交互式 TTY 中按提示输入 hostname 确认；非交互环境一律拒绝输出完整 token，没有 `--yes` 绕过

### auth token

- 输出当前活动 token
- 与 `auth status` 看到的来源保持一致
- 显式传 `--hostname` 时，输出目标主机已存储 token，不再被通用环境变量覆盖
- 输出 token 前会向 stderr 输出安全警告；不要在日志、截图、脚本管道或 AI 代理上下文中暴露该输出
- 必须在交互式 TTY 中按提示输入 hostname 确认；非交互环境一律拒绝输出完整 token，没有 `--yes` 绕过

### auth logout

- 清理本地配置中的认证信息
- 默认主机下，若任一 `GC_TOKEN` / `GITCODE_TOKEN` 已设置且非空，会把提示写入 stderr，并仅列出这些实际生效的变量；自定义主机登出不受通用环境变量影响，不输出该提示

## 代理配置

GitCode CLI 使用 Go 标准库 HTTP client，自动支持标准代理环境变量：

```bash
# HTTP 代理
export HTTP_PROXY="http://proxy.example.com:8080"

# HTTPS 代理
export HTTPS_PROXY="http://proxy.example.com:8080"

# 不走代理的地址（可选）
export NO_PROXY="localhost,127.0.0.1,.internal"
```

**注意事项**：

1. 代理 URL 必须包含完整协议前缀（`http://` 或 `https://`）
2. 小写环境变量（`http_proxy`、`https_proxy`）也支持，但大写优先级更高
3. 如果代理认证失败或 URL 格式错误，会报错：`proxyconnect tcp: dial tcp: lookup ...`

**常见错误**：

```bash
# 错误：缺少协议前缀
export HTTP_PROXY="proxy.example.com:8080"  # ✗

# 正确：包含完整 URL
export HTTP_PROXY="http://proxy.example.com:8080"  # ✓
```
