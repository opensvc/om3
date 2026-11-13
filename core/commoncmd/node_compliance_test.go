package commoncmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/compliance"
)

// apiServer answers the requests of a client with h, as the daemon of a node
// would.
func apiServer(t *testing.T, h http.HandlerFunc) *client.T {
	t.Helper()
	srv := httptest.NewUnstartedServer(h)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	c, err := client.New(client.WithURL(srv.URL), client.WithBearer("x"), client.WithInsecureSkipVerify(true))
	require.NoError(t, err)
	return c
}

func apiComplianceRun() api.ComplianceRun {
	at := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	return api.ComplianceRun{
		Action:     api.ComplianceRunActionCheck,
		Modulesets: []string{"ms1"},
		Modules:    []string{},
		StartedAt:  at,
		FetchedAt:  at.Add(time.Second),
		EndedAt:    at.Add(3 * time.Second),
		ModuleActions: []api.ComplianceModuleAction{
			{
				Action:    api.ComplianceModuleActionActionCheck,
				Module:    "mod1",
				StartedAt: at.Add(time.Second),
				EndedAt:   at.Add(2 * time.Second),
				ExitCode:  1,
				Log: []api.ComplianceLogEntry{
					{Level: api.ComplianceLogEntryLevelOut, Message: "checked the motd"},
					{Level: api.ComplianceLogEntryLevelErr, Message: "the motd is not the one expected"},
				},
			},
			{
				Action:    api.ComplianceModuleActionActionFixable,
				Module:    "mod1",
				StartedAt: at.Add(2 * time.Second),
				EndedAt:   at.Add(3 * time.Second),
				ExitCode:  0,
				Log:       []api.ComplianceLogEntry{},
			},
		},
	}
}

// A run of a node answered by the api renders as the run of this node does:
// the modules, their status and their log lines.
func TestComplianceRunFromAPI(t *testing.T) {
	run := ComplianceRunFromAPI(apiComplianceRun())
	require.Len(t, run.ModuleActions, 2)
	ma := run.ModuleActions[0]
	assert.Equal(t, compliance.ActionCheck, ma.Action)
	assert.Equal(t, "mod1", ma.Module)
	assert.Equal(t, 1, ma.ExitCode)
	assert.Equal(t, time.Second, ma.Duration())
	entries := ma.Log.Entries()
	require.Len(t, entries, 2)
	assert.Equal(t, compliance.LogLevelOut, entries[0].Level)
	assert.Equal(t, compliance.LogLevelErr, entries[1].Level)
	assert.Equal(t, []string{"ms1"}, run.Modsets)

	s := run.Render()
	assert.Contains(t, s, "mod1")
	assert.Contains(t, s, "checked the motd")
	assert.Contains(t, s, "the motd is not the one expected")
	assert.Contains(t, s, "Modules Count:     1")
	assert.Equal(t, compliance.ExitCodeNok, run.ExitCode())
}

func TestComplianceRunsErr(t *testing.T) {
	run := func(codes ...int) *compliance.Run {
		r := &compliance.Run{}
		for i, code := range codes {
			r.ModuleActions = append(r.ModuleActions, &compliance.ModuleAction{
				Module:   string(rune('a' + i)),
				Action:   compliance.ActionCheck,
				ExitCode: code,
			})
		}
		return r
	}
	exitCode := func(err error) int {
		var coder interface{ ExitCode() int }
		if !errors.As(err, &coder) {
			return -1
		}
		return coder.ExitCode()
	}
	t.Run("compliant", func(t *testing.T) {
		var runs ComplianceRuns
		runs.Add(run(0, 2))
		runs.Add(run(0))
		assert.NoError(t, runs.Err())
	})
	t.Run("a node not compliant", func(t *testing.T) {
		var runs ComplianceRuns
		runs.Add(run(0))
		runs.Add(run(0, 1))
		assert.Equal(t, 1, exitCode(runs.Err()))
	})
	t.Run("not applicable anywhere", func(t *testing.T) {
		var runs ComplianceRuns
		runs.Add(run(2))
		runs.Add(run(2, 2))
		assert.Equal(t, 2, exitCode(runs.Err()))
	})
	t.Run("no run", func(t *testing.T) {
		var runs ComplianceRuns
		assert.NoError(t, runs.Err())
	})
}

