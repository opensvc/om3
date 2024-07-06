package daemonsubsystem

type (
	// Dns defines model for Dns daemon subsystem.
	Dns struct {
		Status

		// Nameservers list of nameservers
		Nameservers []string `json:"nameservers"`
	}
)

func (c *Dns) DeepCopy() *Dns {
	return &Dns{
		Status: c.Status,

		Nameservers: append([]string{}, c.Nameservers...),
	}
}
