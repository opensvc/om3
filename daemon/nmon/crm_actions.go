package nmon

import (
	"os"
	"time"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/pubsub"
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

func (o *nmon) crmDrain() error {
	return o.crmAction("*/svc/*", "shutdown", "--local")
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
		command.WithLogger(o.log),
	)
	o.log.Debugf("-> exec %s %s", cmdPath, cmd)
	labels := []pubsub.Label{o.labelLocalhost, {"origin", "nmon"}}
	o.bus.Pub(&msgbus.Exec{Command: cmd.String(), Node: o.localhost, Origin: "nmon"}, labels...)
	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		duration := time.Now().Sub(startTime)
		o.bus.Pub(&msgbus.ExecFailed{Command: cmd.String(), Duration: duration, ErrS: err.Error(), Node: o.localhost, Origin: "nmon"}, labels...)
		o.log.Errorf("failed %s: %s", cmd, err)
		return err
	}
	duration := time.Now().Sub(startTime)
	o.bus.Pub(&msgbus.ExecSuccess{Command: cmd.String(), Duration: duration, Node: o.localhost, Origin: "nmon"}, labels...)
	o.log.Debugf("<- exec %s %s", cmdPath, cmd)
	return nil
}
