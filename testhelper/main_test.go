package testhelper

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_TcpPortAvailable(t *testing.T) {
	port := "1215"
	require.NoErrorf(t, TcpPortAvailable(port), "port %s should be available before test", port)
	if t.Failed() {
		return
	}
	ln, err := net.Listen("tcp", ":"+port)
	require.NoError(t, err, "can't listen on available port")
	require.Error(t, TcpPortAvailable(port), "port should be unavailable")
	if err == nil {
		require.Nil(t, ln.Close())
	}
}
