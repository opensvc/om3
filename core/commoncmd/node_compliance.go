package commoncmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/compliance"
)

type (
	// ComplianceRunOptions are the options of a run of the compliance
	// modules of a node.
	ComplianceRunOptions struct {
		Modulesets string
		Modules    string
		Force      bool
		Attach     bool
	}

	// ComplianceRuns collects the runs of a compliance action, on this
	// node or on the nodes selected, for the exit code of the command.
	ComplianceRuns struct {
		mu    sync.Mutex
		codes []int
	}

	// complianceExitError ends a command whose run is not all compliant
	// with the exit code a module would end with.
	complianceExitError struct {
		code int
	}

	// apiResponse is what a response of the generated client tells of
	// its status.
	apiResponse interface {
		StatusCode() int
		Status() string
	}
)

// Error implements the error interface.
func (t complianceExitError) Error() string {
	if t.code == compliance.ExitCodeNA {
		return "not applicable"
	}
	return "not compliant"
}

// ExitCode is the exit code the command exits with.
func (t complianceExitError) ExitCode() int {
	return t.code
}

// Add adds the run to the ones the exit code of the command reflects.
func (t *ComplianceRuns) Add(run *compliance.Run) {
	if run == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.codes = append(t.codes, run.ExitCode())
}

// Err returns the error the command ends with when a run is not compliant,
// exit code 1, or when every run is not applicable to its node, exit code 2:
// the contract of a compliance module, applied to the command running them.
func (t *ComplianceRuns) Err() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	na := 0
	for _, code := range t.codes {
		switch code {
		case compliance.ExitCodeOk:
		case compliance.ExitCodeNA:
			na++
		default:
			return complianceExitError{code: compliance.ExitCodeNok}
		}
	}
	if len(t.codes) > 0 && na == len(t.codes) {
		return complianceExitError{code: compliance.ExitCodeNA}
	}
	return nil
}

// NodeComplianceRunRemote returns the remote function of a compliance run:
// the run of the node, rendered as a run of this node is, and added to runs.
func NodeComplianceRunRemote(action compliance.Action, opts ComplianceRunOptions, runs *ComplianceRuns) func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		run, err := nodeComplianceRun(ctx, c, nodename, action, opts)
		if err != nil {
			return nil, err
		}
		runs.Add(run)
		return run, nil
	}
}

// nodeComplianceRun asks the node for the compliance run, and returns it as a
// run of this node.
func nodeComplianceRun(ctx context.Context, c *client.T, nodename string, action compliance.Action, opts ComplianceRunOptions) (*compliance.Run, error) {
	var (
		resp apiResponse
		body []byte
		data *api.ComplianceRun
	)
	moduleset, module := ptrOrNil(opts.Modulesets), ptrOrNil(opts.Modules)
	force, attach := &opts.Force, &opts.Attach
	switch action {
	case compliance.ActionAuto:
		params := api.PostNodeComplianceActionAutoParams{Moduleset: moduleset, Module: module, Force: force, Attach: attach}
		r, err := c.PostNodeComplianceActionAutoWithResponse(ctx, nodename, &params)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		resp, body, data = r, r.Body, r.JSON200
	case compliance.ActionCheck:
		params := api.PostNodeComplianceActionCheckParams{Moduleset: moduleset, Module: module, Force: force, Attach: attach}
		r, err := c.PostNodeComplianceActionCheckWithResponse(ctx, nodename, &params)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		resp, body, data = r, r.Body, r.JSON200
	case compliance.ActionFix:
		params := api.PostNodeComplianceActionFixParams{Moduleset: moduleset, Module: module, Force: force, Attach: attach}
		r, err := c.PostNodeComplianceActionFixWithResponse(ctx, nodename, &params)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		resp, body, data = r, r.Body, r.JSON200
	case compliance.ActionFixable:
		params := api.PostNodeComplianceActionFixableParams{Moduleset: moduleset, Module: module, Force: force, Attach: attach}
		r, err := c.PostNodeComplianceActionFixableWithResponse(ctx, nodename, &params)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		resp, body, data = r, r.Body, r.JSON200
	default:
		return nil, fmt.Errorf("unexpected compliance action %s", action)
	}
	if data == nil {
		return nil, nodeResponseError(nodename, resp, body)
	}
	return ComplianceRunFromAPI(*data), nil
}

