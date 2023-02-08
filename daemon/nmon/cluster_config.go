package nmon

import (
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/stringslice"
)

// onJoinRequest handle JoinRequest to update cluster config with new node.
//
// If error occurs publish msgbus.JoinIgnored, or msgbus.JoinError:
//
// - publish msgbus.JoinIgnored,join-node=node (the node already exists in cluster nodes)
// - publish msgbus.JoinError,join-node=node (update cluster config object fails)
func (o *nmon) onJoinRequest(c msgbus.JoinRequest) {
	nodes := ccfg.Get().Nodes
	node := c.Node
	labels := []pubsub.Label{
		{"node", hostname.Hostname()},
		{"join-node", node},
	}
	o.log.Info().Msgf("join request for node %s", node)
	if stringslice.Has(node, nodes) {
		o.log.Debug().Msgf("join request ignored already member")
		o.bus.Pub(msgbus.JoinIgnored{Node: node}, labels...)
	} else if err := o.addClusterNode(node); err != nil {
		o.log.Warn().Err(err).Msgf("join request denied")
		o.bus.Pub(msgbus.JoinError{Node: node, Reason: err.Error()}, labels...)
	}
}

// addClusterNode adds node to cluster config
func (o *nmon) addClusterNode(node string) error {
	o.log.Debug().Msgf("adding cluster node %s", node)
	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}
	op := keyop.New(key.New("cluster", "nodes"), keyop.Append, node, 0)
	if err := ccfg.Config().Set(*op); err != nil {
		return err
	}
	if err := ccfg.Config().Commit(); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	return nil
}

// onLeaveRequest handle LeaveRequest to update cluster config with node removal.
//
// If error occurs publish msgbus.LeaveIgnored, or msgbus.LeaveError:
//
// - publish msgbus.LeaveIgnored,leave-node=node (the node is not a cluster nodes)
// - publish msgbus.LeaveError,leave-node=node (update cluster config object fails)
func (o *nmon) onLeaveRequest(c msgbus.LeaveRequest) {
	nodes := ccfg.Get().Nodes
	node := c.Node
	labels := []pubsub.Label{
		{"node", hostname.Hostname()},
		{"leave-node", node},
	}
	o.log.Info().Msgf("leave request for node %s", node)
	if !stringslice.Has(node, nodes) {
		o.log.Debug().Msgf("leave request ignored for not cluster member")
		o.bus.Pub(msgbus.LeaveIgnored{Node: node}, labels...)
	} else if err := o.removeClusterNode(node); err != nil {
		o.log.Warn().Err(err).Msgf("leave request denied")
		o.bus.Pub(msgbus.LeaveError{Node: node, Reason: err.Error()}, labels...)
	}
}

// removeClusterNode removes node from cluster config
func (o *nmon) removeClusterNode(node string) error {
	o.log.Debug().Msgf("removing cluster node %s", node)
	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}
	op := keyop.New(key.New("cluster", "nodes"), keyop.Remove, node, 0)
	if err := ccfg.Config().Set(*op); err != nil {
		return err
	}
	if err := ccfg.Config().Commit(); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	// remove peer node from daemon data
	if err := o.databus.DropPeerNode(node); err != nil {
		return err
	}
	return nil
}
