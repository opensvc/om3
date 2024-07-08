package daemondata

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/opensvc/om3/core/hbtype"
)

func (d *data) onReceiveHbMsg(msg *hbtype.Msg) {
	switch msg.Kind {
	case "patch":
		d.setFromPeerMsg(msg.Nodename, msg.Kind, len(msg.Events), msg.Gen)
		if err := d.applyMsgEvents(msg); err != nil {
			d.log.Errorf("apply message %s events from %s gens: %v: %s", msg.Kind, msg.Nodename, msg.Gen, err)
		}
		// cleanup previous applied full info
		delete(d.previousRemoteInfo, msg.Nodename)
		onReceiveQueueOperationTotal.With(prometheus.Labels{"operation": "patch"}).Inc()

	case "full":
		d.setFromPeerMsg(msg.Nodename, msg.Kind, 0, msg.Gen)
		if d.hbGens[d.localNode][msg.Nodename] == msg.Gen[msg.Nodename] {
			// already have most recent version of peer
			d.log.Debugf("onReceiveHbMsg skipped %s from %s gens: %v (already have peer gen applied)", msg.Kind, msg.Nodename, msg.Gen)
			return
		}
		if d.hbGens[d.localNode][msg.Nodename]+uint64(len(msg.Events)) >= msg.Gen[msg.Nodename] {
			// We can apply events instead of full
			previouslyApplied := d.hbGens[d.localNode][msg.Nodename]
			if err := d.applyMsgEvents(msg); err != nil {
				d.log.Errorf("apply message %s events from %s gens: %v (previously applied peer gen %d, local gens: %+v): %s",
					msg.Kind, msg.Nodename, msg.Gen,
					previouslyApplied, d.hbGens, err)
			}
			if d.hbGens[d.localNode][msg.Nodename] == msg.Gen[msg.Nodename] {
				// the events have been applied => node data not needed
				d.log.Debugf("apply message %s events from %s gens: %v succeed (previously applied peer %d now %d)",
					msg.Kind, msg.Nodename, msg.Gen,
					previouslyApplied, msg.Gen[msg.Nodename])
				return
			}
		}
		if err := d.applyNodeData(msg); err != nil {
			d.log.Errorf("apply message %s node data from %s gens: %v: %s", msg.Kind, msg.Nodename, msg.Gen, err)
		}
		onReceiveQueueOperationTotal.With(prometheus.Labels{"operation": "full"}).Inc()
	case "ping":
		d.setFromPeerMsg(msg.Nodename, msg.Kind, 0, msg.Gen)
		// cleanup previous applied full info
		delete(d.previousRemoteInfo, msg.Nodename)
		onReceiveQueueOperationTotal.With(prometheus.Labels{"operation": "ping"}).Inc()
	}
}

func (d *data) setFromPeerMsg(peer, msgType string, length int, gen gens) {
	d.setHbMsgType(peer, msgType)
	d.setHbMsgPatchLength(peer, length)
	d.hbGens[peer] = gen
	if gen[d.localNode] != d.hbGens[d.localNode][d.localNode] {
		d.needMsg = true
	}
	if gen[peer] != d.hbGens[d.localNode][peer] {
		d.needMsg = true
	}
}
