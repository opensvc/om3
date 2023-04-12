package daemondata

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/msgbus"
)

var (
	eventId uint64
)

func (d *data) applyPatch(msg *hbtype.Msg) error {
	d.statCount[idApplyPatch]++
	local := d.localNode
	remote := msg.Nodename
	d.log.Debug().Msgf("apply patch %s", remote)
	var (
		sortGen []uint64
	)

	setNeedFull := func() {
		d.hbGens[local][remote] = 0
	}

	if d.hbGens[local][remote] == 0 {
		d.log.Debug().Msgf("apply patch skipped %s gen %v (wait full)", remote, msg.Gen[remote])
		return nil
	}
	if msg.Updated.Before(d.hbPatchMsgUpdated[remote]) {
		d.log.Debug().Msgf(
			"apply patch skipped %s outdated msg "+
				"(msg [gen:%d updated:%s], "+
				"latest applied msg:[gen:%d updated:%s])",
			remote,
			msg.Gen[remote], msg.Updated,
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
		var evIds []string
		for k := range msg.Events {
			evIds = append(evIds, k)
		}
		d.log.Info().Msgf(
			"apply patch skipped %s gen %v (ask full from restarted remote) len delta:%d hbGens:%+v "+
				"evIds: %+v "+
				"hbGens[%s][%s]:%d "+
				"hbGens[%s]:%+v ",
			remote, msg.Gen[remote], len(msg.Events), d.hbGens,
			evIds,
			remote, remote, pendingNodeGen,
			remote, d.hbGens[remote],
		)
		setNeedFull()
		return nil
	}
	if len(msg.Events) == 0 && msg.Gen[remote] > pendingNodeGen {
		var eventIds []string
		for k := range msg.Events {
			eventIds = append(eventIds, k)
		}
		d.log.Info().Msgf(
			"apply patch skipped %s gen %d (ask full from empty patch) len events: %d gens:%+v eventIds: %+v hbGens[%s]: %v",
			remote, msg.Gen[remote], len(msg.Events), d.hbGens, eventIds, remote, d.hbGens[remote])
		d.log.Info().Msgf("Msg is : %+v", *msg)

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
	d.log.Debug().Msgf("apply patch sequence %s %v", remote, sortGen)
	for _, gen := range sortGen {
		genS := strconv.FormatUint(gen, 10)
		if gen <= pendingNodeGen {
			continue
		}
		if gen > pendingNodeGen+1 {
			err := fmt.Errorf("apply patch %s found broken sequence on gen %d from sequence %v, current known gen %d", remote, gen, sortGen, pendingNodeGen)
			d.log.Info().Err(err).Msgf("apply patch need full %s", remote)
			setNeedFull()
			return err
		}
		d.log.Debug().Msgf("apply patch %s delta gen %s", remote, genS)
		for _, ev := range events[genS] {
			if err := d.setCacheAndPublish(ev); err != nil {
				d.log.Info().Err(err).Msgf("error during patch %s events %s kind %s (ask full)", remote, genS, ev.Kind)
				setNeedFull()
				return err
			}
			d.hbGens[local][remote] = gen
		}

		d.hbPatchMsgUpdated[remote] = msg.Updated

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
	if err := d.clusterData.ApplyMessage(msg); err != nil {
		d.log.Error().Err(err).Msgf("apply patch: can't apply message %+v", msg)
		panic("apply patch -> ApplyMessage error "+err.Error())
	}
	switch c := msg.(type) {
	case *msgbus.ObjectStatusDeleted:
		object.StatusData.Unset(c.Path)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.ObjectStatusUpdated:
		object.StatusData.Set(c.Path, &c.Value)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.InstanceConfigDeleted:
		instance.ConfigData.Unset(c.Path, c.Node)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.InstanceConfigUpdated:
		instance.ConfigData.Set(c.Path, c.Node, &c.Value)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.InstanceMonitorDeleted:
		instance.MonitorData.Unset(c.Path, c.Node)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.InstanceMonitorUpdated:
		instance.MonitorData.Set(c.Path, c.Node, &c.Value)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.InstanceStatusDeleted:
		instance.StatusData.Unset(c.Path, c.Node)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.InstanceStatusUpdated:
		instance.StatusData.Set(c.Path, c.Node, &c.Value)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.NodeConfigUpdated:
		node.ConfigData.Set(c.Node, &c.Value)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.NodeMonitorDeleted:
		node.MonitorData.Unset(c.Node)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.NodeMonitorUpdated:
		node.MonitorData.Set(c.Node, &c.Value)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.NodeOsPathsUpdated:
		node.OsPathsData.Set(c.Node, &c.Value)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.NodeStatsUpdated:
		node.StatsData.Set(c.Node, &c.Value)
		d.bus.Pub(c, labelPeerNode)
	case *msgbus.NodeStatusUpdated:
		node.StatusData.Set(c.Node, &c.Value)
		node.GenData.Set(c.Node, &c.Value.Gen)
		d.bus.Pub(c, labelPeerNode)
	default:
		d.log.Error().Msgf("drop msg kind %s %d : %+v\n", ev.Kind, ev.ID, ev.Data)
	}
	return nil
}
