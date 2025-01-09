package imon

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"slices"
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
	"github.com/opensvc/om3/daemon/runner"
	"github.com/opensvc/om3/util/errcontext"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/stringslice"
)

func (t *Manager) onChange() {
	t.enableDelayTimer()
	t.updateIsLeader()
	t.delayOrchestrateEnabled = true
	t.delayUpdateEnabled = true
}

func (t *Manager) updateOrchestrateUpdate() {
	t.enableDelayTimer()
	t.delayPreUpdateEnabled = true
	t.delayOrchestrateEnabled = true
	t.delayUpdateEnabled = true
}

// enableDelayTimer reset delayTimer with delayDuration is the timer is not yet
// enabled.
func (t *Manager) enableDelayTimer() {
	if t.delayTimerEnabled {
		return
	}
	t.delayTimer.Reset(t.delayDuration)
	t.delayTimerEnabled = true
}

// onDelayTimer is run when the delay timer is fired.
// It runs the enabled delayed actions:
//
//	updateIfChange() if delayPreUpdateEnabled
//	orchestrate() if delayOrchestrateEnabled
//	updateIfChange() if delayUpdateEnabled
//
// It also clears the delayTimerEnabled
func (t *Manager) onDelayTimer() {
	t.delayTimerEnabled = false
	if t.delayPreUpdateEnabled {
		t.delayPreUpdateEnabled = false
		t.updateIfChange()
	}
	if t.delayOrchestrateEnabled {
		t.delayOrchestrateEnabled = false
		t.orchestrate()
	}
	if t.delayUpdateEnabled {
		t.delayUpdateEnabled = false
		t.updateIfChange()
	}
}

func (t *Manager) initRelationAvailStatus() {
	config := instance.ConfigData.Get(t.path, t.localhost)
	if config == nil {
		t.log.Infof("skip relations avail status cache init: no config cached yet")
		return
	}
	do := func(relation naming.Relation, name string, cache map[string]status.T) {
		relationS := relation.String()
		if objectPath, node, err := relation.Split(); err != nil {
			t.log.Warnf("init relation %s status cache: split %s: %s", name, relation, err)
		} else if node == "" {
			t.log.Infof("init relation subscribe to %s %s object avail status updates and deletes", name, objectPath)
			t.sub.AddFilter(&msgbus.ObjectStatusUpdated{}, pubsub.Label{"path", objectPath.String()})
			t.sub.AddFilter(&msgbus.ObjectStatusDeleted{}, pubsub.Label{"path", objectPath.String()})
			if st := object.StatusData.Get(objectPath); st != nil {
				t.log.Infof("init relation %s %s avail status init to %s", name, relation, st.Avail)
				cache[relationS] = st.Avail
			} else {
				t.log.Infof("init relation %s %s avail status init to %s", name, relation, status.Undef)
				cache[relationS] = status.Undef
			}
		} else {
			t.log.Infof("subscribe to %s %s@%s instance avail status updates and deletes", name, objectPath, node)
			t.sub.AddFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"path", objectPath.String()}, pubsub.Label{"node", node})
			t.sub.AddFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"path", objectPath.String()}, pubsub.Label{"node", node})
			if st := instance.StatusData.Get(objectPath, node); st != nil {
				t.log.Infof("init relation %s %s avail status init to %s", name, relation, st.Avail)
				cache[relationS] = st.Avail
			} else {
				t.log.Infof("init relation %s %s avail status init to %s", name, relation, status.Undef)
				cache[relationS] = status.Undef
			}
		}
	}
	for _, relation := range config.Children {
		do(relation, "Child", t.state.Children)
	}
	for _, relation := range config.Parents {
		do(relation, "Parent", t.state.Parents)
	}
}

func (t *Manager) onRelationObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	if c.Path == t.path {
		// Can't relate to self.
		return
	}
	changes := false
	do := func(relation string, name string, cache map[string]status.T) {
		if v, ok := cache[relation]; ok && v != status.Undef {
			t.log.Infof("update relation %s %s avail status change %s -> %s (deleted object)", name, relation, cache[relation], status.Undef)
			cache[relation] = status.Undef
			changes = true
		}
	}
	do(c.Path.String(), "Child", t.state.Children)
	do(c.Path.String(), "Parent", t.state.Parents)
	if changes {
		t.change = true
		t.onChange()
	}
}

func (t *Manager) onRelationInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	if c.Path == t.path {
		// Can't relate to self.
		return
	}
	changes := false
	do := func(relation string, name string, cache map[string]status.T) {
		if _, ok := cache[relation]; ok {
			t.log.Infof("update relation %s %s avail status change %s -> %s (deleted instance)", name, relation, cache[relation], status.Undef)
			cache[relation] = status.Undef
			changes = true
		}
	}
	do(c.Path.String()+"@"+c.Node, "Child", t.state.Children)
	do(c.Path.String()+"@"+c.Node, "Parent", t.state.Parents)

	if changes {
		t.change = true
		t.onChange()
	}
}

