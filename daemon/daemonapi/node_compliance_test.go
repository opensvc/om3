package daemonapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/compliance"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

// newNodeTestContext returns a context of a request of a user with the
// grants, and the recorder of its answer.
func newNodeTestContext(method, target string, body io.Reader, grants rbac.Grants) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	ctx := echo.New().NewContext(req, rec)
	ctx.Set("grants", grants)
	ctx.Set("logger", plog.NewDefaultLogger())
	return ctx, rec
}

// fakeCollector serves the routes of the collector the node compliance and
// the node tags call, and records the requests.
type fakeCollector struct {
	mu    sync.Mutex
	reqs  []string
	paths []string
}

func (t *fakeCollector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	t.reqs = append(t.reqs, r.Method+" "+r.URL.Path)
	if p := r.URL.Query().Get("path"); p != "" {
		t.paths = append(t.paths, p)
	}
	t.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.URL.Path == "/api/agent/compliance/data":
		_, _ = w.Write([]byte(`{
			"modulesets": {"ms1": [["motd", false]]},
			"rulesets": {"r1": {"name": "r1", "filter": "explicit attachment", "vars": [["motd", "hello", "raw"]]}},
			"modset_relations": {}
		}`))
	case r.URL.Path == "/api/agent/compliance/logs":
		w.WriteHeader(http.StatusAccepted)
	case strings.HasPrefix(r.URL.Path, "/api/agent/compliance/modulesets/"):
		_, _ = w.Write([]byte(`{"info": "done"}`))
	case r.URL.Path == "/api/agent/tags" && r.Method == http.MethodGet:
		_, _ = w.Write([]byte(`["t1", "t2"]`))
	case r.URL.Path == "/api/agent/tags":
		_, _ = w.Write([]byte(`{"info": "created"}`))
	case r.URL.Path == "/api/agent/tags/attached":
		_, _ = w.Write([]byte(`[{"tag_name": "t1", "tag_data": "d1", "tag_attach_data": "a1"}]`))
	case strings.HasPrefix(r.URL.Path, "/api/agent/tags/"):
		b, _ := io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]string{"info": r.Method + " " + string(b)})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// objectQueries returns the objects the requests were for, by their path
// query parameter.
func (t *fakeCollector) objectQueries() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string{}, t.paths...)
}

func (t *fakeCollector) requests() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string{}, t.reqs...)
}

