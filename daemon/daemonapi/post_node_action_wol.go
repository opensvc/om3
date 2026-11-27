package daemonapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/wakeonlan"
)

func (a *DaemonAPI) PostNodeActionWol(ctx echo.Context, nodename string, params api.PostNodeActionWolParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if err := validateNodeActionWolParams(params); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionWol(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionWol(ctx.Request().Context(), nodename, &params)
	})
}

func validateNodeActionWolParams(params api.PostNodeActionWolParams) error {
	var errs error
	if len(params.Mac) == 0 {
		errs = errors.Join(errs, errors.New("no mac address"))
	}
	if len(params.Broadcast) == 0 {
		errs = errors.Join(errs, errors.New("no broadcast address"))
	}
	for _, s := range params.Mac {
		if _, err := wakeonlan.ParseMAC(s); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	for _, s := range params.Broadcast {
		if _, err := wakeonlan.ParseBroadcast(s); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	if params.Port != nil {
		for _, port := range *params.Port {
			if err := wakeonlan.ValidatePort(port); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}
	return errs
}

func (a *DaemonAPI) localNodeActionWol(ctx echo.Context, params api.PostNodeActionWolParams) error {
	log := LogHandler(ctx, "PostNodeActionWol")
	var requesterSessionID uuid.UUID
	args := []string{
		"node", "wol",
		"--mac=" + strings.Join(params.Mac, ","),
		"--broadcast=" + strings.Join(params.Broadcast, ","),
	}
	if params.Port != nil && len(*params.Port) > 0 {
		ports := make([]string, len(*params.Port))
		for i, port := range *params.Port {
			ports[i] = strconv.Itoa(port)
		}
		args = append(args, "--port="+strings.Join(ports, ","))
	}
	if params.SessionID != nil {
		requesterSessionID = *params.SessionID
	}
	if sessionID, execID, err := a.apiExec(ctx, naming.Path{}, requesterSessionID, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionID: sessionID, ExecID: execID})
	}
}
