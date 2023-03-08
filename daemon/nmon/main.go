// Package nmon is responsible for the local node states
//
// It provides the cluster data:
//
//	.cluster.node.<localhost>.monitor
//	.cluster.node.<localhost>.stats
//	.cluster.node.<localhost>.status
//
// The worker watches local status updates and clear reached status
//
//	=> unsetStatusWhenReached
//	=> orchestrate
//	=> pub new state if change
//
// The worker watches remote nmon updates and converge global expects
//
//	=> convergeGlobalExpectFromRemote
//	=> orchestrate
//	=> pub new state if change
package nmon

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/procfs"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/san"
)

type (
	nmon struct {
		// config is the node merged config
		config *xconfig.T

		// nodeConfig is the published node.Config. It is refreshed when config is
		// created or reloaded.
		nodeConfig node.Config

		state         node.Monitor
		previousState node.Monitor

		ctx          context.Context
		cancel       context.CancelFunc
		cmdC         chan any
		databus      *daemondata.T
		bus          *pubsub.Bus
		log          zerolog.Logger
		rejoinTicker *time.Ticker
		startedAt    time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		scopeNodes  []string
		nodeMonitor map[string]node.Monitor

		cancelReady context.CancelFunc
		localhost   string
		change      bool

		sub *pubsub.Subscription
	}

	// cmdOrchestrate can be used from post action go routines
	cmdOrchestrate struct {
		state    node.MonitorState
		newState node.MonitorState
	}
)

var (
	// statsInterval is the interval duration between 2 stats refresh
	statsInterval = 60 * time.Second
)

// Start launches the nmon worker goroutine
func Start(parent context.Context, drainDuration time.Duration) error {
	ctx, cancel := context.WithCancel(parent)
	localhost := hostname.Hostname()
	o := &nmon{
		state: node.Monitor{
			LocalExpect:  node.MonitorLocalExpectNone,
			GlobalExpect: node.MonitorGlobalExpectNone,
			State:        node.MonitorStateIdle,
		},
		previousState: node.Monitor{
			LocalExpect:  node.MonitorLocalExpectNone,
			GlobalExpect: node.MonitorGlobalExpectNone,
			State:        node.MonitorStateIdle,
		},
		ctx:         ctx,
		cancel:      cancel,
		cmdC:        make(chan any),
		databus:     daemondata.FromContext(ctx),
		bus:         pubsub.BusFromContext(ctx),
		log:         log.Logger.With().Str("func", "nmon").Logger(),
		localhost:   localhost,
		change:      true,
		nodeMonitor: make(map[string]node.Monitor),
	}

	if n, err := object.NewNode(object.WithVolatile(true)); err != nil {
		return err
	} else {
		o.config = n.MergedConfig()
		o.nodeConfig = o.getNodeConfig()
		// we are responsible for publication or node config, don't wait for
		// first ConfigFileUpdated event to do the job.
		if err := o.pubNodeConfig(); err != nil {
			o.log.Error().Err(err).Msg("publish initial node config")
		}
	}

	o.startSubscriptions()
	go func() {
		defer func() {
			go func() {
				tC := time.After(drainDuration)
				for {
					select {
					case <-tC:
						return
					case <-o.cmdC:
					}
				}
			}()
			if err := o.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				o.log.Error().Err(err).Msg("subscription stop")
			}
		}()
		o.worker()
	}()
	return nil
}

func (o *nmon) startSubscriptions() {
	sub := o.bus.Sub("nmon")
	sub.AddFilter(msgbus.ConfigFileUpdated{}, pubsub.Label{"path", ""})
	sub.AddFilter(msgbus.FrozenFileRemoved{})
	sub.AddFilter(msgbus.FrozenFileUpdated{})
	sub.AddFilter(msgbus.HbMessageTypeUpdated{})
	sub.AddFilter(msgbus.JoinRequest{}, pubsub.Label{"node", o.localhost})
	sub.AddFilter(msgbus.LeaveRequest{}, pubsub.Label{"node", o.localhost})
	sub.AddFilter(msgbus.NodeMonitorDeleted{})
	sub.AddFilter(msgbus.NodeMonitorUpdated{}, pubsub.Label{"peer", "true"})
	sub.AddFilter(msgbus.NodeOsPathsUpdated{})
	sub.AddFilter(msgbus.NodeStatusLabelsUpdated{})
	sub.AddFilter(msgbus.SetNodeMonitor{})
	sub.Start()
	o.sub = sub
}

func (o *nmon) startRejoin() {
	hbMessageType := o.databus.GetHbMessageType()
	l := missingNodes(hbMessageType.Nodes, hbMessageType.JoinedNodes)
	if (hbMessageType.Type == "patch") && len(l) == 0 {
		// Skip the rejoin state phase.
		o.rejoinTicker = time.NewTicker(time.Second)
		o.rejoinTicker.Stop()
		o.transitionTo(node.MonitorStateIdle)
	} else {
		// Begin the rejoin state phase.
		// Arm the rejoin grace period ticker.
		// The onHbMessageTypeUpdated() event handler can stop it.
		rejoinGracePeriod := o.nodeConfig.RejoinGracePeriod
		o.rejoinTicker = time.NewTicker(rejoinGracePeriod)
		o.log.Info().Msgf("rejoin grace period timer set to %s", rejoinGracePeriod)
		o.transitionTo(node.MonitorStateRejoin)
	}
}

