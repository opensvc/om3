package daemonsubsystem

type (
	// Listener defines daemon Listener subsystem state.
	Listener struct {
		Status

		Addr string `json:"addr"`

		Port string `json:"port"`
	}
)

func (c *Listener) DeepCopy() *Listener {
	d := *c
	return &d
}