func (t *Manager) onRelationObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	if c.Path == t.path {
		// Can't relate to self. This case is handled by onInstanceStatusUpdated.
		return
	}
	relation := c.Path.String()
	changes := false
	do := func(relation string, name string, cache map[string]status.T) {
		if cache[relation] != c.Value.Avail {
			t.log.Infof("update relation %s %s avail status change %s -> %s", name, relation, cache[relation], c.Value.Avail)
			cache[relation] = c.Value.Avail
			changes = true
		} else {
			t.log.Debugf("update relation %s %s avail status unchanged", name, relation)
		}
	}
	if _, ok := t.state.Children[relation]; ok {
		do(relation, "Child", t.state.Children)
	}
	if _, ok := t.state.Parents[relation]; ok {
		do(relation, "Parent", t.state.Parents)
	}
	if changes {
		t.change = true
		t.onChange()
	}
}

func (t *Manager) onRelationInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	if c.Path == t.path {
		// Can't relate to self. This case is handled by onInstanceStatusUpdated.
		return
	}
	changes := false
	relation := c.Path.String() + "@" + c.Node
	do := func(relation string, name string, cache map[string]status.T) {
		if cache[relation] != c.Value.Avail {
			t.log.Infof("update relation %s %s avail status change %s -> %s", name, relation, cache[relation], c.Value.Avail)
		} else {
			t.log.Debugf("update relation %s %s avail status unchanged", name, relation)
		}
		cache[relation] = c.Value.Avail
		changes = true
	}
	if _, ok := t.state.Children[relation]; ok {
		do(relation, "Child", t.state.Children)
	}
	if _, ok := t.state.Parents[relation]; ok {
		do(relation, "Parent", t.state.Parents)
	}
	if changes {
		t.change = true
		t.onChange()
	}
}

func (t *Manager) onMyInstanceStatusUpdated(srcNode string, srcCmd *msgbus.InstanceStatusUpdated) {
	updateInstStatusMap := func() {
		instStatus, ok := t.instStatus[srcCmd.Node]
		switch {
		case !ok:
			t.log.Debugf("ObjectStatusUpdated %s from InstanceStatusUpdated on %s create instance status", srcNode, srcCmd.Node)
			t.instStatus[srcCmd.Node] = srcCmd.Value
		case instStatus.UpdatedAt.Before(srcCmd.Value.UpdatedAt):
			// only update if more recent
			t.log.Debugf("ObjectStatusUpdated %s from InstanceStatusUpdated on %s update instance status", srcNode, srcCmd.Node)
			t.instStatus[srcCmd.Node] = srcCmd.Value
		default:
			t.log.Debugf("ObjectStatusUpdated %s from InstanceStatusUpdated on %s skip update instance from obsolete status", srcNode, srcCmd.Node)
		}
	}
	setLocalExpectStarted := func() {
		if srcCmd.Node != t.localhost {
			return
		}
		if t.state.State != instance.MonitorStateIdle {
			// wait for idle state, we may be MonitorStateProvisioning, MonitorStateProvisioned ...
			return
		}
		if !srcCmd.Value.Avail.Is(status.Up) {
			return
		}
		if t.state.LocalExpect == instance.MonitorLocalExpectStarted {
			return
		}
		t.enableMonitor("this instance is now considered started")
	}

	updateInstStatusMap()
	setLocalExpectStarted()
}

