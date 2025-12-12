package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/key"
)

var (
	priorityKey = key.Parse("priority")
)

func (a *DaemonAPI) PatchObjectConfig(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PatchObjectConfigParams) error {
	log := LogHandler(ctx, "patchObjectConfig")

	if v, err := assertAdmin(ctx, namespace); !v {
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
		for _, section := range *params.Delete {
			if section == "" {
				// Prevents from accidental remove DEFAULT section. SectionsByName("")
				// return "DEFAULT". Use explicit section="DEFAULT" to remove DEFAULT section.
				continue
			}
			deletes = append(deletes, section)
		}
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
				return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "Keyword operation: %s: %s", kop, err)
			}
			// Priorities have cross-namespaces consequences, so require GrantRoot or a dedicated GrantPrioritizer
			if !hasGrantPrioritizer && (kop.Key == priorityKey) {
				return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "Keyword operation: %s: %s", kop, "setting priority requires the root or prioritizer grant")
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
		changed, err := configUpdate(log, p, deletes, unsets, sets)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Update config", "%s", err)
		}
		return ctx.JSON(http.StatusOK, api.Committed{IsChanged: changed})
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			log.Warnf("new client for %s@%s: %s", p, nodename, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PatchObjectConfigWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			log.Warnf("request proxy %s@%s: %s", p, nodename, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else {
			log.Tracef("request proxy to %s for %s status: %s", nodename, p, resp.Status())
			if len(resp.Body) > 0 {
				return ctx.JSONBlob(resp.StatusCode(), resp.Body)
			} else {
				return ctx.NoContent(resp.StatusCode())
			}
		}
	}

	log.Tracef("can't patch: object not found %s", p)
	return JSONProblemf(ctx, http.StatusNotFound, "Not found", "object not found: %s", p)
}
