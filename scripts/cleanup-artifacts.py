#!/usr/bin/env python3
"""Delete workflow artifacts that exceeded the retention window.

Usage examples:
  ./scripts/cleanup-artifacts.py --repo atomgit-cli/cli --retention-days 2
  ./scripts/cleanup-artifacts.py --repo atomgit-cli/cli --retention-days 2 --dry-run

The script shells out to the repository's own gc CLI (actions artifact
list/delete) and authenticates through the standard GC_TOKEN or
GITCODE_TOKEN environment variable. It never prints the token value.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from datetime import datetime, timezone

DAY_MS = 24 * 60 * 60 * 1000
DEFAULT_RETENTION_DAYS = 2
DEFAULT_GC_PATH = "./gc"


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Delete workflow artifacts older than the retention window."
    )
    parser.add_argument("--repo", required=True, help="Repository in owner/repo format")
    parser.add_argument(
        "--retention-days",
        type=int,
        default=DEFAULT_RETENTION_DAYS,
        help=f"Delete artifacts created more than this many days ago (default: {DEFAULT_RETENTION_DAYS})",
    )
    parser.add_argument(
        "--gc-path",
        default=DEFAULT_GC_PATH,
        help=f"Path to the gc binary (default: {DEFAULT_GC_PATH})",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Only report the artifacts that would be deleted",
    )
    parser.add_argument(
        "--now-ms",
        type=int,
        default=None,
        help="Override the current time in epoch milliseconds (for deterministic runs)",
    )
    return parser.parse_args(argv)


def parse_timestamp_ms(value: object) -> float | None:
    """Decode an Actions v8 timestamp into epoch milliseconds.

    The live endpoint returns RFC 3339 strings ("2026-11-25T10:25:46+08:00"),
    while older fixtures and the CLI's Artifact model assume a millisecond
    epoch string ("1783587145000"). Accept both shapes so a platform-side
    format change cannot silently turn every artifact into "unknown time".
    """
    if value is None or isinstance(value, bool):
        return None
    if isinstance(value, (int, float)):
        return float(value)
    text = str(value).strip()
    if not text:
        return None
    try:
        return float(text)
    except ValueError:
        pass
    try:
        parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError:
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed.timestamp() * 1000.0


def select_expired(
    artifacts: list[dict], now_ms: float, retention_days: int
) -> tuple[list[dict], int]:
    """Return artifacts older than the cutoff, oldest first, plus the unknown-time count."""
    cutoff_ms = now_ms - retention_days * DAY_MS
    expired: list[dict] = []
    unknown_time = 0
    for artifact in artifacts:
        created_ms = parse_timestamp_ms(artifact.get("created_at"))
        if created_ms is None:
            unknown_time += 1
            continue
        if created_ms < cutoff_ms:
            expired.append(artifact)
    expired.sort(key=lambda item: (parse_timestamp_ms(item.get("created_at")) or 0.0, str(item.get("id"))))
    return expired, unknown_time


def run_gc_json(gc_path: str, args: list[str]) -> object:
    cmd = [gc_path, *args]
    result = subprocess.run(
        cmd, check=False, capture_output=True, text=True, encoding="utf-8"
    )
    if result.returncode != 0:
        message = (result.stderr or result.stdout).strip()
        raise RuntimeError(message or f"failed to run: {' '.join(cmd)}")
    output = (result.stdout or "").strip()
    if not output:
        return None
    try:
        return json.loads(output)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"invalid json from {' '.join(cmd)}: {exc}") from exc


def list_artifacts(gc_path: str, repo: str) -> list[dict]:
    payload = run_gc_json(
        gc_path,
        [
            "actions",
            "artifact",
            "list",
            "-R",
            repo,
            "--paginate",
            "--per-page",
            "100",
            "--json",
        ],
    )
    if payload is None:
        return []
    if not isinstance(payload, list):
        raise RuntimeError("unexpected artifact list payload: expected a JSON array")
    return payload


def delete_artifact(gc_path: str, repo: str, artifact_id: object) -> None:
    run_gc_json(
        gc_path,
        [
            "actions",
            "artifact",
            "delete",
            str(artifact_id),
            "-R",
            repo,
            "--yes",
            "--json",
        ],
    )


def format_mib(size_bytes: int) -> str:
    return f"{size_bytes / (1024 * 1024):.1f} MiB"


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if args.retention_days < 1:
        print("error: --retention-days must be at least 1", file=sys.stderr)
        return 2

    now_ms = args.now_ms if args.now_ms is not None else int(time.time() * 1000)

    try:
        artifacts = list_artifacts(args.gc_path, args.repo)
    except RuntimeError as exc:
        print(f"error: failed to list artifacts: {exc}", file=sys.stderr)
        return 1

    expired, unknown_time = select_expired(artifacts, now_ms, args.retention_days)

    print(
        f"artifacts total={len(artifacts)} eligible={len(expired)} "
        f"retention_days={args.retention_days} dry_run={'true' if args.dry_run else 'false'}"
    )
    if unknown_time:
        print(f"skipped {unknown_time} artifact(s) without a usable created_at")

    deleted = 0
    failed = 0
    deleted_bytes = 0
    for artifact in expired:
        label = f"{artifact.get('name') or '<unnamed>'} (id={artifact.get('id')})"
        if args.dry_run:
            print(f"would delete {label}")
            continue
        try:
            delete_artifact(args.gc_path, args.repo, artifact.get("id"))
        except RuntimeError as exc:
            failed += 1
            print(f"failed to delete {label}: {exc}", file=sys.stderr)
            continue
        deleted += 1
        deleted_bytes += int(artifact.get("size_bytes") or 0)
        print(f"deleted {label}")

    print(
        f"summary total={len(artifacts)} eligible={len(expired)} "
        f"deleted={deleted} failed={failed} freed={format_mib(deleted_bytes)}"
    )
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
