package lsnrhttpinet

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	golog "log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/listener/routehttp"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	T struct {
		bus      *pubsub.Bus
		listener *http.Server
		log      *plog.Logger
		addr     string
		certFile string
		keyFile  string
		wg       sync.WaitGroup
	}
)

func New(ctx context.Context, opts ...funcopt.O) *T {
	t := &T{
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/listener/lsnrhttpinet").Attr("lsnr_type", "inet").WithPrefix("daemon: listener: inet: "),
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Errorf("funcopt apply: %s", err)
		return nil
	}
	t.log = t.log.Attr("lsnr_addr", t.addr)
	return t
}

func (t *T) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	if t.listener == nil {
		return nil
	}
	err := (*t.listener).Close()
	if err != nil {
		t.log.Errorf("listener close failure: %s", err)
	}
	t.wg.Wait()
	return err
}

// Start startup the inet http janitor. janitor startup initial inet http listener.
func (t *T) Start(ctx context.Context) error {
	ctx = daemonctx.WithLsnrType(ctx, "inet")

	t.bus = pubsub.BusFromContext(ctx)

	errC := make(chan error)
	go func(ctx context.Context, errC chan error) {
		t.janitor(ctx, errC)
	}(ctx, errC)

	return <-errC
}

func (t *T) start(ctx context.Context, errC chan<- error) {
	t.wg.Add(1)
	defer t.wg.Done()
	ctx = daemonctx.WithListenAddr(ctx, t.addr)

	t.log.Infof("starting")
	for _, fname := range []string{t.certFile, t.keyFile} {
		if !file.Exists(fname) {
			errC <- fmt.Errorf("can't listen: %s does not exist", fname)
			return
		}
	}
	t.listener = &http.Server{
		Addr:    t.addr,
		Handler: routehttp.New(ctx, true),
		TLSConfig: &tls.Config{
			ClientAuth: tls.NoClientCert,
		},
		ErrorLog: golog.New(t.log.Logger(), "", 0),
	}

	lsnr, err := net.Listen("tcp", t.addr)
	if err != nil {
		t.log.Errorf("listen failure: %s", err)
		errC <- err
		return
	}
	defer func(lsnr net.Listener) {
		if err := lsnr.Close(); err != nil && err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
			t.log.Errorf("listener close failure: %s", err)
		}
	}(lsnr)

	tcpAddr := lsnr.Addr().(*net.TCPAddr)
	port := fmt.Sprintf("%d", tcpAddr.Port)
	addr := tcpAddr.IP.String()

	now := time.Now()
	localhost := hostname.Hostname()
	labelLocalhost := pubsub.Label{"node", localhost}
	node.LsnrData.Set(localhost, &node.Lsnr{Addr: addr, Port: port, UpdatedAt: now})
	t.bus.Pub(&msgbus.ListenerUpdated{Node: localhost, Lsnr: node.Lsnr{Addr: addr, Port: port, UpdatedAt: now}},
		labelLocalhost)
	defer func() {
		now := time.Now()
		node.LsnrData.Set(localhost, &node.Lsnr{UpdatedAt: now})
		t.bus.Pub(&msgbus.ListenerUpdated{Node: localhost, Lsnr: node.Lsnr{UpdatedAt: now}},
			labelLocalhost)
	}()
	t.log.Infof("started")
	errC <- nil
	if err := t.listener.ServeTLS(lsnr, t.certFile, t.keyFile); err != nil {
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			t.log.Debugf("listener serve ends with expected error")
		} else {
			t.log.Errorf("listener serve ends with unexpected error: %s", err)
		}
	}
	if t.listener != nil {
		if err := t.listener.Close(); err != nil && err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
			t.log.Errorf("listener close: %s", err)
		}
		t.log.Infof("listener closed")
	}
}

// janitor startup initial http inet listener, then watch events to stop, start or restart listener.
// events are: DaemonCtl,name=lsnr-http-inet, ClusterConfigUpdated,node=<localhost> with changed lsnr addr or port
// TODO: also watch for tls setting changed
func (t *T) janitor(ctx context.Context, errC chan<- error) {
	var started bool
	sub := t.bus.Sub("daemon.lsnr.http.inet")
	sub.AddFilter(&msgbus.ClusterConfigUpdated{},
		pubsub.Label{"node", hostname.Hostname()})
	sub.AddFilter(&msgbus.DaemonCtl{}, pubsub.Label{"id", "lsnr-http-inet"})
	sub.Start()
	defer func() {
		if err := sub.Stop(); err != nil {
			t.log.Errorf("subscription stop: %s", err)
		}
	}()

	stop := func() {
		t.log.Infof("janitor ask for stop")
		if err := t.Stop(); err != nil {
			t.log.Errorf("stop failed: %s", err)
			return
		}
		started = false
	}

	start := func() error {
		if started {
			err := fmt.Errorf("can't start already started listener")
			t.log.Errorf("start: %s", err)
			return err
		}
		t.log.Infof("janitor ask for start")
		errC := make(chan error)
		go t.start(ctx, errC)
		err := <-errC
		if err != nil {
			t.log.Errorf("start failed: %s", err)
			return err
		}
		started = true
		return nil
	}

	errC <- start()

	for {
		select {
		case <-ctx.Done():
			return
		case e := <-sub.C:
			switch m := e.(type) {
			case *msgbus.DaemonCtl:
				t.log.Infof("daemon control %s asked", m.Action)
				switch m.Action {
				case "stop":
					stop()
				case "start":
					if err := start(); err != nil {
						t.log.Errorf("on daemon control %s start failed: %s", m.Action, err)
					}
				case "restart":
					stop()
					select {
					case <-ctx.Done():
						return
					default:
					}
					if err := start(); err != nil {
						t.log.Errorf("on daemon control %s start failed: %s", m.Action, err)
					}
				}
			case *msgbus.ClusterConfigUpdated:
				select {
				case <-ctx.Done():
					return
				default:
				}
				clusterConfig := m.Value
				newAddr := fmt.Sprintf("%s:%d", clusterConfig.Listener.Addr, clusterConfig.Listener.Port)
				if t.addr != newAddr {
					t.log.Infof("will restart: addr changed %s -> %s", t.addr, newAddr)
					stop()
					select {
					case <-ctx.Done():
						return
					default:
					}
					t.addr = newAddr

					t.log = plog.NewDefaultLogger().
						Attr("pkg", "daemon/listener/lsnrhttpinet").
						Attr("lsnr_type", "inet").
						Attr("lsnr_addr", t.addr).
						WithPrefix("daemon: listener: inet: ")
					if err := start(); err != nil {
						t.log.Errorf("on addr changed start failed: %s", err)
					}
					t.log.Infof("restarted on new addr %s", t.addr)
				}
			}
		}
	}
}
