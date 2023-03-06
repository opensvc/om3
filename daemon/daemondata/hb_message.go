package daemondata

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/core/hbtype"
)

type (
	opSetHBSendQ struct {
		errC
		hbSendQ chan<- hbtype.Msg
	}
)

// SetHBSendQ defines daemondata hbSendQ. The hbSendQ is used during queueNewHbMsg
// to push heartbeat message to this queue, see usage example for hb msgToTx multiplexer
// Example:
//
//	msgC := make(chan hbtype.Msg)
//	SetHBSendQ(msgC) // inform daemondata we are listening on this queue
//	defer SetHBSendQ(nil) // inform daemondata, we are not anymore reading on this queue
//	for {
//	   select {
//	   case msg := <- msgC:
//	      ...
//	   case <-ctx.Done():
//	      return
//	   }
//	}
func (t T) SetHBSendQ(hbSendQ chan<- hbtype.Msg) error {
	err := make(chan error, 1)
	op := opSetHBSendQ{hbSendQ: hbSendQ, errC: err}
	t.cmdC <- op
	return <-err
}

// queueNewHbMsg gets a new hb msg, push it to hb send queue, update msgLocalGen
//
// It aborts on done context
func (d *data) queueNewHbMsg(ctx context.Context) error {
	select {
	case <-ctx.Done():
		d.log.Debug().Msg("abort queue new hb message (context is done)")
	default:
	}
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
			select {
			case <-ctx.Done():
				d.log.Debug().Msgf("abort queue a new hb message %s gen %v (context is done)", msg.Kind, msgLocalGen)
			case d.hbSendQ <- msg:
			}
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
	d.setNextMsgType()
	var err error
	msg := hbtype.Msg{
		Compat:   d.pending.Cluster.Node[d.localNode].Status.Compat,
		Kind:     d.hbMessageType,
		Nodename: d.localNode,
		Gen:      d.deepCopyLocalGens(),
		Updated:  time.Now(),
	}
	switch d.hbMessageType {
	case "patch":
		delta, err := d.patchQueue.deepCopy()
		if err != nil {
			d.log.Error().Err(err).Msg("can't create delta for hb patch message")
			return msg, err
		}
		msg.Deltas = delta
		d.setHbMsgMode(d.localNode, fmt.Sprintf("%d", len(msg.Deltas)))
		return msg, nil
	case "full":
		nodeData := d.pending.Cluster.Node[d.localNode]
		msg.Full = *nodeData.DeepCopy()
		d.setHbMsgMode(d.localNode, msg.Kind)
		return msg, nil
	case "ping":
		d.setHbMsgMode(d.localNode, msg.Kind)
		return msg, nil
	default:
		err = fmt.Errorf("opGetHbMessage unsupported message type %s", d.hbMessageType)
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

func (d *data) setNextMsgType() {
	var messageType string
	var remoteNeedFull []string
	if d.hbMessageType == "undef" {
		// init
		messageType = "ping"
	} else if len(d.hbGens) <= 1 || d.hbMessageType == "undef" {
		// no hb msg received yet
		messageType = "ping"
	} else {
		for node, gen := range d.hbGens {
			if node == d.localNode {
				continue
			}
			if _, ok := d.clusterNodes[node]; !ok {
				// unexpected hb gens from non cluster member, so it doesn't need full
				d.log.Warn().Msgf("evict unexpected hb gens %s: not a cluster node", node)
				delete(d.hbGens, node)
				continue
			}
			if gen[d.localNode] == 0 {
				remoteNeedFull = append(remoteNeedFull, node)
			} else if d.hbMessageType == "full" && gen[d.localNode] < d.gen {
				// stay in full, peers not ready for patch
				remoteNeedFull = append(remoteNeedFull, node)
			}

		}
		if len(remoteNeedFull) > 0 || d.hbMessageType == "ping" {
			messageType = "full"
		} else {
			messageType = "patch"
		}
	}
	if messageType != d.hbMessageType {
		if messageType == "full" && len(remoteNeedFull) > 0 {
			sort.Strings(remoteNeedFull)
			d.log.Info().Msgf("hb message type change %s -> %s (gen:%d, need full:[%v], gens:%v)",
				d.hbMessageType, messageType, d.gen, strings.Join(remoteNeedFull, ", "), d.hbGens)
		} else {
			d.log.Info().Msgf("hb message type change %s -> %s (gen:%d, gens:%v)",
				d.hbMessageType, messageType, d.gen, d.hbGens)
		}
		d.hbMessageType = messageType
		d.setHbMsgType(d.localNode, messageType)
	}
	return
}

func (o opSetHBSendQ) call(ctx context.Context, d *data) error {
	d.counterCmd <- idSetHBSendQ
	d.hbSendQ = o.hbSendQ
	return nil
}
