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
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	discover struct {
		cfgCmdC           chan any
		objectMonitorCmdC chan any
		ctx               context.Context
		log               zerolog.Logger

		// cfgMTime is a map of local instance config file time, indexed by object
		// path string representation.
		// More recent remote config files are fetched.
		cfgMTime map[string]time.Time

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

		localhost string
		fsWatcher *fsnotify.Watcher

		subCfgUpdated     pubsub.Subscription
		subCfgDeleted     pubsub.Subscription
		subCfgFileUpdated pubsub.Subscription
	}
)

var (
	dropCmdTimeout = 100 * time.Millisecond
)

// Start function starts file system watcher on config directory
// then listen for config file creation to create
func Start(ctx context.Context) (func(), error) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	d := discover{
		cfgCmdC:           make(chan any),
		objectMonitorCmdC: make(chan any),
		cfgMTime:          make(map[string]time.Time),
		ctx:               ctx,
		log:               daemonlogctx.Logger(ctx).With().Str("name", "daemon.discover").Logger(),

		objectMonitor: make(map[string]map[string]struct{}),

		fetcherFrom:       make(map[string]string),
		fetcherCancel:     make(map[string]context.CancelFunc),
		fetcherNodeCancel: make(map[string]map[string]context.CancelFunc),
		fetcherUpdated:    make(map[string]time.Time),
		localhost:         hostname.Hostname(),
	}
	wg.Add(2)
	cfgStarted := make(chan bool)
	go func(c chan<- bool) {
		defer wg.Done()
		d.cfg(c)
	}(cfgStarted)
	<-cfgStarted

	omonStarted := make(chan bool)
	go func(c chan<- bool) {
		defer wg.Done()
		d.omon(c)
	}(omonStarted)
	<-omonStarted

	stopFSWatcher, err := d.fsWatcherStart()
	if err != nil {
		d.log.Error().Err(err).Msg("start")
		return stopFSWatcher, err
	}

	cancelAndWait := func() {
		stopFSWatcher()
		cancel() // stop cfg and omon via context cancel
		wg.Wait()
	}
	return cancelAndWait, nil
}
