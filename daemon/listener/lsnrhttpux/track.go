package lsnrhttpux

import (
	"net"
	"sync"
)

type (
	// trackListener records the connections it accepts until they close,
	// for them to be closed with the socket: the server does not track
	// the http/2 ones, which the h2c handler takes over from it.
	trackListener struct {
		net.Listener

		mu    sync.Mutex
		conns map[*trackConn]struct{}
	}

	// trackConn is a connection a trackListener accepted.
	trackConn struct {
		net.Conn
		l    *trackListener
		once sync.Once
	}
)

func newTrackListener(l net.Listener) *trackListener {
	return &trackListener{Listener: l, conns: make(map[*trackConn]struct{})}
}

func (t *trackListener) Accept() (net.Conn, error) {
	c, err := t.Listener.Accept()
	if err != nil {
		return nil, err
	}
	tc := &trackConn{Conn: c, l: t}
	t.mu.Lock()
	t.conns[tc] = struct{}{}
	t.mu.Unlock()
	return tc, nil
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

// NetConn returns the connection accepted, as tls.Conn does: the peer of a
// unix socket connection is read from it.
func (c *trackConn) NetConn() net.Conn {
	return c.Conn
}
