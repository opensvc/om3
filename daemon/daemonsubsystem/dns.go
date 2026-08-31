package daemonsubsystem

type (
	// Dns defines model for Dns daemon subsystem.
	Dns struct {
		Status

		// Nameservers list of nameservers
		Nameservers []string `json:"nameservers"`
	}
)

// DeepCopy returns a copy of the dns state sharing nothing with it.
// Nameservers comes out non-nil, see Heartbeat.DeepCopy.
func (c *Dns) DeepCopy() *Dns {
	n := *c
	n.Nameservers = append([]string{}, c.Nameservers...)
	return &n
}