// NodeComplianceEnvRemote returns the remote function of "node compliance
// env".
func NodeComplianceEnvRemote(modulesets, modules string) func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		params := api.GetNodeComplianceEnvParams{Moduleset: ptrOrNil(modulesets), Module: ptrOrNil(modules)}
		resp, err := c.GetNodeComplianceEnvWithResponse(ctx, nodename, &params)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, nodeResponseError(nodename, resp, resp.Body)
		}
		return compliance.Envs(*resp.JSON200), nil
	}
}

// NodeComplianceModulesRemote returns the remote function of "node
// compliance list module": the modules installed on the node.
func NodeComplianceModulesRemote() func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		resp, err := c.GetNodeComplianceModulesWithResponse(ctx, nodename)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, nodeResponseError(nodename, resp, resp.Body)
		}
		return []string(*resp.JSON200), nil
	}
}

// NodeComplianceModulesetsRemote returns the remote function of "node
// compliance list moduleset".
func NodeComplianceModulesetsRemote(filter string) func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		params := api.GetNodeComplianceModulesetsParams{Filter: ptrOrNil(filter)}
		resp, err := c.GetNodeComplianceModulesetsWithResponse(ctx, nodename, &params)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, nodeResponseError(nodename, resp, resp.Body)
		}
		return []string(*resp.JSON200), nil
	}
}

// NodeComplianceRulesetsRemote returns the remote function of "node
// compliance list ruleset".
func NodeComplianceRulesetsRemote(filter string) func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		params := api.GetNodeComplianceRulesetsParams{Filter: ptrOrNil(filter)}
		resp, err := c.GetNodeComplianceRulesetsWithResponse(ctx, nodename, &params)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, nodeResponseError(nodename, resp, resp.Body)
		}
		return []string(*resp.JSON200), nil
	}
}

// NodeComplianceModulesetTreeRemote returns the remote function of "node
// compliance show moduleset".
func NodeComplianceModulesetTreeRemote(modulesets string) func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		params := api.GetNodeComplianceModulesetTreeParams{Moduleset: ptrOrNil(modulesets)}
		resp, err := c.GetNodeComplianceModulesetTreeWithResponse(ctx, nodename, &params)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, nodeResponseError(nodename, resp, resp.Body)
		}
		return ComplianceModulesetTreeFromAPI(*resp.JSON200), nil
	}
}

// NodeComplianceRulesRemote returns the remote function of "node compliance
// show ruleset".
func NodeComplianceRulesRemote() func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		resp, err := c.GetNodeComplianceRulesWithResponse(ctx, nodename)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, nodeResponseError(nodename, resp, resp.Body)
		}
		return ComplianceRulesetsFromAPI(*resp.JSON200), nil
	}
}

// NodeComplianceModulesetAttachmentRemote returns the remote function of
// "node compliance attach moduleset", or of its detach.
func NodeComplianceModulesetAttachmentRemote(moduleset string, attach bool) func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		var (
			resp apiResponse
			body []byte
		)
		if attach {
			params := api.PostNodeComplianceModulesetAttachmentParams{Moduleset: moduleset}
			r, e := c.PostNodeComplianceModulesetAttachmentWithResponse(ctx, nodename, &params)
			if e != nil {
				return nil, fmt.Errorf("node %s: %w", nodename, e)
			}
			resp, body = r, r.Body
		} else {
			params := api.DeleteNodeComplianceModulesetAttachmentParams{Moduleset: moduleset}
			r, e := c.DeleteNodeComplianceModulesetAttachmentWithResponse(ctx, nodename, &params)
			if e != nil {
				return nil, fmt.Errorf("node %s: %w", nodename, e)
			}
			resp, body = r, r.Body
		}
		if resp.StatusCode() != http.StatusNoContent {
			return nil, nodeResponseError(nodename, resp, body)
		}
		return nil, nil
	}
}

