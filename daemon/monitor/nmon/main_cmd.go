package nmon

import (
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/key"
)

func (o *nmon) onCfgFileUpdated(c msgbus.CfgFileUpdated) {
	if !c.Path.IsZero() {
		return
	}
	if o.state.State != cluster.NodeMonitorStateRejoin {
		return
	}
	if err := o.config.Reload(); err != nil {
		o.log.Error().Err(err).Msg("readjust rejoin timer")
		return
	}
	rejoinGracePeriod := o.config.GetDuration(key.New("node", "rejoin_grace_period"))
	left := o.startedAt.Add(*rejoinGracePeriod).Sub(time.Now())
	o.rejoinTicker.Reset(left)
	o.log.Info().Msgf("rejoin grace period timer reset to %s", left)
}

func (o *nmon) onSetNodeMonitor(c msgbus.SetNodeMonitor) {
	doStatus := func() {
		// TODO?
	}

	doLocalExpect := func() {
		// sanity check the local expect value
		if _, ok := cluster.NodeMonitorLocalExpectStrings[c.Monitor.LocalExpect]; !ok {
			o.log.Warn().Msgf("invalid set node monitor local expect: %s", c.Monitor.LocalExpect)
			return
		}

		if o.state.LocalExpect == c.Monitor.LocalExpect {
			o.log.Info().Msgf("local expect is already %s", c.Monitor.LocalExpect)
			return
		}

		o.log.Info().Msgf("set local expect %s -> %s", o.state.LocalExpect, c.Monitor.LocalExpect)
		o.change = true
		o.state.LocalExpect = c.Monitor.LocalExpect
	}

	doGlobalExpect := func() {
		if _, ok := cluster.NodeMonitorGlobalExpectStrings[c.Monitor.GlobalExpect]; !ok {
			o.log.Warn().Msgf("invalid set node monitor local expect: %s", c.Monitor.GlobalExpect)
			return
		}
		if c.Monitor.GlobalExpect != cluster.NodeMonitorGlobalExpectAborted {
			for node, data := range o.nodeMonitor {
				if data.GlobalExpect == c.Monitor.GlobalExpect {
					o.log.Info().Msgf("set nmon: already targeting %s (on node %s)", c.Monitor.GlobalExpect, node)
					return
				}
				if !data.State.IsRankable() {
					o.log.Error().Msgf("set nmon: can't set global expect to %s (node %s is %s)", c.Monitor.GlobalExpect, node, data.State)
					return
				}
				if data.State.IsDoing() {
					o.log.Error().Msgf("set nmon: can't set global expect to %s (node %s is %s)", c.Monitor.GlobalExpect, node, data.State)
					return
				}
			}
		}

		o.log.Info().Msgf("# set nmon: client request global expect to %s %+v", c.Monitor.GlobalExpect, c.Monitor)
		if c.Monitor.GlobalExpect != o.state.GlobalExpect {
			o.change = true
			o.state.GlobalExpect = c.Monitor.GlobalExpect
			o.state.GlobalExpectUpdated = time.Now()
		}
	}

	doStatus()
	doLocalExpect()
	doGlobalExpect()

	if o.change {
		o.updateIfChange()
		o.orchestrate()
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

func (o *nmon) onNodeMonitorUpdated(c msgbus.NodeMonitorUpdated) {
	o.log.Debug().Msgf("updated nmon from node %s  -> %s", c.Node, c.Monitor.GlobalExpect)
	o.nodeMonitor[c.Node] = c.Monitor
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
	if o.state.State != cluster.NodeMonitorStateRejoin {
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
	o.transitionTo(cluster.NodeMonitorStateIdle)
}

func (o *nmon) onOrchestrate(c cmdOrchestrate) {
	if o.state.State == c.state {
		o.transitionTo(c.newState)
	}
	o.orchestrate()
	// avoid fast loop on bug
	time.Sleep(50 * time.Millisecond)
}

func (o *nmon) orchestrateAfterAction(state, nextState cluster.NodeMonitorState) {
	o.cmdC <- cmdOrchestrate{state: state, newState: nextState}
}

func (o *nmon) transitionTo(newState cluster.NodeMonitorState) {
	o.change = true
	o.state.State = newState
	o.updateIfChange()
}
