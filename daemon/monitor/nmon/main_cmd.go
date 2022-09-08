package nmon

import (
	"strings"

	"opensvc.com/opensvc/daemon/monitor/moncmd"
)

func (o *nmon) onSetNmonCmd(c moncmd.SetNmon) {
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

func (o *nmon) onNmonUpdated(c moncmd.NmonUpdated) {
	node := c.Node
	if node == o.localhost {
		return
	}
	data := c.Monitor
	o.log.Debug().Msgf("updated instance nmon from node %s  -> %s", node, data.GlobalExpect)
	o.nmons[node] = data
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *nmon) needOrchestrate(c cmdOrchestrate) {
	if o.state.Status == c.state {
		o.change = true
		o.state.Status = c.newState
		o.updateIfChange()
	}
	o.orchestrate()
}