// NodeComplianceRulesetAttachmentRemote returns the remote function of
// "node compliance attach ruleset", or of its detach.
func NodeComplianceRulesetAttachmentRemote(ruleset string, attach bool) func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		var (
			resp apiResponse
			body []byte
		)
		if attach {
			params := api.PostNodeComplianceRulesetAttachmentParams{Ruleset: ruleset}
			r, e := c.PostNodeComplianceRulesetAttachmentWithResponse(ctx, nodename, &params)
			if e != nil {
				return nil, fmt.Errorf("node %s: %w", nodename, e)
			}
			resp, body = r, r.Body
		} else {
			params := api.DeleteNodeComplianceRulesetAttachmentParams{Ruleset: ruleset}
			r, e := c.DeleteNodeComplianceRulesetAttachmentWithResponse(ctx, nodename, &params)
			if e != nil {
				return nil, fmt.Errorf("node %s: %w", nodename, e)
			}
			resp, body = r, r.Body
		}
		if resp.StatusCode() != http.StatusNoContent {
			return nil, nodeResponseError(nodename, resp, body)
		}
		return nil, nil
	}
}

// ComplianceRunFromAPI returns the run the api answers as a run of this node
// is, so it renders the same.
func ComplianceRunFromAPI(r api.ComplianceRun) *compliance.Run {
	run := &compliance.Run{
		Modsets:       r.Modulesets,
		Mods:          r.Modules,
		Attach:        r.Attach,
		Force:         r.Force,
		InitAt:        r.StartedAt,
		BeginAt:       r.FetchedAt,
		EndAt:         r.EndedAt,
		ModuleActions: make(compliance.ModuleActions, len(r.ModuleActions)),
	}
	for i, e := range r.ModuleActions {
		ma := &compliance.ModuleAction{
			Action:   compliance.Action(e.Action),
			Module:   e.Module,
			BeginAt:  e.StartedAt,
			EndAt:    e.EndedAt,
			ExitCode: e.ExitCode,
		}
		for _, line := range e.Log {
			if line.Level == api.ComplianceLogEntryLevelErr {
				ma.Log.Err(line.Message)
			} else {
				ma.Log.Out(line.Message)
			}
		}
		run.ModuleActions[i] = ma
	}
	return run
}

// ComplianceModulesetTreeFromAPI returns the tree the api answers as the
// tree of this node is.
func ComplianceModulesetTreeFromAPI(r api.ComplianceModulesetTree) *compliance.ModulesetTree {
	tree := &compliance.ModulesetTree{
		Name:       r.Name,
		Modules:    make([]compliance.ModulesetModule, len(r.Modules)),
		Modulesets: make([]*compliance.ModulesetTree, len(r.Modulesets)),
	}
	for i, mod := range r.Modules {
		tree.Modules[i] = compliance.ModulesetModule{Name: mod.Name, AutoFix: mod.Autofix}
	}
	for i, child := range r.Modulesets {
		tree.Modulesets[i] = ComplianceModulesetTreeFromAPI(child)
	}
	return tree
}

// ComplianceRulesetsFromAPI returns the rulesets the api answers as the
// rulesets of this node are.
func ComplianceRulesetsFromAPI(r api.ComplianceRulesets) compliance.Rulesets {
	rsets := make(compliance.Rulesets, len(r))
	for name, rset := range r {
		vars := make(compliance.Vars, len(rset.Rules))
		for i, rule := range rset.Rules {
			vars[i] = compliance.Var{Name: rule.Name, Value: rule.Value, Class: rule.Class}
		}
		rsets[name] = compliance.Ruleset{Name: rset.Name, Filter: rset.Filter, Vars: vars}
	}
	return rsets
}

// nodeResponseError returns the error of the response of a node, the problem
// it answers when it does.
func nodeResponseError(nodename string, resp apiResponse, body []byte) error {
	return fmt.Errorf("node %s: %w", nodename, apiResponseError(resp, body))
}

// apiResponseError returns the error of a response, the problem it answers
// when it does.
func apiResponseError(resp apiResponse, body []byte) error {
	var p api.Problem
	if err := json.Unmarshal(body, &p); err == nil && (p.Title != "" || p.Detail != "") {
		if p.Status == 0 {
			p.Status = resp.StatusCode()
		}
		return errors.New(p.String())
	}
	return fmt.Errorf("unexpected response: %s", resp.Status())
}

// ptrOrNil returns a pointer to s, or nil when it is empty: a parameter not
// sent, rather than sent empty.
func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