// setupNodeWithCollector sets up a node registered on a fake collector.
func setupNodeWithCollector(t *testing.T) *fakeCollector {
	t.Helper()
	testhelper.Setup(t)
	fc := &fakeCollector{}
	srv := httptest.NewServer(fc)
	t.Cleanup(srv.Close)
	cf := "[node]\nuuid = nodesecret\n\n[collector]\nserver = " + srv.URL + "\n"
	require.NoError(t, os.MkdirAll(filepath.Dir(rawconfig.NodeConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(cf), 0o600))
	return fc
}

// The compliance, the checks and the collector tags of a node are root only,
// as the other reads and actions of the node: a run executes modules as root
// and can change the node, an attachment changes what it runs.
func TestNodeComplianceEndpointsAreRootOnly(t *testing.T) {
	a := &DaemonAPI{localhost: "node1"}
	for name, fn := range map[string]func(echo.Context) error{
		"PostNodeComplianceActionAuto": func(c echo.Context) error {
			return a.PostNodeComplianceActionAuto(c, "node1", api.PostNodeComplianceActionAutoParams{})
		},
		"PostNodeComplianceActionCheck": func(c echo.Context) error {
			return a.PostNodeComplianceActionCheck(c, "node1", api.PostNodeComplianceActionCheckParams{})
		},
		"PostNodeComplianceActionFix": func(c echo.Context) error {
			return a.PostNodeComplianceActionFix(c, "node1", api.PostNodeComplianceActionFixParams{})
		},
		"PostNodeComplianceActionFixable": func(c echo.Context) error {
			return a.PostNodeComplianceActionFixable(c, "node1", api.PostNodeComplianceActionFixableParams{})
		},
		"GetNodeComplianceEnv": func(c echo.Context) error {
			return a.GetNodeComplianceEnv(c, "node1", api.GetNodeComplianceEnvParams{})
		},
		"GetNodeComplianceModules": func(c echo.Context) error {
			return a.GetNodeComplianceModules(c, "node1")
		},
		"GetNodeComplianceModulesets": func(c echo.Context) error {
			return a.GetNodeComplianceModulesets(c, "node1", api.GetNodeComplianceModulesetsParams{})
		},
		"GetNodeComplianceModulesetTree": func(c echo.Context) error {
			return a.GetNodeComplianceModulesetTree(c, "node1", api.GetNodeComplianceModulesetTreeParams{})
		},
		"GetNodeComplianceRulesets": func(c echo.Context) error {
			return a.GetNodeComplianceRulesets(c, "node1", api.GetNodeComplianceRulesetsParams{})
		},
		"GetNodeComplianceRules": func(c echo.Context) error {
			return a.GetNodeComplianceRules(c, "node1")
		},
		"PostNodeComplianceModulesetAttachment": func(c echo.Context) error {
			return a.PostNodeComplianceModulesetAttachment(c, "node1", api.PostNodeComplianceModulesetAttachmentParams{Moduleset: "ms1"})
		},
		"DeleteNodeComplianceModulesetAttachment": func(c echo.Context) error {
			return a.DeleteNodeComplianceModulesetAttachment(c, "node1", api.DeleteNodeComplianceModulesetAttachmentParams{Moduleset: "ms1"})
		},
		"PostNodeComplianceRulesetAttachment": func(c echo.Context) error {
			return a.PostNodeComplianceRulesetAttachment(c, "node1", api.PostNodeComplianceRulesetAttachmentParams{Ruleset: "r1"})
		},
		"DeleteNodeComplianceRulesetAttachment": func(c echo.Context) error {
			return a.DeleteNodeComplianceRulesetAttachment(c, "node1", api.DeleteNodeComplianceRulesetAttachmentParams{Ruleset: "r1"})
		},
		"PostNodeActionChecks": func(c echo.Context) error {
			return a.PostNodeActionChecks(c, "node1")
		},
		"GetNodeCollectorTagAttachments": func(c echo.Context) error {
			return a.GetNodeCollectorTagAttachments(c, "node1")
		},
		"PostNodeCollectorTagAttachment": func(c echo.Context) error {
			return a.PostNodeCollectorTagAttachment(c, "node1", "t1")
		},
		"DeleteNodeCollectorTagAttachment": func(c echo.Context) error {
			return a.DeleteNodeCollectorTagAttachment(c, "node1", "t1")
		},
	} {
		t.Run(name, func(t *testing.T) {
			for _, grants := range []rbac.Grants{
				{},
				{rbac.NewGrant(rbac.RoleAdmin, "")},
				{rbac.NewGrant(rbac.RoleAdmin, "root")},
				{rbac.NewGrant(rbac.RoleOperator, ""), rbac.NewGrant(rbac.RoleGuest, "")},
			} {
				ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, grants)
				require.NoError(t, fn(ctx))
				assert.Equal(t, http.StatusForbidden, rec.Code, "grants %v", grants)
			}
		})
	}
}

// The modules and the modulesets of a run are selected one way or the
// other, as a run on the node refuses both.
func TestPostNodeComplianceActionRefusesModulesAndModulesets(t *testing.T) {
	a := &DaemonAPI{localhost: "node1"}
	moduleset, module := "ms1", "mod1"
	ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostNodeComplianceActionCheck(ctx, "node1", api.PostNodeComplianceActionCheckParams{Moduleset: &moduleset, Module: &module}))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// A run on the node is answered with what "om node compliance check" shows:
