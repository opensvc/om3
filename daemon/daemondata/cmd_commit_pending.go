package daemondata

import (
	"strconv"

	"opensvc.com/opensvc/util/jsondelta"
)

type opCommitPending struct {
	done chan<- bool
}

func (o opCommitPending) call(d *data) {
	d.counterCmd <- idCommitPending
	d.log.Debug().Msg("opCommitPending")
	requireFull := d.updateGens()
	if requireFull {
		d.resetPendingOps()
		d.resetPatchQueue()
	} else {
		genChanged := d.updateFromPendingOps()
		if genChanged {
			d.pending.Monitor.Nodes[d.localNode].Gen[d.localNode] = d.gen
		}
		d.purgeAppliedPatchQueue()
	}

	d.committed = d.pending.DeepCopy()
	d.log.Debug().
		Interface("mergedFromPeer", d.mergedFromPeer).
		Interface("mergedOnPeer", d.mergedOnPeer).
		Interface("remotesNeedFull", d.remotesNeedFull).
		Interface("gens", d.pending.Monitor.Nodes[d.localNode].Gen).
		Msg("opCommitPending")
	o.done <- true
}

// updateGens updates local NodeStatus gens from remotesNeedFull and mergedFromPeer
//
// It returns true when some remote needs full
func (d *data) updateGens() (requireFull bool) {
	for n, needFull := range d.remotesNeedFull {
		if needFull {
			d.mergedOnPeer[n] = 0
			d.mergedFromPeer[n] = 0
			d.remotesNeedFull[n] = false
			requireFull = true
		}
	}
	for n, gen := range d.mergedFromPeer {
		d.pending.Monitor.Nodes[d.localNode].Gen[n] = gen
	}
	return
}

func (d *data) resetPendingOps() {
	d.pendingOps = []jsondelta.Operation{}
}

func (d *data) resetPatchQueue() {
	d.patchQueue = make(patchQueue)
}

func (d *data) purgeAppliedPatchQueue() {
	remoteMinGen := d.gen
	for _, gen := range d.mergedOnPeer {
		if gen < remoteMinGen {
			remoteMinGen = gen
		}
	}
	for genS, _ := range d.patchQueue {
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

// updateFromPendingOps move pendingOps to patchQueue
//
// increase d.gen when pendingOps exists and resets pendingOps
// returns true when d.gen is increased
func (d *data) updateFromPendingOps() bool {
	if len(d.pendingOps) > 0 {
		d.gen++
		d.patchQueue[strconv.FormatUint(d.gen, 10)] = d.pendingOps
		d.resetPendingOps()
		return true
	}
	return false
}

// CommitPending handle a commit of pending changes to T
//
// It maintains local NodeStatus Gens
//   from patch/full/ping operations
//   reset gen values for nodes that needs full hb message
//   increase local gen when pendingOps exists
//
// It moves pendingOps to patchQueue, evict already applied gens from patchQueue
//
// When a remote node requires a full hb message pendingOps and patchQueue are purged
//
// It creates new version of committed Status
func (t T) CommitPending() {
	done := make(chan bool)
	t.cmdC <- opCommitPending{
		done: done,
	}
	<-done
}
