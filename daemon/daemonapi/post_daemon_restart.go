package daemonapi

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/util/command"
)

func (a *DaemonAPI) PostDaemonRestart(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}

	if nodename == a.localhost {
		return a.localPostDaemonRestart(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	c, err := a.newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	resp, err := c.PostDaemonRestartWithResponse(ctx.Request().Context(), nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonAPI) localPostDaemonRestart(ctx echo.Context) error {
	log := LogHandler(ctx, "PostDaemonRestart")
	log.Infof("starting")

	execname, err := os.Executable()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "can't detect om execname: %s", err)
	}

	cmd := command.New(
		command.WithName(execname),
		command.WithArgs([]string{"daemon", "restart"}),
	)

	err = cmd.Start()
	if err != nil {
		log.Errorf("called StartProcess: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "daemon restart failed: %s", err)
	}
	log.Infof("called daemon restart")
	return JSONProblem(ctx, http.StatusOK, "background daemon restart has been called", "")
}
