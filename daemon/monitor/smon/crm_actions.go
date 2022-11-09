package smon

import (
	"os"

	"opensvc.com/opensvc/util/command"
)

var (
	cmdPath string

	maxRunners = 25

	// runners chan limit number of // crmActions to maxRunners
	runners = make(chan struct{}, maxRunners)
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

func (o *smon) doAction(action func() error, newState, successState, errorState string) {
	o.transitionTo(newState)
	go func() {
		//o.log.Info().Msgf("in progress '%s' to reach target global expect '%s'", newState, o.state.GlobalExpect)
		var nextState string
		if action() == nil {
			nextState = successState
		} else {
			nextState = errorState
		}
		o.cmdC <- cmdOrchestrate{state: newState, newState: nextState}
	}()
}

func (o *smon) crmDelete() error {
	return o.crmAction("delete", o.path.String(), "delete")
}

func (o *smon) crmFreeze() error {
	return o.crmAction("freeze", o.path.String(), "freeze", "--local")
}

func (o *smon) crmProvisionNonLeader() error {
	return o.crmAction("provision non leader", o.path.String(), "provision", "--local")
}

func (o *smon) crmProvisionLeader() error {
	return o.crmAction("provision leader", o.path.String(), "provision", "--local", "--leader", "--disable-rollback")
}

func (o *smon) crmStart() error {
	return o.crmAction("start", o.path.String(), "start", "--local")
}

func (o *smon) crmStatus() error {
	return o.crmAction("", o.path.String(), "status", "-r")
}

func (o *smon) crmStop() error {
	return o.crmAction("stop", o.path.String(), "stop", "--local")
}

func (o *smon) crmUnfreeze() error {
	return o.crmAction("unfreeze", o.path.String(), "unfreeze", "--local")
}

func (o *smon) crmUnprovisionNonLeader() error {
	return o.crmAction("unprovision non leader", o.path.String(), "unprovision", "--local")
}

func (o *smon) crmUnprovisionLeader() error {
	return o.crmAction("unprovision leader", o.path.String(), "unprovision", "--local", "--leader")
}

func (o *smon) crmAction(title string, cmdArgs ...string) error {
	runners <- struct{}{}
	defer func() {
		<-runners
	}()
	cmd := command.New(
		command.WithName(cmdPath),
		command.WithArgs(cmdArgs),
		command.WithLogger(&o.log),
	)
	if title != "" {
		o.loggerWithState().Info().Msgf(
			"crm action %s (local status:'%s') -> exec %s %s",
			title, o.state.Status, cmdPath, cmdArgs,
		)
	} else {
		o.loggerWithState().Debug().Msgf("-> exec %s %s", cmdPath, cmdArgs)
	}
	if err := cmd.Run(); err != nil {
		o.loggerWithState().Error().Err(err).Msgf("failed %s %s", o.path, cmdArgs)
		return err
	}
	if title != "" {
		o.loggerWithState().Info().Msgf(
			"crm action %s (local status:'%s') <- exec %s %s",
			title, o.state.Status, cmdPath, cmdArgs,
		)
	} else {
		o.loggerWithState().Debug().Msgf("<- exec %s %s", cmdPath, cmdArgs)
	}
	return nil
}
