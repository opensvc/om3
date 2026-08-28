package scheduler

import (
	"fmt"
	"os"
	"slices"
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

// NodeActions and ObjectActions list the schedule entry actions CmdArgs knows
// how to run, split by the scope of the om command each one runs.
var (
	NodeActions = []string{
		"checks",
		"compliance_auto",
		"pushasset",
		"pushdisks",
		"pushpkg",
		"sysreport",
	}
	ObjectActions = []string{
		"push_resinfo",
		"resource_monitor",
		"run",
		"status",
		"sync_update",
	}
)

// CmdArgs returns the om argv the scheduler runs for the entry.
//
// These words are a contract with the om command tree, and nothing but that
// tree enforces it: an argv naming no command has om print a help text and
// exit 0, which the scheduler reports as a successful run. core/om's
// TestSchedulerCmdArgsResolve keeps this function and the tree in sync.
func CmdArgs(e schedule.Entry) ([]string, error) {
	var head, tail []string

	if slices.Contains(NodeActions, e.Action) {
		head = []string{"node"}
	} else {
		head = []string{e.Path.String()}
	}

	switch e.Action {
	case "status":
		tail = []string{"instance", "status", "-r"}
	case "resource_monitor":
		tail = []string{"instance", "status", "-m"}
	case "push_resinfo":
		tail = []string{"resource", "info", "-r"}
	case "run":
		tail = []string{"instance", "run", "--rid", e.RID()}
	case "sync_update":
		tail = []string{"instance", "update", "--rid", e.RID()}
	case "pushasset":
		tail = []string{"push", "asset"}
	case "pushdisks":
		tail = []string{"push", "disk"}
	case "pushpkg":
		tail = []string{"push", "pkg"}
	case "checks":
		tail = []string{"checks"}
	case "compliance_auto":
		tail = []string{"compliance", "auto"}
	case "sysreport":
		tail = []string{"sysreport"}
	default:
		return nil, fmt.Errorf("unknown scheduler action: %s", e.Action)
	}

	return append(head, tail...), nil
}

func (o *T) action(e schedule.Entry) error {
	logger := o.jobLogger(e)
	eid := xsession.NewEid()
	sid := xsession.NewSid()
	labels := []pubsub.Label{{"node", o.localhost}, {"origin", "scheduler"}}
	if !e.Path.IsZero() {
		labels = append(labels, pubsub.Label{"namespace", e.Path.Namespace}, pubsub.Label{"path", e.Path.String()})
	}
	cmdArgs, err := CmdArgs(e)
	if err != nil {
		logger.Errorf("%s", err)
		return err
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
	err = cmd.Wait()
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
