package lsnrhttpux

import (
	"context"
	"errors"
	golog "log"
	"net"
	"net/http"
	"os"
	"sync"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/listener/routehttp"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/peercred"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	T struct {
		listener *net.Listener
		log      *plog.Logger
		addr     string
		wg       sync.WaitGroup
		server   *http.Server
		peerCred bool

		// ownerGroup returns the group owning the socket, nil for the
		// root socket, and group is the one applied.
		ownerGroup func() string
		group      string
		gid        int
	}
)

func New(ctx context.Context, opts ...funcopt.O) *T {
	t := &T{
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/listener/lsnrhttpux").Attr("lsnr_type", "ux").WithPrefix("daemon: listener: ux: "),
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Errorf("funcopt apply: %s", err)
		return nil
	}
	t.log = t.log.Attr("lsnr_addr", t.addr)
	return t
}

func (t *T) Start(ctx context.Context) error {
	ctx = daemonctx.WithLogger(ctx, t.log)
	ctx = daemonctx.WithLsnrType(ctx, "ux")

	errC := make(chan error)
	t.log.Tracef("starting")
	if err := os.RemoveAll(t.addr); err != nil {
		t.log.Errorf("remove file: %s", err)
		return err
	}
	if t.ownerGroup != nil {
		if err := t.applyOwnerGroup(ctx); err != nil {
			t.log.Errorf("own the socket directory: %s", err)
			return err
		}
	}
	if listener, err := net.Listen("unix", t.addr); err != nil {
		t.log.Errorf("listen failed: %s", err)
		return err
	} else {
		t.listener = &listener
	}
	if t.ownerGroup != nil {
		if err := ownSocket(t.addr, t.gid); err != nil {
			t.log.Errorf("own the socket: %s", err)
			_ = (*t.listener).Close()
			t.listener = nil
			return err
		}
		t.log.Infof("socket owned by the group %s (%d)", t.group, t.gid)
	}
	ctx = daemonctx.WithListenAddr(ctx, t.addr)

	t.wg.Add(1)
	go t.serve(ctx, errC)

	t.wg.Add(1)
	go t.janitor(ctx, errC)

	return <-errC
}

func (t *T) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	if t.listener == nil {
		t.log.Infof("listener already closed")
		return nil
	}
	err := (*t.listener).Close()
	if err != nil {
		t.log.Errorf("listener Close failure: %s", err)
	}
	t.wg.Wait()
	return err
}

func (t *T) serve(ctx context.Context, errC chan<- error) {
	defer t.wg.Done()

	t.server = newServer(routehttp.New(ctx, false), golog.New(t.log.Logger(), "", 0))
	if t.peerCred {
		t.server.ConnContext = t.withPeer
	}
	t.log.Infof("started")
	errC <- nil
	if err := t.server.Serve(*t.listener); err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
		t.log.Tracef("serve ends with unexpected error: %s", err)
	}
	t.log.Infof("stopped")
}

// withPeer adds the peer of the connection c to its context. The credentials
// are the kernel's, recorded at connect: a connection whose peer can not be
// read carries none, and is authenticated as nobody.
func (t *T) withPeer(ctx context.Context, c net.Conn) context.Context {
	peer, err := peercred.FromConn(c)
	if err != nil {
		t.log.Warnf("read the peer of a connection: %s", err)
		return ctx
	}
	return daemonctx.WithPeer(ctx, peer)
}

// newServer returns the server of the ux listener, serving handler over
// http/1.1 and cleartext http/2.
//
// The socket is root's, but a client stalling its headers, or keeping an idle
// connection, holds a goroutine of the daemon all the same: both are timed.
// The http/2 connections are read by the http/2 server once the http/1.1
// server read their preface, so it is timed too.
func newServer(handler http.Handler, errorLog *golog.Logger) *http.Server {
	s := &http2.Server{IdleTimeout: daemonenv.HTTPIdleTimeout}
	return &http.Server{
		Handler:           h2c.NewHandler(handler, s),
		ErrorLog:          errorLog,
		ReadHeaderTimeout: daemonenv.HTTPReadHeaderTimeout,
		IdleTimeout:       daemonenv.HTTPIdleTimeout,
	}
}

// janitor watches the events the ux listener acts on, which are the audit
// sessions starting and stopping.
//
// It subscribes to no daemon control message: this listener has no start,
// stop or restart of its own, it lives as long as the daemon does, and the
// api refuses those actions rather than publishing a message it would
// receive and ignore.
func (t *T) janitor(ctx context.Context, errC chan<- error) {
	defer t.wg.Done()
	name := "daemon.lsnr.http.ux"
	if t.ownerGroup != nil {
		name = "daemon.lsnr.http.uxuser"
	}
	sub := pubsub.SubFromContext(ctx, name)
	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})
	if t.ownerGroup != nil {
		sub.AddFilter(&msgbus.ClusterConfigUpdated{}, pubsub.Label{"node", hostname.Hostname()})
	}
	sub.Start()
	defer func() {
		if err := sub.Stop(); err != nil {
			t.log.Errorf("subscription stop: %s", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case e := <-sub.C:
			switch m := e.(type) {
			case *msgbus.AuditStart:
				t.log.HandleAuditStart(m.Q, m.Subsystems, daemonenv.ListenerNameFamily, daemonenv.ListenerNameUX)
			case *msgbus.AuditStop:
				t.log.HandleAuditStop(m.Q, m.Subsystems, daemonenv.ListenerNameFamily, daemonenv.ListenerNameUX)
			case *msgbus.ClusterConfigUpdated:
				// A group emptied is applied at the next daemon
				// start: the socket stays owned by the former one.
				if t.ownerGroup() == "" {
					continue
				}
				if err := t.applyOwnerGroup(ctx); err != nil {
					t.log.Errorf("own the socket: %s", err)
				}
			}
		}
	}
}
