# 编码规范

本文档定义 gitcode-cli 项目的编码规范。

## 职责

定义代码命名、文件组织、错误处理和基本 Go 风格要求。

## 适用场景

- 新增命令
- 修改 API、config、git 或浏览器相关实现
- 调整错误处理和代码结构

## 必须

- 遵守 Go 命名习惯
- 使用 `gofmt`
- 使用清晰的错误包装
- 保持函数和文件结构可维护

## 禁止

- 引入循环依赖
- 滥用全局变量
- 使用不符合 Go 习惯的命名

## 同步要求

- 代码结构调整但不影响行为时，至少检查测试与示例是否仍成立
- 命令行为变化时，不仅改代码，还要检查 `docs/COMMANDS.md`

## 不负责什么

- 提交与合并流程
- 测试执行顺序
- 文档同步总规则
- PR blocker 判定

## 命名规范

### 包名
- 小写、简短、有意义
- 不使用下划线或驼峰

```go
package config    // ✅ 正确
package configUtils  // ❌ 错误
package config_utils  // ❌ 错误
```

### 导出名称
- 导出的函数、类型、常量使用大驼峰（PascalCase）

```go
// ✅ 正确
func NewConfig() (*Config, error)
type UserManager struct {}
const DefaultHost = "gitcode.com"

// ❌ 错误
func newConfig() (*Config, error)  // 内部函数
func Get_config()  // 下划线
```

### 内部名称
- 内部（未导出）的函数、变量使用小驼峰（camelCase）

```go
// ✅ 正确
func parseConfig(data []byte) (*Config, error)
func getUserInfo(id int) *User

// ❌ 错误
func ParseConfig(data []byte) (*Config, error)  // 应该导出
func get_user_info(id int) *User  // 下划线
```

### 常量
- 导出常量使用大驼峰
- 未导出常量使用小驼峰

```go
const (
    DefaultHost    = "gitcode.com"  // 导出
    defaultTimeout = 30             // 内部
)
```

### 接口
- 单方法接口以 "-er" 结尾

```go
// ✅ 正确
type Reader interface { Read() }
type Writer interface { Write() }

// ❌ 错误
type Read interface { Read() }
```

## 文件结构

### import 顺序
按以下顺序分组，组间空行分隔：
1. 标准库
2. 第三方库
3. 内部包

```go
package xxx

import (
    "context"
    "fmt"
    "net/http"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"

    "gitcode.com/gitcode-cli/cli/pkg/config"
    "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)
```

### 文件内容顺序
```go
package xxx

import (...)

const (...)    // 常量

type (...)     // 类型定义

func New()     // 构造函数（放前面）

func (x *Xxx) Public() {}   // 公开方法

func (x *Xxx) private() {}  // 私有方法

func helperFunc() {}        // 辅助函数
```

## 错误处理

### 简单错误
```go
var ErrNotFound = errors.New("not found")
```

### 包装错误
```go
if err != nil {
    return fmt.Errorf("failed to get config: %w", err)
}
```

### 错误信息
- 小写开头，不以句号结尾
- 描述发生了什么，不是为什么

```go
// ✅ 正确
return fmt.Errorf("failed to connect to server")
return fmt.Errorf("config file not found")

// ❌ 错误
return fmt.Errorf("Failed to connect to server.")  // 大写、句号
return fmt.Errorf("the server is down")  // 不是描述发生了什么
```

### 禁止裸 panic
- 不得用 `panic` 处理可预见错误；业务/运行期错误必须以 `error` 返回，由调用方处理。
- 构造期（命令装配、flag 注册/注解等）的「不可能发生」编程错误，若调用方签名无法传播 `error`（如 `NewCmd*` 返回 `*cobra.Command`），且失败仅影响非功能性元数据（如 schema/export 注解），可降级为 stderr 告警 + 继续运行，**不得让进程崩溃**。
- 确需 fail-fast 时，用显式 `Must*` 前缀命名 + 注释说明，仅在构造期对真正不可能的编程错误使用。
- cobra 等第三方库自身的 panic 不在本规则范围（属其内部约定）。

```go
// ✅ 正确：返回 error
func SetFlagEnum(cmd *cobra.Command, name string, values ...string) error {
    if err := cmd.Flags().SetAnnotation(name, FlagEnumAnnotation, values); err != nil {
        return fmt.Errorf("failed to annotate flag %q: %w", name, err)
    }
    return nil
}

// ✅ 构造期降级：调用方无法传播 error，降级告警不崩溃
func SetFlagEnumOrWarn(cmd *cobra.Command, name string, values ...string) {
    if err := SetFlagEnum(cmd, name, values...); err != nil {
        fmt.Fprintf(os.Stderr, "gc: warning: %v\n", err)
    }
}

// ❌ 错误：裸 panic 让进程崩溃
func SetFlagEnum(cmd *cobra.Command, name string, values ...string) {
    if err := cmd.Flags().SetAnnotation(name, FlagEnumAnnotation, values); err != nil {
        panic(fmt.Sprintf("failed to annotate flag %q: %v", name, err))
    }
}
```

### API 调用一致性
- 涉及 GitCode REST API 调用的代码，端点/path/method/body 须与官方 OpenAPI 真相源一致，不类比 GitHub CLI、不猜语义。
- 官方 OpenAPI 参考仓：`api-doc/` submodule（`git submodule update --init` 获取；锁定 commit，需定期 `git submodule update --remote api-doc` sync）；submodule 不可达时 fallback 临时 clone `git@gitcode.com:gitcode-cli/gc-api-doc.git`。结构化定义在 `api-doc/data/official/openapi.json`，按领域分目录的端点文档在 `api-doc/docs/api/<domain>/`。
- 现有 `api/` 包函数端点实测报错（404/400）或 body 形状不确定时，以参考仓为准修正，不得绕过（如改用 curl+token 猜测）。
- 详细流程要求见 [开发工作流程 §5.9.B](../workflows/development-workflow.md#59-横切编排资产)。

## 代码风格

### 行长度
- 最大 120 字符
- 超长行适当换行

### 函数长度
- 单个函数不超过 50 行
- 超过时应拆分为多个函数
- 帮助文本长到会突破上限时，用包级 var 形态提取（见命令开发模板"帮助文本形态"一节），构造器不设豁免

### 注释
- 导出的函数、类型必须有注释
- 注释以函数名开头

```go
// NewConfig creates a new configuration instance.
func NewConfig() (*Config, error) {
    // ...
}

// GetUser returns the user with the given ID.
func (s *UserService) GetUser(id int) (*User, error) {
    // ...
}
```

## 代码组织

### 目录结构
```
pkg/cmd/xxx/          # 命令实现
├── xxx.go            # 主命令
├── xxx_test.go       # 单元测试
└── subcommand.go     # 子命令（如有）

pkg/config/           # 配置管理
internal/             # 内部包
├── authflow/         # 认证流程
└── prompter/         # 交互提示

api/                  # API 客户端
├── client.go         # 客户端
├── queries_issue.go  # Issue 相关
└── queries_pr.go     # PR 相关
```

### 避免的问题
- ❌ 循环导入
- ❌ 过深的嵌套（最多 3 层）
- ❌ 过多的函数参数（最多 5 个）
- ❌ 全局变量

## 参考

- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go)

---

**最后更新**: 2026-03-26
