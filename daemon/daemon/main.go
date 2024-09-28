/*
Package daemon is responsible ot other opensvc daemons start/stop

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

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/collector"
	"github.com/opensvc/om3/daemon/cstat"
	"github.com/opensvc/om3/daemon/daemonapi"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/daemonsys"
	"github.com/opensvc/om3/daemon/daemonvip"
	"github.com/opensvc/om3/daemon/discover"
	"github.com/opensvc/om3/daemon/dns"
	"github.com/opensvc/om3/daemon/hb"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/imon"
	"github.com/opensvc/om3/daemon/istat"
	"github.com/opensvc/om3/daemon/listener"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/nmon"
	"github.com/opensvc/om3/daemon/runner"
	"github.com/opensvc/om3/daemon/scheduler"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/version"
)

type (
	T struct {
		ctx    context.Context
		cancel context.CancelFunc
		log    *plog.Logger

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
	// bufferPublicationDuration is the minimum duration where pubsub buffer
	// publications during daemon startup.
	bufferPublicationDuration = 200 * time.Millisecond
)

func New() *T {
	return &T{
		log: plog.NewDefaultLogger().
			Attr("pkg", "daemon/daemon").
			WithPrefix("daemon: main: "),
		stopFuncs: make([]func() error, 0),
	}
}

// Start is used to startup mandatory daemon components
func (t *T) Start(ctx context.Context) error {
	var (
		qsSmall  = pubsub.WithQueueSize(daemonenv.SubQSSmall)
		qsMedium = pubsub.WithQueueSize(daemonenv.SubQSMedium)
		qsLarge  = pubsub.WithQueueSize(daemonenv.SubQSLarge)
		qsHuge   = pubsub.WithQueueSize(daemonenv.SubQSHuge)

		defaultSubscriptionQueueSize = daemonenv.SubQSSmall
	)

	if t.Running() {
		return fmt.Errorf("can't start again, daemon is already running")
	}
	t.logTransition("starting 🟢")
	go startProfiling()
	t.ctx, t.cancel = context.WithCancel(ctx)
	localhost := hostname.Hostname()
	labelLocalhost := pubsub.Label{"node", localhost}

	bus := pubsub.NewBus("daemon")
	bus.SetDefaultSubscriptionQueueSize(defaultSubscriptionQueueSize)
	bus.SetDrainChanDuration(3 * daemonenv.DrainChanDuration)
	bus.SetPanicOnFullQueue(10 * time.Second)
	t.ctx = pubsub.ContextWithBus(t.ctx, bus)
	t.wg.Add(1)
	bus.Start(t.ctx)
	bus.EnableBufferPublication(2000)
	bus.Pub(&msgbus.DaemonStatusUpdated{Node: localhost, Version: version.Version(), Status: "starting"}, labelLocalhost)

	t.bus = bus
	t.stopFuncs = append(t.stopFuncs, func() error {
		bus.Pub(&msgbus.DaemonStatusUpdated{Node: localhost, Version: version.Version(), Status: "stopped"}, labelLocalhost)
		// give chance for DaemonStatusUpdated message to reach peers
		time.Sleep(300 * time.Millisecond)
		defer t.wg.Done()
		t.log.Infof("stop pubsub bus")
		t.bus.Stop()
		t.log.Infof("stopped pubsub bus")
		return nil
	})

	defer func() {
		go func() {
			// give a chance for event client to reconnect to the daemon
			time.Sleep(bufferPublicationDuration)
			bus.DisableBufferPublication()
		}()
	}()

	defer t.stopWatcher()

	go t.notifyWatchDogSys(t.ctx)

	t.wg.Add(1)
	go func(ctx context.Context) {
		defer t.wg.Done()
		t.notifyWatchDogBus(ctx)
	}(t.ctx)

	dataCmd, dataMsgRecvQ, dataCmdCancel := daemondata.Start(t.ctx, daemonenv.DrainChanDuration, qsHuge)
	t.stopFuncs = append(t.stopFuncs, func() error {
		t.log.Debugf("stop data manager")
		dataCmdCancel()
		return nil
	})

	t.ctx = daemondata.ContextWithBus(t.ctx, dataCmd)
	t.ctx = daemonctx.WithHBRecvMsgQ(t.ctx, dataMsgRecvQ)

	// startup ccfg
	if err := t.startComponent(t.ctx, ccfg.New(daemonenv.DrainChanDuration, qsSmall)); err != nil {
		return err
	}
	initialCcfg := cluster.ConfigData.Get()
	if initialCcfg.Name == "" {
		panic("cluster name read from ccfg is empty")
	}
	if livePort := initialCcfg.Listener.Port; livePort != daemonenv.HTTPPort {
		// update daemonenv.HttpPort from live config value. Discover will need
		// connect to peers to fetch config...
		daemonenv.HTTPPort = initialCcfg.Listener.Port
	}

	// prepare imonFactory for discover component
	imonFactory := imon.Factory{
		DrainDuration: daemonenv.DrainChanDuration,
		DelayDuration: daemonenv.ImonDelayDuration,
		SubQS:         qsMedium,
	}

	t.ctx = daemonapi.WithSubQS(t.ctx, qsMedium)
	for _, s := range []startStopper{
		hbcache.New(2 * daemonenv.DrainChanDuration),
		cstat.New(qsMedium),
		istat.New(qsLarge),
		listener.New(),
		nmon.NewManager(daemonenv.DrainChanDuration, qsMedium),
		dns.NewManager(daemonenv.DrainChanDuration, qsMedium),
		discover.NewManager(daemonenv.DrainChanDuration, qsHuge).
			WithOmonSubQS(qsMedium).
			WithImonStarter(imonFactory),
		hb.New(t.ctx),
		collector.New(t.ctx, qsHuge),
		scheduler.New(qsHuge),
		daemonvip.New(qsSmall),
		runner.NewDefault(qsSmall),
	} {
		if err := t.startComponent(t.ctx, s); err != nil {
			return err
		}
	}

	t.logTransition("started 🟢")
	bus.Pub(&msgbus.DaemonStatusUpdated{
		Node:    localhost,
		Version: version.Version(),
		Status:  "started",
	}, labelLocalhost)
	return nil
}

func (t *T) Stop() error {
	if t.cancel == nil {
		return fmt.Errorf("can't stop not started daemon")
	}
	var errs error
	// stop goroutines without cancel context
	t.logTransition("stopping 🟡")
	localhost := hostname.Hostname()
	t.bus.Pub(&msgbus.DaemonStatusUpdated{Node: localhost, Version: version.Version(), Status: "stopping"}, pubsub.Label{"node", localhost})
	time.Sleep(300 * time.Millisecond)
	defer t.logTransition("stopped 🟡")
	for i := len(t.stopFuncs) - 1; i >= 0; i-- {
		if err := t.stopFuncs[i](); err != nil {
			t.log.Errorf("stop component %d: %s", i, err)
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

func (t *T) logTransition(state string) {
	t.log.Infof("daemon %s", state)
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
			t.log.Debugf("stop watcher terminated")
		}()
		t.log.Debugf("stop watcher running")
		started <- true
		for {
			select {
			case <-t.ctx.Done():
				t.log.Debugf("stop watcher returns on context done")
				return
			case <-signalCtx.Done():
				t.log.Infof("stopping on signal")
				go func() { _ = t.Stop() }()
				return
			case i := <-sub.C:
				switch m := i.(type) {
				case *msgbus.DaemonCtl:
					if m.Action == "stop" {
						t.log.Infof("stopping on daemon ctl message")
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
			t.log.Errorf("stopping component: %s", err)
			return err
		}
		return nil
	})
	return nil
}

func (t *T) notifyWatchDogBus(ctx context.Context) {
	defer t.log.Infof("watch dog bus done")
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	labels := []pubsub.Label{{"node", hostname.Hostname()}, {"bus", t.bus.Name()}}
	msg := msgbus.WatchDog{Bus: t.bus.Name()}
	for {
		select {
		case <-ctx.Done():
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
		t.log.Warnf("disable notify watchdog invalid WATCHDOG_USEC value: %s", s)
		return
	}
	d := i.(*time.Duration)
	sendInterval := *d / 2
	if sendInterval < time.Second {
		t.log.Warnf("disable notify watchdog %s < 1 second ", sendInterval)
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
		t.log.Infof("notify watchdog sys done")
		_ = o.Close()
	}()
	t.log.Infof("notify watchdog sys started")
	ticker := time.NewTicker(sendInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ok, err := o.NotifyWatchdog(); err != nil {
				t.log.Warnf("notifyWatchDogSys: %s", err)
			} else if !ok {
				t.log.Warnf("notifyWatchDogSys not delivered")
			} else {
				t.log.Debugf("notifyWatchDogSys delivered")
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
	cannula.Start(daemonenv.ProfileUnixFile())
}

func GetBufferPublicationDuration() time.Duration {
	return bufferPublicationDuration
}