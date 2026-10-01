package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// dialUnix returns the error a request to a unix socket at p fails with.
func dialUnix(t *testing.T, p string) error {
	t.Helper()
	c := &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", p)
			},
		},
	}
	_, err := c.Get("http://localhost/api")
	return err
}

// A missing socket and a socket nobody listens on are a daemon down, as the
// errors a request fails with say it, wrapped or not. Any other failure is
// not.
func TestIsDaemonDown(t *testing.T) {
	dir := t.TempDir()

	missing := dialUnix(t, filepath.Join(dir, "missing.sock"))
	assert.True(t, IsDaemonDown(missing), "missing socket: %v", missing)
	assert.True(t, IsDaemonDown(fmt.Errorf("select objects: %w", missing)), "wrapped")

	// A socket file left behind by a daemon that died.
	stale := filepath.Join(dir, "stale.sock")
	l, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = l.Close()
	refused := dialUnix(t, stale)
	assert.True(t, IsDaemonDown(refused), "refused: %v", refused)

	assert.False(t, IsDaemonDown(nil))
	assert.False(t, IsDaemonDown(errors.New("[403] forbidden")))
	tcp := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	assert.False(t, IsDaemonDown(tcp), "a node reached over the network")
}
