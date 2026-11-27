package netrule

import (
	"encoding/json"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/tombstone"
)

type (
	// Held are the addresses a rule naming an object or a namespace
	// resolves to, each with the object publishing it.
	//
	// It is the memory of a filter kept current: what tells an address a
	// status no longer shows from one no status ever showed, and whose it
	// was.
	Held map[netip.Addr]Holder

	// Memory are the addresses held of each rule naming an object or a
	// namespace, by the text of the rule, its verb included.
	//
	// The rules are an ordered list, where the place of a rule decides
	// of what it matches: an address is remembered with the rule that
	// resolved to it, so one kept is kept at the place of that rule, on
	// its ports, and as its verb says, see Held.Keep. A list naming an
	// object in an allow rule and in a deny rule keeps the addresses of
	// each apart.
	//
	// The text is the key, not the place: a rule moved in the list, or
	// one a rule is written before, keeps its memory, and two rules
	// written the same resolve to the same.
	Memory map[string]Held

	// Holder is the object publishing an address held: its path, and
	// which object of that path it is, as the configuration it was
	// known by when it published the address tells.
	//
	// A path names the object deleted and the one created again under
	// it alike: the id and the version tell them apart, so the record of
	// a delete takes away the addresses of the object it deleted, and
	// not the ones of the object that took its place, see IsDeletedBy.
	Holder struct {
		// Path is the path of the object.
		Path string `json:"path"`

		// ObjectID is the id of the object, its DEFAULT.id, empty when
		// its configuration tells none, or was not known.
		ObjectID string `json:"object_id,omitempty"`

		// Version is the modification time of the configuration of the
		// object, as the node committing it set it, and as every node
		// holding that configuration carries it: the newest of the
		// instances publishing the address, see AddrIdentifier. Zero
		// when no configuration was known.
		Version time.Time `json:"version,omitzero"`
	}

	// Identifier is a view telling which object a path names for now:
	// the daemon is one, which hears of the configuration of every
	// instance.
	Identifier interface {
		// Identity returns the id of the object of the path, and the
		// version of its configuration, the newest the instances
		// hold, each zero when not known.
		Identity(path naming.Path) (id string, version time.Time)
	}

	// AddrIdentifier is a view telling which object publishes an
	// address of a path: the daemon is one, which hears of the status
	// and of the configuration of each instance.
	//
	// A path may name two objects for a time: the one deleted, an
	// instance of which a node still tells, and the one created again
	// or restored under it, whose configuration another node tells
	// already. An address is the one of the object whose instance
	// publishes it, and the record of the delete of that object lifts
	// it, whatever newer configuration the path is known by.
	AddrIdentifier interface {
		// AddrIdentity returns the id of the object publishing the
		// address under the path, and the version of its
		// configuration, the newest of the instances publishing the
		// address, each zero when the configuration of none is known.
		AddrIdentity(path naming.Path, addr netip.Addr) (id string, version time.Time)
	}

	// Witness is a view telling, beyond what the objects publish, what
	// is not known of them: the daemon is one, which hears of the nodes
	// it loses and of every object.
	Witness interface {
		View

		// IsUnsure says nothing tells the status of an instance of the
		// object holding an address for now: its node is not heard of,
		// as while its daemon restarts, or did not say yet. What its
		// resources hold is then not known, which is not known gone,
		// unless a record of the delete of that very object says so.
		IsUnsure(holder Holder) bool

		// Holders returns the objects publishing the address.
		Holders(addr netip.Addr) []naming.Path
	}
)

// UnmarshalJSON reads a holder, and the one an om knowing no holder wrote:
// the path of the object alone, which tells no id and no version.
func (t *Holder) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*t = Holder{Path: s}
		return nil
	}
	type holder Holder
	var h holder
	if err := json.Unmarshal(b, &h); err != nil {
		return err
	}
	*t = Holder(h)
	return nil
}

// Equal says the two are the same object, known by the same configuration.
func (t Holder) Equal(o Holder) bool {
	return t.Path == o.Path && t.ObjectID == o.ObjectID && t.Version.Equal(o.Version)
}

// IsDeletedBy says the record is the one of the delete of the holder: of
// its path, of its id when both tell one, and of a configuration not older
// than the one the holder was known by.
//
// A record of another id, or of an older configuration, is the delete of
// the object the path named before: the holder was created again, or
// restored, since. A holder of no known version is not judged, as a record
// of none: nothing says the record is about it.
//
// The ids are compared first, then the two versions, each the time the node
// committing the configuration gave it, carried with it: no clock of the
// node judging is read.
func (t Holder) IsDeletedBy(rec tombstone.Record) bool {
	if t.ObjectID != "" && rec.ObjectID != "" && t.ObjectID != rec.ObjectID {
		return false
	}
	return rec.Kills(t.Path, t.Version)
}

