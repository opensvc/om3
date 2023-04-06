package daemondata

import (
	"strconv"

	"github.com/opensvc/om3/core/event"
)

// commitPendingOps manage patch queue from current clusterData ops
//
//	  if clusterData ops exists (changes)
//	     increase gen
//	     refresh local node gen
//
//		 when hb mode is patch
//		  if changes
//		 	  create new event queue entry from clusterData events
//		 	  drop clusterData pending events
//		   drop event queue entries that have been applied on all peers
//
//		  when hb mode is not patch
//			   drop clusterData pending events
//			   drop event queue
func (d *data) commitPendingOps() (changes bool) {
	d.log.Debug().Msg("commitPendingOps")
	if len(d.pendingEvs) > 0 {
		changes = true
		d.gen++
		d.hbGens[d.localNode][d.localNode] = d.gen
		d.clusterData.Cluster.Node[d.localNode].Status.Gen[d.localNode] = d.gen
	}
	if d.hbMessageType == "patch" {
		if changes {
			// add new eventQueue entry created for gen in event queue with events
			d.eventQueue[strconv.FormatUint(d.gen, 10)] = d.pendingEvs
			d.pendingEvs = []event.Event{}
		}
		d.purgeAppliedPatchQueue()
	} else {
		d.pendingEvs = []event.Event{}
		d.eventQueue = make(map[string][]event.Event)
	}
	d.hbGens[d.localNode][d.localNode] = d.gen

	d.log.Debug().
		Interface("local gens", d.clusterData.Cluster.Node[d.localNode].Status.Gen).
		Msg("commitPendingOps")
	return
}

// purgeAppliedPatchQueue purge entries from patch queue that have been merged
// on all peers
func (d *data) purgeAppliedPatchQueue() {
	local := d.localNode
	remoteMinGen := d.gen
	for _, clusterNode := range d.clusterData.Cluster.Node {
		if gen, ok := clusterNode.Status.Gen[local]; ok {
			if gen < remoteMinGen {
				remoteMinGen = gen
			}
		}
	}
	purged := make([]string, 0)
	queueGens := make([]string, 0)
	queueGen := make([]uint64, 0)
	for genS := range d.eventQueue {
		queueGens = append(queueGens, genS)
		gen, err := strconv.ParseUint(genS, 10, 64)
		if err != nil {
			delete(d.eventQueue, genS)
			purged = append(purged, genS)
			continue
		}
		queueGen = append(queueGen, gen)
		if gen <= remoteMinGen {
			delete(d.eventQueue, genS)
			purged = append(purged, genS)
		}
	}
}
