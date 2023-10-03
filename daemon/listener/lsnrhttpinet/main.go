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
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/listener/routehttp"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	T struct {
		bus      *pubsub.Bus
		listener *http.Server
		log      zerolog.Logger
		addr     string
		certFile string
		keyFile  string
		wg       sync.WaitGroup
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("listener funcopt.Apply")
		return nil
	}
	t.log = log.Logger.With().Str("addr", t.addr).Str("sub", "lsnr-http-inet").Logger()
	return t
}

func (t *T) Stop() error {
	t.log.Info().Msg("listener stopping")
	defer t.log.Info().Msg("listener stopped")
	if t.listener == nil {
		return nil
	}
	err := (*t.listener).Close()
	if err != nil {
		t.log.Error().Err(err).Msg("listener close failure")
	}
	t.wg.Wait()
	return err
}

func (t *T) Start(ctx context.Context) error {
	t.bus = pubsub.BusFromContext(ctx)

	go func() {
		t.janitor(ctx)
	}()

	errC := make(chan error)
	go t.start(ctx, errC)
	return <-errC
}

func (t *T) start(ctx context.Context, errC chan<- error) {
	t.wg.Add(1)
	defer t.wg.Done()
	ctx = daemonctx.WithListenAddr(ctx, t.addr)

	t.log.Info().Msg("listener starting")
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
		ErrorLog: golog.New(t.log, "", 0),
	}

	lsnr, err := net.Listen("tcp", t.addr)
	if err != nil {
		t.log.Error().Err(err).Msg("listen failure")
		errC <- err
		return
	}
	defer func(lsnr net.Listener) {
		if err := lsnr.Close(); err != nil && err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
			t.log.Error().Err(err).Msg("listener close failure")
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
	t.log.Info().Msg("listener started")
	errC <- nil
	if err := t.listener.ServeTLS(lsnr, t.certFile, t.keyFile); err != nil {
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			t.log.Debug().Msg("listener ends with expected error")
		} else {
			t.log.Error().Err(err).Msg("listener ends with unexpected error")
		}
	}
	if t.listener != nil {
		if err := t.listener.Close(); err != nil && err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
			t.log.Error().Err(err).Msg("listener close")
		}
		t.log.Info().Msg("listener closed")
	}
}

// janitor watch events that may require a stop, start or restart listener
func (t *T) janitor(ctx context.Context) {
	sub := t.bus.Sub("lsnr-http-inet")
	sub.AddFilter(&msgbus.ClusterConfigUpdated{},
		pubsub.Label{"node", hostname.Hostname()})
	sub.AddFilter(&msgbus.DaemonCtl{}, pubsub.Label{"id", "lsnr-http-inet"})
	sub.Start()
	defer func() {
		if err := sub.Stop(); err != nil {
			t.log.Error().Err(err).Msg("subscription stop")
		}
	}()
	stop := func() {
		t.log.Info().Msg("stopping")
		if err := t.Stop(); err != nil {
			t.log.Error().Err(err).Msg("stop failed")
		}
	}
	start := func() {
		t.log.Info().Msg("starting")
		errC := make(chan error)
		go t.start(ctx, errC)
		if err := <-errC; err != nil {
			t.log.Error().Err(err).Msg("start failed")
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-sub.C:
			switch m := e.(type) {
			case *msgbus.DaemonCtl:
				t.log.Info().Msgf("listener receive a %s order", m.Action)
				switch m.Action {
				case "stop":
					stop()
				case "start":
					start()
				case "restart":
					stop()
					select {
					case <-ctx.Done():
						return
					default:
					}
					start()
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
					t.log.Info().Msgf("listener will restart: addr changed %s -> %s", t.addr, newAddr)
					stop()
					select {
					case <-ctx.Done():
						return
					default:
					}
					t.addr = newAddr
					t.log = log.Logger.With().Str("addr", t.addr).Str("sub", "lsnr-http-inet").Logger()
					start()
					t.log.Info().Msgf("restarted on new addr %s", t.addr)
				}
			}
		}
	}
}
