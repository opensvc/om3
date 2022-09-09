package nmon

import (
	"strings"
	"time"

	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/util/file"
)

func (o *nmon) onSetNmonCmd(c daemonps.SetNmon) {
	strVal := c.Monitor.GlobalExpect
	if strVal == statusIdle {
		strVal = "unset"
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
	switch c.Monitor.GlobalExpect {
	case globalExpectAbort:
		c.Monitor.GlobalExpect = globalExpectUnset
	case globalExpectUnset:
		return
	}
	o.log.Info().Msgf("set nmon: client request global expect to %s %+v", strVal, c.Monitor)
	if c.Monitor.GlobalExpect != o.state.GlobalExpect {
		o.change = true
		o.state.GlobalExpect = c.Monitor.GlobalExpect
		o.state.GlobalExpectUpdated = c.Monitor.GlobalExpectUpdated
		o.updateIfChange()
		o.orchestrate()
	}
}

func (o *nmon) onFrozenFileRemoved(c daemonps.FrozenFileRemoved) {
	daemondata.SetNodeFrozen(o.dataCmdC, time.Time{})
}

func (o *nmon) onFrozenFileUpdated(c daemonps.FrozenFileUpdated) {
	tm := file.ModTime(c.Filename)
	daemondata.SetNodeFrozen(o.dataCmdC, tm)
}

func (o *nmon) onNmonDeleted(c daemonps.NmonDeleted) {
	o.log.Debug().Msgf("deleted nmon for node %s", c.Node)
	delete(o.nmons, c.Node)
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *nmon) onNmonUpdated(c daemonps.NmonUpdated) {
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
}
