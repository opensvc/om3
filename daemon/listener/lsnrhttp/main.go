package lsnrhttp

import (
	"net/http"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		listener     *http.Server
		log          zerolog.Logger
		routineTrace routineTracer
		handler      http.Handler
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
	t.T = subdaemon.New(
		subdaemon.WithName("listenerhttp"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	t.log = t.Log()
	return t
}

func (t *T) MainStart() error {
	t.log.Debug().Msg("mgr starting")
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name() + "-lsnr-http")()
		if err := t.start(); err != nil {
			t.log.Error().Err(err).Msg("starting http")
		}
		started <- true
	}()
	<-started
	t.log.Debug().Msg(" started")
	return nil
}

func (t *T) MainStop() error {
	t.log.Debug().Msg("mgr stopping")
	if err := t.stop(); err != nil {
		t.log.Error().Err(err).Msg("stop")
	}
	t.log.Debug().Msg("mgr stopped")
	return nil
}

func (t *T) stop() error {
	if err := (*t.listener).Close(); err != nil {
		t.log.Error().Err(err).Msg("http listener close failed " + t.addr)
		return err
	}
	t.log.Info().Msg("http listener stopped " + t.addr)
	return nil
}

func (t *T) start() error {
	t.log.Info().Msg("http listener starting " + t.addr)
	started := make(chan bool)
	t.listener = &http.Server{Addr: t.addr, Handler: t.handler}
	go func() {
		started <- true
		err := t.listener.ListenAndServeTLS(t.certFile, t.keyFile)
		if err != http.ErrServerClosed {
			t.log.Error().Err(err).Msg("http listener ends with unexpected error " + t.addr)
		}
	}()
	<-started
	t.log.Info().Msg("http listener started " + t.addr)
	return nil
}
