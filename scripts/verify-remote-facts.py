#!/usr/bin/env python3
"""Verify minimal remote issue/PR facts for the local workflow.

Usage examples:
  ./scripts/verify-remote-facts.py --repo gitcode-cli/cli --pr 87 --head-sha <sha>
  ./scripts/verify-remote-facts.py --repo owner/repo --issue 12 --pr 34 --head-sha <sha>
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", required=True, help="Repository in owner/repo format")
    parser.add_argument("--issue", type=int, help="Issue number to verify")
    parser.add_argument("--pr", type=int, help="PR number to verify")
    parser.add_argument("--head-sha", help="Head commit SHA expected to be in origin/main")
    return parser.parse_args()


ISSUE_REF_RE = re.compile(r"(?:#|!)(\d+)")


def run_json(cmd: list[str]) -> dict:
    result = subprocess.run(cmd, check=False, capture_output=True, text=True, encoding="utf-8")
    if result.returncode != 0:
        raise SystemExit(result.stderr.strip() or f"failed to run: {' '.join(cmd)}")
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        raise SystemExit(f"invalid json from {' '.join(cmd)}: {exc}") from exc


def merged_in_main(sha: str) -> bool:
    cmd = ["git", "merge-base", "--is-ancestor", sha, "origin/main"]
    result = subprocess.run(cmd, check=False, capture_output=True, text=True, encoding="utf-8")
    if result.returncode == 0:
        return True
    if result.returncode == 1:
        return False
    raise SystemExit(result.stderr.strip() or f"failed to run: {' '.join(cmd)}")


def extract_issue_numbers(pr: dict) -> list[int]:
    text = "\n".join(
        [
            str(pr.get("title") or ""),
            str(pr.get("body") or ""),
        ]
    )
    seen: set[int] = set()
    found: list[int] = []
    for match in ISSUE_REF_RE.finditer(text):
        number = int(match.group(1))
        if number in seen:
            continue
        seen.add(number)
        found.append(number)
    return found


def main() -> int:
    args = parse_args()
    problems: list[str] = []

    issue = None
    if args.issue is not None:
        issue = run_json(["./gc", "issue", "view", str(args.issue), "-R", args.repo, "--json"])
        print(f"issue.state={issue.get('state') or 'unknown'}")
        print(f"issue.closed_at={issue.get('closed_at') or 'unknown'}")

    pr = None
    if args.pr is not None:
        pr = run_json(["./gc", "pr", "view", str(args.pr), "-R", args.repo, "--json"])
        pr_state = pr.get("state") or "unknown"
        merged_at = pr.get("merged_at") or "unknown"
        print(f"pr.state={pr_state}")
        print(f"pr.merged_at={merged_at}")
        is_merged = pr_state == "merged" or bool(pr.get("merged_at"))
        if not is_merged:
            problems.append(f"pr #{args.pr} is not merged")
        inferred_issues = extract_issue_numbers(pr)
        if inferred_issues:
            print(
                "pr.referenced_issues="
                + ",".join(str(number) for number in inferred_issues)
            )
            if args.issue is not None and inferred_issues and args.issue not in inferred_issues:
                problems.append(
                    f"--issue {args.issue} is not referenced by pr #{args.pr} "
                    f"(referenced: {','.join(str(n) for n in sorted(inferred_issues))})"
                )
            if args.issue is None and len(inferred_issues) == 1:
                args.issue = inferred_issues[0]
                issue = run_json(
                    ["./gc", "issue", "view", str(args.issue), "-R", args.repo, "--json"]
                )
                print(f"issue.state={issue.get('state') or 'unknown'}")
                print(f"issue.closed_at={issue.get('closed_at') or 'unknown'}")
        else:
            print("pr.referenced_issues=none")

    if args.head_sha:
        in_main = merged_in_main(args.head_sha)
        print(f"head_sha.in_origin_main={'true' if in_main else 'false'}")
        if not in_main:
            problems.append(f"head sha {args.head_sha} is not contained in origin/main")

    if issue and issue.get("state") == "closed":
        if args.pr is None and not args.head_sha:
            problems.append("closed issue requires --pr or --head-sha to verify mainline delivery")
        if args.pr is not None and pr is not None:
            is_merged = (pr.get("state") == "merged") or bool(pr.get("merged_at"))
            if not is_merged:
                problems.append("issue is closed but linked PR is not merged")
        if args.head_sha and not merged_in_main(args.head_sha):
            problems.append("issue is closed but head sha is not in origin/main")

    if problems:
        print("verification=failed")
        for problem in problems:
            print(f"- {problem}")
        return 1

    print("verification=passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
