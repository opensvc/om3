package nmon

import (
	"os"
	"time"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/proc"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/pubsub"
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

func (t *Manager) crmDrain() error {
	return t.crmAction("*/svc/*", "instance", "shutdown")
}

func (t *Manager) crmFreeze() error {
	return t.crmAction("node", "freeze")
}

func (t *Manager) crmUnfreeze() error {
	return t.crmAction("node", "unfreeze")
}

func (t *Manager) crmAction(cmdArgs ...string) error {
	var cmdEnv []string
	cmdEnv = append(
		cmdEnv,
		env.OriginSetenvArg(env.ActionOriginDaemonMonitor),
	)

	// for tests
	if os.Getenv("OSVC_ROOT_PATH") != "" {
		cmdEnv = append(
			cmdEnv,
			"OSVC_ROOT_PATH="+os.Getenv("OSVC_ROOT_PATH"),
		)
	}

	cmd := command.New(
		command.WithName(cmdPath),
		command.WithArgs(cmdArgs),
		command.WithEnv(cmdEnv),
		command.WithLogger(t.log),
	)
	t.log.Tracef("-> exec %s %s", cmdPath, cmd)
	labels := []pubsub.Label{t.labelLocalhost, {"origin", "nmon"}}
	t.publisher.Pub(&msgbus.Exec{Command: cmd.String(), Node: t.localhost, Origin: "nmon"}, labels...)
	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		t.log.Errorf("exec StartProcess: %s", err)
		return err
	}
	pid := cmd.Cmd().Process.Pid
	proc.Register(proc.T{
		Pid:          pid,
		Node:         t.localhost,
		Object:       "-",
		Sid:          "-",
		StartedAt:    startTime,
		Elapsed:      "",
		GlobalExpect: t.state.GlobalExpect.String(),
		Sub:          "nmon",
		Desc:         cmd.String(),
	})
	err := cmd.Wait()
	proc.Unregister(pid)
	if err != nil {
		duration := time.Now().Sub(startTime)
		t.publisher.Pub(&msgbus.ExecFailed{Command: cmd.String(), Duration: duration, ErrS: err.Error(), Node: t.localhost, Origin: "nmon"}, labels...)
		t.log.Errorf("failed %s: %s", cmd, err)
		return err
	}
	duration := time.Now().Sub(startTime)
	t.publisher.Pub(&msgbus.ExecSuccess{Command: cmd.String(), Duration: duration, Node: t.localhost, Origin: "nmon"}, labels...)
	t.log.Tracef("<- exec %s %s", cmdPath, cmd)
	return nil
}
