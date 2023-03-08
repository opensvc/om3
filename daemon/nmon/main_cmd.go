package nmon

import (
	"time"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/key"
)

func (o *nmon) onClusterConfigUpdated(c msgbus.ClusterConfigUpdated) {
	o.clusterConfig = c.Value
	o.setArbitratorConfig()
	if err := o.getAndUpdateStatusArbitrator(); err != nil {
		o.log.Error().Err(err).Msg("arbitrator status failure (after cluster config updated)")
	}
}

// onConfigFileUpdated reloads the config parser and emits the updated
// node.Config data in a NodeConfigUpdated event, so other go routine
// can just subscribe to this event to maintain the cache of keywords
// they care about.
func (o *nmon) onConfigFileUpdated(c msgbus.ConfigFileUpdated) {
	if err := o.config.Reload(); err != nil {
		o.log.Error().Err(err).Msg("reload merged config")
		return
	}
	o.nodeConfig = o.getNodeConfig()
	err := o.pubNodeConfig()
	if err != nil {
		o.log.Error().Err(err).Msg("publish node config")
	}

	// env might have changed. nmon is responsible for updating nodes_info.json
	o.saveNodesInfo()

	// recompute rejoin ticker, perhaps RejoinGracePeriod has been changed
	o.checkRejoinTicker()
}

func (o *nmon) pubNodeConfig() error {
	return o.databus.SetNodeConfig(o.nodeConfig)
}

func (o *nmon) getNodeConfig() node.Config {
	var (
		keyMaintenanceGracePeriod = key.New("node", "maintenance_grace_period")
		keyReadyPeriod            = key.New("node", "ready_period")
		keyRejoinGracePeriod      = key.New("node", "rejoin_grace_period")
		keyEnv                    = key.New("node", "env")
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

func (o *nmon) onSetNodeMonitor(c msgbus.SetNodeMonitor) {
	doState := func() {
		if c.Value.State == nil {
			return
		}
		// sanity check the state value
		if _, ok := node.MonitorStateStrings[*c.Value.State]; !ok {
			o.log.Warn().Msgf("invalid set node monitor state: %s", c.Value.State)
			return
		}

		if o.state.State == *c.Value.State {
			o.log.Info().Msgf("state is already %s", *c.Value.State)
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
			o.log.Warn().Msgf("invalid set node monitor local expect: %s", c.Value.LocalExpect)
			return
		}

		if o.state.LocalExpect == *c.Value.LocalExpect {
			o.log.Info().Msgf("local expect is already %s", *c.Value.LocalExpect)
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
			for node, data := range o.nodeMonitor {
				if data.GlobalExpect == *c.Value.GlobalExpect {
					o.log.Info().Msgf("set nmon: already targeting %s (on node %s)", *c.Value.GlobalExpect, node)
					return
				}
				if !data.State.IsRankable() {
					o.log.Error().Msgf("set nmon: can't set global expect to %s (node %s is %s)", *c.Value.GlobalExpect, node, data.State)
					return
				}
				if data.State.IsDoing() {
					o.log.Error().Msgf("set nmon: can't set global expect to %s (node %s is %s)", *c.Value.GlobalExpect, node, data.State)
					return
				}
			}
		}

		o.log.Info().Msgf("# set nmon: client request global expect to %s %+v", *c.Value.GlobalExpect, c.Value)
		if *c.Value.GlobalExpect != o.state.GlobalExpect {
			o.change = true
			o.state.GlobalExpect = *c.Value.GlobalExpect
			o.state.GlobalExpectUpdated = time.Now()
		}
	}

	doState()
	doLocalExpect()
	doGlobalExpect()

	if o.change {
		o.updateIfChange()
		o.orchestrate()
	}
}

func (o *nmon) onArbitratorTicker() {
	if err := o.getAndUpdateStatusArbitrator(); err != nil {
		o.log.Warn().Err(err).Msg("arbitrator status failure (arbitrator ticker)")
	}
}

func (o *nmon) onFrozenFileRemoved(c msgbus.FrozenFileRemoved) {
	o.databus.SetNodeFrozen(time.Time{})
}

func (o *nmon) onFrozenFileUpdated(c msgbus.FrozenFileUpdated) {
	tm := file.ModTime(c.Filename)
	o.databus.SetNodeFrozen(tm)
}

func (o *nmon) onNodeMonitorDeleted(c msgbus.NodeMonitorDeleted) {
	o.log.Debug().Msgf("deleted nmon for node %s", c.Node)
	delete(o.nodeMonitor, c.Node)
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *nmon) onPeerNodeMonitorUpdated(c msgbus.NodeMonitorUpdated) {
	o.log.Debug().Msgf("updated nmon from node %s  -> %s", c.Node, c.Value.GlobalExpect)
	o.nodeMonitor[c.Node] = c.Value
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func missingNodes(nodes, joinedNodes []string) []string {
	m := make(map[string]any)
	for _, node := range joinedNodes {
		m[node] = nil
	}
	l := make([]string, 0)
	for _, node := range nodes {
		if _, ok := m[node]; !ok {
			l = append(l, node)
		}
	}
	return l
}

func (o *nmon) onHbMessageTypeUpdated(c msgbus.HbMessageTypeUpdated) {
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
	o.transitionTo(node.MonitorStateIdle)
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
