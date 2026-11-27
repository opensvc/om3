package netrule

import (
	"encoding/json"
	"math/rand"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

// testView is a cluster of two nodes and three networks: lan, of one range
// the nodes hand out nothing of, backend, each node handing out a part, and
// l2, of no range.
func testView() Data {
	return Data{
		Networks: map[string]DataNetwork{
			"lan": {Ranges: prefixes("192.168.0.0/24")},
			"backend": {
				Ranges: prefixes("10.100.0.0/22", "fd00:100::/64"),
				NodeRanges: map[string][]netip.Prefix{
					"n1": prefixes("10.100.0.0/24"),
					"n2": prefixes("10.100.1.0/24"),
				},
			},
			"l2": {},
		},
		NodeAddrs: map[string][]netip.Addr{
			"n1": {netip.MustParseAddr("10.29.0.11"), netip.MustParseAddr("fd01::11")},
			"n2": {netip.MustParseAddr("10.29.0.12")},
			"n3": {},
		},
		Nameservers: []netip.Addr{netip.MustParseAddr("10.29.0.53")},
	}
}

func resolve(t *testing.T, value string) (Filter, Notes) {
	t.Helper()
	rules, err := Parse(Keyword, value)
	require.NoError(t, err)
	return Resolve(rules, testView(), "test")
}

// implicitSteps is what ends the filter of every list over the test view.
const implicitSteps = "implicit allow 127.0.0.0/8 ::1/128, implicit allow 10.29.0.53/32, implicit deny any"

// flat returns the two lists a backend knowing no order is given.
func flat(filter Filter) (allow, deny []string) {
	a, d := filter.Flat()
	return Strings(a), Strings(d)
}

func noteTexts(notes Notes) []string {
	texts := make([]string, len(notes))
	for i, note := range notes {
		texts[i] = note.String()
	}
	return texts
}

func TestResolveIsAStepPerRuleThenTheImplicitOnes(t *testing.T) {
	filter, notes := resolve(t, "allow node n2")
	assert.Empty(t, notes)
	assert.Equal(t, "allow 10.29.0.12/32, "+implicitSteps, filter.String(),
		"the loopback addresses and the nameservers are allowed without a rule, and the rest is denied")
	allow, deny := flat(filter)
	assert.Equal(t, []string{"10.29.0.12/32", "10.29.0.53/32", "127.0.0.0/8", "::1/128"}, allow)
	assert.Equal(t, []string{"0.0.0.0/0", "::/0"}, deny)
	assert.Equal(t, "allow 10.29.0.12/32, "+implicitSteps, filter.Describe(), "the status shows the implicit rules")
}

func TestResolveALastAllowAnyAllowsTheRest(t *testing.T) {
	filter, notes := resolve(t, "deny node n2\ndeny address fd00::/8\nallow any")
	assert.Empty(t, notes)
	assert.Equal(t, "deny 10.29.0.12/32, deny fd00::/8, allow any, "+implicitSteps, filter.String())
	assert.Equal(t, "deny 10.29.0.12/32, deny fd00::/8, allow any", filter.Describe(),
		"the implicit rules decide of nothing after an allow any, and are not said")
	allow, deny := flat(filter)
	assert.Empty(t, allow, "what no deny names passes a backend passing the unmatched: every address is not carved around the denied ones")
	assert.Equal(t, []string{"10.29.0.12/32", "fd00::/8"}, deny)
}

func TestResolveTheFirstMatchWins(t *testing.T) {
	// A deny before the allow of what holds it carves it.
	filter, _ := resolve(t, "deny address 192.168.0.128/25\ndeny node n2\nallow network lan\nallow node n2")
	allow, deny := flat(filter)
	assert.Equal(t, []string{"10.29.0.53/32", "127.0.0.0/8", "192.168.0.0/25", "::1/128"}, allow,
		"the denied half of the network and the denied node are taken out of the allowed")
	assert.Equal(t, Strings(Any()), deny)

	// The same deny after the allow decides of nothing the allow matches.
	filter, _ = resolve(t, "allow network lan\nallow node n2\ndeny address 192.168.0.128/25\ndeny node n2")
	allow, _ = flat(filter)
	assert.Equal(t, []string{"10.29.0.12/32", "10.29.0.53/32", "127.0.0.0/8", "192.168.0.0/24", "::1/128"}, allow)

	// An allow of a host, a deny of its network, an allow of the rest.
	filter, _ = resolve(t, "allow address 192.168.0.7\ndeny network lan\nallow any")
	allow, deny = flat(filter)
	assert.Equal(t, []string{"192.168.0.7/32"}, allow)
	assert.Equal(t, []string{"192.168.0.0/24"}, deny)
}

func TestResolveAWrittenRuleWinsOverTheImplicitOnes(t *testing.T) {
	filter, _ := resolve(t, "allow node n2\ndeny dns\ndeny localhost")
	allow, deny := flat(filter)
	assert.Equal(t, []string{"10.29.0.12/32"}, allow)
	assert.Equal(t, Strings(Any()), deny)

	filter, _ = resolve(t, "allow node n2\ndeny address ::1")
	allow, _ = flat(filter)
	assert.Equal(t, []string{"10.29.0.12/32", "10.29.0.53/32", "127.0.0.0/8"}, allow)

	// A deny of what holds the nameserver takes it away, unless an
	// allow dns is written before it.
	filter, _ = resolve(t, "deny address 10.29.0.0/24\nallow any")
	_, deny = flat(filter)
	assert.Equal(t, []string{"10.29.0.0/24"}, deny)
	filter, _ = resolve(t, "allow dns\ndeny address 10.29.0.0/24\nallow any")
	allow, deny = flat(filter)
	assert.Equal(t, []string{"10.29.0.53/32"}, allow)
	assert.Equal(t, []string{"10.29.0.0/24"}, deny)
}

func TestResolveDenyAny(t *testing.T) {
	filter, _ := resolve(t, "deny any")
	allow, deny := flat(filter)
	assert.Empty(t, allow, "the implicit rules come after the deny any, and decide of nothing")
	assert.Equal(t, Strings(Any()), deny)
	assert.Equal(t, "deny any", filter.Describe())

	filter, _ = resolve(t, "deny any\nallow cluster")
	allow, deny = flat(filter)
	assert.Empty(t, allow, "everything allowed after is denied first")
	assert.Equal(t, Strings(Any()), deny)

	filter, _ = resolve(t, "allow cluster\ndeny any")
	allow, _ = flat(filter)
	assert.Equal(t, []string{"10.29.0.11/32", "10.29.0.12/32", "fd01::11/128"}, allow, "nor the loopback nor the nameserver: the deny any is written before them")
}

func TestResolveAllowAnyAloneFiltersNothing(t *testing.T) {
	filter, _ := resolve(t, "allow any")
	assert.False(t, filter.IsZero(), "the steps are the ones of the rules")
	assert.True(t, filter.Flattened().IsZero(), "and a backend knowing no order is given no range")
	assert.Equal(t, "allow any", filter.Describe())
}

func TestResolveNoRuleIsNoFilter(t *testing.T) {
	filter, notes := resolve(t, "")
	assert.True(t, filter.IsZero())
	assert.Empty(t, filter.Steps, "no implicit rule ends no list")
	assert.Empty(t, notes)
	assert.Equal(t, "no filter", filter.Describe())
}

func TestResolveKinds(t *testing.T) {
	cases := []struct {
		rule string
		want []string
	}{
		{"localhost", []string{"127.0.0.0/8", "::1/128"}},
		{"dns", []string{"10.29.0.53/32"}},
		{"address 172.16.0.0/12", []string{"172.16.0.0/12"}},
		{"address fd00::5", []string{"fd00::5/128"}},
		{"network lan", []string{"192.168.0.0/24"}},
		{"network backend", []string{"10.100.0.0/22", "fd00:100::/64"}},
		{"network backend node n2", []string{"10.100.1.0/24"}},
		{"node n1", []string{"10.29.0.11/32", "fd01::11/128"}},
		{"cluster", []string{"10.29.0.11/32", "10.29.0.12/32", "fd01::11/128"}},
	}
	for _, c := range cases {
		for _, verb := range []Verb{VerbAllow, VerbDeny} {
			filter, notes := resolve(t, string(verb)+" "+c.rule)
			assert.Empty(t, notes, c.rule)
			require.Len(t, filter.Steps, 4, c.rule)
			assert.Equal(t, verb, filter.Steps[0].Verb, c.rule)
			assert.Equal(t, c.want, Strings(filter.Steps[0].Ranges), c.rule)
		}
	}
}

func TestResolveNotesTheRulesResolvingToNothing(t *testing.T) {
	filter, notes := resolve(t,
		"allow network nowhere\nallow network backend node n9\nallow node n9\nallow network l2\nallow network lan node n1\nallow node n3\ndeny network elsewhere")
	assert.Equal(t, []string{
		"allow network nowhere: no such network nowhere for now, resolves to nothing until it exists",
		"allow network backend node n9: no such node n9 for now, resolves to nothing until it joins the cluster",
		"allow node n9: no such node n9 for now, resolves to nothing until it joins the cluster",
		"allow network l2: the network l2 has no address range, resolves to nothing",
		"allow network lan node n1: the node n1 hands out no part of the network lan for now, resolves to nothing until it does",
		"allow node n3: the node n3 has no known address, resolves to nothing",
		"deny network elsewhere: no such network elsewhere for now, resolves to nothing until it exists",
	}, noteTexts(notes))
	require.Len(t, filter.Steps, 10, "a rule resolving to nothing keeps its step, for a backend making a set of each")
	allow, deny := flat(filter)
	assert.Equal(t, []string{"10.29.0.53/32", "127.0.0.0/8", "::1/128"}, allow,
		"allow rules resolving to nothing leave the filter stricter: the rest is still denied")
	assert.Equal(t, Strings(Any()), deny)
	assert.Equal(t, implicitSteps, filter.Describe(), "a step matching nothing is not said")
}

// The use case of the keyword: a service on the lan network reaching its
// namespace, the nameservers and the world, and nothing of the cluster.
func TestResolveTheServiceCutFromTheClusterAndOpenToTheWorld(t *testing.T) {
	view := testView()
	view.Objects = Objects{
		"test/svc/web": {"ip#1": addrs("192.168.0.21")},
		"test/svc/db":  {"ip#1": addrs("192.168.0.22")},
		"prod/svc/db":  {"ip#1": addrs("192.168.0.99")},
	}
	rules, err := Parse(Keyword, `
		allow namespace .
		allow dns
		deny cluster
		deny network lan
		allow any
	`)
	require.NoError(t, err)
	filter, notes := Resolve(rules, view, "test")
	assert.Empty(t, notes)
	assert.Equal(t, "allow 192.168.0.21/32 192.168.0.22/32, allow 10.29.0.53/32, deny 10.29.0.11/32 10.29.0.12/32 fd01::11/128, deny 192.168.0.0/24, allow any", filter.Describe())
	allow, deny := flat(filter)
	assert.Equal(t, []string{"10.29.0.53/32", "192.168.0.21/32", "192.168.0.22/32"}, allow)
	assert.Equal(t, []string{"10.29.0.11/32", "10.29.0.12/32", "192.168.0.0/24", "fd01::11/128"}, deny)
	for addr, want := range map[string]bool{
		"192.168.0.21": true,  // its namespace
		"192.168.0.99": false, // another namespace, on the lan
		"192.168.0.1":  false, // the lan
		"10.29.0.53":   true,  // the nameserver, a cluster address or not
		"10.29.0.11":   false, // a node
		"fd01::11":     false,
		"8.8.8.8":      true, // the world
		"2001:db8::1":  true,
		"127.0.0.1":    true,
	} {
		assert.Equal(t, want, firstMatch(filter, netip.MustParseAddr(addr)), addr)
		assert.Equal(t, want, flatPass(filter, netip.MustParseAddr(addr)), addr)
	}
}

// keyView is the test view with the datastores of the test namespace: a
// cfg and a sec, each with a key of addresses.
func keyView() Data {
	view := testView()
	view.Keys = map[string]map[string]string{
		"test/cfg/acl": {
			"allowed": "10.1.0.0/16\n\n# the office\n  192.168.7.9  \nfd00:7::/64\n",
			"bad":     "10.2.0.0/16\nnot an address\n10.3.0.0/33\n10.4.0.1",
			"empty":   "\n# nothing yet\n",
		},
		"test/sec/acl":  {"peers": "10.8.2.3\noops"},
		"prod/cfg/acl":  {"allowed": "10.9.0.0/16"},
		"test/cfg/web":  {"allowed": "10.5.0.0/16"},
		"test/svc/data": {"allowed": "10.6.0.0/16"},
		// Datastores of other namespaces sharing their keys: with
		// the test namespace, with every namespace, and with another
		// than the test one.
		"infra/cfg/acl":  {"allowed": "10.10.0.0/16\nnope"},
		"infra/sec/acl":  {"peers": "10.11.2.3"},
		"infra/cfg/all":  {"allowed": "10.12.0.0/16"},
		"infra/cfg/else": {"allowed": "10.13.0.0/16"},
	}
	view.Shares = map[string][]string{
		"infra/cfg/acl":  {"dev", "test"},
		"infra/sec/acl":  {"test"},
		"infra/cfg/all":  {"*"},
		"infra/cfg/else": {"dev"},
	}
	return view
}

func resolveKeys(t *testing.T, value string) (Filter, []string) {
	t.Helper()
	rules, err := Parse(Keyword, value)
	require.NoError(t, err)
	filter, notes := Resolve(rules, keyView(), "test")
	return filter, noteTexts(notes)
}

func TestResolveTheAddressesOfAKey(t *testing.T) {
	// One address or range per line, the empty lines and the comments
	// skipped, ./ being the namespace of the object.
	filter, notes := resolveKeys(t, "allow address from ./cfg/acl key allowed\nallow address from test/sec/acl key peers")
	assert.Equal(t, []string{"allow address from test/sec/acl key peers: the line 2 of the key peers of the datastore test/sec/acl is no address and no range, left out"}, notes,
		"the line of a sec is named by its number, not by its text")
	allow, _ := flat(filter)
	assert.Equal(t, []string{"10.1.0.0/16", "10.8.2.3/32", "10.29.0.53/32", "127.0.0.0/8", "192.168.7.9/32", "::1/128", "fd00:7::/64"}, allow)

	// A line that is no address is left out and named, the others
	// applying.
	filter, notes = resolveKeys(t, "deny address from ./cfg/acl key bad\nallow any")
	_, deny := flat(filter)
	assert.Equal(t, []string{"10.2.0.0/16", "10.4.0.1/32"}, deny)
	assert.Equal(t, []string{
		`deny address from ./cfg/acl key bad: the line 2 of the key bad of the datastore test/cfg/acl, "not an address", is no address and no range, left out`,
		`deny address from ./cfg/acl key bad: the line 3 of the key bad of the datastore test/cfg/acl, "10.3.0.0/33", is no address and no range, left out`,
	}, notes)

	// On a port, where the backend knows them.
	filter, notes = resolveKeys(t, "allow address from ./cfg/web key allowed port 443 proto tcp")
	assert.Empty(t, notes)
	assert.Equal(t, "allow tcp/443@10.5.0.0/16, "+implicitSteps, filter.Describe())
}

func TestResolveAKeyThatIsNotThereIsNothing(t *testing.T) {
	filter, notes := resolveKeys(t,
		"allow address from ./cfg/nope key allowed\nallow address from ./cfg/acl key nope\nallow address from ./cfg/acl key empty\nallow address from prod/cfg/acl key allowed\ndeny address from ./sec/nope key denied")
	assert.Equal(t, []string{
		"allow address from ./cfg/nope key allowed: no such datastore test/cfg/nope for now, resolves to nothing until it exists",
		"allow address from ./cfg/acl key nope: no such key nope in the datastore test/cfg/acl for now, resolves to nothing until it is set",
		"allow address from ./cfg/acl key empty: the key empty of the datastore test/cfg/acl holds no address, resolves to nothing",
		"allow address from prod/cfg/acl key allowed: the datastore prod/cfg/acl is of another namespace, and its share keyword does not share its keys with the namespace test, resolves to nothing",
		"deny address from ./sec/nope key denied: no such datastore test/sec/nope for now, resolves to nothing until it exists",
	}, notes)
	allow, deny := flat(filter)
	assert.Equal(t, []string{"10.29.0.53/32", "127.0.0.0/8", "::1/128"}, allow,
		"a datastore of another namespace not sharing its keys is not read, whatever it holds")
	assert.Equal(t, Strings(Any()), deny)
}

// A datastore of another namespace is read when its share keyword names
// the namespace of the object, or every namespace, as the keys a resource
// installs are, and not when it names others only.
func TestResolveTheAddressesOfAKeyOfASharedDatastore(t *testing.T) {
	filter, notes := resolveKeys(t,
		"allow address from infra/cfg/acl key allowed\nallow address from infra/sec/acl key peers\nallow address from infra/cfg/all key allowed\nallow address from infra/cfg/else key allowed")
	assert.Equal(t, []string{
		`allow address from infra/cfg/acl key allowed: the line 2 of the key allowed of the datastore infra/cfg/acl, "nope", is no address and no range, left out`,
		"allow address from infra/cfg/else key allowed: the datastore infra/cfg/else is of another namespace, and its share keyword does not share its keys with the namespace test, resolves to nothing",
	}, notes)
	allow, _ := flat(filter)
	assert.Equal(t, []string{"10.10.0.0/16", "10.11.2.3/32", "10.12.0.0/16", "10.29.0.53/32", "127.0.0.0/8", "::1/128"}, allow)

	// The same of a deny rule: what is not shared denies nothing.
	filter, notes = resolveKeys(t, "deny address from infra/cfg/all key allowed\ndeny address from infra/cfg/else key allowed\ndeny address from infra/cfg/else key nope\nallow any")
	_, deny := flat(filter)
	assert.Equal(t, []string{"10.12.0.0/16"}, deny)
	assert.Equal(t, []string{
		"deny address from infra/cfg/else key allowed: the datastore infra/cfg/else is of another namespace, and its share keyword does not share its keys with the namespace test, resolves to nothing",
		"deny address from infra/cfg/else key nope: the datastore infra/cfg/else is of another namespace, and its share keyword does not share its keys with the namespace test, resolves to nothing",
	}, notes, "nothing is said of the keys a datastore not shared holds, there or not")

	// The namespace of the object decides: the same rule, for an
	// object of a namespace the datastore names.
	a, err := Parse(Keyword, "allow address from infra/cfg/else key allowed")
	require.NoError(t, err)
	filter, n := Resolve(a, keyView(), "dev")
	assert.Empty(t, n)
	allow, _ = flat(filter)
	assert.Contains(t, allow, "10.13.0.0/16")

	// A rule refused is told from one naming what does not exist yet.
	a, err = Parse(Keyword, "allow address from infra/cfg/else key allowed\nallow address from infra/cfg/nope key allowed")
	require.NoError(t, err)
	_, n = Resolve(a, keyView(), "test")
	require.Len(t, n, 2)
	assert.True(t, n[0].Refused)
	assert.False(t, n[1].Refused)
}

// The addresses of a cfg another namespace shares are shown as the ones of
// a cfg of the namespace, the share being the consent to read its keys, and
// the ones of a shared sec are counted, as the ones of any sec.
func TestTheAddressesOfASharedCfgAreShown(t *testing.T) {
	a, err := Parse(Keyword, "allow address from infra/cfg/acl key allowed\nallow address from infra/sec/acl key peers\nallow address from infra/cfg/else key allowed")
	require.NoError(t, err)
	public, hidden := Public(a, keyView(), "test")
	shown, _ := flat(public)
	assert.Contains(t, shown, "10.10.0.0/16")
	assert.NotContains(t, shown, "10.11.2.3/32")
	assert.NotContains(t, shown, "10.13.0.0/16")
	assert.Equal(t, Foreign{Secret: 1, Addrs: 1}, hidden)
	assert.False(t, a[0].Hidden("test"))
	assert.True(t, a[1].Hidden("test"))
}

// A view remembering its answers remembers them by the namespace asking.
func TestAFrozenViewRemembersAKeyByTheNamespaceAsking(t *testing.T) {
	view := freeze(keyView())
	other, err := naming.ParsePath("infra/cfg/else")
	require.NoError(t, err)
	_, err = view.Key(other, "allowed", "dev")
	assert.NoError(t, err)
	_, err = view.Key(other, "allowed", "test")
	assert.ErrorIs(t, err, ErrNotShared)
	_, err = view.Key(other, "allowed", "infra")
	assert.NoError(t, err, "its own namespace needs no share")
}

func TestTheAddressesOfASecAreCountedNotShown(t *testing.T) {
	a, err := Parse(Keyword, "allow address from ./cfg/acl key allowed\ndeny address from ./sec/acl key peers\nallow object prod/svc/db")
	require.NoError(t, err)
	view := keyView()
	view.Objects = objectView().Objects
	public, hidden := Public(a, view, "test")
	require.Len(t, public.Steps, 6, "a rule whose addresses are not shown keeps its step, empty")
	assert.Empty(t, public.Steps[1].Ranges)
	assert.Empty(t, public.Steps[2].Ranges)
	assert.Equal(t, []string{"10.1.0.0/16", "192.168.7.9/32", "fd00:7::/64"}, Strings(public.Steps[0].Ranges), "the keys of a cfg are read by who reads its namespace")
	assert.Equal(t, Foreign{Rules: 1, Secret: 1, Addrs: 2}, hidden)
	assert.Equal(t, ", and 2 addresses of 2 rules naming another namespace or reading a sec", hidden.Describe())
	assert.Equal(t, ", and 1 address of 1 rule reading a sec", Foreign{Secret: 1, Addrs: 1}.Describe())
	assert.Equal(t, ", and 3 addresses of 2 rules naming another namespace", Foreign{Rules: 2, Addrs: 3}.Describe())
	assert.True(t, Foreign{Secret: 1}.Any())
	assert.False(t, Foreign{}.Any())
	full, _ := Resolve(a, view, "test")
	assert.Equal(t, []string{"10.8.2.3/32"}, Strings(full.Steps[1].Ranges), "the filter itself holds them")
	assert.Equal(t, []string{"10.0.2.5/32"}, Strings(full.Steps[2].Ranges))
}

func TestDatastoresOfTheRules(t *testing.T) {
	rules, err := Parse(Keyword, "allow address from ./cfg/acl key allowed\nallow address from test/cfg/acl key bad\nallow address 10.0.0.1\nallow address from prod/cfg/acl key allowed\ndeny address from ./sec/acl key peers")
	require.NoError(t, err)
	l := Datastores("test", rules)
	texts := make([]string, len(l))
	for i, p := range l {
		texts[i] = p.String()
	}
	assert.Equal(t, []string{"test/cfg/acl", "prod/cfg/acl", "test/sec/acl"}, texts, "each once, the ones of another namespace too: whether they are read is their share keyword's to say")
	assert.True(t, rules[4].Secret())
	assert.True(t, rules[4].Hidden("test"))
	assert.False(t, rules[0].Hidden("test"))
}

func TestUnknownWarnsOfTheKeysThatAreNotThere(t *testing.T) {
	rules, err := Parse(Keyword, "allow address from ./cfg/acl key allowed\nallow address from ./cfg/acl key bad\ndeny address from ./cfg/nope key k\nallow address from prod/cfg/acl key allowed")
	require.NoError(t, err)
	assert.Equal(t, []string{
		`allow address from ./cfg/acl key bad: the line 2 of the key bad of the datastore test/cfg/acl, "not an address", is no address and no range, left out`,
		`allow address from ./cfg/acl key bad: the line 3 of the key bad of the datastore test/cfg/acl, "10.3.0.0/33", is no address and no range, left out`,
		"deny address from ./cfg/nope key k: no such datastore test/cfg/nope for now, resolves to nothing until it exists",
		"allow address from prod/cfg/acl key allowed: the datastore prod/cfg/acl is of another namespace, and its share keyword does not share its keys with the namespace test, resolves to nothing",
	}, noteTexts(Unknown(rules, keyView(), "test", false)))
}

func TestResolveAPortOpensThePortAlone(t *testing.T) {
	filter, notes := resolve(t, "allow node n2 port 22 proto tcp\nallow network lan port 8000-8099 proto udp")
	assert.Empty(t, notes)
	assert.True(t, filter.HasPorts())
	assert.Equal(t, "allow tcp/22@10.29.0.12/32, allow udp/8000-8099@192.168.0.0/24, "+implicitSteps, filter.Describe(),
		"a rule qualified by a port allows its port, not the address")
	assert.Equal(t, Step{Verb: VerbAllow, Ranges: prefixes("192.168.0.0/24"), Proto: "udp", Min: 8000, Max: 8099}, filter.Steps[1])
	assert.True(t, firstMatchPort(filter, "10.29.0.12", "tcp", 22))
	assert.False(t, firstMatchPort(filter, "10.29.0.12", "tcp", 23))
	assert.False(t, firstMatchPort(filter, "10.29.0.12", "udp", 22))
	assert.False(t, firstMatchPort(filter, "10.29.0.12", "", 0), "a packet with no port, as a ping, matches no step on ports")
}

func TestResolvePortsInTheOrderOfTheRules(t *testing.T) {
	// A denied port before the allow of its network closes that port
	// alone, and an allowed port before the deny of its network is the
	// one thing open there.
	filter, _ := resolve(t, "deny network lan port 25 proto tcp\nallow network lan\nallow node n2 port 22 proto tcp\ndeny cluster\nallow any")
	assert.False(t, firstMatchPort(filter, "192.168.0.9", "tcp", 25))
	assert.True(t, firstMatchPort(filter, "192.168.0.9", "tcp", 80))
	assert.True(t, firstMatchPort(filter, "192.168.0.9", "", 0))
	assert.True(t, firstMatchPort(filter, "10.29.0.12", "tcp", 22))
	assert.False(t, firstMatchPort(filter, "10.29.0.12", "tcp", 80))
	assert.False(t, firstMatchPort(filter, "10.29.0.11", "tcp", 22))
	assert.True(t, firstMatchPort(filter, "8.8.8.8", "udp", 53))
	assert.Equal(t, "deny tcp/25@192.168.0.0/24, allow 192.168.0.0/24, allow tcp/22@10.29.0.12/32, deny 10.29.0.11/32 10.29.0.12/32 fd01::11/128, allow any", filter.Describe())

	// An any on a port ends nothing: the rules after it are reached.
	filter, _ = resolve(t, "deny any port 25 proto tcp\nallow node n2")
	assert.Equal(t, "deny tcp/25@0.0.0.0/0 tcp/25@::/0, allow 10.29.0.12/32, "+implicitSteps, filter.Describe())
}

func TestUnknownWarnsOfWhatTheClusterDoesNotHave(t *testing.T) {
	rules, err := Parse(Keyword, "allow network nowhere\ndeny network backend node n9\nallow node n9\nallow node n1\nallow network lan\nallow network l2\nallow object ./svc/db\nallow address 10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"allow network nowhere: no such network nowhere for now, resolves to nothing until it exists",
		"deny network backend node n9: no such node n9 for now, resolves to nothing until it joins the cluster",
		"allow node n9: no such node n9 for now, resolves to nothing until it joins the cluster",
	}, noteTexts(Unknown(rules, testView(), "test", false)), "where nobody tells the objects, a rule naming one is not judged")
}

