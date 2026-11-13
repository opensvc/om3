package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/compliance"
	"github.com/opensvc/om3/v3/util/xstrings"
)

// clusterObjectPath returns the path of an object the cluster knows, or
// answers the problem of one it does not.
func clusterObjectPath(ctx echo.Context, namespace string, kind naming.Kind, name string) (naming.Path, error) {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return naming.Path{}, JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	if len(instance.ConfigData.GetByPath(p)) == 0 {
		return naming.Path{}, JSONProblemf(ctx, http.StatusNotFound, "Not found", "object not found: %s", p)
	}
	return p, nil
}

// newObjectPathCompliance returns the compliance of the object at p, a
// client of the collector authenticated as the node serving the request,
// asking for the object. The collector holds the attachments of the object,
// so any node of the cluster serves them.
func newObjectPathCompliance(p naming.Path) func(echo.Context) (*compliance.T, error) {
	return func(ctx echo.Context) (*compliance.T, error) {
		comp, err := newNodeCompliance(ctx)
		if comp == nil {
			return nil, err
		}
		comp.SetObjectPath(p)
		return comp, nil
	}
}

// GetObjectComplianceModules answers the modules installed on the node
// serving the request, which no collector is asked for: a read, guest.
func (a *DaemonAPI) GetObjectComplianceModules(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	if p, err := clusterObjectPath(ctx, namespace, kind, name); p.IsZero() {
		return err
	}
	l, err := compliance.New().ListModuleNames()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "List compliance modules", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.ComplianceNames(nonNilStrings(l)))
}

// GetObjectComplianceModulesets answers the modulesets the collector makes
// available to the object: a read, guest.
func (a *DaemonAPI) GetObjectComplianceModulesets(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.GetObjectComplianceModulesetsParams) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	p, err := clusterObjectPath(ctx, namespace, kind, name)
	if p.IsZero() {
		return err
	}
	comp, err := newObjectPathCompliance(p)(ctx)
	if comp == nil {
		return err
	}
	l, err := comp.ListModulesets(valueOf(params.Filter))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "List compliance modulesets", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.ComplianceNames(nonNilStrings(l)))
}

// GetObjectComplianceRulesets answers the rulesets the collector makes
// available to the object: a read, guest.
func (a *DaemonAPI) GetObjectComplianceRulesets(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.GetObjectComplianceRulesetsParams) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	p, err := clusterObjectPath(ctx, namespace, kind, name)
	if p.IsZero() {
		return err
	}
	comp, err := newObjectPathCompliance(p)(ctx)
	if comp == nil {
		return err
	}
	l, err := comp.ListRulesets(valueOf(params.Filter))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "List compliance rulesets", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.ComplianceNames(nonNilStrings(l)))
}

// GetObjectComplianceModulesetTree answers the modulesets attached to the
// object, or the ones selected, with their modules: a read, guest.
func (a *DaemonAPI) GetObjectComplianceModulesetTree(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.GetObjectComplianceModulesetTreeParams) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	p, err := clusterObjectPath(ctx, namespace, kind, name)
	if p.IsZero() {
		return err
	}
	comp, err := newObjectPathCompliance(p)(ctx)
	if comp == nil {
		return err
	}
	data, err := comp.GetData(xstrings.Split(valueOf(params.Moduleset), ","))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Get compliance data", "%s", err)
	}
	return ctx.JSON(http.StatusOK, complianceModulesetTreeToAPI(data.ModulesetsTree()))
}

// PostObjectComplianceModulesetAttachment attaches a moduleset to the
// object, which changes what its instances run: admin of the namespace.
func (a *DaemonAPI) PostObjectComplianceModulesetAttachment(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.PostObjectComplianceModulesetAttachmentParams) error {
	return a.objectComplianceAttachment(ctx, namespace, kind, name, "Attach moduleset", params.Moduleset, (*compliance.T).AttachModuleset)
}

// DeleteObjectComplianceModulesetAttachment detaches a moduleset from the
// object: admin of the namespace.
func (a *DaemonAPI) DeleteObjectComplianceModulesetAttachment(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.DeleteObjectComplianceModulesetAttachmentParams) error {
	return a.objectComplianceAttachment(ctx, namespace, kind, name, "Detach moduleset", params.Moduleset, (*compliance.T).DetachModuleset)
}

// PostObjectComplianceRulesetAttachment attaches a ruleset to the object,
// which changes the rules its modules apply: admin of the namespace.
func (a *DaemonAPI) PostObjectComplianceRulesetAttachment(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.PostObjectComplianceRulesetAttachmentParams) error {
	return a.objectComplianceAttachment(ctx, namespace, kind, name, "Attach ruleset", params.Ruleset, (*compliance.T).AttachRuleset)
}

// DeleteObjectComplianceRulesetAttachment detaches a ruleset from the
// object: admin of the namespace.
func (a *DaemonAPI) DeleteObjectComplianceRulesetAttachment(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.DeleteObjectComplianceRulesetAttachmentParams) error {
	return a.objectComplianceAttachment(ctx, namespace, kind, name, "Detach ruleset", params.Ruleset, (*compliance.T).DetachRuleset)
}

func (a *DaemonAPI) objectComplianceAttachment(ctx echo.Context, namespace string, kind naming.Kind, name, title, target string, fn func(*compliance.T, string) error) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	p, err := clusterObjectPath(ctx, namespace, kind, name)
	if p.IsZero() {
		return err
	}
	if target == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s: empty name", title)
	}
	comp, err := newObjectPathCompliance(p)(ctx)
	if comp == nil {
		return err
	}
	naming.LogWithPath(LogHandler(ctx, "objectComplianceAttachment"), p).Infof("%s %s", title, target)
	if err := fn(comp, target); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, title, "%s", err)
	}
	return ctx.NoContent(http.StatusNoContent)
}
