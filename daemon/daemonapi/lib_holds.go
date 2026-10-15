package daemonapi

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// ErrHoldConflict is the refusal of a configuration whose resources would
// hold a value another object's resources hold on the same node.
var ErrHoldConflict = errors.New("held by another object")

// holdConflictError says the conflicts of a configuration, and is an
// ErrHoldConflict: the answer is titled so, and its detail is the list.
type holdConflictError struct {
	conflicts []string
}

func (t holdConflictError) Error() string {
	return strings.Join(t.conflicts, "; ")
}

func (t holdConflictError) Is(target error) bool {
	return target == ErrHoldConflict
}

type publishedHold struct {
	Path naming.Path
	resource.ObjectHold
}

// publishedHolds returns the holds the nodes published for the objects
// other than p, replaced by the tests: the ones their configurations say, and
// the ones their resources up were found holding, as a filesystem on the
// device another resource exposes, which no configuration says.
var publishedHolds = func(p naming.Path) []publishedHold {
	l := make([]publishedHold, 0)
	for _, e := range instance.ConfigData.GetAll() {
		if e.Path == p || e.Value == nil || e.Value.ActorConfig == nil {
			continue
		}
		for _, h := range e.Value.Holds {
			l = append(l, publishedHold{Path: e.Path, ObjectHold: h})
		}
	}
	for _, e := range instance.StatusData.GetAll() {
		if e.Path == p || e.Value == nil {
			continue
		}
		for rid, rstat := range e.Value.Resources {
			for _, h := range rstat.Holds {
				l = append(l, publishedHold{Path: e.Path, ObjectHold: resource.ObjectHold{RID: rid, Hold: h, Nodes: []string{e.Node}}})
			}
		}
	}
	return l
}

// refuseHoldConflicts refuses the configuration body of the object at p when
// its resources would hold, on the nodes of scope, a value another object's
// resources hold there: a mount point, a block device, a volume group name,
// an address. Provisioning it would take what the other holds, as a clone
// mounting its source's filesystem, or a configuration restored from another
// cluster naming a device in use here.
//
// Only the holds the write adds are judged: a hold the object has already,
// on the same nodes, was checked when it was written, and a cluster holding
// conflicting objects already does not refuse their unrelated changes.
//
// With force, root writes the configuration all the same.
func refuseHoldConflicts(ctx echo.Context, p naming.Path, body []byte, scope []string, force bool) error {
	switch p.Kind {
	case naming.KindSvc, naming.KindVol:
	default:
		return nil
	}
	if force {
		if !grantsFromContext(ctx).HasGrant(rbac.GrantRoot) {
			return fmt.Errorf("%w: force requires the root grant", ErrDenied)
		}
		return nil
	}
	to, err := object.NewConfigurer(p, object.WithConfigData(body), object.WithVolatile(true), object.WithStaticReferences())
	if err != nil {
		return err
	}
	holds := object.Holds(p, to, scope)
	if p.Exists() {
		if from, err := object.NewConfigurer(p, object.WithVolatile(true), object.WithStaticReferences()); err == nil {
			holds = addedHolds(holds, object.Holds(p, from, scope))
		}
	}
	if len(holds) == 0 {
		return nil
	}
	others := publishedHolds(p)
	type conflictKey struct {
		rid   string
		hold  resource.Hold
		path  naming.Path
		other string
	}
	where := make(map[conflictKey][]string)
	everywhere := make(map[conflictKey]bool)
	order := make([]conflictKey, 0)
	for _, h := range holds {
		for _, o := range others {
			if !h.Overlaps(o.ObjectHold) {
				continue
			}
			k := conflictKey{rid: h.RID, hold: h.Hold, path: o.Path, other: o.RID}
			if _, ok := where[k]; !ok {
				where[k] = make([]string, 0)
				order = append(order, k)
			}
			nodes, all := overlapNodes(h, o.ObjectHold)
			if all {
				everywhere[k] = true
			}
			for _, n := range nodes {
				if !slices.Contains(where[k], n) {
					where[k] = append(where[k], n)
				}
			}
		}
	}
	if len(order) == 0 {
		return nil
	}
	l := make([]string, len(order))
	for i, k := range order {
		on := "every node"
		if !everywhere[k] {
			slices.Sort(where[k])
			on = strings.Join(where[k], ", ")
		}
		l[i] = fmt.Sprintf("%s: %s %s on %s is held by %s %s", k.rid, k.hold.Class, k.hold.Value, on, k.path, k.other)
	}
	return holdConflictError{conflicts: l}
}

// addedHolds returns the holds of l the object does not have already, as
// current says: a value it holds anew, or on nodes it did not hold it on.
func addedHolds(l, current []resource.ObjectHold) []resource.ObjectHold {
	added := make([]resource.ObjectHold, 0)
	for _, h := range l {
		if !slices.ContainsFunc(current, func(c resource.ObjectHold) bool {
			if c.RID != h.RID || c.Hold != h.Hold {
				return false
			}
			if c.Cluster {
				return true
			}
			if h.Cluster {
				return false
			}
			for _, n := range h.Nodes {
				if !slices.Contains(c.Nodes, n) {
					return false
				}
			}
			return true
		}) {
			added = append(added, h)
		}
	}
	return added
}

// overlapNodes returns the nodes two overlapping holds hold their value on,
// and true when it is every cluster node.
func overlapNodes(a, b resource.ObjectHold) ([]string, bool) {
	switch {
	case a.Cluster && b.Cluster:
		return nil, true
	case a.Cluster:
		return b.Nodes, false
	case b.Cluster:
		return a.Nodes, false
	}
	l := make([]string, 0)
	for _, n := range a.Nodes {
		if slices.Contains(b.Nodes, n) {
			l = append(l, n)
		}
	}
	return l, false
}
