package imon

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/stringslice"
)

func (o *imon) initRelationAvailStatus() {
	config := instance.ConfigData.Get(o.path, o.localhost)
	if config == nil {
		o.log.Infof("skip relations avail status cache init: no config cached yet")
		return
	}
	do := func(relation naming.Relation, name string, cache map[string]status.T) {
		relationS := relation.String()
		if objectPath, node, err := relation.Split(); err != nil {
			o.log.Warnf("init relation %s status cache: split %s: %s", name, relation)
		} else if node == "" {
			o.log.Infof("init relation subscribe to %s %s object avail status updates and deletes", name, objectPath)
			o.sub.AddFilter(&msgbus.ObjectStatusUpdated{}, pubsub.Label{"path", objectPath.String()})
			o.sub.AddFilter(&msgbus.ObjectStatusDeleted{}, pubsub.Label{"path", objectPath.String()})
			if st := object.StatusData.Get(objectPath); st != nil {
				o.log.Infof("init relation %s %s avail status init to %s", name, relation, st.Avail)
				cache[relationS] = st.Avail
			} else {
				o.log.Infof("init relation %s %s avail status init to %s", o.path, name, relation, status.Undef)
				cache[relationS] = status.Undef
			}
		} else {
			o.log.Infof("subscribe to %s %s@%s instance avail status updates and deletes", name, objectPath, node)
			o.sub.AddFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"path", objectPath.String()}, pubsub.Label{"node", node})
			o.sub.AddFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"path", objectPath.String()}, pubsub.Label{"node", node})
			if st := instance.StatusData.Get(objectPath, node); st != nil {
				o.log.Infof("init relation %s %s avail status init to %s", name, relation, st.Avail)
				cache[relationS] = st.Avail
			} else {
				o.log.Infof("init relation %s %s avail status init to %s", name, relation, status.Undef)
				cache[relationS] = status.Undef
			}
		}
	}
	for _, relation := range config.Children {
		do(relation, "Child", o.state.Children)
	}
	for _, relation := range config.Parents {
		do(relation, "Parent", o.state.Parents)
	}
}

func (o *imon) onRelationObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	if c.Path == o.path {
		// Can't relate to self.
		return
	}
	do := func(relation string, name string, cache map[string]status.T) {
		if v, ok := cache[relation]; ok && v != status.Undef {
			o.log.Infof("update relation %s %s avail status change %s -> %s (deleted object)", name, relation, cache[relation], status.Undef)
			cache[relation] = status.Undef
			o.change = true
		}
	}
	do(c.Path.String(), "Child", o.state.Children)
	do(c.Path.String(), "Parent", o.state.Parents)
}

func (o *imon) onRelationInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	if c.Path == o.path {
		// Can't relate to self.
		return
	}
	do := func(relation string, name string, cache map[string]status.T) {
		if _, ok := cache[relation]; ok {
			o.log.Infof("update relation %s %s avail status change %s -> %s (deleted instance)", name, relation, cache[relation], status.Undef)
			cache[relation] = status.Undef
			o.change = true
		}
	}
	do(c.Path.String()+"@"+c.Node, "Child", o.state.Children)
	do(c.Path.String()+"@"+c.Node, "Parent", o.state.Parents)
}

func (o *imon) onRelationObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	if c.Path == o.path {
		// Can't relate to self. This case is handled by onInstanceStatusUpdated.
		return
	}
	relation := c.Path.String()
	changes := false
	do := func(relation string, name string, cache map[string]status.T) {
		if cache[relation] != c.Value.Avail {
			o.log.Infof("update relation %s %s avail status change %s -> %s", name, relation, cache[relation], c.Value.Avail)
			cache[relation] = c.Value.Avail
			changes = true
		} else {
			o.log.Debugf("update relation %s %s avail status unchanged", name, relation)
		}
	}
	if _, ok := o.state.Children[relation]; ok {
		do(relation, "Child", o.state.Children)
	}
	if _, ok := o.state.Parents[relation]; ok {
		do(relation, "Parent", o.state.Parents)
	}
	if changes {
		o.change = true

		o.objStatus = c.Value
		o.updateIsLeader()
		o.orchestrate()
		o.updateIfChange()
	}
}

