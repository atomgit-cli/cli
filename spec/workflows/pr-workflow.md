# PR 流程

本文档定义 Pull Request 的生命周期管理、自检要求和合并规则。

## 职责

- 定义 PR 的标准状态流转
- 约束作者自检与多角色独立执行主体评审的边界
- 明确合并前必须具备的证据

## 流程概览

```
创建分支
→ 开发代码
→ 编写测试
→ 提交代码
→ 创建 PR draft
→ 作者自检
→ 风险分级
→ 第一轮多角色评审（基础角色）
→ 发现问题则修复 → 第二轮多角色评审（深度角色）
→ approved
→ merged
```

## 1. 创建分支

### 分支命名规范

| 类型 | 命名格式 | 示例 |
|------|----------|------|
| BUG 修复 | `bugfix/issue-<number>` | `bugfix/issue-33` |
| 新特性 | `feature/issue-<number>` | `feature/issue-23` |
| 文档更新 | `docs/issue-<number>` | `docs/issue-5` |
| 重构 | `refactor/issue-<number>` | `refactor/issue-19` |

### 创建步骤

```bash
git checkout main
git pull
git checkout -b feature/issue-23
```

## 2. PR 标签规范

PR 至少应补齐以下维度：

- 类型：`type/bug`、`type/feature`、`type/docs`、`type/refactor`
- 状态：`status/draft`、`status/self-checked`、`status/ready-for-review`、`status/changes-requested`、`status/approved`、`status/merged`
- 风险：`risk/low`、`risk/medium`、`risk/high`
- 范围：开放命名空间，按命令/模块领域命名（示例：`scope/auth`、`scope/repo`、`scope/issue`、`scope/pr`、`scope/release`、`scope/docs`、`scope/api`、`scope/actions`、`scope/ci` 等，可按需扩展）

## 3. 开发与测试

### 开发规范

- 遵循 [编码规范](../foundations/coding-standards.md)
- 使用 [命令开发模板](../foundations/command-template.md)
- 命令行为变化后同步相关文档

### 最低测试要求

```bash
go test ./...
go build -o ./gc ./cmd/gc
```

如果改动影响具体命令行为，还必须补：

```bash
go test ./pkg/cmd/xxx/...
./gc xxx -R infra-test/gctest1
```

## 4. 提交代码

### 提交规范

使用 Conventional Commits：

- `feat:` 新功能
- `fix:` Bug 修复
- `docs:` 文档更新
- `test:` 测试相关
- `refactor:` 重构

### 提交步骤

```bash
git add <files>
git commit -m "feat: add issue label command

Refs #23

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
git push -u origin feature/issue-23
```

### 提交要求

- 单次提交应保持范围清晰
- 不把无关改动混入同一个 PR
- 未经过自检的代码不应直接宣称可合并

## 5. 创建 PR

### 创建命令

```bash
gc pr create --title "feat: add issue label command" \
  --body "## 变更内容

- 新增 gc issue label 命令

## 关联

Refs #23" \
  --base main \
  -R gitcode-cli/cli
```

创建后先补 `status/draft`，不要一开始就当作 ready for review。

### Issue 关联

PR body 必须含 `Closes #XXX`（或 `Fixes #XXX` / `Resolves #XXX`）关联对应 issue：

- **PR body 的 `Closes #XXX` 在 merge 后触发 GitCode 自动关闭关联 issue**
- **GitCode 对 PR body 里的裸 `#NNN`（即无 `Closes`/`Fixes`/`Resolves`/`Refs` 等关键字前缀的 `#NNN`）也触发自动关闭** — PR body 自检/描述/未覆盖项文本不得出现裸 `#NNN`（除 `Closes #XXX` 行外），如需引用其他 issue 用文字描述（如"后续 run view 交付"）
- **commit message 的 `Closes #XXX` 不被 GitCode 识别为自动关闭**（实测：某 issue commit message 含 `Closes` 但 issue 未自动关闭，需手动 close；改用 PR body `Closes` 后 merge 即自动关闭）
- 实测：PR body 自检文本裸写其他 issue 编号（如"jobs(#NNN)"在"未覆盖项"段落）导致 merge 时未交付 issue 被误关；后续 PR body 不得裸写 `#NNN`
- 用 `Refs #XXX` 仅引用不关闭，适用于非修复型 PR（如重构、文档）

示例：

```bash
gc pr create --title "fix: ..." --body-file pr-self-check.md
```

PR body（self-check）末尾含：

```
**Closes #NNN**
```

## 6. 作者自检

作者自检是必需步骤，但不等同于多角色评审。

### 作者自检模板

```markdown
## 作者自检

- 作者主体标识:
- 根因或实现理由:
- 主要修改:
- 单元测试:
- 构建:
- 实际命令验证:
- 安全审查:
- 文档同步:
- 风险:
- 未覆盖项:
```

### 自检要求

- 自检记录应写在 PR 中
- 自检记录必须包含可追踪的作者主体标识
- 自检完成后，PR 才能进入 `status/self-checked`
- 只有自检，不代表可以直接合并

