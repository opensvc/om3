package daemondata

import (
	"strconv"

	"github.com/opensvc/om3/v3/core/event"
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
	if len(d.pendingEvs) > 0 {
		changes = true
		d.gen++
		d.hbGens[d.localNode][d.localNode] = d.gen
		if _, ok := d.clusterData.Cluster.Node[d.localNode]; !ok {
			d.log.Warnf("commitPendingOps -> d.clusterData.Cluster.Node[%s] is absent", d.localNode)
		} else {
			d.clusterData.Cluster.Node[d.localNode].Status.Gen[d.localNode] = d.gen
		}
	}
	switch d.hbMessageType {
	case "patch", "full":
		if changes {
			// add new eventQueue entry created for gen in event queue with events
			d.eventQueue[strconv.FormatUint(d.gen, 10)] = d.pendingEvs
			d.pendingEvs = []event.Event{}
		}
		d.purgeAppliedPatchQueue()
	default:
		d.pendingEvs = []event.Event{}
		d.eventQueue = make(map[string][]event.Event)
	}
	d.hbGens[d.localNode][d.localNode] = d.gen
	return
}

// purgeAppliedPatchQueue purge entries from patch queue that have been merged
// on all peers
func (d *data) purgeAppliedPatchQueue() {
	local := d.localNode
	peerMinGen := d.gen
	for peer, peerGens := range d.hbGens {
		if peer == local {
			continue
		}
		if peerGen := peerGens[local]; peerGen != 0 && peerGen < peerMinGen {
			peerMinGen = peerGen
		}
	}
	for genS := range d.eventQueue {
		gen, err := strconv.ParseUint(genS, 10, 64)
		if err != nil {
			delete(d.eventQueue, genS)
			continue
		}
		if gen <= peerMinGen {
			delete(d.eventQueue, genS)
		}
	}
}
