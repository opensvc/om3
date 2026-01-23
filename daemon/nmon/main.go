// Package nmon is responsible for the local node states
//
// It provides the cluster data:
//
//	.cluster.node.<localhost>.monitor
//	.cluster.node.<localhost>.stats
//	.cluster.node.<localhost>.status (except gen)
//
// # It maintains the nodes_info.json
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
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/procfs"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/hbsecret"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/bootid"
	"github.com/opensvc/om3/v3/util/btime"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/san"
	"github.com/opensvc/om3/v3/util/version"
)

type (
	Manager struct {
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
		publisher    pubsub.Publisher
		log          *plog.Logger
		rejoinTicker *time.Ticker
		startedAt    time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		// frozen is true when local node is frozen
		frozen bool

		nodeMonitor map[string]node.Monitor

		// clusterConfig is a cache of published ClusterConfigUpdated
		clusterConfig cluster.Config

		// hbSecret defines the local heartbeat secret
		hbSecret hbsecret.Secret

		// livePeers is a map of peer nodes
		// exists when we receive msgbus.NodeMonitorUpdated
		// removed when we receive msgbus.ForgetPeer
		livePeers map[string]bool

		// arbitrators is a map for arbitratorConfig
		arbitrators map[string]arbitratorConfig

		localhost string
		change    bool

		sub   *pubsub.Subscription
		subQS pubsub.QueueSizer

		labelLocalhost pubsub.Label

		// cacheNodesInfo is a map of nodes to node.NodeInfo, it is used to
		// maintain the nodes_info.json file.
		// local values are computed by nmon.
		// peer values are updated from msgbus events NodeStatusLabelsUpdated, NodeConfigUpdated, NodeOsPathsUpdated
		// and ForgetPeer.
		cacheNodesInfo nodesinfo.M

		// nodeStatus is the node.Status for localhost that is the source of publication of msgbus.NodeStatusUpdated for
		// localhost.
		nodeStatus node.Status

		wg sync.WaitGroup

		hbSecretRotating           bool
		hbSecretRotatingAt         time.Time
		hbSecretRotatingUUID       uuid.UUID
		hbSecretChecksumByNodename map[string]string
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

func NewManager(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	m := &Manager{
		drainDuration: drainDuration,
		state: node.Monitor{
			LocalExpect:  node.MonitorLocalExpectNone,
			GlobalExpect: node.MonitorGlobalExpectNone,
			State:        node.MonitorStateInit, // this prevents imon orchestration
		},
		previousState: node.Monitor{
			LocalExpect:  node.MonitorLocalExpectNone,
			GlobalExpect: node.MonitorGlobalExpectNone,
			State:        node.MonitorStateInit,
		},
		cmdC:        make(chan any),
		poolC:       make(chan any, 1),
		log:         plog.NewDefaultLogger().Attr("pkg", "daemon/nmon").WithPrefix("daemon: nmon: "),
		localhost:   localhost,
		change:      true,
		nodeMonitor: make(map[string]node.Monitor),
		nodeStatus: node.Status{
			Agent:    version.Version(),
			FrozenAt: time.Now(), // ensure initial frozen
		},
		frozen:    true, // ensure initial frozen
		livePeers: map[string]bool{localhost: true},

		cacheNodesInfo: nodesinfo.M{localhost: {}},
		labelLocalhost: pubsub.Label{"node", localhost},

		subQS: subQS,

		hbSecretChecksumByNodename: make(map[string]string),
	}
	if bt, err := btime.GetBootTime(); err != nil {
		m.log.Warnf("get boot time: %s", err)
	} else {
		m.nodeStatus.BootedAt = bt
	}
	return m
}

// Start launches the nmon worker goroutine
func (t *Manager) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.databus = daemondata.FromContext(t.ctx)
	t.publisher = pubsub.PubFromContext(t.ctx)

	// trigger an initial pool status eval
	t.poolC <- nil

	// load the nodesinfo cache to avoid losing the cached information
	// of peer nodes.
	// TODO: need publish peer info from loaded cache ?
	if data, err := nodesinfo.Load(); errors.Is(err, os.ErrNotExist) {
		t.log.Infof("nodes info cache does not exist ... init with only the local node info")
	} else if err != nil {
		t.log.Warnf("nodes info cache load error: %s ... reset with only the local node info", err)
	} else {
		data[t.localhost] = t.cacheNodesInfo[t.localhost]
		t.cacheNodesInfo = data
	}

	// bootstrap initial node config
	if err := t.bootstrapConfig(); err != nil {
		return fmt.Errorf("bootstrap initial node config: %w", err)
	}

	// we are responsible for publication or node config, don't wait for
	// first ConfigFileUpdated event to do the job.
	if err := t.loadConfigAndPublish(); err != nil {
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
				t.log.Infof("first daemon startup since node boot")
				if osBootedWithOpensvcFreeze() {
					t.log.Infof("will freeze node due to kernel cmdline flag")
					err := t.crmFreeze()
					if err != nil {
						t.log.Errorf("freeze node due to kernel cmdline flag: %s", err)
						return err
					}
				}
			}
		}
		if lastBootID != bootID {
			if err := os.WriteFile(fileLastBootID, []byte(bootID), 0644); err != nil {
				t.log.Errorf("unable to write %s '%s': %s", fileLastBootID, bootID, err)
			}
		}
	}

	t.setArbitratorConfig()

	t.startSubscriptions()
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer func() {
			go func() {
				tC := time.After(t.drainDuration)
				for {
					select {
					case <-tC:
						return
					case <-t.cmdC:
					}
				}
			}()
			if err := t.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Errorf("subscription stop: %s", err)
			}
		}()
		t.worker()
	}()

	// pool status janitor
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer func() {
			go func() {
				tC := time.After(t.drainDuration)
				for {
					select {
					case <-tC:
						return
					case <-t.poolC:
					}
				}
			}()
		}()
		t.poolWorker()
	}()
	t.log.Infof("started")
	return nil
}

