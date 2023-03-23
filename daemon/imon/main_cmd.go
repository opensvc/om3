package imon

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/stringslice"
)

func (o *imon) onInstanceStatusUpdated(srcNode string, srcCmd msgbus.InstanceStatusUpdated) {
	updateInstStatusMap := func() {
		instStatus, ok := o.instStatus[srcCmd.Node]
		switch {
		case !ok:
			o.log.Debug().Msgf("ObjectStatusUpdated %s from InstanceStatusUpdated on %s create instance status", srcNode, srcCmd.Node)
			o.instStatus[srcCmd.Node] = srcCmd.Value
		case instStatus.Updated.Before(srcCmd.Value.Updated):
			// only update if more recent
			o.log.Debug().Msgf("ObjectStatusUpdated %s from InstanceStatusUpdated on %s update instance status", srcNode, srcCmd.Node)
			o.instStatus[srcCmd.Node] = srcCmd.Value
		default:
			o.log.Debug().Msgf("ObjectStatusUpdated %s from InstanceStatusUpdated on %s skip update instance from obsolete status", srcNode, srcCmd.Node)
		}
	}
	setLocalExpectStarted := func() {
		if srcCmd.Node != o.localhost {
			return
		}
		if o.state.State != instance.MonitorStateIdle {
			// wait for idle state, we may be MonitorStateProvisioning, MonitorStateProvisioned ...
			return
		}
		if !srcCmd.Value.Avail.Is(status.Up) {
			return
		}
		if o.state.LocalExpect == instance.MonitorLocalExpectStarted {
			return
		}
		o.log.Info().Msgf("this instance is now considered started, resource restart and monitoring are enabled")
		o.state.LocalExpect = instance.MonitorLocalExpectStarted

		// reset the last monitor action execution time, to rearm the next monitor action
		o.state.MonitorActionExecutedAt = time.Time{}
		o.change = true

	}

	updateInstStatusMap()
	setLocalExpectStarted()
}

