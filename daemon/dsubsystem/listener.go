package dsubsystem

import (
	"net"
	"time"
)

type (
	// Listener describes the OpenSVC daemon listener thread,
	// which is responsible for serving the API.
	Listener struct {
		DaemonSubsystemStatus

		Config ListenerThreadStatusConfig `json:"config"`
		Stats  ListenerThreadStats        `json:"stats"`
	}

	// ListenerThreadStatusConfig holds a summary of the listener configuration
	ListenerThreadStatusConfig struct {
		Addr net.IP `json:"addr"`
		Port int    `json:"port"`
	}

	// ListenerThreadStats describes the statistics of the api listener.
	ListenerThreadStats struct {
		Sessions ListenerThreadSessions `json:"sessions"`
	}

	// ListenerThreadSession describes statistics of a session of the api listener.
	ListenerThreadSession struct {
		Addr      string    `json:"addr"`
		CreatedAt time.Time `json:"created_at"`
		Encrypted bool      `json:"encrypted"`
		Progress  string    `json:"progress"`
		TID       uint64    `json:"tid"`
	}

	// ListenerThreadClient describes the statistics of all session of a single client the api listener.
	ListenerThreadClient struct {
		Accepted      uint64 `json:"accepted"`
		AuthValidated uint64 `json:"auth_validated"`
		RX            uint64 `json:"rx"`
		TX            uint64 `json:"tx"`
	}

	// ListenerThreadSessions describes the sessions statistics of the api listener.
	ListenerThreadSessions struct {
		Accepted      uint64                           `json:"accepted"`
		AuthValidated uint64                           `json:"auth_validated"`
		RX            uint64                           `json:"rx"`
		TX            uint64                           `json:"tx"`
		Alive         map[string]ListenerThreadSession `json:"alive"`
		Clients       map[string]ListenerThreadClient  `json:"clients"`
	}
)
