package object

import (
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

// holdSource reads a resource section for the values it holds, as evaluated
// for one node.
type holdSource struct {
	cfg     *xconfig.T
	path    naming.Path
	section string
	node    string
}

func (t holdSource) Get(option string) (string, bool) {
	v, err := t.cfg.EvalAs(key.New(t.section, option), t.node)
	if err != nil {
		return "", false
	}
	return xconfig.EvaluatedString(v), true
}

func (t holdSource) Path() naming.Path { return t.path }

func (t holdSource) RID() string { return t.section }

// Holds returns the values the resources of the object hold, evaluated for
// each of the nodes, with the nodes each is held on: the nodes it evaluates to
// that value on, or every cluster node for a shared resource.
//
// The configurer evaluates from the configuration alone, as one made with
// WithStaticReferences: a value referencing what another resource exposes is
// not held by this resource, but by the one it comes from. A disabled
// resource holds nothing, never being started.
func Holds(p naming.Path, oc Configurer, nodes []string) []resource.ObjectHold {
	cfg := oc.Config()
	type holdKey struct {
		rid string
		resource.Hold
	}
	merged := make(map[holdKey]*resource.ObjectHold)
	order := make([]holdKey, 0)
	for _, section := range cfg.SectionStrings() {
		holder := sectionHolder(cfg, section)
		if holder == nil {
			continue
		}
		for _, node := range nodes {
			if b, err := cfg.EvalAs(key.New(section, "disable"), node); err == nil {
				if disabled, ok := b.(bool); ok && disabled {
					continue
				}
			}
			shared := false
			if b, err := cfg.EvalAs(key.New(section, "shared"), node); err == nil {
				shared, _ = b.(bool)
			}
			src := holdSource{cfg: cfg, path: p, section: section, node: node}
			for _, h := range holder.Holds(src) {
				if h.Value == "" {
					continue
				}
				k := holdKey{rid: section, Hold: h}
				oh, ok := merged[k]
				if !ok {
					oh = &resource.ObjectHold{RID: section, Hold: h}
					merged[k] = oh
					order = append(order, k)
				}
				if shared {
					oh.Cluster = true
				}
				if !slices.Contains(oh.Nodes, node) {
					oh.Nodes = append(oh.Nodes, node)
				}
			}
		}
	}
	l := make([]resource.ObjectHold, 0, len(order))
	for _, k := range order {
		oh := merged[k]
		if oh.Cluster {
			oh.Nodes = nil
		} else {
			slices.Sort(oh.Nodes)
		}
		l = append(l, *oh)
	}
	slices.SortFunc(l, func(a, b resource.ObjectHold) int {
		if c := strings.Compare(a.RID, b.RID); c != 0 {
			return c
		}
		if c := strings.Compare(a.Class, b.Class); c != 0 {
			return c
		}
		return strings.Compare(a.Value, b.Value)
	})
	return l
}

// sectionHolder returns the driver of the resource section, allocated and
// not configured, when it holds values, and nil otherwise.
func sectionHolder(cfg *xconfig.T, section string) resource.Holder {
	rid, err := resourceid.Parse(section)
	if err != nil {
		return nil
	}
	group := rid.DriverGroup()
	if group == driver.GroupUnknown {
		return nil
	}
	typ := cfg.GetString(key.New(section, "type"))
	if typ == "" {
		typ = driver.DefaultDriver[group]
	}
	for _, i := range driver.All.WithID(group, typ) {
		allocator, ok := i.Allocator.(func() resource.Driver)
		if !ok {
			continue
		}
		if holder, ok := allocator().(resource.Holder); ok {
			return holder
		}
	}
	return nil
}
