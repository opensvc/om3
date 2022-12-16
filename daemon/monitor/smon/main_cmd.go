package smon

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/goombaio/orderedset"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/nodeselector"
	"opensvc.com/opensvc/core/placement"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/stringslice"
)

func (o *smon) onInstanceStatusUpdated(srcNode string, srcCmd msgbus.InstanceStatusUpdated) {
	if _, ok := o.instStatus[srcCmd.Node]; ok {
		if o.instStatus[srcCmd.Node].Updated.Before(srcCmd.Status.Updated) {
			// only update if more recent
			o.log.Debug().Msgf("ObjectAggUpdated %s from InstanceStatusUpdated on %s update instance status", srcNode, srcCmd.Node)
			o.instStatus[srcCmd.Node] = srcCmd.Status
		} else {
			o.log.Debug().Msgf("ObjectAggUpdated %s from InstanceStatusUpdated on %s skip update instance from obsolete status", srcNode, srcCmd.Node)
		}
	} else {
		o.log.Debug().Msgf("ObjectAggUpdated %s from InstanceStatusUpdated on %s create instance status", srcNode, srcCmd.Node)
		o.instStatus[srcCmd.Node] = srcCmd.Status
	}
}

func (o *smon) onCfgUpdated(srcNode string, srcCmd msgbus.CfgUpdated) {
	if srcCmd.Node == o.localhost {
		cfgNodes := make(map[string]any)
		for _, node := range srcCmd.Config.Scope {
			cfgNodes[node] = nil
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
	}
	o.scopeNodes = append([]string{}, srcCmd.Config.Scope...)
	o.log.Debug().Msgf("updated from %s ObjectAggUpdated CfgUpdated on %s scopeNodes=%s", srcNode, srcCmd.Node, o.scopeNodes)
}

func (o *smon) onCfgDeleted(srcNode string, srcCmd msgbus.CfgDeleted) {
	if _, ok := o.instStatus[srcCmd.Node]; ok {
		o.log.Info().Msgf("drop deleted instance status from node %s", srcCmd.Node)
		delete(o.instStatus, srcCmd.Node)
	}
}

// onObjectAggUpdated updateIfChange state global expect from aggregated status
func (o *smon) onObjectAggUpdated(c msgbus.ObjectAggUpdated) {
	if c.SrcEv != nil {
		switch srcCmd := c.SrcEv.(type) {
		case msgbus.InstanceStatusUpdated:
			o.onInstanceStatusUpdated(c.Node, srcCmd)
		case msgbus.CfgUpdated:
			o.onCfgUpdated(c.Node, srcCmd)
		case msgbus.CfgDeleted:
			o.onCfgDeleted(c.Node, srcCmd)
		}
	}
	o.svcAgg = c.AggregatedStatus
	o.updateIsLeader()
	o.orchestrate()
}

func (o *smon) onSetInstanceMonitorClient(c instance.Monitor) {
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
		case statusUnprovisioned:
		case statusUnprovisionFailed:
		case statusUnprovisioning:
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
			// Switch cmd without explicit target nodes.
			// Select some nodes automatically.
			dst := o.nextPlacedAtCandidate()
			if dst == "" {
				o.log.Info().Msg("no destination node could be selected from candidates")
				return
			}
			c.GlobalExpect += dst
		case globalExpectPurged:
		case globalExpectStopped:
		case globalExpectThawed:
		case globalExpectUnprovisioned:
		case globalExpectStarted:
			if v, reason := o.isStartable(); !v {
				o.log.Info().Msg(reason)
				return
			}
		default:
			if strings.HasPrefix(c.GlobalExpect, globalExpectPlacedAt) {
				want := strings.SplitN(c.GlobalExpect, "@", 2)[1]
				can := o.nextPlacedAtCandidates(want)
				if can == "" {
					o.log.Info().Msgf("no destination node could be selected from %s", want)
					return
				} else if can != want {
					o.log.Info().Msgf("change destination nodes from %s to %s", want, can)
				}
				c.GlobalExpect = globalExpectPlacedAt + can
			} else {
				o.log.Warn().Msgf("invalid set smon global expect: %s", c.GlobalExpect)
				return
			}
		}
		_, to := o.logFromTo(o.state.GlobalExpect, c.GlobalExpect)
		for node, instSmon := range o.instSmon {
			if instSmon.GlobalExpect != c.GlobalExpect && instSmon.GlobalExpect != "" && instSmon.GlobalExpectUpdated.After(o.state.GlobalExpectUpdated) {
				o.log.Info().Msgf("global expect is already %s on node %s", to, node)
				return
			}
		}

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

func (o *smon) onNodeMonitorUpdated(c msgbus.NodeMonitorUpdated) {
	o.nodeMonitor[c.Node] = c.Monitor
	o.updateIsLeader()
	o.orchestrate()
	o.updateIfChange()
}

func (o *smon) onNodeStatusUpdated(c msgbus.NodeStatusUpdated) {
	o.nodeStatus[c.Node] = c.Value
	o.updateIsLeader()
	o.orchestrate()
	o.updateIfChange()
}

func (o *smon) onNodeStatsUpdated(c msgbus.NodeStatsUpdated) {
	o.nodeStats[c.Node] = c.Value
	if o.svcAgg.PlacementPolicy == placement.Score {
		o.updateIsLeader()
		o.orchestrate()
		o.updateIfChange()
	}
}

func (o *smon) onRemoteSmonUpdated(c msgbus.InstanceMonitorUpdated) {
	remote := c.Node
	instSmon := c.Status
	o.log.Debug().Msgf("updated instance smon from node %s  -> %s", remote, instSmon.GlobalExpect)
	o.instSmon[remote] = instSmon
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *smon) onSmonDeleted(c msgbus.InstanceMonitorDeleted) {
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

func (o smon) GetInstanceMonitor(node string) (instance.Monitor, bool) {
	if o.localhost == node {
		return o.state, true
	}
	m, ok := o.instSmon[node]
	return m, ok
}

func (o *smon) AllInstanceMonitorStatus(s string) bool {
	for _, instMon := range o.AllInstanceMonitors() {
		if instMon.Status != s {
			return false
		}
	}
	return true
}

func (o smon) AllInstanceMonitors() map[string]instance.Monitor {
	m := make(map[string]instance.Monitor)
	m[o.localhost] = o.state
	for node, instMon := range o.instSmon {
		m[node] = instMon
	}
	return m
}

func (o smon) isExtraInstance() (bool, string) {
	if o.state.IsHALeader {
		return false, "not leader"
	}
	if v, reason := o.isHAOrchestrateable(); !v {
		return false, reason
	}
	if o.svcAgg.Avail != status.Up {
		return false, "not agg up"
	}
	if o.svcAgg.Topology != topology.Flex {
		return false, "not flex"
	}
	if o.svcAgg.UpInstancesCount <= o.svcAgg.FlexTarget {
		return false, fmt.Sprintf("%d/%d up instances", o.svcAgg.UpInstancesCount, o.svcAgg.FlexTarget)
	}
	return true, ""
}

func (o smon) isHAOrchestrateable() (bool, string) {
	if o.svcAgg.Avail == status.Warn {
		return false, "warn agg state"
	}
	switch o.svcAgg.Provisioned {
	case provisioned.Mixed:
		return false, "mixed agg provisioned state"
	case provisioned.False:
		return false, "false agg provisioned state"
	}
	return true, ""
}

func (o smon) isStartable() (bool, string) {
	if v, reason := o.isHAOrchestrateable(); !v {
		return false, reason
	}
	if o.isStarted() {
		return false, "already started"
	}
	return true, "object is startable"
}

func (o smon) isStarted() bool {
	switch o.svcAgg.Topology {
	case topology.Flex:
		return o.svcAgg.UpInstancesCount >= o.svcAgg.FlexTarget
	case topology.Failover:
		return o.svcAgg.Avail == status.Up
	default:
		return false
	}
}

func (o *smon) needOrchestrate(c cmdOrchestrate) {
	if o.state.Status == c.state {
		o.change = true
		o.state.Status = c.newState
		o.updateIfChange()
	}
	o.orchestrate()
}

func (o *smon) sortCandidates(candidates []string) []string {
	switch o.svcAgg.PlacementPolicy {
	case placement.NodesOrder:
		return o.sortWithNodesOrderPolicy(candidates)
	case placement.Spread:
		return o.sortWithSpreadPolicy(candidates)
	case placement.Score:
		return o.sortWithScorePolicy(candidates)
	case placement.Shift:
		return o.sortWithShiftPolicy(candidates)
	default:
		return []string{}
	}
}

func (o *smon) sortWithSpreadPolicy(candidates []string) []string {
	l := append([]string{}, candidates...)
	sum := func(s string) []byte {
		b := append([]byte(o.path.String()), []byte(s)...)
		return md5.New().Sum(b)
	}
	sort.SliceStable(l, func(i, j int) bool {
		return bytes.Compare(sum(l[i]), sum(l[j])) < 0
	})
	return l
}

// sortWithScorePolicy sorts candidates by descending cluster.NodeStats.Score
func (o *smon) sortWithScorePolicy(candidates []string) []string {
	l := append([]string{}, candidates...)
	sort.SliceStable(l, func(i, j int) bool {
		var si, sj uint64
		if stats, ok := o.nodeStats[l[i]]; ok {
			si = stats.Score
		}
		if stats, ok := o.nodeStats[l[j]]; ok {
			sj = stats.Score
		}
		return si > sj
	})
	return l
}

func (o *smon) sortWithLoadAvgPolicy(candidates []string) []string {
	o.log.Warn().Msg("TODO: sortWithLoadAvgPolicy")
	return candidates
}

func (o *smon) sortWithShiftPolicy(candidates []string) []string {
	var i int
	l := o.sortWithNodesOrderPolicy(candidates)
	l = append(l, l...)
	n := len(candidates)
	scalerSliceIndex := o.path.ScalerSliceIndex()
	if n > 0 && scalerSliceIndex > n {
		i = o.path.ScalerSliceIndex() % n
	}
	return candidates[i : i+n]
}

func (o *smon) sortWithNodesOrderPolicy(candidates []string) []string {
	var l []string
	for _, node := range o.scopeNodes {
		if stringslice.Has(node, candidates) {
			l = append(l, node)
		}
	}
	return l
}

func (o *smon) nextPlacedAtCandidates(want string) string {
	want = strings.ReplaceAll(want, ",", " ")
	var wantNodes []string
	for _, node := range nodeselector.LocalExpand(want) {
		if _, ok := o.instStatus[node]; !ok {
			continue
		}
		wantNodes = append(wantNodes, node)
	}
	return strings.Join(wantNodes, ",")
}

func (o *smon) nextPlacedAtCandidate() string {
	if o.svcAgg.Topology == topology.Flex {
		return ""
	}
	var candidates []string
	candidates = append(candidates, o.scopeNodes...)
	candidates = o.sortCandidates(candidates)

	for _, candidate := range candidates {
		if instStatus, ok := o.instStatus[candidate]; ok {
			switch instStatus.Avail {
			case status.Down, status.StandbyDown, status.StandbyUp:
				return candidate
			}
		}
	}
	return ""
}

func (o smon) IsInstanceStartFailed(node string) (bool, bool) {
	instSmon, ok := o.GetInstanceMonitor(node)
	if !ok {
		return false, false
	}
	switch instSmon.Status {
	case statusStartFailed:
		return true, true
	default:
		return false, true
	}
}

func (o *smon) newIsHALeader() bool {
	var candidates []string

	for _, node := range o.scopeNodes {
		if nodeStatus, ok := o.nodeStatus[node]; !ok || nodeStatus.IsFrozen() {
			continue
		}
		if instStatus, ok := o.instStatus[node]; !ok || instStatus.IsFrozen() {
			continue
		}
		if failed, ok := o.IsInstanceStartFailed(node); !ok || failed {
			continue
		}
		candidates = append(candidates, node)
	}
	candidates = o.sortCandidates(candidates)

	var maxLeaders int = 1
	if o.svcAgg.Topology == topology.Flex {
		maxLeaders = o.svcAgg.FlexTarget
	}

	i := stringslice.Index(o.localhost, candidates)
	if i < 0 {
		return false
	}
	return i < maxLeaders
	return false
}

func (o *smon) newIsLeader() bool {
	var candidates []string
	for _, node := range o.scopeNodes {
		if failed, ok := o.IsInstanceStartFailed(node); !ok || failed {
			continue
		}
		candidates = append(candidates, node)
	}
	candidates = o.sortCandidates(candidates)

	var maxLeaders int = 1
	if o.svcAgg.Topology == topology.Flex {
		maxLeaders = o.svcAgg.FlexTarget
	}

	i := stringslice.Index(o.localhost, candidates)
	if i < 0 {
		return false
	}
	return i < maxLeaders
}

func (o *smon) updateIsLeader() {
	if instStatus, ok := o.instStatus[o.localhost]; !ok || instStatus.Avail == status.NotApplicable {
		return
	}
	isLeader := o.newIsLeader()
	if isLeader != o.state.IsLeader {
		o.change = true
		o.state.IsLeader = isLeader
	}
	isHALeader := o.newIsHALeader()
	if isHALeader != o.state.IsHALeader {
		o.change = true
		o.state.IsHALeader = isHALeader
	}
	o.updateIfChange()
	return
}

func (o *smon) parsePlacedAtDestination(s string) *orderedset.OrderedSet {
	set := orderedset.NewOrderedSet()
	l := strings.Split(s, ",")
	if len(l) == 0 {
		return set
	}
	if o.svcAgg.Topology == topology.Failover {
		l = l[:1]
	}
	for _, node := range l {
		set.Add(node)
	}
	return set
}

// doTransitionAction execute action and update transition states
func (o *smon) doTransitionAction(action func() error, newState, successState, errorState string) {
	o.transitionTo(newState)
	if action() != nil {
		o.transitionTo(errorState)
	} else {
		o.transitionTo(successState)
	}
}

// doAction runs action + background orchestration from action state result
//
// 1- set transient state to newState
// 2- run action
// 3- go orchestrateAfterAction(newState, successState or errorState)
func (o *smon) doAction(action func() error, newState, successState, errorState string) {
	o.transitionTo(newState)
	nextState := successState
	if action() != nil {
		nextState = errorState
	}
	go o.orchestrateAfterAction(newState, nextState)
}
