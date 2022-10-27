package lsnrhttpux

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/listener/routehttp"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		listener     *net.Listener
		log          zerolog.Logger
		routineTrace routineTracer
		addr         string
		certFile     string
		keyFile      string
	}

	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("listener funcopt.Apply")
		return nil
	}
	name := "lsnr-http-ux"
	t.log = log.Logger.With().Str("addr", t.addr).Str("sub", name).Logger()
	t.T = subdaemon.New(
		subdaemon.WithName(name),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	return t
}

func (t *T) MainStart(ctx context.Context) error {
	ctx = daemonctx.WithListenAddr(ctx, t.addr)
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name())()
		if err := t.start(ctx); err != nil {
			t.log.Error().Err(err).Msg("mgr start failure")
		}
		started <- true
	}()
	<-started
	return nil
}

func (t *T) MainStop() error {
	if err := t.stop(); err != nil {
		t.log.Error().Err(err).Msg("mgr stop failure")
	}
	return nil
}

func (t *T) stop() error {
	if t.listener == nil {
		t.log.Info().Msg("listener already closed")
		return nil
	}
	if err := (*t.listener).Close(); err != nil {
		t.log.Error().Err(err).Msg("listener Close failure")
		return err
	}
	t.log.Info().Msg("listener closed")
	return nil
}

func (t *T) start(ctx context.Context) error {
	t.log.Info().Msg("listener starting")
	if err := os.RemoveAll(t.addr); err != nil {
		t.log.Error().Err(err).Msg("RemoveAll")
		return err
	}
	started := make(chan bool)
	s := &http2.Server{}
	server := http.Server{
		Handler: h2c.NewHandler(routehttp.New(ctx, false), s),
	}
	listener, err := net.Listen("unix", t.addr)
	if err != nil {
		t.log.Error().Err(err).Msg("listen failed")
		return err
	}
	t.listener = &listener

	go func() {
		started <- true
		err = server.Serve(listener)
		if err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
			t.log.Debug().Err(err).Msg("http listener ends with unexpected error")
		}
		t.log.Info().Msg("listener stopped")
	}()
	<-started
	t.log.Info().Msg("listener started ")
	return nil
}
