//go:build !linux

package restask

import (
	"errors"
	"syscall"
)

// signalRun sends sig to pid when isRun says it is still the process of its
// run, the signal following the check.
func signalRun(pid int, sig syscall.Signal, isRun func(int) bool) error {
	if !isRun(pid) {
		return nil
	}
	if err := syscall.Kill(pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
