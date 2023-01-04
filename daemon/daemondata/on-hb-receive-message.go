package daemondata

import (
	"fmt"

	"opensvc.com/opensvc/core/hbtype"
)

func (d *data) onReceiveHbMsg(msg *hbtype.Msg) {
	switch msg.Kind {
	case "patch":
		mode := fmt.Sprintf("%d", len(msg.Deltas))
		d.setFromPeerMsg(msg.Nodename, msg.Kind, mode, msg.Gen)
		if err := d.applyPatch(msg); err != nil {
			d.log.Error().Err(err).Msgf("ApplyPatch %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
		} else {
			d.hbGens[d.localNode] = d.deepCopyLocalGens()
		}
	case "full":
		mode := msg.Kind
		d.setFromPeerMsg(msg.Nodename, msg.Kind, mode, msg.Gen)
		if err := d.applyFull(msg); err != nil {
			d.log.Error().Err(err).Msgf("applyFull %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
		} else {
			d.hbGens[d.localNode] = d.deepCopyLocalGens()
		}
	case "ping":
		mode := msg.Kind
		d.setFromPeerMsg(msg.Nodename, msg.Kind, mode, msg.Gen)
	}
}

func (d *data) setFromPeerMsg(peer string, msgType, mode string, gen gens) {
	d.setMsgType(peer, msgType)
	d.setMsgMode(peer, mode)
	d.hbGens[peer] = gen
	if gen[d.localNode] != d.hbGens[d.localNode][d.localNode] {
		d.needMsg = true
	}
	if gen[peer] != d.hbGens[d.localNode][peer] {
		d.needMsg = true
	}
}
