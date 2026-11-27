package netrule

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
)

type (
	// View is what the rules are resolved over: the networks and the
	// nodes of the cluster, as the node resolving knows them, and the
	// nameservers of what the rules are resolved for.
	//
	// The driver starting a resource reads the networks and the nodes from
	// the configuration of its node, and asks the daemon the addresses of
	// the objects its rules name, which only the daemon knows. The daemon
	// resolves over its own data, to keep the filter of a running resource
	// current.
	View interface {
		// Network returns the ranges of a cluster network, and false
		// for a network the cluster has none of that name.
		Network(name string) ([]netip.Prefix, bool)

		// NetworkNode returns the part of a cluster network a node hands
		// out, none for a network giving the nodes no part of their own,
		// and false for a network or a node the cluster does not have.
		NetworkNode(name, node string) ([]netip.Prefix, bool)

		// Node returns the addresses of a cluster node, the one its
		// listener is on and the ones its name resolves to, and false
		// for a node the cluster does not have.
		Node(name string) ([]netip.Addr, bool)

		// Nodes returns the names of the cluster nodes.
		Nodes() []string

		// DNS returns the nameservers what is filtered resolves
		// through: the ones of the node for a command run on it, the
		// ones of its own resolver for a container. Opening more, as
		// the nameservers of the cluster for a command that asks the
		// stub resolver of its node, would open hosts it never talks
		// to, on every port.
		DNS() []netip.Addr

		// Object returns the addresses the resources of an object
		// publish, the instances of all the nodes together, and false
		// for an object the cluster does not have.
		Object(path naming.Path) (ObjectAddrs, bool)

		// NamespaceObjects returns the objects of a namespace the
		// cluster has.
		NamespaceObjects(namespace string) []naming.Path

		// Key returns the value of a key of a datastore, a cfg or a
		// sec, as the node resolving holds it, decoded, for a rule
		// of an object of the namespace given: an error that is
		// ErrNoDatastore for a datastore the node does not have,
		// ErrNotShared for one of another namespace whose share
		// keyword does not admit the namespace given, and ErrNoKey
		// for a key the datastore does not have.
		//
		// The view is what holds the datastore, so it is the one
		// place its sharing is judged, at every read: a share taken
		// back is a key no longer read.
		Key(datastore naming.Path, key, namespace string) ([]byte, error)
	}

	// Foreign counts what the rules whose addresses are not shown add
	// to the filter of the resource: the rules naming another namespace
	// than the one of the object, whose addresses are the business of the
	// namespace they are in, and the rules reading a key of a sec, told
	// to who may decode it. It is what the status of the resource says of
	// them.
	Foreign struct {
		// Rules is the number of rules naming an object of another
		// namespace, or another namespace.
		Rules int `json:"rules"`

		// Secret is the number of rules reading a key of a sec.
		Secret int `json:"secret,omitempty"`

		// Addrs is the number of addresses they resolve to, all
		// together.
		Addrs int `json:"addrs"`
	}

	// Step is what a rule resolves to: the ranges a packet is let
	// through from or to, or dropped, when no step before it matched.
	//
	// A step on ports matches a packet from or to its ranges on these
	// ports of the protocol alone. The port is the one of the address
	// named, not the one of what is filtered: a packet sent to a range
	// matches by its destination port, and one received from it by its
	// source port. So a rule opens, or closes, a service of what it
	// names.
	Step struct {
		// Verb says the packets matching are let through, or dropped.
		Verb Verb `json:"verb"`

		// Ranges are the addresses the rule resolved to, none for a
		// rule resolving to nothing for now.
		Ranges []netip.Prefix `json:"ranges"`

		// Proto is the protocol of a step on ports, tcp or udp, and
		// Min and Max the first and the last port. A step on no port
		// has no protocol.
		Proto string `json:"proto,omitempty"`
		Min   uint16 `json:"min,omitempty"`
		Max   uint16 `json:"max,omitempty"`

		// Implicit says the step is one of a rule no keyword wrote,
		// see Rules.WithImplicit.
		Implicit bool `json:"implicit,omitempty"`
	}

	// Filter is what the rules of a resource resolve to, in the form a
	// backend applies: a step per rule, in the order of the rules, the
	// implicit ones included. A packet is judged by the first step it
	// matches, and passes when it matches none.
	//
	// A filter made of rules ends with the step of the implicit deny
	// any, so nothing of it passes unmatched. One that does not is what
	// a backend knowing no order reports of itself, see Flattened, or
	// what a record written before the rules were ordered holds.
	//
	// The steps are one per rule, also for a rule resolving to nothing:
	// a backend making a set of each step keeps the same sets as the
	// addresses move, and replaces their elements.
	Filter struct {
		Steps []Step `json:"steps"`
	}

	// Note is a rule resolving to no address, and why.
	Note struct {
		Rule   Rule
		Reason string

		// Refused says the rule is one that may not be written, not
		// one naming what does not exist yet: it reads a datastore
		// of another namespace that does not share its keys with the
		// one of the object.
		Refused bool
	}

	// Notes are the rules of a keyword resolving to no address.
	Notes []Note
)

