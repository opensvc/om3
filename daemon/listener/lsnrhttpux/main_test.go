package lsnrhttpux

import (
	"bufio"
	"io"
	golog "log"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/daemonenv"
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
