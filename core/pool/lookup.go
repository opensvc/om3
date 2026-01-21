package pool

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/volaccess"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	manager interface {
		Pools() []Pooler
	}
	Lookup struct {
		Name     string
		Type     string
		Access   volaccess.T
		Size     int64
		Format   bool
		Shared   bool
		Usage    bool
		Volatile bool
		Nodes    []string

		manager manager
	}
	WeightedPools []Pooler
	By            func(p1, p2 *StatusItem) bool
	statusSorter  struct {
		data StatusList
		by   func(p1, p2 *StatusItem) bool // Closure used in the Less method.
	}
)

const (
	CapBlk      Capability = "blk"
	CapFile     Capability = "file"
	CapMove     Capability = "move"
	CapROO      Capability = "roo"
	CapROX      Capability = "rox"
	CapRWO      Capability = "rwo"
	CapRWX      Capability = "rwx"
	CapShared   Capability = "shared"
	CapSnap     Capability = "snap"
	CapVolatile Capability = "volatile"
)

func (by By) Sort(l StatusList) {
	s := &statusSorter{
		data: l,
		by:   by, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(s)
}

func (t statusSorter) Len() int {
	return len(t.data)
}

func (t statusSorter) Less(i, j int) bool {
	return t.by(&t.data[i], &t.data[j])
}

func (t statusSorter) Swap(i, j int) {
	t.data[i], t.data[j] = t.data[j], t.data[i]
}

func NewLookup(m manager) *Lookup {
	t := Lookup{
		manager: m,
	}
	return &t
}

func (t Lookup) Do(ctx context.Context) (Pooler, error) {
	cause := make([]string, 0)
	l := NewStatusList()
	m := make(map[string]Pooler)
	for _, p := range t.manager.Pools() {
		if t.Name != "" && t.Name != p.Name() {
			cause = append(cause, fmt.Sprintf("[%s] not matching name %s", p.Name(), t.Name))
			continue
		}
		if t.Type != "" && t.Type != p.Type() {
			cause = append(cause, fmt.Sprintf("[%s] type %s not matching %s", p.Name(), p.Type(), t.Type))
			continue
		}
		if !t.Access.IsZero() && !HasAccess(p, t.Access) {
			cause = append(cause, fmt.Sprintf("[%s] not %s capable %s", p.Name(), t.Access, p.Capabilities()))
			continue
		}
		if t.Format == false && !HasCapability(p, CapBlk) {
			cause = append(cause, fmt.Sprintf("[%s] not blk capable", p.Name()))
			continue
		}
		if t.Format == true && !HasCapability(p, CapFile) {
			cause = append(cause, fmt.Sprintf("[%s] not file capable", p.Name()))
			continue
		}
		if t.Shared == true && !HasCapability(p, CapShared) {
			cause = append(cause, fmt.Sprintf("[%s] not shared capable", p.Name()))
			continue
		}
		if t.Volatile == true && !HasCapability(p, CapVolatile) {
			cause = append(cause, fmt.Sprintf("[%s] not volatile capable", p.Name()))
			continue
		}
		if t.Volatile == false && HasCapability(p, CapVolatile) {
			cause = append(cause, fmt.Sprintf("[%s] not persistent capable", p.Name()))
			continue
		}
		if t.Usage == true {
			usage, err := p.Usage(ctx)
			if err != nil {
				cause = append(cause, fmt.Sprintf("[%s] no usage data: %s", p.Name(), err))
				continue
			}
			if usage.Size > 0 && (usage.Free < t.Size) {
				cause = append(cause, fmt.Sprintf("[%s] not enough free space: %s free, %s requested",
					p.Name(), sizeconv.BSize(float64(usage.Free)), sizeconv.BSize(float64(t.Size))))
				continue
			}
		}
		l = l.Add(ctx, p, t.Usage)
		m[p.Name()] = p
	}
	if len(l) == 0 {
		return nil, fmt.Errorf("no pool matching criteria: %s", strings.Join(cause, " "))
	}
	weight := func(p1, p2 *StatusItem) bool {
		if !t.Shared {
			p1shared := p1.HasCapability(CapShared)
			p2shared := p2.HasCapability(CapShared)
			switch {
			case p1shared && p2shared:
				// not decisive
			case !p1shared && !p2shared:
				// not decisive
			case p1shared && !p2shared:
				// prefer p2, not shared-capable
				return false
			case !p1shared && p2shared:
				// prefer p1, not shared-capable
				return true
			}
		}
		if p1.Usage.Free < p2.Usage.Free {
			return true
		}
		return p1.Name < p2.Name
	}
	By(weight).Sort(l)
	return m[l[0].Name], nil
}

type (
	consumer interface {
		String() string
		Config() *xconfig.T
	}
)

func (t Lookup) Env(p Pooler, c consumer, optional bool) ([]string, error) {
	env := make([]string, 0)
	cfg := c.Config()
	for k1, k2 := range p.Mappings() {
		val, err := cfg.GetStringStrict(key.Parse(k1))
		if err != nil {
			if optional {
				continue
			} else {
				return env, fmt.Errorf("missing mapped key in %s: %s", c, k1)
			}
		}
		if strings.Contains(val, "..") {
			return env, fmt.Errorf("the '..' substring is forbidden in volume env keys: %s %s=%s", c, k1, val)
		}
		s := fmt.Sprintf("%s=%s", k2, val)
		env = append(env, s)
	}
	return env, nil
}

func (t Lookup) ConfigureVolume(ctx context.Context, volume Volumer, obj interface{}) error {
	c, ok := obj.(consumer)
	if !ok {
		return fmt.Errorf("configure volume: the <obj> argument is not a consumer")
	}
	p, err := t.Do(ctx)
	if err != nil {
		return err
	}
	env, err := t.Env(p, c, false)
	if err != nil {
		return err
	}
	return ConfigureVolume(p, volume, t.Size, t.Format, t.Access, t.Shared, t.Nodes, env)
}
