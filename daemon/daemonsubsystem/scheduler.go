package daemonsubsystem

type (
	// Scheduler defines model for daemon scheduler subsystem.
	Scheduler struct {
		Status

		// Count is the number of defined scheduled jobs
		Count int `json:"count"`

		// MaxRunning is the maximum number of running jobs
		MaxRunning int `json:"max_running"`
	}
)

func (d *Scheduler) DeepCopy() *Scheduler {
	n := *d
	return &n
}
