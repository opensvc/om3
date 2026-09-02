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

// DeepCopy returns a copy of the instance sharing nothing with it.
func (t *Instance) DeepCopy() *Instance {
	if t == nil {
		return nil
	}
	n := Instance{Config: t.Config.DeepCopy()}
	if t.Monitor != nil {
		n.Monitor = t.Monitor.DeepCopy()
	}
	if t.Status != nil {
		n.Status = t.Status.DeepCopy()
	}
	return &n
}
