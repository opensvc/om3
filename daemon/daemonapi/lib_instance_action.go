package daemonapi

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonAPI) apiExec(ctx echo.Context, p naming.Path, requesterSid uuid.UUID, args []string, log *plog.Logger) (uuid.UUID, error) {
	execname, err := os.Executable()
	if err != nil {
		return uuid.Nil, fmt.Errorf("can't detect om execname: %w", err)
	}
	sid := uuid.New()
	cmd := command.New(
		command.WithName(execname),
		command.WithArgs(args),
		command.WithLogger(log),
		command.WithVarEnv(
			env.OriginSetenvArg(env.ActionOriginDaemonAPI),
			"OSVC_SESSION_ID="+sid.String(),
			"OSVC_REQUEST_ID="+fmt.Sprint(ctx.Get("uuid")),
			"OSVC_REQUESTER_SESSION_ID="+fmt.Sprint(requesterSid),
		),
	)
	labels := []pubsub.Label{labelOriginAPI}
	if !p.IsZero() {
		labels = append(labels, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()})
	}
	log.Infof("-> exec %s", cmd)
	msg := msgbus.Exec{Command: cmd.String(), Node: a.localhost,
		Origin: "api", SessionID: sid, RequesterSessionID: requesterSid}
	a.Pub.Pub(&msg, labels...)
	startTime := time.Now()
	if err = cmd.Start(); err != nil {
		log.Errorf("exec StartProcess: %s", err)
		return sid, fmt.Errorf("instance action failed: %w", err)
	}
	go func() {
		err := cmd.Wait()
		log.Infof("<- exec %s", cmd)
		duration := time.Now().Sub(startTime)
		if err != nil {
			msg := msgbus.ExecFailed{Command: cmd.String(), Duration: duration, Node: a.localhost,
				Origin: "api", SessionID: sid, RequesterSessionID: requesterSid,
				ErrS: err.Error(),
			}
			a.Pub.Pub(&msg, labels...)
		} else {
			msg := msgbus.ExecSuccess{Command: cmd.String(), Duration: duration, Node: a.localhost,
				Origin: "api", SessionID: sid, RequesterSessionID: requesterSid}
			a.Pub.Pub(&msg, labels...)
		}
	}()
	return sid, nil
}