func TestFilterEqual(t *testing.T) {
	a := FlatFilter(prefixes("127.0.0.0/8", "10.0.0.0/25", "10.0.0.128/25"), Any())
	b := FlatFilter(prefixes("10.0.0.0/24", "127.0.0.0/8"), prefixes("::/0", "0.0.0.0/0"))
	assert.True(t, a.Equal(b), "whatever the order and the split of the ranges")
	assert.False(t, a.Equal(FlatFilter(nil, Any())))
	step := Step{Verb: VerbAllow, Ranges: prefixes("10.0.0.0/24"), Proto: "tcp", Min: 80, Max: 85}
	other := step
	other.Proto = "udp"
	assert.False(t, Filter{Steps: []Step{step}}.Equal(Filter{Steps: []Step{other}}))
	other = step
	other.Verb = VerbDeny
	assert.False(t, Filter{Steps: []Step{step}}.Equal(Filter{Steps: []Step{other}}))
	assert.False(t, Filter{Steps: []Step{step}}.Equal(Filter{Steps: []Step{step, step}}), "the steps are compared one by one")
	assert.True(t, Filter{Steps: []Step{{Verb: VerbDeny}}}.IsZero())
	assert.False(t, Filter{Steps: []Step{step}}.IsZero())
}

// A filter an om knowing no order recorded is read as the steps saying the
// same: its flat form is the two lists it held.
func TestFilterReadsTheListsWrittenBeforeTheRulesWereOrdered(t *testing.T) {
	var filter Filter
	require.NoError(t, json.Unmarshal([]byte(`{"allow":["10.29.0.12/32","127.0.0.0/8"],"deny":["0.0.0.0/0","::/0"]}`), &filter))
	allow, deny := flat(filter)
	assert.Equal(t, []string{"10.29.0.12/32", "127.0.0.0/8"}, allow)
	assert.Equal(t, Strings(Any()), deny)

	require.NoError(t, json.Unmarshal([]byte(`{"allow":[],"deny":["10.29.0.13/32"]}`), &filter))
	allow, deny = flat(filter)
	assert.Empty(t, allow)
	assert.Equal(t, []string{"10.29.0.13/32"}, deny)
	assert.True(t, firstMatch(filter, netip.MustParseAddr("8.8.8.8")), "with denied ranges alone, the rest passed")

	require.NoError(t, json.Unmarshal([]byte(`{"allow":["192.168.0.0/24"],"deny":["0.0.0.0/0","::/0"],
		"allow_ports":[{"prefix":"10.29.0.12/32","proto":"tcp","min":22,"max":22}],
		"deny_ports":[{"prefix":"192.168.0.0/24","proto":"tcp","min":25,"max":25}]}`), &filter))
	assert.Equal(t, "deny tcp/25@192.168.0.0/24, allow 192.168.0.0/24, allow tcp/22@10.29.0.12/32, deny any", filter.Describe())

	require.NoError(t, json.Unmarshal([]byte(`{"allow":[],"deny":[]}`), &filter))
	assert.True(t, filter.IsZero())
	assert.Empty(t, filter.Steps)

	// The steps are written, and read back as they are.
	want, _ := resolve(t, "allow node n2 port 22 proto tcp\ndeny cluster")
	b, err := json.Marshal(want)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &filter))
	assert.True(t, want.Equal(filter))
	assert.True(t, filter.Steps[2].Implicit)
}