func (t Note) String() string {
	return t.Rule.Text + ": " + t.Reason
}

// HasPort says the step matches on ports.
func (t Step) HasPort() bool {
	return t.Proto != ""
}

// isAny says the step matches every packet.
func (t Step) isAny() bool {
	return !t.HasPort() && coversAny(Normalize(t.Ranges))
}

// equal says the two steps judge the same packets the same way.
func (t Step) equal(o Step) bool {
	return t.Verb == o.Verb && t.Proto == o.Proto && t.Min == o.Min && t.Max == o.Max &&
		slices.Equal(Normalize(t.Ranges), Normalize(o.Ranges))
}

// describe says the step in the words of a filter: "allow 10.0.0.0/8", a
// step on ports as "allow tcp/5432@10.0.0.5/32".
func (t Step) describe() string {
	words := make([]string, 0, len(t.Ranges)+2)
	if t.Implicit {
		words = append(words, "implicit")
	}
	words = append(words, string(t.Verb))
	ports := ""
	if t.HasPort() {
		ports = fmt.Sprintf("%s/%d@", t.Proto, t.Min)
		if t.Max != t.Min {
			ports = fmt.Sprintf("%s/%d-%d@", t.Proto, t.Min, t.Max)
		}
	}
	switch l := Normalize(t.Ranges); {
	case coversAny(l) && ports == "":
		words = append(words, "any")
	default:
		for _, prefix := range l {
			words = append(words, ports+prefix.String())
		}
	}
	return strings.Join(words, " ")
}

// IsZero says the filter filters nothing: no step of it names an address.
func (t Filter) IsZero() bool {
	for _, step := range t.Steps {
		if len(step.Ranges) > 0 {
			return false
		}
	}
	return true
}

// HasPorts says the filter is one a backend knowing no port can not apply.
func (t Filter) HasPorts() bool {
	return slices.ContainsFunc(t.Steps, Step.HasPort)
}

// Equal says the two filters are the same steps, in the same order: they
// let the same packets through, and a backend making a set of each step
// holds the same sets for both.
func (t Filter) Equal(o Filter) bool {
	return slices.EqualFunc(t.Steps, o.Steps, Step.equal)
}

func (t Filter) String() string {
	l := make([]string, 0, len(t.Steps))
	for _, step := range t.Steps {
		if len(step.Ranges) == 0 {
			l = append(l, step.describe()+" nothing")
			continue
		}
		l = append(l, step.describe())
	}
	return strings.Join(l, ", ")
}

