package daemonapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/hostname"
)

// knownObject makes the object at p one the cluster knows, as a node holding
// its configuration publishes it.
func knownObject(t *testing.T, p naming.Path) {
	t.Helper()
	instance.ConfigData.Set(p, "node1", &instance.Config{})
	t.Cleanup(func() { instance.ConfigData.Unset(p, "node1") })
}

// The object endpoints of the collector attachments ask for the grants of
// the instance actions: an attachment changes what the object runs, admin of
// the namespace; a read is guest. The tags of the collector belong to no
// namespace: a cluster-wide guest lists them, a cluster-wide admin creates
// one.
func TestObjectCollectorEndpointsGrants(t *testing.T) {
	// The cluster tags allowed are asked of a collector: the fake one,
	// not the one of the host running the tests.
	setupNodeWithCollector(t)
	a := &DaemonAPI{localhost: "node1"}
	type call func(echo.Context) error
	calls := map[string]call{
		"attach moduleset": func(c echo.Context) error {
			return a.PostObjectComplianceModulesetAttachment(c, "ns1", naming.KindSvc, "absent", api.PostObjectComplianceModulesetAttachmentParams{Moduleset: "ms1"})
		},
		"detach moduleset": func(c echo.Context) error {
			return a.DeleteObjectComplianceModulesetAttachment(c, "ns1", naming.KindSvc, "absent", api.DeleteObjectComplianceModulesetAttachmentParams{Moduleset: "ms1"})
		},
		"attach ruleset": func(c echo.Context) error {
			return a.PostObjectComplianceRulesetAttachment(c, "ns1", naming.KindSvc, "absent", api.PostObjectComplianceRulesetAttachmentParams{Ruleset: "r1"})
		},
		"detach ruleset": func(c echo.Context) error {
			return a.DeleteObjectComplianceRulesetAttachment(c, "ns1", naming.KindSvc, "absent", api.DeleteObjectComplianceRulesetAttachmentParams{Ruleset: "r1"})
		},
		"attach tag": func(c echo.Context) error {
			return a.PostObjectCollectorTagAttachment(c, "ns1", naming.KindSvc, "absent", "t1")
		},
		"detach tag": func(c echo.Context) error {
			return a.DeleteObjectCollectorTagAttachment(c, "ns1", naming.KindSvc, "absent", "t1")
		},
		"show tags": func(c echo.Context) error {
			return a.GetObjectCollectorTagAttachments(c, "ns1", naming.KindSvc, "absent")
		},
		"list modules": func(c echo.Context) error {
			return a.GetObjectComplianceModules(c, "ns1", naming.KindSvc, "absent")
		},
		"list modulesets": func(c echo.Context) error {
			return a.GetObjectComplianceModulesets(c, "ns1", naming.KindSvc, "absent", api.GetObjectComplianceModulesetsParams{})
		},
		"list rulesets": func(c echo.Context) error {
			return a.GetObjectComplianceRulesets(c, "ns1", naming.KindSvc, "absent", api.GetObjectComplianceRulesetsParams{})
		},
		"show modulesets": func(c echo.Context) error {
			return a.GetObjectComplianceModulesetTree(c, "ns1", naming.KindSvc, "absent", api.GetObjectComplianceModulesetTreeParams{})
		},
		"list cluster tags": func(c echo.Context) error {
			return a.GetClusterCollectorTags(c)
		},
	}
	guest := rbac.Grants{rbac.NewGrant(rbac.RoleGuest, "ns1")}
	operator := rbac.Grants{rbac.NewGrant(rbac.RoleOperator, "ns1")}
	admin := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "ns1")}
	other := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "ns2")}
	clusterGuest := rbac.Grants{rbac.NewGrant(rbac.RoleGuest, "")}
	for _, tc := range []struct {
		call    string
		allowed []rbac.Grants
		denied  []rbac.Grants
	}{
		{"attach moduleset", []rbac.Grants{admin}, []rbac.Grants{operator, guest, other}},
		{"detach moduleset", []rbac.Grants{admin}, []rbac.Grants{operator, guest, other}},
		{"attach ruleset", []rbac.Grants{admin}, []rbac.Grants{operator, guest, other}},
		{"detach ruleset", []rbac.Grants{admin}, []rbac.Grants{operator, guest, other}},
		{"attach tag", []rbac.Grants{admin}, []rbac.Grants{operator, guest, other}},
		{"detach tag", []rbac.Grants{admin}, []rbac.Grants{operator, guest, other}},
		{"show tags", []rbac.Grants{admin, operator, guest}, []rbac.Grants{other}},
		{"list modules", []rbac.Grants{admin, operator, guest}, []rbac.Grants{other}},
		{"list modulesets", []rbac.Grants{admin, operator, guest}, []rbac.Grants{other}},
		{"list rulesets", []rbac.Grants{admin, operator, guest}, []rbac.Grants{other}},
		{"show modulesets", []rbac.Grants{admin, operator, guest}, []rbac.Grants{other}},
		{"list cluster tags", []rbac.Grants{clusterGuest}, []rbac.Grants{guest, admin}},
	} {
		t.Run(tc.call, func(t *testing.T) {
			for _, grants := range tc.denied {
				ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, grants)
				require.NoError(t, calls[tc.call](ctx))
				assert.Equal(t, http.StatusForbidden, rec.Code, "grants %v", grants)
			}
			for _, grants := range tc.allowed {
				// The object is unknown: past the grants, a 404
				// answers, and nothing is done.
				ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, grants)
				require.NoError(t, calls[tc.call](ctx))
				assert.NotEqual(t, http.StatusForbidden, rec.Code, "grants %v", grants)
			}
		})
	}
	t.Run("create cluster tag", func(t *testing.T) {
		for _, grants := range []rbac.Grants{admin, clusterGuest, {rbac.NewGrant(rbac.RoleOperator, "")}} {
			ctx, rec := newNodeTestContext(http.MethodPost, "/", strings.NewReader(`{"name": "t1"}`), grants)
			require.NoError(t, a.PostClusterCollectorTag(ctx))
			assert.Equal(t, http.StatusForbidden, rec.Code, "grants %v", grants)
		}
	})
}

