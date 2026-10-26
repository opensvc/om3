package lsnrhttpux

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	golog "log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/util/plog"
)

// A client stalling the headers of its request is dropped once the header
// timeout is over, while one sending them is served.
func TestAClientStallingItsHeadersIsDropped(t *testing.T) {
	saved := daemonenv.HTTPReadHeaderTimeout
	daemonenv.HTTPReadHeaderTimeout = 200 * time.Millisecond
	t.Cleanup(func() { daemonenv.HTTPReadHeaderTimeout = saved })

	sock := filepath.Join(t.TempDir(), "http.sock")
	lsnr, err := net.Listen("unix", sock)
	require.NoError(t, err)
	server := newServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}), golog.New(io.Discard, "", 0))
	go func() { _ = server.Serve(lsnr) }()
	t.Cleanup(func() { _ = server.Close() })

	conn, err := net.Dial("unix", sock)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n"))
	require.NoError(t, err)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	begin := time.Now()
	_, err = io.ReadAll(conn)
	require.NoError(t, err, "the server closes the connection rather than the client giving up")
	assert.Less(t, time.Since(begin), 3*time.Second)

	conn, err = net.Dial("unix", sock)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// A request of a listener reading the peers carries the credentials of the
// process connected, as the kernel recorded them.
func TestARequestCarriesItsPeer(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "http.sock")
	lsnr, err := net.Listen("unix", sock)
	require.NoError(t, err)
	lsnrT := &T{log: plog.NewDefaultLogger(), peerCred: true}
	server := newServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer := daemonctx.Peer(r.Context())
		if peer == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprintf(w, "%d %d", peer.UID, peer.PID)
	}), golog.New(io.Discard, "", 0))
	server.ConnContext = lsnrT.withPeer
	go func() { _ = server.Serve(lsnr) }()
	t.Cleanup(func() { _ = server.Close() })

	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		},
	}}
	resp, err := client.Get("http://localhost/")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%d %d", os.Getuid(), os.Getpid()), string(b))
}

// The owner group emptied closes the socket, and the connections open through
// it, http/1.1 and http/2 alike: the members of the former group lose the api
// at once. The connections are tracked, and their peer read through the
// tracking.
func TestCloseSocketDropsTheOpenConnections(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "http.sock")
	raw, err := net.Listen("unix", sock)
	require.NoError(t, err)
	lsnrT := &T{log: plog.NewDefaultLogger(), addr: sock, peerCred: true, tracked: newTrackListener(raw)}
	var lsnr net.Listener = lsnrT.tracked
	lsnrT.listener = &lsnr
	lsnrT.server = newServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer := daemonctx.Peer(r.Context())
		if peer == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprintf(w, "%d", peer.UID)
	}), golog.New(io.Discard, "", 0))
	lsnrT.server.ConnContext = lsnrT.withPeer
	served := make(chan struct{})
	go func() {
		defer close(served)
		_ = lsnrT.server.Serve(lsnr)
	}()

	conn, err := net.Dial("unix", sock)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	require.NoError(t, err)
	r := bufio.NewReader(conn)
	resp, err := http.ReadResponse(r, nil)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, fmt.Sprint(os.Getuid()), string(b), "the peer read through the tracking")

	h2 := &http.Client{Transport: &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, _, _ string, _ *tls.Config) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		},
	}}
	resp, err = h2.Get("http://localhost/")
	require.NoError(t, err)
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, 2, resp.ProtoMajor)
	lsnrT.tracked.mu.Lock()
	open := len(lsnrT.tracked.conns)
	lsnrT.tracked.mu.Unlock()
	assert.Equal(t, 2, open, "the http/1.1 and the http/2 connections")

	lsnrT.closeSocket()
	<-served
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	if err == nil {
		_, err = http.ReadResponse(r, nil)
	}
	assert.Error(t, err, "the http/1.1 connection is closed")
	lsnrT.tracked.mu.Lock()
	open = len(lsnrT.tracked.conns)
	lsnrT.tracked.mu.Unlock()
	assert.Equal(t, 0, open, "the http/2 connection is closed")
	_, err = os.Stat(sock)
	assert.ErrorIs(t, err, os.ErrNotExist, "the socket is removed")
	_, err = net.Dial("unix", sock)
	assert.Error(t, err)
	assert.NoError(t, lsnrT.Stop(), "stopping a closed listener")
}
