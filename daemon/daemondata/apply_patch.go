package daemondata

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/msgbus"
)

var (
	eventID uint64
)

func (d *data) applyMsgEvents(msg *hbtype.Msg) error {
	local := d.localNode
	remote := msg.Nodename
	d.log.Debugf("apply patch %s", remote)
	var (
		sortGen []uint64
	)

	setNeedFull := func() {
		d.hbGens[local][remote] = 0
	}

	if d.hbGens[local][remote] == 0 {
		d.log.Debugf("apply patch skipped %s gen %v (wait full)", remote, msg.Gen[remote])
		return nil
	}
	if msg.UpdatedAt.Before(d.hbPatchMsgUpdated[remote]) {
		d.log.Debugf(
			"apply patch skipped %s outdated msg "+
				"(msg [gen:%d updated:%s], "+
				"latest applied msg:[gen:%d updated:%s])",
			remote,
			msg.Gen[remote], msg.UpdatedAt,
			d.clusterData.Cluster.Node[remote].Status.Gen[remote], d.hbPatchMsgUpdated[remote],
		)
		return nil
	}
	_, ok := d.clusterData.Cluster.Node[remote]
	if !ok {
		panic("apply patch on nil cluster node data for " + remote)
	}
	pendingNodeGen := d.hbGens[local][remote]
	if msg.Gen[remote] < pendingNodeGen {
		var evIDs []string
		for k := range msg.Events {
			evIDs = append(evIDs, k)
		}
		d.log.Infof(
			"apply patch skipped %s gen %v (ask full from restarted remote) len delta:%d hbGens:%+v "+
				"evIDs: %+v "+
				"hbGens[%s][%s]:%d "+
				"hbGens[%s]:%+v ",
			remote, msg.Gen[remote], len(msg.Events), d.hbGens,
			evIDs,
			remote, remote, pendingNodeGen,
			remote, d.hbGens[remote],
		)
		setNeedFull()
		return nil
	}
	if len(msg.Events) == 0 && msg.Gen[remote] > pendingNodeGen {
		var eventIDs []string
		for k := range msg.Events {
			eventIDs = append(eventIDs, k)
		}
		d.log.Infof(
			"apply patch skipped %s gen %d (ask full from empty patch) len events: %d gens:%+v eventIDs: %+v hbGens[%s]: %v",
			remote, msg.Gen[remote], len(msg.Events), d.hbGens, eventIDs, remote, d.hbGens[remote])
		d.log.Debugf("Msg is : %+v", *msg)

		setNeedFull()
		return nil
	}
	events := msg.Events
	for k := range events {
		gen, err1 := strconv.ParseUint(k, 10, 64)
		if err1 != nil {
			continue
		}
		sortGen = append(sortGen, gen)
	}
	sort.Slice(sortGen, func(i, j int) bool { return sortGen[i] < sortGen[j] })
	d.log.Debugf("apply patch sequence %s %v", remote, sortGen)
	for _, gen := range sortGen {
		genS := strconv.FormatUint(gen, 10)
		if gen <= pendingNodeGen {
			continue
		}
		if gen > pendingNodeGen+1 {
			err := fmt.Errorf("apply patch %s found broken sequence on gen %d from sequence %v, current known gen %d", remote, gen, sortGen, pendingNodeGen)
			d.log.Infof("apply patch need full %s: %s", remote, err)
			setNeedFull()
			return err
		}
		d.log.Debugf("apply patch %s delta gen %s", remote, genS)
		for _, ev := range events[genS] {
			if err := d.setCacheAndPublish(ev); err != nil {
				d.log.Infof("can't apply patch %s events %s kind %s (ask full): %s", remote, genS, ev.Kind, err)
				setNeedFull()
				return err
			}
			d.hbGens[local][remote] = gen
		}

		d.hbPatchMsgUpdated[remote] = msg.UpdatedAt

		pendingNodeGen = gen
	}
	remoteNodeData := d.clusterData.Cluster.Node[remote]
	remoteNodeData.Status.Gen = msg.Gen
	d.clusterData.Cluster.Node[remote] = remoteNodeData
	d.clusterData.Cluster.Node[local].Status.Gen[remote] = msg.Gen[remote]
	return nil
}

func (d *data) setCacheAndPublish(ev event.Event) error {
	msg, err := msgbus.EventToMessage(ev)
	if err != nil {
		return nil
	}

	d.clusterData.ApplyMessage(msg)

	switch c := msg.(type) {
	// daemon
	case *msgbus.DaemonCollectorUpdated:
		daemonsubsystem.DataCollector.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.DaemonDataUpdated:
		daemonsubsystem.DataDaemondata.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.DaemonDnsUpdated:
		daemonsubsystem.DataDns.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.DaemonHeartbeatUpdated:
		daemonsubsystem.DataHeartbeat.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.DaemonListenerUpdated:
		daemonsubsystem.DataListener.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.DaemonRunnerImonUpdated:
		daemonsubsystem.DataRunnerImon.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.DaemonSchedulerUpdated:
		daemonsubsystem.DataScheduler.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.DaemonStatusUpdated:
		d.publisher.Pub(c, labelFromPeer)
	// instances...
	case *msgbus.InstanceConfigDeleted:
		instance.ConfigData.Unset(c.Path, c.Node)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.InstanceConfigFor:
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.InstanceConfigUpdated:
		instance.ConfigData.Set(c.Path, c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.InstanceMonitorDeleted:
		instance.MonitorData.Unset(c.Path, c.Node)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.InstanceMonitorUpdated:
		instance.MonitorData.Set(c.Path, c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.InstanceStatusDeleted:
		instance.StatusData.Unset(c.Path, c.Node)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.InstanceStatusUpdated:
		instance.StatusData.Set(c.Path, c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	// node...
	case *msgbus.NodeConfigUpdated:
		node.ConfigData.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.NodeMonitorDeleted:
		node.MonitorData.Unset(c.Node)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.NodeMonitorUpdated:
		node.MonitorData.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.NodeOsPathsUpdated:
		node.OsPathsData.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.NodeStatsUpdated:
		node.StatsData.Set(c.Node, &c.Value)
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.NodeStatusLabelsUpdated:
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.NodeStatusUpdated:
		node.StatusData.Set(c.Node, &c.Value)
		node.GenData.Set(c.Node, &c.Value.Gen)
		d.publisher.Pub(c, labelFromPeer)
	// object...
	case *msgbus.ObjectCreated:
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.ObjectOrchestrationEnd:
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.ObjectOrchestrationRefused:
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.ObjectStatusDeleted:
		d.publisher.Pub(c, labelFromPeer)
	// pool...
	case *msgbus.NodePoolStatusUpdated:
		pool.StatusData.Set(c.Name, c.Node, c.Value.DeepCopy())
		d.publisher.Pub(c, labelFromPeer)
	// overload
	case *msgbus.EnterOverloadPeriod:
		d.publisher.Pub(c, labelFromPeer)
	case *msgbus.LeaveOverloadPeriod:
		d.publisher.Pub(c, labelFromPeer)
	default:
		d.log.Errorf("drop msg kind %s %d : %+v\n", ev.Kind, ev.ID, ev.Data)
	}
	return nil
}
