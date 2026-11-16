package config

import "os"

// SecureWriteFile is the exported hardened-write helper for sibling packages
// (e.g. pkg/larkcli) that persist credential-adjacent state under the gc config
// directory. It delegates to secureWriteFile so that symlink refusal,
// permission hardening, and atomic writes stay consistent across the
// codebase.
func SecureWriteFile(path string, data []byte, perm os.FileMode) error {
	return secureWriteFile(path, data, perm)
}
