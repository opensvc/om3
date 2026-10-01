// Package configkeywords answers the keywords of an object or node
// configuration, raw or evaluated, as a list of keyword items.
//
// The daemon api answers them, and om answers them the same way when it
// reads the configuration itself, with --local or with the daemon down: the
// two answers are the same list, rendered by the same code.
package configkeywords

import (
	"slices"
	"strings"

	"github.com/danwakefield/fnmatch"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// Options says which keywords to answer, and how.
	Options struct {
		// Keywords are the keywords to answer, and none is the whole
		// configuration.
		//
		// The section of one is a resource selector element when it is a
		// driver group or a pattern, and its option is a pattern when it
		// holds one: see IsPattern.
		Keywords []string

		// Evaluate answers the evaluated value of each keyword beside its
		// raw value.
		Evaluate bool

		// Impersonate is the node the values are evaluated as, the local
		// node when empty.
		Impersonate string

		// Redact hides the value of the secret keywords, for a reader not
		// allowed to see them.
		Redact bool
	}

	// selected is a key to answer.
	selected struct {
		key key.T

		// filtered says the key was not named but matched: by a pattern,
		// or by the whole configuration being asked. A key named and
		// failing to evaluate is an error, as the user asked for it. A key
		// matched is one of many, and failing on it would hide the
		// others, so its item says what failed.
		filtered bool
	}
)

// IsPattern says whether a keyword of a selection stands for the keys it
// matches rather than for one key.
//
// The section is a resource selector element, the same as a --rid one: a
// driver group, as "container", or a pattern, as "cont*", matches the
// sections of the resources it names. The option is a pattern when it holds
// one, as "*" or "stop_*", and matches the options the keywords of the
// section have, whether the configuration sets them or not.
//
// A pattern filters: a section without the keyword is skipped, and matching
// nothing is no error. A keyword naming one key selects it, as it always
// did.
func IsPattern(s string) bool {
	k := key.Parse(s)
	return isSectionPattern(k.Section) || isGlob(k.Option)
}

// HasPattern says whether a keyword of the selection is a pattern, which
// answers a list whatever the number of keys it matched.
func HasPattern(l []string) bool {
	return slices.ContainsFunc(l, IsPattern)
}

