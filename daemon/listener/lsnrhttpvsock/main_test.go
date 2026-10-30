package lsnrhttpvsock

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/mdlayher/vsock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	fakeConn struct {
		net.Conn
		addr   net.Addr
		closed bool
	}
	fakeListener struct {
		net.Listener
		conns []*fakeConn
	}
)

func (c *fakeConn) RemoteAddr() net.Addr { return c.addr }
func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

func (l *fakeListener) Accept() (net.Conn, error) {
	if len(l.conns) == 0 {
		return nil, net.ErrClosed
	}
	c := l.conns[0]
	l.conns = l.conns[1:]
	return c, nil
}

// A guest, the loopback or a peer of no vsock address is dropped, and the
// listener goes on to the next connection.
func TestAcceptServesTheHypervisorAlone(t *testing.T) {
	guest := &fakeConn{addr: &vsock.Addr{ContextID: 4, Port: 1023}}
	loopback := &fakeConn{addr: &vsock.Addr{ContextID: vsock.Local, Port: 1023}}
	other := &fakeConn{addr: &net.TCPAddr{}}
	host := &fakeConn{addr: &vsock.Addr{ContextID: vsock.Host, Port: 1023}}
	l := &hostListener{
		Listener: &fakeListener{conns: []*fakeConn{guest, loopback, other, host}},
		log:      plog.NewDefaultLogger(),
	}

	c, err := l.Accept()
	require.NoError(t, err)
	assert.Same(t, host, c)
	assert.True(t, guest.closed)
	assert.True(t, loopback.closed)
	assert.True(t, other.closed)
	assert.False(t, host.closed)

	_, err = l.Accept()
	assert.True(t, errors.Is(err, net.ErrClosed))
}

// A forward accepts no more than a privileged process of the hypervisor.
func TestAcceptReservedOnly(t *testing.T) {
	user := &fakeConn{addr: &vsock.Addr{ContextID: vsock.Host, Port: 1024}}
	root := &fakeConn{addr: &vsock.Addr{ContextID: vsock.Host, Port: 1023}}
	l := &hostListener{
		Listener:     &fakeListener{conns: []*fakeConn{user, root}},
		log:          plog.NewDefaultLogger(),
		reservedOnly: true,
	}
	c, err := l.Accept()
	require.NoError(t, err)
	assert.Same(t, root, c)
	assert.True(t, user.closed)
}

func TestWithPeer(t *testing.T) {
	ctx := withPeer(context.Background(), &fakeConn{addr: &vsock.Addr{ContextID: 2, Port: 1023}})
	cid, port, ok := daemonctx.VsockPeer(ctx)
	assert.True(t, ok)
	assert.Equal(t, uint32(2), cid)
	assert.Equal(t, uint32(1023), port)

	_, _, ok = daemonctx.VsockPeer(withPeer(context.Background(), &fakeConn{addr: &net.TCPAddr{}}))
	assert.False(t, ok)
}
