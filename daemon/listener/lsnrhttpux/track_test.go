package lsnrhttpux

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The listener accepts the peers allowed to use the socket as it is owned
// when the connection is accepted: root, or a member of its group. A peer
// that connected before the group changed, and is accepted after, is closed
// rather than served.
func TestTheAcceptedPeersAreTheGroupMembers(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "http.sock")
	raw, err := net.Listen("unix", sock)
	require.NoError(t, err)
	l := newTrackListener(raw, 100)
	defer l.Close()

	// The peers are told apart by the order they connect in.
	peers := make(chan [2]any, 8)
	l.ids = func(net.Conn) (uint32, []uint32, error) {
		p := <-peers
		return p[0].(uint32), p[1].([]uint32), nil
	}
	dial := func(uid uint32, gids ...uint32) net.Conn {
		peers <- [2]any{uid, gids}
		c, err := net.Dial("unix", sock)
		require.NoError(t, err)
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	closed := func(c net.Conn) bool {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, err := c.Read(make([]byte, 1))
		return err != nil && !isTimeout(err)
	}

	// A member of the group 200, which does not own the socket, then a
	// member of the group 100, which does: the first is closed, the
	// second accepted.
	stranger := dial(1001, 1001, 200)
	member := dial(1002, 1002, 100)
	c, err := l.Accept()
	require.NoError(t, err)
	assert.True(t, closed(stranger), "a peer not in the group is closed")
	assert.Len(t, l.conns, 1)

	// Root is accepted whatever its groups, as a rootful sidecar is.
	root := dial(0, 0)
	_, err = l.Accept()
	require.NoError(t, err)
	assert.Len(t, l.conns, 2)

	// The group changed to 200: the connections accepted are closed, and
	// the member of 200 that connected meanwhile is accepted, the one of
	// 100 not.
	l.regroup(200)
	assert.Empty(t, l.conns)
	assert.True(t, closed(c.(*trackConn).Conn))
	_ = member
	_ = root
	former := dial(1002, 1002, 100)
	newcomer := dial(1001, 1001, 200)
	_, err = l.Accept()
	require.NoError(t, err)
	assert.True(t, closed(former), "a peer of the former group accepted after the change is closed")
	assert.Len(t, l.conns, 1)
	_ = newcomer
}

func isTimeout(err error) bool {
	ne, ok := err.(net.Error)
	return ok && ne.Timeout()
}
