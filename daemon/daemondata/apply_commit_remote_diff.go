package daemondata

import (
	"reflect"
	"time"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/xmap"
)

var (
	labelPeerNode = pubsub.Label{"peer", "true"}
)

func (d *data) getPeersFromPrevAndPending() []string {
	nodes := make(map[string]any)
	for n := range d.pending.Cluster.Node {
		if n == d.localNode {
			continue
		}
		nodes[n] = nil
	}
	for n := range d.previousRemoteInfo {
		if n == d.localNode {
			continue
		}
		nodes[n] = nil
	}
	return xmap.Keys(nodes)
}

// pubPeerDataChanges propagate peers data changes (node status, node monitor,
// node config, node instances) since last call has new publications.
func (d *data) pubPeerDataChanges() {
	for _, nodename := range d.getPeersFromPrevAndPending() {
		current := d.refreshPreviousUpdated(nodename)
		if current == nil {
			continue
		}
		d.pubMsgFromNodeConfigDiffForNode(nodename)
		d.pubMsgFromNodeStatusDiffForNode(nodename)
		d.pubMsgFromNodeStatsDiffForNode(nodename)
		d.pubMsgFromNodeMonitorDiffForNode(nodename, current)
		d.pubMsgFromNodeInstanceDiffForNode(nodename, current)
		d.previousRemoteInfo[nodename] = *current
	}
}

func (d *data) pubMsgFromNodeConfigDiffForNode(nodename string) {
	var (
		prevTime         remoteInfo
		nextNode         node.Node
		next, prev       node.Config
		hasNext, hasPrev bool
	)
	if nextNode, hasNext = d.pending.Cluster.Node[nodename]; hasNext {
		next = nextNode.Config
	}
	prevTime, hasPrev = d.previousRemoteInfo[nodename]
	prev = prevTime.nodeConfig
	onUpdate := func() {
		if !reflect.DeepEqual(prev, next) {
			d.bus.Pub(msgbus.NodeConfigUpdated{Node: nodename, Value: *next.DeepCopy()},
				pubsub.Label{"node", nodename},
			)
		}
	}
	onCreate := func() {
		d.bus.Pub(msgbus.NodeConfigUpdated{Node: nodename, Value: *next.DeepCopy()},
			pubsub.Label{"node", nodename},
		)
	}

	switch {
	case hasNext && hasPrev:
		onUpdate()
	case hasNext:
		onCreate()
	}
}

func (d *data) pubMsgFromNodeStatsDiffForNode(nodename string) {
	var (
		prevTime         remoteInfo
		nextNode         node.Node
		next, prev       node.Stats
		hasNext, hasPrev bool
	)
	if nextNode, hasNext = d.pending.Cluster.Node[nodename]; hasNext {
		next = nextNode.Stats
	}
	prevTime, hasPrev = d.previousRemoteInfo[nodename]
	prev = prevTime.nodeStats
	onUpdate := func() {
		if !reflect.DeepEqual(prev, next) {
			d.bus.Pub(msgbus.NodeStatsUpdated{Node: nodename, Value: *next.DeepCopy()},
				pubsub.Label{"node", nodename},
				labelPeerNode,
			)
		}
	}
	onCreate := func() {
		d.bus.Pub(msgbus.NodeStatsUpdated{Node: nodename, Value: *next.DeepCopy()},
			pubsub.Label{"node", nodename},
			labelPeerNode,
		)
	}

	switch {
	case hasNext && hasPrev:
		onUpdate()
	case hasNext:
		onCreate()
	}
}

func (d *data) pubMsgFromNodeStatusDiffForNode(nodename string) {
	var (
		prevTime         remoteInfo
		nextNode         node.Node
		next, prev       node.Status
		hasNext, hasPrev bool
	)
	if nextNode, hasNext = d.pending.Cluster.Node[nodename]; hasNext {
		next = nextNode.Status
	}
	prevTime, hasPrev = d.previousRemoteInfo[nodename]
	prev = prevTime.nodeStatus
	labels := []pubsub.Label{
		{"node", nodename},
		labelPeerNode,
	}
	onUpdate := func() {
		var changed bool
		if !reflect.DeepEqual(prev.Labels, next.Labels) {
			d.bus.Pub(msgbus.NodeStatusLabelsUpdated{Node: nodename, Value: next.Labels.DeepCopy()}, labels...)
			changed = true
		}
		if changed || !reflect.DeepEqual(prev, next) {
			d.bus.Pub(msgbus.NodeStatusUpdated{Node: nodename, Value: *next.DeepCopy()}, labels...)
		}
	}
	onCreate := func() {
		d.bus.Pub(msgbus.NodeStatusLabelsUpdated{Node: nodename, Value: next.Labels.DeepCopy()}, labels...)
		d.bus.Pub(msgbus.NodeStatusUpdated{Node: nodename, Value: *next.DeepCopy()}, labels...)
	}

	switch {
	case hasNext && hasPrev:
		onUpdate()
	case hasNext:
		onCreate()
	}
}

func (d *data) pubMsgFromNodeMonitorDiffForNode(nodename string, current *remoteInfo) {
	if current == nil {
		return
	}
	prevTimes, hasPrev := d.previousRemoteInfo[nodename]
	if !hasPrev || current.nmonUpdated.After(prevTimes.nmonUpdated) {
		localMonitor := d.pending.Cluster.Node[nodename].Monitor
		d.bus.Pub(msgbus.NodeMonitorUpdated{Node: nodename, Value: *localMonitor.DeepCopy()},
			pubsub.Label{"node", nodename},
			labelPeerNode,
		)
		return
	}
}

