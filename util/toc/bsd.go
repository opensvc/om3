//go:build darwin

package toc

import (
	"github.com/opensvc/om3/v3/util/command"
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
