package daemonsubsystem

import (
	"time"
)

type (
	// Deamon describes the node daemon components
	Deamon struct {
		Nodename  string    `json:"nodename"`
		Routines  int       `json:"routines"`
		CreatedAt time.Time `json:"created_at"`

		Collector  Collector  `json:"collector"`
		Dns        Dns        `json:"dns"`
		Daemondata Daemondata `json:"daemondata"`
		Hb         Hb         `json:"hb"`
		Listener   Listener   `json:"listener"`
		Scheduler  Scheduler  `json:"scheduler"`
	}

	// DaemonSubsystemStatus describes a OpenSVC daemon subsystem: when it
	// was last configured, when it was created, its current state and its
	// id.
	DaemonSubsystemStatus struct {
		ID           string        `json:"id"`
		ConfiguredAt time.Time     `json:"configured_at"`
		CreatedAt    time.Time     `json:"created_at"`
		State        string        `json:"state"`
		Alerts       []ThreadAlert `json:"alerts,omitempty"`
	}

	// ThreadAlert describes a message with a severity. Embedded in DaemonSubsystemStatus
	ThreadAlert struct {
		Message  string `json:"message"`
		Severity string `json:"severity"`
	}
)
