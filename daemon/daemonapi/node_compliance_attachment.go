package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/compliance"
)

// PostNodeComplianceModulesetAttachment attaches a moduleset to the node,
// which changes what the node runs: root only, as the other actions of the
// node.
func (a *DaemonAPI) PostNodeComplianceModulesetAttachment(ctx echo.Context, nodename api.InPathNodeName, params api.PostNodeComplianceModulesetAttachmentParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostNodeComplianceModulesetAttachment(ctx.Request().Context(), nodename, &params)
		})
	}
	return a.localComplianceAttachment(ctx, "Attach moduleset", params.Moduleset, (*compliance.T).AttachModuleset)
}

// DeleteNodeComplianceModulesetAttachment detaches a moduleset from the node.
func (a *DaemonAPI) DeleteNodeComplianceModulesetAttachment(ctx echo.Context, nodename api.InPathNodeName, params api.DeleteNodeComplianceModulesetAttachmentParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.DeleteNodeComplianceModulesetAttachment(ctx.Request().Context(), nodename, &params)
		})
	}
	return a.localComplianceAttachment(ctx, "Detach moduleset", params.Moduleset, (*compliance.T).DetachModuleset)
}

// PostNodeComplianceRulesetAttachment attaches a ruleset to the node, which
// changes the rules its modules apply.
func (a *DaemonAPI) PostNodeComplianceRulesetAttachment(ctx echo.Context, nodename api.InPathNodeName, params api.PostNodeComplianceRulesetAttachmentParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostNodeComplianceRulesetAttachment(ctx.Request().Context(), nodename, &params)
		})
	}
	return a.localComplianceAttachment(ctx, "Attach ruleset", params.Ruleset, (*compliance.T).AttachRuleset)
}

// DeleteNodeComplianceRulesetAttachment detaches a ruleset from the node.
func (a *DaemonAPI) DeleteNodeComplianceRulesetAttachment(ctx echo.Context, nodename api.InPathNodeName, params api.DeleteNodeComplianceRulesetAttachmentParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.DeleteNodeComplianceRulesetAttachment(ctx.Request().Context(), nodename, &params)
		})
	}
	return a.localComplianceAttachment(ctx, "Detach ruleset", params.Ruleset, (*compliance.T).DetachRuleset)
}

// localComplianceAttachment changes an attachment of the node on the
// collector, as the node.
func (a *DaemonAPI) localComplianceAttachment(ctx echo.Context, title, name string, fn func(*compliance.T, string) error) error {
	if name == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s: empty name", title)
	}
	comp, err := newNodeCompliance(ctx)
	if comp == nil {
		return err
	}
	LogHandler(ctx, "localComplianceAttachment").Infof("%s %s", title, name)
	if err := fn(comp, name); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, title, "%s", err)
	}
	return ctx.NoContent(http.StatusNoContent)
}
