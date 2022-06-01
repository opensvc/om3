package daemondata

import (
	"encoding/json"
	"sort"
	"strconv"

	"gopkg.in/errgo.v2/fmt/errors"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/daemonps"
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
	var sortGen []uint64
	for k := range deltas {
		gen, err := strconv.ParseUint(k, 10, 64)
		if err != nil {
			continue
		}
		sortGen = append(sortGen, gen)
	}
	sort.Slice(sortGen, func(i, j int) bool { return sortGen[i] < sortGen[j] })
	d.log.Debug().Msgf("ApplyRemotePatch patch sequence %v", sortGen)
	parentPath := jsondelta.OperationPath{"monitor", "nodes", o.nodename}
	for _, gen := range sortGen {
		genS := strconv.FormatUint(gen, 10)
		if gen <= pendingNodeGen {
			continue
		}
		if gen > pendingNodeGen+1 {
			err := errors.New("ApplyRemotePatch invalid patch gen: " + genS)
			d.log.Info().Err(err).Msgf("need full %s", o.nodename)
			d.remotesNeedFull[o.nodename] = true
			o.err <- err
			return
		}
		patch := jsondelta.NewPatchFromOperations(deltas[genS])
		d.log.Debug().Msgf("ApplyRemotePatch applying %s gen %s", o.nodename, genS)
		pendingB, err = patch.Apply(pendingB)
		if err != nil {
			d.log.Info().Err(err).Msgf("patch apply %s gen %s need full", o.nodename, genS)
			d.remotesNeedFull[o.nodename] = true
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
			daemonps.PubEvent(d.pubSub, event.Event{
				Kind:      "patch",
				ID:        eventId,
				Timestamp: timestamp.Now(),
				Data:      &data,
			})
		}
		pendingNodeGen = gen
	}
	pendingNode = cluster.NodeStatus{}
	if err := json.Unmarshal(pendingB, &pendingNode); err != nil {
		d.log.Error().Err(err).Msgf("Unmarshal pendingB %s", o.nodename)
		o.err <- err
		return
	}
	d.mergedFromPeer[o.nodename] = pendingNodeGen
	if gen, ok := o.msg.Gen[d.localNode]; ok {
		d.mergedOnPeer[o.nodename] = gen
	}
	pendingNode.Gen[o.nodename] = pendingNodeGen
	d.pending.Monitor.Nodes[o.nodename] = pendingNode
	d.log.Debug().
		Interface("mergedFromPeer", d.mergedFromPeer).
		Interface("mergedOnPeer", d.mergedOnPeer).
		Interface("pendingNode.Gen", d.pending.Monitor.Nodes[o.nodename].Gen).
		Interface("remotesNeedFull", d.remotesNeedFull).
		Msgf("opApplyRemotePatch for %s", o.nodename)
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
