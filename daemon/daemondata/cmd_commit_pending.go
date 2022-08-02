package daemondata

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/daemon/daemonps"
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
	requireFull := d.updateGens()
	if requireFull {
		genChanged := d.updateFromPendingOps()
		if genChanged {
			d.pending.Monitor.Nodes[d.localNode].Gen[d.localNode] = d.gen
		}
		d.resetPatchQueue()
	} else {
		genChanged := d.updateFromPendingOps()
		if genChanged {
			d.pending.Monitor.Nodes[d.localNode].Gen[d.localNode] = d.gen
		}
		d.purgeAppliedPatchQueue()
	}

	cfgDeletes, cfgUpdates := d.getCfgDiff()
	statusDeletes, statusUpdates := d.getStatusDiff()
	smonDeletes, smonUpdates := d.getSmonDiff()

	d.committed = d.pending.DeepCopy()

	for _, cfgDelete := range cfgDeletes {
		daemonps.PubCfgDelete(d.bus, cfgDelete.Path.String(), cfgDelete)
	}
	for _, cfgUpdates := range cfgUpdates {
		daemonps.PubCfgUpdate(d.bus, cfgUpdates.Path.String(), cfgUpdates)
	}
	for _, w := range statusDeletes {
		daemonps.PubInstStatusDelete(d.bus, w.Path.String(), w)
	}
	for _, w := range statusUpdates {
		daemonps.PubInstStatusUpdated(d.bus, w.Path.String(), w)
	}
	for _, w := range smonDeletes {
		daemonps.PubSmonDelete(d.bus, w.Path.String(), w)
	}
	for _, w := range smonUpdates {
		daemonps.PubSmonUpdated(d.bus, w.Path.String(), w)
	}

	d.log.Debug().
		Interface("mergedFromPeer", d.mergedFromPeer).
		Interface("mergedOnPeer", d.mergedOnPeer).
		Interface("remotesNeedFull", d.remotesNeedFull).
		Interface("gens", d.pending.Monitor.Nodes[d.localNode].Gen).
		Msg("opCommitPending")
	select {
	case <-ctx.Done():
	case o.done <- true:
	}
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

// updateFromPendingOps move pendingOps to patchQueue
//
// increase d.gen when pendingOps exists and resets pendingOps
// returns true when d.gen is increased
func (d *data) updateFromPendingOps() bool {
	if len(d.pendingOps) > 0 {
		d.gen++
		d.patchQueue[strconv.FormatUint(d.gen, 10)] = d.pendingOps
		err := d.applyCommitPendingOps()
		if err != nil {
			d.log.Error().Err(err).Msg("updateFromPendingOps failure during applyCommitPendingOps, some pending changes may be lost")
		}
		d.resetPendingOps()
		return true
	}
	return false
}

func (d *data) applyCommitPendingOps() error {
	patch := jsondelta.NewPatchFromOperations(d.pendingOps)
	pendingB, err := json.Marshal(d.pending.Monitor.Nodes[d.localNode])
	if err != nil {
		d.log.Error().Err(err).Msg("can't marshal pending local NodeStatus")
		return err
	}
	if pendingB, err := patch.Apply(pendingB); err != nil {
		d.log.Error().Err(err).Msg("can't patch.Apply on local NodeStatus")
		return err
	} else {
		pendingNode := cluster.NodeStatus{}
		if err := json.Unmarshal(pendingB, &pendingNode); err != nil {
			d.log.Error().Err(err).Msg("can't unmarshal patched local NodeStatus")
			return err
		}
		d.pending.Monitor.Nodes[d.localNode] = pendingNode
		d.eventCommitPendingOps()
	}
	return nil
}

func (d *data) eventCommitPendingOps() {
	fromRootPatch := make(jsondelta.Patch, 0)
	prefixPath := jsondelta.OperationPath{"monitor", "nodes", d.localNode}
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
		var data json.RawMessage = eventB
		daemonps.PubEvent(d.bus, event.Event{
			Kind: "patch",
			ID:   eventId,
			Time: time.Now(),
			Data: &data,
		})
	}
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
