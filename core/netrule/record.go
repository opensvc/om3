package netrule

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
)

type (
	// Record is the filter applied to a resource, with what it was
	// resolved from: written by the start of the resource, for its status
	// to compare the filter in force with, and for the daemon to resolve
	// the rules again when what they name changes.
	//
	// It holds all the daemon needs to keep the filter current as what
	// the rules name changes, without reading the configuration of the
	// object: the daemon reads it when it changes, and writes the rules
	// it finds changed here, see WithRules.
	Record struct {
		// NetRules are the rules, in their order, as the start read
		// them, or the daemon since, in a configuration changed under
		// the running resource: a rule per entry, not the text of the
		// keyword, whose layout is none of the record, see Texts.
		NetRules Texts `json:"net_rules"`

		// Namespace is the namespace of the object of the resource,
		// which ./ and . stand for in the rules.
		Namespace string `json:"namespace"`

		// PG is the id of the process group whose slice takes the
		// filter, with the cgroup backend.
		PG string `json:"pg"`

		// Backend is what applies the filter, the cgroup one when not
		// named, and Devs the network interfaces it is applied on,
		// with the netdev one.
		Backend Backend  `json:"backend,omitempty"`
		Devs    []string `json:"devs,omitempty"`

		// DNS are the nameservers what the resource runs resolves
		// through, nil for the ones of the resolver of the node.
		DNS []netip.Addr `json:"dns"`

		// Filter is what the rules resolved to, the last time they
		// were resolved.
		Filter Filter `json:"filter"`

		// Public is the filter without what the rules naming another
		// namespace add to it, and Foreign counts what they add: what
		// the status of the resource shows.
		Public  Filter  `json:"public"`
		Foreign Foreign `json:"foreign"`

		// Held are the addresses the rules naming objects and
		// namespaces resolved to, by rule, each with the object that
		// published it, and Kept the ones they no longer resolve to
		// that the filter keeps, in the step of their rule, see
		// Memory and Held.Keep.
		Held Memory `json:"held"`
		Kept Memory `json:"kept"`
	}
)

