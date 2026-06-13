package scheduler

import (
	"fmt"
	"os"
	"time"

	"github.com/opensvc/om3/v3/daemon/proc"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/xsession"
)

func (o *T) action(e schedule.Entry) error {
	logger := o.jobLogger(e)
	eid := xsession.NewEid()
	sid := xsession.NewSid()
	labels := []pubsub.Label{{"node", o.localhost}, {"origin", "scheduler"}}
	cmdArgs := []string{}
	if e.Path.IsZero() {
		cmdArgs = append(cmdArgs, "node")
	} else {
		p := e.Path.String()
		cmdArgs = append(cmdArgs, p, "instance")
		labels = append(labels, pubsub.Label{"namespace", e.Path.Namespace}, pubsub.Label{"path", p})
	}
	switch e.Action {
	case "status":
		cmdArgs = append(cmdArgs, "status", "-r")
	case "resource_monitor":
		cmdArgs = append(cmdArgs, "status", "-m")
	case "push_resinfo":
		cmdArgs = append(cmdArgs, "resource", "info", "push")
	case "run":
		cmdArgs = append(cmdArgs, "run", "--rid", e.RID())
	case "pushasset":
		cmdArgs = append(cmdArgs, "push", "asset")
	case "reboot":
		cmdArgs = append(cmdArgs, "reboot")
	case "checks":
		cmdArgs = append(cmdArgs, "checks")
	case "compliance_auto":
		cmdArgs = append(cmdArgs, "compliance", "auto")
	case "pushdisks":
		cmdArgs = append(cmdArgs, "push", "disk")
	case "pushpkg":
		cmdArgs = append(cmdArgs, "push", "pkg")
	case "pushpatch":
		cmdArgs = append(cmdArgs, "push", "patch")
	case "pushstats":
		cmdArgs = append(cmdArgs, "push", "stats")
	case "sysreport":
		cmdArgs = append(cmdArgs, "sysreport")
	case "sync_update":
		cmdArgs = append(cmdArgs, "sync", "update")
	default:
		logger.Errorf("unknown scheduler action")
		return fmt.Errorf("unknown scheduler action")
	}
	var cmdEnv []string
	cmdEnv = append(
		cmdEnv,
		env.ActionOriginDaemonScheduler.Var(),
		xsession.Sid().ParentVar(),
		eid.Var(),
		sid.Var(),
	)

	// Unless the daemon runs with --debug or --trace, we don't want to
	// log the execution in journald nor syslogd to avoid uncontrolled
	// growth or rotation of the logging backend files.
	if lvl := zerolog.GlobalLevel(); lvl > zerolog.DebugLevel {
		// OSVC_NO_LOG_FILE=1
		cmdEnv = append(cmdEnv, env.NoLogFileSetenvArg())
	}

	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs(cmdArgs),
		command.WithLogger(logger),
		command.WithEnv(cmdEnv),
	)
	logger.Debugf("-> exec %s", cmd)
	o.publisher.Pub(&msgbus.Exec{
		Command:   cmd.String(),
		Node:      o.localhost,
		Origin:    "scheduler",
		ExecID:    eid,
		SessionID: sid,
	}, labels...)
	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		o.log.Errorf("exec StartProcess: %s", err)
		return err
	}
	pid := cmd.Cmd().Process.Pid
	proc.Register(proc.T{
		Pid:          pid,
		Node:         o.localhost,
		Object:       e.Path.String(),
		Sid:          sid.String(),
		StartedAt:    startTime,
		Elapsed:      "",
		GlobalExpect: "-",
		Sub:          "scheduler",
		Cmd:          cmd.String(),
		Rid:          e.RID(),
	})
	err := cmd.Wait()
	proc.Unregister(pid)
	if err != nil {
		duration := time.Now().Sub(startTime)
		o.publisher.Pub(&msgbus.ExecFailed{
			Command:   cmd.String(),
			Duration:  duration,
			ErrS:      err.Error(),
			Node:      o.localhost,
			Origin:    "scheduler",
			ExecID:    eid,
			SessionID: sid,
		}, labels...)
		logger.Errorf("%s: %s", cmd, err)
		return err
	}
	duration := time.Now().Sub(startTime)
	o.publisher.Pub(&msgbus.ExecSuccess{
		Command:   cmd.String(),
		Duration:  duration,
		Node:      o.localhost,
		Origin:    "scheduler",
		ExecID:    eid,
		SessionID: sid,
	}, labels...)
	logger.Debugf("<- exec %s", cmd)
	return nil
}
