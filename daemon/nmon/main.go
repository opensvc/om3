// Package nmon is responsible for the local node states
//
// It provides the cluster data:
//
//	.cluster.node.<localhost>.monitor
//	.cluster.node.<localhost>.stats
//	.cluster.node.<localhost>.status (except gen)
//
// # It maintains the nodesinfo.json
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
	"sync"
	"time"

	"github.com/prometheus/procfs"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/bootid"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/san"
	"github.com/opensvc/om3/util/version"
)

type (
	nmon struct {
		// config is the node merged config
		config *xconfig.T

		drainDuration time.Duration

		// nodeConfig is the published node.Config. It is refreshed when config is
		// created or reloaded.
		nodeConfig node.Config

		state         node.Monitor
		previousState node.Monitor

		ctx          context.Context
		cancel       context.CancelFunc
		cmdC         chan any
		poolC        chan any
		databus      *daemondata.T
		bus          *pubsub.Bus
		log          zerolog.Logger
		rejoinTicker *time.Ticker
		startedAt    time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		// frozen is true when local node is frozen
		frozen bool

		nodeMonitor map[string]node.Monitor

		// clusterConfig is a cache of published ClusterConfigUpdated
		clusterConfig cluster.Config

		// livePeers is a map of peer nodes
		// exists when we receive msgbus.NodeMonitorUpdated
		// removed when we receive msgbus.ForgetPeer
		livePeers map[string]bool

		// arbitrators is a map for arbitratorConfig
		arbitrators map[string]arbitratorConfig

		localhost string
		change    bool

		sub *pubsub.Subscription

		labelLocalhost pubsub.Label

		// cacheNodesInfo is a map of nodes to node.NodeInfo, it is used to
		// maintain the nodesinfo.json file.
		// local values are computed by nmon.
		// peer values are updated from msgbus events NodeStatusLabelsUpdated, NodeConfigUpdated, NodeOsPathsUpdated
		// and ForgetPeer.
		cacheNodesInfo map[string]node.NodeInfo

		// nodeStatus is the node.Status for localhost that is the source of publication of msgbus.NodeStatusUpdated for
		// localhost.
		nodeStatus node.Status

		wg sync.WaitGroup
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

	// arbitratorInterval is the interval duration between 2 arbitrator checks
	arbitratorInterval = 60 * time.Second

	// To ensure no actions are performed during the split analyse
	// splitActionDelay + arbitratorCheckDuration must be lower than daemonenv.ReadyDuration

	// splitActionDelay is the duration to wait before call split action when
	// we are split and don't have majority
	splitActionDelay = daemonenv.ReadyDuration / 3

	// abitratorCheckDuration is the maximum duration to wait while
	// checking arbitrators
	arbitratorCheckDuration = daemonenv.ReadyDuration / 3

	// unexpectedDelay is a delay duration to wait on unexpected situation
	unexpectedDelay = 500 * time.Millisecond
)

func New(drainDuration time.Duration) *nmon {
	localhost := hostname.Hostname()
	return &nmon{
		drainDuration: drainDuration,
		state: node.Monitor{
			LocalExpect:  node.MonitorLocalExpectNone,
			GlobalExpect: node.MonitorGlobalExpectNone,
			State:        node.MonitorStateZero, // this prevents imon orchestration
		},
		previousState: node.Monitor{
			LocalExpect:  node.MonitorLocalExpectNone,
			GlobalExpect: node.MonitorGlobalExpectNone,
			State:        node.MonitorStateZero,
		},
		cmdC:        make(chan any),
		poolC:       make(chan any, 1),
		log:         log.Logger.With().Str("func", "nmon").Logger(),
		localhost:   localhost,
		change:      true,
		nodeMonitor: make(map[string]node.Monitor),
		nodeStatus: node.Status{
			Agent:    version.Version(),
			FrozenAt: time.Now(), // ensure initial frozen
		},
		frozen:    true, // ensure initial frozen
		livePeers: map[string]bool{localhost: true},

		cacheNodesInfo: map[string]node.NodeInfo{localhost: {}},
		labelLocalhost: pubsub.Label{"node", localhost},
	}
}

// Start launches the nmon worker goroutine
func (o *nmon) Start(parent context.Context) error {
	o.log.Info().Msg("omon starting")
	o.ctx, o.cancel = context.WithCancel(parent)
	o.databus = daemondata.FromContext(o.ctx)
	o.bus = pubsub.BusFromContext(o.ctx)

	// trigger an initial pool status eval
	o.poolC <- nil

	// we are responsible for publication or node config, don't wait for
	// first ConfigFileUpdated event to do the job.
	if err := o.loadAndPublishConfig(); err != nil {
		return err
	}

	bootID := bootid.Get()
	if len(bootID) > 0 {
		var (
			lastBootID     string
			fileLastBootID = filepath.Join(rawconfig.Paths.Var, "node", "last_boot_id")
		)
		if b, err := os.ReadFile(fileLastBootID); err == nil && len(b) > 0 {
			lastBootID = string(b)
			if lastBootID != bootID {
				o.log.Info().Msgf("first daemon startup since node boot")
				if osBootedWithOpensvcFreeze() {
					o.log.Info().Msgf("will freeze node due to kernel cmdline flag")
					err := o.crmFreeze()
					if err != nil {
						o.log.Error().Err(err).Msgf("freeze node due to kernel cmdline flag")
						return err
					}
				}
			}
		}
		if lastBootID != bootID {
			if err := os.WriteFile(fileLastBootID, []byte(bootID), 0644); err != nil {
				o.log.Error().Err(err).Msgf("unable to write %s '%s'", fileLastBootID, bootID)
			}
		}
	}

	o.setArbitratorConfig()

	o.startSubscriptions()
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		defer func() {
			go func() {
				tC := time.After(o.drainDuration)
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

	// pool status janitor
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		defer func() {
			go func() {
				tC := time.After(o.drainDuration)
				for {
					select {
					case <-tC:
						return
					case <-o.poolC:
					}
				}
			}()
		}()
		o.poolWorker()
	}()
	o.log.Info().Msg("omon started")
	return nil
}

func (o *nmon) Stop() error {
	o.log.Info().Msg("nmon stopping")
	defer o.log.Info().Msg("nmon stopped")
	o.cancel()
	o.wg.Wait()
	return nil
}

func (o *nmon) startSubscriptions() {
	sub := o.bus.Sub("nmon")

	// watching for ClusterConfigUpdated (so we get notified when cluster config file
	// has been changed and reloaded
	sub.AddFilter(&msgbus.ClusterConfigUpdated{})

	// We don't need to watch for ConfigFileUpdated on path cluster, instead
	// we watch for ClusterConfigUpdated.
	sub.AddFilter(&msgbus.ConfigFileUpdated{}, pubsub.Label{"path", ""})
	sub.AddFilter(&msgbus.ForgetPeer{})
	sub.AddFilter(&msgbus.HbMessageTypeUpdated{})
	sub.AddFilter(&msgbus.JoinRequest{}, o.labelLocalhost)
	sub.AddFilter(&msgbus.LeaveRequest{}, o.labelLocalhost)
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, pubsub.Label{"from", "peer"})
	sub.AddFilter(&msgbus.NodeFrozenFileRemoved{})
	sub.AddFilter(&msgbus.NodeFrozenFileUpdated{})
	sub.AddFilter(&msgbus.NodeMonitorDeleted{})
	sub.AddFilter(&msgbus.NodeMonitorUpdated{}, pubsub.Label{"from", "peer"})
	sub.AddFilter(&msgbus.NodeOsPathsUpdated{}, pubsub.Label{"from", "peer"})
	sub.AddFilter(&msgbus.NodeRejoin{}, o.labelLocalhost)
	sub.AddFilter(&msgbus.NodeStatusGenUpdates{}, o.labelLocalhost)
	sub.AddFilter(&msgbus.NodeStatusLabelsUpdated{}, pubsub.Label{"from", "peer"})
	sub.AddFilter(&msgbus.SetNodeMonitor{})
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
		// Arm the re-join grace period ticker.
		// The onHbMessageTypeUpdated() event handler can stop it.
		rejoinGracePeriod := o.nodeConfig.RejoinGracePeriod
		o.rejoinTicker = time.NewTicker(rejoinGracePeriod)
		o.log.Info().Msgf("rejoin grace period timer set to %s", rejoinGracePeriod)
		o.transitionTo(node.MonitorStateRejoin)
	}
}