// identity returns which object the path names over the view, nothing when
// the view does not tell.
func identity(view View, p naming.Path) (string, time.Time) {
	if i, ok := view.(Identifier); ok {
		return i.Identity(p)
	}
	return "", time.Time{}
}

// addrIdentity returns which object publishes the address under the path
// over the view: the one the view tells of the address when it tells the
// objects by address, and else the one it knows the path by.
func addrIdentity(view View, p naming.Path, addr netip.Addr) (string, time.Time) {
	if i, ok := view.(AddrIdentifier); ok {
		return i.AddrIdentity(p, addr)
	}
	return identity(view, p)
}

// Holding returns the addresses each rule naming an object or a namespace
// resolves to over the view, each with the object publishing it, for the
// rules of an object of the namespace given. A rule resolving to no address
// holds nothing, and is not there.
//
// The object is the one whose instances publish the address, see
// AddrIdentifier: an address a node still publishes of an object deleted is
// not given the object created again under its path, which the record of
// the delete would then not lift.
func Holding(rules Rules, view View, namespace string) Memory {
	memory := make(Memory)
	for _, rule := range rules {
		held := make(Held)
		hold := func(p naming.Path, rid string) {
			addrs, ok := view.Object(p)
			if !ok {
				return
			}
			for _, addr := range addrs.All(rid) {
				addr = addr.Unmap().WithZone("")
				if _, ok := held[addr]; ok {
					continue
				}
				holder := Holder{Path: p.String()}
				holder.ObjectID, holder.Version = addrIdentity(view, p, addr)
				held[addr] = holder
			}
		}
		switch rule.Kind {
		case KindObject:
			if p, err := rule.ObjectPath(namespace); err == nil {
				hold(p, rule.RID)
			}
		case KindNamespace:
			for _, p := range view.NamespaceObjects(rule.NamespaceName(namespace)) {
				hold(p, "")
			}
		}
		if len(held) > 0 {
			memory[rule.Text] = held
		}
	}
	return memory
}

// Keep returns the addresses of t its rule no longer resolves to, current
// being what it resolves to now, which the filter keeps all the same, in
// the step of that rule. denied says the rule is a deny one.
//
// A filter fails closed: what is not known is decided on the side that
// lets less through.
//
// An allowed address is dropped when the status of its object says it is
// gone. It is kept while that status is not known, the node of an instance
// being lost or not heard of yet: a daemon restarting, on this node or
// another, stops nothing of what the objects run, and cuts nothing of what
// reaches them.
//
// A denied address is kept as long as nothing says who holds it: an object
// stopped, or whose status is lost, may still answer there. It is dropped
// when its object is deleted.
//
// Either is dropped once another object publishes the address: it is that
// object's then, and the rules naming it, if any, resolve to it.
//
// An address whose holder tells no version, as one an om knowing no holder
// recorded, or one of an object whose configuration was not heard yet, is
// given the object the view knows under the path: nothing says it was
// another, and the newer object is the one a record of a delete lifts less
// of.
func (t Held) Keep(current Held, view Witness, denied bool) Held {
	kept := make(Held)
	for addr, holder := range t {
		if _, ok := current[addr]; ok {
			continue
		}
		p, err := naming.ParsePath(holder.Path)
		if err != nil {
			continue
		}
		if slices.ContainsFunc(view.Holders(addr), func(o naming.Path) bool { return o != p }) {
			continue
		}
		if holder.Version.IsZero() {
			if id, version := identity(view, p); !version.IsZero() {
				holder.ObjectID, holder.Version = id, version
			}
		}
		unsure := view.IsUnsure(holder)
		_, exists := view.Object(p)
		if unsure || (denied && exists) {
			kept[addr] = holder
		}
	}
	return kept
}

// sorted returns the addresses in their order.
func (t Held) sorted() []netip.Addr {
	addrs := make([]netip.Addr, 0, len(t))
	for addr := range t {
		addrs = append(addrs, addr)
	}
	slices.SortFunc(addrs, func(a, b netip.Addr) int { return a.Compare(b) })
	return addrs
}

