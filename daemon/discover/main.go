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
	discover struct {
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

		subCfgUpdated     pubsub.Subscription
		subCfgDeleted     pubsub.Subscription
		subCfgFileUpdated pubsub.Subscription

		// drainDuration is the max duration to wait while dropping commands and
		// is the drain duration created imon.
		drainDuration time.Duration

		imonStarter omon.IMonStarter

		wg sync.WaitGroup
	}
)

// New prepares Discover with drainDuration.
func New(drainDuration time.Duration) *discover {
	return &discover{
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
		imonStarter:       imon.Factory{DrainDuration: drainDuration},
	}
}

// Start function starts file system watcher on config directory
// then listen for config file creation to create.
func (d *discover) Start(ctx context.Context) (err error) {
	d.log = plog.NewDefaultLogger().Attr("pkg", "daemon/discover").WithPrefix("daemon: discover: ")
	d.log.Infof("discover starting")

	d.ctx, d.cancel = context.WithCancel(ctx)
	d.databus = daemondata.FromContext(d.ctx)
	d.nodeList = newObjectList(d.ctx, filepath.Join(rawconfig.Paths.Var, "list.nodes"))
	d.objectList = newObjectList(d.ctx, filepath.Join(rawconfig.Paths.Var, "list.objects"))

	d.wg.Add(1)
	cfgStarted := make(chan bool)
	go func(c chan<- bool) {
		defer d.wg.Done()
		defer d.log.Infof("cfg: stopped")
		d.cfg(c)
	}(cfgStarted)
	<-cfgStarted

	omonStarted := make(chan bool)
	d.wg.Add(1)
	go func(c chan<- bool) {
		defer d.wg.Done()
		d.omon(c)
	}(omonStarted)
	<-omonStarted

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.log.Infof("cfg: node list stopped")
		d.nodeList.Add(clusternode.Get()...)
		d.nodeList.Loop()
	}()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.log.Infof("cfg: object list stopped")
		d.objectList.Add(object.StatusData.GetPaths().StrSlice()...)
		d.objectList.Loop()
	}()

	if stopFSWatcher, err := d.fsWatcherStart(); err != nil {
		d.log.Errorf("fs: start failed: %s", err)
		return err
	} else {
		d.fsWatcherStop = stopFSWatcher
	}
	d.log.Infof("fs: started")
	return nil
}

func (d *discover) Stop() error {
	d.log.Infof("stopping")
	defer d.log.Infof("stopped")
	if d.fsWatcherStop != nil {
		d.fsWatcherStop()
	}
	d.cancel() // stop cfg and omon via context cancel
	d.wg.Wait()
	return nil
}

func (d *discover) objectLogger(p naming.Path) *plog.Logger {
	return objectLogger(d.log, p)
}

func objectLogger(l *plog.Logger, p naming.Path) *plog.Logger {
	return l.
		Attr("obj_path", p).
		Attr("obj_name", p.Name).
		Attr("obj_namespace", p.Namespace).
		Attr("obj_kind", p.Kind.String())
}
