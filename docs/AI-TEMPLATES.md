# AI 流程模板

本文件提供 gitcode-cli 仓库中 AI 协作开发常用的固定模板。

它的职责不是重新定义规则，而是把 `spec/` 中已经确定的流程要求沉淀为可直接复用的文本片段，降低 AI 自由发挥空间。

边界说明：

- 本文模板服务 gitcode-cli 仓库内部协作记录
- 本文不适用于外部项目通过 AI 使用 `gc` 的通用说明
- 本文不是项目规则源，正式规则仍以 `spec/` 为准

正式规则仍以 `spec/` 为准，尤其是：

- [开发工作流程](../spec/workflows/development-workflow.md)
- [Issue 流程](../spec/workflows/issue-workflow.md)
- [PR 流程](../spec/workflows/pr-workflow.md)
- [评审流程](../spec/workflows/review-workflow.md)
- [代码质量门禁规范](../spec/foundations/code-quality-gates.md)

## 使用原则

- 模板用于帮助 AI 和人工保持记录格式一致
- 先核远端事实，再填写模板；模板不能替代事实确认
- 模板字段可以补充，但不应删掉关键证据项
- 如果某项未执行，不要留空，应明确写“未执行”及原因
- 作者自检不能替代独立执行主体评审

## 机器校验

仓库提供最小模板校验脚本：

```bash
# 校验模板结构
python3 scripts/validate-ai-record.py --mode template docs/ai-templates/pr-self-check.md

# 校验已填写记录
python3 scripts/validate-ai-record.py --mode record --kind pr-self-check /path/to/pr-self-check.md
```

也可以使用 Makefile 包装入口：

```bash
make validate-ai-template FILE=docs/ai-templates/pr-self-check.md
make validate-ai-templates
make validate-ai-record FILE=/path/to/pr-self-check.md KIND=pr-self-check
```

当前机器校验范围：

- 检查标题是否匹配模板类型
- 检查必填字段是否齐全
- 在 `record` 模式下检查字段值是否为空

当前不负责：

- 验证字段内容是否真实
- 验证远端 issue / PR 状态是否与记录一致
- 替代独立执行主体评审

## 可直接复用的示例文件

以下文件可直接作为 `--body-file` 或 `--comment-file` 的起点：

- [task-start-checklist.md](./ai-templates/task-start-checklist.md)
- [issue-create.md](./ai-templates/issue-create.md)
- [issue-verification.md](./ai-templates/issue-verification.md)
- [issue-progress.md](./ai-templates/issue-progress.md)
- [issue-blocked.md](./ai-templates/issue-blocked.md)
- [issue-close-merged.md](./ai-templates/issue-close-merged.md)
- [issue-close-no-fix.md](./ai-templates/issue-close-no-fix.md)
- [pr-self-check.md](./ai-templates/pr-self-check.md)
- [pr-review-outcome.md](./ai-templates/pr-review-outcome.md)
- [docs-only-self-check.md](./ai-templates/docs-only-self-check.md)

## 0. Issue 创建（四段结构）

适用场景：

- 起草新 issue（bug、feature、test、docs 均适用）
- 模板含起草前查证清单（HTML 注释形态，对平台不可见、对 AI 可见）与验收锚点指引

结构为四段标题式字段（区别于其余模板的列表式字段）：

- `## 背景与证据`（含 `### 问题` 与 `### 证据`，证据须 file:line 或命令输出，不写推测）
- `## 改动建议`（含设计取舍时标注"评审定夺"）
- `## 验收标准`（可测试的判定条件；UX/输出类附示例输出块作为锚点）
- `## 影响范围`

示例命令：

```bash
# 起草后校验结构与章节完整性（record 模式会抓空章节）
python3 scripts/validate-ai-record.py --mode record --kind issue-create /path/to/draft.md
gc issue create -R owner/repo --title "..." --body-file /path/to/draft.md
```

## 1. Issue 验证记录

适用场景：

- issue 初次验证
- 重新验证问题是否仍存在
- 判定 issue 是否继续修复

```markdown
## 验证记录

- 当前版本或分支:
- 验证时间:
- 复现命令:
- 实际结果:
- 预期结果:
- 时间线检查:
- 结论: 继续修复 / 已修复关闭 / 信息不足待补充 / 无需修复
```

示例命令：

```bash
gc issue comment <number> --body-file /path/to/verification.md -R owner/repo
```

## 2. Issue 开发进度

适用场景：

- 已进入 `status/in-progress`
- 准备推进到 `status/ready-for-review`

```markdown
## 开发进度

- 当前状态: in-progress / ready-for-review
- 根因:
- 主要修改:
- 影响范围:
- 单元测试:
- 构建:
- 实际命令验证:
- 安全影响:
- 文档同步:
- 风险或未覆盖项:
- 关联 PR:
```

## 3. Issue 阻塞说明

适用场景：

- 进入 `status/blocked`
- 需要等待外部依赖、API 能力或人工决策

```markdown
## 阻塞说明

- 阻塞原因:
- 当前影响:
- 已尝试动作:
- 需要外部输入:
- 下一步建议:
```

## 4. Issue 关闭说明

### 4.1 合入主干后关闭

```markdown
## 关闭说明

- 关闭原因: 已合入主干
- 关联 PR:
- 合入分支: main
- 验证结论:
```

### 4.2 不修复关闭

```markdown
## 关闭说明

- 关闭原因: closed-no-fix
- 具体判定: 重复 / 无效 / 已修复 / 不再处理
- 依据:
```

## 5. PR 作者自检

适用场景：

- PR 已创建为 `status/draft`
- 请求独立执行主体评审之前

```markdown
## 作者自检

- 作者主体标识:
- 根因或实现理由:
- 主要修改:
- 影响范围:
- 单元测试:
- 构建:
- 实际命令验证:
- 安全审查:
- 文档同步:
- 风险:
- 未覆盖项:
- 自检结论: 可进入 ready-for-review / 仍需继续修改
```

示例命令：

```bash
gc pr review <number> --comment-file /path/to/self-check.md -R owner/repo
```

## 6. PR 评审结论

适用场景：

- 独立执行主体评审
- 合并前最终确认

```markdown
## 评审结论

- 评审主体类型: AI / human
- 评审主体标识:
- 评审范围:
- 发现:
- blocker:
- 安全检查:
- 测试与证据检查:
- 文档同步检查:
- 结论: 可以批准 / 需要继续修改
```

说明：

- 该模板用于独立评审，不用于作者自检
- 如果平台不支持 `request changes`，仍应使用本模板留下结构化修改意见

## 7. docs-only 自检补充

适用场景：

- PR 仅修改文档

```markdown
## docs-only 说明

- 本次改动不涉及代码路径:
- 未运行测试的原因:
- 文档依据:
- 风险:
```

## 8. AI 开发任务启动清单

适用场景：

- AI 接到一个开发、修复或流程推进任务时

```markdown
## 启动清单

- 当前 issue / PR 编号:
- 当前状态标签:
- 远端 issue / PR 当前状态核验:
- 是否已合入主干:
- 是否已检查 merged PR / `origin/main`:
- 是否已完成验证:
- 是否已有开发分支:
- 是否需要补标签:
- 是否需要补验证记录:
- 下一步动作:
```

## 9. 推荐执行顺序

1. 先填写“启动清单”
2. 补 `验证记录`
3. 开始开发并补 `开发进度`
4. 创建 PR 后补 `作者自检`
5. 由非作者、非同一执行主体补 `评审结论`
6. 合入主干后补 `关闭说明`

---

**最后更新**: 2026-10-26
