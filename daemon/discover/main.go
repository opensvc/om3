// Package discover implements object discovery for daemon
//
// It watches config filesystem to create initial instance config worker when
// config file is created. Instance config worker is then responsible for
// watching instance config updates
//
// When is discovers that another remote config is available and no instance
// config worker is running, it fetches remote instance config to local config
// directory.
//
// It is responsible for initial object status worker creation.
package discover

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/imon"
	"github.com/opensvc/om3/daemon/omon"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	Manager struct {
		cfgCmdC           chan any
		objectMonitorCmdC chan any
		ctx               context.Context
		cancel            context.CancelFunc
		log               *plog.Logger
		databus           *daemondata.T

		// cfgMTime is a map of local instance config file time, indexed by object
		// path string representation.
		// More recent remote config files are fetched.
		cfgMTime map[string]time.Time

		clusterConfig       cluster.Config
		objectMonitorCancel map[string]context.CancelFunc
		objectMonitor       map[string]map[string]struct{}

		remoteNodeCtx        map[string]context.Context
		remoteNodeCancel     map[string]context.CancelFunc
		remoteCfgFetchCancel map[string]context.CancelFunc

		// fetcherUpdated map[svc] updated timestamp of svc config being fetched
		fetcherUpdated map[string]time.Time

		// fetcherFrom map[svc] node
		fetcherFrom map[string]string

		// fetcherCancel map[svc] cancel func for svc fetcher
		fetcherCancel map[string]context.CancelFunc

		// fetcherNodeCancel map[node]map[svc] cancel func for node
		fetcherNodeCancel map[string]map[string]context.CancelFunc

		fsWatcher     *fsnotify.Watcher
		fsWatcherStop func()
		localhost     string

		nodeList   *objectList
		objectList *objectList

		subQS pubsub.QueueSizer

		// omonSubQS is the sub queue size of each created omon
		omonSubQS pubsub.QueueSizer

		// drainDuration is the max duration to wait while dropping commands and
		// is the drain duration created imon.
		drainDuration time.Duration

		imonStarter omon.IMonStarter

		wg sync.WaitGroup
	}
)

// NewManager initialize discover.Manager with drainDuration, and sub queues sizes:
//
//   - subQS: the subscription queue size of discover.cfg and discover.omon components.
//   - omonSubQS: the subscription queue size of omon components with omonSubQS.
//   - imonSubQS: the subscription queue size imon components (created from imon.Factory).
func NewManager(drainDuration, imonDelayDuration time.Duration, subQS, omonSubQS, imonSubQS pubsub.QueueSizer) *Manager {
	return &Manager{
		cfgCmdC:           make(chan any),
		objectMonitorCmdC: make(chan any),
		cfgMTime:          make(map[string]time.Time),

		objectMonitor: make(map[string]map[string]struct{}),

		fetcherFrom:       make(map[string]string),
		fetcherCancel:     make(map[string]context.CancelFunc),
		fetcherNodeCancel: make(map[string]map[string]context.CancelFunc),
		fetcherUpdated:    make(map[string]time.Time),
		localhost:         hostname.Hostname(),
		drainDuration:     drainDuration,
		imonStarter:       imon.Factory{DrainDuration: drainDuration, DelayDuration: imonDelayDuration, SubQS: imonSubQS},
		subQS:             subQS,
		omonSubQS:         omonSubQS,
	}
}

// Start function starts file system watcher on config directory
// then listen for config file creation to create.
func (t *Manager) Start(ctx context.Context) (err error) {
	t.log = plog.NewDefaultLogger().Attr("pkg", "daemon/discover").WithPrefix("daemon: discover: ")
	t.log.Infof("discover starting")

	t.ctx, t.cancel = context.WithCancel(ctx)
	t.databus = daemondata.FromContext(t.ctx)
	t.nodeList = newObjectList(t.ctx, filepath.Join(rawconfig.Paths.Var, "list.nodes"))
	t.objectList = newObjectList(t.ctx, filepath.Join(rawconfig.Paths.Var, "list.objects"))

	t.wg.Add(1)
	cfgStarted := make(chan bool)
	go func(c chan<- bool) {
		defer t.wg.Done()
		defer t.log.Infof("cfg: stopped")
		t.cfg(c)
	}(cfgStarted)
	<-cfgStarted

	omonStarted := make(chan bool)
	t.wg.Add(1)
	go func(c chan<- bool) {
		defer t.wg.Done()
		t.omon(c)
	}(omonStarted)
	<-omonStarted

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer t.log.Infof("cfg: node list stopped")
		t.nodeList.Add(clusternode.Get()...)
		t.nodeList.Loop()
	}()

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer t.log.Infof("cfg: object list stopped")
		t.objectList.Add(object.StatusData.GetPaths().StrSlice()...)
		t.objectList.Loop()
	}()

	if stopFSWatcher, err := t.fsWatcherStart(); err != nil {
		t.log.Errorf("fs: start failed: %s", err)
		return err
	} else {
		t.fsWatcherStop = stopFSWatcher
	}
	t.log.Infof("fs: started")
	return nil
}

func (t *Manager) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	if t.fsWatcherStop != nil {
		t.fsWatcherStop()
	}
	t.cancel() // stop cfg and omon via context cancel
	t.wg.Wait()
	return nil
}

func (t *Manager) objectLogger(p naming.Path) *plog.Logger {
	return naming.LogWithPath(t.log, p)
}
