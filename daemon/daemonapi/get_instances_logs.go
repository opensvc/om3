package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetInstanceLogs(ctx echo.Context, nodename string, namespace string, kind naming.Kind, name string, params api.GetInstanceLogsParams) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
		return err
	}
	return a.GetNodeLogs(ctx, nodename, api.GetNodeLogsParams{
		Paths:  naming.Paths{p}.StrSlice(),
		Filter: params.Filter,
		Follow: params.Follow,
		Lines:  params.Lines,
	})
}
