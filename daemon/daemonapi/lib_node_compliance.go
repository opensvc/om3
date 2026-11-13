package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/compliance"
)

type (
	// complianceRunOptions are the parameters of a compliance run, which
	// each of the run endpoints declares a type of its own for.
	complianceRunOptions struct {
		modulesets string
		modules    string
		force      bool
		attach     bool
	}
)

// proxyLong is proxy for a request the peer answers when the work it asks
// for ends: a compliance run executes modules, and a collector call lasts
// as long as the collector takes, both longer than a client allows an answer
// by default. The request of the caller bounds it instead.
func (a *DaemonAPI) proxyLong(ctx echo.Context, nodename string, fn func(*client.T) (*http.Response, error)) error {
	return a.proxyWith(ctx, nodename, fn, nil, client.WithTimeout(0))
}

// newNodeCompliance returns the compliance of the node, a client of the
// collector authenticated as the node.
func newNodeCompliance(ctx echo.Context) (*compliance.T, error) {
	n, err := object.NewNode()
	if err != nil {
		return nil, JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	comp, err := n.NewCompliance()
	if err != nil {
		return nil, JSONProblemf(ctx, http.StatusInternalServerError, "New compliance", "%s", err)
	}
	return comp, nil
}

// newObjectCompliance returns a constructor of the compliance of the object
// at p, a client of the collector authenticated as the node, asking for the
// object. The instance must be on this node.
func newObjectCompliance(p naming.Path) func(echo.Context) (*compliance.T, error) {
	return func(ctx echo.Context) (*compliance.T, error) {
		if !p.Exists() {
			return nil, JSONProblemf(ctx, http.StatusNotFound, "No local instance", "%s has no instance on this node", p)
		}
		o, err := object.NewSvc(p, object.WithVolatile(true))
		if err != nil {
			return nil, JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
		}
		comp, err := o.NewCompliance()
		if err != nil {
			return nil, JSONProblemf(ctx, http.StatusInternalServerError, "New compliance", "%s", err)
		}
		return comp, nil
	}
}

// localComplianceRun runs the compliance action on this node, of the node or
// of the object newComp returns the compliance of, and answers the run, as
// "om node compliance <action>" renders it.
func (a *DaemonAPI) localComplianceRun(ctx echo.Context, action compliance.Action, opts complianceRunOptions, newComp func(echo.Context) (*compliance.T, error)) error {
	log := LogHandler(ctx, "localComplianceRun")
	if opts.modulesets != "" && opts.modules != "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "modules and modulesets can't be selected both")
	}
	comp, err := newComp(ctx)
	if comp == nil {
		return err
	}
	run := comp.NewRun()
	run.SetModulesetsExpr(opts.modulesets)
	run.SetModulesExpr(opts.modules)
	run.SetForce(opts.force)
	run.SetAttach(opts.attach)
	log.Infof("compliance %s modulesets=%s modules=%s force=%v attach=%v", action, opts.modulesets, opts.modules, opts.force, opts.attach)
	switch action {
	case compliance.ActionAuto:
		err = run.Auto()
	case compliance.ActionCheck:
		err = run.Check()
	case compliance.ActionFix:
		err = run.Fix()
	case compliance.ActionFixable:
		err = run.Fixable()
	default:
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "unexpected compliance action %s", action)
	}
	if err != nil {
		log.Warnf("compliance %s: %s", action, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Compliance "+string(action), "%s", err)
	}
	return ctx.JSON(http.StatusOK, complianceRunToAPI(action, run))
}

// complianceRunToAPI returns the run as the api answers it.
func complianceRunToAPI(action compliance.Action, run *compliance.Run) api.ComplianceRun {
	r := api.ComplianceRun{
		Action:        api.ComplianceRunAction(action),
		Modulesets:    nonNilStrings(run.Modsets),
		Modules:       nonNilStrings(run.Mods),
		Force:         run.Force,
		Attach:        run.Attach,
		StartedAt:     run.InitAt,
		FetchedAt:     run.BeginAt,
		EndedAt:       run.EndAt,
		ModuleActions: make([]api.ComplianceModuleAction, len(run.ModuleActions)),
	}
	for i, ma := range run.ModuleActions {
		entries := ma.Log.Entries()
		log := make([]api.ComplianceLogEntry, len(entries))
		for j, e := range entries {
			level := api.ComplianceLogEntryLevelOut
			if e.Level == compliance.LogLevelErr {
				level = api.ComplianceLogEntryLevelErr
			}
			log[j] = api.ComplianceLogEntry{Level: level, Message: e.Msg}
		}
		r.ModuleActions[i] = api.ComplianceModuleAction{
			Action:    api.ComplianceModuleActionAction(ma.Action),
			Module:    ma.Module,
			StartedAt: ma.BeginAt,
			EndedAt:   ma.EndAt,
			ExitCode:  ma.ExitCode,
			Log:       log,
		}
	}
	return r
}

// complianceModulesetTreeToAPI returns the tree as the api answers it.
func complianceModulesetTreeToAPI(tree *compliance.ModulesetTree) api.ComplianceModulesetTree {
	r := api.ComplianceModulesetTree{
		Modules:    make([]api.ComplianceModulesetModule, len(tree.Modules)),
		Modulesets: make([]api.ComplianceModulesetTree, len(tree.Modulesets)),
		Name:       tree.Name,
	}
	for i, mod := range tree.Modules {
		r.Modules[i] = api.ComplianceModulesetModule{Name: mod.Name, Autofix: mod.AutoFix}
	}
	for i, child := range tree.Modulesets {
		r.Modulesets[i] = complianceModulesetTreeToAPI(child)
	}
	return r
}

// complianceRulesetsToAPI returns the rulesets as the api answers them.
func complianceRulesetsToAPI(rsets compliance.Rulesets) api.ComplianceRulesets {
	r := make(api.ComplianceRulesets, len(rsets))
	for name, rset := range rsets {
		rules := make([]api.ComplianceRule, len(rset.Vars))
		for i, v := range rset.Vars {
			rules[i] = api.ComplianceRule{Name: v.Name, Value: v.Value, Class: v.Class}
		}
		r[name] = api.ComplianceRuleset{Name: rset.Name, Filter: rset.Filter, Rules: rules}
	}
	return r
}

func nonNilStrings(l []string) []string {
	if l == nil {
		return []string{}
	}
	return l
}

func valueOf[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