func (t *Manager) onInstanceConfigUpdated(srcNode string, srcCmd *msgbus.InstanceConfigUpdated) {
	janitorInstStatus := func(scope []string) {
		cfgNodes := make(map[string]any)

		// init a instance.Status for new peers not yet in the instStatus map
		for _, node := range srcCmd.Value.Scope {
			cfgNodes[node] = nil
			if _, ok := t.instStatus[node]; !ok {
				t.instStatus[node] = instance.Status{Avail: status.Undef}
			}
		}

		// delete the instStatus key for peers gone out of scope
		for node := range t.instStatus {
			if _, ok := cfgNodes[node]; !ok {
				t.log.Debugf("drop instance status cache for node %s (node no longer in the object's expanded node list)", node)
				delete(t.instStatus, node)
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
				t.log.Warnf("janitor relations %s status cache: split %s: %s", name, relation, err)
				continue
			} else {
				t.log.Infof("janitor relations subscribe to %s %s avail status updates and deletes", name, relationS)
				if node == "" {
					t.sub.AddFilter(&msgbus.ObjectStatusUpdated{}, pubsub.Label{"path", objectPath.String()})
					t.sub.AddFilter(&msgbus.ObjectStatusDeleted{}, pubsub.Label{"path", objectPath.String()})
					if st := object.StatusData.Get(objectPath); st != nil {
						t.log.Infof("janitor relations %s %s avail status init to %s", name, relation, st.Avail)
						cache[relationS] = st.Avail
					} else {
						t.log.Infof("janitor relations %s %s avail status init to %s", name, relation, status.Undef)
						cache[relationS] = status.Undef
					}
					t.change = true
				} else {
					t.sub.AddFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"path", objectPath.String()}, pubsub.Label{"node", node})
					t.sub.AddFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"path", objectPath.String()}, pubsub.Label{"node", node})
					if st := instance.StatusData.Get(objectPath, node); st != nil {
						t.log.Infof("janitor relations %s %s avail status init to %s", name, relation, st.Avail)
						cache[relationS] = st.Avail
					} else {
						t.log.Infof("janitor relations %s %s avail status init to %s", name, relation, status.Undef)
						cache[relationS] = status.Undef
					}
					t.change = true
				}
			}
		}
		for relationS := range cache {
			if _, ok := m[relationS]; !ok {
				t.log.Infof("janitor relations unsubscribe from %s %s avail status updates and deletes", name, relationS)
				objectPath, node, _ := naming.Relation(relationS).Split()
				if node == "" {
					t.sub.DelFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"path", objectPath.String()})
					t.sub.DelFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"path", objectPath.String()})
				} else {
					t.sub.DelFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"path", objectPath.String()}, pubsub.Label{"node", node})
					t.sub.DelFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"path", objectPath.String()})
				}
			}
		}
	}

	if srcCmd.Node == t.localhost {
		defer func() {
			if err := t.queueStatus(); err != nil {
				t.log.Warnf("evaluate instance status via CRM: %s", err)
			}
		}()
		t.instConfig = srcCmd.Value
		t.log.Debugf("refresh resource monitor states on local instance config updated")
		t.initResourceMonitor()
		janitorInstStatus(srcCmd.Value.Scope)
		janitorRelations(srcCmd.Value.Children, "Child", t.state.Children)
		janitorRelations(srcCmd.Value.Parents, "Parent", t.state.Parents)
	}
	t.scopeNodes = append([]string{}, srcCmd.Value.Scope...)
	t.log.Debugf("updated from %s ObjectStatusUpdated InstanceConfigUpdated on %s scopeNodes=%s", srcNode, srcCmd.Node, t.scopeNodes)
}

func (t *Manager) onMyInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	if _, ok := t.instStatus[c.Node]; ok {
		t.log.Debugf("drop deleted instance status from node %s", c.Node)
		delete(t.instStatus, c.Node)
	}
}

func (t *Manager) onInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	if t.path != c.Path {
		t.onRelationInstanceStatusDeleted(c)
	}
}

func (t *Manager) onObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	if t.path == c.Path {
		t.onMyObjectStatusUpdated(c)
	} else {
		t.onRelationObjectStatusUpdated(c)
	}
}

func (t *Manager) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	if t.path != c.Path {
		t.onRelationObjectStatusDeleted(c)
	}
}

func (t *Manager) onMyObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	if c.SrcEv != nil {
		switch srcCmd := c.SrcEv.(type) {
		case *msgbus.InstanceStatusDeleted:
			t.onMyInstanceStatusDeleted(srcCmd)
		case *msgbus.InstanceStatusUpdated:
			t.onMyInstanceStatusUpdated(c.Node, srcCmd)
		case *msgbus.InstanceConfigUpdated:
			t.onInstanceConfigUpdated(c.Node, srcCmd)
		case *msgbus.InstanceConfigDeleted:
			// just a reminder here: no action on InstanceConfigDeleted because
			// if our instance config is deleted our omon launcher will cancel us
		case *msgbus.InstanceMonitorDeleted:
			if srcCmd.Node == t.localhost {
				// this is not expected
				t.log.Warnf("unexpected received ObjectStatusUpdated from self InstanceMonitorDeleted")
			} else {
				t.onInstanceMonitorDeletedFromNode(srcCmd.Node)
			}
		case *msgbus.InstanceMonitorUpdated:
			t.onInstanceMonitorUpdated(srcCmd)
		}
	}
	t.objStatus = c.Value
	t.onChange()
}

