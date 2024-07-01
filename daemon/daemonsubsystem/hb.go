package daemonsubsystem

import "time"

type (
	Hb struct {
		Streams      []HeartbeatStream `json:"streams"`
		LastMessages []HbLastMessage   `json:"last_messages"`
	}

	// HeartbeatStream describes one OpenSVC daemon heartbeat thread,
	// which is responsible for sending or receiving the node DataSet
	// changes to or from peer nodes.
	HeartbeatStream struct {
		DaemonSubsystemStatus

		// Type is the heartbeat type example: unicast, ...
		Type string `json:"type"`

		Peers map[string]HeartbeatPeerStatus `json:"peers"`
	}

	// HeartbeatPeerStatus describes the status of the communication
	// with a specific peer node.
	HeartbeatPeerStatus struct {
		IsBeating bool      `json:"is_beating"`
		LastAt    time.Time `json:"last_at"`
	}

	HbLastMessage struct {
		From string `json:"from"`

		// PatchLength is the type of hb message except when Type is patch where it is the patch queue length
		PatchLength int `json:"patch_length"`

		// Type is the hb message type (unset/ping/full/patch)
		Type string `json:"type"`
	}
)
