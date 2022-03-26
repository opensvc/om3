package daemondata

import (
	"encoding/json"
	"sort"
	"strconv"

	"gopkg.in/errgo.v2/fmt/errors"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/util/eventbus"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/timestamp"
)

type opApplyRemotePatch struct {
	nodename string
	msg      *hbtype.Msg
	err      chan<- error
}

var (
	eventId uint64
)

func (o opApplyRemotePatch) call(d *data) {
	d.counterCmd <- idApplyPatch
	d.log.Debug().Msgf("opApplyRemotePatch for %s", o.nodename)
	var (
		pendingB []byte
		err      error
		data     json.RawMessage
	)
	pendingNode, ok := d.pending.Monitor.Nodes[o.nodename]
	if !ok {
		d.log.Debug().Msgf("skip patch unknown remote %s", o.nodename)
		o.err <- nil
		return
	}
	pendingNodeGen := pendingNode.Gen[o.nodename]
	pendingB, err = json.Marshal(pendingNode)
	if err != nil {
		d.log.Error().Err(err).Msgf("Marshal pendingNode %s", o.nodename)
		o.err <- err
		return
	}
	deltas := o.msg.Deltas
	sortGens := make([]string, 0, len(deltas))
	for k := range deltas {
		sortGens = append(sortGens, k)
	}
	sort.Strings(sortGens)
	parentPath := jsondelta.OperationPath{"monitor", "nodes", o.nodename}
	for genS := range deltas {
		patchGen, err := strconv.ParseUint(string(genS), 10, 64)
		if err != nil {
			continue
		}
		if patchGen <= pendingNodeGen {
			continue
		}
		if patchGen > pendingNodeGen+1 {
			d.pending.Monitor.Nodes[d.localNode].Gen[o.nodename] = uint64(0)
			err := errors.New("ApplyRemotePatch invalid patch gen: " + genS)
			d.log.Info().Err(err).Msgf("need full %s", o.nodename)
			d.pending.Monitor.Nodes[d.localNode].Gen[o.nodename] = 0
			o.err <- err
			return
		}
		patch := jsondelta.NewPatchFromOperations(deltas[genS])
		pendingB, err = patch.Apply(pendingB)
		if err != nil {
			d.log.Info().Err(err).Msgf("patch apply %s need full", o.nodename)
			d.pending.Monitor.Nodes[d.localNode].Gen[o.nodename] = 0
			o.err <- err
			return
		}

		absolutePatch := make(jsondelta.Patch, 0)
		for _, op := range deltas[genS] {
			absolutePatch = append(absolutePatch, jsondelta.Operation{
				OpPath:  append(parentPath, op.OpPath...),
				OpValue: op.OpValue,
				OpKind:  op.OpKind,
			})
		}
		if eventB, err := json.Marshal(absolutePatch); err != nil {
			d.log.Error().Err(err).Msgf("Marshal absolutePatch %s", o.nodename)
			o.err <- err
			return
		} else {
			data = eventB
			eventId++
			eventbus.Pub(d.eventCmd, event.Event{
				Kind:      "patch",
				ID:        eventId,
				Timestamp: timestamp.Now(),
				Data:      &data,
			})
		}
		pendingNodeGen = patchGen
	}
	pendingNode = cluster.NodeStatus{}
	if err := json.Unmarshal(pendingB, &pendingNode); err != nil {
		d.log.Error().Err(err).Msgf("Unmarshal pendingB %s", o.nodename)
		o.err <- err
		return
	}
	pendingNode.Gen[o.nodename] = pendingNodeGen
	d.pending.Monitor.Nodes[d.localNode].Gen[o.nodename] = pendingNodeGen
	d.pending.Monitor.Nodes[o.nodename] = pendingNode
	o.err <- nil
}

func (t T) ApplyPatch(nodename string, msg *hbtype.Msg) error {
	err := make(chan error)
	t.cmdC <- opApplyRemotePatch{
		nodename: nodename,
		msg:      msg,
		err:      err,
	}
	return <-err
}
