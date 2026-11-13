package commoncmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/compliance"
)

// ObjectComplianceRunRemote returns the remote function of a compliance run
// of an object: the run of its instance on the node, rendered as a run on
// this node is, and added to runs.
func ObjectComplianceRunRemote(action compliance.Action, opts ComplianceRunOptions, runs *ComplianceRuns) func(context.Context, naming.Path, string) (any, error) {
	return func(ctx context.Context, p naming.Path, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		run, err := objectComplianceRun(ctx, c, p, nodename, action, opts)
		if err != nil {
			return nil, err
		}
		runs.Add(run)
		return run, nil
	}
}

// objectComplianceRun asks the node for the compliance run of the object,
// and returns it as a run on this node.
func objectComplianceRun(ctx context.Context, c *client.T, p naming.Path, nodename string, action compliance.Action, opts ComplianceRunOptions) (*compliance.Run, error) {
	var (
		resp apiResponse
		body []byte
		data *api.ComplianceRun
	)
	moduleset, module := ptrOrNil(opts.Modulesets), ptrOrNil(opts.Modules)
	force, attach := &opts.Force, &opts.Attach
	switch action {
	case compliance.ActionAuto:
		params := api.PostInstanceComplianceActionAutoParams{Moduleset: moduleset, Module: module, Force: force, Attach: attach}
		r, err := c.PostInstanceComplianceActionAutoWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return nil, fmt.Errorf("%s: node %s: %w", p, nodename, err)
		}
		resp, body, data = r, r.Body, r.JSON200
	case compliance.ActionCheck:
		params := api.PostInstanceComplianceActionCheckParams{Moduleset: moduleset, Module: module, Force: force, Attach: attach}
		r, err := c.PostInstanceComplianceActionCheckWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return nil, fmt.Errorf("%s: node %s: %w", p, nodename, err)
		}
		resp, body, data = r, r.Body, r.JSON200
	case compliance.ActionFix:
		params := api.PostInstanceComplianceActionFixParams{Moduleset: moduleset, Module: module, Force: force, Attach: attach}
		r, err := c.PostInstanceComplianceActionFixWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return nil, fmt.Errorf("%s: node %s: %w", p, nodename, err)
		}
		resp, body, data = r, r.Body, r.JSON200
	case compliance.ActionFixable:
		params := api.PostInstanceComplianceActionFixableParams{Moduleset: moduleset, Module: module, Force: force, Attach: attach}
		r, err := c.PostInstanceComplianceActionFixableWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return nil, fmt.Errorf("%s: node %s: %w", p, nodename, err)
		}
		resp, body, data = r, r.Body, r.JSON200
	default:
		return nil, fmt.Errorf("unexpected compliance action %s", action)
	}
	if data == nil {
		return nil, fmt.Errorf("%s: %w", p, nodeResponseError(nodename, resp, body))
	}
	return ComplianceRunFromAPI(*data), nil
}

// ObjectComplianceEnvRemote returns the remote function of "<path>
// compliance env".
func ObjectComplianceEnvRemote(modulesets, modules string) func(context.Context, naming.Path, string) (any, error) {
	return func(ctx context.Context, p naming.Path, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		params := api.GetInstanceComplianceEnvParams{Moduleset: ptrOrNil(modulesets), Module: ptrOrNil(modules)}
		resp, err := c.GetInstanceComplianceEnvWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return nil, fmt.Errorf("%s: node %s: %w", p, nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, fmt.Errorf("%s: %w", p, nodeResponseError(nodename, resp, resp.Body))
		}
		return compliance.Envs(*resp.JSON200), nil
	}
}

// ObjectComplianceRulesRemote returns the remote function of "<path>
// compliance show ruleset".
func ObjectComplianceRulesRemote() func(context.Context, naming.Path, string) (any, error) {
	return func(ctx context.Context, p naming.Path, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		resp, err := c.GetInstanceComplianceRulesWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name)
		if err != nil {
			return nil, fmt.Errorf("%s: node %s: %w", p, nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, fmt.Errorf("%s: %w", p, nodeResponseError(nodename, resp, resp.Body))
		}
		return ComplianceRulesetsFromAPI(*resp.JSON200), nil
	}
}

