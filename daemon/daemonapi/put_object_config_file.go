package daemonapi

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PutObjectConfigFile(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PutObjectConfigFileParams) error {
	if kind == naming.KindNscfg {
		if v, err := assertNamespaceConfigWriter(ctx); !v {
			return err
		}
	} else if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request path", "%s", err)
	}
	if len(instance.MonitorData.GetByPath(p)) == 0 {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "Use the POST method instead of PUT to create the object")
	}

	// Read before anything is checked against it: the write lands only
	// over this file.
	base, err := readConfigBase(p)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read config", "%s", err)
	}

	// Read and parse the config body to validate RBAC rules
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read body", "%s", err)
	}

	force := params.Force != nil && *params.Force

	// A dry run runs every check, and answers what they found. A
	// configuration that does not parse is not one to check.
	dryRun := params.DryRun != nil && *params.DryRun
	if dryRun {
		if err := parseDraft(p, body); err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad config", "%s", err)
		}
	}
	check := a.newConfigCheck(ctx, p, dryRun)

	// Validate RBAC rules for config keys
	if !check.rbac(body) {
		return check.answer()
	}
	if !check.stopToChange(body, force) {
		return check.answer()
	}
	return a.writeObjectConfigFile(ctx, check, body, base, params.Wait, false, force, false)
}
