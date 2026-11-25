package config

import "os"

// SecureWriteFile is the exported hardened-write helper for sibling packages
// (e.g. pkg/larkcli) that persist credential-adjacent state under the gc config
// directory. It delegates to secureWriteFile so that symlink refusal,
// permission hardening, and atomic writes stay consistent across the
// codebase. The data-level atomicity guarantee holds on every platform
// (successfully read data is always the complete old or new file); on
// Windows a reader or external process holding the target open can
// transiently fail the replacing rename — a loud, retryable error, never
// torn data (spec/foundations/security.md 凭证文件落盘约束 records the
// full contract).
func SecureWriteFile(path string, data []byte, perm os.FileMode) error {
	return secureWriteFile(path, data, perm)
}
