package daemonapi

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
)

// PostObjectConfigValidate validates a draft of the configuration of an
// object, the request body, and writes nothing: the answer is the alerts of
// every check a write of it runs, as a dry run of that write answers them.
//
// The write is the one the draft would be: a replace, PUT, for an object
// that exists, its keywords judged by what the draft changes of the
// configuration, and a create, POST, for one that does not, every keyword
// answering for itself. So the caller is one allowed to make that write: an
// administrator of the namespace, who may create objects there.
func (a *DaemonAPI) PostObjectConfigValidate(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
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
	base, err := readConfigBase(p)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read config", "%s", err)
	}
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read body", "%s", err)
	}
	if err := parseDraft(p, body); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad config", "%s", err)
	}
	// An object exists from the moment its configuration file is written,
	// before the daemon knows it, as a create judges it.
	exists := len(instance.MonitorData.GetByPath(p)) > 0 || base.exists()
	if !exists {
		// The configuration a create writes is the draft without what it
		// recorded of the object it was copied from.
		if _, body, err = object.CreatedConfig(p, body, false); err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad config", "%s", err)
		}
	}
	check := a.newConfigCheck(ctx, p, true)
	check.rbac(body)
	if exists {
		check.stopToChange(body, false)
	}
	return a.writeObjectConfigFile(ctx, check, body, base, nil, false, false, false)
}

// parseDraft refuses a configuration that does not parse, which no check can
// judge.
func parseDraft(p naming.Path, body []byte) error {
	_, err := object.New(p, object.WithConfigData(body), object.WithVolatile(true))
	return err
}
