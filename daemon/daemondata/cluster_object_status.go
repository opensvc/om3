package daemondata

import (
	"context"
	"encoding/json"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	opDelObjectStatus struct {
		errC
		path path.T
	}

	opSetObjectStatus struct {
		errC
		path  path.T
		value object.Status
		srcEv any
	}
)

// DelObjectStatus
//
// cluster.object.*
func (t T) DelObjectStatus(p path.T) error {
	err := make(chan error, 1)
	op := opDelObjectStatus{
		errC: err,
		path: p,
	}
	t.cmdC <- op
	return <-err
}

// SetObjectStatus
//
// cluster.object.*
func (t T) SetObjectStatus(p path.T, v object.Status, ev any) error {
	err := make(chan error, 1)
	op := opSetObjectStatus{
		errC:  err,
		path:  p,
		value: v,
		srcEv: ev,
	}
	t.cmdC <- op
	return <-err
}

func (o opDelObjectStatus) call(ctx context.Context, d *data) error {
	d.counterCmd <- idDelObjectStatus
	s := o.path.String()
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
			d.bus.Pub(msgbus.DataUpdated{RawMessage: eventB}, labelLocalNode)
		}
	}
	d.bus.Pub(
		msgbus.ObjectStatusDeleted{
			Path: o.path,
			Node: d.localNode,
		},
		pubsub.Label{"path", s},
		labelLocalNode,
	)
	return nil
}

func (o opSetObjectStatus) call(ctx context.Context, d *data) error {
	d.counterCmd <- idSetObjectStatus
	s := o.path.String()
	labelPath := pubsub.Label{"path", s}
	d.pending.Cluster.Object[s] = o.value

	// TODO choose between DataUpdated<->pendingOps (pendingOps publish DataUpdated but no easy label)
	patch := jsondelta.Patch{jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"cluster", "object", s},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}}
	if eventB, err := json.Marshal(patch); err != nil {
		d.log.Error().Err(err).Msg("eventCommitPendingOps Marshal fromRootPatch")
	} else {
		eventId++
		d.bus.Pub(msgbus.DataUpdated{RawMessage: eventB}, labelLocalNode, labelPath)
	}
	d.bus.Pub(
		msgbus.ObjectStatusUpdated{
			Path:  o.path,
			Node:  d.localNode,
			Value: o.value,
			SrcEv: o.srcEv,
		},
		labelLocalNode,
		labelPath,
	)
	return nil
}
