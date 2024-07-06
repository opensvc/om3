package daemonsubsystem

type (
	// RunnerImon defines the daemon runner for imon subsystem.
	RunnerImon struct {
		Status

		Count int `json:"count"`
	}
)

func (c *RunnerImon) DeepCopy() *RunnerImon {
	v := *c
	return &v
}
