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

// hardAffinityViolations says why the object may not start on node, as the
// start command does: an object its hard affinity names is not up there, or
// one its hard anti affinity names is.
func (t *Manager) hardAffinityViolations(node string) []string {
	cfg := t.instConfig.ActorConfig
	if cfg == nil {
		return nil
	}
	var l []string
	for _, p := range cfg.HardAffinity {
		switch avail := affineAvail(p, node); avail {
		case status.Up, status.NotApplicable:
		default:
			l = append(l, fmt.Sprintf("hard affinity with %s not satisfied on %s (%s)", p, node, avail))
		}
	}
	for _, p := range cfg.HardAntiAffinity {
		if instance.StatusData.GetByPathAndNode(p, node) == nil {
			continue
		}
		switch avail := affineAvail(p, node); avail {
		case status.Down, status.StandbyUp, status.StandbyDown, status.NotApplicable:
		default:
			l = append(l, fmt.Sprintf("hard anti affinity with %s not satisfied on %s (%s)", p, node, avail))
		}
	}
	return l
}

// isHardAffinitySatisfied says the hard affinities let the object start on
// node.
func (t *Manager) isHardAffinitySatisfied(node string) bool {
	return len(t.hardAffinityViolations(node)) == 0
}

// hardAffinityRefusal says why no node of nodes can start the object, the
// hard affinities excluding every one, and returns "" when one can.
func (t *Manager) hardAffinityRefusal(nodes []string) string {
	var l []string
	for _, node := range nodes {
		violations := t.hardAffinityViolations(node)
		if len(violations) == 0 {
			return ""
		}
		l = append(l, violations...)
	}
	return strings.Join(l, ", ")
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
