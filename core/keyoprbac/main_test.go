package keyoprbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/rbac"
)

// noGrant is the user the policy is written for: one the api did not already
// let through as root.
var noGrant = rbac.Grants{}

// none is a section holding nothing but the keyword being checked.
var none = section()

// section builds the accessor the policy reads the rest of a section through.
func section(options ...string) Section {
	set := make(map[string]bool, len(options))
	for _, option := range options {
		set[option] = true
	}
	return func(option string) bool { return set[option] }
}

func TestDeniedByDriverGroup(t *testing.T) {
	// A driver group the policy says nothing about is refused whatever the
	// keyword, including a keyword nobody has written yet. This is the
	// property the table exists to keep: a driver added to om later is
	// refused until a rule allows it, rather than allowed until someone
	// remembers to refuse it.
	for _, section := range []string{"disk#1", "sync#1", "app#1", "share#1", "expose#1", "certificate#1"} {
		require.Errorf(t, Denied(noGrant, section, "type", "whatever", none), "section %s", section)
		require.Errorf(t, Denied(noGrant, section, "a_keyword_of_a_driver_added_later", "x", none), "section %s", section)
	}
	assert.EqualError(t, Denied(noGrant, "disk#1", "name", "x", none), "this driver group requires the root grant")
}

func TestAllowedInAGroupWithNoRuleForTheKeyword(t *testing.T) {
	// A group the policy admits is writable except for the keywords it names.
	for _, section := range []string{"container#1", "task#1", "volume#1", "fs#1", "DEFAULT", "env", "labels"} {
		require.NoErrorf(t, Denied(noGrant, section, "a_keyword_with_no_rule", "x", none), "section %s", section)
	}
}

func TestDeniedKeywords(t *testing.T) {
	for _, section := range []string{"container#1", "task#1"} {
		for _, option := range []string{"run_args", "dns", "dns_search"} {
			err := Denied(noGrant, section, option, "x", none)
			require.Errorf(t, err, "%s.%s", section, option)
			assert.EqualError(t, err, "requires the root grant")
		}
	}
	assert.EqualError(t, Denied(noGrant, "DEFAULT", "pre_monitor_action", "/bin/true", none), "requires the root grant")
}

func TestDeniedByValue(t *testing.T) {
	cases := []struct {
		section string
		option  string
		value   string
		denied  bool
	}{
		{"container#1", "type", "docker", false},
		{"container#1", "type", "podman", false},
		{"container#1", "type", "oci", false},
		{"container#1", "type", "kvm", true},
		{"container#1", "type", "lxc", true},
		{"task#1", "type", "docker", false},
		{"task#1", "type", "host", true},
		{"fs#1", "type", "flag", false},
		{"fs#1", "type", "xfs", true},
		{"fs#1", "type", "zfs", true},
		{"DEFAULT", "monitor_action", "switch", false},
		{"DEFAULT", "monitor_action", "freezestop", false},
		{"DEFAULT", "monitor_action", "none", false},
		{"DEFAULT", "monitor_action", "reboot", true},
		{"DEFAULT", "monitor_action", "crash", true},

		{"container#1", "volume_mounts", "/vol/data:/data:rw", false},
		{"container#1", "volume_mounts", "_/etc:/etc:ro", true},
		{"container#1", "volume_mounts", "/vol/a:/a ../../etc:/etc", true},
		{"container#1", "volume_mounts", "/vol/a/../../etc:/etc", true},

		{"volume#1", "install", "/etc/nginx.conf from https://example.com/c source https://example.com/c", false},
		{"volume#1", "install", "/etc/nginx.conf source /etc/shadow", true},
	}
	for _, tc := range cases {
		err := Denied(noGrant, tc.section, tc.option, tc.value, none)
		if tc.denied {
			require.Errorf(t, err, "%s.%s=%s must be denied", tc.section, tc.option, tc.value)
		} else {
			require.NoErrorf(t, err, "%s.%s=%s must be allowed", tc.section, tc.option, tc.value)
		}
	}
}

