package daemonsubsystem

type (
	// Daemondata defines model for Daemondata.
	Daemondata struct {
		Status

		// QueueSize the subscription queue size
		QueueSize int `json:"queue_size"`
	}
)

func (c *Daemondata) DeepCopy() *Daemondata {
	n := *c
	return &n
}
