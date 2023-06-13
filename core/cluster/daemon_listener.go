package cluster

import (
	"net"
	"time"
)

type (
	// ListenerThreadSession describes statistics of a session of the api listener.
	ListenerThreadSession struct {
		Addr      string    `json:"addr" yaml:"addr"`
		CreatedAt time.Time `json:"created_at" yaml:"created_at"`
		Encrypted bool      `json:"encrypted" yaml:"encrypted"`
		Progress  string    `json:"progress" yaml:"progress"`
		TID       uint64    `json:"tid" yaml:"tid"`
	}

	// ListenerThreadClient describes the statistics of all session of a single client the api listener.
	ListenerThreadClient struct {
		Accepted      uint64 `json:"accepted" yaml:"accepted"`
		AuthValidated uint64 `json:"auth_validated" yaml:"auth_validated"`
		RX            uint64 `json:"rx" yaml:"rx"`
		TX            uint64 `json:"tx" yaml:"tx"`
	}

	// ListenerThreadSessions describes the sessions statistics of the api listener.
	ListenerThreadSessions struct {
		Accepted      uint64                           `json:"accepted" yaml:"accepted"`
		AuthValidated uint64                           `json:"auth_validated" yaml:"auth_validated"`
		RX            uint64                           `json:"rx" yaml:"rx"`
		TX            uint64                           `json:"tx" yaml:"tx"`
		Alive         map[string]ListenerThreadSession `json:"alive" yaml:"alive"`
		Clients       map[string]ListenerThreadClient  `json:"clients" yaml:"clients"`
	}

	// ListenerThreadStats describes the statistics of the api listener.
	ListenerThreadStats struct {
		Sessions ListenerThreadSessions `json:"sessions" yaml:"sessions"`
	}

	// DaemonListener describes the OpenSVC daemon listener thread,
	// which is responsible for serving the API.
	DaemonListener struct {
		DaemonSubsystemStatus
		Config ListenerThreadStatusConfig `json:"config" yaml:"config"`
		Stats  ListenerThreadStats        `json:"stats" yaml:"stats"`
	}

	// ListenerThreadStatusConfig holds a summary of the listener configuration
	ListenerThreadStatusConfig struct {
		Addr net.IP `json:"addr" yaml:"addr"`
		Port int    `json:"port" yaml:"port"`
	}
)
