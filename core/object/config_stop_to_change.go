package object

import (
	"context"
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
var ErrStopToChange = errors.New("the resource is not known to be stopped")

// CheckStopToChange refuses a change from the configuration from to the
// configuration to of an object of kind, for a resource up on a node, as
// statuses say, the instance statuses of the object by node:
//
//   - a change of the value of a keyword marked StopToChange, as the mount
//     point of a filesystem or the address of an ip, evaluated on the node,
//     so a change of what it references is one too, unless the value was
//     empty, which designates nothing the resource could have set up, as a
//     required keyword left empty, which leaves the resource not configured
//     and its status not known;
//   - a change of its type, which is a change of the driver stopping it;
//   - its disable, which leaves it running with nothing to stop it, and its
//     values held with no hold said: the configuration of a disabled object
//     holds nothing, so another can be made holding them;
//   - the removal of its section, which nothing stops once it is gone.
//
// The stop of the resource acts on the configuration it reads then: what
// the former value set up stays on the node, for root to find and undo, as a
// filesystem still mounted, an address still configured.
//
// A resource is up on a node unless its status there is known to be
// stopped: down, standby down, or n/a, or undef for an error of its
// configuration while it holds nothing there, its ConfigError, which a
// change of the configuration fixes. Any other undef status is not known,
// and the refusal says so, and that a change forced is the operator's to
// make once sure the resource holds nothing. An object created changes
// nothing.
func CheckStopToChange(kind naming.Kind, from, to *xconfig.T, statuses map[string]*instance.Status) error {
	if from == nil || to == nil {
		return nil
	}
	refusals := make([]string, 0)
	unknown := false
	toSections := to.SectionStrings()
	for _, section := range from.SectionStrings() {
		rid, err := resourceid.Parse(section)
		if err != nil || rid.DriverGroup() == driver.GroupUnknown {
			continue
		}
		up := resourceUpOn(section, rid.DriverGroup(), statuses)
		if len(up) == 0 {
			continue
		}
		if up.hasUnknown() {
			unknown = true
		}
		if !slices.Contains(toSections, section) {
			refusals = append(refusals, fmt.Sprintf("delete %s: %s", section, strings.Join(up.texts(), ", ")))
			continue
		}
		for _, option := range KeywordOptions(kind, section) {
			k := key.New(section, option)
			if option != "type" && option != "disable" {
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
				if was == is || was == "" {
					continue
				}
				if option == "disable" {
					refusals = append(refusals, fmt.Sprintf("disable %s: %s", section, u.text()))
				} else {
					refusals = append(refusals, fmt.Sprintf("change %s from %q to %q: %s", k, was, is, u.text()))
				}
				break
			}
		}
	}
	if len(refusals) == 0 {
		return nil
	}
	advice := "stop it before"
	if unknown {
		advice += ", or force the change once sure it holds nothing on the node where its status is not known"
	}
	return fmt.Errorf("%w: %s: %s", ErrStopToChange, strings.Join(refusals, "; "), advice)
}

type (
	resourceUp struct {
		node    string
		status  status.T
		running bool
	}
	resourceUps []resourceUp
)

func (t resourceUp) text() string {
	switch {
	case t.running:
		return fmt.Sprintf("running on %s", t.node)
	case t.status == status.Undef:
		return fmt.Sprintf("status not known (undef) on %s", t.node)
	}
	return fmt.Sprintf("%s on %s", t.status, t.node)
}

// hasUnknown says whether the status of the resource is not known on one
// of the nodes.
func (t resourceUps) hasUnknown() bool {
	for _, u := range t {
		if !u.running && u.status == status.Undef {
			return true
		}
	}
	return false
}

func (t resourceUps) texts() []string {
	l := make([]string, len(t))
	for i, u := range t {
		l[i] = u.text()
	}
	return l
}

// resourceUpOn returns the nodes the resource of section is up on, sorted.
//
// A task is up on a node while it runs there, which is what its stop ends.
// Its status says how its last run went, as a last_run check does: a task
// whose last run succeeded holds nothing on the node.
func resourceUpOn(section string, group driver.Group, statuses map[string]*instance.Status) resourceUps {
	l := make(resourceUps, 0)
	for nodename, st := range statuses {
		if st == nil {
			continue
		}
		if group == driver.GroupTask {
			if st.Running.Has(section) {
				l = append(l, resourceUp{node: nodename, running: true})
			}
			continue
		}
		rstat, ok := st.Resources[section]
		if !ok {
			continue
		}
		if rstat.Status.Is(status.Down, status.StandbyDown, status.NotApplicable) {
			continue
		}
		if rstat.Status == status.Undef && rstat.ConfigError != "" {
			// Undef for an error of its configuration, holding
			// nothing: the change is what fixes it.
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
// node, as its cache says, which the daemon keeps current, or as evaluated
// when the cache is not there or not readable: a change is judged on a
// status known, never on none. An object holding no resource has none.
func localInstanceStatuses(p naming.Path) (map[string]*instance.Status, error) {
	m := make(map[string]*instance.Status)
	switch p.Kind {
	case naming.KindSvc, naming.KindVol:
	default:
		return m, nil
	}
	var st instance.Status
	if b, err := os.ReadFile(filepath.Join(p.VarDir(), "status.json")); err == nil && json.Unmarshal(b, &st) == nil {
		m[hostname.Hostname()] = &st
		return m, nil
	}
	o, err := NewActor(p, WithVolatile(true))
	if err != nil {
		return nil, err
	}
	st, err = o.FreshStatus(context.Background())
	if err != nil {
		return nil, fmt.Errorf("evaluate the status of %s, to tell the resources up: %w", p, err)
	}
	m[hostname.Hostname()] = &st
	return m, nil
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
	statuses, err := localInstanceStatuses(p)
	if err != nil {
		return err
	}
	return CheckStopToChange(p.Kind, from.Config(), to.Config(), statuses)
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
	statuses, err := localInstanceStatuses(p)
	if err != nil {
		return err
	}
	return CheckStopToChange(p.Kind, from.Config(), to.Config(), statuses)
}
