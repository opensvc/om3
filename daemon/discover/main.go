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
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/imon"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/omon"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	Manager struct {
		cfgCmdC chan any
		ctx     context.Context
		cancel  context.CancelFunc
		log     *plog.Logger

		pub pubsub.PublishBuilder

		// cfgDeleting is a map of local crm deleting call indexed by object path
		cfgDeleting map[naming.Path]bool

		// cfgMTime is a map of local instance config file time, indexed by object
		// path string representation.
		// More recent remote config files are fetched.
		cfgMTime map[string]time.Time

		// disableRecover is a map of local running icfg (indexed by object) where
		// the onInstanceConfigManagerDone recover is disabled.
		disableRecover map[naming.Path]time.Time

		clusterConfig       cluster.Config
		objectMonitorCancel map[string]context.CancelFunc

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

		// debouncers is a map of delayed publications to cope with fs event storms
		debouncers map[string]*Debouncer

		watched       map[naming.Path]map[string]any
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

		// retainForeignConfigFor tracks the nodes that haven't yet fetch local
		// config file in the case of localhost event InstanceConfigFor.
		//
		// On localhost event ev InstanceConfigFor: retainForeignConfigFor[ev.path] = ev.Scope
		//
		// On peer event ev InstanceConfigUpdated: ev.Node is removed from the
		// retainForeignConfigFor[ev.path] array.
		//
		// when retainForeignConfigFor[ev.path] is empty we can remove local config file
		retainForeignConfigFor map[naming.Path][]string

		// instanceConfigFor tracks the local instanceConfigFor events that have
		// not been transmitted to peer during daemon warmup (the event is not
		// applied during apply full).
		// So we keep those pending event to retransmit them when hb message type
		// become 'patch'
		instanceConfigFor map[naming.Path]*msgbus.InstanceConfigFor

		labelLocalhost pubsub.Label
	}
)

// NewManager initialize discover.Manager with drainDuration, and sub queue
// sizes for discover.cfg and discover.omon components.
//
// It returns *T.
//
// It doesn't define imon starter, and sub queue size for future created omon
// components. So WithImonStarter and WithOmonSubQS must be called on the
// returned *T
func NewManager(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		cfgDeleting: make(map[naming.Path]bool),

		cfgCmdC:  make(chan any),
		cfgMTime: make(map[string]time.Time),

		retainForeignConfigFor: make(map[naming.Path][]string),

		instanceConfigFor: make(map[naming.Path]*msgbus.InstanceConfigFor),

		disableRecover: make(map[naming.Path]time.Time),

		debouncers:        make(map[string]*Debouncer),
		fetcherFrom:       make(map[string]string),
		fetcherCancel:     make(map[string]context.CancelFunc),
		fetcherNodeCancel: make(map[string]map[string]context.CancelFunc),
		fetcherUpdated:    make(map[string]time.Time),
		localhost:         localhost,
		drainDuration:     drainDuration,
		subQS:             subQS,
		watched:           make(map[naming.Path]map[string]any),
		labelLocalhost:    pubsub.Label{"node", localhost},
	}
}

// WithImonStarter defines the imon factory
func (t *Manager) WithImonStarter(f imon.Factory) *Manager {
	t.imonStarter = f
	return t
}

// WithOmonSubQS defines the subscription queue size of omon created components.
func (t *Manager) WithOmonSubQS(qs pubsub.QueueSizer) *Manager {
	t.omonSubQS = qs
	return t
}

// Start function starts file system watcher on config directory
// then listen for config file creation to create.
func (t *Manager) Start(ctx context.Context) (err error) {
	t.log = plog.NewDefaultLogger().Attr("pkg", "daemon/discover").WithPrefix("daemon: discover: ")
	t.log.Infof("discover starting")

	t.pub = pubsub.PubFromContext(ctx)

	if t.omonSubQS == nil {
		return fmt.Errorf("discover: undefined omon sub queue size, WithOmonSubQS must be called first")
	}
	if t.imonStarter == nil {
		return fmt.Errorf("discover: undefined imon starter, WithImonStarter must be called first")
	}

	t.ctx, t.cancel = context.WithCancel(ctx)
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
