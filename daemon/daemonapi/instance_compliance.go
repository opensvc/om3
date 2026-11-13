package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/compliance"
)

// assertInstanceComplianceRun asserts the grants a compliance run of an
// object asks for. A fix, an auto run, which fixes the autofix modules, or a
// run attaching modulesets changes the node: admin of the namespace, as a
// provision. A check or a fixable test executes the modules without changing
// anything, as an instance status runs the drivers: operator.
func assertInstanceComplianceRun(ctx echo.Context, namespace string, action compliance.Action, attach bool) (bool, error) {
	if attach || action == compliance.ActionFix || action == compliance.ActionAuto {
		return assertAdmin(ctx, namespace)
	}
	return assertOperator(ctx, namespace)
}

// PostInstanceComplianceActionAuto runs the compliance auto of the object on
// the node.
func (a *DaemonAPI) PostInstanceComplianceActionAuto(ctx echo.Context, nodename api.InPathNodeName, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.PostInstanceComplianceActionAutoParams) error {
	if v, err := assertInstanceComplianceRun(ctx, namespace, compliance.ActionAuto, valueOf(params.Attach)); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostInstanceComplianceActionAuto(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	return a.localComplianceRun(ctx, compliance.ActionAuto, complianceRunOptions{
		modulesets: valueOf(params.Moduleset),
		modules:    valueOf(params.Module),
		force:      valueOf(params.Force),
		attach:     valueOf(params.Attach),
	}, newObjectCompliance(p))
}

// PostInstanceComplianceActionCheck runs the compliance check of the object on
// the node.
func (a *DaemonAPI) PostInstanceComplianceActionCheck(ctx echo.Context, nodename api.InPathNodeName, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.PostInstanceComplianceActionCheckParams) error {
	if v, err := assertInstanceComplianceRun(ctx, namespace, compliance.ActionCheck, valueOf(params.Attach)); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostInstanceComplianceActionCheck(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	return a.localComplianceRun(ctx, compliance.ActionCheck, complianceRunOptions{
		modulesets: valueOf(params.Moduleset),
		modules:    valueOf(params.Module),
		force:      valueOf(params.Force),
		attach:     valueOf(params.Attach),
	}, newObjectCompliance(p))
}

// PostInstanceComplianceActionFix runs the compliance fix of the object on
// the node.
func (a *DaemonAPI) PostInstanceComplianceActionFix(ctx echo.Context, nodename api.InPathNodeName, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.PostInstanceComplianceActionFixParams) error {
	if v, err := assertInstanceComplianceRun(ctx, namespace, compliance.ActionFix, valueOf(params.Attach)); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostInstanceComplianceActionFix(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	return a.localComplianceRun(ctx, compliance.ActionFix, complianceRunOptions{
		modulesets: valueOf(params.Moduleset),
		modules:    valueOf(params.Module),
		force:      valueOf(params.Force),
		attach:     valueOf(params.Attach),
	}, newObjectCompliance(p))
}

// PostInstanceComplianceActionFixable runs the compliance fixable of the object on
// the node.
func (a *DaemonAPI) PostInstanceComplianceActionFixable(ctx echo.Context, nodename api.InPathNodeName, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.PostInstanceComplianceActionFixableParams) error {
	if v, err := assertInstanceComplianceRun(ctx, namespace, compliance.ActionFixable, valueOf(params.Attach)); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostInstanceComplianceActionFixable(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	return a.localComplianceRun(ctx, compliance.ActionFixable, complianceRunOptions{
		modulesets: valueOf(params.Moduleset),
		modules:    valueOf(params.Module),
		force:      valueOf(params.Force),
		attach:     valueOf(params.Attach),
	}, newObjectCompliance(p))
}

// GetInstanceComplianceEnv answers the environment the modules of the object
// are executed with on the node: a read, as the resource info of an instance.
func (a *DaemonAPI) GetInstanceComplianceEnv(ctx echo.Context, nodename api.InPathNodeName, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.GetInstanceComplianceEnvParams) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetInstanceComplianceEnv(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	return localComplianceEnv(ctx, valueOf(params.Moduleset), valueOf(params.Module), newObjectCompliance(p))
}

// GetInstanceComplianceRules answers the rulesets exposed to the modules of
// the object on the node: a read, as the resource info of an instance.
func (a *DaemonAPI) GetInstanceComplianceRules(ctx echo.Context, nodename api.InPathNodeName, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetInstanceComplianceRules(ctx.Request().Context(), nodename, namespace, kind, name)
		})
	}
	return localComplianceRules(ctx, newObjectCompliance(p))
}
