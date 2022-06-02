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
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/daemonctx"
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

		moncfg       map[string]struct{}
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
func Start(ctx context.Context) error {
	d := discover{
		cfgCmdC:    make(chan *moncmd.T),
		svcaggCmdC: make(chan *moncmd.T),

		ctx:    ctx,
		log:    daemonctx.Logger(ctx).With().Str("_pkg", "daemondiscover").Logger(),
		moncfg: make(map[string]struct{}),

		monCfgCmdC:   make(map[string]chan<- *moncmd.T),
		svcAggCancel: make(map[string]context.CancelFunc),
		svcAgg:       make(map[string]map[string]struct{}),

		fetcherFrom:       make(map[string]string),
		fetcherCancel:     make(map[string]context.CancelFunc),
		fetcherNodeCancel: make(map[string]map[string]context.CancelFunc),
		fetcherUpdated:    make(map[string]timestamp.T),
		localhost:         hostname.Hostname(),
	}
	if err := d.fsWatcherStart(); err != nil {
		d.log.Error().Err(err).Msg("fsWatcherStart")
		return err
	}
	go d.cfg()
	go d.agg()
	return nil
}
