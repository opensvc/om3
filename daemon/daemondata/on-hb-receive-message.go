package daemondata

import (
	"fmt"

	"github.com/opensvc/om3/core/hbtype"
)

func (d *data) onReceiveHbMsg(msg *hbtype.Msg) {
	switch msg.Kind {
	case "patch":
		mode := fmt.Sprintf("%d", len(msg.Events))
		d.setFromPeerMsg(msg.Nodename, msg.Kind, mode, msg.Gen)
		if err := d.applyMsgEvents(msg); err != nil {
			d.log.Error().Err(err).Msgf("apply message %s events from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
		}
	case "full":
		mode := msg.Kind
		d.setFromPeerMsg(msg.Nodename, msg.Kind, mode, msg.Gen)
		if d.hbGens[d.localNode][msg.Nodename] == msg.Gen[msg.Nodename] {
			// already have most recent version of peer
			d.log.Debug().Msgf("onReceiveHbMsg skipped %s from %s gens: %v (already have peer gen applied)", msg.Kind, msg.Nodename, msg.Gen)
			return
		}
		if d.hbGens[d.localNode][msg.Nodename]+uint64(len(msg.Events)) >= msg.Gen[msg.Nodename] {
			// We can apply events instead of full
			previouslyApplied := d.hbGens[d.localNode][msg.Nodename]
			if err := d.applyMsgEvents(msg); err != nil {
				d.log.Error().Err(err).Msgf("apply message %s events from %s gens: %v (previously applied peer gen %d, local gens: %+v)",
					msg.Kind, msg.Nodename, msg.Gen,
					previouslyApplied, d.hbGens)
			}
			if d.hbGens[d.localNode][msg.Nodename] == msg.Gen[msg.Nodename] {
				// the events have been applied => node data not needed
				d.log.Debug().Msgf("apply message %s events from %s gens: %v succeed (previously applied peer %d now %d)",
					msg.Kind, msg.Nodename, msg.Gen,
					previouslyApplied, msg.Gen[msg.Nodename])
				return
			}
		}
		if err := d.applyNodeData(msg); err != nil {
			d.log.Error().Err(err).Msgf("apply message %s node data from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
		}
	case "ping":
		mode := msg.Kind
		d.setFromPeerMsg(msg.Nodename, msg.Kind, mode, msg.Gen)
	}
}

func (d *data) setFromPeerMsg(peer string, msgType, mode string, gen gens) {
	d.setHbMsgType(peer, msgType)
	d.setHbMsgMode(peer, mode)
	d.hbGens[peer] = gen
	if gen[d.localNode] != d.hbGens[d.localNode][d.localNode] {
		d.needMsg = true
	}
	if gen[peer] != d.hbGens[d.localNode][peer] {
		d.needMsg = true
	}
}
