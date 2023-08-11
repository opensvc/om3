package nmon

import (
	"fmt"
	"os"
	"time"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/stringslice"
	"github.com/opensvc/om3/util/toc"
)

var (
	slitActions = map[string]func() error{
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
func (o *nmon) onClusterConfigUpdated(c *msgbus.ClusterConfigUpdated) {
	o.clusterConfig = c.Value

	if err := o.loadAndPublishConfig(); err != nil {
		o.log.Error().Err(err).Msgf("load and publish config from cluster config updated event")
	}
	o.setArbitratorConfig()

	o.getAndUpdateStatusArbitrator()
}

// onConfigFileUpdated reloads the config parser and emits the updated
// node.Config data in a NodeConfigUpdated event, so other go routine
// can just subscribe to this event to maintain the cache of keywords
// they care about.
func (o *nmon) onConfigFileUpdated(_ *msgbus.ConfigFileUpdated) {
	if err := o.loadAndPublishConfig(); err != nil {
		o.log.Error().Err(err).Msg("load and publish config from node config file updated event")
		return
	}

	// env might have changed. nmon is responsible for updating nodes_info.json
	o.saveNodesInfo()

	// recompute rejoin ticker, perhaps RejoinGracePeriod has been changed
	o.checkRejoinTicker()
}

func (o *nmon) getNodeConfig() node.Config {
	var (
		keyMaintenanceGracePeriod = key.New("node", "maintenance_grace_period")
		keyReadyPeriod            = key.New("node", "ready_period")
		keyRejoinGracePeriod      = key.New("node", "rejoin_grace_period")
		keyEnv                    = key.New("node", "env")
		keySplitAction            = key.New("node", "split_action")
	)
	cfg := node.Config{}
	if d := o.config.GetDuration(keyMaintenanceGracePeriod); d != nil {
		cfg.MaintenanceGracePeriod = *d
	}
	if d := o.config.GetDuration(keyReadyPeriod); d != nil {
		cfg.ReadyPeriod = *d
	}
	if d := o.config.GetDuration(keyRejoinGracePeriod); d != nil {
		cfg.RejoinGracePeriod = *d
	}
	cfg.Env = o.config.GetString(keyEnv)
	cfg.SplitAction = o.config.GetString(keySplitAction)
	return cfg
}

func (o *nmon) checkRejoinTicker() {
	if o.state.State != node.MonitorStateRejoin {
		return
	}
	if left := o.startedAt.Add(o.nodeConfig.RejoinGracePeriod).Sub(time.Now()); left <= 0 {
		return
	} else {
		o.rejoinTicker.Reset(left)
		o.log.Info().Msgf("rejoin grace period timer reset to %s", left)
	}
}

func (o *nmon) onSetNodeMonitor(c *msgbus.SetNodeMonitor) {
	sendError := func(err error) {
		if c.Err != nil {
			c.Err <- err
		}
	}
	doState := func() {
		if c.Value.State == nil {
			return
		}
		// sanity check the state value
		if _, ok := node.MonitorStateStrings[*c.Value.State]; !ok {
			err := fmt.Errorf("%w: %s", node.ErrInvalidState, *c.Value.State)
			sendError(err)
			o.log.Warn().Msgf("%s", err)
			return
		}

		if o.state.State == *c.Value.State {
			err := fmt.Errorf("%w: state is already %s", node.ErrSameState, *c.Value.State)
			sendError(err)
			o.log.Info().Msgf("%s", err)
			return
		}

		o.log.Info().Msgf("set state %s -> %s", o.state.State, *c.Value.State)
		o.change = true
		o.state.State = *c.Value.State
	}

	doLocalExpect := func() {
		if c.Value.LocalExpect == nil {
			return
		}
		// sanity check the local expect value
		if _, ok := node.MonitorLocalExpectStrings[*c.Value.LocalExpect]; !ok {
			err := fmt.Errorf("%w: %s", node.ErrInvalidLocalExpect, *c.Value.LocalExpect)
			sendError(err)
			o.log.Warn().Msgf("%s", err)
			return
		}

		if o.state.LocalExpect == *c.Value.LocalExpect {
			err := fmt.Errorf("%w: %s", node.ErrSameLocalExpect, *c.Value.LocalExpect)
			sendError(err)
			o.log.Info().Msgf("%s", err)
			return
		}

		o.log.Info().Msgf("set local expect %s -> %s", o.state.LocalExpect, *c.Value.LocalExpect)
		o.change = true
		o.state.LocalExpect = *c.Value.LocalExpect
	}

	doGlobalExpect := func() {
		if c.Value.GlobalExpect == nil {
			return
		}
		if _, ok := node.MonitorGlobalExpectStrings[*c.Value.GlobalExpect]; !ok {
			o.log.Warn().Msgf("invalid set node monitor local expect: %s", *c.Value.GlobalExpect)
			return
		}
		if *c.Value.GlobalExpect != node.MonitorGlobalExpectAborted {
			for nodename, data := range o.nodeMonitor {
				if data.GlobalExpect == *c.Value.GlobalExpect {
					err := fmt.Errorf("%w: %s: more recent value %s on node %s", node.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, data.GlobalExpect, nodename)
					sendError(err)
					o.log.Info().Msgf("%s", err)
					return
				}
				if !data.State.IsRankable() {
					err := fmt.Errorf("%w: %s: node %s state is %s", node.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, nodename, data.State)
					sendError(err)
					o.log.Error().Msgf("%s", err)
					return
				}
				if data.State.IsDoing() {
					err := fmt.Errorf("%w: %s: node %s state is %s", node.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, nodename, data.State)
					sendError(err)
					o.log.Error().Msgf("%s", err)
					return
				}
			}
		}

		if *c.Value.GlobalExpect != o.state.GlobalExpect {
			o.log.Info().Msgf("set global expect %s -> %s", o.state.GlobalExpect, *c.Value.GlobalExpect)
			o.change = true
			o.state.GlobalExpect = *c.Value.GlobalExpect
			o.state.GlobalExpectUpdatedAt = time.Now()
		}
	}

	doState()
	doLocalExpect()
	doGlobalExpect()

	// inform the publisher we're done sending errors
	sendError(nil)

	if o.change {
		o.updateIfChange()
		o.orchestrate()
	}
}

func (o *nmon) onArbitratorTicker() {
	o.getAndUpdateStatusArbitrator()
}

func (o *nmon) onForgetPeer(c *msgbus.ForgetPeer) {
	delete(o.livePeers, c.Node)

	delete(o.cacheNodesInfo, c.Node)
	o.saveNodesInfo()

	if !stringslice.Has(c.Node, clusternode.Get()) {
		o.log.Info().Msgf("forget removed peer %s => new live peers: %v", c.Node, o.livePeers)
	} else {
		o.log.Warn().Msgf("forget lost peer %s => new live peers: %v", c.Node, o.livePeers)
	}

	if len(o.livePeers) > len(o.clusterConfig.Nodes)/2 {
		o.log.Warn().Msgf("peer %s not anymore alive, we still have nodes quorum %d > %d", c.Node, len(o.livePeers), len(o.clusterConfig.Nodes)/2)
		return
	}
	if !o.clusterConfig.Quorum {
		o.log.Warn().Msgf("cluster is split, ignore as cluster.quorum is false")
		return
	}
	if o.frozen {
		o.log.Warn().Msgf("cluster is split, ignore as the node is frozen")
		return
	}
	o.log.Warn().Msgf("cluster is split, check for arbitrator votes")
	total := len(o.clusterConfig.Nodes) + len(o.arbitrators)
	arbitratorVotes := o.arbitratorVotes()
	votes := len(o.livePeers) + len(arbitratorVotes)
	livePeers := make([]string, 0)
	for k := range o.livePeers {
		livePeers = append(livePeers, k)
	}
	if votes > total/2 {
		o.log.Warn().Msgf("cluster is split, we have quorum: %d+%d out of %d votes (%s + %s)", len(o.livePeers), len(arbitratorVotes), total, livePeers, arbitratorVotes)
		return
	}
	action := o.nodeConfig.SplitAction
	o.log.Warn().Msgf("cluster is split, we don't have quorum: %d+%d out of %d votes (%s + %s)", len(o.livePeers), len(arbitratorVotes), total, livePeers, arbitratorVotes)
	o.bus.Pub(&msgbus.NodeSplitAction{
		Node:            o.localhost,
		Action:          action,
		NodeVotes:       len(o.livePeers),
		ArbitratorVotes: len(arbitratorVotes),
		Voting:          total,
		ProVoters:       len(o.livePeers) + len(arbitratorVotes),
	}, o.labelLocalhost)

	splitAction, ok := slitActions[action]
	if !ok {
		o.log.Error().Msgf("invalid split action %s", action)
		return
	}
	o.log.Warn().Msgf("cluster is split, will call split action %s in %s", action, splitActionDelay)
	time.Sleep(splitActionDelay)
	o.log.Warn().Msgf("cluster is split, now calling split action %s", action)
	if err := splitAction(); err != nil {
		o.log.Error().Err(err).Msgf("split action %s failed", action)
	}
}

func (o *nmon) onNodeFrozenFileRemoved(_ *msgbus.NodeFrozenFileRemoved) {
	o.frozen = false
	o.nodeStatus.FrozenAt = time.Time{}
	o.bus.Pub(&msgbus.NodeFrozen{Node: o.localhost, Status: o.frozen, FrozenAt: time.Time{}}, o.labelLocalhost)
	node.StatusData.Set(o.localhost, o.nodeStatus.DeepCopy())
	o.bus.Pub(&msgbus.NodeStatusUpdated{Node: o.localhost, Value: *o.nodeStatus.DeepCopy()}, o.labelLocalhost)
}

func (o *nmon) onNodeFrozenFileUpdated(m *msgbus.NodeFrozenFileUpdated) {
	o.frozen = true
	o.nodeStatus.FrozenAt = m.At
	o.bus.Pub(&msgbus.NodeFrozen{Node: o.localhost, Status: o.frozen, FrozenAt: m.At}, o.labelLocalhost)
	node.StatusData.Set(o.localhost, o.nodeStatus.DeepCopy())
	o.bus.Pub(&msgbus.NodeStatusUpdated{Node: o.localhost, Value: *o.nodeStatus.DeepCopy()}, o.labelLocalhost)
}

func (o *nmon) onNodeMonitorDeleted(c *msgbus.NodeMonitorDeleted) {
	o.log.Debug().Msgf("deleted nmon for node %s", c.Node)
	delete(o.nodeMonitor, c.Node)
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *nmon) onPeerNodeMonitorUpdated(c *msgbus.NodeMonitorUpdated) {
	o.log.Debug().Msgf("updated nmon from node %s  -> %s", c.Node, c.Value.GlobalExpect)
	o.nodeMonitor[c.Node] = c.Value
	if _, ok := o.livePeers[c.Node]; !ok {
		o.livePeers[c.Node] = true
		o.log.Info().Msgf("new peer %s => new live peers: %v", c.Node, o.livePeers)
	}
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
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

func (o *nmon) onHbMessageTypeUpdated(c *msgbus.HbMessageTypeUpdated) {
	if o.state.State != node.MonitorStateRejoin {
		return
	}
	if c.To != "patch" {
		return
	}
	if l := missingNodes(c.Nodes, c.JoinedNodes); len(l) > 0 {
		o.log.Info().Msgf("preserve rejoin state, missing nodes %s", l)
		return
	}
	o.rejoinTicker.Stop()
	o.bus.Pub(&msgbus.NodeRejoin{
		Nodes:          c.Nodes,
		LastShutdownAt: file.ModTime(rawconfig.Paths.LastShutdown),
		IsUpgrading:    os.Getenv("OPENSVC_AGENT_UPGRADE") != "",
	}, o.labelLocalhost)
	_ = os.Unsetenv("OPENSVC_AGENT_UPGRADE")
	o.transitionTo(node.MonitorStateIdle)
}

func (o *nmon) onNodeRejoin(c *msgbus.NodeRejoin) {
	if c.IsUpgrading {
		return
	}
	if len(c.Nodes) < 2 {
		// no need to merge frozen on a single node cluster
		return
	}
	if !o.nodeStatus.FrozenAt.IsZero() {
		// already frozen
		return
	}
	if o.state.GlobalExpect == node.MonitorGlobalExpectThawed {
		return
	}
	for _, peer := range o.clusterConfig.Nodes {
		if peer == o.localhost {
			continue
		}
		peerStatus := node.StatusData.Get(peer)
		if peerStatus == nil {
			continue
		}
		if peerStatus.FrozenAt.After(c.LastShutdownAt) {
			if err := o.crmFreeze(); err != nil {
				o.log.Info().Err(err).Send()
			} else {
				o.log.Info().Msgf("node freeze because peer %s was frozen while this daemon was down", peer)
			}
			return
		}
	}
}

func (o *nmon) onOrchestrate(c cmdOrchestrate) {
	if o.state.State == c.state {
		o.transitionTo(c.newState)
	}
	o.orchestrate()
	// avoid fast loop on bug
	time.Sleep(50 * time.Millisecond)
}

func (o *nmon) orchestrateAfterAction(state, nextState node.MonitorState) {
	o.cmdC <- cmdOrchestrate{state: state, newState: nextState}
}

func (o *nmon) transitionTo(newState node.MonitorState) {
	o.change = true
	o.state.State = newState
	o.updateIfChange()
}
