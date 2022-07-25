// Package daemondiscover implements object discovery for daemon
//
// It watches config filesystem to create initial instance config worker when
// config file is created. Instance config worker is then responsible for
// watching instance config updates
//
// When is discovers that another remote config is available and no instance
// config worker is running, it fetches remote instance config to local config
// directory.
//
// It is responsible for initial aggregated worker creation.
//
package daemondiscover

import (
	"context"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	discover struct {
		cfgCmdC    chan *moncmd.T
		svcaggCmdC chan *moncmd.T
		ctx        context.Context
		log        zerolog.Logger

		// moncfg stores instcfg routines stop funcs
		moncfg map[string]func()

		monCfgCmdC   map[string]chan<- *moncmd.T
		svcAggCancel map[string]context.CancelFunc
		svcAgg       map[string]map[string]struct{}

		remoteNodeCtx        map[string]context.Context
		remoteNodeCancel     map[string]context.CancelFunc
		remoteCfgFetchCancel map[string]context.CancelFunc

		// fetcherUpdated map[svc] updated timestamp of svc config being fetched
		fetcherUpdated map[string]timestamp.T

		// fetcherFrom map[svc] node
		fetcherFrom map[string]string

		// fetcherCancel map[svc] cancel func for svc fetcher
		fetcherCancel map[string]context.CancelFunc

		// fetcherNodeCancel map[node]map[svc] cancel func for node
		fetcherNodeCancel map[string]map[string]context.CancelFunc

		localhost string
		fsWatcher *fsnotify.Watcher
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
		cfgCmdC:    make(chan *moncmd.T),
		svcaggCmdC: make(chan *moncmd.T),

		ctx:    ctx,
		log:    daemonlogctx.Logger(ctx).With().Str("name", "daemon.discover").Logger(),
		moncfg: make(map[string]func()),

		monCfgCmdC:   make(map[string]chan<- *moncmd.T),
		svcAggCancel: make(map[string]context.CancelFunc),
		svcAgg:       make(map[string]map[string]struct{}),

		fetcherFrom:       make(map[string]string),
		fetcherCancel:     make(map[string]context.CancelFunc),
		fetcherNodeCancel: make(map[string]map[string]context.CancelFunc),
		fetcherUpdated:    make(map[string]timestamp.T),
		localhost:         hostname.Hostname(),
	}
	stopFSWatcher, err := d.fsWatcherStart()
	if err != nil {
		d.log.Error().Err(err).Msg("start")
		return stopFSWatcher, err
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		d.cfg()
	}()
	go func() {
		defer wg.Done()
		d.agg()
	}()
	cancelAndWait := func() {
		stopFSWatcher()
		for _, stop := range d.moncfg {
			stop()
		}
		cancel() // stop cfg and agg via context cancel
		wg.Wait()
	}
	return cancelAndWait, nil
}
