package daemondata

import (
	"reflect"
	"time"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (d *data) applyNodeData(msg *hbtype.Msg) error {
	remote := msg.Nodename
	peerLabel := pubsub.Label{"node", remote}
	local := d.localNode
	d.log.Debugf("applyNodeData %s", remote)

	d.clusterData.Cluster.Node[remote] = msg.NodeData
	d.hbGens[local][remote] = msg.NodeData.Status.Gen[remote]
	d.clusterData.Cluster.Node[local].Status.Gen[remote] = msg.NodeData.Status.Gen[remote]

	d.publisher.Pub(&msgbus.NodeDataUpdated{Node: remote, Value: msg.NodeData}, peerLabel, labelFromPeer)

	d.pubPeerDataChanges(remote)
	return nil
}

func (d *data) refreshPreviousUpdated(peer string) *remoteInfo {
	c := d.clusterData.Cluster.Node[peer]
	result := remoteInfo{
		nodeStatus:        *c.Status.DeepCopy(),
		nodeStats:         *c.Stats.DeepCopy(),
		nodeConfig:        *c.Config.DeepCopy(),
		imonUpdated:       make(map[string]time.Time),
		instConfigUpdated: make(map[string]time.Time),
		instStatusUpdated: make(map[string]time.Time),
	}

	result.nmonUpdated = c.Monitor.UpdatedAt

	result.collectorUpdated = c.Daemon.Collector.UpdatedAt
	result.daemondataUpdated = c.Daemon.Daemondata.UpdatedAt
	result.dnsUpdated = c.Daemon.Dns.UpdatedAt
	result.listenerUpdated = c.Daemon.Listener.UpdatedAt
	result.runnerImon = c.Daemon.RunnerImon.UpdatedAt
	result.scheduler = c.Daemon.Scheduler.UpdatedAt

	for p, inst := range c.Instance {
		if inst.Status != nil {
			instUpdated := inst.Status.UpdatedAt
			if inst.Status.FrozenAt.After(instUpdated) {
				instUpdated = inst.Status.FrozenAt
			}
			result.instStatusUpdated[p] = instUpdated
		}
		if inst.Config != nil {
			result.instConfigUpdated[p] = inst.Config.UpdatedAt
		}
		if inst.Monitor != nil {
			imonUpdated := inst.Monitor.StateUpdatedAt
			if inst.Monitor.UpdatedAt.After(imonUpdated) {
				imonUpdated = inst.Monitor.UpdatedAt
			}
			result.imonUpdated[p] = imonUpdated
		}
	}

	return &result
}

// pubPeerDataChanges propagate peers data changes (node status, node monitor,
// node config, node instances) since last call has new publications.
func (d *data) pubPeerDataChanges(peer string) {
	current := d.refreshPreviousUpdated(peer)
	if current == nil {
		return
	}
	d.pubMsgFromNodeConfigDiffForNode(peer)
	d.pubMsgFromNodeStatusDiffForNode(peer)
	d.pubMsgFromNodeStatsDiffForNode(peer)
	d.pubMsgFromNodeCollectorDiffForNode(peer, current)
	d.pubMsgFromNodeDaemondataDiffForNode(peer, current)
	d.pubMsgFromNodeDnsDiffForNode(peer, current)
	d.pubMsgFromNodeListenerDiffForNode(peer, current)
	d.pubMsgFromNodeRunnerImonDiffForNode(peer, current)
	d.pubMsgFromNodeSchedulerDiffForNode(peer, current)
	d.pubMsgFromNodeMonitorDiffForNode(peer, current)
	d.pubMsgFromNodeInstanceDiffForNode(peer, current)
	d.previousRemoteInfo[peer] = *current
}

func (d *data) pubMsgFromNodeConfigDiffForNode(peer string) {
	var (
		prevTime         remoteInfo
		nextNode         node.Node
		next, prev       node.Config
		hasNext, hasPrev bool
	)
	if nextNode, hasNext = d.clusterData.Cluster.Node[peer]; hasNext {
		next = nextNode.Config
	}
	prevTime, hasPrev = d.previousRemoteInfo[peer]
	prev = prevTime.nodeConfig
	onUpdate := func() {
		if !reflect.DeepEqual(prev, next) {
			node.ConfigData.Set(peer, next.DeepCopy())
			d.publisher.Pub(&msgbus.NodeConfigUpdated{Node: peer, Value: *next.DeepCopy()},
				pubsub.Label{"node", peer},
			)
		}
	}
	onCreate := func() {
		node.ConfigData.Set(peer, next.DeepCopy())
		d.publisher.Pub(&msgbus.NodeConfigUpdated{Node: peer, Value: *next.DeepCopy()},
			pubsub.Label{"node", peer},
		)
	}

	switch {
	case hasNext && hasPrev:
		onUpdate()
	case hasNext:
		onCreate()
	}
}

func (d *data) pubMsgFromNodeStatsDiffForNode(peer string) {
	var (
		prevTime         remoteInfo
		nextNode         node.Node
		next, prev       node.Stats
		hasNext, hasPrev bool
	)
	if nextNode, hasNext = d.clusterData.Cluster.Node[peer]; hasNext {
		next = nextNode.Stats
	}
	prevTime, hasPrev = d.previousRemoteInfo[peer]
	prev = prevTime.nodeStats
	onUpdate := func() {
		if !reflect.DeepEqual(prev, next) {
			node.StatsData.Set(peer, next.DeepCopy())
			d.publisher.Pub(&msgbus.NodeStatsUpdated{Node: peer, Value: *next.DeepCopy()},
				pubsub.Label{"node", peer},
				labelFromPeer,
			)
		}
	}
	onCreate := func() {
		node.StatsData.Set(peer, next.DeepCopy())
		d.publisher.Pub(&msgbus.NodeStatsUpdated{Node: peer, Value: *next.DeepCopy()},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
	}

	switch {
	case hasNext && hasPrev:
		onUpdate()
	case hasNext:
		onCreate()
	}
}

func (d *data) pubMsgFromNodeStatusDiffForNode(peer string) {
	var (
		prevTime         remoteInfo
		nextNode         node.Node
		next, prev       node.Status
		hasNext, hasPrev bool
	)
	if nextNode, hasNext = d.clusterData.Cluster.Node[peer]; hasNext {
		next = nextNode.Status
	}
	prevTime, hasPrev = d.previousRemoteInfo[peer]
	prev = prevTime.nodeStatus
	labels := []pubsub.Label{
		{"node", peer},
		labelFromPeer,
	}
	onUpdate := func() {
		var changed bool
		if !reflect.DeepEqual(prev.Labels, next.Labels) {
			d.publisher.Pub(&msgbus.NodeStatusLabelsUpdated{Node: peer, Value: next.Labels.DeepCopy()}, labels...)
			changed = true
		}
		if changed || !reflect.DeepEqual(prev, next) {
			node.StatusData.Set(peer, next.DeepCopy())
			d.publisher.Pub(&msgbus.NodeStatusUpdated{Node: peer, Value: *next.DeepCopy()}, labels...)
		}
	}
	onCreate := func() {
		d.publisher.Pub(&msgbus.NodeStatusLabelsUpdated{Node: peer, Value: next.Labels.DeepCopy()}, labels...)
		node.StatusData.Set(peer, next.DeepCopy())
		d.publisher.Pub(&msgbus.NodeStatusUpdated{Node: peer, Value: *next.DeepCopy()}, labels...)
	}

	switch {
	case hasNext && hasPrev:
		onUpdate()
	case hasNext:
		onCreate()
	}
}

func (d *data) pubMsgFromNodeCollectorDiffForNode(peer string, current *remoteInfo) {
	if current == nil {
		return
	}
	prevTimes, hasPrev := d.previousRemoteInfo[peer]
	if !hasPrev || current.collectorUpdated.After(prevTimes.collectorUpdated) {
		dCollector := d.clusterData.Cluster.Node[peer].Daemon.Collector
		daemonsubsystem.DataCollector.Set(peer, dCollector.DeepCopy())
		d.publisher.Pub(&msgbus.DaemonCollectorUpdated{Node: peer, Value: *dCollector.DeepCopy()},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
		return
	}
}

func (d *data) pubMsgFromNodeDaemondataDiffForNode(peer string, current *remoteInfo) {
	if current == nil {
		return
	}
	prevTimes, hasPrev := d.previousRemoteInfo[peer]
	if !hasPrev || current.daemondataUpdated.After(prevTimes.daemondataUpdated) {
		a := d.clusterData.Cluster.Node[peer].Daemon.Daemondata
		daemonsubsystem.DataDaemondata.Set(peer, a.DeepCopy())
		d.publisher.Pub(&msgbus.DaemonDataUpdated{Node: peer, Value: *a.DeepCopy()},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
		return
	}
}

func (d *data) pubMsgFromNodeDnsDiffForNode(peer string, current *remoteInfo) {
	if current == nil {
		return
	}
	prevTimes, hasPrev := d.previousRemoteInfo[peer]
	if !hasPrev || current.dnsUpdated.After(prevTimes.dnsUpdated) {
		a := d.clusterData.Cluster.Node[peer].Daemon.Dns
		daemonsubsystem.DataDns.Set(peer, a.DeepCopy())
		d.publisher.Pub(&msgbus.DaemonDnsUpdated{Node: peer, Value: *a.DeepCopy()},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
		return
	}
}

func (d *data) pubMsgFromNodeListenerDiffForNode(peer string, current *remoteInfo) {
	if current == nil {
		return
	}
	prevTimes, hasPrev := d.previousRemoteInfo[peer]
	if !hasPrev || current.listenerUpdated.After(prevTimes.listenerUpdated) {
		found := d.clusterData.Cluster.Node[peer].Daemon.Listener
		daemonsubsystem.DataListener.Set(peer, found.DeepCopy())
		d.publisher.Pub(&msgbus.DaemonListenerUpdated{Node: peer, Value: *found.DeepCopy()},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
		return
	}
}

func (d *data) pubMsgFromNodeRunnerImonDiffForNode(peer string, current *remoteInfo) {
	if current == nil {
		return
	}
	prevTimes, hasPrev := d.previousRemoteInfo[peer]
	if !hasPrev || current.runnerImon.After(prevTimes.runnerImon) {
		a := d.clusterData.Cluster.Node[peer].Daemon.RunnerImon
		daemonsubsystem.DataRunnerImon.Set(peer, a.DeepCopy())
		d.publisher.Pub(&msgbus.DaemonRunnerImonUpdated{Node: peer, Value: *a.DeepCopy()},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
		return
	}
}

func (d *data) pubMsgFromNodeSchedulerDiffForNode(peer string, current *remoteInfo) {
	if current == nil {
		return
	}
	prevTimes, hasPrev := d.previousRemoteInfo[peer]
	if !hasPrev || current.scheduler.After(prevTimes.scheduler) {
		a := d.clusterData.Cluster.Node[peer].Daemon.Scheduler
		daemonsubsystem.DataScheduler.Set(peer, a.DeepCopy())
		d.publisher.Pub(&msgbus.DaemonSchedulerUpdated{Node: peer, Value: *a.DeepCopy()},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
		return
	}
}

func (d *data) pubMsgFromNodeMonitorDiffForNode(peer string, current *remoteInfo) {
	if current == nil {
		return
	}
	prevTimes, hasPrev := d.previousRemoteInfo[peer]
	if !hasPrev || current.nmonUpdated.After(prevTimes.nmonUpdated) {
		localMonitor := d.clusterData.Cluster.Node[peer].Monitor
		node.MonitorData.Set(peer, localMonitor.DeepCopy())
		d.publisher.Pub(&msgbus.NodeMonitorUpdated{Node: peer, Value: *localMonitor.DeepCopy()},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
		return
	}
}

func getUpdatedRemoved(toPath map[string]naming.Path, previous, current map[string]time.Time) (updates, removes []string) {
	for s, updated := range current {
		if _, ok := toPath[s]; !ok {
			p, err := naming.ParsePath(s)
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
			p, err := naming.ParsePath(s)
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

func (d *data) pubMsgFromNodeInstanceDiffForNode(peer string, current *remoteInfo) {
	var updates, removes []string
	toPath := make(map[string]naming.Path)
	previous, ok := d.previousRemoteInfo[peer]
	if !ok {
		previous = remoteInfo{
			imonUpdated:       make(map[string]time.Time),
			instConfigUpdated: make(map[string]time.Time),
			instStatusUpdated: make(map[string]time.Time),
		}
	}
	updates, removes = getUpdatedRemoved(toPath, previous.instConfigUpdated, current.instConfigUpdated)
	for _, s := range updates {
		if _, ok := previous.instConfigUpdated[s]; !ok {
			// ObjectCreated is published by icfg, before initial
			// InstanceConfigUpdated publication.
			d.publisher.Pub(&msgbus.ObjectCreated{Path: toPath[s], Node: peer},
				pubsub.Label{"namespace", toPath[s].Namespace},
				pubsub.Label{"path", s},
				pubsub.Label{"node", peer},
				labelFromPeer,
			)
		}
		instance.ConfigData.Set(toPath[s], peer, d.clusterData.Cluster.Node[peer].Instance[s].Config.DeepCopy())
		d.publisher.Pub(&msgbus.InstanceConfigUpdated{Path: toPath[s], Node: peer, Value: *d.clusterData.Cluster.Node[peer].Instance[s].Config.DeepCopy()},
			pubsub.Label{"namespace", toPath[s].Namespace},
			pubsub.Label{"path", s},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
	}
	for _, s := range removes {
		instance.ConfigData.Unset(toPath[s], peer)
		d.publisher.Pub(&msgbus.InstanceConfigDeleted{Path: toPath[s], Node: peer},
			pubsub.Label{"namespace", toPath[s].Namespace},
			pubsub.Label{"path", s},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
	}

	updates, removes = getUpdatedRemoved(toPath, previous.instStatusUpdated, current.instStatusUpdated)
	for _, s := range updates {
		instance.StatusData.Set(toPath[s], peer, d.clusterData.Cluster.Node[peer].Instance[s].Status.DeepCopy())
		d.publisher.Pub(&msgbus.InstanceStatusUpdated{Path: toPath[s], Node: peer, Value: *d.clusterData.Cluster.Node[peer].Instance[s].Status.DeepCopy()},
			pubsub.Label{"namespace", toPath[s].Namespace},
			pubsub.Label{"path", s},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
	}
	for _, s := range removes {
		instance.StatusData.Unset(toPath[s], peer)
		d.publisher.Pub(&msgbus.InstanceStatusDeleted{Path: toPath[s], Node: peer},
			pubsub.Label{"namespace", toPath[s].Namespace},
			pubsub.Label{"path", s},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
	}

	updates, removes = getUpdatedRemoved(toPath, previous.imonUpdated, current.imonUpdated)
	for _, s := range updates {
		instance.MonitorData.Set(toPath[s], peer, d.clusterData.Cluster.Node[peer].Instance[s].Monitor.DeepCopy())
		d.publisher.Pub(&msgbus.InstanceMonitorUpdated{Path: toPath[s], Node: peer, Value: *d.clusterData.Cluster.Node[peer].Instance[s].Monitor.DeepCopy()},
			pubsub.Label{"namespace", toPath[s].Namespace},
			pubsub.Label{"path", s},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
	}
	for _, s := range removes {
		instance.MonitorData.Unset(toPath[s], peer)
		d.publisher.Pub(&msgbus.InstanceMonitorDeleted{Path: toPath[s], Node: peer},
			pubsub.Label{"namespace", toPath[s].Namespace},
			pubsub.Label{"path", s},
			pubsub.Label{"node", peer},
			labelFromPeer,
		)
	}
}
