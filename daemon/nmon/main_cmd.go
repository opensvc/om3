package nmon

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"time"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/errcontext"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/toc"
)

var (
	// MinMaxParallel is the minimum value of the setting of maximum number of CRM
	// actions allowed to run in parallel.
	MinMaxParallel = 2

	splitActions = map[string]func() error{
		"crash":    toc.Crash,
		"reboot":   toc.Reboot,
		"disabled": func() error { return nil },
	}
)

// onClusterConfigUpdated updates local config and arbitrator status after detected
// local cluster config updates
//
// it updates cluster config cache from event value
// it loads and publish config (some common settings such as node split_action keyword...)
// it updates arbitrators config
// then refresh arbitrator status
func (t *Manager) onClusterConfigUpdated(c *msgbus.ClusterConfigUpdated) {
	t.clusterConfig = c.Value

	if err := t.loadAndPublishConfig(); err != nil {
		t.log.Errorf("load and publish config from cluster config updated event: %s", err)
	}
	t.setArbitratorConfig()

	t.getAndUpdateStatusArbitrator()

	// recompute rejoin ticker, perhaps RejoinGracePeriod has been changed
	t.checkRejoinTicker()
}

// onConfigFileUpdated reloads the config parser and emits the updated
// node.Config data in a NodeConfigUpdated event, so other go routine
// can just subscribe to this event to maintain the cache of keywords
// they care about.
func (t *Manager) onConfigFileUpdated(_ *msgbus.ConfigFileUpdated) {
	if err := t.loadAndPublishConfig(); err != nil {
		t.log.Errorf("load and publish config from node config file updated event: %s", err)
		return
	}

	// env might have changed. nmon is responsible for updating nodes_info.json
	t.saveNodesInfo()

	// recompute rejoin ticker, perhaps RejoinGracePeriod has been changed
	t.checkRejoinTicker()
}

func (t *Manager) getNodeConfig() node.Config {
	var (
		keyMaintenanceGracePeriod = key.New("node", "maintenance_grace_period")
		keyMaxParallel            = key.New("node", "max_parallel")
		keyReadyPeriod            = key.New("node", "ready_period")
		keyRejoinGracePeriod      = key.New("node", "rejoin_grace_period")
		keyEnv                    = key.New("node", "env")
		keySplitAction            = key.New("node", "split_action")
	)
	cfg := node.Config{}
	if d := t.config.GetDuration(keyMaintenanceGracePeriod); d != nil {
		cfg.MaintenanceGracePeriod = *d
	}
	if d := t.config.GetDuration(keyReadyPeriod); d != nil {
		cfg.ReadyPeriod = *d
	}
	if d := t.config.GetDuration(keyRejoinGracePeriod); d != nil {
		cfg.RejoinGracePeriod = *d
	}
	cfg.MaxParallel = t.config.GetInt(keyMaxParallel)
	cfg.Env = t.config.GetString(keyEnv)
	cfg.SplitAction = t.config.GetString(keySplitAction)

	if cfg.MaxParallel == 0 {
		cfg.MaxParallel = runtime.NumCPU()
	}
	if cfg.MaxParallel < MinMaxParallel {
		cfg.MaxParallel = MinMaxParallel
	}

	return cfg
}

func (t *Manager) checkRejoinTicker() {
	if t.state.State != node.MonitorStateRejoin {
		return
	}
	if left := t.startedAt.Add(t.nodeConfig.RejoinGracePeriod).Sub(time.Now()); left <= 0 {
		t.log.Infof("the new rejoin grace period is already expired")
		t.rejoinTicker.Reset(100 * time.Millisecond)
		return
	} else {
		t.rejoinTicker.Reset(left)
		t.log.Infof("rejoin grace period timer reset to %s", left)
	}
}

