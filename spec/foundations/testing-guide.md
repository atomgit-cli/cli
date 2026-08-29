# 测试指南

本文档说明 gitcode-cli 项目的测试方法和规范。

## 职责

定义测试覆盖要求、真实命令验证范围和测试仓库边界。

## 适用场景

- 新增功能后补测试
- 修复命令行为回归
- 准备提交前做验证

## 必须

- 新功能和行为变更补测试
- 命令行为变更至少做一个真实命令验证
- 真实命令测试只使用 `infra-test/*`

## 禁止

- 只跑单测不做必要的真实命令验证
- 使用个人仓库或生产仓库测试
- 把测试流程要求和提交流程混写在这里

## 同步要求

- 命令行为变化时同步测试用例和相关回归说明
- 回归矩阵变化时同步 `docs/REGRESSION.md`

## 不负责什么

- PR 提交流程
- 代码风格
- 文档治理总规则
- 合并门禁判定

## 单元测试

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./pkg/cmd/issue/...

# 运行特定测试用例
go test -run TestLabelCmd ./pkg/cmd/issue/label/...

# 查看测试覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 运行集成测试
go test -tags=integration ./...
```

### 测试文件命名
- 测试文件与源文件同目录
- 命名格式：`<source>_test.go`

```
pkg/cmd/issue/label/
├── label.go        # 源文件
└── label_test.go   # 测试文件
```

### 测试函数命名
- 函数名以 `Test` 开头
- 描述测试场景

```go
func TestLabelCmd(t *testing.T) {}
func TestLabelCmdWithMultipleLabels(t *testing.T) {}
func TestLabelCmdWithError(t *testing.T) {}
```

### 测试用例模板

```go
func TestXxxCommand(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name    string
        args    []string
        wantErr bool
    }{
        {
            name:    "normal case",
            args:    []string{"--flag", "value"},
            wantErr: false,
        },
        {
            name:    "error case",
            args:    []string{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // 测试逻辑
        })
    }
}
```

### 并行测试

使用 `t.Parallel()` 可减少 CI 执行时间，但**仅在确认无共享可变状态时才允许并行化**。使用 mock HTTP 并不是并行安全的充分条件——必须逐项确认所有可变状态均已隔离。

**前置条件（添加 `t.Parallel()` 前必须逐条确认）：**

- 并行子测试之间**不共享**任何可变状态，包括但不限于：
  - `IOStreams` 实例（每个子测试必须构造独立的 `iostreams.Test()`）
  - `Options` 结构体指针（每个子测试必须构造独立的 Options，或使用值类型 + `tt := tt`）
  - 环境变量（`os.Setenv` / `os.Unsetenv`）
  - 进程工作目录（`os.Chdir`）
  - 临时文件 / 目录（每个子测试使用独立的 `t.TempDir()`）
  - 包级全局变量
  - 默认 `http.Client` / `http.Transport`
- 仅**读取**共享对象（如只读字符串、不可变配置）是安全的；**写入**共享对象（包括赋值、`SetXxx`、写入 buffer）是禁止的

**规则：**

- 顶层 `Test*` 函数：在函数体首行调用 `t.Parallel()`
- 子测试（`t.Run` 回调）：在回调体首行调用 `t.Parallel()`
- **禁止**在调用 `t.Parallel()` 的测试中使用 `t.Setenv()`：Go 中 `t.Setenv` 会在并行测试中 panic。需要环境变量的测试不应调用 `t.Parallel()`
- `t.Setenv()` 在父级调用时，子测试仍可调用 `t.Parallel()`（因为父级未并行）
- 使用 `os.Setenv` / `os.Unsetenv` 的测试**不得**调用 `t.Parallel()`（与 `t.Setenv` 同理）
- Go 1.22+ 循环变量已按值捕获，`tt := tt` 不再严格必要；保留该写法作为兼容习惯是允许的，但不应将其作为强制规则

**示例：**

```go
// 可并行化的测试：每个子测试构造独立的 IOStreams 和 Options
func TestListCmd(t *testing.T) {
    t.Parallel()
    tests := []struct{ ... }{ ... }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            io, _, _, _ := iostreams.Test()  // 每个子测试独立 IOStreams
            f := cmdutil.TestFactory()
            // 使用 testutil.NewRoundTripFunc 的 mock HTTP
        })
    }
}

// 不可并行化的测试（使用 t.Setenv）
func TestListRunJSON(t *testing.T) {
    // 不调用 t.Parallel()
    t.Setenv("GC_TOKEN", "test-token")
    f := cmdutil.TestFactory()
    // ...
}

