package daemondata

import (
	"encoding/json"
	"strings"

	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/util/timestamp"
)

type opGetHbMessage struct {
	data chan<- []byte
}

var (
	lastMessageType = "undef"
)

func (o opGetHbMessage) setDataByte(err error) {
	o.data <- []byte{}
}

// GetHbMessage provides the hb message to send on remotes
//
// It decides which type of message is needed
func (t T) GetHbMessage() []byte {
	b := make(chan []byte)
	t.cmdC <- opGetHbMessage{
		data: b,
	}
	return <-b
}

func (o opGetHbMessage) call(d *data) {
	d.counterCmd <- idGetHbMessage
	d.log.Debug().Msg("opGetHbMessage")
	var nextMessageType string
	var remoteNeedFull []string
	for remote, gen := range d.mergedOnPeer {
		if gen == 0 {
			nextMessageType = "full"
			remoteNeedFull = append(remoteNeedFull, remote)
		}
	}
	if nextMessageType == "" {
		if len(d.mergedFromPeer) > 0 {
			nextMessageType = "patch"
		} else {
			nextMessageType = "ping"
		}
	}
	if nextMessageType != lastMessageType {
		if nextMessageType == "full" {
			d.log.Info().Msgf("hb message full needed for remotes %s", strings.Join(remoteNeedFull, ", "))
		}
		d.log.Info().Msgf("hb message type change %s -> %s", lastMessageType, nextMessageType)
	}
	lastMessageType = nextMessageType
	var msg interface{}
	switch nextMessageType {
	case "patch":
		b, err := json.Marshal(d.patchQueue)
		if err != nil {
			d.log.Error().Err(err).Msg("opGetHbMessage marshal patch queue")
			o.data <- []byte{}
			return
		}
		delta := patchQueue{}
		if err := json.Unmarshal(b, &delta); err != nil {
			d.log.Error().Err(err).Msg("opGetHbMessage unmarshal patch queue")
			o.data <- []byte{}
			return
		}
		msg = hbtype.MsgPatch{
			Kind:     "patch",
			Compat:   d.committed.Monitor.Nodes[d.localNode].Compat,
			Gen:      d.getGens(),
			Updated:  timestamp.Now(),
			Deltas:   delta,
			Nodename: d.localNode,
		}
	case "full":
		msg = hbtype.MsgFull{
			Kind:     "full",
			Compat:   d.committed.Monitor.Nodes[d.localNode].Compat,
			Gen:      d.getGens(),
			Updated:  timestamp.Now(),
			Full:     *GetNodeStatus(d.committed, d.localNode).DeepCopy(),
			Nodename: d.localNode,
		}
	case "ping":
		msg = hbtype.MsgPing{
			Kind:     "ping",
			Nodename: d.localNode,
			Gen:      d.getGens(),
		}
	default:
		d.log.Error().Msgf("opGetHbMessage unexpected message type: %s", nextMessageType)
		return
	}
	if b, err := json.Marshal(msg); err != nil {
		o.data <- []byte{}
	} else {
		o.data <- b
	}
}

func (d *data) getGens() gens {
	localGens := make(gens)
	for n, gen := range d.mergedFromPeer {
		localGens[n] = gen
	}
	localGens[d.localNode] = d.gen
	return localGens
}
