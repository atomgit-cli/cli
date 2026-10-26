# Issue 流程

本文档定义 Issue 的生命周期管理、状态推进和证据要求。

## 职责

- 定义 issue 从创建到关闭的状态流转
- 规定每个阶段必须补充的标签和记录
- 约束 AI 不得绕过验证直接进入开发

## 流程概览

```
创建 Issue
→ triage
→ verified
→ in-progress
→ ready-for-review
→ merged / closed-no-fix
```

## 1. 创建 Issue

### 触发条件

- 发现 Bug
- 需要新功能
- 文档需要更新

### 创建方式

```bash
# 方式一：命令行创建
gc issue create --title "Bug: 描述问题" --body "问题描述" -R owner/repo

# 方式二：在 Web 界面创建
# https://gitcode.com/owner/repo/issues/new
```

模板资产：Web 界面创建可用 `.github/ISSUE_TEMPLATE/` 平台模板（实验证实该路径在 GitCode 与 GitHub 双平台 web UI 生效）；AI 起草以 `docs/ai-templates/issue-create.md` 的四段结构为起点，提交前用 `make validate-ai-record FILE=<草稿> KIND=issue-create` 校验。

### 创建后立即补的最小标签

- 类型标签：`type/bug`、`type/feature`、`type/docs`、`type/refactor`
- 状态标签：`status/triage`
- 范围标签：按模块补 `scope/*`

示例：

```bash
gc issue label <number> --add type/bug,status/triage,scope/pr -R owner/repo
```

## 2. 状态定义

| 状态标签 | 进入条件 | 退出条件 |
|------|------|------|
| `status/triage` | issue 已创建 | 信息足够进入验证，或判定无效 |
| `status/verified` | 已完成复现或确认需求有效 | 准备进入开发 |
| `status/in-progress` | 已创建开发分支并开始修复 | 代码与自检完成 |
| `status/blocked` | 当前存在阻塞 | 阻塞解除 |
| `status/ready-for-review` | 对应 PR 已完成作者自检与风险分级 | 进入独立执行主体评审 |
| `status/merged` | 关联 PR 已合入主干 | 流程结束 |
| `status/closed-no-fix` | 无效、重复、已修复或不做 | 流程结束 |

## 3. 验证问题

**未完成验证，不得开始写代码。**

### 验证步骤

1. 用当前版本执行 issue 中描述的复现步骤
2. 检查 issue 时间线和相关改动，确认不是已修复问题
3. 给出结构化结论

### 验证记录模板

```markdown
## 验证记录

- 当前版本或分支:
- 复现命令:
- 实际结果:
- 结论: 继续修复 / 已修复关闭 / 信息不足待补充
```

### 示例

```bash
gc issue comment <number> --body "## 验证记录

- 当前版本或分支: main
- 复现命令: ./gc pr review 1 --approve -R infra-test/gctest1
- 实际结果: 复现成功，返回 404
- 结论: 继续修复" -R owner/repo
```

验证后更新状态：

```bash
gc issue label <number> --add status/verified -R owner/repo
```

## 4. 进入开发

进入 `status/in-progress` 前必须满足：

- issue 已进入 `status/verified`
- 已创建非 `main` 分支
- 已确认本次修改范围

示例：

```bash
git checkout main
git pull
git checkout -b bugfix/issue-33

gc issue label 33 --add status/in-progress -R gitcode-cli/cli
```

## 5. 开发完成后的 Issue 记录

Issue 在进入 `status/ready-for-review` 前必须补充阶段性说明。

### 进度记录模板

```markdown
## 开发进度

- 根因:
- 主要修改:
- 测试:
- 实际命令验证:
- 安全影响:
- 风险或未覆盖项:
- 关联 PR:
```

### 示例

```bash
gc issue comment 33 --body "## 开发进度

- 根因: review approve 调用了错误 endpoint
- 主要修改: 调整 PR review endpoint 与错误处理
- 测试: go test ./...
- 实际命令验证: ./gc pr review 1 --approve -R infra-test/gctest1
- 安全影响: 已检查无凭证泄漏，本次改动不涉及认证与权限路径
- 风险或未覆盖项: request changes 仍受 API 限制
- 关联 PR: #51" -R gitcode-cli/cli
```

## 6. 关闭规则

### 可以关闭的情况

- 关联 PR 已合入主干
- issue 被明确判定为重复、无效、无需处理
- 当前代码已确认覆盖问题，且已给出验证记录

### 不得关闭的情况

- 代码还在功能分支，未进入主干
- 只有作者自检，没有独立执行主体评审或合并记录
- 只有“已完成实现”的口头说明，没有证据

### 关闭方式

```bash
gc issue comment <number> --body "问题已在 PR #xx 合入主干后关闭" -R owner/repo
gc issue close <number> -R owner/repo
```

如果是不修复关闭，应改用：

```bash
gc issue label <number> --add status/closed-no-fix -R owner/repo
gc issue comment <number> --body "判定为重复/无效/已修复，无需继续开发" -R owner/repo
gc issue close <number> -R owner/repo
```

## 7. 检查清单

- [ ] Issue 已创建
- [ ] 类型、状态、范围标签已补
- [ ] 验证记录已添加
- [ ] 问题已进入 `status/verified`
- [ ] 开发时已进入 `status/in-progress`
- [ ] ready-for-review 前已补开发进度记录
- [ ] Issue 未在主干合入前被错误关闭

---

**最后更新**: 2026-04-03
