package dsubsystem

import (
	"time"
)

type (
	// Scheduler describes the OpenSVC daemon scheduler thread
	// state, which is responsible for executing node and objects scheduled
	// jobs.
	Scheduler struct {
		DaemonSubsystemStatus

		Delayed []SchedulerThreadEntry `json:"delayed"`
	}

	// SchedulerThreadEntry describes a task queued for execution by the
	// opensvc scheduler thread.
	SchedulerThreadEntry struct {
		Action   string    `json:"action"`
		Csum     string    `json:"csum"`
		Path     string    `json:"path"`
		ExpireAt time.Time `json:"expire_at"`
		QueuedAt time.Time `json:"queued_at"`
		Rid      string    `json:"rid"`
	}
)
