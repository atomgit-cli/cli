# 命令开发模板

本文档提供 gitcode-cli 新命令开发的模板和示例。

## 命令结构

### 文件位置
```
pkg/cmd/<command>/<command>.go
pkg/cmd/<command>/<command>_test.go
pkg/cmd/<command>/subcommand.go  # 子命令（如有）
```

## 基本模板

```go
// pkg/cmd/xxx/xxx.go
package xxx

import (
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/MakeNowJust/heredoc/v2"
    "github.com/spf13/cobra"

    "gitcode.com/gitcode-cli/cli/api"
    cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
    "gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type XxxOptions struct {
    IO         *iostreams.IOStreams
    HttpClient func() (*http.Client, error)

    // Arguments
    Repository string

    // Flags
    Option string
    JSON   bool
}

func NewCmdXxx(f *cmdutil.Factory, runF func(*XxxOptions) error) *cobra.Command {
    opts := &XxxOptions{
        IO:         f.IOStreams,
        HttpClient: f.HttpClient,
    }

    cmd := &cobra.Command{
        Use:   "xxx",
        Short: "Short description",
        Long: heredoc.Doc(`
            Longer description of the command.
        `),
        Example: heredoc.Doc(`
            # Example 1
            $ gc xxx --option value

            # Example 2
            $ gc xxx -R owner/repo
        `),
        Args: cobra.NoArgs,  // 或 cobra.ExactArgs(1)
        RunE: func(cmd *cobra.Command, args []string) error {
            if runF != nil {
                return runF(opts)
            }
            return xxxRun(opts)
        },
    }

    // 添加 flags
    cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
    cmd.Flags().StringVarP(&opts.Option, "option", "o", "", "Option description")
    cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output as JSON")

    return cmd
}

func xxxRun(opts *XxxOptions) error {
    cs := opts.IO.ColorScheme()

    // 1. 获取 HTTP 客户端
    httpClient, err := opts.HttpClient()
    if err != nil {
        return fmt.Errorf("failed to create HTTP client: %w", err)
    }

    // 2. 创建 API 客户端
    client := api.NewClientFromHTTP(httpClient)
    token := getEnvToken()
    if token == "" {
        return fmt.Errorf("not authenticated. Run: gc auth login")
    }
    client.SetToken(token, "environment")

    // 3. 解析仓库
    owner, repo, err := parseRepo(opts.Repository)
    if err != nil {
        return err
    }

    // 4. 执行操作
    // ... API 调用 ...

    // 5. 输出结果
    if opts.JSON {
        if err := json.NewEncoder(opts.IO.Out).Encode(map[string]string{"status": "ok"}); err != nil {
            return fmt.Errorf("failed to encode JSON output: %w", err)
        }
        return nil
    }

    fmt.Fprintf(opts.IO.Out, "%s Success\n", cs.Green("✓"))

    return nil
}

func parseRepo(repo string) (string, string, error) {
    if repo == "" {
        return "", "", fmt.Errorf("no repository specified. Use -R owner/repo")
    }
    parts := strings.Split(repo, "/")
    if len(parts) != 2 {
        return "", "", fmt.Errorf("invalid repository format: %s", repo)
    }
    return parts[0], parts[1], nil
}

func getEnvToken() string {
    if token := os.Getenv("GC_TOKEN"); token != "" {
        return token
    }
    return os.Getenv("GITCODE_TOKEN")
}
```

## 帮助文本形态（内联与包级 var）

帮助文本有两种规范形态，按一条机械判据选择：**帮助文本长到会使构造器超过 50 行函数上限时，提取为包级 var；否则内联**（见上方基本模板的默认形态）。

包级 var 形态（长帮助文本的标准逃生通道，先例：`pkg/cmd/auth/gitcredential/gitcredential.go`）：

```go
// gitCredentialLong is the long help text for the command. Package-level
// because the constructor must stay within the 50-line function limit.
var gitCredentialLong = heredoc.Doc(`
    Read the git credential helper protocol from stdin and ...
`)

var gitCredentialExample = heredoc.Doc(`
    # What git runs (git appends the operation argument)
    $ gc auth git-credential get
