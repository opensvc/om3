package daemonapi

import (
	"errors"
	"net/http"
	"syscall"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/proc"
)

func (a *DaemonAPI) DeleteDaemonProcess(ctx echo.Context, nodename string, params api.DeleteDaemonProcessParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}

	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.DeleteDaemonProcess(ctx.Request().Context(), nodename, &params)
		})
	}
	return a.deleteLocalDaemonProcess(ctx, params)
}

func (a *DaemonAPI) deleteLocalDaemonProcess(ctx echo.Context, params api.DeleteDaemonProcessParams) error {
	if params.Pid == nil || len(*params.Pid) == 0 {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "missing pid")
	}

	allowed := localDaemonProcessPIDSet()

	seen := make(map[int]struct{}, len(*params.Pid))
	for _, pid := range *params.Pid {
		if pid <= 0 {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid pid %d", pid)
		}
		if _, done := seen[pid]; done {
			continue
		}
		seen[pid] = struct{}{}

		if _, ok := allowed[pid]; !ok {
			return JSONProblemf(ctx, http.StatusBadRequest, "Not a daemon process", "invalid pid %d", pid)
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Kill daemon process", "%s", err)
		}
	}

	return ctx.NoContent(http.StatusNoContent)
}

func localDaemonProcessPIDSet() map[int]struct{} {
	items := proc.List([]string{})
	out := make(map[int]struct{}, len(items))
	for _, item := range items {
		out[item.Pid] = struct{}{}
	}
	return out
}
