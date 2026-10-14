package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bearer of each request is the one the function returns as it is sent,
// so a token valid a few seconds is not expired by a request waiting to be
// sent.
func TestWithBearerFunc(t *testing.T) {
	t.Setenv("OSVC_CONTEXT", "")
	var (
		mu  sync.Mutex
		got []string
	)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Header.Get("Authorization"))
		mu.Unlock()
	}))
	t.Cleanup(srv.Close)

	calls := 0
	c, err := New(
		WithURL(srv.URL),
		WithInsecureSkipVerify(true),
		WithBearerFunc(func() (string, error) {
			calls++
			return fmt.Sprintf("tk%d", calls), nil
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, 0, calls, "no token signed by the client creation")
	for range 2 {
		resp, err := c.GetNodePing(context.Background(), "node1")
		require.NoError(t, err)
		_ = resp.Body.Close()
	}
	assert.Equal(t, []string{"Bearer tk1", "Bearer tk2"}, got)
}

// A token that can not be signed fails the request, which is not sent.
func TestWithBearerFuncError(t *testing.T) {
	t.Setenv("OSVC_CONTEXT", "")
	sent := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { sent = true }))
	t.Cleanup(srv.Close)

	c, err := New(
		WithURL(srv.URL),
		WithInsecureSkipVerify(true),
		WithBearerFunc(func() (string, error) { return "", errors.New("no key") }),
	)
	require.NoError(t, err)
	_, err = c.GetNodePing(context.Background(), "node1")
	assert.ErrorContains(t, err, "no key")
	assert.False(t, sent)
}
