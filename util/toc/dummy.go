//go:build !linux && !darwin

package toc

import (
	"runtime"
)

func Reboot() error {
	return fmt.Errorf("toc action 'reboot' is not implemented on %s", runtime.GOOS)
}

func Crash() error {
	return fmt.Errorf("toc action 'crash' is not implemented on %s", runtime.GOOS)
}
