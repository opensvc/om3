package smon

import (
	"os"

	"opensvc.com/opensvc/util/command"
)

func (o *smon) action(cmdArgs ...string) {
	cmdPath, err := os.Executable()
	if err != nil {
		o.log.Error().Err(err).Msgf("unable to detect opensvc executable")
		return
	}
	cmd := command.New(
		command.WithName(cmdPath),
		command.WithArgs(cmdArgs),
		command.WithLogger(&o.log),
	)
	o.log.Debug().Msgf("-> exec %s %s", cmdPath, cmdArgs)
	if err := cmd.Run(); err != nil {
		o.log.Error().Err(err).Msgf("failed %s %s", o.path, cmdArgs)
	}
	o.log.Debug().Msgf("<- exec %s %s", cmdPath, cmdArgs)
}

func (o *smon) status() {
	o.action(o.path.String(), "status", "-r")
}
