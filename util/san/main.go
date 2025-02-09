package san

import (
	"fmt"
	"strings"
)

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

func (t Paths) Mapping() string {
	return strings.Join(t.MappingList(), ",")
}

func (t Paths) MappingList() []string {
	l := make([]string, 0)
	for _, p := range t {
		s := p.Initiator.Name + ":" + p.Target.Name
		l = append(l, s)
	}
	return l
}

func ParseMapping(s string) (Paths, error) {
	paths := make(Paths, 0)
	parseFCMap := func(s string) (Path, error) {
		l := strings.Split(s, ":")
		switch len(l) {
		case 1:
			return Path{}, fmt.Errorf("san paths parser: %s path has too few columns. ex: initiatorName:targetName", s)
		case 2:
			// normal
		default:
			return Path{}, fmt.Errorf("san paths parser: %s path has too many columns. ex: initiatorName:targetName", s)
		}
		p := Path{
			Initiator{
				Name: l[0],
				Type: FC,
			},
			Target{
				Name: l[1],
				Type: FC,
			},
		}
		return p, nil
	}
	parseISCSIMap := func(s string) (Path, error) {
		l := strings.Split(s, ":iqn.")
		switch len(l) {
		case 1:
			return Path{}, fmt.Errorf("san paths parser: %s path has too few columns. ex: initiatorName:targetName", s)
		case 2:
			// normal
		default:
			return Path{}, fmt.Errorf("san paths parser: %s path has too many columns. ex: initiatorName:targetName", s)
		}
		p := Path{
			Initiator{
				Name: l[0],
				Type: ISCSI,
			},
			Target{
				Name: "iqn." + l[1],
				Type: ISCSI,
			},
		}
		return p, nil
	}
	for _, one := range strings.Split(s, ",") {
		if s == "" {
			continue
		}
		if strings.Contains(s, "iqn.") {
			if p, err := parseISCSIMap(one); err == nil {
				paths = append(paths, p)
			} else {
				return paths, err
			}
		} else {
			if p, err := parseFCMap(one); err == nil {
				paths = append(paths, p)
			} else {
				return paths, err
			}
		}
	}
	return paths, nil
}

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

// WithTargetName returns the list of paths whose target name matches the argument.
func (t Paths) WithTargetName(name string) Paths {
	l := make(Paths, 0)
	for _, path := range t {
		if path.Target.Name == name {
			l = append(l, path)
		}
	}
	return l
}

func (t Paths) Has(p Path) bool {
	for _, other := range t {
		if p.IsEqual(other) {
			return true
		}
	}
	return false
}

func (t Path) IsIn(paths Paths) bool {
	for _, other := range paths {
		if t.IsEqual(other) {
			return true
		}
	}
	return false
}

func (t Path) IsEqual(other Path) bool {
	if !t.Initiator.IsEqual(other.Initiator) {
		return false
	}
	if !t.Target.IsEqual(other.Target) {
		return false
	}
	return true
}

func (t Initiator) IsEqual(other Initiator) bool {
	if t.Type != other.Type {
		return false
	}
	if t.Name != other.Name {
		return false
	}
	return true
}

func (t Target) IsEqual(other Target) bool {
	if t.Type != other.Type {
		return false
	}
	if t.Name != other.Name {
		return false
	}
	return true
}

func (t Paths) HasAllOf(paths Paths) bool {
	for _, p := range t {
		if !paths.Has(p) {
			return false
		}
	}
	return true
}

func (t Paths) HasAnyOf(paths Paths) bool {
	for _, p := range t {
		if paths.Has(p) {
			return true
		}
	}
	return false
}

func (t Paths) DeepCopy() *Paths {
	l := make(Paths, len(t))
	for i, p := range t {
		l[i] = p.DeepCopy()
	}
	return &l
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