func (t *Manager) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.nmon", t.subQS)

	// watching for ClusterConfigUpdated (so we get notified when cluster config file
	// has been changed and reloaded
	sub.AddFilter(&msgbus.ClusterConfigUpdated{})

	// We don't need to watch for ConfigFileUpdated on path cluster, instead
	// we watch for ClusterConfigUpdated.
	sub.AddFilter(&msgbus.ConfigFileUpdated{}, pubsub.Label{"path", ""})

	sub.AddFilter(&msgbus.DaemonListenerUpdated{})

	sub.AddFilter(&msgbus.ForgetPeer{})
	sub.AddFilter(&msgbus.HeartbeatMessageTypeUpdated{})
	sub.AddFilter(&msgbus.HeartbeatRotateRequest{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, pubsub.Label{"path", naming.SecHb.String()})
	sub.AddFilter(&msgbus.JoinRequest{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.LeaveRequest{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, pubsub.Label{"from", "peer"})
	sub.AddFilter(&msgbus.NodeFrozenFileRemoved{})
	sub.AddFilter(&msgbus.NodeFrozenFileUpdated{})
	sub.AddFilter(&msgbus.NodeMonitorDeleted{})
	sub.AddFilter(&msgbus.NodeMonitorUpdated{}, pubsub.Label{"from", "peer"})
	sub.AddFilter(&msgbus.NodeOsPathsUpdated{}, pubsub.Label{"from", "peer"})
	sub.AddFilter(&msgbus.NodeRejoin{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.NodeStatusGenUpdates{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.NodeStatusLabelsUpdated{}, pubsub.Label{"from", "peer"})
	sub.AddFilter(&msgbus.SetNodeMonitor{})
	sub.Start()
	t.sub = sub
}

func (t *Manager) startRejoin() {
	hbMessageType := t.databus.GetHbMessageType()
	l := missingNodes(hbMessageType.Nodes, hbMessageType.JoinedNodes)
	if (hbMessageType.Type == "patch") && len(l) == 0 {
		// Skip the rejoin state phase.
		t.rejoinTicker = time.NewTicker(time.Second)
		t.rejoinTicker.Stop()
		t.transitionTo(node.MonitorStateIdle)
	} else {
		// Begin the rejoin state phase.
		// Arm the re-join grace period ticker.
		// The onHeartbeatMessageTypeUpdated() event handler can stop it.
		rejoinGracePeriod := t.nodeConfig.RejoinGracePeriod
		t.rejoinTicker = time.NewTicker(rejoinGracePeriod)
		t.log.Infof("rejoin grace period timer set to %s", rejoinGracePeriod)
		t.transitionTo(node.MonitorStateRejoin)
	}
}

func (t *Manager) touchLastShutdown() {
	// remember the last shutdown date via a file mtime
	if err := file.Touch(rawconfig.Paths.LastShutdown, time.Now()); err != nil {
		t.log.Errorf("touch %s: %s", rawconfig.Paths.LastShutdown, err)
	} else {
		t.log.Infof("touch %s", rawconfig.Paths.LastShutdown)
	}
}

func (t *Manager) poolWorker() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-t.poolC:
			t.loadPools()
		case <-ticker.C:
			t.loadPools()
		}
	}
}

// worker watch for local nmon updates
func (t *Manager) worker() {
	defer t.log.Tracef("done")

	t.startedAt = time.Now()

	// cluster nodes at the time the worker starts
	initialNodes := t.config.GetStrings(key.New("cluster", "nodes"))
	for _, name := range initialNodes {
		if nodeMon := node.MonitorData.GetByNode(name); nodeMon != nil {
			t.nodeMonitor[name] = *nodeMon
		} else {
			t.nodeMonitor[name] = node.Monitor{}
		}
	}
	t.updateStats()
	t.refreshSanPaths()
	t.updateIfChange()
	defer t.publisher.Pub(&msgbus.NodeMonitorDeleted{Node: t.localhost}, t.labelLocalhost)
	defer node.MonitorData.Unset(t.localhost)

	t.getAndUpdateStatusArbitrator()

	if len(initialNodes) > 1 {
		t.startRejoin()
	} else {
		t.rejoinTicker = time.NewTicker(time.Millisecond)
		t.rejoinTicker.Stop()
		t.log.Infof("single cluster node, transition to idle")
		t.transitionTo(node.MonitorStateIdle)
	}

	statsTicker := time.NewTicker(statsInterval)
	defer statsTicker.Stop()
	arbitratorTicker := time.NewTicker(arbitratorInterval)
	defer arbitratorTicker.Stop()
	defer t.touchLastShutdown()

	// TODO refreshSanPaths should be refreshed on events,  on ticker ?
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			switch c := i.(type) {
			case *msgbus.ClusterConfigUpdated:
				t.onClusterConfigUpdated(c)
			case *msgbus.ConfigFileUpdated:
				t.onConfigFileUpdated(c)
			case *msgbus.DaemonListenerUpdated:
				t.onDaemonListenerUpdated(c)
			case *msgbus.ForgetPeer:
				t.onForgetPeer(c)
			case *msgbus.JoinRequest:
				t.onJoinRequest(c)
			case *msgbus.HeartbeatRotateRequest:
				t.onHeartbeatRotateRequest(c)
			case *msgbus.HeartbeatMessageTypeUpdated:
				t.onHeartbeatMessageTypeUpdated(c)
			case *msgbus.InstanceConfigUpdated:
				t.onInstanceConfigUpdated(c)
			case *msgbus.NodeConfigUpdated:
				t.onPeerNodeConfigUpdated(c)
			case *msgbus.NodeMonitorDeleted:
				t.onNodeMonitorDeleted(c)
			case *msgbus.NodeMonitorUpdated:
				t.onPeerNodeMonitorUpdated(c)
			case *msgbus.NodeOsPathsUpdated:
				t.onPeerNodeOsPathsUpdated(c)
			case *msgbus.NodeFrozenFileRemoved:
				t.onNodeFrozenFileRemoved(c)
			case *msgbus.NodeFrozenFileUpdated:
				t.onNodeFrozenFileUpdated(c)
			case *msgbus.NodeStatusLabelsUpdated:
				t.onPeerNodeStatusLabelsUpdated(c)
			case *msgbus.NodeStatusGenUpdates:
				t.onNodeStatusGenUpdates(c)
			case *msgbus.LeaveRequest:
				t.onLeaveRequest(c)
			case *msgbus.NodeRejoin:
				t.onNodeRejoin(c)
			case *msgbus.SetNodeMonitor:
				t.onSetNodeMonitor(c)
			}
		case i := <-t.cmdC:
			switch c := i.(type) {
			case cmdOrchestrate:
				t.onOrchestrate(c)
			}
		case <-statsTicker.C:
			t.updateStats()
		case <-arbitratorTicker.C:
			t.onArbitratorTicker()
		case <-t.rejoinTicker.C:
			t.onRejoinGracePeriodExpire()
		}
	}
}

