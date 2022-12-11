package daemondata

import (
	"encoding/json"
	"strconv"

	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

// commitPendingOps manage patch queue from current pending ops
//
//	 when hb mode is patch
//	  if pending ops exists
//	 	  publish patch events from pending ops
//	 	  increase gen and create new patch queue entry from pending ops
//	 	  drop pending ops
//	   drop patch queue entries that have been applied on all peers
//
//	  when hb mode is not patch
//		   increase gen when pending ops exists
//		   drop pending ops
//		   drop patch queue
func (d *data) commitPendingOps() (changes bool) {
	d.counterCmd <- idCommitPending
	d.log.Debug().Msg("commitPendingOps")
	if len(d.pendingOps) > 0 {
		changes = true
	}
	if d.hbMsgType == "patch" {
		d.movePendingOpsToPatchQueue()
		d.purgeAppliedPatchQueue()
	} else {
		if changes {
			// increase gen (we have changes that need to be sent before switching to patch mode)
			d.gen++
			d.pending.Cluster.Node[d.localNode].Status.Gen[d.localNode] = d.gen
		}
		d.pendingOps = []jsondelta.Operation{}
		d.patchQueue = make(patchQueue)
	}
	d.hbGens[d.localNode] = d.deepCopyLocalGens()

	d.log.Debug().
		Interface("local gens", d.pending.Cluster.Node[d.localNode].Status.Gen).
		Msg("commitPendingOps")
	return
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
		d.bus.Pub(msgbus.DataUpdated{RawMessage: eventB}, labelLocalNode)
	}
}
