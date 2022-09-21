package san

const (
	FC    = "fc"
	FCOE  = "fcoe"
	ISCSI = "iscsi"
)

type (
	// Paths is a list of hba:target
	Paths []Path

	// Path is a hba:target link
	Path struct {
		HostBusAdapter HostBusAdapter
		TargetPort     TargetPort
	}

	TargetPorts []TargetPort

	TargetPort struct {
		ID string `json:"tgt_id"`
	}

	HostBusAdapter struct {
		ID   string `json:"hba_id"`
		Type string `json:"hba_type"`
		Host string `json:"host"`
	}
)

// WithHBAID returns the list of paths whose hba id matches the argument.
func (t Paths) WithHBAID(id string) Paths {
	l := make(Paths, 0)
	for _, path := range t {
		if path.HostBusAdapter.ID == id {
			l = append(l, path)
		}
	}
	return l
}
