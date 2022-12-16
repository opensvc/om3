package nmon

import (
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/file"
)

func (o *nmon) onSetNmonCmd(c msgbus.SetNodeMonitor) {
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

		for node, data := range o.nmons {
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
	daemondata.SetNodeFrozen(o.dataCmdC, time.Time{})
}

func (o *nmon) onFrozenFileUpdated(c msgbus.FrozenFileUpdated) {
	tm := file.ModTime(c.Filename)
	daemondata.SetNodeFrozen(o.dataCmdC, tm)
}

func (o *nmon) onNmonDeleted(c msgbus.NodeMonitorDeleted) {
	o.log.Debug().Msgf("deleted nmon for node %s", c.Node)
	delete(o.nmons, c.Node)
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *nmon) onNmonUpdated(c msgbus.NodeMonitorUpdated) {
	o.log.Debug().Msgf("updated nmon from node %s  -> %s", c.Node, c.Monitor.GlobalExpect)
	o.nmons[c.Node] = c.Monitor
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *nmon) onOrchestrate(c cmdOrchestrate) {
	if o.state.State == c.state {
		o.change = true
		o.state.State = c.newState
		o.updateIfChange()
	}
	o.orchestrate()
	// avoid fast loop on bug
	time.Sleep(50 * time.Millisecond)
}

func (o *nmon) orchestrateAfterAction(state, nextState cluster.NodeMonitorState) {
	o.cmdC <- cmdOrchestrate{state: state, newState: nextState}
}