func (o *nmon) touchLastShutdown() {
	// remember the last shutdown date via a file mtime
	if err := file.Touch(rawconfig.Paths.LastShutdown, time.Now()); err != nil {
		o.log.Error().Err(err).Msgf("touch %s", rawconfig.Paths.LastShutdown)
	} else {
		o.log.Info().Msgf("touch %s", rawconfig.Paths.LastShutdown)
	}
}

func (o *nmon) poolWorker() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-o.ctx.Done():
			return
		case <-o.poolC:
			o.loadPools()
		case <-ticker.C:
			o.loadPools()
		}
	}
}

// worker watch for local nmon updates
func (o *nmon) worker() {
	defer o.log.Debug().Msg("done")

	o.startedAt = time.Now()

	// cluster nodes at the time the worker starts
	initialNodes := o.config.GetStrings(key.New("cluster", "nodes"))
	for _, name := range initialNodes {
		if nodeMon := node.MonitorData.Get(name); nodeMon != nil {
			o.nodeMonitor[name] = *nodeMon
		} else {
			o.nodeMonitor[name] = node.Monitor{}
		}
	}
	o.updateStats()
	o.refreshSanPaths()
	o.updateIfChange()
	defer o.bus.Pub(&msgbus.NodeMonitorDeleted{Node: o.localhost}, o.labelLocalhost)
	defer node.MonitorData.Unset(o.localhost)

	o.getAndUpdateStatusArbitrator()

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
	arbitratorTicker := time.NewTicker(arbitratorInterval)
	defer arbitratorTicker.Stop()
	defer o.touchLastShutdown()

	// TODO refreshSanPaths should be refreshed on events,  on ticker ?
	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case *msgbus.ClusterConfigUpdated:
				o.onClusterConfigUpdated(c)
			case *msgbus.ConfigFileUpdated:
				o.onConfigFileUpdated(c)
			case *msgbus.ForgetPeer:
				o.onForgetPeer(c)
			case *msgbus.JoinRequest:
				o.onJoinRequest(c)
			case *msgbus.HbMessageTypeUpdated:
				o.onHbMessageTypeUpdated(c)
			case *msgbus.NodeConfigUpdated:
				o.onPeerNodeConfigUpdated(c)
			case *msgbus.NodeMonitorDeleted:
				o.onNodeMonitorDeleted(c)
			case *msgbus.NodeMonitorUpdated:
				o.onPeerNodeMonitorUpdated(c)
			case *msgbus.NodeOsPathsUpdated:
				o.onPeerNodeOsPathsUpdated(c)
			case *msgbus.NodeFrozenFileRemoved:
				o.onNodeFrozenFileRemoved(c)
			case *msgbus.NodeFrozenFileUpdated:
				o.onNodeFrozenFileUpdated(c)
			case *msgbus.NodeStatusLabelsUpdated:
				o.onPeerNodeStatusLabelsUpdated(c)
			case *msgbus.NodeStatusGenUpdates:
				o.onNodeStatusGenUpdates(c)
			case *msgbus.LeaveRequest:
				o.onLeaveRequest(c)
			case *msgbus.NodeRejoin:
				o.onNodeRejoin(c)
			case *msgbus.SetNodeMonitor:
				o.onSetNodeMonitor(c)
			}
		case i := <-o.cmdC:
			switch c := i.(type) {
			case cmdOrchestrate:
				o.onOrchestrate(c)
			}
		case <-statsTicker.C:
			o.updateStats()
		case <-arbitratorTicker.C:
			o.onArbitratorTicker()
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

