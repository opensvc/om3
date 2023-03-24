package daemondata

import (
	"encoding/json"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/pubsub"
)

// onObjectStatusDeleted delete .cluster.object.<path>
func (d *data) onObjectStatusDeleted(m msgbus.ObjectStatusDeleted) {
	d.statCount[idDelObjectStatus]++
	s := m.Path.String()
	if _, ok := d.pending.Cluster.Object[s]; ok {
		delete(d.pending.Cluster.Object, s)
		patch := jsondelta.Patch{jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"cluster", "object", s},
			OpKind: "remove",
		}}
		if eventB, err := json.Marshal(patch); err != nil {
			d.log.Error().Err(err).Msg("eventCommitPendingOps Marshal fromRootPatch")
		} else {
			eventId++
			d.bus.Pub(msgbus.DataUpdated{RawMessage: eventB}, d.labelLocalNode)
		}
	}
}

// onObjectStatusUpdated updates .cluster.object.<path>
func (d *data) onObjectStatusUpdated(m msgbus.ObjectStatusUpdated) {
	d.statCount[idSetObjectStatus]++
	s := m.Path.String()
	labelPath := pubsub.Label{"path", s}
	d.pending.Cluster.Object[s] = m.Value

	// TODO choose between DataUpdated<->pendingOps (pendingOps publish DataUpdated but no easy label)
	patch := jsondelta.Patch{jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"cluster", "object", s},
		OpValue: jsondelta.NewOptValue(m.Value),
		OpKind:  "replace",
	}}
	if eventB, err := json.Marshal(patch); err != nil {
		d.log.Error().Err(err).Msg("eventCommitPendingOps Marshal fromRootPatch")
	} else {
		eventId++
		d.bus.Pub(msgbus.DataUpdated{RawMessage: eventB}, d.labelLocalNode, labelPath)
	}
}
