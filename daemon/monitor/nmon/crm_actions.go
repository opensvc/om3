package nmon

import (
	"os"

	"opensvc.com/opensvc/core/env"
	"opensvc.com/opensvc/util/command"
)

var (
	cmdPath string
)

func init() {
	var err error
	cmdPath, err = os.Executable()
	if err != nil {
		cmdPath = "/bin/false"
	}
}

// SetCmdPathForTest set the opensvc command path for tests
func SetCmdPathForTest(s string) {
	// TODO use another method to create dedicated side effects for tests
	cmdPath = s
}

func (o *nmon) crmFreeze() error {
	return o.crmAction("node", "freeze", "--local")
}

func (o *nmon) crmUnfreeze() error {
	return o.crmAction("node", "unfreeze", "--local")
}

func (o *nmon) crmAction(cmdArgs ...string) error {
	var cmdEnv []string
	cmdEnv = append(
		cmdEnv,
		env.DaemonOriginSetenvArg(),
	)
	cmd := command.New(
		command.WithName(cmdPath),
		command.WithArgs(cmdArgs),
		command.WithEnv(cmdEnv),
		command.WithLogger(&o.log),
	)
	o.log.Debug().Msgf("-> exec %s %s", cmdPath, cmdArgs)
	if err := cmd.Run(); err != nil {
		o.log.Error().Err(err).Msgf("failed %s", cmdArgs)
		return err
	}
	o.log.Debug().Msgf("<- exec %s %s", cmdPath, cmdArgs)
	return nil
}
