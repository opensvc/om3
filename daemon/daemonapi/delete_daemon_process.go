package daemonapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"syscall"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/proc"
	"golang.org/x/sys/unix"
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

	sig := syscall.SIGKILL
	if params.Signal != nil && *params.Signal != "" {
		var err error
		if sig, err = parseSignal(*params.Signal); err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
		}
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
		if err := syscall.Kill(pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Kill daemon process", "%s", err)
		}
	}

	return ctx.NoContent(http.StatusNoContent)
}

// parseSignal returns the signal designated by s, given as a number (9), a
// name (KILL) or a prefixed name (SIGKILL), case insensitive.
func parseSignal(s string) (syscall.Signal, error) {
	s = strings.TrimSpace(s)
	if num, err := strconv.Atoi(s); err == nil {
		sig := syscall.Signal(num)
		if unix.SignalName(sig) == "" {
			return 0, fmt.Errorf("unknown signal %s", s)
		}
		return sig, nil
	}
	name := strings.ToUpper(s)
	if !strings.HasPrefix(name, "SIG") {
		name = "SIG" + name
	}
	sig := unix.SignalNum(name)
	if sig == 0 {
		return 0, fmt.Errorf("unknown signal %s", s)
	}
	return sig, nil
}

func localDaemonProcessPIDSet() map[int]struct{} {
	items := proc.List([]string{}, nil, "")
	out := make(map[int]struct{}, len(items))
	for _, item := range items {
		out[item.Pid] = struct{}{}
	}
	return out
}
