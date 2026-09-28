//go:build windows

package installupdate

import "syscall"

// detachAttrs returns process attributes that detach the updater from the
// originating console: a new process group stops Ctrl+C broadcasts from
// reaching it, and DETACHED_PROCESS removes the console dependency entirely
// so closing the terminal window cannot kill an update that is mid-install.
func detachAttrs() *syscall.SysProcAttr {
	const createNewProcessGroup = 0x00000200
	const detachedProcess = 0x00000008
	return &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | detachedProcess}
}