// the actions of the modules, their exit code and their log lines.
func TestPostNodeComplianceActionCheck(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("a compliance module is run only when owned by root")
	}
	fc := setupNodeWithCollector(t)
	modDir := filepath.Join(rawconfig.Paths.Var, "compliance")
	require.NoError(t, os.MkdirAll(modDir, 0o755))
	script := "#!/bin/sh\ncase $1 in\ncheck) echo motd checked; echo motd differs >&2; exit 1;;\nfixable) exit 0;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(modDir, "S10motd"), []byte(script), 0o755))

	a := &DaemonAPI{localhost: hostname.Hostname()}
	moduleset := "ms1"
	ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostNodeComplianceActionCheck(ctx, "localhost", api.PostNodeComplianceActionCheckParams{Moduleset: &moduleset}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var run api.ComplianceRun
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &run))
	assert.Equal(t, api.ComplianceRunActionCheck, run.Action)
	assert.Equal(t, []string{"ms1"}, run.Modulesets)
	require.Len(t, run.ModuleActions, 2)
	assert.Equal(t, api.ComplianceModuleActionActionCheck, run.ModuleActions[0].Action)
	assert.Equal(t, "motd", run.ModuleActions[0].Module)
	assert.Equal(t, 1, run.ModuleActions[0].ExitCode)
	assert.ElementsMatch(t, []api.ComplianceLogEntry{
		{Level: api.ComplianceLogEntryLevelOut, Message: "motd checked"},
		{Level: api.ComplianceLogEntryLevelErr, Message: "motd differs"},
	}, run.ModuleActions[0].Log)
	assert.Equal(t, api.ComplianceModuleActionActionFixable, run.ModuleActions[1].Action)
	assert.Equal(t, 0, run.ModuleActions[1].ExitCode)
	assert.False(t, run.EndedAt.Before(run.FetchedAt))
	assert.Contains(t, fc.requests(), "POST /api/agent/compliance/logs", "the run is pushed to the collector")
}

func TestNodeComplianceAttachment(t *testing.T) {
	fc := setupNodeWithCollector(t)
	a := &DaemonAPI{localhost: hostname.Hostname()}

	ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostNodeComplianceModulesetAttachment(ctx, "localhost", api.PostNodeComplianceModulesetAttachmentParams{Moduleset: "ms1"}))
	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	ctx, rec = newNodeTestContext(http.MethodDelete, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.DeleteNodeComplianceModulesetAttachment(ctx, "localhost", api.DeleteNodeComplianceModulesetAttachmentParams{Moduleset: "ms1"}))
	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	assert.Equal(t, []string{
		"POST /api/agent/compliance/modulesets/ms1",
		"DELETE /api/agent/compliance/modulesets/ms1",
	}, fc.requests())

	ctx, rec = newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostNodeComplianceModulesetAttachment(ctx, "localhost", api.PostNodeComplianceModulesetAttachmentParams{}))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetNodeComplianceRules(t *testing.T) {
	setupNodeWithCollector(t)
	a := &DaemonAPI{localhost: hostname.Hostname()}
	ctx, rec := newNodeTestContext(http.MethodGet, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.GetNodeComplianceRules(ctx, "localhost"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var rsets api.ComplianceRulesets
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rsets))
	require.Contains(t, rsets, "r1")
	assert.Equal(t, "explicit attachment", rsets["r1"].Filter)
	assert.Equal(t, []api.ComplianceRule{{Name: "motd", Value: "hello", Class: "raw"}}, rsets["r1"].Rules)
}

func TestGetNodeComplianceModulesetTree(t *testing.T) {
	setupNodeWithCollector(t)
	a := &DaemonAPI{localhost: hostname.Hostname()}
	ctx, rec := newNodeTestContext(http.MethodGet, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.GetNodeComplianceModulesetTree(ctx, "localhost", api.GetNodeComplianceModulesetTreeParams{}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var tree api.ComplianceModulesetTree
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tree))
	require.Len(t, tree.Modulesets, 1)
	assert.Equal(t, "ms1", tree.Modulesets[0].Name)
	assert.Equal(t, []api.ComplianceModulesetModule{{Name: "motd"}}, tree.Modulesets[0].Modules)
}

