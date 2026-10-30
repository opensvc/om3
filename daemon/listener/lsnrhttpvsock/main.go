// Package lsnrhttpvsock serves the api on a vsock port of a guest, so the
// daemon of its hypervisor reaches it with no network and no credential.
package lsnrhttpvsock

import (
	"context"
	"errors"
	"fmt"
	golog "log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/mdlayher/vsock"

	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/listener/lsnrhttpux"
	"github.com/opensvc/om3/v3/daemon/listener/routehttp"
	"github.com/opensvc/om3/v3/util/plog"
)

// cidAny is VMADDR_CID_ANY. vsock.Listen binds the context id of the guest
// instead, which a live migration may change, leaving the listener on an
// address nothing reaches anymore.
const cidAny = 0xffffffff

type (
	T struct {
		listener net.Listener
		log      *plog.Logger
		port     uint32
		wg       sync.WaitGroup
		server   *http.Server

		// stopping says Stop closed the listener: the error the serve
		// loop ends on then is the expected one, which the vsock library
		// does not wrap as net.ErrClosed.
		stopping atomic.Bool
	}

	// hostListener accepts the connections of the hypervisor alone, the
	// ones from a reserved source port with reservedOnly.
	hostListener struct {
		net.Listener
		log          *plog.Logger
		reservedOnly bool
	}
)

// reservedPorts bounds the source ports only a process with
// CAP_NET_BIND_SERVICE can bind.
const reservedPorts = 1024

// ListenHypervisor returns a listener of the vsock port accepting the
// connections of a privileged process of the hypervisor alone, for what has no
// other check of its caller at accept, as the console endpoint.
func ListenHypervisor(port uint32) (net.Listener, error) {
	listener, err := vsock.ListenContextID(cidAny, port, nil)
	if err != nil {
		return nil, fmt.Errorf("listen on the vsock port %d: %w", port, err)
	}
	log := plog.NewDefaultLogger().
		Attr("pkg", "daemon/listener/lsnrhttpvsock").
		Attr("lsnr_type", "vsock").
		Attr("lsnr_addr", addr(port)).
		WithPrefix("daemon: listener: vsock: ")
	return &hostListener{Listener: listener, log: log, reservedOnly: true}, nil
}

// New returns a listener of the vsock port.
func New(port uint32) *T {
	return &T{
		port: port,
		log: plog.NewDefaultLogger().
			Attr("pkg", "daemon/listener/lsnrhttpvsock").
			Attr("lsnr_type", "vsock").
			Attr("lsnr_addr", addr(port)).
			WithPrefix("daemon: listener: vsock: "),
	}
}

func addr(port uint32) string {
	return fmt.Sprintf("vsock:%d", port)
}

func (t *T) Start(ctx context.Context) error {
	ctx = daemonctx.WithLogger(ctx, t.log)
	ctx = daemonctx.WithLsnrType(ctx, "vsock")
	listener, err := vsock.ListenContextID(cidAny, t.port, nil)
	if err != nil {
		return fmt.Errorf("listen on the vsock port %d: %w", t.port, err)
	}
	t.listener = &hostListener{Listener: listener, log: t.log}
	ctx = daemonctx.WithListenAddr(ctx, addr(t.port))
	t.server = lsnrhttpux.NewServer(routehttp.New(ctx, false), golog.New(t.log.Logger(), "", 0))
	t.server.ConnContext = withPeer

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.log.Infof("started")
		if err := t.server.Serve(t.listener); err != nil && !errors.Is(err, http.ErrServerClosed) && !t.stopping.Load() {
			t.log.Errorf("serve: %s", err)
		}
		t.log.Infof("stopped")
	}()
	return nil
}

func (t *T) Stop() error {
	if t.listener == nil {
		return nil
	}
	t.stopping.Store(true)
	err := t.listener.Close()
	if err != nil && !errors.Is(err, net.ErrClosed) {
		t.log.Errorf("listener Close failure: %s", err)
	}
	t.wg.Wait()
	return nil
}

// Accept drops the connections of anything but the hypervisor. The other peers
// have no business here whatever their credential, and a hypervisor enabling
// this listener by mistake serves its guests nothing.
func (t *hostListener) Accept() (net.Conn, error) {
	for {
		c, err := t.Listener.Accept()
		if err != nil {
			return nil, err
		}
		a, ok := c.RemoteAddr().(*vsock.Addr)
		switch {
		case !ok || a.ContextID != vsock.Host:
			t.log.Warnf("drop a connection from %s: not the hypervisor", c.RemoteAddr())
		case t.reservedOnly && a.Port >= reservedPorts:
			t.log.Warnf("drop a connection from %s: not a reserved source port", c.RemoteAddr())
		default:
			return c, nil
		}
		_ = c.Close()
	}
}

// withPeer adds the context id and the source port of the connection c, the
// kernel's, to its context.
func withPeer(ctx context.Context, c net.Conn) context.Context {
	if a, ok := c.RemoteAddr().(*vsock.Addr); ok {
		return daemonctx.WithVsockPeer(ctx, a.ContextID, a.Port)
	}
	return ctx
}
