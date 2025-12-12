package scheduler

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/pubsub"
)

func (o *T) action(e schedule.Entry) error {
	sid := uuid.New()
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
	//case "dequeue_actions":
	//	cmdArgs = append(cmdArgs, "dequeue")
	default:
		o.log.Attr("action", e.Action).Attr("path", e.Path.String()).Errorf("unknown scheduler action")
		return fmt.Errorf("unknown scheduler action")
	}
	var cmdEnv []string
	cmdEnv = append(
		cmdEnv,
		env.OriginSetenvArg(env.ActionOriginDaemonScheduler),
		env.ParentSessionIDSetenvArg(),
		"OSVC_SESSION_ID="+sid.String(),
	)

	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs(cmdArgs),
		command.WithLogger(o.log),
		command.WithEnv(cmdEnv),
	)
	o.log.Tracef("-> exec %s", cmd)
	o.publisher.Pub(&msgbus.Exec{Command: cmd.String(), Node: o.localhost, Origin: "scheduler", SessionID: sid}, labels...)
	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		duration := time.Now().Sub(startTime)
		o.publisher.Pub(&msgbus.ExecFailed{Command: cmd.String(), Duration: duration, ErrS: err.Error(), Node: o.localhost, Origin: "scheduler", SessionID: sid}, labels...)
		o.log.Attr("cmd", cmd.String()).Errorf("%s: %s", cmd, err)
		return err
	}
	duration := time.Now().Sub(startTime)
	o.publisher.Pub(&msgbus.ExecSuccess{Command: cmd.String(), Duration: duration, Node: o.localhost, Origin: "scheduler", SessionID: sid}, labels...)
	o.log.Tracef("<- exec %s", cmd)
	return nil
}
