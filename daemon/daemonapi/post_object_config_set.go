package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonApi) PostObjectConfigSet(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectConfigSetParams) error {
	log := LogHandler(ctx, "PostObjectConfigSet")

	if v, err := assertGrant(ctx, rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot); !v {
		return err
	}

	if params.Kw == nil {
		return nil
	}

	kops := keyop.ParseOps(*params.Kw)
	if len(kops) == 0 {
		return JSONProblemf(ctx, http.StatusBadRequest, "No valid keyword operations", "")
	}

	if !grantsFromContext(ctx).HasGrant(rbac.GrantRoot) {
		// Non-root is not allowed to set dangerous keywords.
		for _, s := range *params.Kw {
			if err := keywordRbac(s); err != nil {
				return JSONProblemf(ctx, http.StatusUnauthorized, "Not allowed keyword (root-only)", "%s", err)
			}
		}
	}

	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	instanceStatusData := instance.StatusData.GetByPath(p)

	if _, ok := instanceStatusData[a.localhost]; ok {
		oc, err := object.NewConfigurer(p)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewConfigurer", "%s", err)
		}
		for _, kop := range kops {
			if err := oc.Config().Set(kop); err != nil {
				return JSONProblemf(ctx, http.StatusInternalServerError, "Set key", "%s: %s", kop, err)
			}
		}
		if alerts, _ := oc.Config().Validate(); alerts.HasError() {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid configuration", "%s", alerts)
		} else if len(alerts) > 0 {
			JSONProblemf(ctx, http.StatusOK, "Configuration warnings", "%s", alerts)
		}
		if err := oc.Config().CommitInvalid(); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Commit", "%s", err)
		}
		return nil
	}

	for nodename, _ := range instance.StatusData.GetByPath(p) {
		c, err := client.New(client.WithURL(nodename))
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PostObjectConfigSetWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
