//go:build !linux

package console

import (
	"errors"
	"os"
	"os/exec"
)

var errNoPTY = errors.New("console sessions are not supported on this platform")

func startInPTY(_ *exec.Cmd) (*os.File, error) {
	return nil, errNoPTY
}

func setPTYSize(_ *os.File, _, _ uint16) error {
	return errNoPTY
}
