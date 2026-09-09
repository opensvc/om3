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

func (t Paths) Diff(other Paths) string {
	oldSet := make(map[string]any)
	newSet := make(map[string]any)

	for _, item := range t {
		oldSet[item.String()] = nil
	}
	for _, item := range other {
		newSet[item.String()] = nil
	}

	var changes []string

	// Check for added
	for s := range newSet {
		if _, exists := oldSet[s]; !exists {
			changes = append(changes, fmt.Sprintf("+%s", s))
		}
	}

	// Check for removed
	for s := range oldSet {
		if _, exists := newSet[s]; !exists {
			changes = append(changes, fmt.Sprintf("-%s", s))
		}
	}

	if len(changes) == 0 {
		return ""
	}
	return strings.Join(changes, ", ")
}

func (t Paths) Mapping() string {
	return strings.Join(t.MappingList(), ",")
}

func (t Paths) MappingList() []string {
	l := make([]string, 0)
	for _, p := range t {
		s := p.String()
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

func (t Path) String() string {
	return fmt.Sprintf("%s:%s", t.Target.Name, t.Initiator.Name)
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

// ParseMappings reads the mappings of a command line, in the grammar the
// collector and the v2 agent write them: one initiator and the targets to
// reach it through, "<hba_id>:<tgt_id>[,<tgt_id>...]", the option repeated
// once per initiator.
//
// ParseMapping reads a different grammar, where the comma separates whole
// pairs. A value written for the collector and read that way loses every
// target but the first, and takes the others for initiators with no target of
// their own.
func ParseMappings(l []string) (Paths, error) {
	paths := make(Paths, 0)
	for _, s := range l {
		if s == "" {
			continue
		}
		hba, targets, ok := strings.Cut(s, ":")
		if !ok || hba == "" || targets == "" {
			return paths, fmt.Errorf("san paths parser: %s is not a <hba_id>:<tgt_id>[,<tgt_id>...] mapping", s)
		}
		for _, target := range strings.Split(targets, ",") {
			if target == "" {
				return paths, fmt.Errorf("san paths parser: %s has an empty target", s)
			}
			paths = append(paths, newPath(hba, target))
		}
	}
	return paths, nil
}

// newPath returns the path of an initiator to a target, of the transport their
// names say they are of.
func newPath(hba, target string) Path {
	transport := func(name string) string {
		if strings.HasPrefix(name, "iqn.") {
			return ISCSI
		}
		return FC
	}
	return Path{
		Initiator{Name: hba, Type: transport(hba)},
		Target{Name: target, Type: transport(target)},
	}
}
