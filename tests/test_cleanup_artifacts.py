import contextlib
import importlib.util
import io
import sys
import unittest
from datetime import datetime, timezone
from pathlib import Path
from unittest import mock

# cleanup-artifacts.py has a hyphen in its name so it cannot be imported
# normally; load it via importlib and register it so mock.patch can find it.
_spec = importlib.util.spec_from_file_location(
    "cleanup_artifacts",
    Path(__file__).parent.parent / "scripts" / "cleanup-artifacts.py",
)
cleanup = importlib.util.module_from_spec(_spec)
sys.modules["cleanup_artifacts"] = cleanup
_spec.loader.exec_module(cleanup)

NOW_MS = 1_800_000_000_000
DAY_MS = cleanup.DAY_MS


def artifact(artifact_id: str, created_days_ago: float, size_bytes: int = 1024) -> dict:
    return {
        "id": artifact_id,
        "name": f"artifact-{artifact_id}",
        "size_bytes": size_bytes,
        "created_at": str(int(NOW_MS - created_days_ago * DAY_MS)),
    }


def rfc3339_artifact(artifact_id: str, created_days_ago: float) -> dict:
    created = datetime.fromtimestamp((NOW_MS - created_days_ago * DAY_MS) / 1000, tz=timezone.utc)
    return {
        "id": artifact_id,
        "name": f"artifact-{artifact_id}",
        "size_bytes": 1024,
        "created_at": created.isoformat(),
    }


class ParseTimestampTests(unittest.TestCase):
    def test_accepts_numeric_string_and_int(self):
        self.assertEqual(cleanup.parse_timestamp_ms("1783587145000"), 1783587145000.0)
        self.assertEqual(cleanup.parse_timestamp_ms(1783587145000), 1783587145000.0)

    def test_accepts_rfc3339_with_offset(self):
        """The live Actions v8 API returns RFC 3339 strings, not millisecond epochs."""
        expected = datetime(2026, 11, 25, 2, 25, 46, tzinfo=timezone.utc).timestamp() * 1000
        self.assertEqual(cleanup.parse_timestamp_ms("2026-11-25T10:25:46+08:00"), expected)
        self.assertEqual(cleanup.parse_timestamp_ms("2026-11-25T02:25:46Z"), expected)

    def test_rejects_missing_and_malformed_values(self):
        for value in (None, "", "  ", "not-a-timestamp", True, {}):
            self.assertIsNone(cleanup.parse_timestamp_ms(value), msg=repr(value))


class SelectExpiredTests(unittest.TestCase):
    def test_selects_only_artifacts_older_than_the_window(self):
        artifacts = [
            artifact("1", 3),
            artifact("2", 1),
            artifact("3", 2.5),
            artifact("4", 0),
        ]
        expired, unknown = cleanup.select_expired(artifacts, NOW_MS, 2)
        self.assertEqual([item["id"] for item in expired], ["1", "3"])
        self.assertEqual(unknown, 0)

    def test_cutoff_is_strictly_older_than_retention_window(self):
        exactly_two_days = artifact("edge", 2)
        expired, _ = cleanup.select_expired([exactly_two_days], NOW_MS, 2)
        self.assertEqual(expired, [])

    def test_orders_oldest_first_and_counts_unknown_timestamps(self):
        artifacts = [
            artifact("new", 3),
            {"id": "broken", "name": "broken", "created_at": ""},
            artifact("old", 10),
        ]
        expired, unknown = cleanup.select_expired(artifacts, NOW_MS, 2)
        self.assertEqual([item["id"] for item in expired], ["old", "new"])
        self.assertEqual(unknown, 1)

    def test_selects_rfc3339_artifacts_from_the_live_api_shape(self):
        artifacts = [
            rfc3339_artifact("old", 3),
            rfc3339_artifact("fresh", 0.5),
        ]
        expired, unknown = cleanup.select_expired(artifacts, NOW_MS, 2)
        self.assertEqual([item["id"] for item in expired], ["old"])
        self.assertEqual(unknown, 0)


