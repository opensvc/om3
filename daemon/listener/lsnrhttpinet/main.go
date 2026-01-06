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

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/opensvc/om3/v3/daemon/daemonapi"
	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/listener/routehttp"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	T struct {
		publisher pubsub.Publisher
		listener  *http.Server
		log       *plog.Logger
		addr      string
		certFile  string
		keyFile   string
		wg        sync.WaitGroup
		status    daemonsubsystem.Listener

		labelLocalhost pubsub.Label
		localhost      string
	}
)

func New(ctx context.Context, opts ...funcopt.O) *T {
	localhost := hostname.Hostname()
	t := &T{
		log: plog.NewDefaultLogger().
			Attr("pkg", "daemon/listener/lsnrhttpinet").
			Attr("lsnr_type", "inet").
			WithPrefix("daemon: listener: inet: "),

		status: daemonsubsystem.Listener{Status: daemonsubsystem.Status{CreatedAt: time.Now()}},

		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
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

	t.publisher = pubsub.PubFromContext(ctx)

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
	ctx = daemonctx.WithListenRateLimiterMemoryStoreConfig(ctx, rate.Limit(200), 1000, 3*time.Second)

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
			ClientAuth: tls.RequestClientCert,
			MinVersion: tls.VersionTLS13,
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP521,
				tls.CurveP384,
				tls.CurveP256,
			},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		},
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		ErrorLog:     golog.New(t.log.Logger(), "", 0),
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
	t.status.UpdatedAt = now
	t.status.ConfiguredAt = now
	t.status.State = "running"
	t.status.Addr = addr
	t.status.Port = port

	t.publish()
	defer func() {
		now := time.Now()
		t.status.State = "stopped"
		t.status.Port = ""
		t.status.Addr = ""
		t.status.UpdatedAt = now
		t.publish()
	}()
	t.log.Infof("started")
	errC <- nil
	if err := t.listener.ServeTLS(lsnr, t.certFile, t.keyFile); err != nil {
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			t.log.Tracef("listener serve ends with expected error")
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
	sub := pubsub.SubFromContext(ctx, "daemon.lsnr.http.inet")
	sub.AddFilter(&msgbus.ClusterConfigUpdated{}, t.labelLocalhost)
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
				case "log-level-panic":
					t.log.Level(zerolog.PanicLevel)
					daemonapi.LogLevel = zerolog.PanicLevel
				case "log-level-fatal":
					t.log.Level(zerolog.FatalLevel)
					daemonapi.LogLevel = zerolog.FatalLevel
				case "log-level-error":
					t.log.Level(zerolog.ErrorLevel)
					daemonapi.LogLevel = zerolog.ErrorLevel
				case "log-level-warn":
					t.log.Level(zerolog.WarnLevel)
					daemonapi.LogLevel = zerolog.WarnLevel
				case "log-level-info":
					t.log.Level(zerolog.InfoLevel)
					daemonapi.LogLevel = zerolog.InfoLevel
				case "log-level-debug":
					t.log.Level(zerolog.DebugLevel)
					daemonapi.LogLevel = zerolog.DebugLevel
				case "log-level-trace":
					t.log.Level(zerolog.TraceLevel)
					daemonapi.LogLevel = zerolog.TraceLevel
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

func (t *T) publish() {
	daemonsubsystem.DataListener.Set(t.localhost, t.status.DeepCopy())
	t.publisher.Pub(&msgbus.DaemonListenerUpdated{Node: t.localhost, Value: *t.status.DeepCopy()}, t.labelLocalhost)
}
