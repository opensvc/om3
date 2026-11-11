package commoncmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
)

// existingObjectServer answers a create of an object that exists 409, and
// replaces it when forced. It records whether it was.
func existingObjectServer(t *testing.T, forced *bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("force") == "true" {
			*forced = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"title": "Conflict", "detail": "exists", "status": 409})
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	return srv
}

// A restore replaces an object that exists through the api too, as it does
// with no daemon running: a hypervisor pushes the config of an encap object
// so, each time it changes.
func TestCreateRestoreReplacesAnExistingObject(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}

	var forced bool
	srv := existingObjectServer(t, &forced)
	defer srv.Close()
	c, err := client.New(client.WithURL(srv.URL), client.WithBearer("x"), client.WithInsecureSkipVerify(true))
	require.NoError(t, err)

	err = CreateObjectConfig(context.Background(), c, p, []byte("[DEFAULT]\n"), nil, false, false, 0, nil)
	assert.ErrorContains(t, err, "already exists", "a plain create")
	assert.False(t, forced)

	require.NoError(t, CreateObjectConfig(context.Background(), c, p, []byte("[DEFAULT]\n"), nil, true, false, 0, nil), "a restore")
	assert.True(t, forced)
}

// A dry run asks the daemon to check the create and write nothing, and does
// not ask it to wait for an object it will not know.
func TestCreateObjectConfigDryRun(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}
	var query map[string][]string
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		if r.URL.Query().Get("dry_run") != "true" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"title": "Forbidden", "detail": "Config validation: denied: a\ndenied: b", "status": 403})
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	c, err := client.New(client.WithURL(srv.URL), client.WithBearer("x"), client.WithInsecureSkipVerify(true))
	require.NoError(t, err)

	err = CreateObjectConfig(context.Background(), c, p, []byte("[DEFAULT]\n"), nil, false, false, 0, &DryRun{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied: a\ndenied: b", "every refusal reported")
	assert.Equal(t, []string{"true"}, query["dry_run"])
	assert.Empty(t, query["wait_local"])
}

// alertsServer answers a dry run with the alerts of its checks.
func alertsServer(t *testing.T, items []map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("dry_run") != "true" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"kind": "ConfigAlertList", "items": items})
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	return srv
}

// A dry run answered alerts fails when one is an error, so the command exits
// non-zero for a create that would be refused, and succeeds when they are
// warnings alone.
func TestCreateObjectConfigDryRunAlerts(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}
	alert := func(level string) map[string]any {
		return map[string]any{"node": "n1", "path": "test/svc/s1", "level": level, "kind": "denied", "key": "container#1.privileged", "driver": "", "comment": "a privileged container requires the root grant"}
	}
	for level, fails := range map[string]bool{"error": true, "warning": false} {
		srv := alertsServer(t, []map[string]any{alert(level)})
		c, err := client.New(client.WithURL(srv.URL), client.WithBearer("x"), client.WithInsecureSkipVerify(true))
		require.NoError(t, err)
		err = CreateObjectConfig(context.Background(), c, p, []byte("[DEFAULT]\n"), nil, false, false, 0, &DryRun{Output: "json"})
		if fails {
			assert.ErrorContains(t, err, "1 error(s)", level)
		} else {
			assert.NoError(t, err, level)
		}
		srv.Close()
	}
}

// A create asked to wait asks the daemon to hold it as long, and a plain one
// leaves the hold to the daemon: the daemon knows the object once the first
// status of its instance is in, which may last longer than its hold of a
// create with no wait. The client waits longer than the daemon holds, for the
// 408 past the hold, saying the object is created, to reach it.
func TestCreateAsksTheDaemonToHoldItAsLongAsItWaits(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}
	var query map[string][]string
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.WriteHeader(http.StatusNoContent)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	c, err := client.New(client.WithURL(srv.URL), client.WithBearer("x"), client.WithInsecureSkipVerify(true))
	require.NoError(t, err)

	require.NoError(t, CreateObjectConfig(context.Background(), c, p, []byte("[DEFAULT]\n"), nil, false, false, 2*time.Minute, nil))
	assert.Equal(t, []string{"true"}, query["wait_local"])
	assert.Equal(t, []string{"2m0s"}, query["wait"])

	require.NoError(t, CreateObjectConfig(context.Background(), c, p, []byte("[DEFAULT]\n"), nil, false, false, 0, nil))
	assert.Equal(t, []string{"true"}, query["wait_local"])
	assert.Empty(t, query["wait"])

	assert.Greater(t, CreateClientTimeout(0), CreateKnownHold, "the client outlasts the hold of a plain create")
	assert.Greater(t, CreateClientTimeout(2*time.Minute), 2*time.Minute, "the client outlasts the wait")
}
