package rigs

import "syscall"

// sysProcAttrTTY makes the child a session leader with its stdin (the pty
// slave, child fd 0) as controlling terminal.
func sysProcAttrTTY() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
}
