// Package keyoprbac is the rbac policy of the object configuration keywords:
// which of them a user holding no root grant may set, and with which values.
//
// The policy is a table rather than a function body because two callers need
// it. The api enforces it on every keyword of a configuration a user sends,
// and the keyword documentation explains it, so a keyword whose rule lives
// here is documented and enforced from one text: neither can drift from the
// other, and a driver keyword added later needs no edit in a second place to
// be documented.
package keyoprbac

import (
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

type (
	// Rule is what the policy says about one keyword.
	//
	// A rule with neither Values nor Denies needs the grant whatever the
	// value. The two are the way a rule can be about the value instead: a
	// keyword can be harmless with one value and a way to run arbitrary code
	// on the node with another.
	Rule struct {
		// Grant is the grant a user must hold to set the keyword.
		Grant rbac.Grant

		// Reason ends the denial the api returns, after the keyword operation
		// it refuses, and is also the sentence the keyword documentation
		// shows. It is written to read after "denied: <section>.<option>=…: ".
		Reason string

		// Values, when not empty, is the values a user holding no grant may
		// set anyway. Any other value needs the grant.
		Values []string

		// Denies, when not nil, reports whether a value needs the grant. It
		// is for the rules a value list cannot express: a shape of the value
		// rather than the whole of it, or the setup the rest of the section
		// describes.
		//
		// A rule with a Denies writes its whole sentence in Reason, values
		// included, because the policy cannot derive one from a function.
		Denies func(value string, section Section) bool
	}

	// Group is the policy of one driver group.
	Group struct {
		// Rules is what the policy says about the keywords it names. A zero
		// Rule is a keyword any user may set, which is how a group whose
		// Default asks for a grant opens a few of its keywords.
		Rules map[string]Rule

		// Default is the rule of every keyword the group does not name. A nil
		// Default leaves them to any user, which suits a group whose keywords
		// describe what the object runs. A group whose keywords describe what
		// the object takes from the node names a Default instead, so a
		// keyword added to one of its drivers later is refused until the
		// policy has weighed it.
		Default *Rule
	}

	// Section answers whether an option is set in the section the keyword
	// being checked belongs to, whatever node it is scoped to.
	//
	// It is a callback because the policy is asked about one keyword at a
	// time, while some setups are only safe or unsafe as a whole: an address
	// a user may not choose is not a keyword of its own, it is a name set
	// where a network to draw one from should have been.
	Section func(option string) bool
)

const (
	// reasonRoot is the reason of every keyword a user may not set at all.
	reasonRoot = "requires the root grant"

	// reasonGroup is the reason of a driver group the policy says nothing
	// about.
	reasonGroup = "this driver group requires the root grant"

	// reasonTrigger is the reason of the keywords that run a command of the
	// user's choosing on the node, whichever driver they are set on.
	reasonTrigger = "triggers require the root grant"
)

// containerTypes is the container and task types a user holding no root grant
// may ask for. They run an image, under the container engine, which confines
// what the object can reach. The other types, the ones that run on the node
// itself, do not.
var containerTypes = []string{"oci", "docker", "podman"}

// rootRule is the group default of the driver groups whose keywords describe
// what the object takes from the node.
var rootRule = Rule{Grant: rbac.GrantRoot, Reason: reasonRoot}

// rules is the policy, by driver group then by keyword.
//
// A driver group absent from here needs the root grant whatever the keyword: a
// group whose consequences nobody has weighed is refused rather than allowed,
// so a driver group added to om later is refused until this table says
// otherwise. A group present here says the same thing about its own keywords
// through its Default.
var rules = map[string]Group{
	"task": {Rules: map[string]Rule{
		"type": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
			Values: containerTypes,
		},
		"run_args": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
		},
		// om writes the resolver of the container from these, so they decide
		// what the names in it resolve to.
		"dns": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
		},
		"dns_search": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
		},
	}},
	"container": {Rules: map[string]Rule{
		"type": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
			Values: containerTypes,
		},
		"run_args": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
		},
		"dns": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
		},
		"dns_search": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
		},
		"volume_mounts": {
			Grant:  rbac.GrantRoot,
			Reason: "host path mounts in container require the root grant",
			Denies: valueOnly(hasHostPathMount),
		},
	}},
	"volume": {Rules: map[string]Rule{
		"install": {
			Grant:  rbac.GrantRoot,
			Reason: "a server-local source uri requires the root grant",
			Denies: valueOnly(datarecv.TextHasLocalSource),
		},
	}},
	"fs": {Rules: map[string]Rule{
		"type": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
			// A flag is a file in the object var directory. Every other fs
			// type mounts something, which is the node's to decide.
			Values: []string{"flag"},
		},
	}},

	// An ip resource takes an address, and a link to carry it, from the node.
	// Which address, and which link, are the node administrator's to decide,
	// so the group is root by default and opens only the keywords that say
	// what the object does with an address it was given.
	"ip": {
		Default: &rootRule,
		Rules: map[string]Rule{
			"type": {
				Grant: rbac.GrantRoot,
				Reason: "requires the root grant, except for a cni address, " +
					"and for a netns address om draws from a cluster network",
				Denies: deniesIPType,
			},

			// The networks are the cluster's, and om hands out the addresses
			// of the one named here.
			"network": {},

			// These say what the object does with its address, inside the
			// object: which of its containers holds the namespace, what the
			// device is called in it, which ports it serves, and how the
			// record is named.
			"netns":           {},
			"nsdev":           {},
			"expose":          {},
			"dns_name_suffix": {},

			// How long the object waits for its own record to appear.
			"wait_dns": {},
		},
	},

	"DEFAULT": {Rules: map[string]Rule{
		"priority": {
			// A priority decides which objects a node sheds first, so it
			// weighs objects of a namespace against objects of another.
			Grant:  rbac.GrantPrioritizer,
			Reason: "requires the prioritizer grant",
		},
		"monitor_action": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
			// These three act on the object. The others act on the node, up
			// to rebooting it.
			Values: []string{"switch", "freezestop", "none"},
		},
		"pre_monitor_action": {
			Grant:  rbac.GrantRoot,
			Reason: reasonRoot,
		},
	}},

	// These two sections hold values the object reads, and nothing om acts
	// on, so no keyword of theirs needs a grant.
	"env":    {},
	"labels": {},
}

