package cstat

import (
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (o *cstat) onNodeStatusUpdated(c *msgbus.NodeStatusUpdated) {
	o.nodeStatus[c.Node] = c.Value
	o.updateCompat()
	o.updateFrozen()
	if o.change {
		o.change = false
		localhost := hostname.Hostname()
		labelLocalNode := pubsub.Label{"node", localhost}
		o.bus.Pub(&msgbus.ClusterStatusUpdated{Node: localhost, Value: o.state}, labelLocalNode)
	}
}

func (o *cstat) updateCompat() {
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
	if compat := getCompat(); o.state.Compat != compat {
		o.state.Compat = compat
		o.change = true
	}
}

func (o *cstat) updateFrozen() {
	getFrozen := func() bool {
		for _, nodeStatus := range o.nodeStatus {
			if nodeStatus.IsFrozen() {
				return true
			}
		}
		return false
	}
	if frozen := getFrozen(); o.state.Frozen != frozen {
		o.state.Frozen = frozen
		o.change = true
	}
}
