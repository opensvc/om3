package instance

type (
	Instance struct {
		Config  *Config  `json:"config" yaml:"config"`
		Monitor *Monitor `json:"monitor" yaml:"monitor"`
		Status  *Status  `json:"status" yaml:"status"`
	}
)
