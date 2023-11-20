package daemonapi

import (
	"fmt"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/command"
)

func (a *DaemonApi) PostInstanceActionStop(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceActionStopParams) error {
	log := LogHandler(ctx, "PostInstanceActionStop")
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)
	execname, err := os.Executable()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "can't detect om execname: %s", err)
	}
	args := []string{p.String(), "stop", "--local"}
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
	sid := uuid.New()
	cmd := command.New(
		command.WithName(execname),
		command.WithArgs(args),
		command.WithLogger(log),
		command.WithVarEnv(
			env.OriginSetenvArg(env.ActionOriginDaemonApi),
			"OSVC_SESSION_ID="+sid.String(),
			"OSVC_REQUEST_ID="+fmt.Sprint(ctx.Get("uuid")),
		),
	)
	if err = cmd.Start(); err != nil {
		log.Errorf("exec StartProcess: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "instance action failed: %s", err)
	}
	log.Infof("-> exec %s", cmd)
	if err := ctx.JSON(http.StatusOK, api.InstanceActionAccepted{SessionId: sid}); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	log.Infof("<- exec %s", cmd)
	return nil
}
