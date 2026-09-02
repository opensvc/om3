package daemonsubsystem

import (
	"time"

	"github.com/fatih/color"
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

		SecretVersion struct {
			Main      uint64 `json:"main"`
			Alternate uint64 `json:"alt"`
		} `json:"secret_version"`

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
		Desc      string `json:"desc"`
		IsBeating bool   `json:"is_beating"`

		// ChangedAt is the time of IsBeating value changed
		ChangedAt time.Time `json:"changed_at"`

		// LastBeatingAt is the last beating time
		LastBeatingAt time.Time `json:"last_beating_at"`
	}

	HeartbeatStreamPeerStatusTable      []HeartbeatStreamPeerStatusTableEntry
	HeartbeatStreamPeerStatusTableEntry struct {
		Node string `json:"node"`
		Peer string `json:"peer"`
		Status
		Type   string  `json:"type"`
		Alerts []Alert `json:"alerts"`
		HeartbeatStreamPeerStatus
		IsSingleNode bool `json:"-"`
	}
)

func (t HeartbeatStreamPeerStatusTableEntry) Unstructured() map[string]any {
	stateText := t.Status.State
	if stateText == "" {
		stateText = "unknown"
	}
	var beatingText string
	if t.IsSingleNode {
		beatingText = "beating"
	} else {
		if t.IsBeating {
			beatingText = "beating"
		} else {
			beatingText = "stale"
		}
	}

	var stateIcon string
	switch t.Status.State {
	case "running":
		stateIcon = color.New(color.FgGreen).Sprint("O")
	case "stopped", "failed":
		stateIcon = color.New(color.FgRed).Sprint("X")
	case "warning":
		stateIcon = color.New(color.FgYellow).Sprint("!")
	default:
		stateIcon = color.New(color.FgHiBlack).Sprint("?")
	}

	var beatingIcon string
	if t.IsSingleNode || t.IsBeating {
		beatingIcon = color.New(color.FgGreen).Sprint("O")
	} else {
		beatingIcon = color.New(color.FgRed).Sprint("X")
	}

	peer := t.Peer
	if peer == "" {
		peer = "N/A"
	}
	desc := t.Desc
	if desc == "" {
		desc = "N/A"
	}

	return map[string]any{
		"node":          t.Node,
		"peer":          peer,
		"type":          t.Type,
		"alerts":        t.Alerts,
		"id":            t.Status.ID,
		"state":         stateText,
		"state_icon":    stateIcon,
		"state_text":    stateText,
		"configured_at": t.Status.ConfiguredAt,
		"updated_at":    t.Status.UpdatedAt,
		"created_at":    t.Status.CreatedAt,
		"desc":          desc,
		// The times are values, not text: the renderer prints them to
		// the second, and the datasets keep the nanoseconds the type
		// carries. The three above are already handed over this way.
		"changed_at":      t.ChangedAt,
		"last_beating_at": t.LastBeatingAt,
		"is_beating":      t.IsBeating,
		"beating":         beatingText,
		"beating_icon":    beatingIcon,
	}
}

func (c *Heartbeat) Table(nodeName string, isSingleNode bool) HeartbeatStreamPeerStatusTable {
	table := make(HeartbeatStreamPeerStatusTable, 0)
	for _, stream := range c.Streams {
		if len(stream.Peers) > 0 {
			for peerName, peerStatus := range stream.Peers {
				table = append(table, HeartbeatStreamPeerStatusTableEntry{
					Node:                      nodeName,
					Peer:                      peerName,
					Status:                    stream.Status,
					Type:                      stream.Type,
					Alerts:                    append([]Alert{}, stream.Alerts...),
					HeartbeatStreamPeerStatus: peerStatus,
					IsSingleNode:              isSingleNode,
				})
			}
		} else {
			table = append(table, HeartbeatStreamPeerStatusTableEntry{
				Node:                      nodeName,
				Peer:                      "",
				Status:                    stream.Status,
				Type:                      stream.Type,
				Alerts:                    append([]Alert{}, stream.Alerts...),
				HeartbeatStreamPeerStatus: HeartbeatStreamPeerStatus{},
				IsSingleNode:              isSingleNode,
			})
		}
	}
	return table
}

// DeepCopy returns a copy of the heartbeat state sharing nothing with it.
//
// The collections come out non-nil even when they went in nil, which is
// not what a copy would normally do. It is deliberate: this is the
// daemon's publication path, none of these fields carry omitempty, and
// a nil serializes as null where an empty one serializes as []. hbctrl
// builds Alerts with append to a nil slice, so an alert-free stream
// arrives here nil and would reach an api client as null without this.
// TestDeepCopyKeepsCollectionsNonNil pins it.
func (c *Heartbeat) DeepCopy() *Heartbeat {
	n := *c
	n.LastMessages = append([]HeartbeatLastMessage{}, c.LastMessages...)
	n.Streams = make([]HeartbeatStream, len(c.Streams))
	for i, stream := range c.Streams {
		n.Streams[i] = *stream.DeepCopy()
	}
	return &n
}

// DeepCopy returns a copy of the stream state sharing nothing with it.
// Its collections come out non-nil, see Heartbeat.DeepCopy.
func (c *HeartbeatStream) DeepCopy() *HeartbeatStream {
	n := *c
	n.Peers = make(map[string]HeartbeatStreamPeerStatus, len(c.Peers))
	for nodename, peerStatus := range c.Peers {
		n.Peers[nodename] = peerStatus
	}
	n.Alerts = append([]Alert{}, c.Alerts...)
	return &n
}
