package cluster

import "time"

type (
	// SchedulerThreadEntry describes a task queued for execution by the
	// opensvc scheduler thread.
	SchedulerThreadEntry struct {
		Action   string    `json:"action" yaml:"action"`
		Csum     string    `json:"csum" yaml:"csum"`
		Path     string    `json:"path" yaml:"path"`
		ExpireAt time.Time `json:"expire_at" yaml:"expire_at"`
		QueuedAt time.Time `json:"queued_at" yaml:"queued_at"`
		Rid      string    `json:"rid" yaml:"rid"`
	}

	// DaemonScheduler describes the OpenSVC daemon scheduler thread
	// state, which is responsible for executing node and objects scheduled
	// jobs.
	DaemonScheduler struct {
		DaemonSubsystemStatus
		Delayed []SchedulerThreadEntry `json:"delayed" yaml:"delayed"`
	}
)