func (o *nmon) update() {
	newValue := o.state
	node.MonitorData.Set(o.localhost, newValue.DeepCopy())
	o.bus.Pub(&msgbus.NodeMonitorUpdated{Node: o.localhost, Value: *newValue.DeepCopy()}, o.labelLocalhost)
	// update cache for localhost, we don't subscribe on self NodeMonitorUpdated
	o.nodeMonitor[o.localhost] = o.state
}

// updateIfChange log updates and publish new state value when changed
func (o *nmon) updateIfChange() {
	if !o.change {
		return
	}
	o.change = false
	o.state.StateUpdatedAt = time.Now()
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
		if *mem.MemTotal > 0 {
			stats.MemTotalMB = *mem.MemTotal / 1024
			stats.MemAvailPct = 100 * *mem.MemAvailable / *mem.MemTotal
		}
		if *mem.SwapTotal > 0 {
			stats.SwapTotalMB = *mem.SwapTotal / 1024
			stats.SwapAvailPct = 100 * *mem.SwapFree / *mem.SwapTotal
		}
		stats.Score += 100 + stats.MemAvailPct
		stats.Score += 2 * (100 + stats.SwapAvailPct)
	}
	stats.Score = stats.Score / 7

	return stats, nil
}

func (o *nmon) updateStats() {
	stats, err := o.getStats()
	if err != nil {
		o.log.Error().Err(err).Msg("get stats")
	}
	node.StatsData.Set(o.localhost, stats.DeepCopy())
	o.bus.Pub(&msgbus.NodeStatsUpdated{Node: o.localhost, Value: *stats.DeepCopy()}, o.labelLocalhost)
}