func (t *Manager) onRejoinGracePeriodExpire() {
	nodeFrozenFile := filepath.Join(rawconfig.Paths.Var, "node", "frozen")
	frozen := file.ModTime(nodeFrozenFile)
	if frozen.Equal(time.Time{}) {
		f, err := os.OpenFile(nodeFrozenFile, os.O_RDONLY|os.O_CREATE, 0666)
		if err != nil {
			t.log.Errorf("rejoin grace period expired: freeze node: %s", err)
			t.rejoinTicker.Reset(2 * time.Second)
			return
		}
		t.log.Infof("rejoin grace period expired: freeze node")
		if err := f.Close(); err != nil {
			t.log.Errorf("rejoin grace period expired: freeze node: %s", err)
			t.rejoinTicker.Reset(2 * time.Second)
			return
		}
		t.transitionTo(node.MonitorStateIdle)
	} else {
		t.log.Infof("rejoin grace period expired: the node is already frozen")
		t.transitionTo(node.MonitorStateIdle)
	}
	t.rejoinTicker.Stop()
}

func (t *Manager) update() {
	newValue := t.state
	node.MonitorData.Set(t.localhost, newValue.DeepCopy())
	t.publisher.Pub(&msgbus.NodeMonitorUpdated{Node: t.localhost, Value: *newValue.DeepCopy()}, t.labelLocalhost)
	// update cache for localhost, we don't subscribe on self NodeMonitorUpdated
	t.nodeMonitor[t.localhost] = t.state
}