func (d *data) refreshPreviousUpdated(nodename string) *remoteInfo {
	if prev, ok := d.previousRemoteInfo[nodename]; ok {
		if prev.gen == d.pending.Cluster.Node[nodename].Status.Gen[nodename] {
			return nil
		}
	}
	c := d.pending.Cluster.Node[nodename]
	result := remoteInfo{
		nodeStatus:        *c.Status.DeepCopy(),
		nodeStats:         *c.Stats.DeepCopy(),
		imonUpdated:       make(map[string]time.Time),
		instConfigUpdated: make(map[string]time.Time),
		instStatusUpdated: make(map[string]time.Time),
	}

	nmonUpdated := c.Monitor.StateUpdated
	if c.Monitor.GlobalExpectUpdated.After(nmonUpdated) {
		nmonUpdated = c.Monitor.GlobalExpectUpdated
	}
	result.nmonUpdated = nmonUpdated

	for p, inst := range c.Instance {
		if inst.Status != nil {
			instUpdated := inst.Status.Updated
			if inst.Status.Frozen.After(instUpdated) {
				instUpdated = inst.Status.Frozen
			}
			result.instStatusUpdated[p] = instUpdated
		}
		if inst.Config != nil {
			result.instConfigUpdated[p] = inst.Config.Updated
		}
		if inst.Monitor != nil {
			imonUpdated := inst.Monitor.StateUpdated
			if inst.Monitor.UpdatedAt.After(imonUpdated) {
				imonUpdated = inst.Monitor.UpdatedAt
			}
			result.imonUpdated[p] = imonUpdated
		}
	}
	result.gen = c.Status.Gen[nodename]

	return &result
}

func getUpdatedRemoved(toPath map[string]path.T, previous, current map[string]time.Time) (updates, removes []string) {
	for s, updated := range current {
		if _, ok := toPath[s]; !ok {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			toPath[s] = p
		}
		if previousUpdated, ok := previous[s]; !ok {
			// new object
			updates = append(updates, s)
		} else if !updated.Equal(previousUpdated) {
			// update object
			updates = append(updates, s)
		}
	}
	for s := range previous {
		if _, ok := toPath[s]; !ok {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			toPath[s] = p
		}
		if _, ok := current[s]; !ok {
			removes = append(removes, s)
		}
	}
	return
}

func (d *data) pubMsgFromNodeInstanceDiffForNode(nodename string, current *remoteInfo) {
	var updates, removes []string
	toPath := make(map[string]path.T)
	previous, ok := d.previousRemoteInfo[nodename]
	if !ok {
		previous = remoteInfo{
			imonUpdated:       make(map[string]time.Time),
			instConfigUpdated: make(map[string]time.Time),
			instStatusUpdated: make(map[string]time.Time),
		}
	}
	updates, removes = getUpdatedRemoved(toPath, previous.instConfigUpdated, current.instConfigUpdated)
	for _, s := range updates {
		d.bus.Pub(msgbus.InstanceConfigUpdated{Path: toPath[s], Node: nodename, Value: *d.pending.Cluster.Node[nodename].Instance[s].Config.DeepCopy()},
			pubsub.Label{"path", s},
			pubsub.Label{"node", nodename},
			labelPeerNode,
		)
	}
	for _, s := range removes {
		d.bus.Pub(msgbus.InstanceConfigDeleted{Path: toPath[s], Node: nodename},
			pubsub.Label{"path", s},
			pubsub.Label{"node", nodename},
			labelPeerNode,
		)
	}

	updates, removes = getUpdatedRemoved(toPath, previous.instStatusUpdated, current.instStatusUpdated)
	for _, s := range updates {
		d.bus.Pub(msgbus.InstanceStatusUpdated{Path: toPath[s], Node: nodename, Value: *d.pending.Cluster.Node[nodename].Instance[s].Status.DeepCopy()},
			pubsub.Label{"path", s},
			pubsub.Label{"node", nodename},
			labelPeerNode,
		)
	}
	for _, s := range removes {
		d.bus.Pub(msgbus.InstanceStatusDeleted{Path: toPath[s], Node: nodename},
			pubsub.Label{"path", s},
			pubsub.Label{"node", nodename},
			labelPeerNode,
		)
	}

	updates, removes = getUpdatedRemoved(toPath, previous.imonUpdated, current.imonUpdated)
	for _, s := range updates {
		d.bus.Pub(msgbus.InstanceMonitorUpdated{Path: toPath[s], Node: nodename, Value: *d.pending.Cluster.Node[nodename].Instance[s].Monitor.DeepCopy()},
			pubsub.Label{"path", s},
			pubsub.Label{"node", nodename},
			labelPeerNode,
		)
	}
	for _, s := range removes {
		d.bus.Pub(msgbus.InstanceMonitorDeleted{Path: toPath[s], Node: nodename},
			pubsub.Label{"path", s},
			pubsub.Label{"node", nodename},
			labelPeerNode,
		)
	}

	for s, updated := range current.instConfigUpdated {
		var update bool
		if previousUpdated, ok := previous.instConfigUpdated[s]; !ok {
			// new cfg object
			update = true
		} else if !updated.Equal(previousUpdated) {
			// update cfg object
			update = true
		}
		if update {

		}
	}
	for s := range previous.instConfigUpdated {
		if _, ok := current.instConfigUpdated[s]; !ok {
			// removal cfg
		}
	}
}
