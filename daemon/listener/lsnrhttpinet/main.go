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
	errC := make(chan error)

	t.wg.Add(1)
	go func(errC chan<- error) {
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
			if err := lsnr.Close(); err != nil {
				t.log.Error().Err(err).Msg("listener close failure")
			}
		}(lsnr)

		bus := pubsub.BusFromContext(ctx)
		tcpAddr := lsnr.Addr().(*net.TCPAddr)
		port := fmt.Sprintf("%d", tcpAddr.Port)
		addr := tcpAddr.IP.String()
		localhost := hostname.Hostname()
		labels := []pubsub.Label{{"node", localhost}}
		node.LsnrData.Set(localhost, &node.Lsnr{Addr: addr, Port: port})
		bus.Pub(&msgbus.ListenerUpdated{Node: localhost, Lsnr: node.Lsnr{Addr: addr, Port: port}}, labels...)
		defer func() {
			node.LsnrData.Unset(localhost)
			bus.Pub(&msgbus.ListenerDeleted{Node: localhost}, labels...)
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
			if err := t.listener.Close(); err != nil {
				//t.log.Error().Err(err).Msg("listener close")
			}
		}
		t.log.Info().Msg("listener stopped")
	}(errC)

	return <-errC
}
