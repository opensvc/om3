// Package netrule is the grammar of the net_rules keyword, and what turns
// its rules into the addresses a filter is made of.
//
// A rule says what a resource may reach, or may not, in the words of the
// cluster: allow or deny, then a node, a network, the cluster itself. The
// rules are an ordered list, and the first one matching a packet decides of
// it. The package parses the rules and resolves them to address sets over a
// view of the cluster, in their order, so the backend applying the filter
// is given addresses and nothing to interpret: the steps as they are for a
// backend judging a packet rule after rule, and flattened to an allowed and
// a denied list for one that knows no order, see Filter.Flat.
//
// It knows no backend, and nothing of where the view comes from: the driver
// feeds it the configuration of the node and what the daemon says of the
// objects, and the daemon what it knows of the cluster, so a rule naming an
// object follows its addresses.
package netrule

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
)

type (
	// Action is what becomes of a packet a rule is the first to match,
	// allow or deny: the word the rule starts with, its verb, and the
	// action of a step of the filter, as the firewalls name it.
	Action string

	// Kind is the noun of a rule, after its verb.
	Kind string

	// Rule is a line of the net_rules keyword.
	Rule struct {
		// Action says the rule lets through, or drops, what it names.
		Action Action

		// Kind is the noun of the rule.
		Kind Kind

		// Implicit says no keyword wrote the rule: it is one of the
		// rules ending every list, see Rules.WithImplicit.
		Implicit bool

		// Line is the line of the keyword value the rule is written on,
		// from 1, the empty lines not counted. For a rule read from a
		// record, it is the entry of the list holding it.
		Line int

		// Text is the rule as written, its verb included, its words
		// separated by a space.
		Text string

		// Prefix is the range of an address rule naming one.
		Prefix netip.Prefix

		// From and Key are the datastore and the key an address rule
		// reads its ranges from.
		From string
		Key  string

		// Network is the cluster network of a network rule.
		Network string

		// Node is the cluster node of a node rule, and the one a network
		// rule narrows the network to.
		Node string

		// Object is the path of an object rule, as written, and RID the
		// resource it narrows the object to.
		Object string
		RID    string

		// Namespace is the namespace of a namespace rule, as written.
		Namespace string

		// HasPort says the rule is qualified by ports, PortMin to PortMax
		// of the protocol Proto.
		HasPort bool
		PortMin uint16
		PortMax uint16
		Proto   string
	}

	// Rules are the rules of a keyword, in the order written, which is
	// the order they are matched in.
	Rules []Rule

	// Texts are the rules of a keyword as a record holds them: a rule
	// per entry, each as its Text says it, its words separated by a
	// space, in their order.
	//
	// A keyword value is a text, a rule or several per line, laid out
	// as the writer of the configuration liked: that is the format of
	// the configuration file, and no reason for what is made of it to
	// be a text too. The rules are what a record remembers, so the
	// same rules laid out another way are the same record.
	Texts []string
)

const (
	// Keyword is the option of the keyword holding the rules.
	Keyword = "net_rules"

	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
)

const (
	KindLocalhost Kind = "localhost"
	KindDNS       Kind = "dns"
	KindAny       Kind = "any"
	KindCluster   Kind = "cluster"
	KindAddress   Kind = "address"
	KindNetwork   Kind = "network"
	KindNode      Kind = "node"
	KindObject    Kind = "object"
	KindNamespace Kind = "namespace"
)

// kinds are the nouns, in the order an error lists them.
var kinds = []Kind{
	KindLocalhost,
	KindDNS,
	KindAny,
	KindCluster,
	KindAddress,
	KindNetwork,
	KindNode,
	KindObject,
	KindNamespace,
}

func isKind(s string) bool {
	for _, k := range kinds {
		if string(k) == s {
			return true
		}
	}
	return false
}

func kindsString() string {
	l := make([]string, len(kinds))
	for i, k := range kinds {
		l[i] = string(k)
	}
	return strings.Join(l, ", ")
}

// isVerb says the word is one a rule starts with.
func isVerb(s string) bool {
	return s == string(ActionAllow) || s == string(ActionDeny)
}

// Parse returns the rules of the value of the keyword named.
//
// A rule is a verb, allow or deny, a noun, and what qualifies the noun, and
// a line holds one rule. A value written on one line, as a command sets it,
// holds several: a verb starts a rule where the rule before it is complete,
// so "allow network lan allow node n1" is two rules. A word a rule may take
// is taken by it: "allow network lan node n1" is one rule, the part of the
// network lan the node n1 hands out. A rule has one verb and one noun: the
// verb is written again for each.
//
// An error names the keyword, the line, and the words refused.
func Parse(keyword, value string) (Rules, error) {
	return parse(keyword, unitLine, strings.Split(value, "\n"))
}

