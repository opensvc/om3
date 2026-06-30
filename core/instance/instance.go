package instance

type (
	Instance struct {
		Config  *Config  `json:"config"`
		Monitor *Monitor `json:"monitor"`
		Status  *Status  `json:"status"`
	}
)

func (t *Instance) IsZero() bool {
	if t.Config != nil {
		return false
	}
	if t.Monitor != nil {
		return false
	}
	if t.Status != nil {
		return false
	}
	return true
}
