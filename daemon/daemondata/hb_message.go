package daemondata

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/node"
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
		d.log.Debugf("abort queue new hb message (context is done)")
	default:
	}
	if msg, err := d.getHbMessage(); err != nil {
		return err
	} else {
		msgLocalGen := make(node.Gen)
		for n, gen := range msg.Gen {
			msgLocalGen[n] = gen
		}
		d.msgLocalGen = msgLocalGen
		if d.hbSendQ != nil {
			d.log.Debugf("queue a new hb message %s gen %v", msg.Kind, msgLocalGen)
			select {
			case <-ctx.Done():
				d.log.Debugf("abort queue a new hb message %s gen %v (context is done)", msg.Kind, msgLocalGen)
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
	d.log.Debugf("getHbMessage")
	d.setNextMsgType()
	var err error
	msg := hbtype.Msg{
		Compat:    d.clusterData.Cluster.Node[d.localNode].Status.Compat,
		Kind:      d.hbMessageType,
		Nodename:  d.localNode,
		Gen:       d.deepCopyLocalGens(),
		UpdatedAt: time.Now(),
	}
	switch d.hbMessageType {
	case "patch":
		events, err := d.eventQueue.deepCopy()
		if err != nil {
			d.log.Errorf("can't create events for hb patch message: %s", err)
			return msg, err
		}
		msg.Events = events
		d.setHbMsgPatchLength(d.localNode, len(msg.Events))
		return msg, nil
	case "full":
		events, err := d.eventQueue.deepCopy()
		if err != nil {
			d.log.Errorf("can't create events for hb patch message: %s", err)
			return msg, err
		} else {
			msg.Events = events
		}
		nodeData := d.clusterData.Cluster.Node[d.localNode]
		msg.NodeData = *nodeData.DeepCopy()
		d.setHbMsgPatchLength(d.localNode, 0)
		return msg, nil
	case "ping":
		d.setHbMsgPatchLength(d.localNode, 0)
		return msg, nil
	default:
		err = fmt.Errorf("opGetHbMessage unsupported message type %s", d.hbMessageType)
		d.log.Errorf("opGetHbMessage: %s", err)
		return msg, err
	}
}

// deepCopy return clone of p
func (p eventQueue) deepCopy() (result eventQueue, err error) {
	var b []byte
	b, err = json.Marshal(p)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &result)
	return
}

func (d *data) deepCopyLocalGens() node.Gen {
	localGens := make(node.Gen)
	for n, gen := range d.hbGens[d.localNode] {
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
				err := fmt.Errorf("bug: d.hbGens[%s] exists without d.clusterNodes[%s]", node, node)
				d.log.Errorf("setNextMsgType cleanup unexpected hb gens %s: %s", node, err)
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
			d.log.Infof("hb message type change %s -> %s (gen:%d, need full:[%v], gens:%v)",
				d.hbMessageType, messageType, d.gen, strings.Join(remoteNeedFull, ", "), d.hbGens)
		} else {
			d.log.Infof("hb message type change %s -> %s (gen:%d, gens:%v)",
				d.hbMessageType, messageType, d.gen, d.hbGens)
		}
		d.hbMessageType = messageType
		d.setHbMsgType(d.localNode, messageType)
	}
	return
}

func (o opSetHBSendQ) call(ctx context.Context, d *data) error {
	d.hbSendQ = o.hbSendQ
	return nil
}