// commonKeywords is the keywords every section carries, whatever its driver.
//
// They describe what the object does with the resource across its own
// lifecycle, not what the resource takes from the node, so a group that asks
// for a grant by default does not ask for one here. Leaving them out would
// revoke, on the day a group gains a Default, keywords every group allowed
// until then.
//
// The triggers are common keywords too, and are refused above: they run a
// command of the user's choosing, which is the one thing in this set that is
// not about the object alone.
var commonKeywords = map[string]bool{
	"comment":          true,
	"disable":          true,
	"encap":            true,
	"monitor":          true,
	"no_preempt_abort": true,
	"optional":         true,

	// The process group limits cap what the object may take of the node, so
	// setting one takes nothing from anybody.
	"pg_blkio_weight":      true,
	"pg_cpu_quota":         true,
	"pg_cpu_shares":        true,
	"pg_cpus":              true,
	"pg_mem_limit":         true,
	"pg_mem_oom_control":   true,
	"pg_mem_swappiness":    true,
	"pg_mems":              true,
	"pg_vmem_limit":        true,
	"prkey":                true,
	"provision":            true,
	"provision_requires":   true,
	"restart":              true,
	"restart_delay":        true,
	"run_requires":         true,
	"scsireserv":           true,
	"shared":               true,
	"standby":              true,
	"start_requires":       true,
	"stat_timeout":         true,
	"stop_requires":        true,
	"subset":               true,
	"sync_requires":        true,
	"tags":                 true,
	"unprovision":          true,
	"unprovision_requires": true,
}

// triggers is the keywords that run a command of the user's choosing, in the
// agent's context, on every driver.
var triggers = map[string]bool{
	"blocking_post_provision":   true,
	"blocking_post_run":         true,
	"blocking_post_start":       true,
	"blocking_post_stop":        true,
	"blocking_post_unprovision": true,

	"blocking_pre_provision":   true,
	"blocking_pre_run":         true,
	"blocking_pre_start":       true,
	"blocking_pre_stop":        true,
	"blocking_pre_unprovision": true,

	"post_provision":   true,
	"post_run":         true,
	"post_start":       true,
	"post_stop":        true,
	"post_unprovision": true,

	"pre_provision":   true,
	"pre_run":         true,
	"pre_start":       true,
	"pre_stop":        true,
	"pre_unprovision": true,
}