// ObjectComplianceModules returns the work of "<path> compliance list
// module": the modules installed on the node serving the request.
func ObjectComplianceModules() ObjectAPIFunc {
	return func(ctx context.Context, c *client.T, p naming.Path) (any, error) {
		resp, err := c.GetObjectComplianceModulesWithResponse(ctx, p.Namespace, p.Kind, p.Name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if resp.JSON200 == nil {
			return nil, objectResponseError(p, resp, resp.Body)
		}
		return []string(*resp.JSON200), nil
	}
}

// ObjectComplianceModulesets returns the work of "<path> compliance list
// moduleset".
func ObjectComplianceModulesets(filter string) ObjectAPIFunc {
	return func(ctx context.Context, c *client.T, p naming.Path) (any, error) {
		params := api.GetObjectComplianceModulesetsParams{Filter: ptrOrNil(filter)}
		resp, err := c.GetObjectComplianceModulesetsWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if resp.JSON200 == nil {
			return nil, objectResponseError(p, resp, resp.Body)
		}
		return []string(*resp.JSON200), nil
	}
}

// ObjectComplianceRulesets returns the work of "<path> compliance list
// ruleset".
func ObjectComplianceRulesets(filter string) ObjectAPIFunc {
	return func(ctx context.Context, c *client.T, p naming.Path) (any, error) {
		params := api.GetObjectComplianceRulesetsParams{Filter: ptrOrNil(filter)}
		resp, err := c.GetObjectComplianceRulesetsWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if resp.JSON200 == nil {
			return nil, objectResponseError(p, resp, resp.Body)
		}
		return []string(*resp.JSON200), nil
	}
}

// ObjectComplianceModulesetTree returns the work of "<path> compliance show
// moduleset".
func ObjectComplianceModulesetTree(modulesets string) ObjectAPIFunc {
	return func(ctx context.Context, c *client.T, p naming.Path) (any, error) {
		params := api.GetObjectComplianceModulesetTreeParams{Moduleset: ptrOrNil(modulesets)}
		resp, err := c.GetObjectComplianceModulesetTreeWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if resp.JSON200 == nil {
			return nil, objectResponseError(p, resp, resp.Body)
		}
		return ComplianceModulesetTreeFromAPI(*resp.JSON200), nil
	}
}

// ObjectComplianceModulesetAttachment returns the work of "<path> compliance
// attach moduleset", or of its detach.
func ObjectComplianceModulesetAttachment(moduleset string, attach bool) ObjectAPIFunc {
	return func(ctx context.Context, c *client.T, p naming.Path) (any, error) {
		var (
			resp apiResponse
			body []byte
		)
		if attach {
			params := api.PostObjectComplianceModulesetAttachmentParams{Moduleset: moduleset}
			r, err := c.PostObjectComplianceModulesetAttachmentWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", p, err)
			}
			resp, body = r, r.Body
		} else {
			params := api.DeleteObjectComplianceModulesetAttachmentParams{Moduleset: moduleset}
			r, err := c.DeleteObjectComplianceModulesetAttachmentWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", p, err)
			}
			resp, body = r, r.Body
		}
		if resp.StatusCode() != http.StatusNoContent {
			return nil, objectResponseError(p, resp, body)
		}
		return nil, nil
	}
}

// ObjectComplianceRulesetAttachment returns the work of "<path> compliance
// attach ruleset", or of its detach.
func ObjectComplianceRulesetAttachment(ruleset string, attach bool) ObjectAPIFunc {
	return func(ctx context.Context, c *client.T, p naming.Path) (any, error) {
		var (
			resp apiResponse
			body []byte
		)
		if attach {
			params := api.PostObjectComplianceRulesetAttachmentParams{Ruleset: ruleset}
			r, err := c.PostObjectComplianceRulesetAttachmentWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", p, err)
			}
			resp, body = r, r.Body
		} else {
			params := api.DeleteObjectComplianceRulesetAttachmentParams{Ruleset: ruleset}
			r, err := c.DeleteObjectComplianceRulesetAttachmentWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", p, err)
			}
			resp, body = r, r.Body
		}
		if resp.StatusCode() != http.StatusNoContent {
			return nil, objectResponseError(p, resp, body)
		}
		return nil, nil
	}
}

// objectResponseError returns the error of the response of an object
// endpoint, the problem it answers when it does.
func objectResponseError(p naming.Path, resp apiResponse, body []byte) error {
	return fmt.Errorf("%s: %w", p, apiResponseError(resp, body))
}
