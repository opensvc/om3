package nmon

import (
	"slices"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/pubsub"
)

// onJoinRequest handle JoinRequest to update cluster config with new node.
//
// If error occurs publish msgbus.JoinIgnored, or msgbus.JoinError:
//
// - publish msgbus.JoinIgnored,join-node=node (the node already exists in cluster nodes)
// - publish msgbus.JoinError,join-node=node (update cluster config object fails)
func (t *Manager) onJoinRequest(c *msgbus.JoinRequest) {
	nodes := cluster.ConfigData.Get().Nodes
	node := c.Node
	labels := []pubsub.Label{
		{"node", hostname.Hostname()},
		{"join-node", node},
	}
	t.log.Infof("join request for node %s", node)
	if slices.Contains(nodes, node) {
		t.log.Debugf("join request ignored already member")
		t.publisher.Pub(&msgbus.JoinIgnored{Node: node}, labels...)
	} else if err := t.addClusterNode(node); err != nil {
		t.log.Warnf("join request denied: %s", err)
		t.publisher.Pub(&msgbus.JoinError{Node: node, Reason: err.Error()}, labels...)
	}
}

// addClusterNode adds node to cluster config
func (t *Manager) addClusterNode(node string) error {
	t.log.Debugf("adding cluster node %s", node)
	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}
	op := keyop.New(key.New("cluster", "nodes"), keyop.Append, node, 0)
	if err := ccfg.Config().Set(*op); err != nil {
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
func (t *Manager) onLeaveRequest(c *msgbus.LeaveRequest) {
	nodes := cluster.ConfigData.Get().Nodes
	node := c.Node
	labels := []pubsub.Label{
		{"node", hostname.Hostname()},
		{"leave-node", node},
	}
	t.log.Infof("leave request for node %s", node)
	if !slices.Contains(nodes, node) {
		t.log.Debugf("leave request ignored for not cluster member")
		t.publisher.Pub(&msgbus.LeaveIgnored{Node: node}, labels...)
	} else if err := t.removeClusterNode(node); err != nil {
		t.log.Warnf("leave request denied: %s", err)
		t.publisher.Pub(&msgbus.LeaveError{Node: node, Reason: err.Error()}, labels...)
	}
}

// removeClusterNode removes node from cluster config
func (t *Manager) removeClusterNode(node string) error {
	t.log.Debugf("removing cluster node %s", node)
	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}
	op := keyop.New(key.New("cluster", "nodes"), keyop.Remove, node, 0)
	if err := ccfg.Config().Set(*op); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	return nil
}
