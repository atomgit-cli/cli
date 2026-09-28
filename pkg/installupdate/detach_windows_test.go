//go:build windows

package installupdate

import "testing"

func TestDetachAttrsFlags(t *testing.T) {
	const createNewProcessGroup = 0x00000200
	const detachedProcess = 0x00000008
	attrs := detachAttrs()
	if attrs.CreationFlags != createNewProcessGroup|detachedProcess {
		t.Fatalf("detachAttrs().CreationFlags = %#x, want %#x", attrs.CreationFlags, createNewProcessGroup|detachedProcess)
	}
}
