package sgcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

func TestTokenFactory(t *testing.T) {
	defer Setup(t)()

	cfg := GetConfig()
	require.NotNil(t, cfg)

	// Create a test server
	var authCalls []string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type bodyStruct struct {
			Scope string
		}

		if r.URL.Path == "/auth/access_token" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			auth := r.Header.Get("Authorization")
			basicAuth, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
			require.NoError(t, err)
			if string(basicAuth) != "clientid1:clientSecret1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			l := strings.Split(string(basicAuth), ":")

			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			scope := r.FormValue("scope")

			t.Logf("Read %s bytes from request body", string(scope))
			w.WriteHeader(http.StatusOK)
			t.Logf("Post new token for for client id: %s", l[0])
			authCalls = append(authCalls, l[0]+":"+scope)
			enc := json.NewEncoder(w)
			enc.Encode(map[string]interface{}{
				"access_token": l[0] + ":" + scope,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	ts := httptest.NewUnstartedServer(handler)

	ln, err := net.Listen("tcp", "127.0.0.1:1215")
	if err != nil {
		t.Fatal(err)
	}

	ts.EnableHTTP2 = true
	ts.Listener = ln
	ts.StartTLS()
	defer ts.Close()

	authInfo := &AuthInfo{
		AccountID:    "account1",
		ClientID:     "clientid1",
		ClientSecret: "clientSecret1",
		Signature:    "the-secret",
	}
	client := ts.Client()
	log := plog.NewDefaultLogger()
	tk := NewTokenFactory(log, client, &cfg.Auth, authInfo)
	ctx := context.Background()

	require.NoError(t, tk.Clear("scope1"))
	require.NoError(t, tk.Clear("scope2", "scope3"))

	callSignScope1 := "clientid1:account1:scope1"
	callSignScope2and3 := "clientid1:account1:scope2 account1:scope3"

	t.Logf("get token for scope1")
	bearer, err := tk.Get(ctx, "scope1")
	require.NoError(t, err)
	require.Equal(t, "clientid1:account1:scope1", bearer)
	expectedCalls := []string{callSignScope1}
	require.Equal(t, expectedCalls, authCalls)

	t.Logf("get again token for scope1, must hit cache")
	bearer, err = tk.Get(ctx, "scope1")
	require.NoError(t, err)
	require.Equal(t, "clientid1:account1:scope1", bearer)
	expectedCalls = []string{callSignScope1}
	require.Equalf(t, expectedCalls, authCalls, "The second call didn't hit cache")

	t.Logf("get again token for scope2 and scope3")
	bearer, err = tk.Get(ctx, "scope2", "scope3")
	require.NoError(t, err)
	require.Equal(t, "clientid1:account1:scope2 account1:scope3", bearer)
	expectedCalls = []string{callSignScope1, callSignScope2and3}
	require.Equalf(t, expectedCalls, authCalls, "wanted calls %v, got %v", expectedCalls, authCalls)

	t.Logf("get again token for scope1, must hit cache")
	bearer, err = tk.Get(ctx, "scope1")
	require.NoError(t, err)
	require.Equal(t, "clientid1:account1:scope1", bearer)
	expectedCalls = []string{callSignScope1, callSignScope2and3}
	require.Equalf(t, expectedCalls, authCalls, "wanted calls %v, got %v", expectedCalls, authCalls)

	t.Logf("clear cache scope1")
	require.NoError(t, tk.Clear("scope1"))
	t.Logf("get again token for scope1, must create new token")
	bearer, err = tk.Get(ctx, "scope1")
	require.NoError(t, err)
	require.Equal(t, "clientid1:account1:scope1", bearer)
	expectedCalls = []string{callSignScope1, callSignScope2and3, callSignScope1}
	require.Equalf(t, expectedCalls, authCalls, "wanted calls %v, got %v", expectedCalls, authCalls)
}
