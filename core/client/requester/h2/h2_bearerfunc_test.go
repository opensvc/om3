package reqh2

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slowListener serves its connections delay after it accepted them, so the
// tls handshake of the client, which needs the server, completes delay after
// the connect, as when a peer whose network comes back is slow to reach.
type slowListener struct {
	net.Listener
	delay time.Duration
}

func (l *slowListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil {
		time.Sleep(l.delay)
	}
	return c, err
}

// nodeTokenLike returns a token valid 5 seconds, its exp in whole seconds as
// the node token has it, in clear so the server reads it back.
func nodeTokenLike() (string, error) {
	return strconv.FormatInt(time.Now().Add(5*time.Second).Unix(), 10), nil
}

// The bearer of BearerFunc is minted once the connection is ready: a token
// valid 5 seconds reaches a server whose connection took 6 seconds with most
// of its validity left, where it used to be minted before the connect and
// arrive expired.
func TestNewInetBearerFuncMintsOnceConnected(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for a 6s connection")
	}
	var (
		mu        sync.Mutex
		remaining time.Duration
		header    string
	)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received := time.Now()
		mu.Lock()
		defer mu.Unlock()
		header = r.Header.Get("Authorization")
		exp, err := strconv.ParseInt(strings.TrimPrefix(header, "Bearer "), 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		remaining = time.Unix(exp, 0).Sub(received)
		if remaining <= 0 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	srv.Listener = &slowListener{Listener: srv.Listener, delay: 6 * time.Second}
	srv.StartTLS()
	defer srv.Close()

	c, err := NewInet(Config{URL: srv.URL, BearerFunc: nodeTokenLike, InsecureSkipVerify: true})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.GetNodePing(ctx, "n1")
	require.NoError(t, err)
	_ = resp.Body.Close()
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Greater(t, remaining, 3*time.Second, "validity left of the token %q as the server received it", header)
}

// A token BearerFunc fails to mint is the error of the request, which is not
// sent, on a new connection as on one reused from the pool.
func TestNewInetBearerFuncError(t *testing.T) {
	var (
		mu       sync.Mutex
		received []string
	)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	fnErr := errors.New("jwt sign key is not loaded")
	failing := false
	fn := func() (string, error) {
		if failing {
			return "", fnErr
		}
		return "tk", nil
	}
	c, err := NewInet(Config{URL: srv.URL, BearerFunc: fn, InsecureSkipVerify: true, Password: "pw", Username: "u"})
	require.NoError(t, err)
	// The first request leaves its connection idle in the pool.
	resp, err := c.GetNodePing(context.Background(), "n1")
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	failing = true
	for range 2 {
		_, err = c.GetNodePing(context.Background(), "n1")
		require.ErrorIs(t, err, fnErr)
	}
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"Bearer tk"}, received)
}

// A redirect to another server is followed with no token: the token of the
// node is minted for the origin of the client alone.
func TestNewInetBearerFuncNotSentAcrossRedirect(t *testing.T) {
	var (
		mu       sync.Mutex
		received []string
	)
	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, r.Header.Get("Authorization"))
	}))
	defer other.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusFound)
	}))
	defer srv.Close()

	minted := 0
	fn := func() (string, error) {
		minted++
		return "tk", nil
	}
	c, err := NewInet(Config{URL: srv.URL, BearerFunc: fn, InsecureSkipVerify: true})
	require.NoError(t, err)
	resp, err := c.GetNodePing(context.Background(), "n1")
	require.NoError(t, err)
	_ = resp.Body.Close()
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{""}, received, "the Authorization the redirect target received")
	assert.Equal(t, 1, minted, "the tokens minted, for the origin alone")
}
