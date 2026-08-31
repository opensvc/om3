package testhelper

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_TcpPortAvailable(t *testing.T) {
	// Any free port will do. Don't pick a well known one: the node may run
	// a real daemon holding it.
	freeLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := strconv.Itoa(freeLn.Addr().(*net.TCPAddr).Port)
	require.NoError(t, freeLn.Close())

	require.NoErrorf(t, TCPPortAvailable(port), "port %s should be available before test", port)
	Trace(t)
	if t.Failed() {
		return
	}
	ln, err := net.Listen("tcp", ":"+port)
	require.NoError(t, err, "can't listen on available port")
	require.Error(t, TCPPortAvailable(port), "port should be unavailable")
	if err == nil {
		require.Nil(t, ln.Close())
	}
}
