package daemonapi

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
)

func (a *DaemonAPI) PostObjectConfigFile(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	log := LogHandler(ctx, "PostObjectConfigFile")

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

	// Read and parse the config body to validate RBAC rules
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read body", "%s", err)
	}

	// Validate RBAC rules for config keys
	if err := configRbac(ctx, p, body); err != nil {
		log.Warnf("%s: rbac: %s", p, err)
		return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "Config validation: %s", err)
	}

	log.Tracef("%s: rbac passed", p)

	return a.writeObjectConfigFile(ctx, p, body)
}