// An object the cluster does not know is answered a 404, not asked of the
// collector.
func TestObjectComplianceOfAnUnknownObject(t *testing.T) {
	fc := setupNodeWithCollector(t)
	a := &DaemonAPI{localhost: hostname.Hostname()}
	ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostObjectComplianceModulesetAttachment(ctx, "ns1", naming.KindSvc, "absent", api.PostObjectComplianceModulesetAttachmentParams{Moduleset: "ms1"}))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Empty(t, fc.requests())
}

// The attachments of an object are asked of the collector for the object,
// by the node serving the request, which holds no instance of it.
func TestObjectComplianceAttachment(t *testing.T) {
	fc := setupNodeWithCollector(t)
	p := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "s1"}
	knownObject(t, p)
	a := &DaemonAPI{localhost: hostname.Hostname()}

	ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostObjectComplianceModulesetAttachment(ctx, "ns1", naming.KindSvc, "s1", api.PostObjectComplianceModulesetAttachmentParams{Moduleset: "ms1"}))
	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	ctx, rec = newNodeTestContext(http.MethodGet, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.GetObjectComplianceModulesetTree(ctx, "ns1", naming.KindSvc, "s1", api.GetObjectComplianceModulesetTreeParams{}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, []string{"POST /api/agent/compliance/modulesets/ms1", "GET /api/agent/compliance/data"}, fc.requests())
	assert.Equal(t, []string{"ns1/svc/s1", "ns1/svc/s1"}, fc.objectQueries())
}

func TestObjectCollectorTag(t *testing.T) {
	fc := setupNodeWithCollector(t)
	p := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "s1"}
	knownObject(t, p)
	a := &DaemonAPI{localhost: hostname.Hostname()}

	ctx, rec := newNodeTestContext(http.MethodGet, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.GetObjectCollectorTagAttachments(ctx, "ns1", naming.KindSvc, "s1"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	ctx, rec = newNodeTestContext(http.MethodPost, "/", strings.NewReader(`{"attach_data": "x"}`), rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostObjectCollectorTagAttachment(ctx, "ns1", naming.KindSvc, "s1", "t1"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	ctx, rec = newNodeTestContext(http.MethodDelete, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.DeleteObjectCollectorTagAttachment(ctx, "ns1", naming.KindSvc, "s1", "t1"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, []string{"GET /api/agent/tags/attached", "POST /api/agent/tags/t1", "DELETE /api/agent/tags/t1"}, fc.requests())
	assert.Equal(t, []string{"ns1/svc/s1", "ns1/svc/s1", "ns1/svc/s1"}, fc.objectQueries())
}

func TestClusterCollectorTags(t *testing.T) {
	fc := setupNodeWithCollector(t)
	a := &DaemonAPI{localhost: hostname.Hostname()}

	ctx, rec := newNodeTestContext(http.MethodGet, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.GetClusterCollectorTags(ctx))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var names api.CollectorTagNames
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &names))
	assert.Equal(t, api.CollectorTagNames{"t1", "t2"}, names)

	ctx, rec = newNodeTestContext(http.MethodPost, "/", strings.NewReader(`{"name": "t3", "data": "d"}`), rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostClusterCollectorTag(ctx))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	ctx, rec = newNodeTestContext(http.MethodPost, "/", strings.NewReader(`{}`), rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostClusterCollectorTag(ctx))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	assert.Equal(t, []string{"GET /api/agent/tags", "POST /api/agent/tags"}, fc.requests())
}
