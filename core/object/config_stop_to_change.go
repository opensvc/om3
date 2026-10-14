package object

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

// ErrStopToChange is the refusal of a configuration change a resource up
// on a node would leak by: a change of a keyword saying what it holds there,
// of its type, or the removal of its section.
var ErrStopToChange = errors.New("the resource is up")

// CheckStopToChange refuses a change from the configuration from to the
// configuration to of an object of kind, for a resource up on a node, as
// statuses say, the instance statuses of the object by node:
//
//   - a change of the value of a keyword marked StopToChange, as the mount
//     point of a filesystem or the address of an ip, evaluated on the node,
//     so a change of what it references is one too;
//   - a change of its type, which is a change of the driver stopping it;
//   - the removal of its section, which nothing stops once it is gone.
//
// The stop of the resource acts on the configuration it reads then: what
// the former value set up stays on the node, for root to find and undo, as a
// filesystem still mounted, an address still configured.
//
// A resource is up on a node unless its status there is known to be
// stopped: down, standby down, or n/a. An object created changes nothing.
func CheckStopToChange(kind naming.Kind, from, to *xconfig.T, statuses map[string]*instance.Status) error {
	if from == nil || to == nil {
		return nil
	}
	refusals := make([]string, 0)
	toSections := to.SectionStrings()
	for _, section := range from.SectionStrings() {
		rid, err := resourceid.Parse(section)
		if err != nil || rid.DriverGroup() == driver.GroupUnknown {
			continue
		}
		up := resourceUpOn(section, statuses)
		if len(up) == 0 {
			continue
		}
		if !slices.Contains(toSections, section) {
			refusals = append(refusals, fmt.Sprintf("delete %s: %s", section, strings.Join(up.texts(), ", ")))
			continue
		}
		for _, option := range KeywordOptions(kind, section) {
			k := key.New(section, option)
			if option != "type" {
				if from.Referrer == nil {
					continue
				}
				kw := from.Referrer.KeywordLookup(k, from.SectionType(k))
				if kw == nil || !kw.StopToChange {
					continue
				}
			}
			for _, u := range up {
				was, is := evaluatedAs(from, k, u.node), evaluatedAs(to, k, u.node)
				if was == is {
					continue
				}
				refusals = append(refusals, fmt.Sprintf("change %s from %q to %q: %s", k, was, is, u.text()))
				break
			}
		}
	}
	if len(refusals) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s: stop it before", ErrStopToChange, strings.Join(refusals, "; "))
}

type (
	resourceUp struct {
		node   string
		status status.T
	}
	resourceUps []resourceUp
)

func (t resourceUp) text() string {
	return fmt.Sprintf("%s on %s", t.status, t.node)
}

func (t resourceUps) texts() []string {
	l := make([]string, len(t))
	for i, u := range t {
		l[i] = u.text()
	}
	return l
}

// resourceUpOn returns the nodes the resource of section is up on, sorted.
func resourceUpOn(section string, statuses map[string]*instance.Status) resourceUps {
	l := make(resourceUps, 0)
	for nodename, st := range statuses {
		if st == nil {
			continue
		}
		rstat, ok := st.Resources[section]
		if !ok {
			continue
		}
		if rstat.Status.Is(status.Down, status.StandbyDown, status.NotApplicable) {
			continue
		}
		l = append(l, resourceUp{node: nodename, status: rstat.Status})
	}
	slices.SortFunc(l, func(a, b resourceUp) int { return strings.Compare(a.node, b.node) })
	return l
}

// evaluatedAs returns the value of k on the node, as written when it does
// not evaluate.
func evaluatedAs(cfg *xconfig.T, k key.T, nodename string) string {
	v, err := cfg.EvalAs(k, nodename)
	if err != nil {
		return cfg.Get(k)
	}
	return xconfig.EvaluatedString(v)
}

// localInstanceStatuses returns the instance status of the object on this
// node, as its cache says, which the daemon keeps current: none when the
// cache is not there, as before a first status.
func localInstanceStatuses(p naming.Path) map[string]*instance.Status {
	m := make(map[string]*instance.Status)
	b, err := os.ReadFile(filepath.Join(p.VarDir(), "status.json"))
	if err != nil {
		return m
	}
	var st instance.Status
	if err := json.Unmarshal(b, &st); err == nil {
		m[hostname.Hostname()] = &st
	}
	return m
}

// CheckLocalUpdateStopToChange refuses the update of the configuration of
// the object at p a resource up on this node would leak by, as
// CheckStopToChange says, for a write made without the daemon, which knows
// the status of the other nodes.
func CheckLocalUpdateStopToChange(p naming.Path, deletes []string, unsets []key.T, sets []keyop.T) error {
	from, err := NewConfigurer(p, WithVolatile(true))
	if err != nil {
		return err
	}
	to, err := NewConfigurer(p, WithVolatile(true))
	if err != nil {
		return err
	}
	if err := to.Config().PrepareUpdate(deletes, unsets, sets); err != nil {
		return err
	}
	return CheckStopToChange(p.Kind, from.Config(), to.Config(), localInstanceStatuses(p))
}

// CheckLocalFileStopToChange refuses the configuration file at dst replacing
// the one of the object at p, as an edit does, when a resource up on this
// node would leak by it, as CheckStopToChange says.
func CheckLocalFileStopToChange(p naming.Path, dst string) error {
	from, err := NewConfigurer(p, WithVolatile(true))
	if err != nil {
		return err
	}
	to, err := NewConfigurer(p, WithConfigFile(dst), WithVolatile(true))
	if err != nil {
		return err
	}
	return CheckStopToChange(p.Kind, from.Config(), to.Config(), localInstanceStatuses(p))
}