## 7. 多角色评审（必须执行）

### 7.1 第一轮评审（基础角色）

PR 进入 ready-for-review 前必须完成 **4 个基础评审角色**：

| 角色 | 检查内容 |
|------|----------|
| 代码审查 | 代码逻辑、项目模式遵循、命名规范 |
| 安全审查 | 凭证泄漏、Token 处理、API 安全 |
| 测试审查 | 测试覆盖、测试质量、边界条件 |
| 文档审查 | COMMANDS.md 更新、示例正确 |

### 7.2 第二轮评审（深度角色）

发现问题时执行 **4 个深度评审角色**：

| 角色 | 检查内容 |
|------|----------|
| 架构一致性 | Options 结构、函数命名、与其他命令对比 |
| API 契约 | API 参数、调用时序、错误处理 |
| 边界条件 | 输入验证、空值处理、错误场景 |
| 用户体验 | 帮助文本、错误消息、输出格式 |

### 7.3 评审执行

```bash
# 1. 创建评审团队
TeamCreate

# 2. 启动第一轮 4 个评审 Agent（并行）
Agent(description="代码审查", subagent_type="general-purpose")
Agent(description="安全审查", subagent_type="general-purpose")
Agent(description="测试审查", subagent_type="general-purpose")
Agent(description="文档审查", subagent_type="general-purpose")

# 3. 收集评审结论，发现问题则修复

# 4. 启动第二轮评审（如有需要）
Agent(description="架构一致性", subagent_type="general-purpose")
Agent(description="API 契约", subagent_type="general-purpose")
Agent(description="边界条件", subagent_type="general-purpose")
Agent(description="用户体验", subagent_type="general-purpose")

# 5. 添加评审汇总评论到 PR
gc pr review <number> --comment "## 多角色评审汇总..." -R owner/repo
```

### 7.4 评审汇总要求

评审汇总评论必须包含：
- 各角色评审结论表格
- 发现问题及修复情况
- 后续跟踪 Issues（非阻塞问题）
- 总体评审结论
- 评审执行主体标识

详见 [评审流程](./review-workflow.md)。

## 8. Ready For Review

进入 `status/ready-for-review` 前必须满足：

- PR 已有作者自检记录
- PR 已完成第一轮多角色评审
- 所有评审角色结论为 approved
- 关联 issue 已进入 `status/ready-for-review`
- 所有本地门禁已完成
- 安全审查已完成
- 文档同步已完成
- 风险分级已完成

## 9. Approved 与合并

### Approved 前检查

- [ ] 单元测试通过
- [ ] 构建通过
- [ ] 实际命令测试通过
- [ ] 第一轮多角色评审全部通过
- [ ] 第二轮多角色评审全部通过（如有）
- [ ] 安全审查已完成
- [ ] issue 已有验证记录和开发进度记录
- [ ] PR 已有作者自检记录
- [ ] PR 已有评审汇总评论
- [ ] 文档已同步
- [ ] 风险分级结果已记录
- [ ] `risk/high` 改动已完成人工最终确认
- [ ] 正式审批记录或等效独立评审证据已留档（平台无法记录审批时须按下方"合并授权"规则人工确认）

### 合并授权（AI 代理边界）

- **评审结论不等于合并授权**：AI 代理完成评审后，必须向人工报告评审结论与验证证据，等待人工对**该 PR** 的明确合并指令后才能执行 `gc pr merge`
- 人工授权必须**逐 PR 显式给出**（如"合并 #XXX"）；"继续"、"下一个"等推进类指令不得推定为合并授权
- 平台无法记录正式审批（如 fork MR 的 approve 返回 403）时，AI 代理必须单独上报该限制及已有评审证据，由人工决定是否合并；不得默认"证据留档后即可合并"
- **零正式审批记录**的 PR（含仅作者自评、仅 AI 代理单轮评审等情形）合并前必须人工确认

### 合并命令

```bash
gc pr merge <number> -R gitcode-cli/cli
```

### 合并后操作

```bash
git checkout main
git pull origin main
```

合并后更新状态：

- PR：`status/merged`
- Issue：`status/merged` 或关闭
- 删除本地开发分支：`git branch -d feature/issue-xxx`

## 10. 检查清单

- [ ] 分支已创建
- [ ] 代码已开发
- [ ] 测试已编写并执行
- [ ] PR 已以 `status/draft` 创建
- [ ] 作者自检已完成
- [ ] PR 已进入 `status/self-checked`
- [ ] **第一轮多角色评审已完成**（必须）
- [ ] **第二轮多角色评审已完成**（如有问题）
- [ ] 评审汇总评论已添加
- [ ] PR 已进入 `status/approved`
- [ ] **人工已给出该 PR 的明确合并指令**（AI 代理执行合并的前提）
- [ ] PR 已合并
- [ ] Issue 已关闭

---

**最后更新**: 2026-09-20
