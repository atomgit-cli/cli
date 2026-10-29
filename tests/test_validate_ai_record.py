import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path

# validate-ai-record.py has a hyphen in its name so it cannot be imported
# normally; load it via importlib (same pattern as test_verify_remote_facts).
_spec = importlib.util.spec_from_file_location(
    "validate_ai_record",
    Path(__file__).parent.parent / "scripts" / "validate-ai-record.py",
)
var = importlib.util.module_from_spec(_spec)
sys.modules["validate_ai_record"] = var
_spec.loader.exec_module(var)

TEMPLATES = Path(__file__).parent.parent / "docs" / "ai-templates"

FILLED_RECORD = """## 背景与证据

### 问题

真实问题。

### 证据

- pkg/foo/bar.go:42

## 改动建议

按 A 方案修复。

## 验收标准

输出与文档一致。

## 影响范围

pkg/foo/
"""


class SectionContentTests(unittest.TestCase):
    """record-mode section semantics for heading-style fields (issue 629).

    These branches are exercised by CI only through this file: the make
    targets run template mode, so a regression in section_content would
    otherwise be completely silent.
    """

    def test_body_returns_real_content_until_next_heading(self):
        lines = ["## 一段", "", "真实内容", "## 二段", "其他"]
        self.assertEqual(var.section_content(lines, "## 一段"), "真实内容")

    def test_comments_do_not_count_as_content(self):
        lines = ["## 一段", "<!-- guidance -->", ""]
        self.assertEqual(var.section_content(lines, "## 一段"), "")

    def test_whole_line_placeholder_does_not_count_as_content(self):
        lines = ["## 一段", "（问题描述）"]
        self.assertEqual(var.section_content(lines, "## 一段"), "")

    def test_list_form_placeholder_does_not_count_as_content(self):
        # The evidence section's placeholder ships as a list item; the
        # placeholder rule must cover both shapes (fb27983).
        lines = ["## 一段", "- （file:line / 命令输出 / 引用）"]
        self.assertEqual(var.section_content(lines, "## 一段"), "")

    def test_mixed_list_line_with_parenthetical_still_counts(self):
        # Real evidence lines keep their content: only entire values wrapped
        # in full-width parens are placeholders.
        lines = ["## 一段", "- pkg/foo/bar.go:42（实证）"]
        self.assertEqual(var.section_content(lines, "## 一段"), "- pkg/foo/bar.go:42（实证）")


class ParseFieldsTests(unittest.TestCase):
    def test_list_and_heading_fields_coexist(self):
        lines = ["- 列表字段: 值", "## 标题字段", "- 另一列表: v"]
        found = var.parse_fields(lines)
        self.assertEqual(found["列表字段"], "值")
        self.assertEqual(found["另一列表"], "v")
        self.assertIn("## 标题字段", found)


class ValidateRecordModeTests(unittest.TestCase):
    def _validate(self, text: str, kind: str = "issue-create"):
        with tempfile.NamedTemporaryFile(
            "w", suffix=".md", delete=False, encoding="utf-8"
        ) as handle:
            handle.write(text)
            path = handle.name
        try:
            return var.validate(Path(path), kind, "record")
        finally:
            Path(path).unlink()

    def test_filled_record_passes(self):
        self.assertEqual(self._validate(FILLED_RECORD), [])

    def test_emptied_section_is_reported(self):
        broken = FILLED_RECORD.replace("按 A 方案修复。", "")
        self.assertIn("empty value: ## 改动建议", self._validate(broken))

    def test_comment_only_section_is_reported(self):
        broken = FILLED_RECORD.replace(
            "按 A 方案修复。", "<!-- guidance only -->"
        )
        self.assertIn("empty value: ## 改动建议", self._validate(broken))

    def test_unreplaced_placeholder_is_reported(self):
        broken = FILLED_RECORD.replace("按 A 方案修复。", "（待填）")
        self.assertIn("empty value: ## 改动建议", self._validate(broken))

    def test_mixed_draft_with_evidence_placeholder_is_reported(self):
        # Every section filled except the evidence placeholder: the exact
        # partial-draft bypass that three reviewers converged on.
        broken = FILLED_RECORD.replace(
            "- pkg/foo/bar.go:42", "- （file:line / 命令输出 / 引用）"
        )
        self.assertIn("empty value: ### 证据", self._validate(broken))


class AsShippedTemplateTests(unittest.TestCase):
    """The factory behaviour: templates as shipped must fail record mode,
    matching the ten list-style kinds (whose empty fields always did)."""

    def test_issue_create_template_as_shipped_fails_record(self):
        errors = var.validate(
            TEMPLATES / "issue-create.md", "issue-create", "record"
        )
        self.assertTrue(errors, "as-shipped template must not pass record mode")

    def test_list_style_template_as_shipped_fails_record(self):
        errors = var.validate(
            TEMPLATES / "pr-self-check.md", "pr-self-check", "record"
        )
        self.assertTrue(errors, "as-shipped list template must not pass record mode")

    def test_issue_create_template_passes_template_mode(self):
        errors = var.validate(
            TEMPLATES / "issue-create.md", "issue-create", "template"
        )
        self.assertEqual(errors, [])


if __name__ == "__main__":
    unittest.main()
