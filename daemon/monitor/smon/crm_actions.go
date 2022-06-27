package smon

import (
	"os"

	"opensvc.com/opensvc/util/command"
)

func (o *smon) crmStart() error {
	return o.crmAction(o.path.String(), "start", "--local")
}

func (o *smon) crmStatus() error {
	return o.crmAction(o.path.String(), "status", "-r")
}

func (o *smon) crmStop() error {
	return o.crmAction(o.path.String(), "stop", "--local")
}

func (o *smon) crmAction(cmdArgs ...string) error {
	cmdPath, err := os.Executable()
	if err != nil {
		o.log.Error().Err(err).Msgf("unable to detect opensvc executable")
		return err
	}
	cmd := command.New(
		command.WithName(cmdPath),
		command.WithArgs(cmdArgs),
		command.WithLogger(&o.log),
	)
	o.log.Debug().Msgf("-> exec %s %s", cmdPath, cmdArgs)
	if err := cmd.Run(); err != nil {
		o.log.Error().Err(err).Msgf("failed %s %s", o.path, cmdArgs)
		return err
	}
	o.log.Debug().Msgf("<- exec %s %s", cmdPath, cmdArgs)
	return nil
}
