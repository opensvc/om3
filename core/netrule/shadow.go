package netrule

import (
	"fmt"
	"net/netip"

	"github.com/opensvc/om3/v3/core/naming"
)

// Shadowed returns the rules that can never be the first to match a
// packet, each with the reason: a rule written before it matches all it
// names, whatever the cluster holds. namespace is the one of the object of
// the rules, which ./ and . stand for.
//
// The first rule matching decides, so such a rule decides of nothing: it is
// what a validation warns of, most often a rule added after a last allow
// any, where an append to the keyword puts it.
//
// Only what the rules say by themselves is judged, a rule against each rule
// before it: any rule after an allow any or a deny any, a node after the
// cluster, an object after its namespace or after itself whole, the part a
// node hands out of a network after that network, an address or a range
// after a range holding it, a rule written twice. Nothing is resolved: two
// rules naming the same addresses on this cluster today, as a node and its
// address, are not found, nor a rule several rules cover together. And the
// verbs are not compared: a rule after one matching all it names is never
// reached, whether it would say the same or the opposite.
//
// A rule on ports matches the packets of its ports alone, and so shadows
// the rules on ports within its own.
func Shadowed(rules Rules, namespace string) Notes {
	notes := make(Notes, 0)
	for i, rule := range rules {
		for _, before := range rules[:i] {
			if before.covers(rule, namespace) {
				notes = append(notes, Note{Rule: rule, Reason: fmt.Sprintf("never matches: %s, written before it, matches all it names", before.Text)})
				break
			}
		}
	}
	return notes
}

// HasAllow says a rule of the list lets something through: a list with
// none denies everything but what the implicit rules allow, the loopback
// addresses and the nameservers, which is seldom what its writer means, a
// list of deny rules being ended by an allow any to deny them alone.
func (t Rules) HasAllow() bool {
	for _, rule := range t {
		if rule.Verb == VerbAllow {
			return true
		}
	}
	return false
}

// covers says the rule matches every packet the other does, by what the
// two rules say alone.
func (t Rule) covers(o Rule, namespace string) bool {
	if t.HasPort && (!o.HasPort || t.Proto != o.Proto || o.PortMin < t.PortMin || o.PortMax > t.PortMax) {
		return false
	}
	if t.Kind == KindAny {
		return true
	}
	if a, ok := t.static(); ok {
		b, ok := o.static()
		if !ok {
			return false
		}
		for _, p := range b {
			if !within(a, p) {
				return false
			}
		}
		return true
	}
	switch t.Kind {
	case KindDNS:
		return o.Kind == KindDNS
	case KindCluster:
		return o.Kind == KindCluster || o.Kind == KindNode
	case KindNode:
		return o.Kind == KindNode && o.Node == t.Node
	case KindNetwork:
		return o.Kind == KindNetwork && o.Network == t.Network && (t.Node == "" || t.Node == o.Node)
	case KindAddress:
		// Reading a key: the same key of the same datastore.
		return o.Kind == KindAddress && o.From != "" && o.Key == t.Key && sameName(t.From, o.From, namespace)
	case KindObject:
		return o.Kind == KindObject && sameName(t.Object, o.Object, namespace) && (t.RID == "" || t.RID == o.RID)
	case KindNamespace:
		name := t.NamespaceName(namespace)
		switch o.Kind {
		case KindNamespace:
			return o.NamespaceName(namespace) == name
		case KindObject:
			p, err := o.ObjectPath(namespace)
			return err == nil && p.Namespace == name
		}
	}
	return false
}

// static returns the ranges of a rule naming them by itself, whatever the
// cluster: the loopback addresses, an address or a range, every address.
func (t Rule) static() ([]netip.Prefix, bool) {
	switch {
	case t.Kind == KindLocalhost:
		return localhost, true
	case t.Kind == KindAny:
		return Any(), true
	case t.Kind == KindAddress && t.From == "":
		return []netip.Prefix{t.Prefix}, true
	}
	return nil, false
}

// within says a range of l holds the range p.
func within(l []netip.Prefix, p netip.Prefix) bool {
	for _, a := range l {
		if contains(a, p) {
			return true
		}
	}
	return false
}

// sameName says the two paths, of objects or of datastores, as rules write
// them, name the same for an object of the namespace given: ./svc/db and
// the path spelling the namespace are one. Paths that are none are the same
// when written the same.
func sameName(a, b, namespace string) bool {
	pa, errA := naming.ParsePathRel(a, namespace)
	pb, errB := naming.ParsePathRel(b, namespace)
	if errA != nil || errB != nil {
		return a == b
	}
	return pa == pb
}