`)

func NewCmdGitCredential(f *cmdutil.Factory, runF func(*GitCredentialOptions) error) *cobra.Command {
    opts := &GitCredentialOptions{...}

    cmd := &cobra.Command{
        Use:     "git-credential [operation]",
        Short:   "Provide stored GitCode credentials to git (helper protocol)",
        Long:    gitCredentialLong,
        Example: gitCredentialExample,
        ...
    }
    ...
}
```

规则：

- 变量命名 `<command>Long` / `<command>Example`，放在文件顶部（Options 结构体前后），以符号名开头的注释说明提取动机（先例 gitcredential 的两个 var 早于本注释要求，补注释属可选清理）
- `coding-standards.md` 的"单个函数不超过 50 行"**保持严格**：本形态是标准逃生通道，构造器没有任何豁免——不要为长文本放宽上限
- 存量内联超限的构造器（如 `NewCmdLogin`）按本判据属可选清理项，不强制回填；**新命令必须遵循本判据**，不得新增内联超限构造器

## API 客户端用法

### 创建客户端
```go
httpClient, _ := f.HttpClient()
client := api.NewClientFromHTTP(httpClient)
client.SetToken(token, "environment")
```

### GET 请求
```go
var user User
err := client.Get("/repos/"+owner+"/"+repo+"/user", &user)
```

### POST 请求
```go
var result Result
err := client.Post("/repos/"+owner+"/"+repo+"/issues", &CreateOptions{
    Title: "Title",
    Body:  "Body",
}, &result)
```

### PATCH 请求
```go
err := client.Patch("/repos/"+owner+"/"+repo+"/issues/"+number, &UpdateOptions{
    Title: "New Title",
}, &result)
```

### DELETE 请求
```go
err := client.Delete("/repos/"+owner+"/"+repo+"/issues/"+number)
```

## 配置访问

```go
cfg, _ := f.Config()

// 获取 Git 协议
protocol := cfg.GitProtocol(hostname).Value

// 获取认证配置
authCfg := cfg.Authentication()
token, source := authCfg.ActiveToken(hostname)
```

## 输出处理

### 颜色输出
```go
cs := opts.IO.ColorScheme()

// 成功消息
fmt.Fprintf(opts.IO.Out, "%s Success\n", cs.Green("✓"))

// 错误消息
fmt.Fprintf(opts.IO.ErrOut, "%s Error\n", cs.Red("!"))

// 粗体
fmt.Fprintf(opts.IO.Out, "%s\n", cs.Bold(title))
```

### 表格输出
```go
tp := tableprinter.New(opts.IO, tableprinter.WithHeader("ID", "TITLE", "STATE"))

for _, item := range items {
    tp.AddField(item.ID)
    tp.AddField(item.Title)
    tp.AddField(item.State)
    tp.EndRow()
}

tp.Render()
```

## 交互提示

```go
err := cmdutil.ConfirmOrAbort(cmdutil.ConfirmOptions{
    IO:       opts.IO,
    Yes:      opts.Yes,
    Expected: "owner/repo",
    Prompt:   "Type the repository name to confirm: ",
})
```

## 测试模板

