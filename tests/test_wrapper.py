import os
import runpy
import stat
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from gc_cli import wrapper


class WrapperTests(unittest.TestCase):
    def test_windows_binary_name(self):
        with mock.patch("platform.system", return_value="Windows"):
            with mock.patch("platform.machine", return_value="AMD64"):
                self.assertEqual(wrapper.get_binary_name(), "gc-windows-amd64.exe")

    def test_unknown_architecture_is_rejected(self):
        with mock.patch("platform.system", return_value="Linux"):
            with mock.patch("platform.machine", return_value="riscv64"):
                with self.assertRaisesRegex(RuntimeError, "Unsupported platform: linux riscv64"):
                    wrapper.get_binary_name()

    def test_windows_arm64_is_rejected_until_a_binary_is_shipped(self):
        with mock.patch("platform.system", return_value="Windows"):
            with mock.patch("platform.machine", return_value="ARM64"):
                with self.assertRaisesRegex(RuntimeError, "Unsupported platform: windows arm64"):
                    wrapper.get_binary_name()

    def test_ensure_executable_skips_windows_execute_bit_check(self):
        with tempfile.NamedTemporaryFile() as tmp:
            path = Path(tmp.name)
            with mock.patch("platform.system", return_value="Windows"):
                with mock.patch("os.access", side_effect=AssertionError("should not check X_OK")):
                    wrapper.ensure_executable(path)

    def test_ensure_executable_chmods_non_executable_posix_binary(self):
        with tempfile.NamedTemporaryFile() as tmp:
            path = Path(tmp.name)
            path.chmod(stat.S_IRUSR | stat.S_IWUSR)

            with mock.patch("platform.system", return_value="Linux"):
                wrapper.ensure_executable(path)

            self.assertTrue(os.access(path, os.X_OK))

    def test_main_runs_packaged_binary_with_arguments(self):
        with tempfile.NamedTemporaryFile() as tmp:
            binary = Path(tmp.name)
            completed = mock.Mock(returncode=7)

            with mock.patch("gc_cli.wrapper.get_binary_path", return_value=binary):
                with mock.patch("gc_cli.wrapper.ensure_executable") as ensure_executable:
                    with mock.patch("subprocess.run", return_value=completed) as run:
                        with mock.patch("sys.argv", ["gitcode", "version"]):
                            self.assertEqual(wrapper.main(), 7)

            ensure_executable.assert_called_once_with(binary)
            args, kwargs = run.call_args
            self.assertEqual(args[0], [str(binary), "version"])
            self.assertEqual(kwargs["cwd"], os.getcwd())
            self.assertEqual(kwargs["env"][wrapper.COMMAND_NAME_ENV], "gitcode")
            self.assertEqual(kwargs["env"][wrapper.DISTRIBUTION_ENV], "pypi")
            self.assertEqual(kwargs["env"][wrapper.BINARY_ENV], str(binary.resolve()))

    def test_command_name_defaults_to_gitcode_for_python_module_on_windows(self):
        with mock.patch("platform.system", return_value="Windows"):
            with mock.patch("sys.argv", ["python", "-m", "gc_cli", "version"]):
                self.assertEqual(wrapper.get_command_name(), "gitcode")

    def test_command_name_preserves_gc_script_name(self):
        with mock.patch("platform.system", return_value="Windows"):
            with mock.patch("sys.argv", ["gc", "version"]):
                self.assertEqual(wrapper.get_command_name(), "gc")

    def test_module_entrypoint_delegates_to_wrapper_main(self):
        with mock.patch("gc_cli.wrapper.main", return_value=0) as main:
            with self.assertRaises(SystemExit) as exc:
                runpy.run_module("gc_cli", run_name="__main__")

        self.assertEqual(exc.exception.code, 0)
        main.assert_called_once_with()


if __name__ == "__main__":
    unittest.main()


class DetectDistributionTests(unittest.TestCase):
    def test_uv_tool_venv_reports_uv(self):
        for prefix in (
            "/home/u/.local/share/uv/tools/gitcode-cli",
            "C:\\Users\\u\\AppData\\Roaming\\uv\\tools\\gc",
        ):
            with mock.patch("sys.prefix", prefix):
                with mock.patch("pathlib.Path.resolve", return_value=Path(prefix)):
                    self.assertEqual(wrapper.detect_distribution(), "uv", prefix)

    def test_ordinary_venv_reports_pypi(self):
        for prefix in ("/usr", "/home/u/venv", "C:\\Users\\u\\venv"):
            with mock.patch("sys.prefix", prefix):
                with mock.patch("pathlib.Path.resolve", return_value=Path(prefix)):
                    self.assertEqual(wrapper.detect_distribution(), "pypi", prefix)

    def test_main_declares_uv_channel_for_uv_tools(self):
        captured = {}

        def fake_run(argv, **kwargs):
            captured.update(kwargs.get("env") or {})
            return mock.Mock(returncode=0)

        binary = Path("/tmp/fake-gc")
        with mock.patch("gc_cli.wrapper.get_binary_path", return_value=binary):
            with mock.patch("gc_cli.wrapper.ensure_executable"):
                with mock.patch("subprocess.run", side_effect=fake_run):
                    with mock.patch("sys.argv", ["gitcode", "version"]):
                        with mock.patch(
                            "sys.prefix", "/home/u/.local/share/uv/tools/gitcode-cli"
                        ):
                            with mock.patch(
                                "pathlib.Path.resolve", return_value=Path("/home/u/.local/share/uv/tools/gitcode-cli")
                            ):
                                wrapper.main()

        self.assertEqual(captured.get(wrapper.DISTRIBUTION_ENV), "uv")


if __name__ == "__main__":
    unittest.main()
