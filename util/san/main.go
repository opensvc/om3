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
		Initiator Initiator `json:"initiator"`
		Target    Target    `json:"target"`
	}

	Targets []Target

	Target struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	Initiator struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
)

// WithInitiatorName returns the list of paths whose initiator name matches the argument.
func (t Paths) WithInitiatorName(name string) Paths {
	l := make(Paths, 0)
	for _, path := range t {
		if path.Initiator.Name == name {
			l = append(l, path)
		}
	}
	return l
}

func (t Paths) DeepCopy() Paths {
	l := make(Paths, len(t))
	for i, p := range t {
		l[i] = p.DeepCopy()
	}
	return l
}

func (t Path) DeepCopy() Path {
	return Path{
		Initiator: t.Initiator.DeepCopy(),
		Target:    t.Target.DeepCopy(),
	}
}

func (t Initiator) DeepCopy() Initiator {
	return Initiator{
		Type: t.Type,
		Name: t.Name,
	}
}

func (t Target) DeepCopy() Target {
	return Target{
		Type: t.Type,
		Name: t.Name,
	}
}
