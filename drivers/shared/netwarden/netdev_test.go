package netwarden

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/util/plog"
)

// fakeNft is a nft holding the tables it is loaded, as the sets of their
// filter and the number of rules of their chains: enough of nft to replay
// what the backend loads, and list it back.
type fakeNft struct {
	tables  map[string]map[string][]string
	rules   map[string]map[string]int
	scripts []string
	loadErr error

	// lost names, by table, a chain taken away by hand, as nft delete
	// chain does, until the table is loaded whole again.
	lost map[string]string
}

var (
	reElements = regexp.MustCompile(`^\s*elements = \{ (.*) \}$`)
	reSet      = regexp.MustCompile(`^\s*set (\S+) \{$`)
	reChain    = regexp.MustCompile(`^\s*chain (\S+) \{$`)
	reFlush    = regexp.MustCompile(`^flush set netdev (\S+) (\S+)$`)
	reAdd      = regexp.MustCompile(`^add element netdev (\S+) (\S+) \{ (.*) \}$`)
	reTable    = regexp.MustCompile(`^table netdev (\S+) \{$`)
	reAddTable = regexp.MustCompile(`^add table netdev (\S+)$`)
	reDelTable = regexp.MustCompile(`^delete table netdev (\S+)$`)
)

func newFakeNft() *fakeNft {
	return &fakeNft{tables: make(map[string]map[string][]string), rules: make(map[string]map[string]int)}
}

// Load applies a script whole or not at all, as nft does.
func (t *fakeNft) Load(_ context.Context, script string) error {
	t.scripts = append(t.scripts, script)
	if t.loadErr != nil {
		return t.loadErr
	}
	next := make(map[string]map[string][]string)
	for name, sets := range t.tables {
		next[name] = make(map[string][]string)
		for set, l := range sets {
			next[name][set] = append([]string{}, l...)
		}
	}
	rules := make(map[string]map[string]int)
	for name, chains := range t.rules {
		rules[name] = make(map[string]int)
		for chain, n := range chains {
			rules[name][chain] = n
		}
	}
	table, set, chain := "", "", ""
	for _, line := range strings.Split(script, "\n") {
		switch {
		case reAddTable.MatchString(line):
			name := reAddTable.FindStringSubmatch(line)[1]
			if next[name] == nil {
				next[name] = make(map[string][]string)
			}
		case reDelTable.MatchString(line):
			name := reDelTable.FindStringSubmatch(line)[1]
			if next[name] == nil {
				return fmt.Errorf("delete table %s: No such file or directory", name)
			}
			delete(next, name)
			delete(rules, name)
		case reTable.MatchString(line):
			table = reTable.FindStringSubmatch(line)[1]
			next[table] = make(map[string][]string)
			rules[table] = make(map[string]int)
			delete(t.lost, table)
		case reSet.MatchString(line):
			set = reSet.FindStringSubmatch(line)[1]
			next[table][set] = []string{}
		case reChain.MatchString(line):
			chain = reChain.FindStringSubmatch(line)[1]
			rules[table][chain] = 0
		case chain != "" && strings.TrimSpace(line) == "}":
			chain = ""
		case chain != "" && !strings.Contains(line, "type filter hook"):
			for _, word := range strings.Fields(line) {
				if name, ok := strings.CutPrefix(word, "@"); ok {
					if _, ok := next[table][name]; !ok {
						return fmt.Errorf("chain %s: no such set %s", chain, name)
					}
				}
			}
			rules[table][chain]++
		case reElements.MatchString(line):
			next[table][set] = strings.Split(reElements.FindStringSubmatch(line)[1], ", ")
		case reFlush.MatchString(line):
			m := reFlush.FindStringSubmatch(line)
			if _, ok := next[m[1]][m[2]]; !ok {
				return fmt.Errorf("flush set %s %s: No such file or directory", m[1], m[2])
			}
			next[m[1]][m[2]] = []string{}
		case reAdd.MatchString(line):
			m := reAdd.FindStringSubmatch(line)
			if _, ok := next[m[1]][m[2]]; !ok {
				return fmt.Errorf("add element %s %s: No such file or directory", m[1], m[2])
			}
			next[m[1]][m[2]] = append(next[m[1]][m[2]], strings.Split(m[3], ", ")...)
		}
	}
	t.tables, t.rules = next, rules
	return nil
}

