package cstat

import (
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (o *T) onNodeStatusUpdated(c *msgbus.NodeStatusUpdated) {
	o.nodeStatus[c.Node] = c.Value
	o.updateCompat()
	o.updateFrozen()
	if o.change {
		o.change = false
		localhost := hostname.Hostname()
		labelLocalhost := pubsub.Label{"node", localhost}
		o.publisher.Pub(&msgbus.ClusterStatusUpdated{Node: localhost, Value: o.state}, labelLocalhost)
	}
}

func (o *T) updateCompat() {
	getCompat := func() bool {
		var lastCompat uint64
		for _, nodeStatus := range o.nodeStatus {
			if lastCompat == 0 {
				lastCompat = nodeStatus.Compat
			} else if lastCompat != nodeStatus.Compat {
				return false
			}
		}
		return true
	}
	if compat := getCompat(); o.state.IsCompat != compat {
		o.state.IsCompat = compat
		o.change = true
	}
}

func (o *T) updateFrozen() {
	getFrozen := func() bool {
		for _, nodeStatus := range o.nodeStatus {
			if nodeStatus.IsFrozen() {
				return true
			}
		}
		return false
	}
	if frozen := getFrozen(); o.state.IsFrozen != frozen {
		o.state.IsFrozen = frozen
		o.change = true
	}
}
