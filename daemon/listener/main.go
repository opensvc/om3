package listener

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/enable"
	"github.com/opensvc/om3/daemon/listener/lsnrhttpinet"
	"github.com/opensvc/om3/daemon/listener/lsnrhttpux"
	"github.com/opensvc/om3/daemon/listener/routehttp"
	"github.com/opensvc/om3/daemon/routinehelper"
	"github.com/opensvc/om3/daemon/subdaemon"
	"github.com/opensvc/om3/util/funcopt"
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

	// authOption implements interfaces for daemonauth.Init
	authOption struct {
		*ccfg.NodeDB
		*object.UsrDB
	}
)

func (a *authOption) ListenAddr(ctx context.Context) string {
	return daemonctx.ListenAddr(ctx)
}

func (a *authOption) X509CACertFile() string {
	return daemonenv.CAsCertFile()
}

func (a *authOption) SignKeyFile() string {
	return daemonenv.CAKeyFile()
}

func (a *authOption) VerifyKeyFile() string {
	return daemonenv.CAsCertFile()
}

func getMandatorySub() map[string]sub {
	clusterConfig := ccfg.Get()
	subs := make(map[string]sub)
	subs["listenerHttpUx"] = sub{
		new: func(t *T) subdaemon.Manager {
			return lsnrhttpux.New(
				lsnrhttpux.WithRoutineTracer(&t.TT),
				lsnrhttpux.WithAddr(daemonenv.PathUxHttp()),
				lsnrhttpux.WithCertFile(daemonenv.CertChainFile()),
				lsnrhttpux.WithKeyFile(daemonenv.KeyFile()),
			)
		},
	}
	if clusterConfig.Listener.Port > 0 {
		subs["listenerHttpInet"] = sub{
			new: func(t *T) subdaemon.Manager {
				return lsnrhttpinet.New(
					lsnrhttpinet.WithRoutineTracer(&t.TT),
					lsnrhttpinet.WithAddr(fmt.Sprintf("%s:%d", clusterConfig.Listener.Addr, clusterConfig.Listener.Port)),
					lsnrhttpinet.WithCertFile(daemonenv.CertChainFile()),
					lsnrhttpinet.WithKeyFile(daemonenv.KeyFile()),
				)
			},
		}
	}
	return subs
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
	if err := startCertFS(); err != nil {
		t.log.Err(err).Msgf("start certificates volatile fs")
	}
	if strategies, err := daemonauth.InitStategies(&authOption{}); err != nil {
		return err
	} else {
		ctx = context.WithValue(ctx, "authStrategies", strategies)
		ctx = context.WithValue(ctx, "JWTCreator", &daemonauth.JWTCreator{})
	}
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