// The options of the run reach the node as the parameters of the endpoint of
// the action, and the run it answers comes back as a run of this node.
func TestNodeComplianceRun(t *testing.T) {
	var got *http.Request
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apiComplianceRun())
	})
	opts := ComplianceRunOptions{Modulesets: "ms1", Force: true}
	run, err := nodeComplianceRun(context.Background(), c, "node1", compliance.ActionFix, opts)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/api/node/name/node1/compliance/action/fix", got.URL.Path)
	assert.Equal(t, "ms1", got.URL.Query().Get("moduleset"))
	assert.Equal(t, "true", got.URL.Query().Get("force"))
	assert.False(t, got.URL.Query().Has("module"))
	require.Len(t, run.ModuleActions, 2)
	assert.Equal(t, "mod1", run.ModuleActions[0].Module)
}

// A problem the node answers is the error, with the node it is of.
func TestNodeComplianceRunProblem(t *testing.T) {
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(api.Problem{Title: "Compliance check", Detail: "collector unreachable", Status: 500})
	})
	_, err := nodeComplianceRun(context.Background(), c, "node1", compliance.ActionCheck, ComplianceRunOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node node1")
	assert.Contains(t, err.Error(), "collector unreachable")
}

func TestComplianceModulesetTreeFromAPI(t *testing.T) {
	tree := ComplianceModulesetTreeFromAPI(api.ComplianceModulesetTree{
		Modules: []api.ComplianceModulesetModule{},
		Modulesets: []api.ComplianceModulesetTree{
			{
				Name:    "ms1",
				Modules: []api.ComplianceModulesetModule{{Name: "mod1", Autofix: true}},
				Modulesets: []api.ComplianceModulesetTree{
					{Name: "ms2", Modules: []api.ComplianceModulesetModule{{Name: "mod2"}}, Modulesets: []api.ComplianceModulesetTree{}},
				},
			},
		},
	})
	assert.Equal(t, "ms1\n mod1 (autofix)\n ms2\n  mod2\n", tree.Render())
}

func TestComplianceRulesetsFromAPI(t *testing.T) {
	rsets := ComplianceRulesetsFromAPI(api.ComplianceRulesets{
		"r1": {Name: "r1", Filter: "explicit attachment", Rules: []api.ComplianceRule{{Name: "motd", Value: "hello", Class: "raw"}}},
	})
	require.Contains(t, rsets, "r1")
	assert.Equal(t, "explicit attachment", rsets["r1"].Filter)
	assert.Equal(t, "hello", rsets["r1"].GetString("motd"))
	assert.Contains(t, rsets.Render(), "OSVC_COMP_MOTD=hello")
}

func TestNodeCheckLines(t *testing.T) {
	pushErr := "collector unreachable"
	lines, err := NodeCheckLines(api.CheckList{
		Kind: api.CheckListKindCheckList,
		Items: api.CheckItems{
			{Kind: "CheckItem", Meta: api.NodeMeta{Node: "node1"}, Data: api.CheckResult{Type: "fs_u", Driver: "df", Instance: "/", Unit: "%", Value: 42}},
		},
		PushError: &pushErr,
	})
	require.Len(t, lines, 1)
	assert.Equal(t, "node1", lines[0].Node)
	assert.Equal(t, "fs_u", lines[0].DriverGroup)
	assert.Equal(t, "42%", lines[0].ValueText)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node node1: push checks: collector unreachable")
}

func TestNodeCollectorTagShow(t *testing.T) {
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/node/name/node1/collector/tag/attachment", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.CollectorTagAttachments{
			{TagName: "t1", TagData: "d1", TagAttachData: "a1"},
			{TagName: "t2"},
		})
	})
	names, err := nodeCollectorTagShow(context.Background(), c, "node1", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"t1", "t2"}, names)
	l, err := nodeCollectorTagShow(context.Background(), c, "node1", true)
	require.NoError(t, err)
	assert.Equal(t, collector.TagAttachmentList{
		{TagName: "t1", TagData: "d1", TagAttachData: "a1"},
		{TagName: "t2"},
	}, l)
}
