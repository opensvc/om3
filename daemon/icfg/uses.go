package icfg

import (
	"slices"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

// getUses returns the cfg, sec and vol objects the resources of the object
// use on this node, as it evaluates the configuration, sorted: the
// datastores they read keys of, by name or by a pattern of their name, and
// the vol objects the volume resources are. A disabled resource uses
// nothing, never being started.
func (t *Manager) getUses(cf *xconfig.T) []string {
	l := make([]string, 0)
	add := func(s string) {
		if s != t.path.String() && !slices.Contains(l, s) {
			l = append(l, s)
		}
	}
	for _, section := range cf.SectionStrings() {
		rid, err := resourceid.Parse(section)
		if err != nil {
			continue
		}
		group := rid.DriverGroup()
		if group == driver.GroupUnknown {
			continue
		}
		if cf.GetBool(key.New(section, "disable")) {
			continue
		}
		get := func(option string) []string {
			return cf.GetStrings(key.New(section, option))
		}
		for _, s := range datarecv.SectionDatastores(get, t.path.Namespace) {
			add(s)
		}
		if group == driver.GroupVolume {
			name := cf.GetString(key.New(section, "name"))
			if p, err := naming.NewPath(t.path.Namespace, naming.KindVol, name); err == nil {
				add(p.String())
			}
		}
	}
	slices.Sort(l)
	return l
}
