// Package nmon is responsible for of the local node monitor state
//
//	It provides the cluster data:
//		["cluster", "node", <localhost>, "services", "status", <instance>, "monitor"]
//		["cluster", "node", <localhost>, "services", "nmon", <instance>]
//
//	worker watches on local status updates to clear reached status
//		=> unsetStatusWhenReached
//		=> orchestrate
//		=> pub new state if change
//
//	worker watches on remote nmon updates converge global expects
//		=> convergeGlobalExpectFromRemote
//		=> orchestrate
//		=> pub new state if change
package nmon

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/prometheus/procfs"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/nodesinfo"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/san"
)

type (
	nmon struct {
		state         cluster.NodeMonitor
		previousState cluster.NodeMonitor

		ctx      context.Context
		cancel   context.CancelFunc
		cmdC     chan any
		dataCmdC chan<- any
		log      zerolog.Logger

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		scopeNodes []string
		nmons      map[string]cluster.NodeMonitor

		cancelReady context.CancelFunc
		localhost   string
		change      bool

		sub *pubsub.Subscription
	}

	// cmdOrchestrate can be used from post action go routines
	cmdOrchestrate struct {
		state    cluster.NodeMonitorStatus
		newState cluster.NodeMonitorStatus
	}
)

// Start launches the nmon worker goroutine
func Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)

	previousState := cluster.NodeMonitor{
		Status: cluster.NodeMonitorStatusIdle,
	}
	state := previousState

	o := &nmon{
		state:         state,
		previousState: previousState,
		ctx:           ctx,
		cancel:        cancel,
		cmdC:          make(chan any),
		dataCmdC:      daemondata.BusFromContext(ctx),
		log:           log.Logger.With().Str("func", "nmon").Logger(),
		localhost:     hostname.Hostname(),
		change:        true,
		nmons:         make(map[string]cluster.NodeMonitor),
	}

	o.startSubscriptions()
	go func() {
		defer func() {
			msgbus.DropPendingMsg(o.cmdC, time.Second)
			o.sub.Stop()
		}()
		o.worker()
	}()
	return nil
}

func (o *nmon) startSubscriptions() {
	bus := pubsub.BusFromContext(o.ctx)
	sub := bus.Sub("nmon")
	sub.AddFilter(msgbus.NodeMonitorUpdated{})
	sub.AddFilter(msgbus.NodeMonitorDeleted{})
	sub.AddFilter(msgbus.FrozenFileRemoved{})
	sub.AddFilter(msgbus.FrozenFileUpdated{})
	sub.AddFilter(msgbus.SetNodeMonitor{})
	sub.AddFilter(msgbus.NodeStatusLabelsUpdated{})
	sub.AddFilter(msgbus.NodeOsPathsUpdated{})
	sub.Start()
	o.sub = sub
}

// worker watch for local nmon updates
func (o *nmon) worker() {
	defer o.log.Debug().Msg("done")

	initialNodes := strings.Fields(rawconfig.ClusterSection().Nodes)
	for _, node := range initialNodes {
		o.nmons[node] = daemondata.GetNmon(o.dataCmdC, node)
	}
	o.updateStats()
	o.setNodeOsPaths()
	o.updateIfChange()
	defer o.delete()
	o.log.Debug().Msg("started")

	statsTicker := time.NewTicker(10 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case msgbus.NodeMonitorUpdated:
				o.onNmonUpdated(c)
			case msgbus.NodeMonitorDeleted:
				o.onNmonDeleted(c)
			case msgbus.FrozenFileRemoved:
				o.onFrozenFileRemoved(c)
			case msgbus.FrozenFileUpdated:
				o.onFrozenFileUpdated(c)
			case msgbus.SetNodeMonitor:
				o.onSetNmonCmd(c)
			case msgbus.NodeStatusLabelsUpdated:
				o.onNodeStatusLabelsUpdated(c)
			case msgbus.NodeOsPathsUpdated:
				o.onNodeOsPathsUpdated(c)
			}
		case i := <-o.cmdC:
			switch c := i.(type) {
			case cmdOrchestrate:
				o.onOrchestrate(c)
			}
		case <-statsTicker.C:
			o.updateStats()
		}
	}
}

