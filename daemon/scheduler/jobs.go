package scheduler

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/pubsub"
)

func (o *T) action(e schedule.Entry) error {
	sid := uuid.New()
	labels := []pubsub.Label{{"node", o.localhost}, {"origin", "scheduler"}}
	cmdArgs := []string{}
	if e.Path.IsZero() {
		cmdArgs = append(cmdArgs, "node")
	} else {
		p := e.Path.String()
		cmdArgs = append(cmdArgs, p)
		labels = append(labels, pubsub.Label{"namespace", e.Path.Namespace}, pubsub.Label{"path", p})
	}
	switch e.Action {
	case "status":
		cmdArgs = append(cmdArgs, "status", "-r", "--local")
	case "resource_monitor":
		cmdArgs = append(cmdArgs, "status", "-m", "--local")
	case "push_resinfo":
		cmdArgs = append(cmdArgs, "push", "resinfo", "--local")
	case "run":
		cmdArgs = append(cmdArgs, "run", "--rid", e.RID(), "--local")
	case "pushasset":
		cmdArgs = append(cmdArgs, "push", "asset", "--local")
	case "reboot":
		cmdArgs = append(cmdArgs, "reboot", "--local")
	case "checks":
		cmdArgs = append(cmdArgs, "checks", "--local")
	case "compliance_auto":
		cmdArgs = append(cmdArgs, "compliance", "auto", "--local")
	case "pushdisks":
		cmdArgs = append(cmdArgs, "push", "disk", "--local")
	case "pushpkg":
		cmdArgs = append(cmdArgs, "push", "pkg", "--local")
	case "pushpatch":
		cmdArgs = append(cmdArgs, "push", "patch", "--local")
	case "pushstats":
		cmdArgs = append(cmdArgs, "push", "stats", "--local")
	case "sysreport":
		cmdArgs = append(cmdArgs, "sysreport", "--local")
	case "sync_update":
		cmdArgs = append(cmdArgs, "sync", "update", "--local")
	//case "collect_stats":
	//	cmdArgs = append(cmdArgs, "collect", "stats", "--local")
	//case "dequeue_actions":
	//	cmdArgs = append(cmdArgs, "dequeue", "--local")
	//case "rotate_root_pw":
	//	cmdArgs = append(cmdArgs, "rotate", "root", "pw", "--local")
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
	o.log.Debugf("-> exec %s", cmd)
	o.pubsub.Pub(&msgbus.Exec{Command: cmd.String(), Node: o.localhost, Origin: "scheduler", SessionID: sid}, labels...)
	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		duration := time.Now().Sub(startTime)
		o.pubsub.Pub(&msgbus.ExecFailed{Command: cmd.String(), Duration: duration, ErrS: err.Error(), Node: o.localhost, Origin: "scheduler", SessionID: sid}, labels...)
		o.log.Attr("cmd", cmd.String()).Errorf("exec error: %s", err)
		return err
	}
	duration := time.Now().Sub(startTime)
	o.pubsub.Pub(&msgbus.ExecSuccess{Command: cmd.String(), Duration: duration, Node: o.localhost, Origin: "scheduler", SessionID: sid}, labels...)
	o.log.Debugf("<- exec %s", cmd)
	return nil
}
