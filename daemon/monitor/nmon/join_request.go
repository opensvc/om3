package nmon

import (
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/stringslice"
)

func (o *nmon) onJoinRequest(c msgbus.JoinRequest) {
	bus := pubsub.BusFromContext(o.ctx)
	if err := o.config.Reload(); err != nil {
		o.log.Info().Err(err).Msg("Reload config before join request")
	}
	nodes := o.config.GetStrings(key.New("cluster", "nodes"))
	node := c.Node
	labels := []pubsub.Label{
		{"node", hostname.Hostname()},
		{"join-node", node},
	}
	o.log.Info().Msgf("join request for node %s", node)
	if stringslice.Has(node, nodes) {
		reason := "already member"
		o.log.Info().Msgf("join request ignored %s", reason)
		bus.Pub(msgbus.JoinIgnored{Node: node, Reason: reason}, labels...)
	} else if err := o.crmAddNode(node); err != nil {
		o.log.Warn().Err(err).Msgf("join request denied")
		bus.Pub(msgbus.JoinDenied{Node: node, Reason: err.Error()}, labels...)
	}
}
