package cluster

import "time"

type (
	// SchedulerThreadEntry describes a task queued for execution by the
	// opensvc scheduler thread.
	SchedulerThreadEntry struct {
		Action string    `json:"action"`
		Csum   string    `json:"csum"`
		Path   string    `json:"path"`
		Expire time.Time `json:"expire"`
		Queued time.Time `json:"queued"`
		Rid    string    `json:"rid"`
	}

	// SchedulerThreadStatus describes the OpenSVC daemon scheduler thread
	// state, which is responsible for executing node and objects scheduled
	// jobs.
	SchedulerThreadStatus struct {
		ThreadStatus
		Delayed []SchedulerThreadEntry `json:"delayed"`
	}
)
