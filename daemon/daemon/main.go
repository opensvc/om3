/*
Package daemon provide the subdaemon main responsible ot other opensvc daemons

It is responsible for other sub daemons (listener, discover, scheduler, hb...)
*/
package daemon

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/opensvc/om3/daemon/hb"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/istat"
	"github.com/opensvc/om3/daemon/listener"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/nmon"
	"github.com/opensvc/om3/daemon/scheduler"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/version"
)

type (
	T struct {
		ctx    context.Context
		cancel context.CancelFunc
		log    zerolog.Logger

		bus *pubsub.Bus

		stopFuncs []func() error
		wg        sync.WaitGroup
	}

	startStopper interface {
		Start(ctx context.Context) error
		Stop() error
	}
)

var (
	profiling = true
)

func New() *T {
	return &T{
		log:       log.Logger,
		stopFuncs: make([]func() error, 0),
	}
}

// Start is used to startup mandatory daemon components
func (t *T) Start(ctx context.Context) error {
	if t.Running() {
		return fmt.Errorf("can't start again, daemon is already running")
	}
	t.log.Info().Msg("daemon starting")
	go startProfiling()
	t.ctx, t.cancel = context.WithCancel(ctx)

	bus := pubsub.NewBus("daemon")
	bus.SetDefaultSubscriptionQueueSize(200)
	bus.SetDrainChanDuration(3 * daemonenv.DrainChanDuration)
	t.ctx = pubsub.ContextWithBus(t.ctx, bus)
	t.wg.Add(1)
	bus.Start(t.ctx)
	t.bus = bus
	t.stopFuncs = append(t.stopFuncs, func() error {
		defer t.wg.Done()
		t.log.Info().Msg("stop daemon pubsub bus")
		t.bus.Stop()
		t.log.Info().Msg("stopped daemon pubsub bus")
		return nil
	})
	localhost := hostname.Hostname()

	defer t.stopWatcher()

	go t.notifyWatchDogSys(t.ctx)

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.notifyWatchDogBus()
	}()

	dataCmd, dataMsgRecvQ, dataCmdCancel := daemondata.Start(t.ctx, daemonenv.DrainChanDuration)
	t.stopFuncs = append(t.stopFuncs, func() error {
		t.log.Debug().Msg("stop daemon data")
		dataCmdCancel()
		return nil
	})
	t.ctx = daemondata.ContextWithBus(t.ctx, dataCmd)
	t.ctx = daemonctx.WithHBRecvMsgQ(t.ctx, dataMsgRecvQ)

	// startup ccfg
	if err := t.startComponent(t.ctx, ccfg.New(daemonenv.DrainChanDuration)); err != nil {
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
		hbcache.New(2 * daemonenv.DrainChanDuration),
		cstat.New(),
		istat.New(),
		listener.New(),
		nmon.New(daemonenv.DrainChanDuration),
		dns.New(daemonenv.DrainChanDuration),
		discover.New(daemonenv.DrainChanDuration),
		hb.New(),
		scheduler.New(),
	} {
		if err := t.startComponent(t.ctx, s); err != nil {
			return err
		}
	}

	bus.Pub(&msgbus.DaemonStart{Node: localhost, Version: version.Version()})
	t.log.Info().Msg("daemon started")
	return nil
}

func (t *T) Stop() error {
	if t.cancel == nil {
		return fmt.Errorf("can't stop not started daemon")
	}
	var errs error
	// stop goroutines without cancel context
	defer t.log.Info().Msg("daemon stopped")
	t.log.Info().Msg("daemon stopping")
	for i := len(t.stopFuncs) - 1; i >= 0; i-- {
		if err := t.stopFuncs[i](); err != nil {
			t.log.Error().Err(err).Msgf("stop daemon component %d failed", i)
			errs = errors.Join(errs, errs)
		}
	}
	t.stopFuncs = make([]func() error, 0)

	t.cancel()
	t.cancel = nil

	t.wg.Wait()
	return errs
}

func (t *T) Running() bool {
	if t.ctx == nil {
		return false
	}
	select {
	case <-t.ctx.Done():
		return false
	default:
		return true
	}
}

func (t *T) Wait() {
	t.wg.Wait()
}

func (t *T) stopWatcher() {
	sub := pubsub.BusFromContext(t.ctx).Sub("daemon.stop.watcher")
	sub.AddFilter(&msgbus.DaemonCtl{}, pubsub.Label{"node", hostname.Hostname()}, pubsub.Label{"id", "daemon"})
	sub.Start()

	signal.Ignore(syscall.SIGHUP)
	signalCtx, signalCancel := signal.NotifyContext(t.ctx, os.Interrupt, syscall.SIGTERM)

	started := make(chan bool)
	go func() {
		defer func() {
			signalCancel()
			_ = sub.Stop()
			t.log.Info().Msg("daemon stop watcher done")
		}()
		t.log.Info().Msg("daemon stop watcher started")
		started <- true
		for {
			select {
			case <-t.ctx.Done():
				t.log.Info().Msg("daemon stop watcher returns on context done")
				return
			case <-signalCtx.Done():
				t.log.Info().Msg("daemon stopping on signal")
				go func() { _ = t.Stop() }()
				return
			case i := <-sub.C:
				switch m := i.(type) {
				case *msgbus.DaemonCtl:
					if m.Action == "stop" {
						t.log.Info().Msg("daemon stopping on daemon ctl message")
						go func() { _ = t.Stop() }()
						return
					}
				}
			}
		}
	}()
	<-started
}

// startComponent startup a component and add glue to wait group.
//
// on succeed startup the wait group is updated,
// the t.stopFuncs list is updated with a.Stop + wait group update.
func (t *T) startComponent(ctx context.Context, a startStopper) error {
	if err := a.Start(ctx); err != nil {
		return err
	}
	t.wg.Add(1)
	t.stopFuncs = append(t.stopFuncs, func() error {
		defer t.wg.Done()
		if err := a.Stop(); err != nil {
			t.log.Error().Err(err).Msg("stopping component failed")
			return err
		}
		return nil
	})
	return nil
}

func (t *T) notifyWatchDogBus() {
	defer t.log.Info().Msg("watch dog bus done")
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	labels := []pubsub.Label{{"node", hostname.Hostname()}, {"bus", t.bus.Name()}}
	msg := msgbus.WatchDog{Bus: t.bus.Name()}
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.bus.Pub(&msg, labels...)
		}
	}
}

// notifyWatchDogSys is a notify watch dog loop that send notify watch dog
//
// It does nothing when:
//   - env var WATCHDOG_USEC is empty, os is < 2s
//   - if there is no daemon sysmanager (daemonsys.New retuns error)
func (t *T) notifyWatchDogSys(ctx context.Context) {
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
		t.log.Info().Msg("notify watchdog sys done")
		_ = o.Close()
	}()
	t.log.Info().Msg("notify watchdog sys started")
	ticker := time.NewTicker(sendInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ok, err := o.NotifyWatchdog(); err != nil {
				t.log.Warn().Err(err).Msg("notifyWatchDogSys")
			} else if !ok {
				t.log.Warn().Msg("notifyWatchDogSys not delivered")
			} else {
				t.log.Debug().Msg("notifyWatchDogSys delivered")
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
