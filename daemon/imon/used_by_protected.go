package imon

import (
	"fmt"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
)

// kindsUsedByObjects are the kinds of the objects other objects use, which an
// object leaving its unprovision to the sysadmin keeps from a purge or a
// delete: the datastores its resources read keys of, and the vol objects its
// volume resources are.
var kindsUsedByObjects = naming.NewKinds(naming.KindCfg, naming.KindSec, naming.KindUsr, naming.KindVol)

// usedByProtectedRefusal says why a purge or a delete of the object is
// refused upfront, empty when it is not: an object using it leaves its
// unprovision to the sysadmin, unprovision=false, on a node where its
// instance is provisioned. The object goes on using it there, a vol for its
// data, a datastore for the keys it installs at each start, and would be left
// broken, as the instance of a volume whose vol is gone.
//
// Each node evaluates, and publishes with the instance configuration, the
// unprovision keyword and the objects the instance uses, so a scoped
// unprovision@node protects on the nodes it says. The refusal holds whoever
// asks, the admin of the namespace of the object among them: the object
// protecting it may live in another namespace, a datastore shared with it.
//
// An instance the sysadmin freed, and declared so with --state-only, uses
// nothing any more: its purge goes on, so do the ones of what it used.
func (t *Manager) usedByProtectedRefusal() string {
	if !kindsUsedByObjects.Has(t.path.Kind) {
		return ""
	}
	nodesByUser := make(map[naming.Path][]string)
	for _, e := range instance.ConfigData.GetAll() {
		if e.Path == t.path || e.Value == nil || e.Value.ActorConfig == nil {
			continue
		}
		if !e.Value.UnprovisionDisabled || !e.Value.UsesObject(t.path) {
			continue
		}
		if freedInstance(e.Path, e.Node) {
			continue
		}
		nodesByUser[e.Path] = append(nodesByUser[e.Path], e.Node)
	}
	if len(nodesByUser) == 0 {
		return ""
	}
	users := make(naming.Paths, 0, len(nodesByUser))
	for p := range nodesByUser {
		users = append(users, p)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].String() < users[j].String() })
	l := make([]string, 0, len(users))
	for _, p := range users {
		nodes := nodesByUser[p]
		sort.Strings(nodes)
		l = append(l, fmt.Sprintf("%s on %s", p, strings.Join(nodes, ", ")))
	}
	return fmt.Sprintf("used by %s, provisioned there with its unprovision disabled (unprovision=false): free it first, unprovisioning it by hand and declaring it with 'instance unprovision --state-only', or remove the reference", strings.Join(l, "; "))
}

// childInstanceProtected says why the child instance of a relation on a node
// holds its parent provisioned for good, empty when it does not: it is
// provisioned, and leaves its unprovision to the sysadmin there,
// unprovision=false, so no orchestration unprovisions it, and the parent, as
// the vol of its volume resource, waiting for it would wait for ever.
//
// A relation to a whole object is not looked at, as childInstanceProvisioned
// does not.
func childInstanceProtected(relation string) string {
	p, node, err := naming.Relation(relation).Split()
	if err != nil || node == "" {
		return ""
	}
	cfg := instance.ConfigData.GetByPathAndNode(p, node)
	if cfg == nil || cfg.ActorConfig == nil || !cfg.UnprovisionDisabled {
		return ""
	}
	st := instance.StatusData.GetByPathAndNode(p, node)
	if st == nil || !st.Provisioned.IsOneOf(provisioned.True, provisioned.Mixed) {
		return ""
	}
	return fmt.Sprintf("%s is provisioned with its unprovision disabled (unprovision=false): unprovision it by hand there, then declare it with 'instance unprovision --state-only'", relation)
}

// protectedChild returns why a child of the relations holds its parent
// provisioned for good, the first one of them in the order of the relations,
// empty when none does.
func protectedChild(children map[string]status.T) string {
	relations := make([]string, 0, len(children))
	for relation := range children {
		relations = append(relations, relation)
	}
	sort.Strings(relations)
	for _, relation := range relations {
		if reason := childInstanceProtected(relation); reason != "" {
			return reason
		}
	}
	return ""
}

// failTeardownHeldByProtectedChild ends the unprovision, or the purge, of the
// local instance as a failure, with the reason: a child instance holds it
// provisioned for good, so waiting for it would never end, as an
// unprovision of a vol a stopped service leaving its unprovision to the
// sysadmin holds did, until the client gave up waiting.
func (t *Manager) failTeardownHeldByProtectedChild(reason string) {
	state := instance.MonitorStateUnprovisionFailure
	if t.state.GlobalExpect == instance.MonitorGlobalExpectPurged {
		state = instance.MonitorStatePurgeFailed
	}
	t.log.Warnf("%s orchestration: %s -> %s", t.state.GlobalExpect, reason, state)
	t.transitionTo(state)
	t.done()
}

// freedInstance says the instance of p on node holds nothing: it is not
// provisioned, and no action or orchestration is under way that could
// provision it, which an unprovisioned status alone does not say while a
// provision is queued or running.
func freedInstance(p naming.Path, node string) bool {
	st := instance.StatusData.GetByPathAndNode(p, node)
	if st == nil || st.Provisioned != provisioned.False {
		return false
	}
	mon := instance.MonitorData.GetByPathAndNode(p, node)
	if mon == nil {
		return true
	}
	if mon.State.IsOneOf(instance.MonitorStatesFailure...) {
		// A failed action is terminal, nothing retries it: the instance,
		// unprovisioned, holds nothing until someone acts on it again.
		return true
	}
	switch mon.GlobalExpect {
	case instance.MonitorGlobalExpectNone, instance.MonitorGlobalExpectInit:
	default:
		return false
	}
	return mon.State == instance.MonitorStateIdle
}