// unitLine and unitRule are what an error counts to place the words it
// refuses: the lines of a keyword value, which the writer of a
// configuration reads, and the entries of the list of a record, which hold
// a rule each.
const (
	unitLine = "line"
	unitRule = "rule"
)

// parse returns the rules of the lines given, the empty ones not counted,
// an error naming the keyword, the unit the lines are counted in, and the
// words refused.
func parse(keyword, unit string, lines []string) (Rules, error) {
	rules := make(Rules, 0)
	n := 0
	for _, line := range lines {
		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}
		n++
		for len(words) > 0 {
			rule, rest, err := parseRule(words)
			if err != nil {
				return nil, fmt.Errorf("%s %s %d: %w", keyword, unit, n, err)
			}
			rule.Line = n
			rule.Text = strings.Join(words[:len(words)-len(rest)], " ")
			rules = append(rules, rule)
			words = rest
		}
	}
	return rules, nil
}

// supported returns the error of the first rule the backend given makes no
// filter of, placed as parse places the words it refuses.
func (t Rules) supported(keyword, unit string, backend Backend) error {
	for _, rule := range t {
		if err := rule.Unsupported(backend); err != nil {
			return fmt.Errorf("%s %s %d: %w", keyword, unit, rule.Line, err)
		}
	}
	return nil
}

// ParseKeyword returns the rules of the value of the keyword, as a
// configuration has it, for a resource whose filter the backend given
// applies, and the error of a rule the grammar refuses or that backend
// makes no filter of, naming the keyword and the line of its value.
func ParseKeyword(value string, backend Backend) (Rules, error) {
	rules, err := Parse(Keyword, value)
	if err != nil {
		return nil, err
	}
	if err = rules.supported(Keyword, unitLine, backend); err != nil {
		return nil, err
	}
	return rules, nil
}

// implicit are the rules ending every list, which no keyword writes.
var implicit = Rules{
	{Action: ActionAllow, Kind: KindLocalhost, Implicit: true, Text: "allow localhost"},
	{Action: ActionAllow, Kind: KindDNS, Implicit: true, Text: "allow dns"},
	{Action: ActionDeny, Kind: KindAny, Implicit: true, Text: "deny any"},
}

// WithImplicit returns the rules followed by the ones ending every list:
// allow localhost, allow dns, then deny any.
//
// What no rule matches is denied: a list says what is let through. The
// loopback addresses and the nameservers are allowed just before, without a
// rule saying so: a filter written for the peers of an application is not
// meant to cut it from its own node, nor from the names it resolves. They
// come last, so a written rule matching them wins: a deny of the cluster
// nodes takes the nameservers they are away, unless an allow dns is written
// before it.
//
// No rule is no list: nothing is added to it, and nothing is filtered.
func (t Rules) WithImplicit() Rules {
	if len(t) == 0 {
		return nil
	}
	return append(slices.Clone(t), implicit...)
}

// Texts returns the rules as a record holds them.
func (t Rules) Texts() Texts {
	if len(t) == 0 {
		return nil
	}
	l := make(Texts, len(t))
	for i, rule := range t {
		l[i] = rule.Text
	}
	return l
}

// TextsOf returns the rules of the value of a keyword as a record holds
// them: a rule per entry, however the value lays them out.
//
// A value the grammar refuses has no rules to tell: its lines are returned,
// their words separated by a space, the empty ones left out. They are no
// rules of any record, so the value still reads as a change of the rules,
// and reading them again refuses the same words.
func TextsOf(value string) Texts {
	if rules, err := Parse("", value); err == nil {
		return rules.Texts()
	}
	var l Texts
	for _, line := range strings.Split(value, "\n") {
		if words := strings.Fields(line); len(words) > 0 {
			l = append(l, strings.Join(words, " "))
		}
	}
	return l
}

// Equal says the two are the same rules, in the same order.
func (t Texts) Equal(o Texts) bool {
	return slices.Equal(t, o)
}

// MarshalJSON writes the rules as a list, an empty one for no rule.
func (t Texts) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(t))
}

// UnmarshalJSON reads the list of the rules, an empty one as no rule.
func (t *Texts) UnmarshalJSON(b []byte) error {
	var l []string
	if err := json.Unmarshal(b, &l); err != nil {
		return err
	}
	*t = nil
	if len(l) > 0 {
		*t = Texts(l)
	}
	return nil
}