// Describe says the filter in a line, in the order a packet is judged:
// "allow 10.0.1.5/32, deny 10.0.0.0/8, implicit allow 127.0.0.0/8 ::1/128,
// implicit deny any". A range on ports is said tcp/5432@10.0.0.5/32.
//
// A step naming no address for now matches nothing, and is not said. The
// steps after one matching every packet are never reached, and are not said
// either: after an allow any, the implicit rules decide of nothing.
func (t Filter) Describe() string {
	l := make([]string, 0, len(t.Steps))
	for _, step := range t.Steps {
		if len(step.Ranges) == 0 {
			continue
		}
		l = append(l, step.describe())
		if step.isAny() {
			break
		}
	}
	if len(l) == 0 {
		return "no filter"
	}
	return strings.Join(l, ", ")
}

// Flat returns the filter as an allowed and a denied list, for a backend
// knowing no order, as systemd: one where a packet from or to an allowed
// range passes, one from or to a denied range and no allowed one is
// dropped, and any other passes. The two lists let through exactly what the
// steps do.
//
// The denied list is every range a deny step names. The allowed list is,
// for each allow step, its ranges less the ones of the deny steps before
// it: an address a deny step matches first is in no allowed range, so it is
// dropped, and one an allow step matches first was taken out of none, so it
// passes, whatever denies it after.
//
// The steps after one matching every packet are never reached, and left
// out. When that step allows, it is left out too: what reaches it is what
// no deny step before it names, which a backend passing the unmatched lets
// through already. So a list ending with allow any, written to deny a few
// hosts, is these hosts denied, and not every address less them, allowed
// range by range around each.
//
// The steps on ports are none of such a backend, see HasPorts.
func (t Filter) Flat() (allow, deny []netip.Prefix) {
	allow = make([]netip.Prefix, 0)
	deny = make([]netip.Prefix, 0)
	for _, step := range t.Steps {
		if step.HasPort() {
			continue
		}
		if step.Verb == VerbAllow {
			if step.isAny() {
				break
			}
			allow = append(allow, Subtract(step.Ranges, deny)...)
			continue
		}
		deny = append(deny, step.Ranges...)
		if step.isAny() {
			break
		}
	}
	allow, deny = Normalize(allow), Normalize(deny)
	if coversAny(allow) {
		// Allow steps that are every address together, with no deny
		// step before them: everything passes.
		return []netip.Prefix{}, []netip.Prefix{}
	}
	return allow, deny
}

// Flattened returns the filter as a backend knowing no order holds it: the
// step of its allowed list, then the one of its denied list, see Flat. It
// is what such a backend reports of itself, and what it is compared with.
func (t Filter) Flattened() Filter {
	allow, deny := t.Flat()
	return FlatFilter(allow, deny)
}

// FlatFilter returns the filter of an allowed and a denied list, as a
// backend knowing no order holds them: an allowed range wins, so the
// allowed step comes first.
func FlatFilter(allow, deny []netip.Prefix) Filter {
	return Filter{Steps: []Step{
		{Verb: VerbAllow, Ranges: Normalize(allow)},
		{Verb: VerbDeny, Ranges: Normalize(deny)},
	}}
}