func (o *imon) onRelationInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	if c.Path == o.path {
		// Can't relate to self. This case is handled by onInstanceStatusUpdated.
		return
	}
	relation := c.Path.String() + "@" + c.Node
	do := func(relation string, name string, cache map[string]status.T) {
		if cache[relation] != c.Value.Avail {
			o.log.Infof("update relation %s %s avail status change %s -> %s", name, relation, cache[relation], c.Value.Avail)
		} else {
			o.log.Debugf("update relation %s %s avail status unchanged", name, relation)
		}
		cache[relation] = c.Value.Avail
		o.change = true
	}
	if _, ok := o.state.Children[relation]; ok {
		do(relation, "Child", o.state.Children)
	}
	if _, ok := o.state.Parents[relation]; ok {
		do(relation, "Parent", o.state.Parents)
	}
}

func (o *imon) onMyInstanceStatusUpdated(srcNode string, srcCmd *msgbus.InstanceStatusUpdated) {
	updateInstStatusMap := func() {
		instStatus, ok := o.instStatus[srcCmd.Node]
		switch {
		case !ok:
			o.log.Debugf("ObjectStatusUpdated %s from InstanceStatusUpdated on %s create instance status", srcNode, srcCmd.Node)
			o.instStatus[srcCmd.Node] = srcCmd.Value
		case instStatus.UpdatedAt.Before(srcCmd.Value.UpdatedAt):
			// only update if more recent
			o.log.Debugf("ObjectStatusUpdated %s from InstanceStatusUpdated on %s update instance status", srcNode, srcCmd.Node)
			o.instStatus[srcCmd.Node] = srcCmd.Value
		default:
			o.log.Debugf("ObjectStatusUpdated %s from InstanceStatusUpdated on %s skip update instance from obsolete status", srcNode, srcCmd.Node)
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
		o.log.Infof("this instance is now considered started, resource restart and monitoring are enabled")
		o.state.LocalExpect = instance.MonitorLocalExpectStarted

		// reset the last monitor action execution time, to rearm the next monitor action
		o.state.MonitorActionExecutedAt = time.Time{}
		o.change = true

	}

	updateInstStatusMap()
	setLocalExpectStarted()
}

func (o *imon) onInstanceConfigUpdated(srcNode string, srcCmd *msgbus.InstanceConfigUpdated) {
	janitorInstStatus := func(scope []string) {
		cfgNodes := make(map[string]any)

		// init a instance.Status for new peers not yet in the instStatus map
		for _, node := range srcCmd.Value.Scope {
			cfgNodes[node] = nil
			if _, ok := o.instStatus[node]; !ok {
				o.instStatus[node] = instance.Status{Avail: status.Undef}
			}
		}

		// delete the instStatus key for peers gone out of scope
		for node := range o.instStatus {
			if _, ok := cfgNodes[node]; !ok {
				o.log.Debugf("drop instance status cache for node %s (node no longer in the object's expanded node list)", node)
				delete(o.instStatus, node)
			}
		}
	}

	janitorRelations := func(relations []naming.Relation, name string, cache map[string]status.T) {
		// m is a map representation of relations, used to determine
		// if a relation in cache is still in the configured relations
		m := make(map[string]any)

		for _, relation := range relations {
			relationS := relation.String()
			m[relationS] = nil
			if _, ok := cache[relationS]; ok {
				continue
			} else if objectPath, node, err := relation.Split(); err != nil {
				o.log.Warnf("janitor relations %s status cache: split %s: %s", name, relation, err)
				continue
			} else {
				o.log.Infof("janitor relations subscribe to %s %s avail status updates and deletes", name, relationS)
				if node == "" {
					o.sub.AddFilter(&msgbus.ObjectStatusUpdated{}, pubsub.Label{"path", objectPath.String()})
					o.sub.AddFilter(&msgbus.ObjectStatusDeleted{}, pubsub.Label{"path", objectPath.String()})
					if st := object.StatusData.Get(objectPath); st != nil {
						o.log.Infof("janitor relations %s %s avail status init to %s", name, relation, st.Avail)
						cache[relationS] = st.Avail
					} else {
						o.log.Infof("janitor relations %s %s avail status init to %s", name, relation, status.Undef)
						cache[relationS] = status.Undef
					}
					o.change = true
				} else {
					o.sub.AddFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"path", objectPath.String()}, pubsub.Label{"node", node})
					o.sub.AddFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"path", objectPath.String()}, pubsub.Label{"node", node})
					if st := instance.StatusData.Get(objectPath, node); st != nil {
						o.log.Infof("janitor relations %s %s avail status init to %s", name, relation, st.Avail)
						cache[relationS] = st.Avail
					} else {
						o.log.Infof("janitor relations %s %s avail status init to %s", name, relation, status.Undef)
						cache[relationS] = status.Undef
					}
					o.change = true
				}
			}
		}
		for relationS := range cache {
			if _, ok := m[relationS]; !ok {
				o.log.Infof("janitor relations unsubscribe from %s %s avail status updates and deletes", name, relationS)
				objectPath, node, _ := naming.Relation(relationS).Split()
				if node == "" {
					o.sub.DelFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"path", objectPath.String()})
					o.sub.DelFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"path", objectPath.String()})
				} else {
					o.sub.DelFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"path", objectPath.String()}, pubsub.Label{"node", node})
					o.sub.DelFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"path", objectPath.String()})
				}
			}
		}
	}

	if srcCmd.Node == o.localhost {
		defer func() {
			if err := o.crmStatus(); err != nil {
				o.log.Warnf("evaluate instance status via CRM: %s", err)
			}
		}()
		o.instConfig = srcCmd.Value
		o.initResourceMonitor()
		janitorInstStatus(srcCmd.Value.Scope)
		janitorRelations(srcCmd.Value.Children, "Child", o.state.Children)
		janitorRelations(srcCmd.Value.Parents, "Parent", o.state.Parents)
	}
	o.scopeNodes = append([]string{}, srcCmd.Value.Scope...)
	o.log.Debugf("updated from %s ObjectStatusUpdated InstanceConfigUpdated on %s scopeNodes=%s", srcNode, srcCmd.Node, o.scopeNodes)
}

