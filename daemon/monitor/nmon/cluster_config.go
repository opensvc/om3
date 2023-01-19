package nmon

import (
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/stringslice"
)

// onJoinRequest handle JoinRequest to update cluster config with new node.
//
// If error occurs publish msgbus.JoinIgnored, or msgbus.JoinError:
//
// - publish msgbus.JoinIgnored,join-node=node (the node already exists in cluster nodes)
// - publish msgbus.JoinError,join-node=node (update cluster config object fails)
func (o *nmon) onJoinRequest(c msgbus.JoinRequest) {
	nodes := o.config.GetStrings(key.New("cluster", "nodes"))
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
	ccfg, err := object.NewCcfg(path.Cluster, object.WithVolatile(false))
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
