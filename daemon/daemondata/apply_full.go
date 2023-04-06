package daemondata

import (
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (d *data) applyFull(msg *hbtype.Msg) error {
	d.statCount[idApplyFull]++
	remote := msg.Nodename
	peerLabel := pubsub.Label{"node", remote}
	local := d.localNode
	d.log.Debug().Msgf("applyFull %s", remote)

	d.clusterData.Cluster.Node[remote] = msg.Full
	d.hbGens[local][remote] = msg.Full.Status.Gen[remote]
	d.clusterData.Cluster.Node[local].Status.Gen[remote] = msg.Full.Status.Gen[remote]

	d.bus.Pub(&msgbus.NodeDataUpdated{Node: remote, Value: msg.Full}, peerLabel, labelPeerNode)

	// TODO improve delta change has it was done in apply_commit_remote_diff to
	// avoid recreate all events on each applied full
	node.StatusData.Set(remote, &msg.Full.Status)
	d.bus.Pub(&msgbus.NodeStatusUpdated{Node: remote, Value: msg.Full.Status},
		labelPeerNode,
		peerLabel,
	)
	node.ConfigData.Set(remote, &msg.Full.Config)
	d.bus.Pub(&msgbus.NodeConfigUpdated{Node: remote, Value: msg.Full.Config},
		labelPeerNode,
		peerLabel,
	)
	node.MonitorData.Set(remote, &msg.Full.Monitor)
	d.bus.Pub(&msgbus.NodeMonitorUpdated{Node: remote, Value: msg.Full.Monitor},
		labelPeerNode,
		peerLabel,
	)
	for s, i := range msg.Full.Instance {
		p, err := path.Parse(s)
		if err != nil {
			panic("invalid instance path: " + s)
		}
		if i.Config != nil {
			instance.ConfigData.Set(p, remote, i.Config)
			d.bus.Pub(&msgbus.InstanceConfigUpdated{Path: p, Node: remote, Value: *i.Config},
				pubsub.Label{"path", s},
				labelPeerNode,
				peerLabel,
			)
		}
		if i.Status != nil {
			instance.StatusData.Set(p, remote, i.Status)
			d.bus.Pub(&msgbus.InstanceStatusUpdated{Path: p, Node: remote, Value: *i.Status},
				pubsub.Label{"path", s},
				labelPeerNode,
				peerLabel,
			)
		}
		if i.Monitor != nil {
			instance.MonitorData.Set(p, remote, i.Monitor)
			d.bus.Pub(&msgbus.InstanceMonitorUpdated{Path: p, Node: remote, Value: *i.Monitor},
				pubsub.Label{"path", s},
				labelPeerNode,
				peerLabel,
			)
		}
	}

	return nil
}
