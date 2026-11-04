//go:build linux

package restask

import (
	"errors"
	"syscall"

	"golang.org/x/sys/unix"
)

// signalRun sends sig to pid when isRun says it is still the process of its
// run. A pidfd, where the kernel has one, holds the process the check is
// made on, so the signal reaches it, or none if it ended: the pid can not be
// given to another process in between. The kernels of the older
// distributions have no pidfd: the signal then follows the check, a window
// open between the two.
func signalRun(pid int, sig syscall.Signal, isRun func(int) bool) error {
	fd, err := unix.PidfdOpen(pid, 0)
	switch {
	case err == nil:
		defer func() { _ = unix.Close(fd) }()
		if !isRun(pid) {
			return nil
		}
		if err := unix.PidfdSendSignal(fd, sig, nil, 0); err != nil && !errors.Is(err, unix.ESRCH) {
			return err
		}
		return nil
	case errors.Is(err, unix.ESRCH):
		return nil
	}
	if !isRun(pid) {
		return nil
	}
	if err := unix.Kill(pid, sig); err != nil && !errors.Is(err, unix.ESRCH) {
		return err
	}
	return nil
}
