//go:build linux

package udevadm

import (
	"github.com/opensvc/om3/util/command"
)

func Settle() {
	cmd := command.New(
		command.WithName("udevadm"),
		command.WithVarArgs("settle"),
	)
	cmd.Run()
}
