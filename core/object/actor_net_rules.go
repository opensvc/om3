package object

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

// netRuleOptions is the keyword holding network rules, of each section.
var netRuleOptions = []string{netrule.Keyword}

// netRuleView returns the view the rules are looked up in, and is replaced
// by the tests.
var netRuleView = netrule.LocalView

// netRuleObjects returns the objects the rules name as the daemon knows
// them, and is replaced by the tests. Which objects they are is not asked:
// a validation looks for the ones the cluster does not have.
var netRuleObjects = func(objects []naming.Path, namespaces []string) (netrule.Objects, error) {
	data, _, err := netrule.LookupObjects(objects, namespaces)
	return data, err
}

// netRuleAlerts warns of the rules of the net_rules keyword of
// the resource sections naming a network, a node, an object, a resource or a
// namespace the cluster does not have, on every node of the object: such a
// rule resolves to no address until what it names exists. The warning names
// the object or the namespace the rule resolves to for an object of the
// namespace given, which ./ and . stand for.
//
// A rule reading its addresses from a key of a datastore is warned of when
// the datastore or the key does not exist, and of the lines of the key that
// are no address, each named. A datastore of another namespace is read as
// one of the namespace of the object when its share keyword shares its keys
// with it, and is an error when it does not: the rule is refused, whoever
// writes it, the sharing being the consent of who administers the
// datastore, which the root grant is not.
//
// It warns too of what the order and the verbs of the rules say that their
// writer most likely does not mean, judged on the rules as written, with
// nothing of the cluster asked: a rule that can never match, a rule before
// it matching all it names, see netrule.Shadowed, as a rule appended after
// a last allow any, and a list with no allow rule, which lets through the
// loopback addresses and the nameservers alone.
//
// The rules the grammar refuses are errors of the validation of their
// keyword, and are not judged here. Where nothing is known of the cluster,
// as in a binary driving it through its api, nothing is warned of, and
// where the daemon does not answer, nothing of the objects. Nothing is said
// of the objects of another namespace, see netrule.Unknown.
func netRuleAlerts(cfg *xconfig.T, namespace string) xconfig.Alerts {
	alerts := make(xconfig.Alerts, 0)
	if cfg == nil || cfg.Referrer == nil {
		return alerts
	}
	var view netrule.View
	reported := make(map[string]bool)
	for _, section := range cfg.SectionStrings() {
		rid, err := resourceid.Parse(section)
		if err != nil || rid.DriverGroup() == driver.GroupUnknown {
			continue
		}
		sectionType := cfg.GetString(key.New(section, "type"))
		for _, option := range netRuleOptions {
			k := key.New(section, option)
			kw := cfg.Referrer.KeywordLookup(k, sectionType)
			if kw == nil {
				continue
			}
			for _, nodename := range keySourceNodes(cfg) {
				v, err := cfg.EvalKeywordAs(k, kw, nodename)
				if err != nil {
					continue
				}
				value, _ := v.(string)
				if value == "" {
					continue
				}
				rules, err := netrule.Parse(option, value)
				if err != nil {
					continue
				}
				if kw.Validate != nil && kw.Validate(value) != nil {
					// A rule the resource makes no filter of, as
					// one on a port where the filter knows none:
					// the error of its keyword says it.
					continue
				}
				warn := func(comment string) {
					id := k.String() + " " + comment
					if reported[id] {
						return
					}
					reported[id] = true
					alerts = append(alerts, cfg.NewAlertNetRuleOrder(k, sectionDriverID(cfg, section), comment))
				}
				for _, note := range netrule.Shadowed(rules, namespace) {
					warn(note.String())
				}
				if !rules.HasAllow() {
					warn("no allow rule: nothing is let through but the loopback addresses and the nameservers, which the implicit rules allow, and an allow any written last denies what the rules name alone")
				}
				if view == nil {
					// Read once, and only for a configuration with
					// rules.
					if view, err = netRuleView(); err != nil {
						return alerts
					}
				}
				// The objects are asked for the rules naming some,
				// which most configurations have none of.
				ruleView, hasObjects := view, false
				if objects, namespaces := netrule.References(namespace, rules); len(objects) > 0 || len(namespaces) > 0 {
					if data, err := netRuleObjects(objects, namespaces); err == nil {
						ruleView, hasObjects = netrule.WithObjects(view, data), true
					}
				}
				for _, note := range netrule.Unknown(rules, ruleView, namespace, hasObjects) {
					id := k.String() + " " + note.String()
					if reported[id] {
						continue
					}
					reported[id] = true
					if note.Refused {
						// Not a thing to wait for: refused, as a key
						// installed from a datastore not sharing it
						// is.
						alerts = append(alerts, cfg.NewAlertCandidates(k, sectionDriverID(cfg, section), note.String()))
						continue
					}
					alerts = append(alerts, cfg.NewAlertNetRule(k, sectionDriverID(cfg, section), note.String()))
				}
			}
		}
	}
	return alerts
}
