package lsnrhttpux

import (
	"context"
	"errors"
	golog "log"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/listener/routehttp"
	"github.com/opensvc/om3/util/funcopt"
)

type (
	T struct {
		listener *net.Listener
		log      zerolog.Logger
		addr     string
		wg       sync.WaitGroup
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("listener funcopt.Apply")
		return nil
	}
	t.log = log.Logger.With().Str("addr", t.addr).Str("sub", "lsnr-http-ux").Logger()
	return t
}

func (t *T) Start(ctx context.Context) error {
	errC := make(chan error)
	t.log.Info().Msg("listener starting")
	if err := os.RemoveAll(t.addr); err != nil {
		t.log.Error().Err(err).Msg("RemoveAll")
		return err
	}
	if listener, err := net.Listen("unix", t.addr); err != nil {
		t.log.Error().Err(err).Msg("listen failed")
		return err
	} else {
		t.listener = &listener
	}
	t.wg.Add(1)
	go func(errC chan<- error) {
		defer t.wg.Done()
		ctx = daemonctx.WithListenAddr(ctx, t.addr)

		s := &http2.Server{}
		server := http.Server{
			Handler:  h2c.NewHandler(routehttp.New(ctx, false), s),
			ErrorLog: golog.New(t.log, "", 0),
		}
		t.log.Info().Msg("listener started")
		errC <- nil
		if err := server.Serve(*t.listener); err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
			t.log.Debug().Err(err).Msg("http listener ends with unexpected error")
		}
		t.log.Info().Msg("listener stopped")
	}(errC)

	return <-errC
}

func (t *T) Stop() error {
	t.log.Info().Msgf("listener stopping %s", t.addr)
	defer t.log.Info().Msgf("listener stopped %s", t.addr)
	if t.listener == nil {
		t.log.Info().Msg("listener already closed")
		return nil
	}
	err := (*t.listener).Close()
	if err != nil {
		t.log.Error().Err(err).Msg("listener Close failure")
	}
	t.wg.Wait()
	return err
}