// valueOnly adapts a check that only reads the value to the rule signature.
func valueOnly(f func(value string) bool) func(string, Section) bool {
	return func(value string, _ Section) bool {
		return f(value)
	}
}

// deniesIPType reports whether an ip resource of this type decides an address,
// or a link to the node, that a user holding no root grant may not decide.
//
// A cni address is drawn from a cluster network, by the plugin om configured
// for it, and the ip.cni driver has no keyword naming an address.
//
// A netns address is drawn from a cluster network too, but only when the
// resource names one to draw from and no address of its own: naming the
// address is exactly what a user may not do here. A network that turns out to
// hand out no address is refused by the driver rather than silently letting
// the user pick, so the network needs no checking here, which is as well: the
// networks are the node's, and this runs on whichever node received the
// configuration.
func deniesIPType(value string, section Section) bool {
	switch value {
	case "cni":
		return false
	case "netns":
		return !section("network") || section("name")
	default:
		return true
	}
}

// hasHostPathMount reports whether a volume_mounts value mounts a path of the
// node rather than a volume of the object, either by naming it outright or by
// climbing out of the volume with a relative path.
func hasHostPathMount(value string) bool {
	for _, e := range strings.Fields(value) {
		if strings.HasPrefix(e, "_") || strings.Contains(e, "/../") || strings.HasPrefix(e, "../") || strings.HasSuffix(e, "../") {
			return true
		}
	}
	return false
}

// normalize returns the driver group and the keyword a section and an option
// name.
//
// A section carries the resource index, and an option the node or environment
// it is scoped to. Neither changes what the keyword is, so the policy is
// written without them and they are stripped here.
func normalize(section, option string) (string, string) {
	group, _, _ := strings.Cut(section, "#")
	option, _, _ = strings.Cut(option, "@")
	return group, option
}

// Lookup returns the rule of a keyword, and whether the policy has one.
//
// A keyword of a driver group the policy says nothing about has the group
// rule, which needs the root grant: the answer is never "no rule" because the
// table forgot a group. Neither is it "no rule" for a keyword of a group whose
// Default asks for a grant.
func Lookup(section, option string) (Rule, bool) {
	group, option := normalize(section, option)
	if triggers[option] {
		return Rule{Grant: rbac.GrantRoot, Reason: reasonTrigger}, true
	}
	groupPolicy, ok := rules[group]
	if !ok {
		return Rule{Grant: rbac.GrantRoot, Reason: reasonGroup}, true
	}
	rule, ok := groupPolicy.Rules[option]
	if ok {
		// A zero rule is a keyword the group opens, whatever its Default.
		return rule, rule.Grant != ""
	}
	if commonKeywords[option] {
		return Rule{}, false
	}
	if groupPolicy.Default != nil {
		return *groupPolicy.Default, true
	}
	return Rule{}, false
}

// Denied returns the reason these grants are not enough to set this keyword to
// this value, or nil when they are.
//
// The caller has already let a user holding the root grant through, so a rule
// naming that grant is a refusal here. The reason is returned bare, for the
// caller to prefix with the operation it refuses.
// The set callback answers what else the section holds, for the rules that are
// about the setup rather than the keyword. A rule needing it and given none
// refuses: a check that could not run is not a check that passed.
func Denied(grants rbac.Grants, section, option, value string, set Section) error {
	rule, ok := Lookup(section, option)
	if !ok {
		return nil
	}
	switch {
	case rule.Denies != nil:
		if set != nil && !rule.Denies(value, set) {
			return nil
		}
	case len(rule.Values) > 0:
		for _, allowed := range rule.Values {
			if value == allowed {
				return nil
			}
		}
	}
	if grants.HasGrant(rule.Grant) {
		return nil
	}
	return fmt.Errorf("%s", rule.Reason)
}

// Doc returns the sentence the documentation of a keyword shows about the
// grant needed to set it through the api, or an empty string when any user may
// set it.
func Doc(section, option string) string {
	rule, ok := Lookup(section, option)
	if !ok {
		return ""
	}
	s := rule.Reason
	if s == "" {
		return ""
	}
	s = strings.ToUpper(s[:1]) + s[1:]
	if rule.Denies == nil && len(rule.Values) > 0 {
		s += ", except for the values " + strings.Join(rule.Values, ", ")
	}
	return s + "."
}
