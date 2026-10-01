//go:build linux

package console

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// startInPTY starts the command with a new pseudo terminal as its
// controlling terminal, and returns the master side of it.
//
// The command leads a session of its own, so closing the master hangs up
// the terminal of everything the command started, and nothing else.
func startInPTY(cmd *exec.Cmd) (*os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		_ = master.Close()
		return nil, fmt.Errorf("unlock pty: %w", err)
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		_ = master.Close()
		return nil, fmt.Errorf("pty number: %w", err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		_ = master.Close()
		return nil, err
	}
	defer slave.Close()
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}
	if err := cmd.Start(); err != nil {
		_ = master.Close()
		return nil, err
	}
	return master, nil
}

// setPTYSize tells the terminal, and through it the command, the size of the
// terminal of the client.
func setPTYSize(master *os.File, cols, rows uint16) error {
	return unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: cols})
}
