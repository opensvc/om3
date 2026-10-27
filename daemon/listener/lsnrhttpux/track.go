package lsnrhttpux

import (
	"net"
	"slices"
	"sync"

	"github.com/opensvc/om3/v3/util/peercred"
)

type (
	// trackListener records the connections it accepts until they close,
	// for them to be closed with the socket: the server does not track
	// the http/2 ones, which the h2c handler takes over from it.
	//
	// It accepts only the peers allowed to use the socket as it is owned
	// now: root, or a member of its group, as the kernel recorded the
	// peer at connect. The ownership of the socket file is checked by the
	// kernel at connect only, so a connection made before the group
	// changed, and accepted after, would otherwise outlive the change. The
	// group is changed, and the connections closed, under the lock the
	// accepts are checked under, so a connection is either checked against
	// the new group or closed with the former one.
	trackListener struct {
		net.Listener

		mu    sync.Mutex
		conns map[*trackConn]struct{}
		gid   int

		// ids returns the uid and the gids of the peer of a connection,
		// peercred.IDs but in the tests.
		ids func(net.Conn) (uint32, []uint32, error)
	}

	// trackConn is a connection a trackListener accepted.
	trackConn struct {
		net.Conn
		l    *trackListener
		once sync.Once
	}
)

// newTrackListener returns l tracking its connections, and accepting the
// peers allowed to use the socket owned by the group gid.
func newTrackListener(l net.Listener, gid int) *trackListener {
	return &trackListener{Listener: l, conns: make(map[*trackConn]struct{}), gid: gid, ids: peercred.IDs}
}

// Accept returns the next connection of a peer allowed to use the socket,
// closing the others.
func (t *trackListener) Accept() (net.Conn, error) {
	for {
		c, err := t.Listener.Accept()
		if err != nil {
			return nil, err
		}
		tc := &trackConn{Conn: c, l: t}
		t.mu.Lock()
		allowed := t.allowed(c)
		if allowed {
			t.conns[tc] = struct{}{}
		}
		t.mu.Unlock()
		if allowed {
			return tc, nil
		}
		_ = c.Close()
	}
}

// allowed says the peer of c may use the socket owned by the group: it is
// root, which a rootful container sidecar is, or the group is one of its
// own.
func (t *trackListener) allowed(c net.Conn) bool {
	uid, gids, err := t.ids(c)
	if err != nil {
		return false
	}
	return uid == 0 || slices.Contains(gids, uint32(t.gid))
}

// regroup makes gid the group the peers accepted are members of, and closes
// the connections accepted for the former one.
func (t *trackListener) regroup(gid int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.gid = gid
	for c := range t.conns {
		c.closeLocked()
	}
}

// closeConns closes the connections accepted and open.
func (t *trackListener) closeConns() {
	t.mu.Lock()
	conns := make([]*trackConn, 0, len(t.conns))
	for c := range t.conns {
		conns = append(conns, c)
	}
	t.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

func (c *trackConn) Close() error {
	c.once.Do(func() {
		c.l.mu.Lock()
		delete(c.l.conns, c)
		c.l.mu.Unlock()
	})
	return c.Conn.Close()
}

// closeLocked closes c, the lock of its listener held.
func (c *trackConn) closeLocked() {
	c.once.Do(func() {
		delete(c.l.conns, c)
	})
	_ = c.Conn.Close()
}

// NetConn returns the connection accepted, as tls.Conn does: the peer of a
// unix socket connection is read from it.
func (c *trackConn) NetConn() net.Conn {
	return c.Conn
}
