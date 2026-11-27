package netrule

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReadsARulePerLine(t *testing.T) {
	rules, err := Parse(Keyword, `
		allow localhost
		allow dns

		deny network backend3
		allow network backend3 node dev2n2
		allow object ./svc/db
		deny object ./svc/db rid ip#1
		allow object prod/svc/db
		allow namespace .
		deny namespace prod
		allow node dev2n2
		deny cluster
		allow address 10.0.0.0/8
		allow address 10.1.2.3
		allow address fd00::1
		allow address from ./cfg/acl key allowed_cidrs
		deny any
	`)
	require.NoError(t, err)
	texts := make([]string, len(rules))
	for i, rule := range rules {
		texts[i] = rule.Text
		assert.Equal(t, i+1, rule.Line, "the empty lines are not counted: %s", rule.Text)
		assert.False(t, rule.Implicit)
	}
	assert.Equal(t, []string{
		"allow localhost",
		"allow dns",
		"deny network backend3",
		"allow network backend3 node dev2n2",
		"allow object ./svc/db",
		"deny object ./svc/db rid ip#1",
		"allow object prod/svc/db",
		"allow namespace .",
		"deny namespace prod",
		"allow node dev2n2",
		"deny cluster",
		"allow address 10.0.0.0/8",
		"allow address 10.1.2.3",
		"allow address fd00::1",
		"allow address from ./cfg/acl key allowed_cidrs",
		"deny any",
	}, texts)
	assert.Equal(t, Rule{Verb: VerbAllow, Kind: KindNetwork, Line: 4, Text: "allow network backend3 node dev2n2", Network: "backend3", Node: "dev2n2"}, rules[3])
	assert.Equal(t, Rule{Verb: VerbDeny, Kind: KindObject, Line: 6, Text: "deny object ./svc/db rid ip#1", Object: "./svc/db", RID: "ip#1"}, rules[5])
	assert.Equal(t, netip.MustParsePrefix("10.1.2.3/32"), rules[12].Prefix)
	assert.Equal(t, netip.MustParsePrefix("fd00::1/128"), rules[13].Prefix)
	assert.Equal(t, Rule{Verb: VerbAllow, Kind: KindAddress, Line: 15, Text: "allow address from ./cfg/acl key allowed_cidrs", From: "./cfg/acl", Key: "allowed_cidrs"}, rules[14])
}

func TestParseReadsSeveralRulesOfALine(t *testing.T) {
	// As a value set by a command, on one line: a verb starts a rule
	// where the one before is complete.
	rules, err := Parse(Keyword, "allow localhost deny dns allow node n1 deny address 10.0.0.1 allow network lan node n2 deny cluster")
	require.NoError(t, err)
	texts := make([]string, len(rules))
	for i, rule := range rules {
		texts[i] = rule.Text
		assert.Equal(t, 1, rule.Line)
	}
	assert.Equal(t, []string{"allow localhost", "deny dns", "allow node n1", "deny address 10.0.0.1", "allow network lan node n2", "deny cluster"}, texts,
		"a node after a network narrows it")

	// The network and the node as two rules: the verb is written again.
	rules, err = Parse(Keyword, "allow network backend allow node n2")
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, Rule{Verb: VerbAllow, Kind: KindNetwork, Line: 1, Text: "allow network backend", Network: "backend"}, rules[0])
	assert.Equal(t, Rule{Verb: VerbAllow, Kind: KindNode, Line: 1, Text: "allow node n2", Node: "n2"}, rules[1])

	// A word a rule takes is taken, a verb included: a node may be named
	// allow.
	rules, err = Parse(Keyword, "deny node allow allow node deny")
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, "allow", rules[0].Node)
	assert.Equal(t, "deny", rules[1].Node)
}

func TestParseMasksARange(t *testing.T) {
	rules, err := Parse(Keyword, "allow address 10.1.2.3/8")
	require.NoError(t, err)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), rules[0].Prefix)
}

func TestParseReadsThePortQualifier(t *testing.T) {
	rules, err := Parse(Keyword, "allow node n1 port 5432 proto tcp\ndeny any port 8000-8099 proto udp")
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.True(t, rules[0].HasPort)
	assert.Equal(t, [2]uint16{5432, 5432}, [2]uint16{rules[0].PortMin, rules[0].PortMax})
	assert.Equal(t, "tcp", rules[0].Proto)
	assert.Equal(t, [2]uint16{8000, 8099}, [2]uint16{rules[1].PortMin, rules[1].PortMax})
	assert.Equal(t, "udp", rules[1].Proto)
	assert.Equal(t, VerbDeny, rules[1].Verb)
}

