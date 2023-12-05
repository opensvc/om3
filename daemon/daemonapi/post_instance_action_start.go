package daemonapi

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) apiExec(ctx echo.Context, p naming.Path, requesterSid uuid.UUID, args []string, log *plog.Logger) (uuid.UUID, error) {
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
			env.OriginSetenvArg(env.ActionOriginDaemonApi),
			"OSVC_SESSION_ID="+sid.String(),
			"OSVC_REQUEST_ID="+fmt.Sprint(ctx.Get("uuid")),
			"OSVC_REQUESTER_SESSION_ID="+fmt.Sprint(requesterSid),
		),
	)
	labels := []pubsub.Label{
		labelApi,
		{"path", p.String()},
		{"sid", sid.String()},
		{"requester_sid", requesterSid.String()},
	}
	log.Infof("-> exec %s", cmd)
	msg := msgbus.Exec{Command: cmd.String(), Node: hostname.Hostname(), Origin: "api"}
	a.EventBus.Pub(&msg, labels...)
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
			msg := msgbus.ExecFailed{Command: cmd.String(), Duration: duration, ErrS: err.Error(), Node: hostname.Hostname(), Origin: "api"}
			a.EventBus.Pub(&msg, labels...)
		} else {
			msg := msgbus.ExecSuccess{Command: cmd.String(), Duration: duration, Node: hostname.Hostname(), Origin: "api"}
			a.EventBus.Pub(&msg, labels...)
		}
	}()
	return sid, nil
}

func (a *DaemonApi) PostInstanceActionStart(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceActionStartParams) error {
	log := LogHandler(ctx, "PostInstanceActionStart")
	var requesterSid uuid.UUID
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)
	args := []string{p.String(), "start", "--local"}
	if params.DisableRollback != nil && *params.DisableRollback {
		args = append(args, "--disable-rollback")
	}
	if params.Force != nil && *params.Force {
		args = append(args, "--force")
	}
	if params.To != nil && *params.To != "" {
		args = append(args, "--to", *params.To)
	}
	if params.Rid != nil && *params.Rid != "" {
		args = append(args, "--rid", *params.Rid)
	}
	if params.Subset != nil && *params.Subset != "" {
		args = append(args, "--subset", *params.Subset)
	}
	if params.Tag != nil && *params.Tag != "" {
		args = append(args, "--tag", *params.Tag)
	}
	if params.RequesterSid != nil {
		requesterSid = *params.RequesterSid
	}
	if sid, err := a.apiExec(ctx, p, requesterSid, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.InstanceActionAccepted{SessionId: sid})
	}
}
