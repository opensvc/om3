package daemonsubsystem

import (
	"time"
)

type (
	// Heartbeat defines model for Daemon Heartbeat subsystem.
	Heartbeat struct {
		//TODO: add ctrl, msgToTx, msgToRx ?

		// LastMessages is the last messages details received from  cluster
		// peer nodes plus the last message details localhost has sent.
		LastMessages []HeartbeatLastMessage `json:"last_messages"`

		// LastMessage is the last message sent from localhost
		LastMessage HeartbeatLastMessage `json:"last_message"`

		// Streams list of daemon heartbeat streams
		Streams []HeartbeatStream `json:"streams"`

		UpdatedAt time.Time `json:"updated_at"`
	}

	HeartbeatLastMessage struct {
		// From a cluster node
		From string `json:"from"`

		// PatchLength the patch queue length when type is patch, else it is 0
		PatchLength int `json:"patch_length"`

		// Type the heartbeat message type used by node
		Type string `json:"type"`
	}

	// HeartbeatStream defines model for Heartbeat Stream (a sending or receiving heartbeat).
	//   - a sending daemon heartbeat is responsible for sending node dataset
	//     changes to peers
	//   - a receiving daemon heartbeat is responsible for receiving node dataset
	//     changes from peers
	HeartbeatStream struct {
		Status

		// Type of heartbeat stream (unicast, multicast, ...)
		Type string `json:"type"`

		// Peers map of peer names to daemon hb stream peer status
		Peers map[string]HeartbeatStreamPeerStatus `json:"peers"`

		Alerts []Alert `json:"alerts"`
	}

	// HeartbeatStreamPeerStatus status of the communication with a specific peer node.
	HeartbeatStreamPeerStatus struct {
		Desc      string    `json:"desc"`
		IsBeating bool      `json:"is_beating"`
		LastAt    time.Time `json:"last_at"`
	}
)

func (c *Heartbeat) DeepCopy() *Heartbeat {
	streams := make([]HeartbeatStream, 0, len(c.Streams))
	for _, stream := range c.Streams {
		streams = append(streams, *stream.DeepCopy())
	}
	return &Heartbeat{
		LastMessages: append([]HeartbeatLastMessage{}, c.LastMessages...),
		LastMessage:  c.LastMessage,
		Streams:      append([]HeartbeatStream{}, streams...),
	}
}

func (c *HeartbeatStream) DeepCopy() *HeartbeatStream {
	peers := make(map[string]HeartbeatStreamPeerStatus)
	for k, v := range c.Peers {
		peers[k] = v
	}
	return &HeartbeatStream{
		Status: c.Status,
		Type:   c.Type,
		Peers:  peers,
		Alerts: append([]Alert{}, c.Alerts...),
	}
}