func TestParseErrorsNameTheKeywordTheLineAndTheWords(t *testing.T) {
	const nouns = "localhost, dns, any, cluster, address, network, node, object, namespace"
	cases := []struct {
		value string
		err   string
	}{
		{
			"allow localhost\nallow dns\nallow netwrok backend3",
			`net_rules line 3: "allow netwrok backend3": unknown rule, expected one of ` + nouns,
		},
		{
			"allow localhost deny netwrok backend3 allow dns",
			`net_rules line 1: "deny netwrok backend3": unknown rule, expected one of ` + nouns,
		},
		{
			"allow dns\nnode n1",
			`net_rules line 2: "node n1": expected allow or deny to start a rule, as allow node n1: a rule has one verb and one noun`,
		},
		{
			"allow node n1 node n2",
			`net_rules line 1: "node n2": expected allow or deny to start a rule, as allow node n2: a rule has one verb and one noun`,
		},
		{
			"allow localhost dns deny cluster",
			`net_rules line 1: "dns": expected allow or deny to start a rule, as allow dns: a rule has one verb and one noun`,
		},
		{
			"permit node n1",
			`net_rules line 1: "permit node n1": expected allow or deny to start a rule, as allow node n1`,
		},
		{
			"allow",
			`net_rules line 1: "allow": expected what to allow, one of ` + nouns,
		},
		{
			"allow node n1 deny",
			`net_rules line 1: "deny": expected what to deny, one of ` + nouns,
		},
		{
			"allow address",
			`net_rules line 1: "allow address": expected an address or a range, as allow address 10.0.0.0/8`,
		},
		{
			"allow address 10.0.0.300",
			`net_rules line 1: "allow address 10.0.0.300": 10.0.0.300 is no address and no range, as 10.0.0.1, 10.0.0.0/8 or fd00::/8`,
		},
		{
			"deny address 10.0.0.0/33",
			`net_rules line 1: "deny address 10.0.0.0/33": 10.0.0.0/33 is no address and no range, as 10.0.0.1, 10.0.0.0/8 or fd00::/8`,
		},
		{
			"allow address from ./cfg/acl",
			`net_rules line 1: "allow address from ./cfg/acl": expected key after the datastore, as allow address from ./cfg/acl key allowed`,
		},
		{
			"allow network",
			`net_rules line 1: "allow network": expected a network name, as allow network backend`,
		},
		{
			"allow dns\nallow network lan node",
			`net_rules line 2: "allow network lan node": expected a node name after node, as allow network backend node n1`,
		},
		{
			"allow node",
			`net_rules line 1: "allow node": expected a node name, as allow node n1`,
		},
		{
			"allow object",
			`net_rules line 1: "allow object": expected an object path, as allow object ./svc/db`,
		},
		{
			"allow object ./svc/db rid",
			`net_rules line 1: "allow object ./svc/db rid": expected a resource id after rid, as allow object ./svc/db rid ip#1`,
		},
		{
			"allow namespace",
			`net_rules line 1: "allow namespace": expected a namespace name, or . for the one of the object`,
		},
		{
			"allow port 5432 proto tcp",
			`net_rules line 1: "allow port 5432 proto tcp": a port qualifies a rule, as allow any port 5432 proto tcp`,
		},
		{
			"allow any port 0 proto tcp",
			`net_rules line 1: "allow any port 0 proto tcp": port 0: a port is a number from 1 to 65535`,
		},
		{
			"allow any port 70000 proto tcp",
			`net_rules line 1: "allow any port 70000 proto tcp": port 70000: a port is a number from 1 to 65535`,
		},
		{
			"allow any port 90-80 proto tcp",
			`net_rules line 1: "allow any port 90-80 proto tcp": port 90-80: the last port of a range is not under the first`,
		},
		{
			"allow any port 80",
			`net_rules line 1: "allow any port 80": port 80: expected proto tcp or proto udp after the port`,
		},
		{
			"allow any port 80 proto icmp",
			`net_rules line 1: "allow any port 80 proto icmp": port 80: expected proto tcp or proto udp after the port`,
		},
	}
	for _, c := range cases {
		_, err := Parse(Keyword, c.value)
		assert.EqualError(t, err, c.err, c.value)
	}
}

func TestTheImplicitRulesEndAListAndNoEmptyOne(t *testing.T) {
	rules, err := Parse(Keyword, "deny cluster")
	require.NoError(t, err)
	all := rules.WithImplicit()
	assert.Equal(t, Texts{"deny cluster", "allow localhost", "allow dns", "deny any"}, all.Texts())
	assert.False(t, all[0].Implicit)
	assert.True(t, all[1].Implicit && all[2].Implicit && all[3].Implicit)
	assert.Len(t, rules, 1, "the rules written are left as they are")
	assert.Empty(t, Rules{}.WithImplicit(), "no rule is no list, and no filter")
}

func TestValidatorRefusesWhatNoFilterIsMadeOfYet(t *testing.T) {
	validate := Validator(Keyword, BackendCgroup)
	assert.NoError(t, validate("allow localhost\ndeny node n1\nallow network lan node n1\ndeny cluster\nallow address fd00::/8\nallow any\ndeny dns"))
	assert.NoError(t, validate("allow dns\nallow object ./svc/db\ndeny object ./svc/{name}-db rid ip#1\nallow object prod/svc/db\nallow namespace .\ndeny namespace prod"))
	assert.NoError(t, validate("allow address from ./cfg/acl key allowed\ndeny address from ./sec/acl key denied\nallow address from ./cfg/{name} key k"))
	assert.EqualError(t, validate("deny address from ./svc/acl key allowed"),
		`net_rules line 1: "deny address from ./svc/acl key allowed": ./svc/acl is no datastore, a cfg or a sec, as ./cfg/acl`)
	assert.EqualError(t, validate("deny address from acl key allowed"),
		`net_rules line 1: "deny address from acl key allowed": acl is no datastore, a cfg or a sec, as ./cfg/acl`)
	assert.EqualError(t, validate("deny nodes n1"),
		`net_rules line 1: "deny nodes n1": unknown rule, expected one of localhost, dns, any, cluster, address, network, node, object, namespace`)
}

func TestValidatorRefusesAPortWhereTheBackendKnowsNone(t *testing.T) {
	assert.EqualError(t, Validator(Keyword, BackendCgroup)("allow localhost\ndeny node n1 port 5432 proto tcp"),
		`net_rules line 2: "deny node n1 port 5432 proto tcp": port 5432: the cgroup filter of the app, task, oci and lxc container resources knows no port, only the nftables filter of container.kvm does`)
	assert.NoError(t, Validator(Keyword, BackendNetdev)("allow localhost\ndeny node n1 port 5432 proto tcp\nallow any port 8000-8099 proto udp"))
	assert.False(t, BackendCgroup.Ports())
	assert.True(t, BackendNetdev.Ports())
}
