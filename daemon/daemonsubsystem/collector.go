package daemonsubsystem

type (
	// Collector describes the daemon collector subsystem.
	Collector struct {
		Status

		Url string `json:"url"`

		// Speaker is the node the collector of the node takes for the
		// speaker, as one it waits for, away: a node restarting takes the
		// speech back only while its peers wait for it.
		Speaker string `json:"speaker,omitempty"`
	}
)

func (c *Collector) DeepCopy() *Collector {
	n := *c
	return &n
}
