// +build linux
package udevadm

import (
	"opensvc.com/opensvc/util/command"
)

func Settle() {
	cmd := command.New(
		command.WithName("udevadm"),
		command.WithVarArgs("settle"),
	)
	cmd.Run()
}