func (o *imon) onMyInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	if _, ok := o.instStatus[c.Node]; ok {
		o.log.Debugf("drop deleted instance status from node %s", c.Node)
		delete(o.instStatus, c.Node)
	}
}

func (o *imon) onInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	if o.path != c.Path {
		o.onRelationInstanceStatusDeleted(c)
	}
}

func (o *imon) onObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	if o.path == c.Path {
		o.onMyObjectStatusUpdated(c)
	} else {
		o.onRelationObjectStatusUpdated(c)
	}
}

func (o *imon) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	if o.path != c.Path {
		o.onRelationObjectStatusDeleted(c)
	}
}

func (o *imon) onMyObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	if c.SrcEv != nil {
		switch srcCmd := c.SrcEv.(type) {
		case *msgbus.InstanceStatusDeleted:
			o.onMyInstanceStatusDeleted(srcCmd)
		case *msgbus.InstanceStatusUpdated:
			o.onMyInstanceStatusUpdated(c.Node, srcCmd)
		case *msgbus.InstanceConfigUpdated:
			o.onInstanceConfigUpdated(c.Node, srcCmd)
		case *msgbus.InstanceConfigDeleted:
			// just a reminder here: no action on InstanceConfigDeleted because
			// if our instance config is deleted our omon launcher will cancel us
		case *msgbus.InstanceMonitorDeleted:
			if srcCmd.Node == o.localhost {
				// this is not expected
				o.log.Warnf("unexpected received ObjectStatusUpdated from self InstanceMonitorDeleted")
			} else {
				o.onInstanceMonitorDeletedFromNode(srcCmd.Node)
			}
		case *msgbus.InstanceMonitorUpdated:
			o.onInstanceMonitorUpdated(srcCmd)
		}
	}
	o.objStatus = c.Value
	o.updateIsLeader()
	o.orchestrate()
	o.updateIfChange()
}