func (o *imon) onInstanceConfigUpdated(srcNode string, srcCmd msgbus.InstanceConfigUpdated) {
	if srcCmd.Node == o.localhost {
		o.instConfig = srcCmd.Value
		o.initResourceMonitor()
		cfgNodes := make(map[string]any)
		for _, node := range srcCmd.Value.Scope {
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
	o.scopeNodes = append([]string{}, srcCmd.Value.Scope...)
	o.log.Debug().Msgf("updated from %s ObjectStatusUpdated InstanceConfigUpdated on %s scopeNodes=%s", srcNode, srcCmd.Node, o.scopeNodes)
}

func (o *imon) onInstanceConfigDeleted(srcNode string, srcCmd msgbus.InstanceConfigDeleted) {
	if _, ok := o.instStatus[srcCmd.Node]; ok {
		o.log.Info().Msgf("drop deleted instance status from node %s", srcCmd.Node)
		delete(o.instStatus, srcCmd.Node)
	}
}

// onObjectStatusUpdated updateIfChange state global expect from object status
func (o *imon) onObjectStatusUpdated(c msgbus.ObjectStatusUpdated) {
	if c.SrcEv != nil {
		switch srcCmd := c.SrcEv.(type) {
		case msgbus.InstanceStatusUpdated:
			o.onInstanceStatusUpdated(c.Node, srcCmd)
		case msgbus.InstanceConfigUpdated:
			o.onInstanceConfigUpdated(c.Node, srcCmd)
		case msgbus.InstanceConfigDeleted:
			o.onInstanceConfigDeleted(c.Node, srcCmd)
		case msgbus.InstanceMonitorUpdated:
			o.onInstanceMonitorUpdated(srcCmd)
		case msgbus.InstanceMonitorDeleted:
			o.onInstanceMonitorDeleted(srcCmd)
		}
	}
	o.objStatus = c.Value
	o.updateIsLeader()
	o.orchestrate()
	o.updateIfChange()
}

// onProgressInstanceMonitor updates the fields of instance.Monitor applying policies:
// if state goes from stopping/shutting to idle and local expect is started, reset the
// the local expect, so the resource restart is disabled.
func (o *imon) onProgressInstanceMonitor(c msgbus.ProgressInstanceMonitor) {
	prevState := o.state.State
	doLocalExpect := func() {
		if !o.change {
			return
		}
		if c.IsPartial {
			return
		}
		if c.State != instance.MonitorStateIdle {
			return
		}
		if o.state.LocalExpect != instance.MonitorLocalExpectStarted {
			return
		}
		switch prevState {
		case instance.MonitorStateStopping, instance.MonitorStateShutting:
			// pass
		default:
			return
		}
		o.log.Info().Msgf("this instance is no longer considered started, resource restart and monitoring are disabled")
		o.change = true
		o.state.LocalExpect = instance.MonitorLocalExpectNone
	}
	doState := func() {
		if prevState == c.State {
			return
		}
		switch o.state.SessionId {
		case "":
		case c.SessionId:
			// pass
		default:
			o.log.Warn().Msgf("received progress instance monitor for wrong sid state %s(%s) -> %s(%s)", o.state.State, o.state.SessionId, c.State, c.SessionId)
		}
		o.log.Info().Msgf("set instance monitor state %s -> %s", o.state.State, c.State)
		o.change = true
		o.state.State = c.State
		if c.State == instance.MonitorStateIdle {
			o.state.SessionId = ""
		} else {
			o.state.SessionId = c.SessionId
		}
	}

	doState()
	doLocalExpect()

	if o.change {
		o.updateIsLeader()
		o.orchestrate()
		o.updateIfChange()
	}
}

func (o *imon) onSetInstanceMonitor(c msgbus.SetInstanceMonitor) {
	doState := func() {
		if c.Value.State == nil {
			return
		}
		if _, ok := instance.MonitorStateStrings[*c.Value.State]; !ok {
			o.log.Warn().Msgf("invalid set instance monitor state: %s", *c.Value.State)
			return
		}
		if *c.Value.State == instance.MonitorStateZero {
			return
		}
		if o.state.State == *c.Value.State {
			o.log.Info().Msgf("instance monitor state is already %s", *c.Value.State)
			return
		}
		o.log.Info().Msgf("set instance monitor state %s -> %s", o.state.State, *c.Value.State)
		o.change = true
		o.state.State = *c.Value.State
	}

	doGlobalExpect := func() {
		if c.Value.GlobalExpect == nil {
			return
		}
		if _, ok := instance.MonitorGlobalExpectStrings[*c.Value.GlobalExpect]; !ok {
			o.log.Warn().Msgf("refuse to set global expect '%s': invalid value", *c.Value.GlobalExpect)
			return
		}
		switch *c.Value.GlobalExpect {
		case instance.MonitorGlobalExpectZero:
			return
		case instance.MonitorGlobalExpectPlacedAt:
			options, ok := c.Value.GlobalExpectOptions.(instance.MonitorGlobalExpectOptionsPlacedAt)
			if !ok || len(options.Destination) == 0 {
				// Switch cmd without explicit target nodes.
				// Select some nodes automatically.
				dst := o.nextPlacedAtCandidate()
				if dst == "" {
					o.log.Info().Msgf("refuse to set global expect '%s': no destination node could be selected from candidates", *c.Value.GlobalExpect)
					return
				}
				options.Destination = []string{dst}
				c.Value.GlobalExpectOptions = options
			} else {
				want := options.Destination
				can := o.nextPlacedAtCandidates(want)
				if can == "" {
					o.log.Info().Msgf("refuse to set global expect '%s': no destination node could be selected from %s", *c.Value.GlobalExpect, want)
					return
				} else if can != want[0] {
					o.log.Info().Msgf("change destination nodes from %s to %s", want, can)
				}
				options.Destination = []string{can}
				c.Value.GlobalExpectOptions = options
			}
		case instance.MonitorGlobalExpectStarted:
			if v, reason := o.isStartable(); !v {
				o.log.Info().Msgf("refuse to set global expect '%s': %s", *c.Value.GlobalExpect, reason)
				return
			}
		}
		for node, instMon := range o.instMonitor {
			if instMon.GlobalExpect == *c.Value.GlobalExpect {
				continue
			}
			if instMon.GlobalExpect == instance.MonitorGlobalExpectZero {
				continue
			}
			if instMon.GlobalExpect == instance.MonitorGlobalExpectNone {
				continue
			}
			if instMon.GlobalExpectUpdated.After(o.state.GlobalExpectUpdated) {
				o.log.Info().Msgf("refuse to set global expect '%s': node %s global expect is already '%s'", instMon.GlobalExpect, node, *c.Value.GlobalExpect)
				return
			}
		}

		if *c.Value.GlobalExpect != o.state.GlobalExpect {
			o.change = true
			o.state.GlobalExpect = *c.Value.GlobalExpect
			o.state.GlobalExpectOptions = c.Value.GlobalExpectOptions
			// update GlobalExpectUpdated now
			// This will allow remote nodes to pickup most recent value
			o.state.GlobalExpectUpdated = time.Now()
		}
	}

	doLocalExpect := func() {
		if c.Value.LocalExpect == nil {
			return
		}
		switch *c.Value.LocalExpect {
		case instance.MonitorLocalExpectNone:
		case instance.MonitorLocalExpectStarted:
		default:
			o.log.Warn().Msgf("invalid set instance monitor local expect: %s", *c.Value.LocalExpect)
			return
		}
		target := *c.Value.LocalExpect
		if o.state.LocalExpect == target {
			o.log.Info().Msgf("local expect is already %s", *c.Value.LocalExpect)
			return
		}
		o.log.Info().Msgf("set local expect %s -> %s", o.state.LocalExpect, target)
		o.change = true
		o.state.LocalExpect = target
	}

	doState()
	doGlobalExpect()
	doLocalExpect()

	if o.change {
		o.state.OrchestrationId = c.Value.CandidateOrchestrationId
		o.acceptedOrchestrationId = c.Value.CandidateOrchestrationId
		o.updateIsLeader()
		o.orchestrate()
		o.updateIfChange()
	} else {
		o.pubsubBus.Pub(msgbus.ObjectOrchestrationEnd{
			Node:  o.localhost,
			Path:  o.path,
			Id:    c.Value.CandidateOrchestrationId,
			Error: errors.Errorf("dropped set instance monitor request: %v", c.Value),
		},
			o.labelPath,
			o.labelLocalhost,
		)
	}

}

func (o *imon) onNodeConfigUpdated(c msgbus.NodeConfigUpdated) {
	o.readyDuration = c.Value.ReadyPeriod
	o.orchestrate()
	o.updateIfChange()
}

func (o *imon) onNodeMonitorUpdated(c msgbus.NodeMonitorUpdated) {
	o.nodeMonitor[c.Node] = c.Value
	o.updateIsLeader()
	o.orchestrate()
	o.updateIfChange()
}

func (o *imon) onNodeStatusUpdated(c msgbus.NodeStatusUpdated) {
	o.nodeStatus[c.Node] = c.Value
	o.updateIsLeader()
	o.orchestrate()
	o.updateIfChange()
}

func (o *imon) onNodeStatsUpdated(c msgbus.NodeStatsUpdated) {
	o.nodeStats[c.Node] = c.Value
	if o.objStatus.PlacementPolicy == placement.Score {
		o.updateIsLeader()
		o.orchestrate()
		o.updateIfChange()
	}
}

func (o *imon) onInstanceMonitorUpdated(c msgbus.InstanceMonitorUpdated) {
	if c.Node != o.localhost {
		o.onRemoteInstanceMonitorUpdated(c)
	}
}

func (o *imon) onRemoteInstanceMonitorUpdated(c msgbus.InstanceMonitorUpdated) {
	remote := c.Node
	instMon := c.Value
	o.log.Debug().Msgf("updated instance imon from node %s  -> %s", remote, instMon.GlobalExpect)
	o.instMonitor[remote] = instMon
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *imon) onInstanceMonitorDeleted(c msgbus.InstanceMonitorDeleted) {
	node := c.Node
	if node == o.localhost {
		return
	}
	o.log.Debug().Msgf("delete remote instance imon from node %s", node)
	delete(o.instMonitor, c.Node)
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o imon) GetInstanceMonitor(node string) (instance.Monitor, bool) {
	if o.localhost == node {
		return o.state, true
	}
	m, ok := o.instMonitor[node]
	return m, ok
}

func (o *imon) AllInstanceMonitorState(s instance.MonitorState) bool {
	for _, instMon := range o.AllInstanceMonitors() {
		if instMon.State != s {
			return false
		}
	}
	return true
}

func (o imon) AllInstanceMonitors() map[string]instance.Monitor {
	m := make(map[string]instance.Monitor)
	m[o.localhost] = o.state
	for node, instMon := range o.instMonitor {
		m[node] = instMon
	}
	return m
}

func (o imon) isExtraInstance() (bool, string) {
	if o.state.IsHALeader {
		return false, "object is not leader"
	}
	if v, reason := o.isHAOrchestrateable(); !v {
		return false, reason
	}
	if o.objStatus.Avail != status.Up {
		return false, "object is not up"
	}
	if o.objStatus.Topology != topology.Flex {
		return false, "object is not flex"
	}
	if o.objStatus.UpInstancesCount <= o.objStatus.FlexTarget {
		return false, fmt.Sprintf("%d/%d up instances", o.objStatus.UpInstancesCount, o.objStatus.FlexTarget)
	}
	return true, ""
}

func (o imon) isHAOrchestrateable() (bool, string) {
	if (o.objStatus.Topology == topology.Failover) && (o.objStatus.Avail == status.Warn) {
		return false, "failover object is warn state"
	}
	switch o.objStatus.Provisioned {
	case provisioned.Mixed:
		return false, "mixed object provisioned state"
	case provisioned.False:
		return false, "false object provisioned state"
	}
	return true, ""
}

func (o imon) isStartable() (bool, string) {
	if v, reason := o.isHAOrchestrateable(); !v {
		return false, reason
	}
	if o.isStarted() {
		return false, "already started"
	}
	return true, "object is startable"
}

func (o imon) isStarted() bool {
	switch o.objStatus.Topology {
	case topology.Flex:
		return o.objStatus.UpInstancesCount >= o.objStatus.FlexTarget
	case topology.Failover:
		return o.objStatus.Avail == status.Up
	default:
		return false
	}
}

func (o *imon) needOrchestrate(c cmdOrchestrate) {
	if c.state == instance.MonitorStateZero {
		return
	}
	select {
	case <-o.ctx.Done():
		return
	default:
	}
	if o.state.State == c.state {
		o.change = true
		o.state.State = c.newState
		o.updateIfChange()
	}
	select {
	case <-o.ctx.Done():
		return
	default:
	}
	o.orchestrate()
}

func (o *imon) sortCandidates(candidates []string) []string {
	switch o.objStatus.PlacementPolicy {
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

func (o *imon) sortWithSpreadPolicy(candidates []string) []string {
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
func (o *imon) sortWithScorePolicy(candidates []string) []string {
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

func (o *imon) sortWithLoadAvgPolicy(candidates []string) []string {
	o.log.Warn().Msg("TODO: sortWithLoadAvgPolicy")
	return candidates
}

func (o *imon) sortWithShiftPolicy(candidates []string) []string {
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

func (o *imon) sortWithNodesOrderPolicy(candidates []string) []string {
	var l []string
	for _, node := range o.scopeNodes {
		if stringslice.Has(node, candidates) {
			l = append(l, node)
		}
	}
	return l
}

func (o *imon) nextPlacedAtCandidates(want []string) string {
	expr := strings.Join(want, " ")
	var wantNodes []string
	for _, node := range nodeselector.LocalExpand(expr) {
		if _, ok := o.instStatus[node]; !ok {
			continue
		}
		wantNodes = append(wantNodes, node)
	}
	return strings.Join(wantNodes, ",")
}

func (o *imon) nextPlacedAtCandidate() string {
	if o.objStatus.Topology == topology.Flex {
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

func (o imon) IsInstanceStartFailed(node string) (bool, bool) {
	instMon, ok := o.GetInstanceMonitor(node)
	if !ok {
		return false, false
	}
	switch instMon.State {
	case instance.MonitorStateStartFailed:
		return true, true
	default:
		return false, true
	}
}

func (o imon) IsNodeMonitorStatusRankable(node string) (bool, bool) {
	nodeMonitor, ok := o.nodeMonitor[node]
	if !ok {
		return false, false
	}
	return nodeMonitor.State.IsRankable(), true
}

func (o *imon) newIsHALeader() bool {
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
		if v, ok := o.IsNodeMonitorStatusRankable(node); !ok || !v {
			continue
		}
		candidates = append(candidates, node)
	}
	candidates = o.sortCandidates(candidates)

	var maxLeaders int = 1
	if o.objStatus.Topology == topology.Flex {
		maxLeaders = o.objStatus.FlexTarget
	}

	i := stringslice.Index(o.localhost, candidates)
	if i < 0 {
		return false
	}
	return i < maxLeaders
}

func (o *imon) newIsLeader() bool {
	var candidates []string
	for _, node := range o.scopeNodes {
		if failed, ok := o.IsInstanceStartFailed(node); !ok || failed {
			continue
		}
		candidates = append(candidates, node)
	}
	candidates = o.sortCandidates(candidates)

	var maxLeaders int = 1
	if o.objStatus.Topology == topology.Flex {
		maxLeaders = o.objStatus.FlexTarget
	}

	i := stringslice.Index(o.localhost, candidates)
	if i < 0 {
		return false
	}
	return i < maxLeaders
}

func (o *imon) updateIsLeader() {
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

// doTransitionAction execute action and update transition states
func (o *imon) doTransitionAction(action func() error, newState, successState, errorState instance.MonitorState) {
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
func (o *imon) doAction(action func() error, newState, successState, errorState instance.MonitorState) {
	o.transitionTo(newState)
	nextState := successState
	if action() != nil {
		nextState = errorState
	}
	go o.orchestrateAfterAction(newState, nextState)
}

func (o *imon) initResourceMonitor() {
	m := make(map[string]instance.ResourceMonitor)
	for rid, res := range o.instConfig.Resources {
		m[rid] = instance.ResourceMonitor{
			Restart: instance.ResourceMonitorRestart{
				Remaining: res.Restart,
			},
		}
	}
	o.state.Resources = m
	o.change = true
}