// List lists a table as nft -j does: the chains, their rules, and the sets,
// their elements as addresses and ranges with a length.
func (t *fakeNft) List(_ context.Context, table string) ([]byte, error) {
	sets, ok := t.tables[table]
	if !ok {
		return nil, errNoTable
	}
	addr := func(s string) any {
		prefix := netip.MustParsePrefix(s)
		if prefix.IsSingleIP() {
			return prefix.Addr().String()
		}
		return map[string]any{"prefix": map[string]any{"addr": prefix.Addr().String(), "len": prefix.Bits()}}
	}
	objects := []any{}
	for _, chain := range []struct{ name, hook string }{{"from_guest", "ingress"}, {"to_guest", "egress"}} {
		if t.lost[table] == chain.name {
			continue
		}
		objects = append(objects, map[string]any{"chain": map[string]any{"name": chain.name, "hook": chain.hook}})
		for i := 0; i < t.rules[table][chain.name]; i++ {
			objects = append(objects, map[string]any{"rule": map[string]any{"chain": chain.name, "handle": i + 2}})
		}
	}
	for name, l := range sets {
		elem := make([]any, 0)
		for _, s := range l {
			elem = append(elem, addr(s))
		}
		objects = append(objects, map[string]any{"set": map[string]any{"name": name, "elem": elem}})
	}
	return json.Marshal(map[string]any{"nftables": objects})
}

// setupNetdev fakes nft, the interfaces and the kernel, and returns the
// fake nft and the target of a guest of two interfaces.
func setupNetdev(t *testing.T) (*fakeNft, Target) {
	t.Helper()
	_, target := setup(t)
	f := newFakeNft()
	savedNft, savedHasNft, savedHasDev, savedRelease := nft, hasNft, hasDev, kernelRelease
	t.Cleanup(func() { nft, hasNft, hasDev, kernelRelease = savedNft, savedHasNft, savedHasDev, savedRelease })
	nft = f
	hasNft = func() bool { return true }
	hasDev = func(string) bool { return true }
	kernelRelease = func() string { return "6.8.0-149-generic" }
	dir := t.TempDir()
	return f, Target{
		Path:    naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "vm"},
		Backend: netrule.BackendNetdev,
		Devs:    []string{"kvaan0", "kvaan1"},
		VarDir:  func() string { return dir },
		Log:     plog.NewDefaultLogger(),
		DNS:     target.DNS,
	}
}