func TestLocalViewIsRegistered(t *testing.T) {
	_, err := LocalView()
	assert.ErrorIs(t, err, ErrNoLocalView)
	RegisterLocalView(func() (View, error) { return testView(), nil })
	t.Cleanup(func() { RegisterLocalView(nil) })
	view, err := LocalView()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"n1", "n2", "n3"}, view.Nodes())
}

// The nameservers are the ones of what is filtered, which its driver knows:
// a view given others names the same networks and nodes, and opens these
// nameservers alone, without a rule and for a dns rule.
func TestWithDNS(t *testing.T) {
	view := WithDNS(testView(), []netip.Addr{netip.MustParseAddr("127.0.0.53"), netip.MustParseAddr("192.168.100.53")})
	a, err := Parse(Keyword, "allow node n2")
	require.NoError(t, err)
	filter, notes := Resolve(a, view, "test")
	assert.Empty(t, notes)
	allow, _ := flat(filter)
	assert.Equal(t, []string{"10.29.0.12/32", "127.0.0.0/8", "192.168.100.53/32", "::1/128"}, allow,
		"the nameserver of the view it wraps, 10.29.0.53, is not opened")

	d, err := Parse(Keyword, "deny dns\nallow any")
	require.NoError(t, err)
	filter, notes = Resolve(d, view, "test")
	assert.Empty(t, notes)
	_, deny := flat(filter)
	assert.Equal(t, []string{"127.0.0.53/32", "192.168.100.53/32"}, deny)

	filter, notes = Resolve(d, WithDNS(testView(), nil), "test")
	require.Len(t, notes, 1, "the implicit allow dns resolving to nothing is not noted")
	assert.Equal(t, "deny dns: the resource has no nameserver, resolves to nothing", notes[0].String())
	assert.True(t, filter.Flattened().IsZero())
}

