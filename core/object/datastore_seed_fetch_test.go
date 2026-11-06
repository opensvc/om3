package object_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/plog"
)

// A key seeded from an url holds node.max_key_size at most: a body announced
// larger is refused before it is read, and one sent larger once that much is
// read, rather than read whole for the key add to refuse it. A server
// sending nothing holds the seeding as long as the action lasts.
func TestSeedKeyFetchIsCapped(t *testing.T) {
	setupKeySources(t)
	big := strings.Repeat("x", 2*1024*1024)
	block := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/small":
			_, _ = w.Write([]byte("small"))
		case "/announced":
			w.Header().Set("Content-Length", "2097152")
			_, _ = w.Write([]byte(big))
		case "/streamed":
			// No length announced: the body is sent in chunks.
			for i := 0; i < 4; i++ {
				_, _ = w.Write([]byte(big[:512*1024]))
				w.(http.Flusher).Flush()
			}
		case "/stalled":
			select {
			case <-block:
			case <-r.Context().Done():
			}
		}
	}))
	defer srv.Close()
	defer close(block)
	defer object.SetSeedHttpClient(srv.Client())()

	sec := naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "db"}
	ds, _, err := object.EnsureDataStore(sec, "dev")
	require.NoError(t, err)
	p := naming.Path{Namespace: "dev", Kind: naming.KindSvc, Name: "web"}
	seed := func(ctx context.Context, k, path string) error {
		return object.SeedKey(ctx, ds, k, srv.URL+path, plog.NewDefaultLogger(), p, "")
	}

	require.NoError(t, seed(context.Background(), "small", "/small"))
	assert.True(t, object.HasDataStoreKey(sec, "small"))

	for _, path := range []string{"/announced", "/streamed"} {
		err := seed(context.Background(), "big", path)
		require.ErrorIs(t, err, object.ErrValueTooBig, path)
		assert.ErrorContains(t, err, "node.max_key_size", path)
		assert.ErrorContains(t, err, srv.URL+path, path)
	}
	assert.False(t, object.HasDataStoreKey(sec, "big"))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- seed(ctx, "stalled", "/stalled") }()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("the seeding outlives its action")
	}
}