func ranges(l ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(l))
	for i, s := range l {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

// testFilter is the filter of five rules: a deny on a port, an allow, an
// allow on a range of ports, a deny resolving to nothing, and the implicit
// deny any.
func testFilter() netrule.Filter {
	return netrule.Filter{Steps: []netrule.Step{
		{Verb: netrule.VerbDeny, Ranges: netrule.Any(), Proto: "tcp", Min: 25, Max: 25},
		{Verb: netrule.VerbAllow, Ranges: ranges("127.0.0.0/8", "10.29.0.12/32", "::1/128")},
		{Verb: netrule.VerbAllow, Ranges: ranges("192.168.0.0/24", "fd00::5/128"), Proto: "udp", Min: 8000, Max: 8099},
		{Verb: netrule.VerbDeny},
		{Verb: netrule.VerbDeny, Ranges: netrule.Any(), Implicit: true},
	}}
}

func TestNetdevRuleset(t *testing.T) {
	assert.Equal(t, `add table netdev osvc_netwarden_kvaan0
delete table netdev osvc_netwarden_kvaan0
table netdev osvc_netwarden_kvaan0 {
  set s0_deny_tcp_25_25_4 {
    type ipv4_addr
    flags interval
    elements = { 0.0.0.0/0 }
  }
  set s0_deny_tcp_25_25_6 {
    type ipv6_addr
    flags interval
    elements = { ::/0 }
  }
  set s1_allow_4 {
    type ipv4_addr
    flags interval
    elements = { 10.29.0.12/32, 127.0.0.0/8 }
  }
  set s1_allow_6 {
    type ipv6_addr
    flags interval
    elements = { ::1/128 }
  }
  set s2_allow_udp_8000_8099_4 {
    type ipv4_addr
    flags interval
    elements = { 192.168.0.0/24 }
  }
  set s2_allow_udp_8000_8099_6 {
    type ipv6_addr
    flags interval
    elements = { fd00::5/128 }
  }
  set s3_deny_4 {
    type ipv4_addr
    flags interval
  }
  set s3_deny_6 {
    type ipv6_addr
    flags interval
  }
  set s4_deny_4 {
    type ipv4_addr
    flags interval
    elements = { 0.0.0.0/0 }
  }
  set s4_deny_6 {
    type ipv6_addr
    flags interval
    elements = { ::/0 }
  }
  chain from_guest {
    type filter hook ingress device "kvaan0" priority -400; policy drop;
    ether type arp accept
    ether type ip udp sport 68 udp dport 67 accept
    ether type ip6 icmpv6 type { nd-router-solicit, nd-neighbor-solicit, nd-neighbor-advert, mld-listener-query, mld-listener-report, mld-listener-done, mld2-listener-report } accept
    ether type ip6 udp sport 546 udp dport 547 accept
    ip daddr @s0_deny_tcp_25_25_4 tcp dport 25 drop
    ip6 daddr @s0_deny_tcp_25_25_6 tcp dport 25 drop
    ip daddr @s1_allow_4 accept
    ip6 daddr @s1_allow_6 accept
    ip daddr @s2_allow_udp_8000_8099_4 udp dport 8000-8099 accept
    ip6 daddr @s2_allow_udp_8000_8099_6 udp dport 8000-8099 accept
    ip daddr @s3_deny_4 drop
    ip6 daddr @s3_deny_6 drop
    ip daddr @s4_deny_4 drop
    ip6 daddr @s4_deny_6 drop
    ether type { ip, ip6 } accept
  }
  chain to_guest {
    type filter hook egress device "kvaan0" priority -400; policy drop;
    ether type arp accept
    ether type ip udp sport 67 udp dport 68 accept
    ether type ip icmp type { destination-unreachable, time-exceeded, parameter-problem } accept
    ether type ip6 icmpv6 type { destination-unreachable, packet-too-big, time-exceeded, parameter-problem, nd-router-advert, nd-neighbor-solicit, nd-neighbor-advert, mld-listener-query, mld-listener-report, mld-listener-done, mld2-listener-report } accept
    ether type ip6 udp sport 547 udp dport 546 accept
    ip saddr @s0_deny_tcp_25_25_4 tcp sport 25 drop
    ip6 saddr @s0_deny_tcp_25_25_6 tcp sport 25 drop
    ip saddr @s1_allow_4 accept
    ip6 saddr @s1_allow_6 accept
    ip saddr @s2_allow_udp_8000_8099_4 udp sport 8000-8099 accept
    ip6 saddr @s2_allow_udp_8000_8099_6 udp sport 8000-8099 accept
    ip saddr @s3_deny_4 drop
    ip6 saddr @s3_deny_6 drop
    ip saddr @s4_deny_4 drop
    ip6 saddr @s4_deny_6 drop
    ether type { ip, ip6 } accept
  }
}
`, NetdevRuleset("kvaan0", testFilter()), "a pair of sets and a line of each chain by rule, in the order of the rules, after what an interface needs to hold an address")
}

func TestNetdevSetNames(t *testing.T) {
	for i, step := range testFilter().Steps {
		for _, family := range []int{4, 6} {
			at, read, fam, err := parseNetdevSetName(netdevSetName(i, step, family))
			require.NoError(t, err)
			assert.Equal(t, i, at)
			assert.Equal(t, family, fam)
			assert.Equal(t, netdevSetName(i, step, 4), netdevSetName(i, read, 4))
		}
	}
	for _, name := range []string{"allow4", "deny6_ports", "s0_permit_4", "sx_allow_4", "s0_allow_5", "s0_allow_icmp_1_2_4", "s0_allow_tcp_0_2_4", "s0_allow_tcp_9_2_4", "s0_allow_tcp_2_4"} {
		_, _, _, err := parseNetdevSetName(name)
		assert.Error(t, err, name)
	}
}

func TestNetdevUpdateReplacesTheElementsOfTheSetsInOneTransaction(t *testing.T) {
	moved := testFilter()
	moved.Steps[1].Ranges = ranges("10.29.0.13/32")
	moved.Steps[2].Ranges = nil
	assert.Equal(t, `flush set netdev osvc_netwarden_kvaan0 s0_deny_tcp_25_25_4
flush set netdev osvc_netwarden_kvaan0 s0_deny_tcp_25_25_6
flush set netdev osvc_netwarden_kvaan0 s1_allow_4
flush set netdev osvc_netwarden_kvaan0 s1_allow_6
flush set netdev osvc_netwarden_kvaan0 s2_allow_udp_8000_8099_4
flush set netdev osvc_netwarden_kvaan0 s2_allow_udp_8000_8099_6
flush set netdev osvc_netwarden_kvaan0 s3_deny_4
flush set netdev osvc_netwarden_kvaan0 s3_deny_6
flush set netdev osvc_netwarden_kvaan0 s4_deny_4
flush set netdev osvc_netwarden_kvaan0 s4_deny_6
add element netdev osvc_netwarden_kvaan0 s0_deny_tcp_25_25_4 { 0.0.0.0/0 }
add element netdev osvc_netwarden_kvaan0 s0_deny_tcp_25_25_6 { ::/0 }
add element netdev osvc_netwarden_kvaan0 s1_allow_4 { 10.29.0.13/32 }
add element netdev osvc_netwarden_kvaan0 s4_deny_4 { 0.0.0.0/0 }
add element netdev osvc_netwarden_kvaan0 s4_deny_6 { ::/0 }
`, NetdevUpdate("kvaan0", moved))

	f, target := setupNetdev(t)
	b := Netdev{Devs: target.Devs}
	require.NoError(t, b.Load(testFilter()))
	require.Len(t, f.scripts, 1, "the tables of every interface are loaded in one transaction")
	current, err := b.Current()
	require.NoError(t, err)
	assert.True(t, current.Equal(testFilter()), current.String())

	// The addresses the rules name move: the elements of the sets are
	// replaced, the chains left as they are.
	require.NoError(t, b.Update(moved))
	require.Len(t, f.scripts, 2, "one transaction replaces the sets of every interface")
	assert.NotContains(t, f.scripts[1], "table netdev osvc_netwarden_kvaan0 {", "the chains are left as they are")
	assert.NotContains(t, f.scripts[1], "chain ")
	assert.Contains(t, f.scripts[1], "flush set netdev osvc_netwarden_kvaan0 s1_allow_4\n")
	assert.Contains(t, f.scripts[1], "flush set netdev osvc_netwarden_kvaan1 s1_allow_4\n")
	current, err = b.Current()
	require.NoError(t, err)
	assert.True(t, current.Equal(moved), current.String())

	// An interface that lost its table is loaded whole with the update
	// of the other.
	delete(f.tables, "osvc_netwarden_kvaan1")
	_, err = b.Current()
	assert.EqualError(t, err, "the interface kvaan1 has no filter: no nft table osvc_netwarden_kvaan1")
	require.NoError(t, b.Update(moved))
	assert.Contains(t, f.scripts[2], "table netdev osvc_netwarden_kvaan1 {")
	assert.Contains(t, f.scripts[2], "flush set netdev osvc_netwarden_kvaan0 s1_allow_4\n")
	current, err = b.Current()
	require.NoError(t, err)
	assert.True(t, current.Equal(moved))

	// An interface whose table lost a chain by hand is loaded whole: the
	// traffic the chain judged would go unfiltered under new sets alone.
	f.lost = map[string]string{"osvc_netwarden_kvaan0": "to_guest"}
	_, err = b.Current()
	assert.ErrorContains(t, err, "osvc_netwarden_kvaan0")
	require.NoError(t, b.Update(moved))
	assert.Contains(t, f.scripts[3], "table netdev osvc_netwarden_kvaan0 {", "the table that lost a chain is loaded whole")
	assert.Contains(t, f.scripts[3], "flush set netdev osvc_netwarden_kvaan1 s1_allow_4\n", "the other keeps its chains")
	current, err = b.Current()
	require.NoError(t, err)
	assert.True(t, current.Equal(moved))

	// So is one whose chain lost a rule by hand.
	f.rules["osvc_netwarden_kvaan1"]["from_guest"]--
	_, err = b.Current()
	assert.EqualError(t, err, "the nft table osvc_netwarden_kvaan1 of the interface kvaan1: the chain from_guest holds 14 rules, not the 15 of its 5 steps")
	require.NoError(t, b.Update(moved))
	assert.Contains(t, f.scripts[4], "table netdev osvc_netwarden_kvaan1 {")
	_, err = b.Current()
	require.NoError(t, err)

	// A set changed by hand on one interface is told.
	f.tables["osvc_netwarden_kvaan1"]["s1_allow_4"] = append(f.tables["osvc_netwarden_kvaan1"]["s1_allow_4"], "10.9.9.9/32")
	_, err = b.Current()
	assert.EqualError(t, err, "the interfaces kvaan0 and kvaan1 do not hold the same filter")

	// A transaction refused changes nothing.
	f.loadErr = errors.New("nft -f: exit status 1")
	assert.Error(t, b.Update(testFilter()))
	f.loadErr = nil

	// No filter is no table.
	assert.True(t, b.HasFilter())
	require.NoError(t, b.Update(netrule.Filter{}))
	assert.False(t, b.HasFilter())
	assert.Empty(t, f.tables)
	require.NoError(t, b.Load(netrule.Filter{}), "nothing to remove is nothing to do")
}

// A list of rules changed under a running guest is another list of lines
// of the chains: the table is loaded whole, sets and chains, in the
// transaction of the update, where the addresses moving replace the
// elements of the sets alone.
func TestNetdevUpdateWritesTheChainsAgainWhenTheRulesChange(t *testing.T) {
	f, target := setupNetdev(t)
	b := Netdev{Devs: target.Devs}
	require.NoError(t, b.Load(testFilter()))

	for name, change := range map[string]func(*netrule.Filter){
		"a rule added": func(filter *netrule.Filter) {
			filter.Steps = append([]netrule.Step{{Verb: netrule.VerbAllow, Ranges: ranges("10.0.0.1/32")}}, filter.Steps...)
		},
		"a rule removed": func(filter *netrule.Filter) { filter.Steps = filter.Steps[1:] },
		"two rules swapped": func(filter *netrule.Filter) {
			filter.Steps[0], filter.Steps[1] = filter.Steps[1], filter.Steps[0]
		},
		"the verb of a rule":  func(filter *netrule.Filter) { filter.Steps[3].Verb = netrule.VerbAllow },
		"the ports of a rule": func(filter *netrule.Filter) { filter.Steps[2].Max = 8100 },
		"a rule given a port": func(filter *netrule.Filter) {
			filter.Steps[1].Proto, filter.Steps[1].Min, filter.Steps[1].Max = "tcp", 22, 22
		},
	} {
		require.NoError(t, b.Update(testFilter()), name)
		before := len(f.scripts)
		next := testFilter()
		change(&next)
		require.NoError(t, b.Update(next), name)
		require.Len(t, f.scripts, before+1, "%s: one transaction", name)
		script := f.scripts[before]
		for _, dev := range target.Devs {
			assert.Contains(t, script, "table netdev osvc_netwarden_"+dev+" {", "%s: the table of %s is loaded whole", name, dev)
			assert.Contains(t, script, NetdevRuleset(dev, next), name)
		}
		assert.NotContains(t, script, "flush set", name)
		current, err := b.Current()
		require.NoError(t, err, name)
		assert.True(t, current.Equal(next), "%s: %s", name, current)
	}

	// The addresses alone moving after it: the sets alone.
	next := testFilter()
	next.Steps[1].Ranges = ranges("10.29.0.99/32")
	require.NoError(t, b.Update(testFilter()))
	require.NoError(t, b.Update(next))
	assert.NotContains(t, f.scripts[len(f.scripts)-1], "chain ")

	// A table an om knowing no order loaded, of sets named otherwise, is
	// loaded whole too.
	f.tables["osvc_netwarden_kvaan0"] = map[string][]string{"allow4": {"10.0.0.1/32"}, "allow6": {}, "deny4": {}, "deny6": {}}
	_, err := b.Current()
	assert.ErrorContains(t, err, "is none of a filter om loaded")
	require.NoError(t, b.Update(next))
	assert.Contains(t, f.scripts[len(f.scripts)-1], "table netdev osvc_netwarden_kvaan0 {")
	assert.NotContains(t, f.scripts[len(f.scripts)-1], "table netdev osvc_netwarden_kvaan1 {")
	current, err := b.Current()
	require.NoError(t, err)
	assert.True(t, current.Equal(next))
}

func TestParseNetdevTableRefusesWhatOmDidNotLoad(t *testing.T) {
	_, err := parseNetdevTable([]byte(`{"nftables": [{"chain": {"name": "from_guest", "hook": "ingress"}}]}`))
	assert.EqualError(t, err, "no chain on the egress hook")
	_, err = parseNetdevTable([]byte(`{"nftables": [{"chain": {"hook": "ingress"}}, {"chain": {"hook": "egress"}}, {"set": {"name": "s0_allow_4"}}]}`))
	assert.EqualError(t, err, "a set of the rule 1 is missing")
	_, err = parseNetdevTable([]byte(`{"nftables": [{"set": {"name": "s1_allow_4"}}, {"set": {"name": "s1_allow_6"}}]}`))
	assert.EqualError(t, err, "no set of the rule 1")
	_, err = parseNetdevTable([]byte(`{"nftables": [{"set": {"name": "s0_allow_4"}}, {"set": {"name": "s0_deny_6"}}]}`))
	assert.EqualError(t, err, "the two sets of the rule 1 are not of the same rule")
	_, err = parseNetdevTable([]byte(`{"nftables": [{"set": {"name": "allow4"}}]}`))
	assert.EqualError(t, err, "the set allow4 is none of a filter om loaded")
	_, err = parseNetdevTable([]byte(`{"nftables": [{"set": {"name": "s0_allow_4", "elem": [{"range": ["10.0.0.1", "10.0.0.9"]}]}}]}`))
	assert.ErrorContains(t, err, "set s0_allow_4: the element")
	_, err = parseNetdevTable([]byte(`{"nftables": [{"chain": {"name": "a", "hook": "ingress"}}, {"chain": {"name": "b", "hook": "egress"}}, {"set": {"name": "s0_allow_4"}}, {"set": {"name": "s0_allow_6"}}]}`))
	assert.EqualError(t, err, "the chain a holds 0 rules, not the 7 of its 1 steps")
}

func TestNetdevCheckRefusals(t *testing.T) {
	_, target := setupNetdev(t)
	net := T{Rules: "deny any port 25 proto tcp\nallow node n2 port 22 proto tcp"}
	require.NoError(t, net.Check(target), "the filter of the interfaces knows the ports")

	kernelRelease = func() string { return "5.15.0-91-generic" }
	assert.EqualError(t, net.Check(target),
		"netwarden: net_rules can not be applied: the kernel 5.15.0-91-generic has no egress hook on a network interface, which Linux 5.16 brought: what is sent to the guest would not be filtered")
	require.NoError(t, T{}.Check(target), "refused only with rules")
	kernelRelease = func() string { return "5.16.0" }
	require.NoError(t, net.Check(target))

	hasNft = func() bool { return false }
	assert.EqualError(t, net.Check(target),
		"netwarden: net_rules can not be applied: the filter of a network interface is set by nft, which this node has not")
	hasNft = func() bool { return true }

	target.Devs = nil
	assert.EqualError(t, net.Check(target),
		"netwarden: net_rules can not be applied: the resource has no network interface made by om to filter")

	for release, want := range map[string]bool{
		"5.15.0-91-generic": false, "5.16.0": true, "5.4.0": false, "4.18.0-553.el8.x86_64": false,
		"6.8.0-149-generic": true, "6.1.0-18-amd64": true, "10.0": true, "": true, "weird": true,
	} {
		assert.Equal(t, want, egressSupported(release), release)
	}
}

func TestNetdevApplyStatusAndClear(t *testing.T) {
	f, target := setupNetdev(t)
	net := T{Rules: "deny any port 25 proto tcp\nallow node n2 port 22 proto tcp\nallow network lan"}
	filtered, err := net.Apply(target)
	require.NoError(t, err)
	assert.True(t, filtered)
	require.Len(t, f.tables, 2)
	assert.Equal(t, []string{"0.0.0.0/0"}, f.tables["osvc_netwarden_kvaan1"]["s0_deny_tcp_25_25_4"])
	assert.Equal(t, []string{"::/0"}, f.tables["osvc_netwarden_kvaan1"]["s0_deny_tcp_25_25_6"])
	assert.Equal(t, []string{"10.29.0.12/32"}, f.tables["osvc_netwarden_kvaan1"]["s1_allow_tcp_22_22_4"])
	assert.Equal(t, []string{"192.168.0.0/24"}, f.tables["osvc_netwarden_kvaan0"]["s2_allow_4"])
	assert.Equal(t, []string{"127.0.0.0/8"}, f.tables["osvc_netwarden_kvaan0"]["s3_allow_4"], "the implicit allow localhost")
	assert.Empty(t, f.tables["osvc_netwarden_kvaan0"]["s4_allow_4"], "the implicit allow dns, of a resource with no nameserver")
	assert.Equal(t, []string{"0.0.0.0/0"}, f.tables["osvc_netwarden_kvaan0"]["s5_deny_4"], "the implicit deny any")
	assert.Len(t, f.tables["osvc_netwarden_kvaan0"], 12)

	rec, err := netrule.LoadRecord(target.appliedFile())
	require.NoError(t, err)
	assert.Equal(t, netrule.BackendNetdev, rec.Backend)
	assert.Equal(t, target.Devs, rec.Devs)
	assert.Empty(t, rec.PG)

	infos, warnings := net.Status(target)
	assert.Empty(t, warnings)
	assert.Equal(t, []string{"netwarden: deny tcp/25@0.0.0.0/0 tcp/25@::/0, allow tcp/22@10.29.0.12/32, allow 192.168.0.0/24, implicit allow 127.0.0.0/8 ::1/128, implicit deny any"}, infos)

	// The sets are not the ones applied.
	f.tables["osvc_netwarden_kvaan0"]["s2_allow_4"] = []string{"0.0.0.0/0"}
	f.tables["osvc_netwarden_kvaan1"]["s2_allow_4"] = []string{"0.0.0.0/0"}
	_, warnings = net.Status(target)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "netwarden: the nft filter of kvaan0 kvaan1 holds deny tcp/25@0.0.0.0/0 tcp/25@::/0, allow tcp/22@10.29.0.12/32, allow 0.0.0.0/0, allow 127.0.0.0/8 ::1/128, deny any, not the ")
	delete(f.tables, "osvc_netwarden_kvaan0")
	_, warnings = net.Status(target)
	assert.Equal(t, []string{"netwarden: the interface kvaan0 has no filter: no nft table osvc_netwarden_kvaan0"}, warnings)

	// The interfaces still there, the filter is kept; gone, removed with
	// the record.
	require.NoError(t, net.Clear(target))
	assert.Len(t, f.tables, 1)
	hasDev = func(string) bool { return false }
	require.NoError(t, net.Clear(target))
	assert.Empty(t, f.tables)
	_, err = netrule.LoadRecord(target.appliedFile())
	assert.Error(t, err)
}