// A node with no collector says so, rather than answer an empty run.
func TestNodeComplianceWithoutCollector(t *testing.T) {
	testhelper.Setup(t)
	a := &DaemonAPI{localhost: hostname.Hostname()}
	ctx, rec := newNodeTestContext(http.MethodPost, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostNodeComplianceActionCheck(ctx, "localhost", api.PostNodeComplianceActionCheckParams{}))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "collector")
}

func TestNodeCollectorTag(t *testing.T) {
	fc := setupNodeWithCollector(t)
	a := &DaemonAPI{localhost: hostname.Hostname()}

	ctx, rec := newNodeTestContext(http.MethodGet, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.GetNodeCollectorTagAttachments(ctx, "localhost"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var l api.CollectorTagAttachments
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &l))
	assert.Equal(t, api.CollectorTagAttachments{{TagName: "t1", TagData: "d1", TagAttachData: "a1"}}, l)

	ctx, rec = newNodeTestContext(http.MethodPost, "/", strings.NewReader(`{"attach_data": "x"}`), rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.PostNodeCollectorTagAttachment(ctx, "localhost", "t1"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var info api.CollectorInfo
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &info))
	assert.Equal(t, `POST {"attach_data":"x"}`, strings.TrimSpace(info.Info))

	ctx, rec = newNodeTestContext(http.MethodDelete, "/", nil, rbac.Grants{rbac.GrantRoot})
	require.NoError(t, a.DeleteNodeCollectorTagAttachment(ctx, "localhost", "t1"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, []string{
		"GET /api/agent/tags/attached",
		"POST /api/agent/tags/t1",
		"DELETE /api/agent/tags/t1",
	}, fc.requests())
}

func TestComplianceRunToAPI(t *testing.T) {
	at := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	ma := &compliance.ModuleAction{Action: compliance.ActionFix, Module: "mod1", BeginAt: at, EndAt: at.Add(time.Second), ExitCode: 0}
	ma.Log.Out("fixed")
	ma.Log.Err("warned")
	run := &compliance.Run{
		Modsets:       []string{"ms1"},
		Force:         true,
		InitAt:        at,
		BeginAt:       at.Add(time.Second),
		EndAt:         at.Add(2 * time.Second),
		ModuleActions: compliance.ModuleActions{ma},
	}
	r := complianceRunToAPI(compliance.ActionFix, run)
	assert.Equal(t, api.ComplianceRunActionFix, r.Action)
	assert.Equal(t, []string{"ms1"}, r.Modulesets)
	assert.Equal(t, []string{}, r.Modules, "an empty selection is an empty list, not null")
	assert.True(t, r.Force)
	assert.Equal(t, at, r.StartedAt)
	assert.Equal(t, at.Add(time.Second), r.FetchedAt)
	assert.Equal(t, at.Add(2*time.Second), r.EndedAt)
	require.Len(t, r.ModuleActions, 1)
	assert.Equal(t, []api.ComplianceLogEntry{
		{Level: api.ComplianceLogEntryLevelOut, Message: "fixed"},
		{Level: api.ComplianceLogEntryLevelErr, Message: "warned"},
	}, r.ModuleActions[0].Log)
}

func TestCheckList(t *testing.T) {
	a := &DaemonAPI{localhost: "node1"}
	rs := check.ResultSet{Data: []check.Result{{DriverGroup: "fs_u", DriverName: "df", Instance: "/", Unit: "%", Value: 42}}}
	l := a.checkList(rs, errors.New("collector unreachable"))
	assert.Equal(t, api.CheckListKindCheckList, l.Kind)
	require.Len(t, l.Items, 1)
	assert.Equal(t, "node1", l.Items[0].Meta.Node)
	assert.Equal(t, api.CheckResult{Type: "fs_u", Driver: "df", Instance: "/", Unit: "%", Value: 42}, l.Items[0].Data)
	require.NotNil(t, l.PushError)
	assert.Equal(t, "collector unreachable", *l.PushError)
	assert.Nil(t, a.checkList(rs, nil).PushError)
}
