"""
Wrapper module for GitCode CLI binary.

This module provides a Python entry point that calls the appropriate
pre-compiled binary based on the current platform.
"""

import os
import platform
import subprocess
import sys
from pathlib import Path

COMMAND_NAME_ENV = "GITCODE_CLI_COMMAND_NAME"
DISTRIBUTION_ENV = "GITCODE_CLI_DISTRIBUTION"
ENTRYPOINT_ENV = "GITCODE_CLI_ENTRYPOINT"
BINARY_ENV = "GITCODE_CLI_BINARY"


def get_binary_name() -> str:
    """Get the binary name for the current platform."""
    system = platform.system().lower()
    machine = platform.machine().lower()

    # Map machine architecture
    arch_map = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "aarch64": "arm64",
        "arm64": "arm64",
    }
    arch = arch_map.get(machine)
    if arch is None:
        raise RuntimeError(f"Unsupported platform: {system} {machine}")

    # Map system to binary name
    if system == "linux":
        return f"gc-linux-{arch}"
    elif system == "darwin":
        return f"gc-darwin-{arch}"
    elif system == "windows" and arch == "amd64":
        return "gc-windows-amd64.exe"
    else:
        raise RuntimeError(f"Unsupported platform: {system} {machine}")


def get_binary_path() -> Path:
    """Get the path to the binary for the current platform."""
    package_dir = Path(__file__).parent
    binary_name = get_binary_name()
    binary_path = package_dir / "bin" / binary_name

    if not binary_path.exists():
        raise FileNotFoundError(
            f"Binary not found for your platform: {binary_path}\n"
            f"Supported platforms: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64"
        )

    return binary_path


def get_command_name() -> str:
    """Get the command name that should appear in CLI output."""
    command = Path(sys.argv[0]).stem.lower()
    if command in {"gc", "gitcode"}:
        return command
    if platform.system() == "Windows":
        return "gitcode"
    return "gc"


def ensure_executable(binary_path: Path) -> None:
    """Ensure packaged binaries are executable on POSIX platforms."""
    if platform.system() == "Windows":
        return
    if not os.access(binary_path, os.X_OK):
        binary_path.chmod(0o755)


def detect_distribution() -> str:
    """Report the installation channel for the distribution env.

    uv-managed tools run from .../uv/tools/<name>/... venvs; reporting "uv"
    (instead of the blanket "pypi") makes `gc update` suggest
    "uv tool upgrade" rather than running pip inside the uv-managed
    environment — which is exactly what uv tells users not to do.
    """
    prefix = str(Path(sys.prefix).resolve()).replace("\\", "/")
    return "uv" if "/uv/tools/" in prefix else "pypi"


def main() -> int:
    """Main entry point for the GitCode CLI command."""
    try:
        binary_path = get_binary_path()

        ensure_executable(binary_path)
        env = os.environ.copy()
        env.setdefault(COMMAND_NAME_ENV, get_command_name())
        env.setdefault(DISTRIBUTION_ENV, detect_distribution())
        env.setdefault(ENTRYPOINT_ENV, str(Path(sys.argv[0]).resolve()))
        env.setdefault(BINARY_ENV, str(binary_path.resolve()))

        # Run the binary with all arguments
        result = subprocess.run(
            [str(binary_path)] + sys.argv[1:],
            cwd=os.getcwd(),
            env=env,
        )

        return result.returncode

    except FileNotFoundError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Error running GitCode CLI: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
