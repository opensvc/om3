package scheduler

import (
	"fmt"
	"os"
	"time"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/pubsub"
)

func (o T) action(e schedule.Entry) error {
	labels := []pubsub.Label{{"node", o.localhost}, {"origin", "scheduler"}}
	cmdArgs := []string{}
	if e.Path.IsZero() {
		cmdArgs = append(cmdArgs, "node")
	} else {
		p := e.Path.String()
		cmdArgs = append(cmdArgs, p)
		labels = append(labels, pubsub.Label{"path", p})
	}
	switch e.Action {
	case "status":
		cmdArgs = append(cmdArgs, "status", "-r", "--local")
	case "resource_monitor":
		cmdArgs = append(cmdArgs, "resource", "monitor", "--local")
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
		o.log.Error().Str("action", e.Action).Stringer("path", e.Path).Msg("unknown scheduler action")
		return fmt.Errorf("unknown scheduler action")
	}
	var cmdEnv []string
	cmdEnv = append(
		cmdEnv,
		env.DaemonOriginSetenvArg(),
		env.ParentSessionIDSetenvArg(),
	)

	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs(cmdArgs),
		command.WithLogger(&o.log),
		command.WithEnv(cmdEnv),
	)
	o.log.Debug().Msgf("-> exec %s", cmd)
	o.pubsub.Pub(&msgbus.Exec{Command: cmd.String(), Node: o.localhost, Origin: "scheduler"}, labels...)
	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		duration := time.Now().Sub(startTime)
		o.pubsub.Pub(&msgbus.ExecFailed{Command: cmd.String(), Duration: duration, ErrS: err.Error(), Node: o.localhost, Origin: "scheduler"}, labels...)
		o.log.Error().Err(err).Stringer("cmd", cmd).Msg("exec error")
		return err
	}
	duration := time.Now().Sub(startTime)
	o.pubsub.Pub(&msgbus.ExecSuccess{Command: cmd.String(), Duration: duration, Node: o.localhost, Origin: "scheduler"}, labels...)
	o.log.Debug().Msgf("<- exec %s", cmd)
	return nil
}
