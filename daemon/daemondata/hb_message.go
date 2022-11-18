package daemondata

import (
	"encoding/json"
	"fmt"
	"time"

	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/hbcache"
)

// queueNewHbMsg gets a new hb msg, push it to hb send queue, update msgLocalGen
func (d *data) queueNewHbMsg() error {
	if msg, err := d.getHbMessage(); err != nil {
		return err
	} else {
		msgLocalGen := make(gens)
		for n, gen := range msg.Gen {
			msgLocalGen[n] = gen
		}
		d.msgLocalGen = msgLocalGen
		if d.hbSendQ != nil {
			d.log.Debug().Msgf("queue a new hb message %s gen %v", msg.Kind, msgLocalGen)
			d.hbSendQ <- msg
		}
	}
	return nil
}

// getHbMessage retrieves next hb message to send.
// the message type is result of hbcache.MsgType()
// on success it updates hbcache with latest HbMsgInfo:
//
//	"full", "ping" or len <msg.delta> (patch)
func (d *data) getHbMessage() (hbtype.Msg, error) {
	d.counterCmd <- idGetHbMessage
	d.log.Debug().Msg("getHbMessage")
	d.hbMsgType = hbcache.MsgType()
	var err error
	msg := hbtype.Msg{
		Compat:   d.pending.Cluster.Node[d.localNode].Status.Compat,
		Kind:     d.hbMsgType,
		Nodename: d.localNode,
		Gen:      d.deepCopyLocalGens(),
		Updated:  time.Now(),
	}
	switch d.hbMsgType {
	case "patch":
		delta, err := d.patchQueue.deepCopy()
		if err != nil {
			d.log.Error().Err(err).Msg("can't create delta for hb patch message")
			return msg, err
		}
		msg.Deltas = delta
		hbcache.SetLocalHbMsgInfo(fmt.Sprintf("%d", len(msg.Deltas)))
		return msg, nil
	case "full":
		nodeData := d.pending.Cluster.Node[d.localNode]
		msg.Full = *nodeData.DeepCopy()
		hbcache.SetLocalHbMsgInfo(msg.Kind)
		return msg, nil
	case "ping":
		hbcache.SetLocalHbMsgInfo(msg.Kind)
		return msg, nil
	default:
		err = fmt.Errorf("opGetHbMessage unsupported message type %s", d.hbMsgType)
		d.log.Error().Err(err).Msg("opGetHbMessage")
		return msg, err
	}
}

// deepCopy return clone of p
func (p patchQueue) deepCopy() (result patchQueue, err error) {
	var b []byte
	b, err = json.Marshal(p)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &result)
	return
}

func (d *data) deepCopyLocalGens() gens {
	localGens := make(gens)
	for n, gen := range d.pending.Cluster.Node[d.localNode].Status.Gen {
		localGens[n] = gen
	}
	return localGens
}
