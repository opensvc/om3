package smon

import (
	"time"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/msgbus"
)

// cmdSvcAggUpdated updateIfChange state global expect from aggregated status
func (o *smon) cmdSvcAggUpdated(c msgbus.MonSvcAggUpdated) {
	if c.SrcEv != nil {
		switch srcCmd := (*c.SrcEv).(type) {
		case msgbus.InstStatusUpdated:
			srcNode := srcCmd.Node
			if _, ok := o.instStatus[srcNode]; ok {
				instStatus := srcCmd.Status
				if o.instStatus[srcNode].Updated.Before(instStatus.Updated) {
					// only update if more recent
					o.instStatus[srcNode] = instStatus
				}
			}
		case msgbus.CfgUpdated:
			if srcCmd.Node == o.localhost {
				cfgNodes := make(map[string]struct{})
				for _, node := range srcCmd.Config.Scope {
					cfgNodes[node] = struct{}{}
					if _, ok := o.instStatus[node]; !ok {
						o.instStatus[node] = instance.Status{Avail: status.Undef}
					}
				}
				for node := range o.instStatus {
					if _, ok := cfgNodes[node]; !ok {
						o.log.Info().Msgf("drop not anymore in local config status from node %s", node)
						delete(o.instStatus, node)
					}
				}
				o.scopeNodes = append([]string{}, srcCmd.Config.Scope...)
			}
		}
	}
	o.svcAgg = c.SvcAgg
	o.orchestrate()
}

func (o *smon) cmdSetSmonClient(c instance.Monitor) {
	strVal := c.GlobalExpect
	if strVal == statusIdle {
		strVal = "unset"
	}
	for node, status := range o.instSmon {
		if status.GlobalExpect == c.GlobalExpect {
			msg := "set smon: already targeting " + strVal + " (on node " + node + ")"
			o.log.Info().Msg(msg)
			return
		}
	}
	switch c.GlobalExpect {
	case globalExpectAbort:
		c.GlobalExpect = globalExpectUnset
	case globalExpectUnset:
		return
	case globalExpectStarted:
		if o.svcAgg.Avail == status.Up {
			msg := "set smon: already started"
			o.log.Info().Msg(msg)
			return
		}
	}
	o.log.Info().Msgf("set smon: client request global expect to %s %+v", strVal, c)
	if c.GlobalExpect != o.state.GlobalExpect {
		o.change = true
		o.state.GlobalExpect = c.GlobalExpect
		o.state.GlobalExpectUpdated = time.Now()
		o.updateIfChange()
		o.orchestrate()
	}
}

func (o *smon) cmdSmonUpdated(c msgbus.SmonUpdated) {
	node := c.Node
	if node == o.localhost {
		return
	}
	instSmon := c.Status
	o.log.Debug().Msgf("updated instance smon from node %s  -> %s", node, instSmon.GlobalExpect)
	o.instSmon[node] = instSmon
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *smon) cmdSmonDeleted(c msgbus.SmonDeleted) {
	node := c.Node
	if node == o.localhost {
		return
	}
	o.log.Debug().Msgf("delete remote instance smon from node %s", node)
	delete(o.instSmon, c.Node)
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *smon) needOrchestrate(c cmdOrchestrate) {
	if o.state.Status == c.state {
		o.change = true
		o.state.Status = c.newState
		o.updateIfChange()
	}
	o.orchestrate()
}
