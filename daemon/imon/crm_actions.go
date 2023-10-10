package imon

import (
	"os"
	"strings"
	"time"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/pubsub"
)

var (
	cmdPath string

	maxRunners = 25

	// runners chan limit number of // crmActions to maxRunners
	runners = make(chan struct{}, maxRunners)

	// crmAction can be used to define alternate crmAction for tests
	crmAction func(title string, cmdArgs ...string) error
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

func (o *imon) orchestrateAfterAction(state, newState instance.MonitorState) {
	select {
	case <-o.ctx.Done():
		return
	default:
	}
	o.cmdC <- cmdOrchestrate{state: state, newState: newState}
}

func (o *imon) crmBoot() error {
	return o.crmAction("boot", o.path.String(), "boot", "--local")
}

func (o *imon) crmDelete() error {
	return o.crmAction("delete", o.path.String(), "delete", "--local")
}

func (o *imon) crmFreeze() error {
	return o.crmAction("freeze", o.path.String(), "freeze", "--local")
}

func (o *imon) crmProvisionNonLeader() error {
	return o.crmAction("provision non leader", o.path.String(), "provision", "--local")
}

func (o *imon) crmProvisionLeader() error {
	return o.crmAction("provision leader", o.path.String(), "provision", "--local", "--leader", "--disable-rollback")
}

func (o *imon) crmResourceStartStandby(rids []string) error {
	s := strings.Join(rids, ",")
	return o.crmAction("start", o.path.String(), "startstandby", "--local", "--rid", s)
}

func (o *imon) crmResourceStart(rids []string) error {
	s := strings.Join(rids, ",")
	return o.crmAction("start", o.path.String(), "start", "--local", "--rid", s)
}

func (o *imon) crmStart() error {
	return o.crmAction("start", o.path.String(), "start", "--local")
}

func (o *imon) crmStatus() error {
	return o.crmAction("status", o.path.String(), "status", "-r")
}

func (o *imon) crmStop() error {
	return o.crmAction("stop", o.path.String(), "stop", "--local")
}

func (o *imon) crmUnfreeze() error {
	return o.crmAction("unfreeze", o.path.String(), "unfreeze", "--local")
}

func (o *imon) crmUnprovisionNonLeader() error {
	return o.crmAction("unprovision non leader", o.path.String(), "unprovision", "--local")
}

func (o *imon) crmUnprovisionLeader() error {
	return o.crmAction("unprovision leader", o.path.String(), "unprovision", "--local", "--leader")
}

func (o *imon) crmAction(title string, cmdArgs ...string) error {
	if crmAction != nil {
		return crmAction(title, cmdArgs...)
	}
	return o.crmDefaultAction(title, cmdArgs...)
}

func (o *imon) crmDefaultAction(title string, cmdArgs ...string) error {
	runners <- struct{}{}
	defer func() {
		<-runners
	}()
	cmd := command.New(
		command.WithName(cmdPath),
		command.WithArgs(cmdArgs),
		command.WithLogger(&o.log),
		command.WithVarEnv(env.DaemonOriginSetenvArg()),
	)
	labels := []pubsub.Label{o.labelLocalhost, o.labelPath, {"origin", "imon"}}
	if title != "" {
		o.loggerWithState().Info().Msgf(
			"crm action %s (instance state: %s) -> exec %s %s",
			title, o.state.State, cmdPath, cmdArgs,
		)
	} else {
		o.loggerWithState().Debug().Msgf("-> exec %s %s", cmdPath, cmdArgs)
	}
	o.pubsubBus.Pub(&msgbus.Exec{Command: cmd.String(), Node: o.localhost, Origin: "imon", Title: title}, labels...)
	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		duration := time.Now().Sub(startTime)
		o.pubsubBus.Pub(&msgbus.ExecFailed{Command: cmd.String(), Duration: duration, ErrS: err.Error(), Node: o.localhost, Origin: "imon", Title: title}, labels...)
		o.loggerWithState().Error().Err(err).Msgf("failed %s %s", o.path, cmdArgs)
		return err
	}
	duration := time.Now().Sub(startTime)
	o.pubsubBus.Pub(&msgbus.ExecSuccess{Command: cmd.String(), Duration: duration, Node: o.localhost, Origin: "imon", Title: title}, labels...)
	if title != "" {
		o.loggerWithState().Info().Msgf(
			"crm action %s (instance state: %s) <- exec %s %s",
			title, o.state.State, cmdPath, cmdArgs,
		)
	} else {
		o.loggerWithState().Debug().Msgf("<- exec %s %s", cmdPath, cmdArgs)
	}
	return nil
}
