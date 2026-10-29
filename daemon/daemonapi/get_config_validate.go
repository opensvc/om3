package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

// GetNodeConfigValidate validates the configuration of the node on the node,
// for root, who reads the configuration file.
func (a *DaemonAPI) GetNodeConfigValidate(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeConfigValidate(ctx.Request().Context(), nodename)
		})
	}
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	alerts, err := n.ValidateConfig()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Validate config", "%s", err)
	}
	return ctx.JSON(http.StatusOK, a.configAlertList(alerts))
}

// GetInstanceConfigValidate validates the configuration of the object on the
// node, as om validate config does there: the keywords, and the checks the
// drivers make loading the resources, on the node. The alerts may quote the
// values of the keywords, the ones declared secret included, so it is the
// administrators' of the namespace and root's, who read them.
func (a *DaemonAPI) GetInstanceConfigValidate(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}
	if redact, ok, err := configReadAccess(ctx, p); !ok {
		return err
	} else if redact {
		return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "the validation of %s may quote its secrets: the administrators of namespace %s and root only", p, p.Namespace)
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetInstanceConfigValidate(ctx.Request().Context(), nodename, namespace, kind, name)
		})
	}
	if !p.Exists() {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "no configuration of %s on %s", p, a.localhost)
	}
	o, err := object.New(p, object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
	}
	c, ok := o.(object.Configurer)
	if !ok {
		return JSONProblemf(ctx, http.StatusBadRequest, "Validate config", "%s is not a configurer", p)
	}
	// A read: the action lock would have a validation wait for an action
	// running, and a write of the configuration is a rename.
	rctx := actioncontext.WithLockDisabled(ctx.Request().Context(), true)
	alerts, err := c.ValidateConfig(rctx)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Validate config", "%s", err)
	}
	if v, ok := o.(interface{ ValidateResources() xconfig.Alerts }); ok {
		alerts = append(alerts, v.ValidateResources()...)
	}
	return ctx.JSON(http.StatusOK, a.configAlertList(alerts))
}

func (a *DaemonAPI) configAlertList(alerts xconfig.Alerts) api.ConfigAlertList {
	l := api.ConfigAlertList{Kind: "ConfigAlertList", Items: make(api.ConfigAlertItems, 0, len(alerts))}
	for _, e := range alerts {
		path, drv := "", ""
		if !e.Path.IsZero() {
			path = e.Path.String()
		}
		if !e.Driver.IsZero() {
			drv = e.Driver.String()
		}
		l.Items = append(l.Items, api.ConfigAlert{
			Node:    a.localhost,
			Path:    path,
			Level:   api.ConfigAlertLevel(e.Level.String()),
			Kind:    e.Kind.String(),
			Key:     e.Key.String(),
			Driver:  drv,
			Comment: e.Comment,
		})
	}
	return l
}
