package rigs

// Raw /dev/ptmx shim (what zpty and creack/pty do underneath), stdlib only.
// The fake tmux client in the harness needs a REAL pty: tmux refuses
// non-control attachment without one, and script(1) on macOS refuses to run
// when ITS stdin isn't a tty, so neither pipes nor script can fake this.

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	tiocptygrant = 0x20007454 // grantpt
	tiocptyunlk  = 0x20007452 // unlockpt
	tiocptygname = 0x40807453 // ptsname, 128-byte out buffer
	tiocswinsz   = 0x80087467
)

type winsize struct{ row, col, x, y uint16 }

func ioctl(fd uintptr, req uintptr, arg uintptr) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg); e != 0 {
		return e
	}
	return nil
}

// openPty returns a master/slave pair with the slave sized rows x cols.
func openPty(rows, cols int) (master, slave *os.File, err error) {
	master, err = os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	fail := func(e error) (*os.File, *os.File, error) { master.Close(); return nil, nil, e }
	if err := ioctl(master.Fd(), tiocptygrant, 0); err != nil {
		return fail(err)
	}
	if err := ioctl(master.Fd(), tiocptyunlk, 0); err != nil {
		return fail(err)
	}
	var name [128]byte
	if err := ioctl(master.Fd(), tiocptygname, uintptr(unsafe.Pointer(&name[0]))); err != nil {
		return fail(err)
	}
	n := 0
	for n < len(name) && name[n] != 0 {
		n++
	}
	slave, err = os.OpenFile(string(name[:n]), os.O_RDWR, 0)
	if err != nil {
		return fail(err)
	}
	ws := winsize{row: uint16(rows), col: uint16(cols)}
	if err := ioctl(slave.Fd(), tiocswinsz, uintptr(unsafe.Pointer(&ws))); err != nil {
		slave.Close()
		return fail(err)
	}
	return master, slave, nil
}