// updateIfChange log updates and publish new state value when changed
func (t *Manager) updateIfChange() {
	if !t.change {
		return
	}
	t.change = false
	now := time.Now()
	t.state.UpdatedAt = now
	previousVal := t.previousState
	newVal := t.state
	if newVal.State != previousVal.State {
		t.state.StateUpdatedAt = now
		t.log.Infof("change monitor state %s -> %s", previousVal.State, newVal.State)
	}
	if newVal.GlobalExpect != previousVal.GlobalExpect {
		t.log.Infof("change monitor global expect %s -> %s", previousVal.GlobalExpect, newVal.GlobalExpect)
	}
	if newVal.LocalExpect != previousVal.LocalExpect {
		t.state.LocalExpectUpdatedAt = now
		t.log.Infof("change monitor local expect %s -> %s", previousVal.LocalExpect, newVal.LocalExpect)
	}
	t.previousState = t.state
	t.update()
}

func (t *Manager) hasOtherNodeActing() bool {
	for remoteNode, remoteNodeMonitor := range t.nodeMonitor {
		if remoteNode == t.localhost {
			continue
		}
		if remoteNodeMonitor.State.IsDoing() {
			return true
		}
	}
	return false
}

func (t *Manager) createPendingWithCancel() {
	t.pendingCtx, t.pendingCancel = context.WithCancel(t.ctx)
}

func (t *Manager) createPendingWithDuration(duration time.Duration) {
	t.pendingCtx, t.pendingCancel = context.WithTimeout(t.ctx, duration)
}

