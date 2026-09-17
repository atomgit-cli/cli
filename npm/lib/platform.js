// Platform -> binary name resolution for the bundled GitCode CLI binaries.
//
// Mirrors the selection logic in gc_cli/wrapper.py (get_binary_name): Node's
// process.platform/process.arch are mapped to the bundled binary file name
// under npm/bin/platforms/.

"use strict";

// Node arch -> gc arch segment. x64 binaries are named with "amd64".
const ARCH_MAP = {
  x64: "amd64",
  amd64: "amd64", // defensive; Node does not emit amd64 today
  arm64: "arm64",
};

// Node platform -> gc platform segment. OpenHarmony reuses the bundled
// Linux binary: the OpenHarmony standard system runs the Linux kernel and
// gc ships fully static binaries (CGO_ENABLED=0), so gc-linux-* runs there
// unmodified. Node reports process.platform === "openharmony" on that OS.
const PLATFORM_MAP = {
  linux: "linux",
  openharmony: "linux",
  darwin: "darwin",
  win32: "windows",
};

/**
 * Resolve the bundled binary file name for the current platform/arch.
 * Returns e.g. "gc-linux-amd64" or "gc-windows-amd64.exe".
 * Throws when the platform/arch is unsupported (no bundled binary exists).
 */
function resolveBinaryName(platform, arch) {
  const p = PLATFORM_MAP[platform];
  const a = ARCH_MAP[arch];
  if (!p || !a || (p === "windows" && a !== "amd64")) {
    throw new Error(
      `unsupported platform/arch: ${platform}/${arch}; ` +
        `supported: linux/x64, linux/arm64, openharmony/arm64, darwin/x64, darwin/arm64, win32/x64`
    );
  }
  const name = `gc-${p}-${a}`;
  return p === "windows" ? `${name}.exe` : name;
}

/**
 * True when the resolved binary is bundled (i.e. the platform/arch combo is
 * one we ship). Used by install to decide whether to fall back.
 */
function isSupported(platform, arch) {
  try {
    resolveBinaryName(platform, arch);
    return true;
  } catch {
    return false;
  }
}

module.exports = { resolveBinaryName, isSupported, ARCH_MAP, PLATFORM_MAP };