// 不可并行化的测试（子测试共享 IOStreams 并修改其字段）
func TestGetBody(t *testing.T) {
    io, _, _, _ := iostreams.Test()  // 共享 IOStreams
    // 子测试会写入 io.In，不可并行
}
```

### 测试覆盖率要求
- 新功能代码覆盖率 ≥ 70%
- 核心模块覆盖率 ≥ 80%

## 实际命令测试

**重要：单元测试无法覆盖所有场景，必须进行实际命令测试！**

### 测试仓库

**只能使用以下测试仓库：**

| 仓库 | 用途 |
|------|------|
| `infra-test/gctest1` | 主要测试仓库（首选） |
| `infra-test` 组织下其他仓库 | 其他测试场景 |

**禁止行为：**
- ❌ 使用个人仓库测试
- ❌ 使用其他组织或用户的仓库测试
- ❌ 使用 `gitcode-cli/cli` 测试
- ❌ 在生产环境测试

### 测试前准备

```bash
# 完成认证
gc auth login

# 验证认证状态
./gc auth status
```

### 测试步骤

1. **构建本地版本**
   ```bash
   go build -o ./gc ./cmd/gc
   ```

2. **优先执行核心回归脚本**
   ```bash
   ./scripts/regression-core.sh
   go test -tags=system ./tests/system
   ./tests/system/run.sh --read
   ```

3. **按需补充 issue 相关测试命令**
   ```bash
   # 示例：测试 Issue label 命令
   ./gc issue label 1 --add bug -R infra-test/gctest1
   ./gc issue label 1 --list -R infra-test/gctest1

   # 示例：测试 PR 创建
   ./gc pr create --title "Test PR" --body "Test body" -R infra-test/gctest1
   ```

4. **验证结果**
   - 检查命令输出是否正确
   - 在 Web 界面验证操作结果
   - 检查错误信息是否清晰

### 核心回归矩阵

优先使用 [docs/REGRESSION.md](../../docs/REGRESSION.md) 和 `go test -tags=system ./tests/system` 执行结构化真实命令系统测试；`./tests/system/run.sh --read` 保留为直接 shell 入口，`./scripts/regression-core.sh` 保留为最小冒烟回归入口。

默认覆盖：
- `auth login/status/token/logout`
- `repo view`
- `issue list/view`
- 非 Git 目录错误路径

可选写路径：
- `pr create`，仅在显式提供测试仓库和 head 分支时执行

### 测试检查清单

- [ ] 正常流程测试通过
- [ ] 边界条件测试通过
- [ ] 错误处理测试通过
- [ ] 输出格式正确
- [ ] 错误信息清晰

## 测试用例规范

### 覆盖范围
每个新命令的测试用例应覆盖：

| 类型 | 说明 | 示例 |
|------|------|------|
| 正常流程 | 正常使用场景 | `--flag value` |
| 边界条件 | 极端输入 | 空值、最大值、特殊字符 |
| 错误处理 | 预期错误 | 缺少参数、无效输入 |

### 示例

```go
func TestIssueCreate(t *testing.T) {
    tests := []struct {
        name    string
        args    []string
        wantErr bool
    }{
        {
            name:    "create with title and body",
            args:    []string{"--title", "Test", "--body", "Body"},
            wantErr: false,
        },
        {
            name:    "create without title",
            args:    []string{"--body", "Body"},
            wantErr: true,  // 缺少必填参数
        },
        {
            name:    "create with empty title",
            args:    []string{"--title", "", "--body", "Body"},
            wantErr: true,  // 标题为空
        },
    }
    // ...
}
```

## 测试工具

### 表格驱动测试
推荐使用表格驱动测试，子测试使用 `tt := tt` 闭包捕获并调用 `t.Parallel()`：

```go
tests := []struct {
    name string
    input string
    want string
}{
    {"case1", "input1", "output1"},
    {"case2", "input2", "output2"},
}

for _, tt := range tests {
    tt := tt
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        got := process(tt.input)
        if got != tt.want {
            t.Errorf("got %q, want %q", got, tt.want)
        }
    })
}
```

## 下一步去看哪里

- 如果你在安排测试执行顺序，继续看 [测试流程](../workflows/test-workflow.md)
- 如果你准备提交，继续看 [代码质量门禁规范](./code-quality-gates.md)

### Mock 和 Stub
对于外部依赖，使用接口和 mock：

```go
type MockClient struct {
    response *Response
    err      error
}

func (m *MockClient) Do(req *Request) (*Response, error) {
    return m.response, m.err
}
```

---

**最后更新**: 2026-08-25