// UnmarshalJSON reads a record, and the one an om knowing two keywords
// wrote, net_allow and net_deny, whose deny rules won over the allow ones,
// and which allowed what no rule named when it had deny rules only.
//
// Such a record is read as the list saying the same: its deny rules, then
// its allow rules, each given its verb, and a last allow any when it has no
// allow rule. The addresses it held of all its allow rules together, and of
// all its deny rules, are given to the rules of each verb naming their
// object, see Memory.adopt.
func (t *Record) UnmarshalJSON(b []byte) error {
	type record Record
	var v struct {
		record
		NetRules *Texts          `json:"net_rules"`
		NetAllow json.RawMessage `json:"net_allow"`
		NetDeny  json.RawMessage `json:"net_deny"`
		Held     json.RawMessage `json:"held"`
		Kept     json.RawMessage `json:"kept"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*t = Record(v.record)
	if v.NetRules != nil || (v.NetAllow == nil && v.NetDeny == nil) {
		if v.NetRules != nil {
			t.NetRules = *v.NetRules
		}
		for raw, memory := range map[*json.RawMessage]*Memory{&v.Held: &t.Held, &v.Kept: &t.Kept} {
			if *raw == nil {
				continue
			}
			if err := json.Unmarshal(*raw, memory); err != nil {
				return err
			}
		}
		return nil
	}
	allow, err := formerTexts(v.NetAllow)
	if err != nil {
		return err
	}
	deny, err := formerTexts(v.NetDeny)
	if err != nil {
		return err
	}
	t.NetRules = nil
	for _, text := range deny {
		t.NetRules = append(t.NetRules, string(VerbDeny)+" "+text)
	}
	for _, text := range allow {
		t.NetRules = append(t.NetRules, string(VerbAllow)+" "+text)
	}
	if len(allow) == 0 && len(deny) > 0 {
		t.NetRules = append(t.NetRules, string(VerbAllow)+" "+string(KindAny))
	}
	rules, err := parse(Keyword, unitRule, t.NetRules)
	if err != nil {
		// Rules this om does not read: nothing is known of what
		// their addresses were held of, and Rules says the error.
		return nil
	}
	for raw, memory := range map[*json.RawMessage]*Memory{&v.Held: &t.Held, &v.Kept: &t.Kept} {
		var sides struct {
			Allow Held `json:"allow"`
			Deny  Held `json:"deny"`
		}
		if *raw != nil {
			if err := json.Unmarshal(*raw, &sides); err != nil {
				return err
			}
		}
		*memory = make(Memory)
		memory.adopt(rules, t.Namespace, VerbAllow, sides.Allow)
		memory.adopt(rules, t.Namespace, VerbDeny, sides.Deny)
	}
	return nil
}

// formerTexts reads the rules of a keyword of a record written before the
// rules had a verb, each a noun and what qualifies it: a list of them, or
// the text of the keyword, where a noun started a rule where the one before
// it was complete. Words that are no rule are returned as they are, for
// the rules of the record to be refused.
func formerTexts(raw json.RawMessage) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	var lines []string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		lines = strings.Split(s, "\n")
	} else if err := json.Unmarshal(raw, &lines); err != nil {
		return nil, err
	}
	l := make([]string, 0, len(lines))
	for _, line := range lines {
		words := strings.Fields(line)
		for len(words) > 0 {
			_, rest, err := parseRule(append([]string{string(VerbAllow)}, words...))
			if err != nil {
				l = append(l, strings.Join(words, " "))
				break
			}
			l = append(l, strings.Join(words[:len(words)-len(rest)], " "))
			words = rest
		}
	}
	return l, nil
}

// Rules returns the rules of the record, each entry read as a line of the
// keyword is, and the error of a rule the grammar refuses or no filter is
// made of.
//
// The error names the keyword and the entry of its list, counted from 1,
// as net_rules rule 2: a record tells no line of the configuration the
// rules were read from, whose own errors name one, see ParseKeyword.
func (t Record) Rules() (Rules, error) {
	rules, err := parse(Keyword, unitRule, t.NetRules)
	if err != nil {
		return nil, err
	}
	if err := rules.supported(Keyword, unitRule, t.Backend); err != nil {
		return nil, err
	}
	return rules, nil
}

// HasRules says the rules of the record are the ones of the value of the
// keyword given, as a configuration has it: the same rules in the same
// order, however the value lays them out, on one line or several, indented
// or not. The same rules in another order are other rules: the first one
// matching decides.
//
// A value the grammar refuses is the rules of no record: it is a change,
// for who applies it to say it is refused.
func (t Record) HasRules(value string) bool {
	return t.NetRules.Equal(TextsOf(value))
}

// WithRules returns the record with the rules of the value of the keyword
// given in place of its own, as a configuration changed under the running
// resource has it, to be resolved by Refresh.
//
// The addresses held and kept are the memory of the rules. The ones of a
// rule still written stay its own, wherever the rule is in the list now.
// The ones of a rule no longer written go to the rules of its verb still
// naming their object, by itself or by its namespace, and are kept or
// dropped there as before the change: a rule written another way, as one
// narrowed to a resource, is not a reason to let through what was denied.
// The ones of an object no rule of the verb names any more are forgotten,
// where Refresh would keep them as addresses their rule lost, a denied one
// for as long as its object exists.
//
// Rules the grammar refuses, or no filter is made of, are an error naming
// the line of the value, and the record is returned as it was.
func (t Record) WithRules(value string) (Record, error) {
	rules, err := ParseKeyword(value, t.Backend)
	if err != nil {
		return t, err
	}
	next := t
	next.NetRules = rules.Texts()
	next.Held = t.Held.carried(rules, t.Namespace)
	next.Kept = t.Kept.carried(rules, t.Namespace)
	return next, nil
}

// Refresh returns the record of what the rules resolve to over the view
// now, and the rules resolving to nothing.
//
// Over a view telling what is not known of the objects, the addresses a
// rule resolved to the last time and no longer does are kept or dropped as
// Held.Keep says, the ones of an allow rule and the ones of a deny rule
// each their way, and the ones kept are addresses of the step of their
// rule: an address denied by a rule written before the one allowing its
// network is still denied first. Over any other view, as the one of a
// start, nothing is kept: a start has no memory.
//
// Each object is asked of the view once: the filter and the addresses held
// are of one state of the cluster.
func (t Record) Refresh(view View) (Record, Notes, error) {
	rules, err := t.Rules()
	if err != nil {
		return t, nil, err
	}
	witness, _ := view.(Witness)
	view = freeze(view)
	if t.DNS != nil {
		view = WithDNS(view, t.DNS)
	}
	next := t
	before := t.Held.with(t.Kept)
	next.Held = Holding(rules, view, t.Namespace)
	// The configuration of an instance may go before its status, as the
	// data of a node lost is dropped: the object an address was known
	// of is not forgotten for the time in between.
	next.Held.remember(before)
	next.Kept = make(Memory)
	if witness != nil {
		for _, rule := range rules {
			was, ok := before[rule.Text]
			if !ok {
				continue
			}
			if kept := was.Keep(next.Held[rule.Text], witness, rule.Verb == VerbDeny); len(kept) > 0 {
				next.Kept[rule.Text] = kept
			}
		}
	}
	r := resolver{view: view, namespace: t.Namespace, kept: next.Kept}
	var notes Notes
	next.Filter, notes = r.resolve(rules)
	next.Public, next.Foreign = r.public(rules)
	return next, notes, nil
}

// LoadRecord reads the record of a resource from its file.
func LoadRecord(p string) (Record, error) {
	var t Record
	b, err := os.ReadFile(p)
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return t, fmt.Errorf("%s: %w", filepath.Base(p), err)
	}
	return t, nil
}

// Save writes the record to the file of its resource, whole or not at all:
// the status of the resource and the daemon read it while it is written.
func (t Record) Save(p string) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), "."+filepath.Base(p)+".")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(f.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(f.Name(), p)
}
