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
	"opensvc.com/opensvc/daemon/hbcache"
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
	d.log.Debug().Msgf("apply patch %s", o.nodename)
	var (
		pendingB []byte
		err      error
		changes  bool
		sortGen  []uint64
		needFull bool
	)
	local := d.localNode
	remote := o.nodename
	defer func() {
		if needFull {
			d.pending.Cluster.Node[local].Status.Gen[remote] = 0
		}
		d.log.Debug().
			Interface("msg gens", o.msg.Gen).
			Interface("patch_sequence", sortGen).
			Msgf("apply patch %s gen %v", remote, o.msg.Gen[remote])

		hbcache.SetLocalGens(d.pending.Cluster.Node[local].Status.Gen)
		o.err <- err
		return
	}()
	if d.pending.Cluster.Node[local].Status.Gen[remote] == 0 {
		d.log.Debug().Msgf("apply patch skipped %s gen %v (wait full)", remote, o.msg.Gen[remote])
		return
	}
	pendingRemote, ok := d.pending.Cluster.Node[remote]
	if !ok {
		panic("apply patch on nil cluster node data for " + remote)
	}
	pendingNodeGen := pendingRemote.Status.Gen[remote]
	if o.msg.Gen[remote] < pendingNodeGen {
		d.log.Info().Msgf("apply patch skipped %s gen %v (ask full from restarted remote)", remote, o.msg.Gen[remote])
		needFull = true
		return
	}
	if len(o.msg.Deltas) == 0 && o.msg.Gen[remote] > pendingRemote.Status.Gen[remote] {
		d.log.Info().Msgf("apply patch skipped %s gen %v (ask full from empty patch)", remote, o.msg.Gen[remote])
		needFull = true
		return
	}
	deltas := o.msg.Deltas
	for k := range deltas {
		gen, err1 := strconv.ParseUint(k, 10, 64)
		if err1 != nil {
			continue
		}
		sortGen = append(sortGen, gen)
	}
	sort.Slice(sortGen, func(i, j int) bool { return sortGen[i] < sortGen[j] })
	d.log.Debug().Msgf("apply patch sequence %s %v", remote, sortGen)
	parentPath := jsondelta.OperationPath{"cluster", "node", remote}
	for _, gen := range sortGen {
		genS := strconv.FormatUint(gen, 10)
		if gen <= pendingNodeGen {
			continue
		}
		if gen > pendingNodeGen+1 {
			msg := fmt.Sprintf("apply patch %s found broken sequence on gen %d from sequence %v, current known gen %d", remote, gen, sortGen, pendingNodeGen)
			err = errors.New(msg)
			d.log.Info().Err(err).Msgf("apply patch need full %s", remote)
			needFull = true
			return
		}
		if !changes {
			// initiate pendingB
			pendingB, err = json.Marshal(pendingRemote)
			if err != nil {
				d.log.Error().Err(err).Msgf("Marshal pendingRemote %s", remote)
				return
			}
			changes = true
		}
		patch := jsondelta.NewPatchFromOperations(deltas[genS])
		d.log.Debug().Msgf("apply patch %s delta gen %s", remote, genS)
		pendingB, err = patch.Apply(pendingB)
		if err != nil {
			d.log.Info().Err(err).Msgf("apply patch %s delta %s (ask full)", remote, genS)
			needFull = true
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
		var b []byte
		if b, err = json.Marshal(absolutePatch); err != nil {
			d.log.Error().Err(err).Msgf("Marshal absolutePatch %s", remote)
			return
		} else {
			eventId++
			msgbus.PubEvent(d.bus, event.Event{
				Kind: "patch",
				ID:   eventId,
				Time: time.Now(),
				Data: b,
			})
		}
		pendingNodeGen = gen
	}
	if changes {
		// patches has been applied get update pendingRemote
		pendingRemote = cluster.NodeData{}
		if err = json.Unmarshal(pendingB, &pendingRemote); err != nil {
			d.log.Error().Err(err).Msgf("Unmarshal pendingB %s", remote)
			return
		}
	}
	pendingRemote.Status.Gen = o.msg.Gen
	d.pending.Cluster.Node[remote] = pendingRemote
	d.pending.Cluster.Node[local].Status.Gen[remote] = o.msg.Gen[remote]
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
