//go:build !linux && !darwin

package toc

import (
	"runtime"

	"github.com/pkg/errors"
)

func Reboot() error {
	return errors.Errorf("toc action 'reboot' is not implemented on %s", runtime.GOOS)
}

func Crash() error {
	return errors.Errorf("toc action 'crash' is not implemented on %s", runtime.GOOS)
}