// firstMatch judges a packet from or to the address, on no port, as the
// rules say it: by the first step matching, and passed when none does.
func firstMatch(filter Filter, addr netip.Addr) bool {
	return firstMatchPort(filter, addr.String(), "", 0)
}

// firstMatchPort judges a packet from or to the address on the port of the
// protocol given, none for a packet with no port.
func firstMatchPort(filter Filter, addr, proto string, port uint16) bool {
	a := netip.MustParseAddr(addr)
	for _, step := range filter.Steps {
		if step.HasPort() && (step.Proto != proto || port < step.Min || port > step.Max) {
			continue
		}
		for _, prefix := range step.Ranges {
			if prefix.Contains(a) {
				return step.Verb == VerbAllow
			}
		}
	}
	return true
}

// flatPass judges a packet from or to the address as systemd does over the
// two lists of the filter: passed from or to an allowed range, dropped from
// or to a denied one, and passed otherwise.
func flatPass(filter Filter, addr netip.Addr) bool {
	allow, deny := filter.Flat()
	in := func(l []netip.Prefix) bool {
		for _, prefix := range l {
			if prefix.Contains(addr) {
				return true
			}
		}
		return false
	}
	return in(allow) || !in(deny)
}

// The two lists given to a backend knowing no order let through exactly
// what the first rule matching does: random lists of rules over a small
// world of addresses, of both families, each address judged both ways.
func TestTheFlatFilterLetsThroughWhatTheFirstMatchDoes(t *testing.T) {
	rng := rand.New(rand.NewSource(20261127))
	// The world: the addresses of 10.0.0.0/28 and of fd00::/124, and the
	// ranges a step may name, down to one address and up to everything.
	world := make([]netip.Addr, 0, 40)
	for i := 0; i < 16; i++ {
		world = append(world, netip.AddrFrom4([4]byte{10, 0, 0, byte(i)}))
		b := netip.MustParseAddr("fd00::").As16()
		b[15] = byte(i)
		world = append(world, netip.AddrFrom16(b))
	}
	world = append(world, netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1"))
	pick := func() netip.Prefix {
		switch rng.Intn(12) {
		case 0:
			return anyV4
		case 1:
			return anyV6
		}
		addr := world[rng.Intn(32)]
		bits := addr.BitLen() - rng.Intn(5)
		return netip.PrefixFrom(addr, bits).Masked()
	}
	for n := 0; n < 3000; n++ {
		var filter Filter
		for i, steps := 0, rng.Intn(7); i < steps; i++ {
			step := Step{Verb: VerbAllow}
			if rng.Intn(2) == 0 {
				step.Verb = VerbDeny
			}
			switch rng.Intn(10) {
			case 0:
				step.Ranges = Any()
			case 1:
			default:
				for j, ranges := 0, 1+rng.Intn(3); j < ranges; j++ {
					step.Ranges = append(step.Ranges, pick())
				}
			}
			filter.Steps = append(filter.Steps, step)
		}
		if rng.Intn(4) > 0 {
			// As a filter made of rules ends.
			filter.Steps = append(filter.Steps,
				Step{Verb: VerbAllow, Ranges: localhost, Implicit: true},
				Step{Verb: VerbDeny, Ranges: Any(), Implicit: true})
		}
		for _, addr := range world {
			if want, got := firstMatch(filter, addr), flatPass(filter, addr); want != got {
				allow, deny := flat(filter)
				t.Fatalf("%s: %s passes the first match: %v, and the lists allow %v deny %v: %v", filter, addr, want, allow, deny, got)
			}
		}
		assert.True(t, filter.Flattened().Equal(filter.Flattened().Flattened()), "flattening a flat filter changes nothing: %s", filter)
	}
}
