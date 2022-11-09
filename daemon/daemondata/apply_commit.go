package daemondata

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/daemon/hbcache"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type opCommitPending struct {
	done chan<- bool
}

func (o opCommitPending) setDone(b bool) {
	o.done <- b
}

func (o opCommitPending) call(ctx context.Context, d *data) {
	d.counterCmd <- idCommitPending
	d.log.Debug().Msg("opCommitPending")
	if d.hbMsgType == "patch" {
		d.movePendingOpsToPatchQueue()
		d.purgeAppliedPatchQueue()
	} else {
		d.patchQueue = make(patchQueue)
	}
	hbcache.SetLocalGens(d.deepCopyLocalGens())
	d.pubMsgFromNodeDataDiff()

	d.log.Debug().
		Interface("local gens", d.pending.Cluster.Node[d.localNode].Status.Gen).
		Msg("opCommitPending")
	select {
	case <-ctx.Done():
	case o.done <- true:
	}
}

// purgeAppliedPatchQueue purge entries from patch queue that have been merged
// on all peers
func (d *data) purgeAppliedPatchQueue() {
	local := d.localNode
	remoteMinGen := d.gen
	for _, clusterNode := range d.pending.Cluster.Node {
		if gen, ok := clusterNode.Status.Gen[local]; ok {
			if gen < remoteMinGen {
				remoteMinGen = gen
			}
		}
	}
	purged := make([]string, 0)
	queueGens := make([]string, 0)
	queueGen := make([]uint64, 0)
	for genS := range d.patchQueue {
		queueGens = append(queueGens, genS)
		gen, err := strconv.ParseUint(genS, 10, 64)
		if err != nil {
			delete(d.patchQueue, genS)
			purged = append(purged, genS)
			continue
		}
		queueGen = append(queueGen, gen)
		if gen <= remoteMinGen {
			delete(d.patchQueue, genS)
			purged = append(purged, genS)
		}
	}
}

// movePendingOpsToPatchQueue moves pendingOps to patchQueue.
//
// If pendingOps exists:
//
//	increase local gen by 1.
//	new entry for new gen is created in patch queue with pending operations.
//	pending operations are cleared.
func (d *data) movePendingOpsToPatchQueue() {
	if len(d.pendingOps) > 0 {
		d.gen++
		d.patchQueue[strconv.FormatUint(d.gen, 10)] = d.pendingOps
		d.eventCommitPendingOps()
		d.pendingOps = []jsondelta.Operation{}
		d.pending.Cluster.Node[d.localNode].Status.Gen[d.localNode] = d.gen
	}
}

func (d *data) eventCommitPendingOps() {
	fromRootPatch := make(jsondelta.Patch, 0)
	prefixPath := jsondelta.OperationPath{"cluster", "node", d.localNode}
	for _, op := range d.pendingOps {
		fromRootPatch = append(fromRootPatch, jsondelta.Operation{
			OpPath:  append(prefixPath, op.OpPath...),
			OpValue: op.OpValue,
			OpKind:  op.OpKind,
		})
	}
	if eventB, err := json.Marshal(fromRootPatch); err != nil {
		d.log.Error().Err(err).Msg("eventCommitPendingOps Marshal fromRootPatch")
	} else {
		eventId++
		d.bus.Pub(event.Event{
			Kind: "patch",
			ID:   eventId,
			Time: time.Now(),
			Data: eventB,
		})
	}
}

// CommitPending handle a commit of pending changes to T
//
// when pending ops exists
// => increase localhost gen
// => move pending ops to patch queue (to populate next hb message)
// => evict already applied gens from patchQueue
//
// if message mode is not patch
// => patch queue is purged
//
// detected changes from peer data are published for client side event getters
func (t T) CommitPending(ctx context.Context) {
	done := make(chan bool)
	t.cmdC <- opCommitPending{
		done: done,
	}
	select {
	case <-ctx.Done():
		return
	case <-done:
		return
	}
}