// onProgressInstanceMonitor updates the fields of instance.Monitor applying policies:
// if state goes from stopping/shutting to idle and local expect is started, reset the
// local expect, so the resource restart is disabled.
func (o *imon) onProgressInstanceMonitor(c *msgbus.ProgressInstanceMonitor) {
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
		o.log.Infof("this instance is no longer considered started, resource restart and monitoring are disabled")
		o.change = true
		o.state.LocalExpect = instance.MonitorLocalExpectNone
	}
	doState := func() {
		if prevState == c.State {
			return
		}
		switch o.state.SessionId {
		case uuid.Nil:
		case c.SessionId:
			// pass
		default:
			o.log.Warnf("received progress instance monitor for wrong sid state %s(%s) -> %s(%s)", o.state.State, o.state.SessionId, c.State, c.SessionId)
		}
		o.log.Infof("set instance monitor state %s -> %s", o.state.State, c.State)
		o.change = true
		o.state.State = c.State
		if c.State == instance.MonitorStateIdle {
			o.state.SessionId = uuid.Nil
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

func (o *imon) onSetInstanceMonitor(c *msgbus.SetInstanceMonitor) {
	sendError := func(err error) {
		if c.Err != nil {
			c.Err <- err
		}
	}
	doState := func() {
		if c.Value.State == nil {
			return
		}
		if _, ok := instance.MonitorStateStrings[*c.Value.State]; !ok {
			err := fmt.Errorf("%w: %s", instance.ErrInvalidState, *c.Value.State)
			sendError(err)
			o.log.Warnf("set instance monitor: %s", err)
			return
		}
		if *c.Value.State == instance.MonitorStateZero {
			err := fmt.Errorf("%w: %s", instance.ErrInvalidState, *c.Value.State)
			sendError(err)
			return
		}
		if o.state.State == *c.Value.State {
			err := fmt.Errorf("%w: %s", instance.ErrSameState, *c.Value.State)
			sendError(err)
			o.log.Infof("set instance monitor: %s", err)
			return
		}
		o.log.Infof("set instance monitor state %s -> %s", o.state.State, *c.Value.State)
		o.change = true
		o.state.State = *c.Value.State
	}

	globalExpectRefused := func() {
		o.pubsubBus.Pub(&msgbus.SetInstanceMonitorRefused{
			Path:  o.path,
			Node:  o.localhost,
			Value: c.Value,
		}, o.labelPath, o.labelLocalhost)
	}

	doGlobalExpect := func() {
		if c.Value.GlobalExpect == nil {
			return
		}
		if _, ok := instance.MonitorGlobalExpectStrings[*c.Value.GlobalExpect]; !ok {
			err := fmt.Errorf("%w: %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect)
			sendError(err)
			o.log.Warnf("set instance monitor: %s", err)
			globalExpectRefused()
			return
		}
		if o.state.OrchestrationId != uuid.Nil && *c.Value.GlobalExpect != instance.MonitorGlobalExpectAborted {
			err := fmt.Errorf("%w: daemon: imon: %s: a %s orchestration is already in progress with id %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, o.state.GlobalExpect, o.state.OrchestrationId)
			sendError(err)
			return
		}
		switch *c.Value.GlobalExpect {
		case instance.MonitorGlobalExpectPlacedAt:
			options, ok := c.Value.GlobalExpectOptions.(instance.MonitorGlobalExpectOptionsPlacedAt)
			if !ok || len(options.Destination) == 0 {
				// Switch cmd without explicit target nodes.
				// Select some nodes automatically.
				dst := o.nextPlacedAtCandidate()
				if dst == "" {
					err := fmt.Errorf("%w: daemon: imon: %s: no destination node could be selected from candidates", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect)
					sendError(err)
					o.log.Infof("set instance monitor: %s", err)
					globalExpectRefused()
					return
				}
				options.Destination = []string{dst}
				c.Value.GlobalExpectOptions = options
			} else {
				want := options.Destination
				can, err := o.nextPlacedAtCandidates(want)
				if err != nil {
					err2 := fmt.Errorf("%w: daemon: imon: %s: no destination node could ne selected from %s: %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, want, err)
					sendError(err2)
					o.log.Infof("set instance monitor: %s", err)
					globalExpectRefused()
					return
				}
				if can == "" {
					err := fmt.Errorf("%w: daemon: imon: %s: no destination node could ne selected from %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, want)
					sendError(err)
					o.log.Infof("set instance monitor: %s", err)
					globalExpectRefused()
					return
				} else if can != want[0] {
					o.log.Infof("set instance monitor: change destination nodes from %s to %s", want, can)
				}
				options.Destination = []string{can}
				c.Value.GlobalExpectOptions = options
			}
		case instance.MonitorGlobalExpectStarted:
			if v, reason := o.isStartable(); !v {
				err := fmt.Errorf("%w: daemon: imon: %s: %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, reason)
				sendError(err)
				o.log.Infof("set instance monitor %s", o.path, err)
				globalExpectRefused()
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
			if instMon.GlobalExpectUpdatedAt.After(o.state.GlobalExpectUpdatedAt) {
				err := fmt.Errorf("%w: daemon: imon: %s: more recent value %s on node %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, instMon.GlobalExpect, node)
				sendError(err)
				o.log.Infof("set instance monitor: %s", o.path, err)
				globalExpectRefused()
				return
			}
		}

		if *c.Value.GlobalExpect != o.state.GlobalExpect {
			o.change = true
			o.state.GlobalExpect = *c.Value.GlobalExpect
			o.state.GlobalExpectOptions = c.Value.GlobalExpectOptions
			// update GlobalExpectUpdated now
			// This will allow remote nodes to pickup most recent value
			o.state.GlobalExpectUpdatedAt = time.Now()

			// reset state to idle to allow the new orchestration to begin
			o.state.State = instance.MonitorStateIdle
			o.state.OrchestrationIsDone = false
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
			err := fmt.Errorf("%w: %s", instance.ErrInvalidLocalExpect, *c.Value.LocalExpect)
			sendError(err)
			o.log.Warnf("set instance monitor: %s", err)
			return
		}
		target := *c.Value.LocalExpect
		if o.state.LocalExpect == target {
			err := fmt.Errorf("%w: %s", instance.ErrSameLocalExpect, *c.Value.LocalExpect)
			sendError(err)
			o.log.Infof("set instance monitor: %s", err)
			return
		}
		o.log.Infof("set instance monitor: set local expect %s -> %s", o.state.LocalExpect, target)
		o.change = true
		o.state.LocalExpect = target
	}

	doState()
	doGlobalExpect()
	doLocalExpect()

	// inform the publisher we're done sending errors
	sendError(nil)

	if o.change {
		if o.state.OrchestrationId.String() != c.Value.CandidateOrchestrationId.String() {
			o.log = o.newLogger(c.Value.CandidateOrchestrationId)
		}
		o.state.OrchestrationId = c.Value.CandidateOrchestrationId
		o.acceptedOrchestrationId = c.Value.CandidateOrchestrationId
		o.updateIsLeader()
		o.orchestrate()
		o.updateIfChange()
	} else {
		o.pubsubBus.Pub(&msgbus.ObjectOrchestrationRefused{
			Node:   o.localhost,
			Path:   o.path,
			Id:     c.Value.CandidateOrchestrationId.String(),
			Reason: fmt.Sprintf("set instance monitor request => no changes: %v", c.Value),
		},
			o.labelPath,
			o.labelLocalhost,
		)
	}
}

func (o *imon) onNodeConfigUpdated(c *msgbus.NodeConfigUpdated) {
	o.readyDuration = c.Value.ReadyPeriod
	o.orchestrate()
	o.updateIfChange()
}

func (o *imon) onNodeMonitorUpdated(c *msgbus.NodeMonitorUpdated) {
	o.nodeMonitor[c.Node] = c.Value
	o.updateIsLeader()
	o.orchestrate()
	o.updateIfChange()
}

func (o *imon) onNodeStatusUpdated(c *msgbus.NodeStatusUpdated) {
	o.nodeStatus[c.Node] = c.Value
	o.updateIsLeader()
	o.orchestrate()
	o.updateIfChange()
}

func (o *imon) onNodeStatsUpdated(c *msgbus.NodeStatsUpdated) {
	o.nodeStats[c.Node] = c.Value
	if o.objStatus.PlacementPolicy == placement.Score {
		o.updateIsLeader()
		o.orchestrate()
		o.updateIfChange()
	}
}

func (o *imon) onInstanceMonitorUpdated(c *msgbus.InstanceMonitorUpdated) {
	// ignore self msgbus.InstanceMonitorUpdated
	if c.Node != o.localhost {
		o.onRemoteInstanceMonitorUpdated(c)
	}
}

func (o *imon) onRemoteInstanceMonitorUpdated(c *msgbus.InstanceMonitorUpdated) {
	remote := c.Node
	instMon := c.Value
	o.log.Debugf("updated instance imon from peer node %s -> global expect:%s, state: %s", remote, instMon.GlobalExpect, instMon.State)
	o.instMonitor[remote] = instMon
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *imon) onInstanceMonitorDeletedFromNode(node string) {
	if node == o.localhost {
		// this is not expected
		o.log.Warnf("onInstanceMonitorDeletedFromNode should never be called from localhost")
		return
	}
	o.log.Debugf("delete remote instance imon from node %s", node)
	delete(o.instMonitor, node)
	o.convergeGlobalExpectFromRemote()
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *imon) GetInstanceMonitor(node string) (instance.Monitor, bool) {
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

func (o *imon) AllInstanceMonitors() map[string]instance.Monitor {
	m := make(map[string]instance.Monitor)
	m[o.localhost] = o.state
	for node, instMon := range o.instMonitor {
		if node == o.localhost {
			err := fmt.Errorf("Func AllInstanceMonitors is not expected to have localhost in o.instMonitor keys")
			o.log.Errorf("%s", err)
			panic(err)
		}
		m[node] = instMon
	}
	return m
}

func (o *imon) isExtraInstance() (bool, string) {
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

func (o *imon) isHAOrchestrateable() (bool, string) {
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

func (o *imon) isStartable() (bool, string) {
	if v, reason := o.isHAOrchestrateable(); !v {
		return false, reason
	}
	if o.isStarted() {
		return false, "already started"
	}
	return true, "object is startable"
}

func (o *imon) isStarted() bool {
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
	case placement.LastStart:
		return o.sortWithLastStartPolicy(candidates)
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
	l := append([]string{}, candidates...)
	sort.SliceStable(l, func(i, j int) bool {
		var si, sj float64
		if stats, ok := o.nodeStats[l[i]]; ok {
			si = stats.Load15M
		}
		if stats, ok := o.nodeStats[l[j]]; ok {
			sj = stats.Load15M
		}
		return si > sj
	})
	return l
}

func (o *imon) sortWithLastStartPolicy(candidates []string) []string {
	l := append([]string{}, candidates...)
	sort.SliceStable(l, func(i, j int) bool {
		var si, sj time.Time
		if instStatus, ok := o.instStatus[l[i]]; ok {
			si = instStatus.LastStartedAt
		}
		if instStatus, ok := o.instStatus[l[j]]; ok {
			sj = instStatus.LastStartedAt
		}
		return si.After(sj)
	})
	return l
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

func (o *imon) nextPlacedAtCandidates(want []string) (string, error) {
	expr := strings.Join(want, " ")
	var wantNodes []string
	nodes, err := nodeselector.LocalExpand(expr)
	if err != nil {
		return "", err
	}
	for _, node := range nodes {
		if _, ok := o.instStatus[node]; !ok {
			continue
		}
		wantNodes = append(wantNodes, node)
	}
	return strings.Join(wantNodes, ","), nil
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

func (o *imon) IsInstanceStatusNotApplicable(node string) (bool, bool) {
	instStatus, ok := o.instStatus[node]
	if !ok {
		return false, false
	}
	switch instStatus.Avail {
	case status.NotApplicable:
		return true, true
	default:
		return false, true
	}
}

func (o *imon) IsInstanceStartFailed(node string) (bool, bool) {
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

func (o *imon) IsNodeMonitorStatusRankable(node string) (bool, bool) {
	nodeMonitor, ok := o.nodeMonitor[node]
	if !ok {
		return false, false
	}
	return nodeMonitor.State.IsRankable(), true
}

func (o *imon) newIsHALeader() bool {
	var candidates []string

	for _, node := range o.scopeNodes {
		if v, ok := o.IsInstanceStatusNotApplicable(node); !ok || v {
			continue
		}
		if nodeStatus, ok := o.nodeStatus[node]; !ok || nodeStatus.IsFrozen() {
			continue
		}
		if instStatus, ok := o.instStatus[node]; !ok || instStatus.IsFrozen() {
			continue
		}
		if instStatus, ok := o.instStatus[node]; !ok || instStatus.Provisioned.IsOneOf(provisioned.Mixed, provisioned.False) {
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
		if v, ok := o.IsInstanceStatusNotApplicable(node); !ok || v {
			continue
		}
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
	m := make(instance.ResourceMonitors, 0)
	for rid, rcfg := range o.instConfig.Resources {
		m[rid] = instance.ResourceMonitor{
			Restart: instance.ResourceMonitorRestart{
				Remaining: rcfg.Restart,
			},
		}
	}
	o.state.Resources = m
	o.change = true
}

func (o *imon) onNodeRejoin(c *msgbus.NodeRejoin) {
	if c.IsUpgrading {
		return
	}
	if len(o.instStatus) < 2 {
		// no need to merge frozen if the object has a single instance
		return
	}
	instStatus, ok := o.instStatus[o.localhost]
	if !ok {
		return
	}
	if !instStatus.FrozenAt.IsZero() {
		// already frozen
		return
	}
	if o.state.GlobalExpect == instance.MonitorGlobalExpectThawed {
		return
	}
	if o.instConfig.Orchestrate != "ha" {
		return
	}
	for peer, peerStatus := range o.instStatus {
		if peer == o.localhost {
			continue
		}
		if peerStatus.FrozenAt.After(c.LastShutdownAt) {
			msg := fmt.Sprintf("Freeze %s instance because peer %s instance was frozen while this daemon was down", o.path, peer)
			if err := o.crmFreeze(); err != nil {
				o.log.Infof("%s: %s", msg, err)
			} else {
				o.log.Infof(msg)
			}
			return
		}

	}
}
