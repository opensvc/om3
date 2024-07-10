package daemonsubsystem

type (
	// RunnerImon defines the daemon runner for imon subsystem.
	RunnerImon struct {
		Status

		MaxRunning int `json:"max_running"`
	}
)

func (c *RunnerImon) DeepCopy() *RunnerImon {
	v := *c
	return &v
}
