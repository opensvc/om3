//go:build linux

package peercred

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromConn(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "s.sock")
	l, err := net.Listen("unix", sock)
	require.NoError(t, err)
	defer l.Close()
	go func() {
		c, err := net.Dial("unix", sock)
		if err == nil {
			defer c.Close()
			buf := make([]byte, 1)
			_, _ = c.Read(buf)
		}
	}()
	c, err := l.Accept()
	require.NoError(t, err)
	defer c.Close()
	peer, err := FromConn(c)
	require.NoError(t, err)
	assert.Equal(t, uint32(os.Getuid()), peer.UID)
	assert.Equal(t, uint32(os.Getgid()), peer.GID)
	assert.Equal(t, int32(os.Getpid()), peer.PID)
	assert.False(t, peer.ForeignMountNS, "the peer is this process")
	t.Logf("pinned=%v cgroup=%s", peer.Pinned, peer.Cgroup)
	if peer.Pinned {
		b, err := os.ReadFile("/proc/self/cgroup")
		require.NoError(t, err)
		assert.Equal(t, systemdCgroup(b), peer.Cgroup)
	}
}

func TestFromConnNotUnix(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	go func() {
		if c, err := net.Dial("tcp", l.Addr().String()); err == nil {
			c.Close()
		}
	}()
	c, err := l.Accept()
	require.NoError(t, err)
	defer c.Close()
	_, err = FromConn(c)
	assert.ErrorIs(t, err, ErrNotUnix)
}

func TestSystemdCgroup(t *testing.T) {
	assert.Equal(t, "/system.slice/a.service", systemdCgroup([]byte("2:cpu:/x\n1:name=systemd:/system.slice/a.service\n0::/y\n")))
	assert.Equal(t, "/user.slice/u.scope", systemdCgroup([]byte("0::/user.slice/u.scope\n")))
	assert.Equal(t, "", systemdCgroup([]byte("2:cpu:/x\n")))
}

// Without a pidfd, the process is read by its pid: the peer is known, but not
// pinned.
func TestReadProcessByPid(t *testing.T) {
	peer := &T{PID: int32(os.Getpid())}
	require.NoError(t, peer.readProcess(-1))
	assert.False(t, peer.Pinned)
	assert.False(t, peer.ForeignMountNS, "the peer is this process")
	b, err := os.ReadFile("/proc/self/cgroup")
	require.NoError(t, err)
	assert.Equal(t, systemdCgroup(b), peer.Cgroup)
}

// A process that can not be read leaves the peer unknown, rather than taken
// for a process of the node.
func TestReadProcessGone(t *testing.T) {
	b, err := os.ReadFile("/proc/sys/kernel/pid_max")
	require.NoError(t, err)
	var max int32
	_, err = fmt.Sscan(string(b), &max)
	require.NoError(t, err)
	peer := &T{PID: max}
	assert.Error(t, peer.readProcess(-1))
	assert.False(t, peer.ForeignMountNS)
}

// The ids of the peer are the ones the access to the socket was checked
// with: its uid, and its groups, the primary one first.
func TestIDs(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "s.sock")
	l, err := net.Listen("unix", sock)
	require.NoError(t, err)
	defer l.Close()
	go func() {
		c, err := net.Dial("unix", sock)
		if err == nil {
			defer c.Close()
			buf := make([]byte, 1)
			_, _ = c.Read(buf)
		}
	}()
	c, err := l.Accept()
	require.NoError(t, err)
	defer c.Close()
	uid, gids, err := IDs(c)
	require.NoError(t, err)
	assert.Equal(t, uint32(os.Getuid()), uid)
	require.NotEmpty(t, gids)
	assert.Equal(t, uint32(os.Getgid()), gids[0])
	groups, err := os.Getgroups()
	require.NoError(t, err)
	for _, g := range groups {
		assert.Contains(t, gids, uint32(g))
	}
}

func TestProcGroups(t *testing.T) {
	groups, err := procGroups(int32(os.Getpid()))
	require.NoError(t, err)
	want, err := os.Getgroups()
	require.NoError(t, err)
	assert.Len(t, groups, len(want))
}
