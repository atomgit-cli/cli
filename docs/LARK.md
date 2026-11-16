# 飞书/Lark 通知

GitCode CLI 通过官方 `lark-cli` 工具与飞书/Lark 集成，可向飞书群或自己发送通知。本指南面向首次使用者，覆盖安装、配置、登录到发送的完整流程。

命令行为参考见 [命令手册 · 飞书/Lark 命令](./COMMANDS.md#飞书lark-命令-lark)。

## 设计要点

- **子进程委托**：`gc` 以 `exec` 调用已安装的 `lark-cli`，复用其 OAuth/keychain/令牌刷新，不在 Go 侧重写飞书 OpenAPI 与认证。
- **凭证隔离**：飞书 OAuth 凭证由 `lark-cli` 存入操作系统 keychain，`gc` 不读取、不打印、不日志记录飞书令牌。
- **正文安全扫描**：`--text`/`--markdown`/`--body-file` 正文发送前经 `cmdutil.ScanContentForSecrets` 扫描，若包含当前 `GC_TOKEN`/`GITCODE_TOKEN` 值则拒绝发送。`--file` 附件为二进制路径，不由 `gc` 扫描内容。
- **lazy 安装**：`lark-cli` 缺失时，`gc lark *` 子命令返回明确错误并提示 `gc lark install`，不影响 `gc` 其他命令。

## 前置要求

- 已安装 GitCode CLI（`gc version` 可运行）
- 本机有 Node.js（`node -v`，`lark-cli` 经 npm/npx 安装）

## 1. 安装 lark-cli（一次性）

```bash
gc lark install
```

该命令运行 `npx @larksuite/cli@latest install`。

若安装后当前 shell 的 PATH 未刷新，`gc lark doctor` 会提示。可重开终端，或设置环境变量指向二进制：

```bash
export GC_LARK_CLI_BIN=$(which lark-cli)
```

## 2. 配置飞书应用凭证（一次性，交互式）

```bash
lark-cli config init --new
```

终端会输出一个授权 URL，**在浏览器打开它**完成"创建/绑定飞书应用"。应用凭证存入 OS keychain，`gc` 不接触。

## 3. 登录授权（一次性，交互式）

```bash
lark-cli auth login --recommend
```

同样输出授权 URL，浏览器确认授权。`--recommend` 勾选可自动批准的常用 scope。完成后即为 `lark-cli` 的登录用户。

> 步骤 2、3 属交互式 OAuth，需在真实终端由本人完成，不能由脚本/AI 代行。

## 4. 验证安装与登录

```bash
gc lark doctor          # 友好输出
gc lark doctor --json   # 供脚本/Agent 消费
```

看到 `installed: true` 与 `login ready` 即就绪。也可用 `gc lark auth status` 查看登录详情与已授权 scope。

## 5. 发送通知

### 5.1 给自己发（推荐）

```bash
gc lark send --to-self --text "部署完成"
gc lark send --to-self --markdown "## 发布\n- v1.2.3 上线"
echo "CI 通过" | gc lark send --to-self --body-file -   # 从 stdin 读正文
```

`--to-self` 自动从 `lark-cli auth status` 解析你的 open_id，默认以 **bot 身份**发送（bot→你 P2P 单聊）。**无需任何群、不受外部群限制**，是"每人用自己 bot 给自己发通知"最省事的路径。

### 5.2 发到一个群

```bash
gc lark send --chat-id oc_xxx --as bot --text "hello"
```

要求：你的应用 bot **在该群内**（飞书群设置 → 群机器人 → 添加"应用机器人"）。

### 5.3 给指定用户发单聊

```bash
gc lark send --user-id ou_xxx --as bot --text "hi"
```

### 5.4 默认群（免每次传 chat-id）

```bash
gc lark config set --default-chat oc_xxx   # 持久化到 ~/.config/gc/lark.json
gc lark send --text "build green"          # 自动发到默认群
# 或环境变量覆盖：
export GC_LARK_DEFAULT_CHAT_ID=oc_xxx
```

默认群解析优先级：`--chat-id` > `GC_LARK_DEFAULT_CHAT_ID` 环境变量 > `~/.config/gc/lark.json` 的 `default_chat_id`。

### 5.5 预览与 JSON 输出

```bash
gc lark send --to-self --text "hi" --dry-run     # 预览将执行的 lark-cli 调用，不发送
gc lark send --to-self --text "hi" --json        # 结构化输出（供 AI 代理/脚本）
```

## 令牌刷新

- 访问令牌约 2 小时过期；刷新令牌约 7 天过期。
- 只要 7 天内用过一次 `gc lark send`，`lark-cli` 自动刷新访问令牌，无需重新登录。
- 超过 7 天未使用，需重跑第 3 步 `lark-cli auth login`。

## 常见问题

### `Error: lark-cli is not installed`

未安装 `lark-cli`。运行 `gc lark install`。

### `missing required scope(s): im:message.send_as_user`

以 **user 身份**发消息需要该 scope，`lark-cli auth login --recommend` 默认未勾选。bot 身份（`--to-self` 与 `--as bot`）不需要它，一般够用。确需 user 身份：

```bash
lark-cli auth login --scope "im:message.send_as_user"
```

> 注意：该 scope 是飞书内测权限，对外共享机器人不支持，未必能批。

### `Bot/User can NOT be out of the chat`（错误码 230002）

应用 bot 不在目标群内。要么把"应用机器人"（非自定义 webhook 机器人）拉进群，要么改用 `--to-self` 自通知。

### `The operator or invited bots does NOT have the authority to manage external chats`（错误码 232033）

目标群是**外部群**（含跨租户成员）。把应用 bot 合入外部群需给应用开"对外共享"能力 + 实名认证 + 发布审核（见 [飞书开放平台 · 机器人支持外部群](https://open.feishu.cn/document/develop-robots/add-bot-to-external-group?lang=zh-CN)）。自通知 `--to-self` 可绕开此限制。

### `Bot has NO availability to this user`

外部用户未与机器人建立单聊会话。需对方先主动与机器人发起一次单聊。

## 环境变量

| 变量 | 说明 |
|------|------|
| `GC_LARK_CLI_BIN` | 覆盖 `lark-cli` 二进制路径，优先于 PATH 查找（安装后 PATH 未刷新时用） |
| `GC_LARK_DEFAULT_CHAT_ID` | 覆盖 `gc lark send` 的默认飞书群 id，优先于 `~/.config/gc/lark.json` |

## 安全约束

- 飞书令牌始终由 `lark-cli` keychain 管理，`gc` 不读取 `auth.json`/`access_token`/`refresh_token`，不打印飞书凭证。
- `lark-cli` 子进程不继承 `gc` 的 stdin，且环境中的 `GC_TOKEN`/`GITCODE_TOKEN` 会被剥离（纵深防御）。
- 默认群配置 `~/.config/gc/lark.json` 复用 `config.SecureWriteFile` 写入：拒绝符号链接、`0600` 文件/`0700` 目录、temp+rename 原子替换（成功读到的数据必为完整旧文件或完整新文件）。
- 不得把飞书令牌、密钥写入 issue/PR/comment 内容。`gc lark send` 的 `--text`/`--markdown`/`--body-file` 正文在发送前会扫描当前 `GC_TOKEN`/`GITCODE_TOKEN` 值。

## 参考

- 命令参考：[命令手册 · 飞书/Lark 命令](./COMMANDS.md#飞书lark-命令-lark)
- `lark-cli` 仓库：https://github.com/larksuite/cli
- 飞书开放平台 · 机器人支持外部群：https://open.feishu.cn/document/develop-robots/add-bot-to-external-group?lang=zh-CN
