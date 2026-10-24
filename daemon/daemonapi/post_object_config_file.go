package daemonapi

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

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
	// Read before anything is checked against it: the write lands only
	// over this file.
	base, err := readConfigBase(p)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read config", "%s", err)
	}

	// Forced, the configuration of an object that exists is replaced as a
	// creation writes it, with the checks of an update. An object exists
	// from the moment its configuration file is written, before the daemon
	// knows it: two creates racing, the second is not one.
	force := params.Force != nil && *params.Force
	exists := len(instance.MonitorData.GetByPath(p)) > 0 || base.exists()
	if exists && !force {
		return JSONProblemf(ctx, http.StatusConflict, "Conflict", "Use the PUT method instead of POST to update the object config")
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
	var src *storeSource
	if params.Store != nil || params.Snapshot != nil {
		if len(body) > 0 {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad request", "a create from a store takes the configuration of the snapshot, and no body")
		}
		var status int
		if src, status, err = openStoreSource(ctx, p, params.Store, params.Snapshot); err != nil {
			return JSONProblemf(ctx, status, http.StatusText(status), "%s", err)
		}
		defer src.close()
		body = src.config
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

	// The keys the configuration references, and the snapshot its
	// provision restores from, come once the configuration passed: the
	// requester may read the snapshot, and the keys land with the
	// configuration in a namespace it is an administrator of.
	if src != nil {
		if oc, body, base, err = src.apply(ctx, p, body, base); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Create from store", "%s", err)
		}
	}

	if exists {
		if err := stopToChange(ctx, p, body, force); err != nil {
			return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "%s", err)
		}
	}

	// Flag the instance stopped on purpose if the daemon would start it on
	// its own, so it does not do so before the operator asked for anything.
	var flagStopped bool
	switch oc.Config().GetString(key.Parse("orchestrate")) {
	case "ha", "start":
		flagStopped = true
	}
	waitLocal := params.WaitLocal != nil && *params.WaitLocal
	return a.writeObjectConfigFile(ctx, p, body, base, params.Wait, waitLocal, force, flagStopped)
}
