//go:build !windows

package installupdate

import "syscall"

// detachAttrs returns process attributes that move the detached updater into
// its own session, so closing the originating terminal (SIGHUP) or pressing
// Ctrl+C (SIGINT to the foreground process group) cannot kill an update that
// is mid-install.
func detachAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
