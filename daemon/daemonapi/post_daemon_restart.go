package daemonapi

import (
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/util/command"
)

func (a *DaemonAPI) PostDaemonRestart(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
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

	// The pid tells the command it serves the api of this daemon: it
	// queues the restart from outside of the systemd unit, whose control
	// group it lives in, and does it only if this daemon still runs, so a
	// stop requested meanwhile wins.
	cmd := command.New(
		command.WithName(execname),
		command.WithArgs([]string{"daemon", "restart", "--api-daemon-pid", strconv.Itoa(os.Getpid())}),
	)

	err = cmd.Start()
	if err != nil {
		log.Errorf("called StartProcess: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "daemon restart failed: %s", err)
	}
	log.Infof("called daemon restart")
	return JSONProblem(ctx, http.StatusOK, "background daemon restart has been called", "")
}