// The use case of the keyword on a guest: a line of each chain by rule, in
// the order written, the sets holding what each rule resolves to.
func TestNetdevOfTheServiceCutFromTheClusterAndOpenToTheWorld(t *testing.T) {
	f, target := setupNetdev(t)
	lookupObjects = func([]naming.Path, []string) (netrule.Objects, netrule.Identities, error) {
		return netrule.Objects{"test/svc/web": {"ip#1": {netip.MustParseAddr("192.168.0.21")}}}, nil, nil
	}
	target.DNS = func() ([]netip.Addr, error) { return []netip.Addr{netip.MustParseAddr("10.29.0.12")}, nil }
	net := T{Rules: `
		allow namespace .
		allow dns
		deny cluster
		deny network lan
		allow any
	`}
	_, err := net.Apply(target)
	require.NoError(t, err)
	require.Len(t, f.scripts, 1)
	sets := f.tables["osvc_netwarden_kvaan0"]
	assert.Equal(t, []string{"192.168.0.21/32"}, sets["s0_allow_4"])
	assert.Equal(t, []string{"10.29.0.12/32"}, sets["s1_allow_4"], "the nameserver, a cluster node, allowed before the cluster is denied")
	assert.Equal(t, []string{"10.29.0.12/32"}, sets["s2_deny_4"])
	assert.Equal(t, []string{"192.168.0.0/24"}, sets["s3_deny_4"])
	assert.Equal(t, []string{"0.0.0.0/0"}, sets["s4_allow_4"])
	assert.Equal(t, []string{"::/0"}, sets["s4_allow_6"])
	var lines []string
	for _, line := range strings.Split(f.scripts[0], "\n") {
		if strings.HasPrefix(line, "    ip daddr @") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	assert.Equal(t, []string{
		"ip daddr @s0_allow_4 accept",
		"ip daddr @s1_allow_4 accept",
		"ip daddr @s2_deny_4 drop",
		"ip daddr @s3_deny_4 drop",
		"ip daddr @s4_allow_4 accept",
		"ip daddr @s5_allow_4 accept",
		"ip daddr @s6_allow_4 accept",
		"ip daddr @s7_deny_4 drop",
	}, lines[:8], "the chain of the first interface, the implicit rules last")
	infos, warnings := net.Status(target)
	assert.Empty(t, warnings)
	assert.Equal(t, []string{"netwarden: allow 192.168.0.21/32, allow 10.29.0.12/32, deny 10.29.0.12/32, deny 192.168.0.0/24, allow any"}, infos)
}