// onProgressInstanceMonitor updates the fields of instance.Monitor applying policies:
// if state goes from stopping/shutting to idle and local expect is started, reset the
// local expect, so the resource restart is disabled.
func (t *Manager) onProgressInstanceMonitor(c *msgbus.ProgressInstanceMonitor) {
	if t.state.State == c.State {
		return
	}

	// state change
	switch t.state.SessionID {
	case uuid.Nil:
	case c.SessionID:
		// pass
	default:
		t.log.Warnf("received progress instance monitor for wrong sid state %s(%s) -> %s(%s)", t.state.State, t.state.SessionID, c.State, c.SessionID)
	}
	t.log.Infof("set instance monitor state %s -> %s", t.state.State, c.State)
	t.change = true
	t.state.State = c.State
	if c.State == instance.MonitorStateIdle {
		t.state.SessionID = uuid.Nil
	} else {
		t.state.SessionID = c.SessionID
	}

	// local expect change ?
	switch c.State {
	case instance.MonitorStateStopping, instance.MonitorStateUnprovisioning, instance.MonitorStateShutting:

		switch t.state.LocalExpect {
		case instance.MonitorLocalExpectStarted:
			if c.IsPartial {
				t.disableMonitor("user is stopping some instance resources")
			} else {
				t.disableMonitor("user is stopping the instance")
			}
		}
	}

	t.onChange()
}