// upToVerb returns the words up to the next verb, the first excluded from the
// search: what an error quotes of a rule it refuses.
func upToVerb(words []string) string {
	for i := 1; i < len(words); i++ {
		if isVerb(words[i]) {
			return strings.Join(words[:i], " ")
		}
	}
	return strings.Join(words, " ")
}

// parseRule reads a rule at the head of words, and returns the words left.
func parseRule(words []string) (Rule, []string, error) {
	var rule Rule
	refuse := func(format string, a ...any) (Rule, []string, error) {
		return rule, nil, fmt.Errorf("%q: %s", upToVerb(words), fmt.Sprintf(format, a...))
	}
	if !isVerb(words[0]) {
		if isKind(words[0]) || words[0] == "port" {
			return refuse("expected allow or deny to start a rule, as allow %s: a rule has one verb and one noun", upToVerb(words))
		}
		return refuse("expected allow or deny to start a rule, as allow node n1")
	}
	rule.Action = Action(words[0])
	if len(words) == 1 {
		return refuse("expected what to %s, one of %s", rule.Action, kindsString())
	}
	noun, rest := words[1], words[2:]
	// arg takes the word a noun or a qualifier needs after it.
	arg := func() (string, bool) {
		if len(rest) == 0 {
			return "", false
		}
		s := rest[0]
		rest = rest[1:]
		return s, true
	}
	// qualifier takes the word given when it is next.
	qualifier := func(s string) bool {
		if len(rest) == 0 || rest[0] != s {
			return false
		}
		rest = rest[1:]
		return true
	}
	if noun == "port" {
		return refuse("a port qualifies a rule, as allow any port 5432 proto tcp")
	}
	if !isKind(noun) {
		return refuse("unknown rule, expected one of %s", kindsString())
	}
	rule.Kind = Kind(noun)
	switch rule.Kind {
	case KindLocalhost, KindDNS, KindAny, KindCluster:
	case KindAddress:
		if qualifier("from") {
			from, ok := arg()
			if !ok {
				return refuse("expected a datastore after from, as allow address from ./cfg/acl key allowed")
			}
			if !qualifier("key") {
				return refuse("expected key after the datastore, as allow address from ./cfg/acl key allowed")
			}
			k, ok := arg()
			if !ok {
				return refuse("expected a key name after key, as allow address from ./cfg/acl key allowed")
			}
			rule.From, rule.Key = from, k
			if !strings.ContainsAny(from, "{}") {
				// A reference left to expand is judged once it
				// is.
				if p, err := naming.ParsePathRel(from, "x"); err != nil || (p.Kind != naming.KindCfg && p.Kind != naming.KindSec) {
					return refuse("%s is no datastore, a cfg or a sec, as ./cfg/acl", from)
				}
			}
			break
		}
		s, ok := arg()
		if !ok {
			return refuse("expected an address or a range, as allow address 10.0.0.0/8")
		}
		prefix, err := parsePrefix(s)
		if err != nil {
			return refuse("%s is no address and no range, as 10.0.0.1, 10.0.0.0/8 or fd00::/8", s)
		}
		rule.Prefix = prefix
	case KindNetwork:
		s, ok := arg()
		if !ok {
			return refuse("expected a network name, as allow network backend")
		}
		rule.Network = s
		if qualifier("node") {
			if s, ok = arg(); !ok {
				return refuse("expected a node name after node, as allow network backend node n1")
			}
			rule.Node = s
		}
	case KindNode:
		s, ok := arg()
		if !ok {
			return refuse("expected a node name, as allow node n1")
		}
		rule.Node = s
	case KindObject:
		s, ok := arg()
		if !ok {
			return refuse("expected an object path, as allow object ./svc/db")
		}
		rule.Object = s
		if qualifier("rid") {
			if s, ok = arg(); !ok {
				return refuse("expected a resource id after rid, as allow object ./svc/db rid ip#1")
			}
			rule.RID = s
		}
	case KindNamespace:
		s, ok := arg()
		if !ok {
			return refuse("expected a namespace name, or . for the one of the object")
		}
		rule.Namespace = s
	}
	if qualifier("port") {
		s, ok := arg()
		if !ok {
			return refuse("expected a port or a range of ports after port, as port 5432 or port 8000-8099")
		}
		lo, hi, err := parsePorts(s)
		if err != nil {
			return refuse("port %s: %s", s, err)
		}
		if !qualifier("proto") {
			return refuse("port %s: expected proto tcp or proto udp after the port", s)
		}
		proto, ok := arg()
		if !ok || (proto != "tcp" && proto != "udp") {
			return refuse("port %s: expected proto tcp or proto udp after the port", s)
		}
		rule.HasPort, rule.PortMin, rule.PortMax, rule.Proto = true, lo, hi, proto
	}
	return rule, rest, nil
}

