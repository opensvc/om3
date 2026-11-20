package capexec

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSdNotifyReady(t *testing.T) {
	p := filepath.Join(t.TempDir(), "notify")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: p, Net: "unixgram"})
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	t.Setenv("NOTIFY_SOCKET", p)
	require.NoError(t, sdNotifyReady())
	b := make([]byte, 64)
	n, err := conn.Read(b)
	require.NoError(t, err)
	assert.Equal(t, "READY=1", string(b[:n]))

	t.Setenv("NOTIFY_SOCKET", "")
	assert.ErrorContains(t, sdNotifyReady(), "no NOTIFY_SOCKET")
}

func TestArgvSdNotify(t *testing.T) {
	b := true
	assert.Contains(t, T{SdNotify: &b}.Argv(), "--sd-notify")
	assert.NotContains(t, T{}.Argv(), "--sd-notify")
}
