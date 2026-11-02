package commoncmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

	err = CreateObjectConfig(context.Background(), c, p, []byte("[DEFAULT]\n"), nil, false, false)
	assert.ErrorContains(t, err, "already exists", "a plain create")
	assert.False(t, forced)

	require.NoError(t, CreateObjectConfig(context.Background(), c, p, []byte("[DEFAULT]\n"), nil, true, false), "a restore")
	assert.True(t, forced)
}
