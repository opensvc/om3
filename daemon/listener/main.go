package listener

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/daemon/daemonauth"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/daemon/enable"
	"opensvc.com/opensvc/daemon/listener/lsnrhttpinet"
	"opensvc.com/opensvc/daemon/listener/lsnrhttpux"
	"opensvc.com/opensvc/daemon/listener/lsnrrawinet"
	"opensvc.com/opensvc/daemon/listener/lsnrrawux"
	"opensvc.com/opensvc/daemon/listener/routehttp"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/key"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		log          zerolog.Logger
		loopC        chan action
		loopDelay    time.Duration
		loopEnabled  *enable.T
		routineTrace routineTracer
		rootDaemon   subdaemon.RootManager
		httpHandler  http.Handler
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

func getMandatorySub() map[string]sub {
	return map[string]sub{
		"listenerRaw": {
			new: func(t *T) subdaemon.Manager {
				return lsnrrawux.New(
					lsnrrawux.WithRoutineTracer(&t.TT),
					lsnrrawux.WithAddr(daemonenv.PathUxRaw()),
				)
			},
		},
		"listenerRawInet": {
			new: func(t *T) subdaemon.Manager {
				return lsnrrawinet.New(
					lsnrrawinet.WithRoutineTracer(&t.TT),
					lsnrrawinet.WithAddr(fmt.Sprintf(":%d", daemonenv.RawPort)),
				)
			},
		},
		"listenerHttpInet": {
			new: func(t *T) subdaemon.Manager {
				return lsnrhttpinet.New(
					lsnrhttpinet.WithRoutineTracer(&t.TT),
					lsnrhttpinet.WithAddr(fmt.Sprintf(":%d", daemonenv.HttpPort)),
					lsnrhttpinet.WithCertFile(daemonenv.CertChainFile()),
					lsnrhttpinet.WithKeyFile(daemonenv.KeyFile()),
				)
			},
		},
		"listenerHttpUx": {
			new: func(t *T) subdaemon.Manager {
				return lsnrhttpux.New(
					lsnrhttpux.WithRoutineTracer(&t.TT),
					lsnrhttpux.WithAddr(daemonenv.PathUxHttp()),
					lsnrhttpux.WithCertFile(daemonenv.CertChainFile()),
					lsnrhttpux.WithKeyFile(daemonenv.KeyFile()),
				)
			},
		},
	}
}

func New(opts ...funcopt.O) *T {
	t := &T{
		loopDelay:   1 * time.Second,
		loopEnabled: enable.New(),
		log:         log.Logger.With().Str("name", "listener").Logger(),
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

func (t *T) MainStart(ctx context.Context) error {
	node, err := object.NewNode()
	if err != nil {
		return err
	}
	if err := startCertFS(); err != nil {
		t.log.Err(err).Msgf("start certificates volatile fs")
	}
	if err := daemonauth.Init(); err != nil {
		return err
	}
	daemonenv.HttpPort = node.Config().GetInt(key.New("listener", "tls_port"))
	daemonenv.RawPort = node.Config().GetInt(key.New("listener", "port"))
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name() + "-loop")()
		defer func() {
			_ = stopCertFS()
		}()
		//defer t.cancel()
		started <- true
		t.loop(ctx)
	}()
	t.httpHandler = routehttp.New(ctx, false)
	for subName, sub := range getMandatorySub() {
		sub.subActions = sub.new(t)
		if err := t.Register(sub.subActions); err != nil {
			t.log.Err(err).Msgf("%s register", subName)
			return err
		}
		if err := sub.subActions.Start(ctx); err != nil {
			t.log.Err(err).Msgf("%s start", subName)
			return err
		}
	}
	<-started
	return nil
}

func (t *T) MainStop() error {
	//t.cancel()
	return nil
}

func (t *T) loop(ctx context.Context) {
	t.log.Info().Msg("loop started")
	t.aLoop()
	ticker := time.NewTicker(t.loopDelay)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.log.Info().Msg("loop stopped")
			return
		case <-ticker.C:
			t.aLoop()
		}
	}
}

func (t *T) aLoop() {
}