func (o *nmon) delete() {
	if err := daemondata.DelNmon(o.dataCmdC); err != nil {
		o.log.Error().Err(err).Msg("DelNmon")
	}
}

func (o *nmon) update() {
	newValue := o.state
	if err := daemondata.SetNmon(o.dataCmdC, newValue); err != nil {
		o.log.Error().Err(err).Msg("SetNmon")
	}
}

// updateIfChange log updates and publish new state value when changed
func (o *nmon) updateIfChange() {
	if !o.change {
		return
	}
	o.change = false
	o.state.StatusUpdated = time.Now()
	previousVal := o.previousState
	newVal := o.state
	if newVal.Status != previousVal.Status {
		o.log.Info().Msgf("change monitor state %s -> %s", previousVal.Status, newVal.Status)
	}
	if newVal.GlobalExpect != previousVal.GlobalExpect {
		o.log.Info().Msgf("change monitor global expect %s -> %s", previousVal.GlobalExpect, newVal.GlobalExpect)
	}
	if newVal.LocalExpect != previousVal.LocalExpect {
		o.log.Info().Msgf("change monitor local expect %s -> %s", previousVal.LocalExpect, newVal.LocalExpect)
	}
	o.previousState = o.state
	o.update()
}

func (o *nmon) hasOtherNodeActing() bool {
	for remoteNode, remoteNmon := range o.nmons {
		if remoteNode == o.localhost {
			continue
		}
		if remoteNmon.Status.IsDoing() {
			return true
		}
	}
	return false
}

func (o *nmon) createPendingWithCancel() {
	o.pendingCtx, o.pendingCancel = context.WithCancel(o.ctx)
}

func (o *nmon) createPendingWithDuration(duration time.Duration) {
	o.pendingCtx, o.pendingCancel = context.WithTimeout(o.ctx, duration)
}

func (o *nmon) clearPending() {
	if o.pendingCancel != nil {
		o.pendingCancel()
		o.pendingCancel = nil
		o.pendingCtx = nil
	}
}

func (o *nmon) getStats() (cluster.NodeStats, error) {
	stats := cluster.NodeStats{}
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return stats, err
	}
	if load, err := fs.LoadAvg(); err != nil {
		return stats, err
	} else {
		stats.Load15M = load.Load15
		stats.Score += uint64(100 / math.Max(load.Load15, 1))
	}
	if mem, err := fs.Meminfo(); err != nil {
		return stats, err
	} else {
		stats.MemTotalMB = *mem.MemTotal / 1024
		stats.MemAvailPct = 100 * *mem.MemAvailable / *mem.MemTotal
		stats.SwapTotalMB = *mem.SwapTotal / 1024
		stats.SwapAvailPct = 100 * *mem.SwapFree / *mem.SwapTotal
		stats.Score += 100 + stats.MemAvailPct
		stats.Score += 2 * (100 + stats.SwapAvailPct)
	}
	stats.Score = stats.Score / 7

	return stats, nil
}

func (o *nmon) updateStats() {
	if stats, err := o.getStats(); err != nil {
		o.log.Error().Err(err).Msg("get stats")
	} else if err := daemondata.SetNodeStats(o.dataCmdC, stats); err != nil {
		o.log.Error().Err(err).Msg("set stats")
	}
}

func (o *nmon) setNodeOsPaths() {
	if paths, err := san.GetPaths(); err != nil {
		o.log.Error().Err(err).Msg("get san paths")
	} else if err := daemondata.SetNodeOsPaths(o.dataCmdC, paths); err != nil {
		o.log.Error().Err(err).Msg("set san paths")
	}
}

func (o *nmon) onNodeOsPathsUpdated(c msgbus.NodeOsPathsUpdated) {
	o.saveNodesInfo()
}

func (o *nmon) onNodeStatusLabelsUpdated(c msgbus.NodeStatusLabelsUpdated) {
	o.saveNodesInfo()
}

func (o *nmon) saveNodesInfo() {
	data := *daemondata.GetNodesInfo(o.dataCmdC)
	if err := nodesinfo.Save(data); err != nil {
		o.log.Error().Err(err).Msg("save nodes info")
	} else {
		o.log.Info().Msg("nodes info cache refreshed")
	}

}
