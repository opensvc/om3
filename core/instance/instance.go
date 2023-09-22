package instance

type (
	Instance struct {
		Config  *Config  `json:"config"`
		Monitor *Monitor `json:"monitor"`
		Status  *Status  `json:"status"`
	}
)
