package netrule

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func shadowed(t *testing.T, value string) []string {
	t.Helper()
	rules, err := Parse(Keyword, value)
	require.NoError(t, err)
	return noteTexts(Shadowed(rules, "test"))
}

// A rule a rule before it matches all of never decides: said of the rules
// as written, whatever their verbs.
func TestShadowedRulesNeverMatch(t *testing.T) {
	for value, want := range map[string][]string{
		// Any rule after an allow any, as one appended to the keyword,
		// or after a deny any.
		"allow namespace .\nallow any\ndeny node n1": {"deny node n1: never matches: allow any, written before it, matches all it names"},
		"deny any\nallow cluster\nallow address 10.0.0.1": {
			"allow cluster: never matches: deny any, written before it, matches all it names",
			"allow address 10.0.0.1: never matches: deny any, written before it, matches all it names",
		},
		// A node after the cluster, of the opposite verb or not.
		"deny cluster\nallow node n1":  {"allow node n1: never matches: deny cluster, written before it, matches all it names"},
		"allow cluster\nallow node n1": {"allow node n1: never matches: allow cluster, written before it, matches all it names"},
		// The part of a network after the network, an object after
		// its namespace or after itself whole, an address after its
		// range, a rule written twice.
		"deny network lan\nallow network lan node n1":             {"allow network lan node n1: never matches: deny network lan, written before it, matches all it names"},
		"deny namespace .\nallow object ./svc/db":                 {"allow object ./svc/db: never matches: deny namespace ., written before it, matches all it names"},
		"deny namespace test\nallow object test/svc/db rid ip#1":  {"allow object test/svc/db rid ip#1: never matches: deny namespace test, written before it, matches all it names"},
		"allow object test/svc/db\ndeny object ./svc/db rid ip#1": {"deny object ./svc/db rid ip#1: never matches: allow object test/svc/db, written before it, matches all it names"},
		"deny address 10.0.0.0/8\nallow address 10.1.2.3":         {"allow address 10.1.2.3: never matches: deny address 10.0.0.0/8, written before it, matches all it names"},
		"deny address fd00::/8\nallow address fd00:1::/64":        {"allow address fd00:1::/64: never matches: deny address fd00::/8, written before it, matches all it names"},
		"allow localhost\ndeny address 127.0.0.1":                 {"deny address 127.0.0.1: never matches: allow localhost, written before it, matches all it names"},
		"allow dns\ndeny dns":                                     {"deny dns: never matches: allow dns, written before it, matches all it names"},
		"allow address from ./cfg/acl key k\ndeny address from test/cfg/acl key k": {
			"deny address from test/cfg/acl key k: never matches: allow address from ./cfg/acl key k, written before it, matches all it names",
		},
		"allow node n1 deny node n2 deny node n1": {"deny node n1: never matches: allow node n1, written before it, matches all it names"},
		// On ports: a rule on ports shadows the rules on ports within
		// its own, and a rule on no port every rule of what it names.
		"deny any port 20-30 proto tcp\nallow node n1 port 22 proto tcp": {"allow node n1 port 22 proto tcp: never matches: deny any port 20-30 proto tcp, written before it, matches all it names"},
		"deny cluster\nallow node n1 port 22 proto tcp":                  {"allow node n1 port 22 proto tcp: never matches: deny cluster, written before it, matches all it names"},
	} {
		assert.Equal(t, want, shadowed(t, value), value)
	}
}

// What is reached, or may be, is not warned of: the exception written
// before its rule, rules the cluster alone tells apart, and the use case of
// the keyword.
func TestRulesReachedAreNotShadowed(t *testing.T) {
	for _, value := range []string{
		"",
		"allow namespace .\nallow dns\ndeny cluster\ndeny network lan\nallow any",
		"allow node n1\ndeny cluster\nallow any",
		"deny network lan node n1\nallow network lan",
		"allow object ./svc/db\ndeny namespace .",
		"allow object ./svc/db rid ip#1\ndeny object ./svc/db",
		"allow object prod/svc/db\ndeny namespace .",
		"allow address 10.1.2.3\ndeny address 10.0.0.0/8",
		"deny address 10.0.0.0/8\nallow address 10.0.0.0/7",
		"deny address 10.0.0.0/8\nallow address fd00::1",
		"deny network lan node n1\nallow network lan node n2\ndeny network backend node n1",
		// Nothing is resolved: a node and its address, the nameservers
		// and the cluster, a key and an address.
		"deny node n1\nallow address 10.29.0.11",
		"deny cluster\nallow dns",
		"deny dns\nallow cluster",
		"deny address from ./cfg/acl key k\nallow address 10.1.0.0/16\nallow address from ./cfg/acl key other",
		"deny address 0.0.0.0/0\nallow address fd00::1 deny address ::/0",
		// A rule on ports leaves the other packets of what it names to
		// the rules after it.
		"deny any port 25 proto tcp\nallow any",
		"allow any port 22 proto tcp\ndeny node n1 port 22 proto udp\ndeny node n1 port 21-22 proto tcp\ndeny node n1",
		"deny node n1 port 22 proto tcp\nallow cluster",
	} {
		assert.Empty(t, shadowed(t, value), value)
	}
}

func TestRulesHasAllow(t *testing.T) {
	for value, want := range map[string]bool{
		"":                        false,
		"deny cluster":            false,
		"deny cluster deny dns":   false,
		"deny cluster\nallow any": true,
		"allow localhost":         true,
		"allow node n1 port 22 proto tcp\ndeny any": true,
	} {
		rules, err := Parse(Keyword, value)
		require.NoError(t, err)
		assert.Equal(t, want, rules.HasAllow(), value)
	}
}
