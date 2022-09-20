package daemondata

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"gopkg.in/errgo.v2/fmt/errors"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type opApplyRemotePatch struct {
	nodename string
	msg      *hbtype.Msg
	err      chan<- error
}

var (
	eventId uint64
)

func (o opApplyRemotePatch) call(ctx context.Context, d *data) {
	d.counterCmd <- idApplyPatch
	d.log.Debug().Msgf("apply remote patch for %s", o.nodename)
	var (
		pendingB []byte
		err      error
		data     json.RawMessage
		changes  bool
	)
	pendingNode, ok := d.pending.Cluster.Node[o.nodename]
	if !ok {
		d.log.Debug().Msgf("apply remote patch skip unknown remote %s", o.nodename)
		o.err <- nil
		return
	}
	pendingNodeGen := pendingNode.Status.Gen[o.nodename]
	if o.msg.Gen[o.nodename] < pendingNodeGen {
		d.log.Debug().Msgf("apply remote patch for %s drop message gen %d < %d", o.nodename, o.msg.Gen[o.nodename], pendingNodeGen)
		o.err <- nil
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
	d.log.Debug().Msgf("apply remote patch for %s sequence %v, current known remote gen %+v, pendingNodeGen:%d", o.nodename, sortGen, pendingNode.Status.Gen, pendingNodeGen)
	parentPath := jsondelta.OperationPath{"cluster", "node", o.nodename}
	for _, gen := range sortGen {
		genS := strconv.FormatUint(gen, 10)
		if gen <= pendingNodeGen {
			continue
		}
		if gen > pendingNodeGen+1 {
			msg := fmt.Sprintf("apply remote patch for %s found broken sequence on gen %d from sequence %v, current known gen %d", o.nodename, gen, sortGen, pendingNodeGen)
			err = errors.New(msg)
			d.log.Info().Err(err).Msgf("need full %s", o.nodename)
			d.remotesNeedFull[o.nodename] = true
			select {
			case <-ctx.Done():
			case o.err <- err:
			}
			return
		}
		if !changes {
			// initiate pendingB
			pendingB, err = json.Marshal(pendingNode)
			if err != nil {
				d.log.Error().Err(err).Msgf("Marshal pendingNode %s", o.nodename)
				o.err <- err
				return
			}
			changes = true
		}
		patch := jsondelta.NewPatchFromOperations(deltas[genS])
		d.log.Debug().Msgf("apply remote patch for %s applying gen %s", o.nodename, genS)
		pendingB, err = patch.Apply(pendingB)
		if err != nil {
			d.log.Info().Err(err).Msgf("apply remote patch for %s gen %s need full", o.nodename, genS)
			d.remotesNeedFull[o.nodename] = true
			select {
			case <-ctx.Done():
			case o.err <- err:
			}
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
			select {
			case <-ctx.Done():
			case o.err <- err:
			}
			return
		} else {
			data = eventB
			eventId++
			msgbus.PubEvent(d.bus, event.Event{
				Kind: "patch",
				ID:   eventId,
				Time: time.Now(),
				Data: &data,
			})
		}
		pendingNodeGen = gen
	}
	if changes {
		// patches has been applied get update pendingNode
		pendingNode = cluster.TNodeData{}
		if err := json.Unmarshal(pendingB, &pendingNode); err != nil {
			d.log.Error().Err(err).Msgf("Unmarshal pendingB %s", o.nodename)
			select {
			case <-ctx.Done():
			case o.err <- err:
			}
			return
		}
	}
	d.mergedFromPeer[o.nodename] = pendingNodeGen
	if gen, ok := o.msg.Gen[d.localNode]; ok {
		d.mergedOnPeer[o.nodename] = gen
	}
	pendingNode.Status.Gen = o.msg.Gen
	d.pending.Cluster.Node[o.nodename] = pendingNode
	d.pending.Cluster.Node[d.localNode].Status.Gen[o.nodename] = o.msg.Gen[o.nodename]
	d.log.Debug().
		Interface("mergedFromPeer", d.mergedFromPeer).
		Interface("mergedOnPeer", d.mergedOnPeer).
		Interface("pendingNode.Gen", d.pending.Cluster.Node[o.nodename].Status.Gen).
		Interface("remotesNeedFull", d.remotesNeedFull).
		Interface("patch_sequence", sortGen).
		Msgf("apply remote patch for %s", o.nodename)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
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
