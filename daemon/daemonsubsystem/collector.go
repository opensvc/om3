package daemonsubsystem

type (
	// Collector describes the daemon collector subsystem.
	Collector struct {
		Status

		Url string `json:"url"`
	}
)

func (c *Collector) DeepCopy() *Collector {
	n := *c
	return &n
}