func (t *Manager) clearPending() {
	if t.pendingCancel != nil {
		t.pendingCancel()
		t.pendingCancel = nil
		t.pendingCtx = nil
	}
}

func (t *Manager) getStats() (node.Stats, error) {
	stats := node.Stats{}
	if runtime.GOOS != "linux" {
		return stats, nil
	}

	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return stats, err
	}
	if load, err := fs.LoadAvg(); err != nil {
		return stats, err
	} else {
		stats.Load15M = load.Load15
		stats.Score += int(100 / math.Max(load.Load15, 1))
	}
	if mem, err := fs.Meminfo(); err != nil {
		return stats, err
	} else {
		if *mem.MemTotal > 0 {
			stats.MemTotalMB = *mem.MemTotal / 1024
			stats.MemAvailPct = int(100 * *mem.MemAvailable / *mem.MemTotal)
		}
		if *mem.SwapTotal > 0 {
			stats.SwapTotalMB = *mem.SwapTotal / 1024
			stats.SwapAvailPct = int(100 * *mem.SwapFree / *mem.SwapTotal)
		}
		stats.Score += 100 + stats.MemAvailPct
		stats.Score += 2 * (100 + stats.SwapAvailPct)
	}
	stats.Score = stats.Score / 7

	return stats, nil
}

func (t *Manager) updateStats() {
	stats, err := t.getStats()
	if err != nil {
		t.log.Errorf("get stats: %s", err)
	}
	node.StatsData.Set(t.localhost, stats.DeepCopy())
	t.publisher.Pub(&msgbus.NodeStatsUpdated{Node: t.localhost, Value: *stats.DeepCopy()}, t.labelLocalhost)
	if changed := t.updateIsOverloaded(stats); changed {
		t.publishNodeStatus()
	}
}

func (t *Manager) updateIsOverloaded(stats node.Stats) bool {
	isOverloaded := t.isOverloaded(stats)
	if isOverloaded == t.nodeStatus.IsOverloaded {
		return false
	}
	t.nodeStatus.IsOverloaded = isOverloaded
	fmtVals := func(pct, minPct int) string {
		if minPct <= 0 {
			return fmt.Sprintf("%d%%/-", pct)
		} else {
			return fmt.Sprintf("%d%%/%d%%", pct, minPct)
		}
	}
	if isOverloaded {
		t.publisher.Pub(&msgbus.EnterOverloadPeriod{}, t.labelLocalhost)
		t.log.Warnf("node is now overloaded: avail mem=%s swap=%s",
			fmtVals(stats.MemAvailPct, t.nodeConfig.MinAvailMemPct),
			fmtVals(stats.SwapAvailPct, t.nodeConfig.MinAvailSwapPct),
		)
	} else {
		t.publisher.Pub(&msgbus.LeaveOverloadPeriod{}, t.labelLocalhost)
		t.log.Infof("node is no longer overloaded: avail mem=%s swap=%s",
			fmtVals(stats.MemAvailPct, t.nodeConfig.MinAvailMemPct),
			fmtVals(stats.SwapAvailPct, t.nodeConfig.MinAvailSwapPct),
		)
	}
	return true
}

func (t *Manager) isOverloaded(stats node.Stats) bool {
	if (t.nodeConfig.MinAvailMemPct > 0) && (stats.MemAvailPct < t.nodeConfig.MinAvailMemPct) {
		return true
	}
	if (t.nodeConfig.MinAvailSwapPct > 0) && (stats.SwapAvailPct < t.nodeConfig.MinAvailSwapPct) {
		return true
	}
	return false
}

func (t *Manager) refreshSanPaths() {
	paths, err := san.GetPaths()
	if err != nil {
		t.log.Errorf("get san paths: %s", err)
		return
	}
	localNodeInfo := t.cacheNodesInfo[t.localhost]
	localNodeInfo.Paths = append(san.Paths{}, paths...)
	t.cacheNodesInfo[t.localhost] = localNodeInfo
	t.publisher.Pub(&msgbus.NodeOsPathsUpdated{Node: t.localhost, Value: paths}, t.labelLocalhost)
}

