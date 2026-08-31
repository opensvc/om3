package daemonsubsystem

import (
	"time"
)

type (
	// DaemonLocal defines model for DaemonLocal data that are not sent to peers.
	DaemonLocal struct {
		// Nodename is used to identify the nodename that have sent Daemon struct
		Nodename string `json:"nodename"`
		Routines int    `json:"routines"`
	}

	// Daemon defines model for Daemon.
	Daemon struct {
		// Collector DaemonCollector describes the OpenSVC daemon collector subsystem state,
		// which is responsible for communicating with the collector on behalf
		// of the cluster. Only one node on the cluster is the collector speaker
		Collector Collector `json:"collector"`

		// Daemondata DaemonDaemondata describes the OpenSVC daemon data subsystem state,
		// which is responsible for aggregating data messages and selecting
		// candidate data messages to forward to peer nodes.
		Daemondata Daemondata `json:"daemondata"`

		// Dns describes the OpenSVC daemon dns subsystem state, which is
		// responsible for janitoring and serving the cluster Dns zone.
		// This zone is dynamically populated by ip address allocated for the
		// services (frontend and backend).
		Dns Dns `json:"dns"`

		Heartbeat Heartbeat `json:"heartbeat"`

		// Listener DaemonListener describes the OpenSVC daemon listener subsystem state,
		// which is responsible for serving the API.
		Listener Listener `json:"listener"`

		Nodename string `json:"nodename"`

		// Pid the main daemon process id
		// it is sent on the full hb message, then not anymore changed
		Pid int `json:"pid"`

		// StartedAt is the time when daemon has been started
		// it is sent on the full hb message, then not anymore changed
		StartedAt time.Time `json:"started_at"`

		RunnerImon RunnerImon `json:"runner_imon"`

		// Scheduler DaemonScheduler describes the OpenSVC daemon scheduler subsystem state,
		// which is responsible for executing node and objects scheduled jobs.
		Scheduler Scheduler `json:"scheduler"`
	}

	// Status describes a OpenSVC daemon subsystem: when it was last created,
	// configured an updated, what its current state is and its id.
	Status struct {
		ID string `json:"id"`

		State string `json:"state"`

		ConfiguredAt time.Time `json:"configured_at"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	// Alert describes a message with a severity
	Alert struct {
		Message  string `json:"message"`
		Severity string `json:"severity"`
	}
)

// DeepCopy returns a copy of the daemon state sharing nothing with it.
//
// It copies the struct first and deepens the members after, rather than
// listing the fields it wants: enumerating them silently dropped Nodename
// here, and SecretVersion and UpdatedAt in Heartbeat, until a real
// clusterdump.Data.DeepCopy started routing through these methods.
func (d *Daemon) DeepCopy() *Daemon {
	n := *d
	n.Collector = *d.Collector.DeepCopy()
	n.Daemondata = *d.Daemondata.DeepCopy()
	n.Dns = *d.Dns.DeepCopy()
	n.Heartbeat = *d.Heartbeat.DeepCopy()
	n.Listener = *d.Listener.DeepCopy()
	n.RunnerImon = *d.RunnerImon.DeepCopy()
	n.Scheduler = *d.Scheduler.DeepCopy()
	return &n
}