func (t *Manager) onSetNodeMonitor(c *msgbus.SetNodeMonitor) {
	doState := func() error {
		if c.Value.State == nil {
			return nil
		}
		// sanity check the state value
		if _, ok := node.MonitorStateStrings[*c.Value.State]; !ok {
			err := fmt.Errorf("%w: %s", node.ErrInvalidState, *c.Value.State)
			t.log.Warnf("%s", err)
			return err
		}

		if t.state.State == *c.Value.State {
			err := fmt.Errorf("%w: state is already %s", node.ErrSameState, *c.Value.State)
			t.log.Infof("%s", err)
			return err
		}

		t.log.Infof("set state %s -> %s", t.state.State, *c.Value.State)
		t.change = true
		t.state.State = *c.Value.State
		return nil
	}

	doLocalExpect := func() error {
		if c.Value.LocalExpect == nil {
			return nil
		}
		// sanity check the local expect value
		if _, ok := node.MonitorLocalExpectStrings[*c.Value.LocalExpect]; !ok {
			err := fmt.Errorf("%w: %s", node.ErrInvalidLocalExpect, *c.Value.LocalExpect)
			t.log.Warnf("%s", err)
			return err
		}

		if t.state.LocalExpect == *c.Value.LocalExpect {
			err := fmt.Errorf("%w: %s", node.ErrSameLocalExpect, *c.Value.LocalExpect)
			t.log.Infof("%s", err)
			return err
		}

		t.log.Infof("set local expect %s -> %s", t.state.LocalExpect, *c.Value.LocalExpect)
		t.change = true
		t.state.LocalExpect = *c.Value.LocalExpect
		return nil
	}

	doGlobalExpect := func() error {
		if c.Value.GlobalExpect == nil {
			return nil
		}
		if _, ok := node.MonitorGlobalExpectStrings[*c.Value.GlobalExpect]; !ok {
			t.log.Warnf("invalid set node monitor local expect: %s", *c.Value.GlobalExpect)
			return nil
		}
		if *c.Value.GlobalExpect != node.MonitorGlobalExpectAborted {
			for nodename, data := range t.nodeMonitor {
				if data.GlobalExpect == *c.Value.GlobalExpect {
					err := fmt.Errorf("%w: %s: more recent value %s on node %s", node.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, data.GlobalExpect, nodename)
					t.log.Infof("%s", err)
					return err
				}
				if !data.State.IsRankable() {
					err := fmt.Errorf("%w: %s: node %s state is %s", node.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, nodename, data.State)
					t.log.Errorf("%s", err)
					return err
				}
				if data.State.IsDoing() {
					err := fmt.Errorf("%w: %s: node %s state is %s", node.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, nodename, data.State)
					t.log.Errorf("%s", err)
					return err
				}
			}
		}

		if *c.Value.GlobalExpect != t.state.GlobalExpect {
			t.log.Infof("set global expect %s -> %s", t.state.GlobalExpect, *c.Value.GlobalExpect)
			t.change = true
			t.state.GlobalExpect = *c.Value.GlobalExpect
			t.state.GlobalExpectUpdatedAt = time.Now()
		}
		return nil
	}

	err := errors.Join(doState(), doLocalExpect(), doGlobalExpect())

	if v, ok := c.Err.(errcontext.ErrCloseSender); ok {
		v.Send(err)
		v.Close()
	}

	if t.change {
		t.updateIfChange()
		t.orchestrate()
	}
}

func (t *Manager) onArbitratorTicker() {
	t.getAndUpdateStatusArbitrator()
}

func (t *Manager) onForgetPeer(c *msgbus.ForgetPeer) {
	delete(t.livePeers, c.Node)

	delete(t.cacheNodesInfo, c.Node)
	t.saveNodesInfo()

	var forgetType string
	if !slices.Contains(clusternode.Get(), c.Node) {
		forgetType = "removed"
		t.log.Infof("forget %s peer %s => new live peers: %v", forgetType, c.Node, t.livePeers)
	} else {
		forgetType = "lost"
		t.log.Warnf("forget %s peer %s => new live peers: %v", forgetType, c.Node, t.livePeers)
	}

	if t.updateSpeaker() {
		t.publishNodeStatus()
	}

	if len(t.livePeers) > len(t.clusterConfig.Nodes)/2 {
		t.log.Infof("forget %s peer %s, we still have nodes quorum %d > %d", forgetType, c.Node, len(t.livePeers), len(t.clusterConfig.Nodes)/2)
		return
	}
	if !t.clusterConfig.Quorum {
		t.log.Warnf("cluster is split, ignore as cluster.quorum is false")
		return
	}
	if t.frozen {
		t.log.Warnf("cluster is split, ignore as the node is frozen")
		return
	}
	t.log.Warnf("cluster is split, check for arbitrator votes")
	total := len(t.clusterConfig.Nodes) + len(t.arbitrators)
	arbitratorVotes := t.arbitratorVotes()
	votes := len(t.livePeers) + len(arbitratorVotes)
	livePeers := make([]string, 0)
	for k := range t.livePeers {
		livePeers = append(livePeers, k)
	}
	if votes > total/2 {
		t.log.Warnf("cluster is split, we have quorum: %d+%d out of %d votes (%s + %s)", len(t.livePeers), len(arbitratorVotes), total, livePeers, arbitratorVotes)
		return
	}
	action := t.nodeConfig.SplitAction
	t.log.Warnf("cluster is split, we don't have quorum: %d+%d out of %d votes (%s + %s)", len(t.livePeers), len(arbitratorVotes), total, livePeers, arbitratorVotes)
	t.bus.Pub(&msgbus.NodeSplitAction{
		Node:            t.localhost,
		Action:          action,
		NodeVotes:       len(t.livePeers),
		ArbitratorVotes: len(arbitratorVotes),
		Voting:          total,
		ProVoters:       len(t.livePeers) + len(arbitratorVotes),
	}, t.labelLocalhost)

	splitAction, ok := splitActions[action]
	if !ok {
		t.log.Errorf("invalid split action %s", action)
		return
	}
	t.log.Warnf("cluster is split, will call split action %s in %s", action, splitActionDelay)
	time.Sleep(splitActionDelay)
	t.log.Warnf("cluster is split, now calling split action %s", action)
	if err := splitAction(); err != nil {
		t.log.Errorf("split action %s failed: %s", action, err)
	}
}

