package sgcp

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	TTkBuilder struct{}
)

func (ttk *TTkBuilder) Get(ctx context.Context, scope ...string) (string, error) {
	return "tk_scopes=" + strings.Join(scope, ","), nil
}

func TestDo(t *testing.T) {
	defer Setup(t)()
	InstallConfig(t)

	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Create a test server
	var calls []string

	validAuth := "Bearer tk_scopes=scope1,scope2"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/foo/bar" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			auth := r.Header.Get("Authorization")
			if auth != validAuth {
				t.Logf("invalid auth %s instead of %s", auth, validAuth)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			t.Logf("%s %s auth %s", r.Method, r.URL.Path, auth)
			w.WriteHeader(http.StatusOK)
			calls = append(calls, "test")
			enc := json.NewEncoder(w)
			enc.Encode(map[string]interface{}{"test": calls})
			return
		}
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

	client := ts.Client()
	log := plog.NewDefaultLogger()

	api := Api{
		client: client,
		log:    log,
		tk:     &TTkBuilder{},
	}

	ctx := context.Background()

	// First request should hit the server
	code, data1, err := api.do(ctx, "GET", "https://127.0.0.1:1215/foo/bar", nil, "scope1", "scope2")
	require.Equal(t, 200, code)
	require.NoError(t, err)
	t.Logf("First request result: %s", string(data1))
	assert.Equal(t, string([]byte(`{"test":["test"]}`)), strings.TrimSuffix(string(data1), "\n"))
}

// TestCheckStatusCode tests the status code checking
func TestCheckStatusCode(t *testing.T) {
	a := &Api{
		log: plog.NewDefaultLogger(),
	}

	err := a.CheckStatusCode(http.MethodGet, "https://localhost:1215/foo", 403, 200, 201)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code for GET https://localhost:1215/foo got 403 wanted [200 201]")

	err = a.CheckStatusCode(http.MethodGet, "https://localhost:1215/foo", 201, 200, 201)
	assert.Nil(t, err)
}