func (t *Manager) onDaemonListenerUpdated(m *msgbus.DaemonListenerUpdated) {
	if m.Value.State == "stopped" {
		// Don't update nodes info file when peer listener is stopped
		// TODO: verify the skip update nodes info file rule
		nodeInfo := t.cacheNodesInfo[m.Node]
		nodeInfo.Lsnr.UpdatedAt = m.Value.UpdatedAt
		nodeInfo.Lsnr.State = m.Value.State
		return
	}
	nodeInfo := t.cacheNodesInfo[m.Node]
	nodeInfo.Lsnr = m.Value
	t.cacheNodesInfo[m.Node] = nodeInfo
	t.saveNodesInfo()
}

func (t *Manager) onPeerNodeConfigUpdated(m *msgbus.NodeConfigUpdated) {
	peerNodeInfo := t.cacheNodesInfo[m.Node]
	peerNodeInfo.Env = m.Value.Env
	t.cacheNodesInfo[m.Node] = peerNodeInfo
	t.saveNodesInfo()
}

func (t *Manager) onPeerNodeOsPathsUpdated(m *msgbus.NodeOsPathsUpdated) {
	peerNodeInfo := t.cacheNodesInfo[m.Node]
	peerNodeInfo.Paths = m.Value
	t.cacheNodesInfo[m.Node] = peerNodeInfo
	t.saveNodesInfo()
}

func (t *Manager) onPeerNodeStatusLabelsUpdated(m *msgbus.NodeStatusLabelsUpdated) {
	peerNodeInfo := t.cacheNodesInfo[m.Node]
	changed := !maps.Equal(peerNodeInfo.Labels, m.Value)
	peerNodeInfo.Labels = m.Value
	t.cacheNodesInfo[m.Node] = peerNodeInfo
	t.saveNodesInfo()
	if changed {
		t.publisher.Pub(&msgbus.NodeStatusLabelsCommited{Node: m.Node, Value: m.Value.DeepCopy()}, t.labelLocalhost)
	}
}

// onNodeStatusGenUpdates updates the localhost node status gen from daemondata
// msgbus.NodeStatusGenUpdates publication. It is daemondata that is responsible for
// localhost gens management. The value stored here is lazy updated for debug.
// We must not publish a msgbus.NodeStatusUpdated to avoid ping pong nmon<->data
func (t *Manager) onNodeStatusGenUpdates(m *msgbus.NodeStatusGenUpdates) {
	gens := make(node.Gen)
	for k, v := range m.Value {
		gens[k] = v
	}
	t.nodeStatus.Gen = gens
}

func (t *Manager) saveNodesInfo() {
	if err := nodesinfo.Save(t.cacheNodesInfo); err != nil {
		t.log.Errorf("save nodes info: %s", err)
	} else {
		t.log.Infof("nodes info cache refreshed %s", t.cacheNodesInfo.Keys())
	}
}

func (t *Manager) publishNodeStatus() {
	node.StatusData.Set(t.localhost, t.nodeStatus.DeepCopy())
	t.publisher.Pub(&msgbus.NodeStatusUpdated{Node: t.localhost, Value: *t.nodeStatus.DeepCopy()}, t.labelLocalhost)
}

// bootstrapConfig initializes the node configuration and generates or retrieves
// the node persistent reservation key if necessary.
func (t *Manager) bootstrapConfig() error {
	n, err := object.NewNode(object.WithVolatile(false))
	if err != nil {
		return err
	}
	t.config = n.MergedConfig()
	nodeConfig := t.getNodeConfig()
	if nodeConfig.PRKey != "" {
		t.log.Tracef("node prkey %s", nodeConfig.PRKey)
		return nil
	}
	prKey, err := n.PRKey()
	if err != nil {
		return fmt.Errorf("unable to initialize node prkey, %w", err)
	}
	t.log.Infof("initialized node prkey: %s", prKey)
	return nil
}

