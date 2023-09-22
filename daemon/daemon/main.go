/*
Package daemon provide the subdaemon main responsible ot other opensvc daemons

It is responsible for other sub daemons (listener, discover, scheduler, hb...)
*/
package daemon

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/retailnext/cannula"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/omcrypto"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/cstat"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/daemonsys"
	"github.com/opensvc/om3/daemon/discover"
	"github.com/opensvc/om3/daemon/dns"
	"github.com/opensvc/om3/daemon/enable"
	"github.com/opensvc/om3/daemon/hb"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/istat"
	"github.com/opensvc/om3/daemon/listener"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/nmon"
	"github.com/opensvc/om3/daemon/scheduler"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/version"
)

type (
	T struct {
		ctx    context.Context
		cancel context.CancelFunc
		log    zerolog.Logger
		loopC  chan action

		// loopDelay is the interval of sub... updates
		loopDelay time.Duration

		loopEnabled *enable.T
		cancelFuncs []context.CancelFunc
		wg          sync.WaitGroup
	}
	action struct {
		do   string
		done chan string
	}

	startStopper interface {
		Start(ctx context.Context) error
		Stop() error
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
	if err := funcopt.Apply(t, opts...); err != nil {
		return nil
	}
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
func (t *T) Start(ctx context.Context) error {
	t.log.Info().Msg("daemon starting")
	t.ctx, t.cancel = context.WithCancel(ctx)
	signal.Ignore(syscall.SIGHUP)
	notifyCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	go t.notifyWatchDog(ctx)

	bus := pubsub.NewBus("daemon")
	bus.SetDefaultSubscriptionQueueSize(200)
	bus.SetDrainChanDuration(3 * daemonenv.DrainChanDuration)
	bus.Start(t.ctx)
	t.cancelFuncs = append(t.cancelFuncs, func() {
		t.log.Debug().Msg("stop daemon pubsub bus")
		bus.Stop()
	})
	t.ctx = pubsub.ContextWithBus(t.ctx, bus)
	localhost := hostname.Hostname()

	subStarted := make(chan bool)
	t.wg.Add(1)
	go func(ctx context.Context, started chan<- bool) {
		defer t.wg.Done()
		mainSub := bus.Sub("main")
		mainSub.AddFilter(&msgbus.DaemonCtl{}, pubsub.Label{"node", localhost}, pubsub.Label{"id", "daemon"})
		mainSub.Start()
		defer mainSub.Stop()
		labels := []pubsub.Label{
			{"node", localhost},
			{"bus", bus.Name()},
		}
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		defer stop()
		subStarted <- true
		for {
			select {
			case <-ticker.C:
				bus.Pub(&msgbus.WatchDog{Bus: bus.Name()}, labels...)
			case <-notifyCtx.Done():
				t.Stop()
				return
			case <-ctx.Done():
				return
			case i := <-mainSub.C:
				switch m := i.(type) {
				case *msgbus.DaemonCtl:
					t.log.Info().Msgf("daemon ctl received %v", m)
					if m.Action == "stop" {
						t.log.Info().Msg("daemon ctl received, daemon will be stopped")
						if err := t.Stop(); err != nil {
							t.log.Error().Err(err).Msg("daemon ctl failed to stop daemon")
						}
						return
					}
				}
			}
		}
	}(ctx, subStarted)
	<-subStarted

	hbcache.Start(t.ctx, 2*daemonenv.DrainChanDuration)

	dataCmd, dataMsgRecvQ, dataCmdCancel := daemondata.Start(t.ctx, daemonenv.DrainChanDuration)
	t.cancelFuncs = append(t.cancelFuncs, func() {
		t.log.Debug().Msg("stop daemon data")
		dataCmdCancel()
	})
	t.ctx = daemondata.ContextWithBus(t.ctx, dataCmd)
	t.ctx = daemonctx.WithHBRecvMsgQ(t.ctx, dataMsgRecvQ)

	if err := ccfg.Start(t.ctx, daemonenv.DrainChanDuration); err != nil {
		return err
	}
	if err := cstat.Start(t.ctx); err != nil {
		return err
	}
	if err := istat.Start(t.ctx); err != nil {
		return err
	}

	initialCcfg := ccfg.Get()
	if initialCcfg.Name == "" {
		panic("cluster name read from ccfg is empty")
	}
	// Before any icfg, hb, or listener: ensure omcrypto has cluster name and secret
	omcrypto.SetClusterName(initialCcfg.Name)
	omcrypto.SetClusterSecret(initialCcfg.Secret())

	if livePort := initialCcfg.Listener.Port; livePort != daemonenv.HttpPort {
		// update daemonenv.HttpPort from live config value. Discover will need
		// connect to peers to fetch config...
		daemonenv.HttpPort = initialCcfg.Listener.Port
	}

	for _, s := range []startStopper{
		listener.New(),
		nmon.New(daemonenv.DrainChanDuration),
		dns.New(daemonenv.DrainChanDuration),
		discover.New(daemonenv.DrainChanDuration),
		hb.New(),
		scheduler.New(),
	} {
		if err := t.start(t.ctx, s); err != nil {
			return err
		}
	}

	bus.Pub(&msgbus.DaemonStart{Node: localhost, Version: version.Version()})
	t.log.Info().Msg("daemon started")
	return nil
}

func (t *T) start(ctx context.Context, a startStopper) error {
	if err := a.Start(ctx); err != nil {
		return err
	}
	t.wg.Add(1)
	t.cancelFuncs = append(t.cancelFuncs, func() {
		if err := a.Stop(); err != nil {
			t.log.Error().Err(err).Msg("stopping component failed")
		}
		t.wg.Done()
	})
	return nil
}

func (t *T) Stop() error {
	// stop goroutines without cancel context
	t.log.Info().Msg("daemon stopping")
	defer t.log.Info().Msg("daemon stopped")
	for i := len(t.cancelFuncs) - 1; i >= 0; i-- {
		t.cancelFuncs[i]()
	}

	t.cancel()
	return nil
}

func (t *T) Wait() {
	t.wg.Wait()
}

// notifyWatchDog is a notify watch dog loop that send notify watch dog
//
// It does nothing when:
//   - env var WATCHDOG_USEC is empty, os is < 2s
//   - if there is no daemon sysmanager (daemonsys.New retuns error)
func (t *T) notifyWatchDog(ctx context.Context) {
	var (
		i   interface{}
		err error
	)
	s := os.Getenv("WATCHDOG_USEC")
	if s == "" {
		return
	}
	i, err = converters.Duration.Convert(s + "us")
	if err != nil {
		t.log.Warn().Msgf("disable notify watchdog invalid WATCHDOG_USEC value: %s", s)
		return
	}
	d := i.(*time.Duration)
	sendInterval := *d / 2
	if sendInterval < time.Second {
		t.log.Warn().Msgf("disable notify watchdog %s < 1 second ", sendInterval)
		return
	}
	i, err = daemonsys.New(ctx)
	if err != nil {
		return
	}
	type notifyWatchDogCloser interface {
		NotifyWatchdog() (bool, error)
		Close() error
	}
	o, ok := i.(notifyWatchDogCloser)
	if !ok {
		return
	}
	defer func() {
		_ = o.Close()
	}()
	ticker := time.NewTicker(sendInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ok, err := o.NotifyWatchdog(); err != nil {
				t.log.Warn().Err(err).Msg("notifyWatchDog")
			} else if !ok {
				t.log.Warn().Msg("notifyWatchDog not delivered")
			} else {
				t.log.Debug().Msg("notifyWatchDog delivered")
			}
		}
	}
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
