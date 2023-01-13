//go:build darwin

package toc

import (
	"opensvc.com/opensvc/util/command"
)

func Reboot() error {
	cmd := command.New(
		command.WithName("reboot"),
		command.WithVarArgs("-q"),
	)
	return cmd.Run()
}

func Crash() error {
	cmd := command.New(
		command.WithName("halt"),
		command.WithVarArgs("-q"),
	)
	return cmd.Run()
}
