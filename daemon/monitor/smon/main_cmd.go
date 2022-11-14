package smon

import (
	"bytes"
	"crypto/md5"
	"sort"
	"strings"
	"time"

	"github.com/goombaio/orderedset"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/nodeselector"
	"opensvc.com/opensvc/core/placement"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/stringslice"
)

// onSvcAggUpdated updateIfChange state global expect from aggregated status
func (o *smon) onSvcAggUpdated(c msgbus.ObjectAggUpdated) {
	if c.SrcEv != nil {
		switch srcCmd := c.SrcEv.(type) {
		case msgbus.InstanceStatusUpdated:
			srcNode := srcCmd.Node
			srcInstStatus := srcCmd.Status
			if _, ok := o.instStatus[srcNode]; ok {
				if o.instStatus[srcNode].Updated.Before(srcInstStatus.Updated) {
					// only update if more recent
					o.log.Debug().Msgf("ObjectAggUpdated %s from InstanceStatusUpdated on %s update instance status", c.Node, srcNode)
					o.instStatus[srcNode] = srcInstStatus
				} else {
					o.log.Debug().Msgf("ObjectAggUpdated %s from InstanceStatusUpdated on %s skip update instance from obsolete status", c.Node, srcNode)
				}
			} else {
				o.log.Debug().Msgf("ObjectAggUpdated %s from InstanceStatusUpdated on %s create instance status", c.Node, srcNode)
				o.instStatus[srcNode] = srcInstStatus
			}
		case msgbus.CfgUpdated:
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
			o.log.Debug().Msgf("updated from %s ObjectAggUpdated CfgUpdated on %s scopeNodes=%s", c.Node, srcCmd.Node, o.scopeNodes)
		case msgbus.CfgDeleted:
			node := srcCmd.Node
			if _, ok := o.instStatus[node]; ok {
				o.log.Info().Msgf("drop deleted instance status from node %s", node)
				delete(o.instStatus, node)
			}
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
			if o.svcAgg.Avail == status.Up {
				o.log.Info().Msg("preserve global expect: object is already started")
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

func (o *smon) needOrchestrate(c cmdOrchestrate) {
	if o.state.Status == c.state {
		o.change = true
		o.state.Status = c.newState
		o.updateIfChange()
	}
	o.orchestrate()
}

func (o *smon) sortCandidates(policy placement.Policy, candidates []string) []string {
	switch policy {
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

func (o *smon) sortWithScorePolicy(candidates []string) []string {
	o.log.Warn().Msg("TODO: sortWithScorePolicy")
	return candidates
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
	instStatus, ok := o.instStatus[o.localhost]
	if !ok {
		return ""
	}
	if instStatus.Topology == topology.Flex {
		return ""
	}
	var candidates []string
	candidates = append(candidates, o.scopeNodes...)
	candidates = o.sortCandidates(instStatus.Placement, candidates)

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

func (o *smon) newIsLeader(instStatus instance.Status) bool {
	var candidates []string
	candidates = append(candidates, o.scopeNodes...)
	candidates = o.sortCandidates(instStatus.Placement, candidates)

	maxLeaders := 1
	if instStatus.Topology == topology.Flex {
		maxLeaders = instStatus.FlexTarget
	}

	for i, candidate := range candidates {
		if candidate != o.localhost {
			continue
		}
		return i < maxLeaders
	}
	return false
}

func (o *smon) updateIsLeader() {
	instStatus, ok := o.instStatus[o.localhost]
	if !ok {
		o.log.Debug().Msgf("skip updateIsLeader while no instStatus for %s", o.localhost)
		return
	}
	isLeader := o.newIsLeader(instStatus)
	if isLeader != o.state.IsLeader {
		o.change = true
	}
	o.state.IsLeader = isLeader
	o.updateIfChange()
	return
}

func (o *smon) parsePlacedAtDestination(s string) *orderedset.OrderedSet {
	set := orderedset.NewOrderedSet()
	l := strings.Split(s, ",")
	if len(l) == 0 {
		return set
	}
	if instStatus, ok := o.instStatus[o.localhost]; ok && instStatus.Topology == topology.Failover {
		l = l[:1]
	}
	for _, node := range l {
		set.Add(node)
	}
	return set
}
