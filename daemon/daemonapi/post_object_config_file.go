package daemonapi

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/flagfile"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/key"
)

func (a *DaemonAPI) PostObjectConfigFile(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectConfigFileParams) error {
	log := LogHandler(ctx, "PostObjectConfigFile")

	// A namespace configuration is the namespace itself, so creating one is
	// creating the namespace, whatever admin of it anyone holds.
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
	if len(instance.MonitorData.GetByPath(p)) > 0 {
		return JSONProblemf(ctx, http.StatusConflict, "Conflict", "Use the PUT method instead of POST to update the object config")
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

	// The configuration written is the one the checks judge: without what
	// the configuration it was copied from recorded of itself, unless
	// restored.
	restore := params.Restore != nil && *params.Restore
	var sets []keyop.T
	if params.Set != nil {
		sets = keyop.ParseOps(*params.Set)
	}
	oc, body, err := object.CreatedConfig(p, body, restore, sets...)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad config", "%s", err)
	}

	// Validate RBAC rules for config keys
	if err := configRbac(ctx, p, body); err != nil {
		log.Warnf("%s: rbac: %s", p, err)
		return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "Config validation: %s", err)
	}

	log.Tracef("%s: rbac passed", p)

	// Flag the instance stopped on purpose if the daemon would start it on
	// its own, so it does not do so before the operator asked for anything.
	// The flag is set before the configuration lands, which the daemon acts
	// on, and taken away if it did not land.
	stopped := false
	switch oc.Config().GetString(key.Parse("orchestrate")) {
	case "ha", "start":
		if err := flagfile.Set(p.StoppedFile()); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Flag the instance stopped", "%s", err)
		}
		stopped = true
	}
	waitLocal := params.WaitLocal != nil && *params.WaitLocal
	err = a.writeObjectConfigFile(ctx, p, body, base, params.Wait, waitLocal)
	if stopped && !p.Exists() {
		if err := flagfile.Unset(p.StoppedFile()); err != nil {
			log.Warnf("%s: the configuration was not written, and the stopped flag set for it is kept: %s", p, err)
		}
	}
	return err
}
