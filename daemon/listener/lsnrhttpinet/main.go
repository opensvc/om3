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
		t.configWatcher(ctx)
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
		node.LsnrData.Unset(localhost)
		t.bus.Pub(&msgbus.ListenerDeleted{Node: localhost}, labelLocalhost)
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
	}
	t.log.Info().Msg("listener stopped")
}

// configWatcher watch cluster config lsnr port changes to restart
// listener.
func (t *T) configWatcher(ctx context.Context) {
	sub := t.bus.Sub("lsnr-http-inet")
	sub.AddFilter(&msgbus.ClusterConfigUpdated{},
		pubsub.Label{"node", hostname.Hostname()})
	sub.Start()
	defer func() {
		if err := sub.Stop(); err != nil {
			t.log.Error().Err(err).Msg("subscription stop")
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-sub.C:
			switch m := e.(type) {
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
					if err := t.Stop(); err != nil {
						t.log.Error().Err(err).Msg("restarting has stop failure")
					}
					select {
					case <-ctx.Done():
						return
					default:
					}
					t.addr = newAddr
					t.log = log.Logger.With().Str("addr", t.addr).Str("sub", "lsnr-http-inet").Logger()
					errC := make(chan error)
					go t.start(ctx, errC)
					if err := <-errC; err != nil {
						t.log.Error().Err(err).Msg("restarting has start failure")
					}
					t.log.Info().Msgf("restarted on new addr %s", t.addr)
				}
			}
		}
	}
}
