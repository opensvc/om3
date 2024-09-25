package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/key"
)

var (
	priorityKey = key.Parse("priority")
)

func (a *DaemonAPI) PostObjectConfigUpdate(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectConfigUpdateParams) error {
	log := LogHandler(ctx, "PostObjectConfigUpdate")

	if v, err := assertGrant(ctx, rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot); !v {
		return err
	}

	sets := make(keyop.L, 0)
	unsets := make(key.L, 0)
	deletes := make([]string, 0)

	if params.Set != nil {
		sets = keyop.ParseOps(*params.Set)
	}
	if params.Unset != nil {
		unsets = key.ParseStrings(*params.Unset)
	}
	if params.Delete != nil {
		deletes = *params.Delete
	}
	if len(sets)+len(unsets)+len(deletes) == 0 {
		return JSONProblemf(ctx, http.StatusBadRequest, "No valid update requested", "")
	}

	hasGrantRoot := grantsFromContext(ctx).HasGrant(rbac.GrantRoot)
	hasGrantPrioritizer := grantsFromContext(ctx).HasGrant(rbac.GrantPrioritizer)

	if !hasGrantRoot {
		for _, kop := range sets {
			// Dangerous keywords require GrantRoot.
			if err := keyopRbac(kop); err != nil {
				return JSONProblemf(ctx, http.StatusUnauthorized, "Not allowed keyword (root-only)", "%s", err)
			}
			// Priorities have cross-namespaces consequences, so require GrantRoot or a dedicated GrantPrioritizer
			if !hasGrantPrioritizer && (kop.Key == priorityKey) {
				return JSONProblemf(ctx, http.StatusUnauthorized, "Not allowed to set priority (the root or prioritizer grant is required)", "")
			}
		}
	}

	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	instanceConfigData := instance.ConfigData.GetByPath(p)

	if _, ok := instanceConfigData[a.localhost]; ok {
		oc, err := object.NewConfigurer(p)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewConfigurer", "%s", err)
		}
		if err := oc.Config().PrepareUpdate(deletes, unsets, sets); err != nil {
			log.Debugf("PrepareUpdate %s: %s", p, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "Update config", "%s", err)
		}
		alerts, err := oc.Config().Validate()
		if err != nil {
			log.Debugf("Validate %s: %s", p, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "Validate config", "%s", err)
		}
		if alerts.HasError() {
			log.Debugf("Validate has errors %s", p)
			return JSONProblemf(ctx, http.StatusBadRequest, "Validate config", "%s", alerts.StringWithoutMeta())
		}
		log.Infof("committing %s", p)
		if err := oc.Config().CommitInvalid(); err != nil {
			log.Errorf("CommitInvalid %s: %s", p, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "Commit", "%s", err)
		}
		log.Infof("committed %s", p)
		return ctx.NoContent(http.StatusNoContent)
	}

	for nodename := range instanceConfigData {
		c, err := newProxyClient(ctx, nodename)
		if err != nil {
			log.Warnf("new client for %s@%s: %s", p, nodename, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PostObjectConfigUpdateWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			log.Warnf("request proxy %s@%s: %s", p, nodename, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else {
			log.Debugf("request proxy to %s for %s status: %s", nodename, p, resp.Status())
			if len(resp.Body) > 0 {
				return ctx.JSONBlob(resp.StatusCode(), resp.Body)
			} else {
				return ctx.NoContent(resp.StatusCode())
			}
		}
	}

	return nil
}
