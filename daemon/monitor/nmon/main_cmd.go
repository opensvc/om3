package nmon

import (
	"strings"
	"time"

	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/file"
)

func (o *nmon) onSetNmonCmd(c msgbus.SetNodeMonitor) {
	strVal := c.Monitor.GlobalExpect
	if strVal == statusIdle {
		strVal = "unset"
	}
	doStatus := func() {
		// TODO?
	}

	doLocalExpect := func() {
		// sanity check the local expect value
		switch c.Monitor.LocalExpect {
		case localExpectUnset:
			return
		case localExpectDrained:
		default:
			o.log.Warn().Msgf("invalid set smon local expect: %s", c.Monitor.LocalExpect)
			return
		}

		// set the valid local expect value
		var target string
		if c.Monitor.LocalExpect == "unset" {
			target = localExpectUnset
		} else {
			target = c.Monitor.LocalExpect
		}
		if o.state.LocalExpect == target {
			o.log.Info().Msgf("local expect is already %s", c.Monitor.LocalExpect)
			return
		}
		o.log.Info().Msgf("set local expect %s -> %s", o.state.LocalExpect, target)
		o.change = true
		o.state.LocalExpect = target
	}

	doGlobalExpect := func() {
		switch c.Monitor.GlobalExpect {
		case globalExpectUnset:
			return
		case globalExpectAborted:
		case globalExpectFrozen:
		case globalExpectThawed:
		default:
			o.log.Warn().Msgf("invalid set node monitor global expect: %s", c.Monitor.GlobalExpect)
			return
		}

		for node, data := range o.nmons {
			if data.GlobalExpect == c.Monitor.GlobalExpect {
				msg := "set nmon: already targeting " + strVal + " (on node " + node + ")"
				o.log.Info().Msg(msg)
				return
			}
			if _, ok := statusUnrankable[data.Status]; ok {
				msg := "set nmon: can't set global expect to " + strVal + " (node " + node + " is " + data.Status + ")"
				o.log.Error().Msg(msg)
				return
			}
			if strings.HasSuffix(data.Status, "ing") {
				msg := "set nmon: can't set global expect to " + strVal + " (node " + node + " is " + data.Status + ")"
				o.log.Error().Msg(msg)
				return
			}
		}

		o.log.Info().Msgf("set nmon: client request global expect to %s %+v", strVal, c.Monitor)
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
	if o.state.Status == c.state {
		o.change = true
		o.state.Status = c.newState
		o.updateIfChange()
	}
	o.orchestrate()
	// avoid fast loop on bug
	time.Sleep(50 * time.Millisecond)
}
