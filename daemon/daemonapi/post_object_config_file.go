package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
)

func (a *DaemonAPI) PostObjectConfigFile(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request path", "%s", err)
	}
	if len(instance.MonitorData.GetByPath(p)) > 0 {
		return JSONProblemf(ctx, http.StatusConflict, "Conflict", "Use the PUT method instead of POST to update the object config")
	}
	return a.writeObjectConfigFile(ctx, p)
}