func (t *Manager) onNodeFrozenFileRemoved(_ *msgbus.NodeFrozenFileRemoved) {
	t.frozen = false
	t.nodeStatus.FrozenAt = time.Time{}
	t.bus.Pub(&msgbus.NodeFrozen{Node: t.localhost, Status: t.frozen, FrozenAt: time.Time{}}, t.labelLocalhost)
	t.publishNodeStatus()
}

func (t *Manager) onNodeFrozenFileUpdated(m *msgbus.NodeFrozenFileUpdated) {
	t.frozen = true
	t.nodeStatus.FrozenAt = m.At
	t.bus.Pub(&msgbus.NodeFrozen{Node: t.localhost, Status: t.frozen, FrozenAt: m.At}, t.labelLocalhost)
	t.publishNodeStatus()
}

func (t *Manager) onNodeMonitorDeleted(c *msgbus.NodeMonitorDeleted) {
	t.log.Debugf("deleted nmon for node %s", c.Node)
	delete(t.nodeMonitor, c.Node)
	t.convergeGlobalExpectFromRemote()
	t.updateIfChange()
	t.orchestrate()
	t.updateIfChange()
}

func (t *Manager) onPeerNodeMonitorUpdated(c *msgbus.NodeMonitorUpdated) {
	t.log.Debugf("updated nmon from node %s  -> %s", c.Node, c.Value.GlobalExpect)
	t.nodeMonitor[c.Node] = c.Value
	if _, ok := t.livePeers[c.Node]; !ok {
		t.livePeers[c.Node] = true
		t.log.Infof("new peer %s => new live peers: %v", c.Node, t.livePeers)
		if t.updateSpeaker() {
			t.publishNodeStatus()
		}
	}
	t.convergeGlobalExpectFromRemote()
	t.updateIfChange()
	t.orchestrate()
	t.updateIfChange()
}

func missingNodes(nodes, joinedNodes []string) []string {
	m := make(map[string]any)
	for _, nodename := range joinedNodes {
		m[nodename] = nil
	}
	l := make([]string, 0)
	for _, nodename := range nodes {
		if _, ok := m[nodename]; !ok {
			l = append(l, nodename)
		}
	}
	return l
}

func (t *Manager) onHbMessageTypeUpdated(c *msgbus.HbMessageTypeUpdated) {
	if t.state.State != node.MonitorStateRejoin {
		return
	}
	if c.To != "patch" {
		return
	}
	if l := missingNodes(c.Nodes, c.JoinedNodes); len(l) > 0 {
		t.log.Infof("preserve rejoin state, missing nodes %s", l)
		return
	}
	t.rejoinTicker.Stop()
	t.bus.Pub(&msgbus.NodeRejoin{
		Nodes:          c.Nodes,
		LastShutdownAt: file.ModTime(rawconfig.Paths.LastShutdown),
		IsUpgrading:    os.Getenv("OPENSVC_AGENT_UPGRADE") != "",
	}, t.labelLocalhost)
	_ = os.Unsetenv("OPENSVC_AGENT_UPGRADE")
	t.transitionTo(node.MonitorStateIdle)
}

func (t *Manager) onNodeRejoin(c *msgbus.NodeRejoin) {
	if c.IsUpgrading {
		return
	}
	if len(c.Nodes) < 2 {
		// no need to merge frozen on a single node cluster
		return
	}
	if !t.nodeStatus.FrozenAt.IsZero() {
		// already frozen
		return
	}
	if t.state.GlobalExpect == node.MonitorGlobalExpectThawed {
		return
	}
	for _, peer := range t.clusterConfig.Nodes {
		if peer == t.localhost {
			continue
		}
		peerStatus := node.StatusData.Get(peer)
		if peerStatus == nil {
			continue
		}
		if peerStatus.FrozenAt.After(c.LastShutdownAt) {
			if err := t.crmFreeze(); err != nil {
				t.log.Infof("node freeze error: %s", err)
			} else {
				t.log.Infof("node freeze because peer %s was frozen while this daemon was down", peer)
			}
			return
		}
	}
}

func (t *Manager) onOrchestrate(c cmdOrchestrate) {
	if t.state.State == c.state {
		t.transitionTo(c.newState)
	}
	t.orchestrate()
	// avoid fast loop on bug
	time.Sleep(50 * time.Millisecond)
}

func (t *Manager) orchestrateAfterAction(state, nextState node.MonitorState) {
	t.cmdC <- cmdOrchestrate{state: state, newState: nextState}
}

func (t *Manager) transitionTo(newState node.MonitorState) {
	t.change = true
	t.state.State = newState
	t.updateIfChange()
}