func TestDeniedTriggersOnEveryGroup(t *testing.T) {
	// A trigger runs a command of the user's choosing, so it is refused
	// wherever it is set, including in the groups the policy otherwise
	// admits.
	for _, section := range []string{"container#1", "task#1", "fs#1", "ip#1", "volume#1", "DEFAULT", "disk#1"} {
		for _, option := range []string{"pre_start", "post_stop", "blocking_pre_run", "pre_unprovision"} {
			err := Denied(noGrant, section, option, "/bin/rm -rf /", none)
			require.Errorf(t, err, "%s.%s", section, option)
			assert.EqualErrorf(t, err, "triggers require the root grant", "%s.%s", section, option)
		}
	}
}

func TestDeniedIgnoresTheIndexAndTheScope(t *testing.T) {
	// The resource index and the scoping suffix do not change what a keyword
	// is, so neither is a way around a rule.
	assert.Error(t, Denied(noGrant, "container#12", "run_args", "x", none))
	assert.Error(t, Denied(noGrant, "container#a-name", "run_args", "x", none))
	assert.Error(t, Denied(noGrant, "container#1", "run_args@node1", "x", none))
	assert.Error(t, Denied(noGrant, "container#1", "dns@fr-par", "x", none))
	assert.Error(t, Denied(noGrant, "container#1", "type@node1", "kvm", none))
	assert.NoError(t, Denied(noGrant, "container#1", "type@node1", "docker", none))
}

func TestDeniedHonorsTheGrantTheRuleNames(t *testing.T) {
	// A priority weighs the objects of one namespace against those of
	// another, so it takes a grant of its own rather than root.
	err := Denied(noGrant, "DEFAULT", "priority", "10", none)
	require.Error(t, err)
	assert.EqualError(t, err, "requires the prioritizer grant")

	assert.NoError(t, Denied(rbac.Grants{rbac.GrantPrioritizer}, "DEFAULT", "priority", "10", none))
	assert.Error(t, Denied(rbac.Grants{rbac.GrantPrioritizer}, "container#1", "run_args", "x", none),
		"the prioritizer grant must not open the keywords asking for root")

	// The api lets a root user through before asking, but the rule says the
	// same thing on its own.
	assert.NoError(t, Denied(rbac.Grants{rbac.GrantRoot}, "container#1", "run_args", "x", none))
}

func TestDocSaysWhatTheRuleEnforces(t *testing.T) {
	assert.Equal(t, "Requires the root grant.", Doc("container", "run_args"))
	assert.Equal(t, "Requires the root grant.", Doc("container#1", "dns"))
	assert.Equal(t, "Requires the root grant, except for the values oci, docker, podman.", Doc("container", "type"))
	assert.Equal(t, "Requires the root grant, except for the values flag.", Doc("fs", "type"))
	assert.Equal(t, "Requires the root grant, except for a cni address, and for a netns address om draws from a cluster network.", Doc("ip", "type"))
	assert.Equal(t, "Requires the root grant.", Doc("ip", "name"))
	assert.Equal(t, "", Doc("ip", "network"))
	assert.Equal(t, "Host path mounts in container require the root grant.", Doc("container", "volume_mounts"))
	assert.Equal(t, "A server-local source uri requires the root grant.", Doc("volume", "install"))
	assert.Equal(t, "Requires the prioritizer grant.", Doc("DEFAULT", "priority"))
	assert.Equal(t, "Triggers require the root grant.", Doc("container", "pre_start"))
	assert.Equal(t, "This driver group requires the root grant.", Doc("disk", "name"))

	// A keyword any user may set says nothing, rather than saying it needs
	// nothing on every keyword of every driver.
	assert.Equal(t, "", Doc("container", "image"))
	assert.Equal(t, "", Doc("env", "anything"))
}