// UnmarshalJSON reads the steps of a filter, and what an om knowing no
// order wrote: an allowed and a denied list, with the ranges on ports
// apart, which it judged in this order: dropped on a denied port, passed
// from or to an allowed range, passed on an allowed port, dropped from or to
// a denied range, and passed otherwise. They are read as the steps saying
// the same.
func (t *Filter) UnmarshalJSON(b []byte) error {
	type ports struct {
		Prefix netip.Prefix `json:"prefix"`
		Proto  string       `json:"proto"`
		Min    uint16       `json:"min"`
		Max    uint16       `json:"max"`
	}
	var v struct {
		Steps      *[]Step        `json:"steps"`
		Allow      []netip.Prefix `json:"allow"`
		Deny       []netip.Prefix `json:"deny"`
		AllowPorts []ports        `json:"allow_ports"`
		DenyPorts  []ports        `json:"deny_ports"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if v.Steps != nil {
		t.Steps = *v.Steps
		return nil
	}
	t.Steps = nil
	if len(v.Allow) == 0 && len(v.Deny) == 0 && len(v.AllowPorts) == 0 && len(v.DenyPorts) == 0 {
		return nil
	}
	on := func(verb Verb, l []ports) {
		for _, p := range l {
			t.Steps = append(t.Steps, Step{Verb: verb, Ranges: []netip.Prefix{p.Prefix}, Proto: p.Proto, Min: p.Min, Max: p.Max})
		}
	}
	on(VerbDeny, v.DenyPorts)
	t.Steps = append(t.Steps, Step{Verb: VerbAllow, Ranges: Normalize(v.Allow)})
	on(VerbAllow, v.AllowPorts)
	t.Steps = append(t.Steps, Step{Verb: VerbDeny, Ranges: Normalize(v.Deny)})
	return nil
}

// Any says rules add to the filter addresses that are not shown.
func (t Foreign) Any() bool {
	return t.Rules > 0 || t.Secret > 0
}

// Describe says what the rules whose addresses are not shown add to a
// filter, to follow what is shown of it, and nothing when they add nothing.
func (t Foreign) Describe() string {
	if !t.Any() {
		return ""
	}
	plural := func(n int, word string) string {
		if n == 1 {
			return fmt.Sprintf("%d %s", n, word)
		}
		if strings.HasSuffix(word, "s") {
			return fmt.Sprintf("%d %ses", n, word)
		}
		return fmt.Sprintf("%d %ss", n, word)
	}
	switch {
	case t.Secret == 0:
		return fmt.Sprintf(", and %s of %s naming another namespace", plural(t.Addrs, "address"), plural(t.Rules, "rule"))
	case t.Rules == 0:
		return fmt.Sprintf(", and %s of %s reading a sec", plural(t.Addrs, "address"), plural(t.Secret, "rule"))
	}
	return fmt.Sprintf(", and %s of %s naming another namespace or reading a sec", plural(t.Addrs, "address"), plural(t.Rules+t.Secret, "rule"))
}

// Resolve returns the filter the rules of a resource make over the view,
// and the rules resolving to nothing. namespace is the one of the object of
// the resource, which ./ and . stand for in its rules.
//
// The filter is a step per rule, in their order, then the steps of the
// implicit rules, see Rules.WithImplicit: the first step matching a packet
// decides of it, and what no rule matches is denied. No rule is no filter.
//
// A rule naming what the view does not have, as a network not created yet or
// an object not started, resolves to nothing, and is noted: its step matches
// nothing until what it names exists, so an allow rule lets nothing more
// through, and a deny rule denies nothing, leaving the packets to the rules
// after it.
func Resolve(rules Rules, view View, namespace string) (Filter, Notes) {
	r := resolver{view: view, namespace: namespace}
	return r.resolve(rules)
}

// Public returns the filter the rules make with the ones naming another
// namespace, and the ones reading a sec, resolving to nothing, and what
// those add to it: what is shown of the filter to who reads the status of
// the resource.
func Public(rules Rules, view View, namespace string) (Filter, Foreign) {
	return resolver{view: view, namespace: namespace}.public(rules)
}

// resolver resolves the rules of an object of a namespace over a view.
type resolver struct {
	view      View
	namespace string

	// own has the rules whose addresses are not shown resolve to
	// nothing.
	own bool

	// kept are the addresses the rules naming objects no longer resolve
	// to and the filter keeps, by rule, see Memory: each is one more
	// address of the step of its rule, at the place of that rule.
	kept Memory
}

func (t resolver) public(rules Rules) (Filter, Foreign) {
	var foreign Foreign
	seen := make(map[netip.Prefix]bool)
	for _, rule := range rules {
		if !rule.Hidden(t.namespace) {
			continue
		}
		if rule.Secret() {
			foreign.Secret++
		} else {
			foreign.Rules++
		}
		prefixes, _, _, _ := t.resolveRule(rule)
		for _, prefix := range append(prefixes, t.kept.prefixes(rule)...) {
			seen[prefix] = true
		}
	}
	foreign.Addrs = len(seen)
	t.own = true
	filter, _ := t.resolve(rules)
	return filter, foreign
}

// resolve returns the steps of the rules, then of the implicit ones, and
// the rules written resolving to nothing.
func (t resolver) resolve(rules Rules) (Filter, Notes) {
	var filter Filter
	notes := make(Notes, 0)
	for _, rule := range rules.WithImplicit() {
		step := Step{Verb: rule.Verb, Implicit: rule.Implicit, Ranges: []netip.Prefix{}}
		if rule.HasPort {
			step.Proto, step.Min, step.Max = rule.Proto, rule.PortMin, rule.PortMax
		}
		if !t.own || !rule.Hidden(t.namespace) {
			prefixes, reason, warnings, refused := t.resolveRule(rule)
			// An implicit rule resolving to nothing, as the dns one
			// of a resource with no nameserver, is nothing its
			// writer can act on.
			if !rule.Implicit {
				for _, warning := range warnings {
					notes = append(notes, Note{Rule: rule, Reason: warning})
				}
				if len(prefixes) == 0 {
					if reason == "" {
						reason = "resolves to nothing for now"
					}
					notes = append(notes, Note{Rule: rule, Reason: reason, Refused: refused})
				}
			}
			step.Ranges = Normalize(append(prefixes, t.kept.prefixes(rule)...))
		}
		filter.Steps = append(filter.Steps, step)
	}
	return filter, notes
}

// resolveRule returns the ranges of a rule, and why it has none. The
// warnings are what is wrong with a rule that may resolve all the same, as
// a line of a key that is no address. The last value says the rule is
// refused, see Note.Refused.
func (t resolver) resolveRule(rule Rule) ([]netip.Prefix, string, []string, bool) {
	if rule.Kind == KindAddress && rule.From != "" {
		return t.keyRanges(rule)
	}
	prefixes, reason := t.resolveNamed(rule)
	return prefixes, reason, nil, false
}

// keyRanges returns the ranges an address rule reads from a key of a
// datastore, one per line, why it has none, and the lines that are none.
//
// An empty line, and one starting with #, is skipped. A line that is no
// address and no range is left out and warned of, the others applying: by
// its number, and by its text unless the datastore is a sec, whose keys a
// log is not to tell.
//
// A datastore of another namespace is read when it shares its keys with the
// namespace of the object, as the keys a resource installs or is seeded
// from are: the share keyword of the datastore is the consent of its
// administrator. One that does not resolves to nothing, for an allow rule
// and for a deny rule alike, and the last value says the rule is refused.
// Nothing is said of what such a datastore holds, the key being there or
// not.
func (t resolver) keyRanges(rule Rule) ([]netip.Prefix, string, []string, bool) {
	p, err := rule.Datastore(t.namespace)
	if err != nil {
		return nil, err.Error() + ", resolves to nothing", nil, false
	}
	b, err := t.view.Key(p, rule.Key, t.namespace)
	switch {
	case errors.Is(err, ErrNoDatastore):
		return nil, fmt.Sprintf("no such datastore %s for now, resolves to nothing until it exists", p), nil, false
	case errors.Is(err, ErrNotShared):
		return nil, fmt.Sprintf("the datastore %s is of another namespace, and its share keyword does not share its keys with the namespace %s, resolves to nothing", p, t.namespace), nil, true
	case errors.Is(err, ErrNoKey):
		return nil, fmt.Sprintf("no such key %s in the datastore %s for now, resolves to nothing until it is set", rule.Key, p), nil, false
	case err != nil:
		return nil, fmt.Sprintf("the key %s of the datastore %s is not read, resolves to nothing: %s", rule.Key, p, err), nil, false
	}
	prefixes := make([]netip.Prefix, 0)
	warnings := make([]string, 0)
	for i, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		prefix, err := parsePrefix(line)
		switch {
		case err == nil:
			prefixes = append(prefixes, prefix)
		case p.Kind == naming.KindSec:
			warnings = append(warnings, fmt.Sprintf("the line %d of the key %s of the datastore %s is no address and no range, left out", i+1, rule.Key, p))
		default:
			warnings = append(warnings, fmt.Sprintf("the line %d of the key %s of the datastore %s, %q, is no address and no range, left out", i+1, rule.Key, p, line))
		}
	}
	return prefixes, fmt.Sprintf("the key %s of the datastore %s holds no address, resolves to nothing", rule.Key, p), warnings, false
}

// resolveNamed returns the ranges of a rule naming what the view tells, and
// why it has none.
func (t resolver) resolveNamed(rule Rule) ([]netip.Prefix, string) {
	view := t.view
	switch rule.Kind {
	case KindLocalhost:
		return localhost, ""
	case KindAny:
		return Any(), ""
	case KindDNS:
		return addrPrefixes(view.DNS()), "the resource has no nameserver, resolves to nothing"
	case KindAddress:
		return []netip.Prefix{rule.Prefix}, ""
	case KindNetwork:
		if rule.Node == "" {
			l, ok := view.Network(rule.Network)
			if !ok {
				return nil, fmt.Sprintf("no such network %s for now, resolves to nothing until it exists", rule.Network)
			}
			return l, fmt.Sprintf("the network %s has no address range, resolves to nothing", rule.Network)
		}
		if _, ok := view.Network(rule.Network); !ok {
			return nil, fmt.Sprintf("no such network %s for now, resolves to nothing until it exists", rule.Network)
		}
		l, ok := view.NetworkNode(rule.Network, rule.Node)
		if !ok {
			return nil, fmt.Sprintf("no such node %s for now, resolves to nothing until it joins the cluster", rule.Node)
		}
		return l, fmt.Sprintf("the node %s hands out no part of the network %s for now, resolves to nothing until it does", rule.Node, rule.Network)
	case KindNode:
		l, ok := view.Node(rule.Node)
		if !ok {
			return nil, fmt.Sprintf("no such node %s for now, resolves to nothing until it joins the cluster", rule.Node)
		}
		return addrPrefixes(l), fmt.Sprintf("the node %s has no known address, resolves to nothing", rule.Node)
	case KindCluster:
		l := make([]netip.Prefix, 0)
		for _, name := range view.Nodes() {
			addrs, _ := view.Node(name)
			l = append(l, addrPrefixes(addrs)...)
		}
		return l, "no cluster node has a known address, resolves to nothing"
	case KindObject:
		p, err := rule.ObjectPath(t.namespace)
		if err != nil {
			return nil, err.Error() + ", resolves to nothing"
		}
		addrs, ok := view.Object(p)
		if !ok {
			return nil, noSuchObject(p)
		}
		if rule.RID != "" {
			if _, ok := addrs[rule.RID]; !ok {
				return nil, noSuchResource(p, rule.RID)
			}
			return addrPrefixes(addrs.All(rule.RID)), fmt.Sprintf("the resource %s of %s publishes no address for now, resolves to nothing until it is started", rule.RID, p)
		}
		return addrPrefixes(addrs.All("")), fmt.Sprintf("the object %s publishes no address for now, resolves to nothing until it is started", p)
	case KindNamespace:
		name := rule.NamespaceName(t.namespace)
		paths := view.NamespaceObjects(name)
		if len(paths) == 0 {
			return nil, noSuchNamespace(name)
		}
		l := make([]netip.Prefix, 0)
		for _, p := range paths {
			addrs, _ := view.Object(p)
			l = append(l, addrPrefixes(addrs.All(""))...)
		}
		return l, fmt.Sprintf("the objects of the namespace %s publish no address for now, resolves to nothing until one is started", name)
	}
	return nil, ""
}

func noSuchObject(p naming.Path) string {
	return fmt.Sprintf("no such object %s for now, resolves to nothing until it exists", p)
}

func noSuchResource(p naming.Path, rid string) string {
	return fmt.Sprintf("no such resource %s in the object %s for now, resolves to nothing until it exists", rid, p)
}

func noSuchNamespace(name string) string {
	return fmt.Sprintf("no object in the namespace %s for now, resolves to nothing until one exists", name)
}

// addrPrefixes returns the addresses as ranges of one address.
func addrPrefixes(l []netip.Addr) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(l))
	for _, addr := range l {
		if !addr.IsValid() {
			continue
		}
		addr = addr.Unmap().WithZone("")
		out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return out
}

// Unknown returns the rules naming a network, a node, an object, a resource
// or a namespace the view does not have, each with the reason it resolves to
// nothing, which names the object or the namespace as resolved for an object
// of the namespace given. objects says the view knows the objects of the
// cluster: where nobody tells them, their rules are not judged.
//
// A rule naming another namespace is not judged either: a validation is run
// for whoever writes a configuration, who is not to learn from it which
// objects another namespace holds. The start of the resource says what such
// a rule resolves to.
//
// It is what a validation warns of, without resolving an address: a
// configuration may be written before what it names exists, and its filter
// is the stricter for it until then.
func Unknown(rules Rules, view View, namespace string, objects bool) Notes {
	notes := make(Notes, 0)
	nodes := view.Nodes()
	note := func(rule Rule, reason string) {
		notes = append(notes, Note{Rule: rule, Reason: reason})
	}
	for _, rule := range rules {
		switch rule.Kind {
		case KindNetwork:
			if _, ok := view.Network(rule.Network); !ok {
				note(rule, fmt.Sprintf("no such network %s for now, resolves to nothing until it exists", rule.Network))
				continue
			}
			if rule.Node != "" && !slices.Contains(nodes, rule.Node) {
				note(rule, fmt.Sprintf("no such node %s for now, resolves to nothing until it joins the cluster", rule.Node))
			}
		case KindNode:
			if !slices.Contains(nodes, rule.Node) {
				note(rule, fmt.Sprintf("no such node %s for now, resolves to nothing until it joins the cluster", rule.Node))
			}
		case KindObject:
			p, err := rule.ObjectPath(namespace)
			if err != nil {
				note(rule, err.Error()+", resolves to nothing")
				continue
			}
			if !objects || rule.Foreign(namespace) {
				continue
			}
			addrs, ok := view.Object(p)
			if !ok {
				note(rule, noSuchObject(p))
				continue
			}
			if _, ok := addrs[rule.RID]; rule.RID != "" && !ok {
				note(rule, noSuchResource(p, rule.RID))
			}
		case KindNamespace:
			if !objects || rule.Foreign(namespace) {
				continue
			}
			if name := rule.NamespaceName(namespace); len(view.NamespaceObjects(name)) == 0 {
				note(rule, noSuchNamespace(name))
			}
		case KindAddress:
			if rule.From == "" {
				continue
			}
			// The key is read, to warn of the lines that are no
			// address: who writes the rule writes the key, or reads
			// it, the datastore being of the namespace of the
			// object, or sharing its keys with it. One that does
			// not is read nothing of, and its rule is refused.
			prefixes, reason, warnings, refused := resolver{view: view, namespace: namespace}.keyRanges(rule)
			for _, warning := range warnings {
				note(rule, warning)
			}
			if len(prefixes) == 0 {
				notes = append(notes, Note{Rule: rule, Reason: reason, Refused: refused})
			}
		}
	}
	return notes
}
