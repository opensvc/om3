package lsnrhttpux

import (
	"context"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupGID(t *testing.T) {
	gid, err := lookupGID(context.Background(), "1234")
	require.NoError(t, err)
	assert.Equal(t, 1234, gid)
	g, err := user.LookupGroupId("0")
	require.NoError(t, err)
	gid, err = lookupGID(context.Background(), g.Name)
	require.NoError(t, err)
	assert.Equal(t, 0, gid)
	_, err = lookupGID(context.Background(), "no-such-group-of-om3-tests")
	assert.Error(t, err)
}

// The directory is made before the socket, so that no one else reaches the
// socket while its own mode is not set, and both are the group's.
func TestOwnedSocket(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	gid := 4242
	addr := filepath.Join(t.TempDir(), "uxuser", "http.sock")
	require.NoError(t, prepareOwnedDir(addr, gid))
	l, err := net.Listen("unix", addr)
	require.NoError(t, err)
	defer l.Close()
	require.NoError(t, ownSocket(addr, gid))
	for path, mode := range map[string]os.FileMode{filepath.Dir(addr): ownedDirMode, addr: ownedSockMode} {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, mode, info.Mode().Perm(), path)
		st := info.Sys().(*syscall.Stat_t)
		assert.Equal(t, uint32(0), st.Uid, path)
		assert.Equal(t, uint32(gid), st.Gid, path)
	}
}
