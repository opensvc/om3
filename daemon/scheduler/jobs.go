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
		"pusharray",
		"pushasset",
		"pushdisks",
		"pushpkg",
		"sysreport",
	}

	// PlaceholderActions are the actions a schedule entry can carry that no om
	// command implements yet.
	//
	// The node configuration schedules one per switch and per backup section,
	// as it does per array section, and nothing pushes those two yet. Naming
	// them here says the entry is known and unimplemented, rather than letting
	// it fall through as an action nobody has heard of, and wiring one later
	// is a case in CmdArgs and a move to NodeActions.
	//
	// None of the three has a default schedule, so an entry only exists where
	// a configuration wrote one.
	PlaceholderActions = []string{
		"pushbackup",
		"pushswitch",
	}
	ObjectActions = []string{
		"info",
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

	if slices.Contains(PlaceholderActions, e.Action) {
		return nil, fmt.Errorf("scheduler action %s has no om command yet", e.Action)
	}

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
	case "info":
		tail = []string{"instance", "info", "--refresh"}
	case "run":
		tail = []string{"instance", "run", "--rid", e.RID()}
	case "sync_update":
		tail = []string{"instance", "update", "--rid", e.RID()}
	case "pushasset":
		tail = []string{"push", "asset"}
	case "pusharray":
		// The array is the section the schedule was read from.
		tail = []string{"push", "array", e.RID()}
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
	execID := xsession.NewExecID()
	sessionID := xsession.NewSessionID()
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
		xsession.SessionID().ParentVar(),
		execID.Var(),
		sessionID.Var(),
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
	startTime := time.Now()
	o.publisher.Pub(&msgbus.Exec{
		Command:   cmd.String(),
		Node:      o.localhost,
		Origin:    "scheduler",
		RID:       e.RID(),
		ExecID:    execID,
		SessionID: sessionID,
		StartedAt: startTime,
	}, labels...)
	if err := cmd.Start(); err != nil {
		// The start of this exec was announced, so its end has to be too, or
		// it stays running in the exec store for as long as the store keeps
		// it. There is no exit status to report: it never ran.
		o.publisher.Pub(&msgbus.ExecFailed{
			Command:   cmd.String(),
			Duration:  time.Now().Sub(startTime),
			ErrS:      err.Error(),
			ExitCode:  -1,
			Node:      o.localhost,
			Origin:    "scheduler",
			ExecID:    execID,
			SessionID: sessionID,
		}, labels...)
		o.log.Errorf("exec StartProcess: %s", err)
		return err
	}
	pid := cmd.Cmd().Process.Pid
	proc.Register(proc.T{Pid: pid, ExecID: execID.String()})
	err = cmd.Wait()
	proc.Unregister(pid)
	if err != nil {
		duration := time.Now().Sub(startTime)
		o.publisher.Pub(&msgbus.ExecFailed{
			Command:   cmd.String(),
			Duration:  duration,
			ErrS:      err.Error(),
			ExitCode:  cmd.NormalizedExitCode(),
			Node:      o.localhost,
			Origin:    "scheduler",
			ExecID:    execID,
			SessionID: sessionID,
		}, labels...)
		logger.Errorf("%s: %s", cmd, err)
		return err
	}
	duration := time.Now().Sub(startTime)
	o.publisher.Pub(&msgbus.ExecSuccess{
		Command:   cmd.String(),
		Duration:  duration,
		ExitCode:  cmd.NormalizedExitCode(),
		Node:      o.localhost,
		Origin:    "scheduler",
		ExecID:    execID,
		SessionID: sessionID,
	}, labels...)
	logger.Debugf("<- exec %s", cmd)
	return nil
}