func isGlob(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func isSectionPattern(section string) bool {
	if isGlob(section) {
		return true
	}
	rid, err := resourceid.Parse(section)
	return err == nil && rid.DriverGroup().IsValid() && rid.Index() == ""
}

// evaluatedAs is the node the values are evaluated as.
func (o Options) evaluatedAs() string {
	if o.Impersonate != "" {
		return o.Impersonate
	}
	return hostname.Hostname()
}

// selection returns the keys to answer: the ones the keywords name or match,
// in the order they were asked, or every key of all when none is asked.
//
// conf is the configuration the sections and the keywords are read from, and
// kind the kind of object it is of, naming.KindInvalid for the node.
func (o Options) selection(all key.L, conf *xconfig.T, kind naming.Kind) []selected {
	l := make([]selected, 0)
	if len(o.Keywords) == 0 {
		for _, k := range all {
			l = append(l, selected{key: k, filtered: true})
		}
		return l
	}
	seen := make(map[key.T]bool)
	add := func(k key.T, filtered bool) {
		if seen[k] {
			return
		}
		seen[k] = true
		l = append(l, selected{key: k, filtered: filtered})
	}
	for _, s := range o.Keywords {
		k := key.Parse(s)
		sectionPattern, optionPattern := isSectionPattern(k.Section), isGlob(k.Option)
		if !sectionPattern && !optionPattern {
			add(k, false)
			continue
		}
		sections := []string{k.Section}
		if sectionPattern {
			sections = matchingSections(conf, k.Section)
		}
		for _, section := range sections {
			for _, option := range matchingOptions(conf, kind, section, k.Option, optionPattern) {
				add(key.New(section, option), true)
			}
		}
	}
	return l
}

// matchingSections returns the resource sections of the configuration the
// selector element matches, in the order of the configuration.
//
// Only a section with the shape of a resource id is matched: DEFAULT, env,
// data, labels and the subsets are named, not matched.
func matchingSections(conf *xconfig.T, pattern string) []string {
	l := make([]string, 0)
	for _, section := range conf.SectionStrings() {
		rid, err := resourceid.Parse(section)
		if err != nil {
			continue
		}
		if rid.Match(pattern) {
			l = append(l, section)
		}
	}
	return l
}

// matchingOptions returns the options of the keywords of the section the
// option matches, sorted.
//
// A keyword the section does not have is not matched, whether it was named
// or a pattern matched it: a section pattern spans resources of several
// drivers, and the ones without the keyword have nothing to answer.
//
// The env, data and labels sections hold any option, so a pattern matches
// the ones the configuration sets there.
func matchingOptions(conf *xconfig.T, kind naming.Kind, section, option string, optionPattern bool) []string {
	has := func(option string) bool {
		k := key.New(section, option)
		return conf.Referrer.KeywordLookup(k, conf.SectionType(k)) != nil
	}
	if !optionPattern {
		if has(option) {
			return []string{option}
		}
		return nil
	}
	var candidates []string
	switch section {
	case "env", "data", "labels":
		candidates = conf.Keys(section)
	default:
		candidates = append(object.KeywordOptions(kind, section), conf.Keys(section)...)
	}
	slices.Sort(candidates)
	candidates = slices.Compact(candidates)
	l := make([]string, 0)
	for _, candidate := range candidates {
		if fnmatch.Match(option, candidate, 0) && has(candidate) {
			l = append(l, candidate)
		}
	}
	return l
}

// Object answers the keywords of the configuration of an object.
//
// A keyword selection is a user input, so a key it names and can not
// evaluate is an error, which is xconfig.ErrNoKeyword for a keyword that does
// not exist. The keys a pattern matches, or the whole configuration, are not:
// failing on the first one would hide all the others, so each item says what
// failed.
func Object(p naming.Path, o Options) (api.KeywordItems, error) {
	oc, err := object.NewCore(p)
	if err != nil {
		return nil, err
	}
	conf := oc.Config()
	evaluatedAs := o.evaluatedAs()
	items := make(api.KeywordItems, 0)
	for _, sel := range o.selection(conf.KeyList(), conf, p.Kind) {
		k := sel.key
		item := api.KeywordItem{
			Object:  p.String(),
			Keyword: k.String(),
		}
		if s, err := conf.GetStrict(k); err == nil {
			item.Value = s
		}

		// A secret is not shown, raw or evaluated, to a reader not allowed
		// to see it. The keyword is listed all the same, so the reader
		// knows it is set.
		if o.Redact && object.IsSecretKey(p.Kind, k, conf.GetString(key.New(k.Section, "type"))) {
			item.Value = object.RedactedValue
			if o.Evaluate {
				var v any = object.RedactedValue
				text := object.RedactedValue
				item.Evaluated = &v
				item.EvaluatedText = &text
				item.EvaluatedAs = evaluatedAs
			}
			items = append(items, item)
			continue
		}

		if o.Evaluate {
			v, err := oc.EvalAs(k, evaluatedAs)
			switch {
			case err != nil && sel.filtered:
				s := err.Error()
				item.Error = &s
				item.EvaluatedAs = evaluatedAs
			case err != nil:
				return nil, err
			default:
				text := conf.EvaluatedText(k, v)
				item.Evaluated = &v
				item.EvaluatedText = &text
				item.EvaluatedAs = evaluatedAs
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// Node answers the keywords of the configuration of the local node, named
// nodename in the items.
//
// The keys of the whole configuration are the ones the node configuration
// sets. The value of a keyword is the one that applies to the node, from its
// configuration or the cluster's, and a keyword set nowhere still evaluates,
// to its default. A pattern matches the sections of both, as a heartbeat is
// configured in the cluster's.
func Node(nodename string, o Options) (api.KeywordItems, error) {
	n, err := object.NewNode()
	if err != nil {
		return nil, err
	}
	merged := n.MergedConfig()
	evaluatedAs := o.evaluatedAs()
	items := make(api.KeywordItems, 0)
	for _, sel := range o.selection(n.Config().KeyList(), merged, naming.KindInvalid) {
		k := sel.key
		item := api.KeywordItem{
			Node:    nodename,
			Keyword: k.String(),
		}
		if s, err := merged.GetStrict(k); err == nil {
			item.Value = s
		}
		if o.Evaluate {
			v, err := merged.EvalAs(k, evaluatedAs)
			switch {
			case err != nil && sel.filtered:
				s := err.Error()
				item.Error = &s
				item.EvaluatedAs = evaluatedAs
			case err != nil:
				return nil, err
			default:
				text := merged.EvaluatedText(k, v)
				item.Evaluated = &v
				item.EvaluatedText = &text
				item.EvaluatedAs = evaluatedAs
			}
		}
		items = append(items, item)
	}
	return items, nil
}