func TestIPDrawnFromAClusterNetwork(t *testing.T) {
	// A cni address is the network's to choose, and the driver has no keyword
	// naming one.
	assert.NoError(t, Denied(noGrant, "ip#1", "type", "cni", section("network", "netns")))

	// A netns address is om's to choose when the resource names a network to
	// draw from and no address of its own.
	assert.NoError(t, Denied(noGrant, "ip#1", "type", "netns", section("network", "netns")))
	assert.NoError(t, Denied(noGrant, "ip#1", "type", "netns", section("network")))

	// Naming the address is what a user holding no root grant may not do,
	// whether instead of a network or alongside one.
	assert.Error(t, Denied(noGrant, "ip#1", "type", "netns", section("name")))
	assert.Error(t, Denied(noGrant, "ip#1", "type", "netns", section("network", "name")))
	assert.Error(t, Denied(noGrant, "ip#1", "type", "netns", none),
		"a netns resource drawing from no network has an address of its own")

	// A scope is not a way around it: the section is read as written, so a
	// name set for a peer node counts here.
	assert.Error(t, Denied(noGrant, "ip#1", "type", "netns", section("network", "name")))

	// Every other ip type addresses a node interface.
	for _, typ := range []string{"host", "route", "sgcp_dnsalias", "amazon", ""} {
		assert.Errorf(t, Denied(noGrant, "ip#1", "type", typ, section("network")), "type %s", typ)
	}

	// The keywords naming an address, or the link that carries it, are the
	// node administrator's whatever the type.
	for _, option := range []string{"name", "dev", "gateway", "netmask", "macaddr", "mode", "alias", "vlan_tag", "vlan_mode", "del_net_route", "check_carrier"} {
		assert.Errorf(t, Denied(noGrant, "ip#1", option, "x", section("network")), "option %s", option)
	}

	// An ip keyword added to a driver later is refused until the policy has
	// weighed it, as a whole group is.
	assert.Error(t, Denied(noGrant, "ip#1", "an_ip_keyword_added_later", "x", section("network")))

	// What the object does with the address it was given stays open, which is
	// every keyword an ip.cni resource needs.
	for _, option := range []string{"network", "netns", "nsdev", "expose", "dns_name_suffix"} {
		assert.NoErrorf(t, Denied(noGrant, "ip#1", option, "x", none), "option %s", option)
	}
}

func TestDeniedWithoutASectionRefuses(t *testing.T) {
	// A rule about the setup cannot be answered without the section, and a
	// check that could not run is not a check that passed.
	assert.Error(t, Denied(noGrant, "ip#1", "type", "netns", nil))
	assert.Error(t, Denied(noGrant, "ip#1", "type", "cni", nil))
	assert.Error(t, Denied(noGrant, "container#1", "volume_mounts", "/vol/a:/a", nil))

	// A rule that reads only the keyword still answers.
	assert.NoError(t, Denied(noGrant, "container#1", "image", "nginx", nil))
	assert.Error(t, Denied(noGrant, "container#1", "run_args", "x", nil))
}

// TestEveryRuleIsDocumented keeps the table honest: a rule with no reason
// enforces something the documentation cannot explain.
func TestEveryRuleIsDocumented(t *testing.T) {
	check := func(name string, rule Rule) {
		assert.NotEmptyf(t, rule.Reason, "%s has no reason", name)
		assert.NotEmptyf(t, rule.Grant, "%s names no grant", name)
		assert.Falsef(t, rule.Denies != nil && len(rule.Values) > 0,
			"%s has both Values and Denies, and Denies would win", name)
	}
	for group, groupPolicy := range rules {
		if groupPolicy.Default != nil {
			check(group+" default", *groupPolicy.Default)
		}
		for option, rule := range groupPolicy.Rules {
			name := group + "." + option
			if rule.Grant == "" {
				// A zero rule is a keyword the group opens on purpose.
				assert.Emptyf(t, rule.Reason, "%s is open but gives a reason", name)
				assert.Emptyf(t, Doc(group, option), "%s is open but documents a grant", name)
				continue
			}
			check(name, rule)
			assert.NotEmptyf(t, Doc(group, option), "%s documents nothing", name)
		}
	}
}

// TestCommonKeywordsSurviveAGroupDefault pins that giving a group a Default
// refuses the keywords of its drivers, not the keywords every section carries.
// A user who may create an ip resource may still say it is optional, tag it,
// or put it in a subset.
func TestCommonKeywordsSurviveAGroupDefault(t *testing.T) {
	for _, option := range []string{"optional", "disable", "monitor", "standby", "shared", "tags", "subset", "restart", "restart_delay", "provision", "start_requires"} {
		assert.NoErrorf(t, Denied(noGrant, "ip#1", option, "true", section("network")), "option %s", option)
		assert.Emptyf(t, Doc("ip", option), "option %s", option)
	}

	// A trigger is a common keyword too, and stays refused everywhere.
	assert.Error(t, Denied(noGrant, "ip#1", "pre_start", "/bin/true", section("network")))
}
