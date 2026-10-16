package imon

import (
	"fmt"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// affinityPaths returns the objects the affinities of the object name.
func (t *Manager) affinityPaths() naming.Paths {
	cfg := t.instConfig.ActorConfig
	if cfg == nil {
		return nil
	}
	var l naming.Paths
	for _, paths := range []naming.Paths{cfg.HardAffinity, cfg.HardAntiAffinity, cfg.SoftAffinity, cfg.SoftAntiAffinity} {
		for _, p := range paths {
			if p != t.path && !slices.Contains(l, p) {
				l = append(l, p)
			}
		}
	}
	return l
}

// janitorAffinities subscribes to the instance status updates of the objects
// the affinities name, and unsubscribes from those they no longer name: a
// change of their instances may change the nodes the object can start on.
// The namespace label makes the filters ones of their own, a relation
// watching the same object subscribing with the path label only.
func (t *Manager) janitorAffinities() {
	paths := t.affinityPaths()
	for _, p := range paths {
		if _, ok := t.affinitySubscribed[p]; ok {
			continue
		}
		t.log.Infof("subscribe to %s instance status updates and deletes, named by an affinity", p)
		t.sub.AddFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()})
		t.sub.AddFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()})
		t.affinitySubscribed[p] = struct{}{}
	}
	for p := range t.affinitySubscribed {
		if slices.Contains(paths, p) {
			continue
		}
		t.log.Infof("unsubscribe from %s instance status updates and deletes, no longer named by an affinity", p)
		t.sub.DelFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()})
		t.sub.DelFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()})
		delete(t.affinitySubscribed, p)
		for k := range t.affinityAvail {
			if strings.HasPrefix(k, p.String()+"@") {
				delete(t.affinityAvail, k)
			}
		}
	}
}

// onAffinityInstanceStatus reevaluates the start candidates when the avail
// status of an instance of an object an affinity names changes, deleted
// being undef.
func (t *Manager) onAffinityInstanceStatus(p naming.Path, node string, avail status.T) {
	if _, ok := t.affinitySubscribed[p]; !ok {
		return
	}
	k := p.String() + "@" + node
	if last, ok := t.affinityAvail[k]; ok && last == avail {
		return
	}
	t.log.Infof("affinity %s avail status change %s -> %s", k, t.affinityAvail[k], avail)
	t.affinityAvail[k] = avail
	t.onChange()
}

// affineAvail returns the avail status of the instance of p on node,
// undef when there is none.
func affineAvail(p naming.Path, node string) status.T {
	if st := instance.StatusData.GetByPathAndNode(p, node); st != nil {
		return st.Avail
	}
	return status.Undef
}

// affinityViolation is a hard affinity refusing a start on a node, and the
// object of a hard affinity cycle the object waits for to start first, if
// that is what refuses it.
type affinityViolation struct {
	text    string
	follows naming.Path
}

// affinityViolations says why the object may not start on node, as the start
// command does: an object its hard affinity names is not up there, or one its
// hard anti affinity names is.
func (t *Manager) affinityViolations(node string) []affinityViolation {
	cfg := t.instConfig.ActorConfig
	if cfg == nil {
		return nil
	}
	var l []affinityViolation
	for _, p := range cfg.HardAffinity {
		switch avail := affineAvail(p, node); avail {
		case status.Up, status.NotApplicable:
		default:
			cycle := t.hardAffinityCycle(p)
			switch {
			case len(cycle) == 0:
				l = append(l, affinityViolation{text: fmt.Sprintf("hard affinity with %s not satisfied on %s (%s)", p, node, avail)})
			case cycle[0] == t.path:
				// This object starts first, the others follow it.
			default:
				l = append(l, affinityViolation{
					text:    fmt.Sprintf("hard affinity with %s not satisfied on %s (%s), %s starting first among %s", p, node, avail, cycle[0], cycle),
					follows: cycle[0],
				})
			}
		}
	}
	for _, p := range cfg.HardAntiAffinity {
		if instance.StatusData.GetByPathAndNode(p, node) == nil {
			continue
		}
		switch avail := affineAvail(p, node); avail {
		case status.Down, status.StandbyUp, status.StandbyDown, status.NotApplicable:
		default:
			l = append(l, affinityViolation{text: fmt.Sprintf("hard anti affinity with %s not satisfied on %s (%s)", p, node, avail)})
		}
	}
	return l
}

// hardAffinityViolations says why the object may not start on node.
func (t *Manager) hardAffinityViolations(node string) []string {
	var l []string
	for _, v := range t.affinityViolations(node) {
		l = append(l, v.text)
	}
	return l
}

