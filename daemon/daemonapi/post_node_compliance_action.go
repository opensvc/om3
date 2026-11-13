package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/compliance"
)

// PostNodeComplianceActionAuto runs the fixes of the autofix modules, and
// the checks of the others. A fix changes the node: root only, as the other
// actions of the node.
func (a *DaemonAPI) PostNodeComplianceActionAuto(ctx echo.Context, nodename api.InPathNodeName, params api.PostNodeComplianceActionAutoParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localComplianceRun(ctx, compliance.ActionAuto, complianceRunOptions{
			modulesets: valueOf(params.Moduleset),
			modules:    valueOf(params.Module),
			force:      valueOf(params.Force),
			attach:     valueOf(params.Attach),
		}, newNodeCompliance)
	}
	return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeComplianceActionAuto(ctx.Request().Context(), nodename, &params)
	})
}

// PostNodeComplianceActionCheck runs the checks of the modules. A check
// executes the modules as root, and pushes its results to the collector:
// root only, as the other reads and actions of the node.
func (a *DaemonAPI) PostNodeComplianceActionCheck(ctx echo.Context, nodename api.InPathNodeName, params api.PostNodeComplianceActionCheckParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localComplianceRun(ctx, compliance.ActionCheck, complianceRunOptions{
			modulesets: valueOf(params.Moduleset),
			modules:    valueOf(params.Module),
			force:      valueOf(params.Force),
			attach:     valueOf(params.Attach),
		}, newNodeCompliance)
	}
	return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeComplianceActionCheck(ctx.Request().Context(), nodename, &params)
	})
}

// PostNodeComplianceActionFix runs the fixes of the modules.
func (a *DaemonAPI) PostNodeComplianceActionFix(ctx echo.Context, nodename api.InPathNodeName, params api.PostNodeComplianceActionFixParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localComplianceRun(ctx, compliance.ActionFix, complianceRunOptions{
			modulesets: valueOf(params.Moduleset),
			modules:    valueOf(params.Module),
			force:      valueOf(params.Force),
			attach:     valueOf(params.Attach),
		}, newNodeCompliance)
	}
	return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeComplianceActionFix(ctx.Request().Context(), nodename, &params)
	})
}

// PostNodeComplianceActionFixable runs the fixable tests of the modules.
func (a *DaemonAPI) PostNodeComplianceActionFixable(ctx echo.Context, nodename api.InPathNodeName, params api.PostNodeComplianceActionFixableParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localComplianceRun(ctx, compliance.ActionFixable, complianceRunOptions{
			modulesets: valueOf(params.Moduleset),
			modules:    valueOf(params.Module),
			force:      valueOf(params.Force),
			attach:     valueOf(params.Attach),
		}, newNodeCompliance)
	}
	return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeComplianceActionFixable(ctx.Request().Context(), nodename, &params)
	})
}