// worker watch for local nmon updates
func (o *nmon) worker() {
	defer o.log.Debug().Msg("done")

	o.startedAt = time.Now()

	// cluster nodes at the time the worker starts
	initialNodes := o.config.GetStrings(key.New("cluster", "nodes"))
	for _, node := range initialNodes {
		o.nodeMonitor[node] = o.databus.GetNodeMonitor(node)
	}
	o.updateStats()
	o.setNodeOsPaths()
	o.updateIfChange()
	defer o.delete()

	if len(initialNodes) > 1 {
		o.startRejoin()
	} else {
		o.rejoinTicker = time.NewTicker(time.Millisecond)
		o.rejoinTicker.Stop()
		o.log.Info().Msgf("single cluster node, transition to idle")
		o.transitionTo(node.MonitorStateIdle)
	}

	statsTicker := time.NewTicker(statsInterval)
	defer statsTicker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case msgbus.ConfigFileUpdated:
				o.onConfigFileUpdated(c)
			case msgbus.NodeMonitorUpdated:
				o.onPeerNodeMonitorUpdated(c)
			case msgbus.NodeMonitorDeleted:
				o.onNodeMonitorDeleted(c)
			case msgbus.FrozenFileRemoved:
				o.onFrozenFileRemoved(c)
			case msgbus.FrozenFileUpdated:
				o.onFrozenFileUpdated(c)
			case msgbus.HbMessageTypeUpdated:
				o.onHbMessageTypeUpdated(c)
			case msgbus.JoinRequest:
				o.onJoinRequest(c)
			case msgbus.LeaveRequest:
				o.onLeaveRequest(c)
			case msgbus.SetNodeMonitor:
				o.onSetNodeMonitor(c)
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
		case <-o.rejoinTicker.C:
			o.onRejoinGracePeriodExpire()
		}
	}
}

func (o *nmon) onRejoinGracePeriodExpire() {
	nodeFrozenFile := filepath.Join(rawconfig.Paths.Var, "node", "frozen")
	frozen := file.ModTime(nodeFrozenFile)
	if frozen.Equal(time.Time{}) {
		f, err := os.OpenFile(nodeFrozenFile, os.O_RDONLY|os.O_CREATE, 0666)
		if err != nil {
			o.log.Error().Err(err).Msgf("rejoin grace period expired: freeze node")
			o.rejoinTicker.Reset(2 * time.Second)
			return
		}
		o.log.Info().Msgf("rejoin grace period expired: freeze node")
		if err := f.Close(); err != nil {
			o.log.Error().Err(err).Msgf("rejoin grace period expired: freeze node")
			o.rejoinTicker.Reset(2 * time.Second)
			return
		}
		o.transitionTo(node.MonitorStateIdle)
	} else {
		o.log.Info().Msgf("rejoin grace period expired: the node is already frozen")
		o.transitionTo(node.MonitorStateIdle)
	}
	o.rejoinTicker.Stop()
}

func (o *nmon) delete() {
	if err := o.databus.DelNodeMonitor(); err != nil {
		o.log.Error().Err(err).Msg("DelNodeMonitor")
	}
}

func (o *nmon) update() {
	newValue := o.state
	if err := o.databus.SetNodeMonitor(newValue); err != nil {
		o.log.Error().Err(err).Msg("SetNodeMonitor")
	}
	// update cache for localhost, we don't subscribe on self NodeMonitorUpdated
	o.nodeMonitor[o.localhost] = o.state
}

// updateIfChange log updates and publish new state value when changed
func (o *nmon) updateIfChange() {
	if !o.change {
		return
	}
	o.change = false
	o.state.StateUpdated = time.Now()
	previousVal := o.previousState
	newVal := o.state
	if newVal.State != previousVal.State {
		o.log.Info().Msgf("change monitor state %s -> %s", previousVal.State, newVal.State)
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
	for remoteNode, remoteNodeMonitor := range o.nodeMonitor {
		if remoteNode == o.localhost {
			continue
		}
		if remoteNodeMonitor.State.IsDoing() {
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

func (o *nmon) getStats() (node.Stats, error) {
	stats := node.Stats{}
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
	} else if err := o.databus.SetNodeStats(stats); err != nil {
		o.log.Error().Err(err).Msg("set stats")
	}
}

func (o *nmon) setNodeOsPaths() {
	if paths, err := san.GetPaths(); err != nil {
		o.log.Error().Err(err).Msg("get san paths")
	} else if err := o.databus.SetNodeOsPaths(paths); err != nil {
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
	data := *o.databus.GetNodesInfo()
	if err := nodesinfo.Save(data); err != nil {
		o.log.Error().Err(err).Msg("save nodes info")
	} else {
		o.log.Info().Msg("nodes info cache refreshed")
	}
}
