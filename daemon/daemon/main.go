/*
Package daemon provide the subdaemon main responsible ot other opensvc daemons

It is responsible for other sub daemons (listener, discover, scheduler, hb...)
*/
package daemon

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/retailnext/cannula"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/soellman/pidfile"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/cstat"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/discover"
	"github.com/opensvc/om3/daemon/dns"
	"github.com/opensvc/om3/daemon/enable"
	"github.com/opensvc/om3/daemon/hb"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/istat"
	"github.com/opensvc/om3/daemon/listener"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/nmon"
	"github.com/opensvc/om3/daemon/routinehelper"
	"github.com/opensvc/om3/daemon/scheduler"
	"github.com/opensvc/om3/daemon/subdaemon"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/version"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		ctx    context.Context
		cancel context.CancelFunc
		log    zerolog.Logger
		loopC  chan action

		// loopDelay is the interval of sub... updates
		loopDelay time.Duration

		loopEnabled *enable.T
		cancelFuncs []context.CancelFunc
	}
	action struct {
		do   string
		done chan string
	}
)

var (
	profiling = true
)

func New(opts ...funcopt.O) *T {
	t := &T{
		loopDelay:   1 * time.Second,
		loopEnabled: enable.New(),
		log:         log.Logger,
	}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		return nil
	}
	t.T = subdaemon.New(
		subdaemon.WithName("root"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	t.cancelFuncs = make([]context.CancelFunc, 0)
	t.loopC = make(chan action)
	return t
}

// RunDaemon starts main daemon
func RunDaemon(opts ...funcopt.O) (*T, error) {
	if profiling {
		go startProfiling()
	}

	main := New(opts...)
	ctx := context.Background()
	if err := main.Start(ctx); err != nil {
		main.log.Error().Err(err).Msg("daemon Start")
		return main, err
	}
	return main, nil
}

// MainStart starts loop, mandatory subdaemons
func (t *T) MainStart(ctx context.Context) error {
	daemonPidFile := DaemonPidFile()
	if err := pidfile.WriteControl(daemonPidFile, os.Getpid(), true); err != nil {
		return nil
	}
	t.cancelFuncs = append(t.cancelFuncs, func() {
		if err := os.Remove(DaemonPidFile()); err != nil {
			t.log.Error().Err(err).Msg("remove pid file")
		}
	})

	signal.Ignore(syscall.SIGHUP)
	notifyCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	t.ctx = ctx
	started := make(chan bool)
	t.Add(1)
	go func() {
		defer t.Trace(t.Name() + "-loop")()
		defer t.Done()
		started <- true
		t.loop()
	}()

	bus := pubsub.NewBus("daemon")
	bus.SetDrainChanDuration(3 * daemonenv.DrainChanDuration)
	bus.Start(t.ctx)
	t.cancelFuncs = append(t.cancelFuncs, func() {
		t.log.Debug().Msg("stop daemon pubsub bus")
		bus.Stop()
	})
	t.ctx = pubsub.ContextWithBus(t.ctx, bus)

	localhost := hostname.Hostname()

	go func(ctx context.Context) {
		labels := []pubsub.Label{
			{"node", localhost},
			{"bus", bus.Name()},
		}
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		defer stop()
		for {
			select {
			case <-ticker.C:
				bus.Pub(&msgbus.WatchDog{Bus: bus.Name()}, labels...)
			case <-notifyCtx.Done():
				t.Stop()
				return
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	t.ctx = daemonctx.WithDaemon(t.ctx, t)

	hbcache.Start(t.ctx, 2*daemonenv.DrainChanDuration)

	dataCmd, dataMsgRecvQ, dataCmdCancel := daemondata.Start(t.ctx, daemonenv.DrainChanDuration)
	t.cancelFuncs = append(t.cancelFuncs, func() {
		t.log.Debug().Msg("stop daemon data")
		dataCmdCancel()
	})
	t.ctx = daemondata.ContextWithBus(t.ctx, dataCmd)
	t.ctx = daemonctx.WithHBRecvMsgQ(t.ctx, dataMsgRecvQ)

	<-started

	if err := ccfg.Start(t.ctx, daemonenv.DrainChanDuration); err != nil {
		return err
	}
	if err := cstat.Start(t.ctx); err != nil {
		return err
	}

	if err := istat.Start(t.ctx); err != nil {
		return err
	}

	if ccfg.Get().Name == "" {
		panic("cluster name read from ccfg is empty")
	}
	lsnr := listener.New(listener.WithRoutineTracer(&t.TT))
	if err := t.Register(lsnr); err != nil {
		return err
	}
	if err := lsnr.Start(t.ctx); err != nil {
		return err
	}

	cancelNMon, err := nmon.Start(t.ctx, daemonenv.DrainChanDuration)
	if err != nil {
		return err
	}
	t.cancelFuncs = append(t.cancelFuncs, func() {
		t.log.Debug().Msg("stop nmon")
		cancelNMon()
		t.log.Debug().Msg("stopped nmon")
	})

	if err := dns.Start(t.ctx, daemonenv.DrainChanDuration); err != nil {
		return err
	}

	cancelDiscover, err := discover.Start(t.ctx, daemonenv.DrainChanDuration)
	if err != nil {
		return err
	}
	t.cancelFuncs = append(t.cancelFuncs, func() {
		t.log.Debug().Msg("stop daemon discover")
		cancelDiscover()
		t.log.Debug().Msg("stopped daemon discover")
	})

	for _, sub := range []subdaemon.Manager{
		hb.New(hb.WithRoutineTracer(&t.TT), hb.WithRootDaemon(t)),
		scheduler.New(scheduler.WithRoutineTracer(&t.TT)),
	} {
		if err := t.Register(sub); err != nil {
			return err
		}
		if err := sub.Start(t.ctx); err != nil {
			return err
		}
	}

	bus.Pub(&msgbus.DaemonStart{Node: localhost, Version: version.Version()})
	return nil
}

func (t *T) MainStop() error {
	// stop goroutines without cancel context
	for i := len(t.cancelFuncs) - 1; i >= 0; i-- {
		t.cancelFuncs[i]()
	}

	// goroutines started by MainStart are stopped by the context cancel
	return nil
}

func (t *T) loop() {
	t.log.Info().Msg("loop started")
	t.loopEnabled.Enable()
	ticker := time.NewTicker(t.loopDelay)
	defer ticker.Stop()
	t.aLoop()
	for {
		select {
		case <-ticker.C:
			t.aLoop()
		case <-t.ctx.Done():
			return
		}
	}
}

func (t *T) aLoop() {
}

func startProfiling() {
	// Starts pprof listener on lsnr/profile.sock to allow profiling without auth
	// for local root user on node
	//
	// Usage example from client node:
	//    $ nohup ssh -L 9090:/var/lib/opensvc/lsnr/profile.sock node1 'sleep 35' >/dev/null 2>&1 </dev/null &
	//    $ pprof -http=: opensvc http://localhost:9090/debug/pprof/profile
	//
	// Usage example from cluster node1:
	//    $ curl -o profile.out --unix-socket /var/lib/opensvc/lsnr/profile.sock http://localhost/debug/pprof/profile
	//    $ pprof opensvc profile.out
	cannula.Start(daemonenv.PathUxProfile())
}

func DaemonPidFile() string {
	return filepath.Join(rawconfig.Paths.Var, "osvcd.pid")
}
