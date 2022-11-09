package daemondata

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/hbcache"
)

type opGetHbMessageResponse struct {
	msg hbtype.Msg
	err error
}

type opGetHbMessage struct {
	msgType  string
	response chan<- opGetHbMessageResponse
}

func (o opGetHbMessage) setDataByte(err error) {
	o.response <- opGetHbMessageResponse{}
}

// GetHbMessage provides the hb message to send on remotes
//
// retrieve next hb message to send.
// the message type is result of hbcache.MsgType()
// on success it updates hbcache with latest HbMsgInfo:
//
//	"full", "ping" or len <msg.delta> (patch)
func (t T) GetHbMessage(ctx context.Context) (msg hbtype.Msg, err error) {
	msgType := hbcache.MsgType()
	responseC := make(chan opGetHbMessageResponse)
	t.cmdC <- opGetHbMessage{
		msgType:  msgType,
		response: responseC,
	}
	select {
	case <-ctx.Done():
		return
	case response := <-responseC:
		err = response.err
		if err != nil {
			return
		}
		msg = response.msg
		if msgType == "patch" {
			hbcache.SetLocalHbMsgInfo(fmt.Sprintf("%d", len(msg.Deltas)))
		} else {
			hbcache.SetLocalHbMsgInfo(msg.Kind)
		}
		return
	}
}

func (o opGetHbMessage) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetHbMessage
	d.log.Debug().Msg("opGetHbMessage")
	d.hbMsgType = hbcache.MsgType()
	var err error
	msg := hbtype.Msg{
		Compat:   d.pending.Cluster.Node[d.localNode].Status.Compat,
		Kind:     d.hbMsgType,
		Nodename: d.localNode,
		Gen:      d.deepCopyLocalGens(),
		Updated:  time.Now(),
	}
	defer func() {
		// release cmd bus
		go func() {
			o.response <- opGetHbMessageResponse{
				msg: msg,
				err: err,
			}
		}()
	}()
	switch d.hbMsgType {
	case "patch":
		delta, err := d.patchQueue.deepCopy()
		if err != nil {
			d.log.Error().Err(err).Msg("can't create delta for hb patch message")
			return
		}
		msg.Deltas = delta
		return
	case "full":
		nodeData := d.pending.Cluster.Node[d.localNode]
		msg.Full = *nodeData.DeepCopy()
		return
	case "ping":
		return
	default:
		err = fmt.Errorf("opGetHbMessage unsupported message type %s", d.hbMsgType)
		d.log.Error().Err(err).Msg("opGetHbMessage")
		return
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
