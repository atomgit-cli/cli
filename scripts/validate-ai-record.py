#!/usr/bin/env python3
"""Validate AI collaboration markdown templates and filled records.

Usage examples:
  ./scripts/validate-ai-record.py --mode template docs/ai-templates/pr-self-check.md
  ./scripts/validate-ai-record.py --kind pr-self-check --mode record /tmp/pr-self-check.md
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


SCHEMAS = {
    "task-start-checklist": {
        "heading": "## 启动清单",
        "fields": [
            "当前 issue / PR 编号",
            "当前状态标签",
            "远端 issue / PR 当前状态核验",
            "是否已合入主干",
            "是否已检查 merged PR / `origin/main`",
            "是否已完成验证",
            "是否已有开发分支",
            "是否需要补标签",
            "是否需要补验证记录",
            "下一步动作",
        ],
    },
    "issue-create": {
        "heading": "## 背景与证据",
        "fields": [
            "### 问题",
            "### 证据",
            "## 改动建议",
            "## 验收标准",
            "## 影响范围",
        ],
    },
    "issue-verification": {
        "heading": "## 验证记录",
        "fields": [
            "当前版本或分支",
            "验证时间",
            "复现命令",
            "实际结果",
            "预期结果",
            "时间线检查",
            "结论",
        ],
    },
    "issue-progress": {
        "heading": "## 开发进度",
        "fields": [
            "当前状态",
            "根因",
            "主要修改",
            "影响范围",
            "单元测试",
            "构建",
            "实际命令验证",
            "安全影响",
            "文档同步",
            "风险或未覆盖项",
            "关联 PR",
        ],
    },
    "issue-blocked": {
        "heading": "## 阻塞说明",
        "fields": [
            "阻塞原因",
            "当前影响",
            "已尝试动作",
            "需要外部输入",
            "下一步建议",
        ],
    },
    "issue-close-merged": {
        "heading": "## 关闭说明",
        "fields": [
            "关闭原因",
            "关联 PR",
            "合入分支",
            "验证结论",
        ],
    },
    "issue-close-no-fix": {
        "heading": "## 关闭说明",
        "fields": [
            "关闭原因",
            "具体判定",
            "依据",
        ],
    },
    "pr-self-check": {
        "heading": "## 作者自检",
        "fields": [
            "作者主体标识",
            "根因或实现理由",
            "主要修改",
            "影响范围",
            "单元测试",
            "构建",
            "实际命令验证",
            "安全审查",
            "文档同步",
            "风险",
            "未覆盖项",
            "自检结论",
        ],
    },
    "pr-review-outcome": {
        "heading": "## 评审结论",
        "fields": [
            "评审主体类型",
            "评审主体标识",
            "评审范围",
            "发现",
            "blocker",
            "安全检查",
            "测试与证据检查",
            "文档同步检查",
            "结论",
        ],
    },
    "docs-only-self-check": {
        "heading": "## docs-only 说明",
        "fields": [
            "本次改动不涉及代码路径",
            "未运行测试的原因",
            "文档依据",
            "风险",
        ],
    },
}


FIELD_RE = re.compile(r"^- ([^:]+):(.*)$")
HEADING_RE = re.compile(r"^(#{2,6} .+)$")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", nargs="?", help="Path to markdown file")
    parser.add_argument(
        "--kind",
        choices=sorted(SCHEMAS.keys()),
        help="Record kind. Defaults to inferring from filename.",
    )
    parser.add_argument(
        "--mode",
        choices=("template", "record"),
        default="record",
        help="template checks structure only; record also requires non-empty values",
    )
    parser.add_argument(
        "--list-kinds",
        action="store_true",
        help="List supported kinds and exit",
    )
    return parser.parse_args()


def infer_kind(path: Path) -> str | None:
    stem = path.stem
    if stem in SCHEMAS:
        return stem
    return None


def read_lines(path: Path) -> list[str]:
    try:
        return path.read_text(encoding="utf-8").splitlines()
    except FileNotFoundError:
        raise SystemExit(f"error: file not found: {path}")


def first_non_empty(lines: list[str]) -> str:
    for line in lines:
        if line.strip():
            return line.strip()
    return ""


def parse_fields(lines: list[str]) -> dict[str, str]:
    found: dict[str, str] = {}
    for raw in lines:
        stripped = raw.strip()
        match = FIELD_RE.match(stripped)
        if match:
            found[match.group(1).strip()] = match.group(2).strip()
            continue
        # Heading-style fields (section templates such as issue-create):
        # the value is the section body, so it starts empty here and is
        # filled by section_content below.
        heading = HEADING_RE.match(stripped)
        if heading:
            found.setdefault(heading.group(1).strip(), "")
    return found


def section_content(lines: list[str], heading: str) -> str:
    """Return the body of a heading-style field: everything until the next
    heading, ignoring blanks and HTML comment guidance."""
    body: list[str] = []
    inside = False
    for raw in lines:
        stripped = raw.strip()
        if HEADING_RE.match(stripped):
            if inside:
                break
            inside = stripped == heading
            continue
        if inside and stripped and not stripped.startswith("<!--"):
            body.append(stripped)
    return "\n".join(body)


def validate(path: Path, kind: str, mode: str) -> list[str]:
    schema = SCHEMAS[kind]
    lines = read_lines(path)
    errors: list[str] = []

    heading = first_non_empty(lines)
    if heading != schema["heading"]:
        errors.append(
            f"heading mismatch: expected '{schema['heading']}', got '{heading or '<empty>'}'"
        )

    found = parse_fields(lines)
    for field in schema["fields"]:
        if field not in found:
            errors.append(f"missing field: {field}")
            continue
        if mode == "record":
            if field.startswith("#"):
                if not section_content(lines, field):
                    errors.append(f"empty value: {field}")
            elif not found[field]:
                errors.append(f"empty value: {field}")

    return errors


def main() -> int:
    args = parse_args()

    if args.list_kinds:
        for kind in sorted(SCHEMAS.keys()):
            print(kind)
        return 0

    if not args.path:
        print("error: path is required unless --list-kinds is used", file=sys.stderr)
        return 2

    path = Path(args.path)
    kind = args.kind or infer_kind(path)
    if not kind:
        print(
            "error: unable to infer kind from filename; pass --kind explicitly",
            file=sys.stderr,
        )
        return 2

    errors = validate(path, kind, args.mode)
    if errors:
        print(f"invalid {kind} ({args.mode}): {path}")
        for err in errors:
            print(f"- {err}")
        return 1

    print(f"ok: {kind} ({args.mode}) {path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
