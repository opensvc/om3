package lsnrhttpinet

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/listener/routehttp"
	"github.com/opensvc/om3/daemon/routinehelper"
	"github.com/opensvc/om3/daemon/subdaemon"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		listener     *http.Server
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
	name := "lsnr-http-inet"
	t.log = log.Logger.With().Str("addr", t.addr).Str("sub", name).Logger()
	t.T = subdaemon.New(
		subdaemon.WithName("lsnr-http-inet"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	return t
}

func (t *T) MainStart(ctx context.Context) error {
	ctx = daemonctx.WithListenAddr(ctx, t.addr)
	started := make(chan error)
	go func() {
		defer t.Trace(t.Name())()
		if err := t.start(ctx); err != nil {
			started <- err
			return
		}
		started <- nil
	}()
	if err := <-started; err != nil {
		return err
	}
	return nil
}

func (t *T) MainStop() error {
	if err := t.stop(); err != nil {
		return err
	}
	return nil
}

func (t *T) stop() error {
	if t.listener == nil {
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
	for _, fname := range []string{t.certFile, t.keyFile} {
		if !file.Exists(fname) {
			return errors.Errorf("can't listen: %s does not exist", fname)
		}
	}
	started := make(chan bool)
	t.listener = &http.Server{
		Addr:    t.addr,
		Handler: routehttp.New(ctx, true),
		TLSConfig: &tls.Config{
			ClientAuth: tls.NoClientCert,
		},
	}
	go func() {
		started <- true
		err := t.listener.ListenAndServeTLS(t.certFile, t.keyFile)
		if err == http.ErrServerClosed || errors.Is(err, net.ErrClosed) {
			t.log.Debug().Err(err).Msg("listener ends with expected error ")
		} else {
			t.log.Error().Err(err).Msg("listener ends with unexpected error ")
		}
		if t.listener != nil {
			if err := t.listener.Close(); err != nil {
				//t.log.Error().Err(err).Msg("listener close")
			}
		}
		t.log.Info().Msg("listener stopped")
	}()
	<-started
	t.log.Info().Msg("listener started ")
	return nil
}