// affinityConfig returns the actor configuration of the object p, as a node
// publishes it, the local one first.
func (t *Manager) affinityConfig(p naming.Path) (*instance.Config, bool) {
	if p == t.path {
		return &t.instConfig, t.instConfig.ActorConfig != nil
	}
	if cfg := instance.ConfigData.GetByPathAndNode(p, t.localhost); cfg != nil && cfg.ActorConfig != nil {
		return cfg, true
	}
	for _, cfg := range instance.ConfigData.GetByPath(p) {
		if cfg != nil && cfg.ActorConfig != nil {
			return cfg, true
		}
	}
	return nil, false
}

// hardAffinityReach returns the objects the hard affinities lead to from p,
// p included.
func (t *Manager) hardAffinityReach(p naming.Path) map[naming.Path]bool {
	seen := map[naming.Path]bool{p: true}
	todo := []naming.Path{p}
	for len(todo) > 0 {
		cur := todo[0]
		todo = todo[1:]
		cfg, ok := t.affinityConfig(cur)
		if !ok {
			continue
		}
		for _, next := range cfg.ActorConfig.HardAffinity {
			if !seen[next] {
				seen[next] = true
				todo = append(todo, next)
			}
		}
	}
	return seen
}

// hardAffinityCycle returns the objects whose hard affinities lead to each
// other, through p, the object and p among them, when none of them is up on
// any node: none can start where another is up, so one of them starts first
// and the others follow it. The first of the list is the one starting first:
// the smallest priority, then the first path in alphanumeric order. It
// returns nil when p leads back to no cycle with the object, or a member of
// the cycle is up somewhere, which the others follow.
func (t *Manager) hardAffinityCycle(p naming.Path) naming.Paths {
	if !t.hardAffinityReach(p)[t.path] {
		return nil
	}
	var l naming.Paths
	for member := range t.hardAffinityReach(t.path) {
		if member != t.path && !t.hardAffinityReach(member)[t.path] {
			continue
		}
		for _, st := range instance.StatusData.GetByPath(member) {
			if st != nil && st.Avail.Is(status.Up, status.Warn) {
				return nil
			}
		}
		l = append(l, member)
	}
	priority := func(p naming.Path) int {
		if cfg, ok := t.affinityConfig(p); ok {
			return int(cfg.Priority)
		}
		return 0
	}
	slices.SortFunc(l, func(a, b naming.Path) int {
		if d := priority(a) - priority(b); d != 0 {
			return d
		}
		return strings.Compare(a.String(), b.String())
	})
	return l
}

// isHardAffinitySatisfied says the hard affinities let the object start on
// node.
func (t *Manager) isHardAffinitySatisfied(node string) bool {
	return len(t.hardAffinityViolations(node)) == 0
}

// hardAffinityRefusal says why no node of nodes can start the object, the
// hard affinities excluding every one, and returns "" when one can.
//
// A node refused only for waiting on the object of a hard affinity cycle
// starting first can: a start asked for it follows that object, started
// with it or later, and the objects it waits for are returned.
func (t *Manager) hardAffinityRefusal(nodes []string) (string, naming.Paths) {
	var (
		l       []string
		follows naming.Paths
	)
	for _, node := range nodes {
		violations := t.affinityViolations(node)
		if len(violations) == 0 {
			return "", nil
		}
		waits := true
		for _, v := range violations {
			l = append(l, v.text)
			if v.follows.IsZero() {
				waits = false
			} else if !slices.Contains(follows, v.follows) {
				follows = append(follows, v.follows)
			}
		}
		if waits {
			return "", follows
		}
	}
	return strings.Join(l, ", "), nil
}

// softAffinityScore counts the objects the soft affinity names up on node,
// less those the soft anti affinity names.
func (t *Manager) softAffinityScore(node string) int {
	cfg := t.instConfig.ActorConfig
	if cfg == nil {
		return 0
	}
	score := 0
	for _, p := range cfg.SoftAffinity {
		if affineAvail(p, node) == status.Up {
			score++
		}
	}
	for _, p := range cfg.SoftAntiAffinity {
		if affineAvail(p, node) == status.Up {
			score--
		}
	}
	return score
}

// sortBySoftAffinity moves ahead the candidates the soft affinities prefer,
// keeping the placement policy order among those they rank the same, as v2
// did.
func (t *Manager) sortBySoftAffinity(candidates []string) []string {
	cfg := t.instConfig.ActorConfig
	if cfg == nil || len(cfg.SoftAffinity)+len(cfg.SoftAntiAffinity) == 0 {
		return candidates
	}
	scores := make(map[string]int, len(candidates))
	for _, node := range candidates {
		scores[node] = t.softAffinityScore(node)
	}
	l := slices.Clone(candidates)
	slices.SortStableFunc(l, func(a, b string) int {
		return scores[b] - scores[a]
	})
	return l
}
