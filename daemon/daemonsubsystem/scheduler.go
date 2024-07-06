package daemonsubsystem

type (
	// Scheduler defines model for daemon scheduler subsystem.
	Scheduler struct {
		Status

		// Count is the number of defined scheduled jobs
		Count int `json:"count"`
	}
)

func (d *Scheduler) DeepCopy() *Scheduler {
	n := *d
	return &n
}
