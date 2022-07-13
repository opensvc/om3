package smon

import (
	"os"

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

func (o *smon) crmDelete() error {
	return o.crmAction(o.path.String(), "delete")
}

func (o *smon) crmFreeze() error {
	return o.crmAction(o.path.String(), "freeze", "--local")
}

func (o *smon) crmProvision() error {
	return o.crmAction(o.path.String(), "provision", "--local")
}

func (o *smon) crmProvisionLeader() error {
	return o.crmAction(o.path.String(), "provision", "--leader", "--local")
}

func (o *smon) crmStart() error {
	return o.crmAction(o.path.String(), "start", "--local")
}

func (o *smon) crmStatus() error {
	return o.crmAction(o.path.String(), "status", "-r")
}

func (o *smon) crmStop() error {
	return o.crmAction(o.path.String(), "stop", "--local")
}

func (o *smon) crmUnfreeze() error {
	return o.crmAction(o.path.String(), "unfreeze", "--local")
}

func (o *smon) crmUnprovisionLeader() error {
	return o.crmAction(o.path.String(), "unprovision", "--leader", "--local")
}

func (o *smon) crmAction(cmdArgs ...string) error {
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