func (o *nmon) refreshSanPaths() {
	paths, err := san.GetPaths()
	if err != nil {
		o.log.Error().Err(err).Msg("get san paths")
		return
	}
	localNodeInfo := o.cacheNodesInfo[o.localhost]
	localNodeInfo.Paths = append(san.Paths{}, paths...)
	o.cacheNodesInfo[o.localhost] = localNodeInfo
	o.bus.Pub(&msgbus.NodeOsPathsUpdated{Node: o.localhost, Value: paths}, o.labelLocalhost)
}

func (o *nmon) onPeerNodeConfigUpdated(m *msgbus.NodeConfigUpdated) {
	peerNodeInfo := o.cacheNodesInfo[m.Node]
	peerNodeInfo.Env = m.Value.Env
	o.cacheNodesInfo[m.Node] = peerNodeInfo
	o.saveNodesInfo()
}

func (o *nmon) onPeerNodeOsPathsUpdated(m *msgbus.NodeOsPathsUpdated) {
	peerNodeInfo := o.cacheNodesInfo[m.Node]
	peerNodeInfo.Paths = m.Value
	o.cacheNodesInfo[m.Node] = peerNodeInfo
	o.saveNodesInfo()
}

func (o *nmon) onPeerNodeStatusLabelsUpdated(m *msgbus.NodeStatusLabelsUpdated) {
	peerNodeInfo := o.cacheNodesInfo[m.Node]
	peerNodeInfo.Labels = m.Value
	o.cacheNodesInfo[m.Node] = peerNodeInfo
	o.saveNodesInfo()
}

// onNodeStatusGenUpdates updates the localhost node status gen from daemondata
// msgbus.NodeStatusGenUpdates publication. It is daemondata that is responsible for
// localhost gens management. The value stored here is lazy updated for debug.
// We must not publish a msgbus.NodeStatusUpdated to avoid ping pong nmon<->data
func (o *nmon) onNodeStatusGenUpdates(m *msgbus.NodeStatusGenUpdates) {
	gens := make(map[string]uint64)
	for k, v := range m.Value {
		gens[k] = v
	}
	o.nodeStatus.Gen = gens
}

func (o *nmon) saveNodesInfo() {
	if err := nodesinfo.Save(o.cacheNodesInfo); err != nil {
		o.log.Error().Err(err).Msg("save nodes info")
	} else {
		o.log.Info().Msg("nodes info cache refreshed")
	}
}

func (o *nmon) loadConfig() error {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return err
	}
	localNodeInfo := o.cacheNodesInfo[o.localhost]
	localNodeInfo.Labels = n.Labels()
	o.config = n.MergedConfig()
	o.nodeConfig = o.getNodeConfig()
	localNodeInfo.Env = o.nodeConfig.Env
	o.cacheNodesInfo[o.localhost] = localNodeInfo
	return nil
}

func (o *nmon) loadAndPublishConfig() error {
	if err := o.loadConfig(); err != nil {
		return err
	}
	node.ConfigData.Set(o.localhost, o.nodeConfig.DeepCopy())
	o.bus.Pub(&msgbus.NodeConfigUpdated{Node: o.localhost, Value: o.nodeConfig}, o.labelLocalhost)
	localNodeInfo := o.cacheNodesInfo[o.localhost]
	o.bus.Pub(&msgbus.NodeStatusLabelsUpdated{Node: o.localhost, Value: localNodeInfo.Labels.DeepCopy()}, o.labelLocalhost)
	o.nodeStatus.Labels = localNodeInfo.Labels
	node.StatusData.Set(o.localhost, o.nodeStatus.DeepCopy())
	o.bus.Pub(&msgbus.NodeStatusUpdated{Node: o.localhost, Value: *o.nodeStatus.DeepCopy()}, o.labelLocalhost)
	paths := localNodeInfo.Paths.DeepCopy()
	node.OsPathsData.Set(o.localhost, &paths)
	o.bus.Pub(&msgbus.NodeOsPathsUpdated{Node: o.localhost, Value: localNodeInfo.Paths.DeepCopy()}, o.labelLocalhost)
	select {
	case o.poolC <- nil:
	default:
	}
	return nil
}

func (o *nmon) loadPools() {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		o.log.Warn().Err(err).Msg("load pools status")
		return
	}
	renewed := make(map[string]any)
	for _, p := range n.Pools() {
		data := pool.GetStatus(p, true)
		renewed[data.Name] = nil
		pool.StatusData.Set(data.Name, &data)
	}
	for _, e := range pool.StatusData.GetAll() {
		if _, ok := renewed[e.Name]; !ok {
			pool.StatusData.Unset(e.Name)
		}
	}
}
