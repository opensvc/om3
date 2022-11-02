package smon

import (
	"strings"
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
		case msgbus.CfgDeleted:
			node := srcCmd.Node
			if _, ok := o.instStatus[node]; ok {
				o.log.Info().Msgf("drop deleted instance status from node %s", node)
				delete(o.instStatus, node)
			}
		}
	}
	o.svcAgg = c.SvcAgg
	o.orchestrate()
}

func (o *smon) cmdSetSmonClient(c instance.Monitor) {
	doStatus := func() {
		switch c.Status {
		case "":
			return
		case statusDeleted:
		case statusDeleting:
		case statusFreezeFailed:
		case statusFreezing:
		case statusFrozen:
		case statusIdle:
		case statusProvisioned:
		case statusProvisioning:
		case statusProvisionFailed:
		case statusPurgeFailed:
		case statusReady:
		case statusStarted:
		case statusStartFailed:
		case statusStarting:
		case statusStopFailed:
		case statusStopping:
		case statusThawed:
		case statusThawedFailed:
		case statusThawing:
		case statusUnProvisioned:
		case statusUnProvisionFailed:
		case statusUnProvisioning:
		case statusWaitLeader:
		case statusWaitNonLeader:
		default:
			o.log.Warn().Msgf("invalid set smon status: %s", c.Status)
			return
		}
		if o.state.Status == c.Status {
			o.log.Info().Msgf("status is already %s", c.Status)
			return
		}
		o.log.Info().Msgf("set status %s -> %s", o.state.Status, c.Status)
		o.change = true
		o.state.Status = c.Status
	}

	doGlobalExpect := func() {
		switch c.GlobalExpect {
		case "":
			return
		case globalExpectAborted:
		case globalExpectFrozen:
		case globalExpectProvisioned:
		case globalExpectPlaced:
		case globalExpectPlacedAt:
		case globalExpectPurged:
		case globalExpectStopped:
		case globalExpectThawed:
		case globalExpectUnProvisioned:
		case globalExpectStarted:
			if o.svcAgg.Avail == status.Up {
				o.log.Info().Msg("preserve global expect: object is already started")
				return
			}
		default:
			if !strings.HasPrefix(c.GlobalExpect, globalExpectPlacedAt) {
				o.log.Warn().Msgf("invalid set smon global expect: %s", c.GlobalExpect)
				return
			}
		}
		from, to := o.logFromTo(o.state.GlobalExpect, c.GlobalExpect)
		for node, instSmon := range o.instSmon {
			if instSmon.GlobalExpect != c.GlobalExpect && instSmon.GlobalExpect != "" && instSmon.GlobalExpectUpdated.After(o.state.GlobalExpectUpdated) {
				o.log.Info().Msgf("global expect is already %s on node %s", to, node)
				return
			}
		}

		o.log.Info().Msgf("prepare to set global expect %s -> %s", from, to)
		if c.GlobalExpect != o.state.GlobalExpect {
			o.change = true
			o.state.GlobalExpect = c.GlobalExpect
			// update GlobalExpectUpdated now
			// This will allow remote nodes to pickup most recent value
			o.state.GlobalExpectUpdated = time.Now()
		}
	}

	doLocalExpect := func() {
		switch c.LocalExpect {
		case localExpectUnset:
			return
		case localExpectStarted:
		default:
			o.log.Warn().Msgf("invalid set smon local expect: %s", c.LocalExpect)
			return
		}
		var target string
		if c.LocalExpect == "unset" {
			target = localExpectUnset
		} else {
			target = c.LocalExpect
		}
		if o.state.LocalExpect == target {
			o.log.Info().Msgf("local expect is already %s", c.LocalExpect)
			return
		}
		o.log.Info().Msgf("set local expect %s -> %s", o.state.LocalExpect, target)
		o.change = true
		o.state.LocalExpect = target
	}

	doStatus()
	doGlobalExpect()
	doLocalExpect()

	if o.change {
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