```go
// xxx_test.go
package xxx

import (
    "testing"

    cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdXxx(t *testing.T) {
    tests := []struct {
        name    string
        args    []string
        wantErr bool
    }{
        {
            name:    "normal case",
            args:    []string{"--option", "value"},
            wantErr: false,
        },
        {
            name:    "error case",
            args:    []string{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            f := cmdutil.TestFactory()
            cmd := NewCmdXxx(f, func(opts *XxxOptions) error {
                return nil
            })
            cmd.SetArgs(tt.args)

            err := cmd.Execute()
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Function Injection for Testability

本项目的命令构造函数统一接受可选的 `runF func(*Options) error` 参数，这是一个**函数注入（Function Injection）**模式，目的是让测试代码可以在不依赖真实网络、认证、文件系统等外部状态的情况下验证命令行为。

### 为什么不用接口 Mock？

Go 中常见的 mock 方式需要先定义接口，再生成 mock 实现。函数注入更轻量：

- 不需要额外的接口定义
- 不需要 mock 生成工具
- 测试代码可以直接在闭包中定制行为
- 新增可注入点只需在 Options 结构体中增加一个函数字段

### 主要模式：runF 注入

每个命令的 `NewCmd*` 构造函数都接受可选的 `runF` 参数：

```go
func NewCmdXxx(f *cmdutil.Factory, runF func(*XxxOptions) error) *cobra.Command {
    opts := &XxxOptions{
        IO:         f.IOStreams,
        HttpClient: f.HttpClient,
    }

    cmd := &cobra.Command{
        // ...
        RunE: func(cmd *cobra.Command, args []string) error {
            if runF != nil {
                return runF(opts)
            }
            return xxxRun(opts)
        },
    }
    // ...
}
```

测试代码通过注入自定义 `runF` 跳过复杂的外部依赖：

```go
// 基础参数验证测试：不需要真实 API 调用
cmd := NewCmdXxx(f, func(opts *XxxOptions) error {
    // 验证 opts 的字段已正确解析
    if opts.Repository != "owner/repo" {
        t.Errorf("expected owner/repo, got %s", opts.Repository)
    }
    return nil
})
cmd.SetArgs([]string{"-R", "owner/repo"})
err := cmd.Execute()
// err 为 nil 表示参数解析和验证通过
```

```go
// 错误路径测试：模拟 API 调用失败
cmd := NewCmdXxx(f, func(opts *XxxOptions) error {
    return fmt.Errorf("simulated API error")
})
cmd.SetArgs([]string{"-R", "owner/repo"})
err := cmd.Execute()
// 验证错误传播正确
```

### 辅助模式：行为函数注入

当命令依赖特定行为函数（如仓库名解析、文件操作等），可以在 Options 结构体中直接注入函数字段：

```go
type ForkOptions struct {
    IO         *iostreams.IOStreams
    HttpClient func() (*http.Client, error)
    Config     func() (cmdutil.Config, error)

    // 可注入的行为函数
    ParseRepo func(string) (*git.Repo, error)
}
```

构造函数在初始化时设置默认行为，测试可以替换：

```go
func NewCmdFork(f *cmdutil.Factory, runF func(*ForkOptions) error) *cobra.Command {
    opts := &ForkOptions{
        IO:         f.IOStreams,
        HttpClient: f.HttpClient,
        Config:     f.Config,
        ParseRepo:  git.ParseRepo, // 默认实现
    }
    // ...
}
```

测试中注入 mock 行为：

```go
cmd := NewCmdFork(f, func(opts *ForkOptions) error {
    // 使用注入的 ParseRepo 验证行为
    repo, err := opts.ParseRepo("owner/repo")
    // ...
    return nil
})
// 也可以直接替换 ParseRepo：
// opts.ParseRepo = func(s string) (*git.Repo, error) { ... }
```

### 何时增加函数注入点

遵循以下原则：

| 场景 | 做法 |
|------|------|
| 命令依赖外部 API 调用 | 已有 `runF` 模式覆盖，不需要额外注入 |
| 命令依赖特定工具函数（解析、转换、文件 I/O） | 在 Options 中增加函数字段，提供默认实现 |
| 行为函数在 2+ 测试中需要替换 | 应在 Options 中暴露为可注入函数 |
| 仅在一个测试中需要特殊行为 | 优先在 `runF` 闭包中内联处理 |

### 与 cmdutil.TestFactory 配合

测试中通常使用 `cmdutil.TestFactory()` 创建轻量的 Factory：

```go
func TestNewCmdXxx(t *testing.T) {
    f := cmdutil.TestFactory()
    cmd := NewCmdXxx(f, func(opts *XxxOptions) error {
        return nil
    })
    cmd.SetArgs([]string{"--json"})
    if err := cmd.Execute(); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

`TestFactory()` 返回的 Factory 提供内存中的 IOStreams 和可用的 HttpClient，无需真实配置文件或认证 token。

---

**最后更新**: 2026-06-27