func (t *Manager) loadConfig() error {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return err
	}
	localNodeInfo := t.cacheNodesInfo[t.localhost]
	localNodeInfo.Labels = n.Labels()
	t.config = n.MergedConfig()
	t.nodeConfig = t.getNodeConfig()
	localNodeInfo.Env = t.nodeConfig.Env

	if lsnr := daemonsubsystem.DataListener.Get(t.localhost); lsnr != nil {
		localNodeInfo.Lsnr = *lsnr
	}
	t.cacheNodesInfo[t.localhost] = localNodeInfo
	return nil
}

func (t *Manager) loadConfigAndPublish() error {
	if err := t.loadConfig(); err != nil {
		return err
	}

	node.ConfigData.Set(t.localhost, t.nodeConfig.DeepCopy())
	t.publisher.Pub(&msgbus.NodeConfigUpdated{Node: t.localhost, Value: t.nodeConfig}, t.labelLocalhost)

	if stats := node.StatsData.GetByNode(t.localhost); stats != nil && stats.MemTotalMB != 0 {
		t.updateIsOverloaded(*stats)
	}

	var labelsChanged, pathsChanged bool

	localNodeInfo := t.cacheNodesInfo[t.localhost]
	if !maps.Equal(localNodeInfo.Labels, t.nodeStatus.Labels) {
		t.nodeStatus.Labels = localNodeInfo.Labels
		labelsChanged = true
	}
	paths := node.OsPathsData.GetByNode(t.localhost)
	if paths == nil || !slices.Equal(localNodeInfo.Paths, *paths) {
		node.OsPathsData.Set(t.localhost, localNodeInfo.Paths.DeepCopy())
		pathsChanged = true
	}
	if labelsChanged || pathsChanged {
		t.saveNodesInfo()
	}
	if labelsChanged {
		t.publisher.Pub(&msgbus.NodeStatusLabelsUpdated{Node: t.localhost, Value: localNodeInfo.Labels.DeepCopy()}, t.labelLocalhost)
		t.publisher.Pub(&msgbus.NodeStatusLabelsCommited{Node: t.localhost, Value: localNodeInfo.Labels.DeepCopy()}, t.labelLocalhost)
	}
	if pathsChanged {
		t.publisher.Pub(&msgbus.NodeOsPathsUpdated{Node: t.localhost, Value: *localNodeInfo.Paths.DeepCopy()}, t.labelLocalhost)
	}

	t.updateSpeaker()
	t.publishNodeStatus()

	select {
	case t.poolC <- nil:
	default:
	}
	return nil
}

func (t *Manager) loadPools() {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		t.log.Warnf("load pools status: %s", err)
		return
	}
	renewed := make(map[string]any)

	var renewedMu sync.Mutex
	var wg sync.WaitGroup

	renew := func(p pool.Pooler) {
		defer wg.Done()

		ctx, cancel := context.WithTimeout(t.ctx, 10*time.Second)
		defer cancel()

		poolName := p.Name()
		data := pool.GetStatus(ctx, p, true)

		if ctx.Err() != nil {
			t.log.Warnf("loading pool '%s' status: %s", poolName, ctx.Err())
			return
		}
		t.log.Infof("pool '%s' status loaded", poolName)

		renewedMu.Lock()
		renewed[poolName] = nil
		renewedMu.Unlock()

		pool.StatusData.Set(poolName, t.localhost, data.DeepCopy())
		t.publisher.Pub(&msgbus.NodePoolStatusUpdated{Node: t.localhost, Name: poolName, Value: data}, t.labelLocalhost)

	}
	for _, p := range n.Pools() {
		wg.Add(1)
		go renew(p)
	}

	wg.Wait()
	for _, e := range pool.StatusData.GetByNode(t.localhost) {
		renewedMu.Lock()
		_, ok := renewed[e.Name]
		renewedMu.Unlock()
		if !ok {
			pool.StatusData.Unset(e.Name, t.localhost)
		}
	}
}
