package compliance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/httphelper"
	"github.com/opensvc/om3/v3/util/requestfactory"
)

func newCollectorT(t *testing.T, h http.HandlerFunc) *T {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	comp := New()
	comp.SetCollectorClient(httphelper.New(server.Client(), requestfactory.New(u, http.Header{})))
	return comp
}

func TestGetData(t *testing.T) {
	var got *http.Request
	comp := newCollectorT(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		_, _ = w.Write([]byte(`{
			"modulesets": {"m1": [["mod1", true]]},
			"rulesets": {"r1": {"name": "r1", "filter": "f", "vars": [["a", "b", "raw"], ["nodes.os", "linux"]]},
			             "osvc_collector": {"name": "osvc_collector", "filter": "", "vars": [["ruleset_md5", "abc"]]}},
			"modset_relations": {"m1": ["m2"]}
		}`))
	})
	data, err := comp.GetData([]string{"m1", "m2"})
	require.NoError(t, err)
	assert.Equal(t, "/api/compliance/agent/data", got.URL.Path)
	assert.Equal(t, []string{"m1", "m2"}, got.URL.Query()["moduleset"])
	assert.False(t, got.URL.Query().Has("path"))
	assert.Equal(t, "mod1", data.Modsets["m1"][0].Name)
	assert.True(t, data.Modsets["m1"][0].AutoFix)
	assert.Equal(t, "abc", data.RulesetsMD5())
	assert.Equal(t, "raw", data.Rsets["r1"].Vars[1].Class)
}

func TestAttachmentOfObject(t *testing.T) {
	var got *http.Request
	comp := newCollectorT(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		_, _ = w.Write([]byte(`{"info": "done"}`))
	})
	p, err := naming.ParsePath("ns1/svc/s1")
	require.NoError(t, err)
	comp.SetObjectPath(p)
	require.NoError(t, comp.AttachModuleset("my set"))
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/api/compliance/agent/modulesets/my set", got.URL.Path)
	assert.Equal(t, "ns1/svc/s1", got.URL.Query().Get("path"))
	require.NoError(t, comp.DetachRuleset("r1"))
	assert.Equal(t, http.MethodDelete, got.Method)
	assert.Equal(t, "/api/compliance/agent/rulesets/r1", got.URL.Path)
}

func TestCollectorProblem(t *testing.T) {
	comp := newCollectorT(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"text": "moduleset m is not attachable"})
	})
	require.ErrorContains(t, comp.AttachModuleset("m"), "[403] moduleset m is not attachable")
}

func TestPush(t *testing.T) {
	var (
		got  *http.Request
		body []map[string]any
	)
	comp := newCollectorT(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusAccepted)
	})
	run := comp.NewRun()
	run.data = Data{Rsets: Rulesets{"osvc_collector": {Vars: Vars{{Name: "ruleset_md5", Value: "abc"}}}}}
	run.ModuleActions = append(run.ModuleActions, &ModuleAction{Module: "mod1", Action: ActionCheck, ExitCode: 2})
	require.NoError(t, run.Push())
	assert.Equal(t, "/api/compliance/agent/logs", got.URL.Path)
	require.Len(t, body, 1)
	assert.Equal(t, "mod1", body[0]["module"])
	assert.EqualValues(t, 2, body[0]["status"])
	assert.Equal(t, "check", body[0]["action"])
	assert.Equal(t, "abc", body[0]["rset_md5"])
}

func TestExpandModulesOfModuleset(t *testing.T) {
	data := Data{
		Modsets: Modulesets{
			"parent": {{Name: "mod1", AutoFix: true}},
			"child":  {{Name: "mod2"}},
		},
		ModsetRelations: ModulesetRelations{"parent": {"child"}},
	}
	mods := data.ExpandModules([]string{"parent"}, nil)
	byName := make(map[string]*Module)
	for _, mod := range mods {
		byName[mod.Name()] = mod
	}
	require.Len(t, byName, 2)
	assert.True(t, byName["mod1"].autofix)
	assert.Equal(t, "child", byName["mod2"].modset)
}
