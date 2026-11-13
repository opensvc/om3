package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/compliance"
	"github.com/opensvc/om3/v3/util/xstrings"
)

// GetNodeComplianceEnv answers the environment the modules are executed
// with. It carries the rules, root only as the other reads of the node.
func (a *DaemonAPI) GetNodeComplianceEnv(ctx echo.Context, nodename api.InPathNodeName, params api.GetNodeComplianceEnvParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeComplianceEnv(ctx.Request().Context(), nodename, &params)
		})
	}
	return localComplianceEnv(ctx, valueOf(params.Moduleset), valueOf(params.Module), newNodeCompliance)
}

// localComplianceEnv answers the environment the modules are executed with,
// of the node or of the object newComp returns the compliance of.
func localComplianceEnv(ctx echo.Context, modulesets, modules string, newComp func(echo.Context) (*compliance.T, error)) error {
	if modulesets != "" && modules != "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "modules and modulesets can't be selected both")
	}
	comp, err := newComp(ctx)
	if comp == nil {
		return err
	}
	run := comp.NewRun()
	run.SetModulesetsExpr(modulesets)
	run.SetModulesExpr(modules)
	envs, err := run.Env()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Compliance env", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.ComplianceEnv(envs))
}

// GetNodeComplianceModules answers the modules installed on the node. No
// collector is asked, none is needed.
func (a *DaemonAPI) GetNodeComplianceModules(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeComplianceModules(ctx.Request().Context(), nodename)
		})
	}
	l, err := compliance.New().ListModuleNames()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "List compliance modules", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.ComplianceNames(nonNilStrings(l)))
}

// GetNodeComplianceModulesets answers the modulesets the collector makes
// available to the node.
func (a *DaemonAPI) GetNodeComplianceModulesets(ctx echo.Context, nodename api.InPathNodeName, params api.GetNodeComplianceModulesetsParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeComplianceModulesets(ctx.Request().Context(), nodename, &params)
		})
	}
	comp, err := newNodeCompliance(ctx)
	if comp == nil {
		return err
	}
	l, err := comp.ListModulesets(valueOf(params.Filter))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "List compliance modulesets", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.ComplianceNames(nonNilStrings(l)))
}

// GetNodeComplianceModulesetTree answers the modulesets attached to the
// node, or the ones selected, with their modules.
func (a *DaemonAPI) GetNodeComplianceModulesetTree(ctx echo.Context, nodename api.InPathNodeName, params api.GetNodeComplianceModulesetTreeParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeComplianceModulesetTree(ctx.Request().Context(), nodename, &params)
		})
	}
	comp, err := newNodeCompliance(ctx)
	if comp == nil {
		return err
	}
	data, err := comp.GetData(xstrings.Split(valueOf(params.Moduleset), ","))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Get compliance data", "%s", err)
	}
	return ctx.JSON(http.StatusOK, complianceModulesetTreeToAPI(data.ModulesetsTree()))
}

// GetNodeComplianceRulesets answers the rulesets the collector makes
// available to the node.
func (a *DaemonAPI) GetNodeComplianceRulesets(ctx echo.Context, nodename api.InPathNodeName, params api.GetNodeComplianceRulesetsParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeComplianceRulesets(ctx.Request().Context(), nodename, &params)
		})
	}
	comp, err := newNodeCompliance(ctx)
	if comp == nil {
		return err
	}
	l, err := comp.ListRulesets(valueOf(params.Filter))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "List compliance rulesets", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.ComplianceNames(nonNilStrings(l)))
}

// GetNodeComplianceRules answers the rulesets exposed to the modules of the
// node, their rules contextualized for the node.
func (a *DaemonAPI) GetNodeComplianceRules(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeComplianceRules(ctx.Request().Context(), nodename)
		})
	}
	return localComplianceRules(ctx, newNodeCompliance)
}

// localComplianceRules answers the rulesets exposed to the modules, of the
// node or of the object newComp returns the compliance of.
func localComplianceRules(ctx echo.Context, newComp func(echo.Context) (*compliance.T, error)) error {
	comp, err := newComp(ctx)
	if comp == nil {
		return err
	}
	rsets, err := comp.GetRulesets()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Get compliance rulesets", "%s", err)
	}
	return ctx.JSON(http.StatusOK, complianceRulesetsToAPI(rsets))
}
