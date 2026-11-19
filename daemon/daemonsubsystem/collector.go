package daemonsubsystem

import "time"

type (
	// Collector describes the daemon collector subsystem.
	Collector struct {
		Status

		Url string `json:"url"`

		// Speaker is the node the collector of the node takes for the
		// speaker, as one it waits for, away: a node restarting takes the
		// speech back only while its peers wait for it.
		Speaker string `json:"speaker,omitempty"`

		// UnreachableSince is the time the speaker found the collector
		// unreachable at, its sends failing before any answer, as on a
		// name not resolved or a connection refused, and nil once it
		// answers again: the scheduler holds the jobs feeding the
		// collector meanwhile.
		UnreachableSince *time.Time `json:"unreachable_since,omitempty"`
	}
)

func (c *Collector) DeepCopy() *Collector {
	n := *c
	if c.UnreachableSince != nil {
		v := *c.UnreachableSince
		n.UnreachableSince = &v
	}
	return &n
}