// with returns the addresses of t and of o together.
func (t Held) with(o Held) Held {
	l := make(Held, len(t)+len(o))
	for addr, s := range t {
		l[addr] = s
	}
	for addr, s := range o {
		l[addr] = s
	}
	return l
}

// remember gives the addresses whose holder tells no version the holder
// they had before, when it is of the same path and tells one: an object is
// known by the configuration it published the address with, and the view
// may no longer tell it while the address is still published.
func (t Held) remember(before Held) {
	for addr, holder := range t {
		if !holder.Version.IsZero() {
			continue
		}
		if was, ok := before[addr]; ok && was.Path == holder.Path && !was.Version.IsZero() {
			t[addr] = was
		}
	}
}

// Equal says the two hold the same addresses of the same objects, known by
// the same configurations.
func (t Held) Equal(o Held) bool {
	if len(t) != len(o) {
		return false
	}
	for addr, holder := range t {
		if v, ok := o[addr]; !ok || !v.Equal(holder) {
			return false
		}
	}
	return true
}

// verbOf returns the verb of the text of a rule.
func verbOf(text string) Verb {
	verb, _, _ := strings.Cut(text, " ")
	return Verb(verb)
}

// prefixes returns the addresses held of the rule, as ranges of one
// address, in their order. An implicit rule holds none.
func (t Memory) prefixes(rule Rule) []netip.Prefix {
	if rule.Implicit {
		return nil
	}
	return addrPrefixes(t[rule.Text].sorted())
}

// Count returns the number of addresses held of the rules of the verb
// given, an address held of two of them counted once.
func (t Memory) Count(verb Verb) int {
	seen := make(map[netip.Addr]bool)
	for text, held := range t {
		if verbOf(text) != verb {
			continue
		}
		for addr := range held {
			seen[addr] = true
		}
	}
	return len(seen)
}

// IsZero says nothing is held.
func (t Memory) IsZero() bool {
	for _, held := range t {
		if len(held) > 0 {
			return false
		}
	}
	return true
}

// Equal says the two hold the same addresses of the same rules, a rule
// holding nothing being one not there.
func (t Memory) Equal(o Memory) bool {
	for text, held := range t {
		if !held.Equal(o[text]) {
			return false
		}
	}
	for text, held := range o {
		if _, ok := t[text]; !ok && len(held) > 0 {
			return false
		}
	}
	return true
}

// remember gives the addresses whose holder tells no version the holder
// they had before, rule by rule, see Held.remember.
func (t Memory) remember(before Memory) {
	for text, held := range t {
		held.remember(before[text])
	}
}

// with returns the addresses of t and of o together, rule by rule.
func (t Memory) with(o Memory) Memory {
	l := make(Memory, len(t)+len(o))
	for text, held := range t {
		l[text] = held.with(o[text])
	}
	for text, held := range o {
		if _, ok := l[text]; !ok {
			l[text] = held.with(nil)
		}
	}
	return l
}

// adopt gives the addresses of held to the rules of the verb given naming
// their object, by itself or by its namespace, for the rules of an object
// of the namespace given: each rule naming the object, as nothing says
// which resolved to the address. An address whose object no rule of the
// verb names is forgotten.
//
// It is how the memory of a rule taken out of the list is handed to the
// ones still naming its objects, and how the addresses a record written
// before the rules were ordered held, of all its allow rules together and
// of all its deny rules, are given to the rules of each verb.
func (t Memory) adopt(rules Rules, namespace string, verb Verb, held Held) {
	for addr, holder := range held {
		p, err := naming.ParsePath(holder.Path)
		if err != nil {
			continue
		}
		for _, rule := range rules {
			if rule.Verb != verb || !rule.Names(p, namespace) {
				continue
			}
			if t[rule.Text] == nil {
				t[rule.Text] = make(Held)
			}
			if _, ok := t[rule.Text][addr]; !ok {
				t[rule.Text][addr] = holder
			}
		}
	}
}

// carried returns the memory of the rules given, new, out of the one of
// the rules they replace: the addresses of a rule still written are its
// own, and the ones of a rule no longer written go to the rules of its verb
// naming their object, see adopt.
func (t Memory) carried(rules Rules, namespace string) Memory {
	next := make(Memory)
	written := make(map[string]bool)
	for _, rule := range rules {
		written[rule.Text] = true
	}
	for text, held := range t {
		if written[text] && len(held) > 0 {
			next[text] = held.with(nil)
		}
	}
	for text, held := range t {
		if !written[text] {
			next.adopt(rules, namespace, verbOf(text), held)
		}
	}
	return next
}