// parsePrefix reads an address or a range, an address being the range of
// itself. The bits of a range past its length are dropped, so 10.1.2.3/8 is
// 10.0.0.0/8, as the tools taking a range read it.
func parsePrefix(s string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(s); err == nil {
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	if addr.Zone() != "" {
		return netip.Prefix{}, fmt.Errorf("an address with a zone")
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

// parsePorts reads a port, or a range of ports written N-M.
func parsePorts(s string) (uint16, uint16, error) {
	port := func(s string) (uint16, error) {
		n, err := strconv.ParseUint(s, 10, 16)
		if err != nil || n == 0 {
			return 0, fmt.Errorf("a port is a number from 1 to 65535")
		}
		return uint16(n), nil
	}
	first, last, isRange := strings.Cut(s, "-")
	lo, err := port(first)
	if err != nil {
		return 0, 0, err
	}
	if !isRange {
		return lo, lo, nil
	}
	hi, err := port(last)
	if err != nil {
		return 0, 0, err
	}
	if hi < lo {
		return 0, 0, fmt.Errorf("the last port of a range is not under the first")
	}
	return lo, hi, nil
}

// Backend is what applies a filter to what a resource runs, which says the
// rules a filter is made of there.
type Backend string

const (
	// BackendCgroup is the filter systemd attaches to the cgroup of the
	// units of a slice, the one of the app, task, oci and lxc container
	// resources: it knows addresses, and no port. It is the backend of a
	// record naming none.
	BackendCgroup Backend = ""

	// BackendNetdev is the nftables filter of the network interfaces of
	// a guest, the one of container.kvm, which knows the ports too.
	BackendNetdev Backend = "netdev"
)

// Ports says the backend filters on the ports of a rule.
func (t Backend) Ports() bool {
	return t == BackendNetdev
}

// Unsupported returns why a rule the grammar reads is not one the backend
// given makes a filter of, nil when it is.
//
// The grammar is the same for every resource, so a configuration is refused
// for what it asks of the resource it is written on, not for how it is
// written.
func (t Rule) Unsupported(backend Backend) error {
	if t.HasPort && !backend.Ports() {
		return fmt.Errorf("%q: port %s: the cgroup filter of the app, task, oci and lxc container resources knows no port, only the nftables filter of container.kvm does", t.Text, t.ports())
	}
	return nil
}

// ObjectPath returns the object an object rule names, for a rule of an
// object of the namespace given: a path starting with ./ is one of that
// namespace, as a datastore is named, and any other is the path as written.
func (t Rule) ObjectPath(namespace string) (naming.Path, error) {
	p, err := naming.ParsePathRel(t.Object, namespace)
	if err != nil {
		return naming.Path{}, fmt.Errorf("%s is no object path, as ./svc/db or prod/svc/db", t.Object)
	}
	return p, nil
}

// ErrNotShared is the error of a view asked a key of a datastore of another
// namespace than the one of the object, which its share keyword does not
// share its keys with.
var ErrNotShared = errors.New("the datastore does not share its keys with the namespace of the object")

// Datastore returns the datastore an address rule reads its ranges from,
// for a rule of an object of the namespace given: a path starting with ./
// is one of that namespace, as an object rule names an object.
//
// The datastore may be one of another namespace. Whether its keys are read
// is not said here but by the view asked for them, which knows what the
// datastore shares, see View.Key: a filter reading the keys of a namespace
// that did not consent to it would tell them, address by address.
func (t Rule) Datastore(namespace string) (naming.Path, error) {
	p, err := naming.ParsePathRel(t.From, namespace)
	if err != nil || (p.Kind != naming.KindCfg && p.Kind != naming.KindSec) {
		return naming.Path{}, fmt.Errorf("%s is no datastore, a cfg or a sec, as ./cfg/acl", t.From)
	}
	return p, nil
}

// Secret says the rule reads its ranges from a key of a sec: what a reader
// of the resource is not shown, the keys of a sec being told to who may
// decode them only.
func (t Rule) Secret() bool {
	if t.Kind != KindAddress || t.From == "" {
		return false
	}
	p, err := naming.ParsePathRel(t.From, "x")
	return err == nil && p.Kind == naming.KindSec
}

// Hidden says the addresses of the rule are not shown to who reads the
// status of the resource, and counted instead: the ones of the objects of
// another namespace, and the ones read from a sec.
//
// The ones read from a cfg another namespace shares with the one of the
// object are shown, as the ones of a cfg of its own: the share keyword of
// the cfg is its administrator saying the namespace of the object may read
// its keys, which are no secret there, and a reader of the resource is a
// reader of that namespace. The addresses of the objects of another
// namespace are hidden for want of such a consent.
func (t Rule) Hidden(namespace string) bool {
	return t.Secret() || t.Foreign(namespace)
}

// Datastores returns the datastores the rules read their ranges from, for
// the rules of an object of the namespace given, each once.
func Datastores(namespace string, lists ...Rules) []naming.Path {
	l := make([]naming.Path, 0)
	for _, rules := range lists {
		for _, rule := range rules {
			if rule.Kind != KindAddress || rule.From == "" {
				continue
			}
			if p, err := rule.Datastore(namespace); err == nil && !slices.Contains(l, p) {
				l = append(l, p)
			}
		}
	}
	return l
}

// NamespaceName returns the namespace a namespace rule names, for a rule of
// an object of the namespace given, which a dot stands for.
func (t Rule) NamespaceName(namespace string) string {
	if t.Namespace == "." {
		return namespace
	}
	return t.Namespace
}

// Foreign says the rule names an object of another namespace than the one
// given, or another namespace: what its administrator has no say on, so the
// rule is root's to write, and its addresses are not shown to who reads the
// status of the resource.
//
// It is judged on what the rule resolves to, not on how it is written: a
// path spelling the namespace of the object is the same reference as its ./
// form. A path that is none resolves to nothing, and is nobody's.
func (t Rule) Foreign(namespace string) bool {
	switch {
	case t.Kind == KindObject:
		p, err := t.ObjectPath(namespace)
		return err == nil && p.Namespace != namespace
	case t.Kind == KindNamespace:
		return t.NamespaceName(namespace) != namespace
	}
	return false
}

// Reference returns what a rule naming an object or a namespace resolves
// to, for a rule of an object of the namespace given, in the words of a
// rule: "object prod/svc/db rid ip#1", "namespace prod". It is empty for any
// other rule, and for a path that is none.
func (t Rule) Reference(namespace string) string {
	switch t.Kind {
	case KindObject:
		p, err := t.ObjectPath(namespace)
		if err != nil {
			return ""
		}
		if t.RID != "" {
			return "object " + p.String() + " rid " + t.RID
		}
		return "object " + p.String()
	case KindNamespace:
		return "namespace " + t.NamespaceName(namespace)
	}
	return ""
}

// References returns the objects and the namespaces the rules name, for the
// rules of an object of the namespace given, each once.
func References(namespace string, lists ...Rules) (objects []naming.Path, namespaces []string) {
	seenObjects := make(map[naming.Path]bool)
	seenNamespaces := make(map[string]bool)
	for _, rules := range lists {
		for _, rule := range rules {
			switch rule.Kind {
			case KindObject:
				p, err := rule.ObjectPath(namespace)
				if err != nil || seenObjects[p] {
					continue
				}
				seenObjects[p] = true
				objects = append(objects, p)
			case KindNamespace:
				name := rule.NamespaceName(namespace)
				if name == "" || seenNamespaces[name] {
					continue
				}
				seenNamespaces[name] = true
				namespaces = append(namespaces, name)
			}
		}
	}
	return objects, namespaces
}

func (t Rule) ports() string {
	if t.PortMin == t.PortMax {
		return strconv.Itoa(int(t.PortMin))
	}
	return fmt.Sprintf("%d-%d", t.PortMin, t.PortMax)
}

// Validator returns the function saying whether a value is one the keyword
// named takes, on a resource whose filter the backend given applies: rules
// the grammar reads, and that backend makes a filter of.
func Validator(keyword string, backend Backend) func(string) error {
	return func(value string) error {
		rules, err := Parse(keyword, value)
		if err != nil {
			return err
		}
		return rules.supported(keyword, unitLine, backend)
	}
}

// Names says the rule names the object of the path given, by itself or by
// its namespace, for a rule of an object of the namespace given: the rule
// resolves to the addresses the object publishes, or to the ones of a
// resource of it.
func (t Rule) Names(p naming.Path, namespace string) bool {
	switch t.Kind {
	case KindObject:
		o, err := t.ObjectPath(namespace)
		return err == nil && o == p
	case KindNamespace:
		return t.NamespaceName(namespace) == p.Namespace
	}
	return false
}
