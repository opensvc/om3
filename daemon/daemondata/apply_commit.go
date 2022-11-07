package daemondata

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"opensvc.com/opensvc/core/event"
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
	d.movePendingOpsToPatchQueue()
	if d.clearMergedGenFromRemotesNeedFull() {
		// one remote needs full, we can reset the patch queue because next msg type will be 'full'
		d.patchQueue = make(patchQueue)
	} else {
		d.purgeAppliedPatchQueue()
	}

	d.pubMsgFromNodeDataDiff()

	d.log.Debug().
		Interface("mergedFromPeer", d.mergedFromPeer).
		Interface("mergedOnPeer", d.mergedOnPeer).
		Interface("remotesNeedFull", d.remotesNeedFull).
		Interface("gens", d.pending.Cluster.Node[d.localNode].Status.Gen).
		Msg("opCommitPending")
	select {
	case <-ctx.Done():
	case o.done <- true:
	}
}

// updateLocalNodeMergedGens updates cluster.nodes.<localhost>.gen.<remoteX>.<genX>
// from and mergedFromPeer
func (d *data) updateLocalNodeMergedGens() {
	for n, gen := range d.mergedFromPeer {
		d.pending.Cluster.Node[d.localNode].Status.Gen[n] = gen
	}
	return
}

// clearMergedGenFromRemotesNeedFull clears the merged state information from
// remotesNeedFull information. It returns true when a remote needs full
func (d *data) clearMergedGenFromRemotesNeedFull() (requireFull bool) {
	for n, needFull := range d.remotesNeedFull {
		if needFull {
			d.mergedOnPeer[n] = 0
			d.mergedFromPeer[n] = 0
			d.remotesNeedFull[n] = false
			requireFull = true
		}
	}
	return requireFull
}

// purgeAppliedPatchQueue delete from patch queue entries that have been
// merged on all peers
func (d *data) purgeAppliedPatchQueue() {
	remoteMinGen := d.gen
	for _, gen := range d.mergedOnPeer {
		if gen < remoteMinGen {
			remoteMinGen = gen
		}
	}
	for genS := range d.patchQueue {
		gen, err := strconv.ParseUint(genS, 10, 64)
		if err != nil {
			delete(d.patchQueue, genS)
			continue
		}
		if gen <= remoteMinGen {
			delete(d.patchQueue, genS)
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
		msgbus.PubEvent(d.bus, event.Event{
			Kind: "patch",
			ID:   eventId,
			Time: time.Now(),
			Data: eventB,
		})
	}
}

// CommitPending handle a commit of pending changes to T
//
// It maintains local NodeStatus Gens
//
//	from patch/full/ping operations
//	reset gen values for nodes that needs full hb message
//	increase local gen when pendingOps exists
//
// # It moves pendingOps to patchQueue, evict already applied gens from patchQueue
//
// # When a remote node requires a full hb message pendingOps and patchQueue are purged
//
// It creates new version of previous Status
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