class MainTests(unittest.TestCase):
    def run_main(self, argv, artifacts, delete_side_effect=None):
        stdout, stderr = io.StringIO(), io.StringIO()
        delete = mock.Mock(side_effect=delete_side_effect)
        with mock.patch.object(cleanup, "list_artifacts", return_value=artifacts):
            with mock.patch.object(cleanup, "delete_artifact", delete):
                with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                    code = cleanup.main(argv + ["--now-ms", str(NOW_MS)])
        return code, stdout.getvalue(), stderr.getvalue(), delete

    def test_dry_run_reports_without_deleting(self):
        artifacts = [artifact("1", 5), artifact("2", 1)]
        code, out, _, delete = self.run_main(
            ["--repo", "atomgit-cli/cli", "--retention-days", "2", "--dry-run"], artifacts
        )
        self.assertEqual(code, 0)
        self.assertIn("eligible=1", out)
        self.assertIn("would delete artifact-1 (id=1)", out)
        delete.assert_not_called()

    def test_deletes_only_expired_artifacts(self):
        artifacts = [artifact("1", 5, size_bytes=2 * 1024 * 1024), artifact("2", 1)]
        code, out, _, delete = self.run_main(
            ["--repo", "atomgit-cli/cli", "--retention-days", "2"], artifacts
        )
        self.assertEqual(code, 0)
        delete.assert_called_once()
        self.assertIn("deleted artifact-1 (id=1)", out)
        self.assertIn("deleted=1 failed=0", out)
        self.assertIn("freed=2.0 MiB", out)

    def test_reports_deletion_failures_and_exits_nonzero(self):
        artifacts = [artifact("1", 5), artifact("2", 6)]
        code, _, err, delete = self.run_main(
            ["--repo", "atomgit-cli/cli", "--retention-days", "2"],
            artifacts,
            delete_side_effect=RuntimeError("HTTP 403"),
        )
        self.assertEqual(code, 1)
        self.assertEqual(delete.call_count, 2)
        self.assertIn("failed to delete", err)
        self.assertIn("HTTP 403", err)

    def test_listing_failure_exits_nonzero_without_deleting(self):
        stdout, stderr = io.StringIO(), io.StringIO()
        delete = mock.Mock()
        with mock.patch.object(
            cleanup, "list_artifacts", side_effect=RuntimeError("HTTP 401")
        ):
            with mock.patch.object(cleanup, "delete_artifact", delete):
                with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                    code = cleanup.main(["--repo", "atomgit-cli/cli"])
        self.assertEqual(code, 1)
        self.assertIn("failed to list artifacts: HTTP 401", stderr.getvalue())
        delete.assert_not_called()

    def test_rejects_non_positive_retention_window(self):
        stdout, stderr = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            code = cleanup.main(["--repo", "atomgit-cli/cli", "--retention-days", "0"])
        self.assertEqual(code, 2)
        self.assertIn("--retention-days must be at least 1", stderr.getvalue())


class ListArtifactsTests(unittest.TestCase):
    def test_lists_every_page_as_json(self):
        payload = [artifact("1", 5)]
        with mock.patch.object(cleanup, "run_gc_json", return_value=payload) as run:
            self.assertEqual(cleanup.list_artifacts("./gc", "atomgit-cli/cli"), payload)
        args = run.call_args[0][1]
        self.assertEqual(args[:3], ["actions", "artifact", "list"])
        self.assertIn("--paginate", args)
        self.assertIn("--json", args)

    def test_rejects_non_array_payload(self):
        with mock.patch.object(cleanup, "run_gc_json", return_value={"artifacts": []}):
            with self.assertRaises(RuntimeError):
                cleanup.list_artifacts("./gc", "atomgit-cli/cli")

    def test_gc_failure_surfaces_stderr(self):
        mock_result = mock.Mock(returncode=1, stdout="", stderr="HTTP 404")
        with mock.patch.object(cleanup.subprocess, "run", return_value=mock_result):
            with self.assertRaises(RuntimeError) as ctx:
                cleanup.run_gc_json("./gc", ["actions", "artifact", "list"])
        self.assertIn("HTTP 404", str(ctx.exception))


if __name__ == "__main__":
    unittest.main()