func (t *Manager) onSetInstanceMonitor(c *msgbus.SetInstanceMonitor) {
	doState := func() error {
		if c.Value.State == nil {
			return nil
		}
		if _, ok := instance.MonitorStateStrings[*c.Value.State]; !ok {
			err := fmt.Errorf("%w %s", instance.ErrInvalidState, *c.Value.State)
			t.log.Warnf("set instance monitor: %s", err)
			return err
		}
		if *c.Value.State == instance.MonitorStateInit {
			err := fmt.Errorf("%w %s", instance.ErrInvalidState, *c.Value.State)
			return err
		}
		if t.state.State == *c.Value.State {
			err := fmt.Errorf("%w %s", instance.ErrSameState, *c.Value.State)
			t.log.Infof("set instance monitor: %s", err)
			return err
		}
		t.log.Infof("set instance monitor state %s -> %s", t.state.State, *c.Value.State)
		t.change = true
		t.state.State = *c.Value.State
		return nil
	}

	globalExpectRefused := func() {
		t.pubsubBus.Pub(&msgbus.SetInstanceMonitorRefused{
			Path:  t.path,
			Node:  t.localhost,
			Value: c.Value,
		}, t.labelPath, t.labelLocalhost)
	}

	doGlobalExpect := func() error {
		if c.Value.GlobalExpect == nil {
			return nil
		}
		if _, ok := instance.MonitorGlobalExpectStrings[*c.Value.GlobalExpect]; !ok {
			err := fmt.Errorf("%w %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect)
			t.log.Warnf("set instance monitor: %s", err)
			globalExpectRefused()
			return err
		}
		if t.state.OrchestrationID != uuid.Nil && *c.Value.GlobalExpect != instance.MonitorGlobalExpectAborted {
			err := fmt.Errorf("%w: daemon: imon: %s: a %s orchestration is already in progress with id %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, t.state.GlobalExpect, t.state.OrchestrationID)
			return err
		}
		switch *c.Value.GlobalExpect {
		case instance.MonitorGlobalExpectPlacedAt:
			options, ok := c.Value.GlobalExpectOptions.(instance.MonitorGlobalExpectOptionsPlacedAt)
			if !ok || len(options.Destination) == 0 {
				// Switch cmd without explicit target nodes.
				// Select some nodes automatically.
				dst := t.nextPlacedAtCandidate()
				if dst == "" {
					err := fmt.Errorf("%w: daemon: imon: %s: no destination node could be selected from candidates", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect)
					t.log.Infof("set instance monitor: %s", err)
					globalExpectRefused()
					return err
				}
				options.Destination = []string{dst}
				c.Value.GlobalExpectOptions = options
			} else {
				want := options.Destination
				can, err := t.nextPlacedAtCandidates(want)
				if err != nil {
					err2 := fmt.Errorf("%w: daemon: imon: %s: no destination node could ne selected from %s: %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, want, err)
					t.log.Infof("set instance monitor: %s", err)
					globalExpectRefused()
					return err2
				}
				if can == "" {
					err := fmt.Errorf("%w: daemon: imon: %s: no destination node could ne selected from %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, want)
					t.log.Infof("set instance monitor: %s", err)
					globalExpectRefused()
					return err
				} else if can != want[0] {
					t.log.Infof("set instance monitor: change destination nodes from %s to %s", want, can)
				}
				options.Destination = []string{can}
				c.Value.GlobalExpectOptions = options
			}
		case instance.MonitorGlobalExpectStarted:
			if v, reason := t.isStartable(); !v {
				err := fmt.Errorf("%w: daemon: imon: %s: %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, reason)
				t.log.Infof("set instance monitor %s", t.path, err)
				globalExpectRefused()
				return err
			}
		}
		for node, instMon := range t.instMonitor {
			if instMon.GlobalExpect == *c.Value.GlobalExpect {
				continue
			}
			if instMon.GlobalExpect == instance.MonitorGlobalExpectInit {
				continue
			}
			if instMon.GlobalExpect == instance.MonitorGlobalExpectNone {
				continue
			}
			if instMon.GlobalExpectUpdatedAt.After(t.state.GlobalExpectUpdatedAt) {
				err := fmt.Errorf("%w: daemon: imon: %s: more recent value %s on node %s", instance.ErrInvalidGlobalExpect, *c.Value.GlobalExpect, instMon.GlobalExpect, node)
				t.log.Infof("set instance monitor: %s", t.path, err)
				globalExpectRefused()
				return err
			}
		}

		if *c.Value.GlobalExpect != t.state.GlobalExpect {
			t.change = true
			t.state.GlobalExpect = *c.Value.GlobalExpect
			t.state.GlobalExpectOptions = c.Value.GlobalExpectOptions
			// update GlobalExpectUpdated now
			// This will allow remote nodes to pickup most recent value
			t.state.GlobalExpectUpdatedAt = time.Now()

			// reset state to idle to allow the new orchestration to begin
			t.state.State = instance.MonitorStateIdle
			t.state.OrchestrationIsDone = false
		}
		return nil
	}

	doLocalExpect := func() error {
		if c.Value.LocalExpect == nil {
			return nil
		}
		switch *c.Value.LocalExpect {
		case instance.MonitorLocalExpectNone:
		case instance.MonitorLocalExpectEvicted:
		case instance.MonitorLocalExpectStarted:
		case instance.MonitorLocalExpectShutdown:
		default:
			err := fmt.Errorf("%w %s", instance.ErrInvalidLocalExpect, *c.Value.LocalExpect)
			t.log.Warnf("set instance monitor: %s", err)
			return err
		}
		target := *c.Value.LocalExpect
		if t.state.LocalExpect == target {
			err := fmt.Errorf("%w %s", instance.ErrSameLocalExpect, *c.Value.LocalExpect)
			t.log.Infof("set instance monitor: %s", err)
			return err
		}
		t.setLocalExpect(target, "set instance monitor: update local expect to %s", target)
		return nil
	}

	err := errors.Join(doState(), doGlobalExpect(), doLocalExpect())

	if v, ok := c.Err.(errcontext.ErrCloseSender); ok {
		v.Send(err)
		v.Close()
	}

	if t.change {
		if t.state.OrchestrationID.String() != c.Value.CandidateOrchestrationID.String() {
			t.log = t.newLogger(c.Value.CandidateOrchestrationID)
		}
		t.state.OrchestrationID = c.Value.CandidateOrchestrationID
		t.acceptedOrchestrationID = c.Value.CandidateOrchestrationID
		t.onChange()
	} else {
		t.pubsubBus.Pub(&msgbus.ObjectOrchestrationRefused{
			Node:                t.localhost,
			Path:                t.path,
			ID:                  c.Value.CandidateOrchestrationID.String(),
			Reason:              fmt.Sprintf("set instance monitor request => no changes: %v", c.Value),
			GlobalExpect:        c.Value.GlobalExpect,
			GlobalExpectOptions: c.Value.GlobalExpectOptions,
		},
			t.labelPath,
			t.labelLocalhost,
		)
	}
}

func (t *Manager) onNodeConfigUpdated(c *msgbus.NodeConfigUpdated) {
	t.readyDuration = c.Value.ReadyPeriod
	t.orchestrate()
	t.updateIfChange()
}

func (t *Manager) onNodeMonitorUpdated(c *msgbus.NodeMonitorUpdated) {
	t.nodeMonitor[c.Node] = c.Value
	t.onChange()
}

func (t *Manager) onNodeStatusUpdated(c *msgbus.NodeStatusUpdated) {
	t.nodeStatus[c.Node] = c.Value
	t.onChange()
}

func (t *Manager) onNodeStatsUpdated(c *msgbus.NodeStatsUpdated) {
	t.nodeStats[c.Node] = c.Value
	if t.objStatus.PlacementPolicy == placement.Score {
		t.onChange()
	}
}

func (t *Manager) onInstanceMonitorUpdated(c *msgbus.InstanceMonitorUpdated) {
	// ignore self msgbus.InstanceMonitorUpdated
	if c.Node != t.localhost {
		t.onRemoteInstanceMonitorUpdated(c)
	}
}

func (t *Manager) onRemoteInstanceMonitorUpdated(c *msgbus.InstanceMonitorUpdated) {
	remote := c.Node
	instMon := c.Value
	t.log.Debugf("updated instance imon from peer node %s -> global expect:%s, state: %s", remote, instMon.GlobalExpect, instMon.State)
	t.instMonitor[remote] = instMon
	t.convergeGlobalExpectFromRemote()
	t.updateOrchestrateUpdate()
}

func (t *Manager) onInstanceMonitorDeletedFromNode(node string) {
	if node == t.localhost {
		// this is not expected
		t.log.Warnf("onInstanceMonitorDeletedFromNode should never be called from localhost")
		return
	}
	t.log.Debugf("delete remote instance imon from node %s", node)
	delete(t.instMonitor, node)
	t.convergeGlobalExpectFromRemote()
	t.updateOrchestrateUpdate()
}

func (t *Manager) GetInstanceMonitor(node string) (instance.Monitor, bool) {
	if t.localhost == node {
		return t.state, true
	}
	m, ok := t.instMonitor[node]
	return m, ok
}

func (t *Manager) AllInstanceMonitorState(s instance.MonitorState) bool {
	for _, instMon := range t.AllInstanceMonitors() {
		if instMon.State != s {
			return false
		}
	}
	return true
}

func (t *Manager) AllInstanceMonitors() map[string]instance.Monitor {
	m := make(map[string]instance.Monitor)
	m[t.localhost] = t.state
	for node, instMon := range t.instMonitor {
		if node == t.localhost {
			err := fmt.Errorf("Func AllInstanceMonitors is not expected to have localhost in o.instMonitor keys")
			t.log.Errorf("%s", err)
			panic(err)
		}
		m[node] = instMon
	}
	return m
}

func (t *Manager) isHAOrchestrateable() (bool, string) {
	if (t.objStatus.Topology == topology.Failover) && (t.objStatus.Avail == status.Warn) {
		return false, "failover object is warn state"
	}
	switch t.objStatus.Provisioned {
	case provisioned.Mixed:
		return false, "mixed object provisioned state"
	case provisioned.False:
		return false, "false object provisioned state"
	}
	return true, ""
}

func (t *Manager) isStartable() (bool, string) {
	if v, reason := t.isHAOrchestrateable(); !v {
		return false, reason
	}
	if t.isStarted() {
		return false, "already started"
	}
	return true, "object is startable"
}

func (t *Manager) isStarted() bool {
	switch t.objStatus.Topology {
	case topology.Flex:
		return t.objStatus.UpInstancesCount >= t.objStatus.FlexTarget
	case topology.Failover:
		return t.objStatus.Avail == status.Up
	default:
		return false
	}
}

func (t *Manager) needOrchestrate(c cmdOrchestrate) {
	if c.state == instance.MonitorStateInit {
		return
	}
	select {
	case <-t.ctx.Done():
		return
	default:
	}
	if t.state.State == c.state {
		t.change = true
		t.state.State = c.newState
		t.updateIfChange()
	}
	select {
	case <-t.ctx.Done():
		return
	default:
	}
	t.orchestrate()
}

func (t *Manager) sortCandidates(candidates []string) []string {
	switch t.objStatus.PlacementPolicy {
	case placement.NodesOrder:
		return t.sortWithNodesOrderPolicy(candidates)
	case placement.Spread:
		return t.sortWithSpreadPolicy(candidates)
	case placement.Score:
		return t.sortWithScorePolicy(candidates)
	case placement.Shift:
		return t.sortWithShiftPolicy(candidates)
	case placement.LastStart:
		return t.sortWithLastStartPolicy(candidates)
	default:
		return []string{}
	}
}

func (t *Manager) sortWithSpreadPolicy(candidates []string) []string {
	l := append([]string{}, candidates...)
	sum := func(s string) []byte {
		b := append([]byte(t.path.String()), []byte(s)...)
		return md5.New().Sum(b)
	}
	sort.SliceStable(l, func(i, j int) bool {
		return bytes.Compare(sum(l[i]), sum(l[j])) < 0
	})
	return l
}

// sortWithScorePolicy sorts candidates by descending cluster.NodeStats.Score
func (t *Manager) sortWithScorePolicy(candidates []string) []string {
	l := append([]string{}, candidates...)
	sort.SliceStable(l, func(i, j int) bool {
		var si, sj uint64
		if stats, ok := t.nodeStats[l[i]]; ok {
			si = stats.Score
		}
		if stats, ok := t.nodeStats[l[j]]; ok {
			sj = stats.Score
		}
		return si > sj
	})
	return l
}

func (t *Manager) sortWithLoadAvgPolicy(candidates []string) []string {
	l := append([]string{}, candidates...)
	sort.SliceStable(l, func(i, j int) bool {
		var si, sj float64
		if stats, ok := t.nodeStats[l[i]]; ok {
			si = stats.Load15M
		}
		if stats, ok := t.nodeStats[l[j]]; ok {
			sj = stats.Load15M
		}
		return si > sj
	})
	return l
}

func (t *Manager) sortWithLastStartPolicy(candidates []string) []string {
	l := append([]string{}, candidates...)
	sort.SliceStable(l, func(i, j int) bool {
		var si, sj time.Time
		if instStatus, ok := t.instStatus[l[i]]; ok {
			si = instStatus.LastStartedAt
		}
		if instStatus, ok := t.instStatus[l[j]]; ok {
			sj = instStatus.LastStartedAt
		}
		return si.After(sj)
	})
	return l
}

func (t *Manager) sortWithShiftPolicy(candidates []string) []string {
	var i int
	l := t.sortWithNodesOrderPolicy(candidates)
	l = append(l, l...)
	n := len(candidates)
	scalerSliceIndex := t.path.ScalerSliceIndex()
	if n > 0 && scalerSliceIndex > n {
		i = t.path.ScalerSliceIndex() % n
	}
	return candidates[i : i+n]
}

func (t *Manager) sortWithNodesOrderPolicy(candidates []string) []string {
	var l []string
	for _, node := range t.scopeNodes {
		if slices.Contains(candidates, node) {
			l = append(l, node)
		}
	}
	return l
}

func (t *Manager) nextPlacedAtCandidates(want []string) (string, error) {
	expr := strings.Join(want, " ")
	var wantNodes []string
	nodes, err := nodeselector.Expand(expr)
	if err != nil {
		return "", err
	}
	for _, node := range nodes {
		if _, ok := t.instStatus[node]; !ok {
			continue
		}
		wantNodes = append(wantNodes, node)
	}
	return strings.Join(wantNodes, ","), nil
}

func (t *Manager) nextPlacedAtCandidate() string {
	if t.objStatus.Topology == topology.Flex {
		return ""
	}
	var candidates []string
	candidates = append(candidates, t.scopeNodes...)
	candidates = t.sortCandidates(candidates)

	for _, candidate := range candidates {
		if instStatus, ok := t.instStatus[candidate]; ok {
			switch instStatus.Avail {
			case status.Down, status.StandbyDown, status.StandbyUp:
				return candidate
			}
		}
	}
	return ""
}

func (t *Manager) IsInstanceStatusNotApplicable(node string) (bool, bool) {
	instStatus, ok := t.instStatus[node]
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

func (t *Manager) IsInstanceStartFailed(node string) (bool, bool) {
	instMon, ok := t.GetInstanceMonitor(node)
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

func (t *Manager) IsNodeMonitorStatusRankable(node string) (bool, bool) {
	nodeMonitor, ok := t.nodeMonitor[node]
	if !ok {
		return false, false
	}
	return nodeMonitor.State.IsRankable(), true
}

func (t *Manager) newIsHALeader() bool {
	var candidates []string

	for _, node := range t.scopeNodes {
		if v, ok := t.IsInstanceStatusNotApplicable(node); !ok || v {
			continue
		}
		if nodeStatus, ok := t.nodeStatus[node]; !ok || nodeStatus.IsFrozen() {
			continue
		}
		if instStatus, ok := t.instStatus[node]; !ok || instStatus.IsFrozen() {
			continue
		}
		if instStatus, ok := t.instStatus[node]; !ok || instStatus.Provisioned.IsOneOf(provisioned.Mixed, provisioned.False) {
			continue
		}
		if failed, ok := t.IsInstanceStartFailed(node); !ok || failed {
			continue
		}
		if v, ok := t.IsNodeMonitorStatusRankable(node); !ok || !v {
			continue
		}
		candidates = append(candidates, node)
	}
	candidates = t.sortCandidates(candidates)

	var maxLeaders int = 1
	if t.objStatus.Topology == topology.Flex {
		maxLeaders = t.objStatus.FlexTarget
	}

	i := stringslice.Index(t.localhost, candidates)
	if i < 0 {
		return false
	}
	return i < maxLeaders
}

func (t *Manager) newIsLeader() bool {
	var candidates []string
	for _, node := range t.scopeNodes {
		if v, ok := t.IsInstanceStatusNotApplicable(node); !ok || v {
			continue
		}
		if failed, ok := t.IsInstanceStartFailed(node); !ok || failed {
			continue
		}
		candidates = append(candidates, node)
	}
	candidates = t.sortCandidates(candidates)

	var maxLeaders int = 1
	if t.objStatus.Topology == topology.Flex {
		maxLeaders = t.objStatus.FlexTarget
	}

	i := stringslice.Index(t.localhost, candidates)
	if i < 0 {
		return false
	}
	return i < maxLeaders
}

func (t *Manager) updateIsLeader() {
	isLeader := t.newIsLeader()
	if isLeader != t.state.IsLeader {
		t.change = true
		t.state.IsLeader = isLeader
	}
	isHALeader := t.newIsHALeader()
	if isHALeader != t.state.IsHALeader {
		t.change = true
		t.state.IsHALeader = isHALeader
	}
	return
}

// doTransitionAction execute action and update transition states
func (t *Manager) doTransitionAction(action func() error, newState, successState, errorState instance.MonitorState) {
	t.transitionTo(newState)
	if action() != nil {
		t.transitionTo(errorState)
	} else {
		t.transitionTo(successState)
	}
}

func (t *Manager) queueLastAction(action func() error, newState, successState, errorState instance.MonitorState) {
	_ = runner.Run(t.instConfig.Priority, func() error {
		t.doLastAction(action, newState, successState, errorState)
		return nil
	})
}

func (t *Manager) queueAction(action func() error, newState, successState, errorState instance.MonitorState) {
	_ = runner.Run(t.instConfig.Priority, func() error {
		t.doAction(action, newState, successState, errorState)
		return nil
	})
}

// doAction runs action + background orchestration from action state result
//
// 1- set transient state to newState
// 2- run action
// 3- go orchestrateAfterAction(newState, successState or errorState)
func (t *Manager) doAction(action func() error, newState, successState, errorState instance.MonitorState) {
	t.transitionTo(newState)
	nextState := successState
	if action() != nil {
		nextState = errorState
	}
	go t.orchestrateAfterAction(newState, nextState)
}

func (t *Manager) doLastAction(action func() error, newState, successState, errorState instance.MonitorState) {
	t.transitionTo(newState)
	nextState := successState
	if action() != nil {
		nextState = errorState
	}
	t.done()
	go t.orchestrateAfterAction(newState, nextState)
}

func (t *Manager) initResourceMonitor() {
	// Stop any pending restart timers before init. We may be called after
	// instance config refreshed with some previous resource restart scheduled.
	t.resetResourceMonitorTimers()

	logDropped := func(l map[string]bool, comment string) {
		if l != nil {
			dropped := make([]string, 0)
			for rid := range l {
				dropped = append(dropped, rid)
			}
			if len(dropped) > 0 {
				t.log.Infof("instance config has been updated: drop previously scheduled restart %s %v", comment, dropped)
			}
		}
	}
	logDropped(t.resourceWithRestartScheduled, "resources")
	logDropped(t.resourceStandbyWithRestartScheduled, "standby resources")

	t.resourceWithRestartScheduled = make(map[string]bool)
	t.resourceStandbyWithRestartScheduled = make(map[string]bool)

	if monitorAction, ok := t.getValidMonitorAction(0); !ok {
		t.initialMonitorAction = instance.MonitorActionNone
	} else {
		t.initialMonitorAction = monitorAction
	}

	hasMonitorActionNone := t.initialMonitorAction == instance.MonitorActionNone

	m := make(instance.ResourceMonitors, 0)
	for rid, rcfg := range t.instConfig.Resources {
		m[rid] = instance.ResourceMonitor{
			Restart: instance.ResourceMonitorRestart{
				Remaining: rcfg.Restart,
			},
		}
		if rcfg.IsMonitored && hasMonitorActionNone {
			t.log.Infof("unusable monitor action: resource %s is monitored, but monitor action is none", rid)
		}
	}
	t.state.Resources = m

	t.change = true
}

func (t *Manager) onNodeRejoin(c *msgbus.NodeRejoin) {
	if c.IsUpgrading {
		return
	}
	if len(t.instStatus) < 2 {
		// no need to merge frozen if the object has a single instance
		return
	}
	instStatus, ok := t.instStatus[t.localhost]
	if !ok {
		return
	}
	if !instStatus.FrozenAt.IsZero() {
		// already frozen
		return
	}
	if t.state.GlobalExpect == instance.MonitorGlobalExpectThawed {
		return
	}
	if t.instConfig.Orchestrate != "ha" {
		return
	}
	for peer, peerStatus := range t.instStatus {
		if peer == t.localhost {
			continue
		}
		if peerStatus.FrozenAt.After(c.LastShutdownAt) {
			msg := fmt.Sprintf("Freeze %s instance because peer %s instance was frozen while this daemon was down", t.path, peer)
			if err := t.queueFreeze(); err != nil {
				t.log.Infof("%s: %s", msg, err)
			} else {
				t.log.Infof(msg)
			}
			return
		}

	}
}
