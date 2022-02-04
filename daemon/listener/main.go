package listener

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/enable"
	"opensvc.com/opensvc/daemon/listener/lsnrhttp"
	"opensvc.com/opensvc/daemon/listener/lsnrraw"
	"opensvc.com/opensvc/daemon/listener/mux/httpmux"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		log          zerolog.Logger
		loopC        chan action
		loopDelay    time.Duration
		loopEnabled  *enable.T
		routineTrace routineTracer
		rootDaemon   subdaemon.RootManager
		httpHandler  http.Handler
		routinehelper.TT
	}
	action struct {
		do   string
		done chan string
	}
	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}

	sub struct {
		new        func(t *T) subdaemon.Manager
		subActions subdaemon.Manager
	}
)

var (
	mandatorySubs = map[string]sub{
		"listenerRaw": {
			new: func(t *T) subdaemon.Manager {
				return lsnrraw.New(
					lsnrraw.WithRoutineTracer(&t.TT),
					lsnrraw.WithHttpHandler(t.httpHandler),
					lsnrraw.WithAddr(socketPathUds),
				)
			},
		},
		"listenerHttp": {
			new: func(t *T) subdaemon.Manager {
				return lsnrhttp.New(
					lsnrhttp.WithRoutineTracer(&t.TT),
					lsnrhttp.WithHandler(t.httpHandler),
					lsnrhttp.WithAddr(":1225"),
					lsnrhttp.WithCertFile("/tmp/certificate_chain"),
					lsnrhttp.WithKeyFile("/tmp/private_key"),
				)
			},
		},
	}

	socketPathUds = "/tmp/lsnr_ux"
)

func New(opts ...funcopt.O) *T {
	t := &T{
		loopDelay:   1 * time.Second,
		loopEnabled: enable.New(),
	}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("listener funcopt.Apply")
		return nil
	}
	t.T = subdaemon.New(
		subdaemon.WithName("listener"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	t.log = t.Log()
	t.loopC = make(chan action)
	return t
}

func (t *T) MainStart() error {
	t.log.Info().Msg("mgr starting")
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name() + "-loop")()
		t.loop(started)
	}()
	t.httpHandler = httpmux.New(t.log, t.rootDaemon)
	for subName, sub := range mandatorySubs {
		sub.subActions = sub.new(t)
		if err := sub.subActions.Init(); err != nil {
			t.log.Err(err).Msgf("%s Init", subName)
			return err
		}
		if err := t.Register(sub.subActions); err != nil {
			t.log.Err(err).Msgf("%s register", subName)
			return err
		}
		if err := sub.subActions.Start(); err != nil {
			t.log.Err(err).Msgf("%s start", subName)
			return err
		}
	}
	<-started
	t.log.Info().Msg("mgr started")
	return nil
}

func (t *T) MainStop() error {
	t.log.Info().Msg("mgr stopping")
	if t.loopEnabled.Enabled() {
		done := make(chan string)
		t.loopC <- action{"stop", done}
		<-done
	}
	t.log.Info().Msg("mgr stopped")
	return nil
}

func (t *T) loop(c chan bool) {
	t.log.Info().Msg("loop started")
	t.loopEnabled.Enable()
	t.aLoop()
	c <- true
	for {
		select {
		case a := <-t.loopC:
			t.loopEnabled.Disable()
			t.log.Info().Msg("loop stopped")
			a.done <- "loop stopped"
			return
		case <-time.After(t.loopDelay):
			t.aLoop()
		}
	}
}

func (t *T) aLoop() {
	t.log.Debug().Msg("loop")
}
