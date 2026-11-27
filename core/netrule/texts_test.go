package netrule

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The layout of a keyword value is the one of the configuration file: a
// record holds the rules it reads there, one per entry, a line of several
// rules being as many entries.
func TestTheRulesOfAValueAreARulePerEntry(t *testing.T) {
	assert.Equal(t,
		Texts{"allow object system/svc/haproxy", "allow object system/svc/vip", "deny network default"},
		TextsOf("\n   allow object system/svc/haproxy\n   allow object system/svc/vip\n   deny network default"))
	assert.Equal(t, Texts{"allow localhost", "allow dns", "deny node n1"}, TextsOf("allow localhost allow dns deny node n1"))
	assert.Equal(t,
		Texts{"allow network lan node n1", "deny any port 5432 proto tcp"},
		TextsOf("  allow   network   lan node n1\tdeny any port 5432 proto tcp\n\n"))
	assert.Nil(t, TextsOf(""))
	assert.Nil(t, TextsOf(" \n\t\n"))

	// A value the grammar refuses tells its lines: the rules of no
	// record, refused again as they are read.
	refused := TextsOf("allow node n1\n  nowhere   at all\nallow node n2 allow dns")
	assert.Equal(t, Texts{"allow node n1", "nowhere at all", "allow node n2 allow dns"}, refused)
	_, err := Record{NetRules: refused}.Rules()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `net_rules rule 2: "nowhere at all": expected allow or deny to start a rule`)
}

func TestRecordWritesTheRulesAsAList(t *testing.T) {
	rules, err := ParseKeyword("\n   allow object system/svc/haproxy\n   allow object system/svc/vip\n   deny network default", BackendCgroup)
	require.NoError(t, err)
	rec := Record{NetRules: rules.Texts(), Namespace: "system"}
	file := filepath.Join(t.TempDir(), "res", "netwarden.json")
	require.NoError(t, rec.Save(file))
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &raw))
	assert.JSONEq(t, `["allow object system/svc/haproxy", "allow object system/svc/vip", "deny network default"]`, string(raw["rules"]))
	assert.NotContains(t, raw, "net_allow")
	assert.NotContains(t, raw, "net_deny")
	assert.NotContains(t, raw, "net_rules")

	loaded, err := LoadRecord(file)
	require.NoError(t, err)
	assert.Equal(t, rec.NetRules, loaded.NetRules)
	read, err := loaded.Rules()
	require.NoError(t, err)
	require.Len(t, read, 3)
	assert.Equal(t, KindNetwork, read[2].Kind)
	assert.Equal(t, ActionDeny, read[2].Action)
	assert.Equal(t, "default", read[2].Network)
	assert.Equal(t, 3, read[2].Line, "the entry of the list")

	require.NoError(t, Record{}.Save(file))
	b, err = os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"rules":[]`, "no rule is an empty list")
}

// A record an om knowing net_allow and net_deny wrote is read as the list
// saying the same, whether it held the rules as lists or as the text of the
// keywords: the deny rules, which won, then the allow rules, the rest being
// denied by the implicit rule.
func TestRecordReadsTheRulesOfTheTwoKeywords(t *testing.T) {
	file := filepath.Join(t.TempDir(), "netwarden.json")
	read := func(s string) Record {
		t.Helper()
		require.NoError(t, os.WriteFile(file, []byte(s), 0o600))
		rec, err := LoadRecord(file)
		require.NoError(t, err)
		return rec
	}

	// As lists, with the addresses held of each side, the holders whole
	// or of a path alone.
	rec := read(`{
		"net_allow": ["object system/svc/haproxy", "namespace .", "network default node n1", "object ./svc/db rid ip#1 port 5432 proto tcp"],
		"net_deny": ["object system/svc/db", "address 10.9.9.9"],
		"namespace": "system",
		"backend": "netdev",
		"devs": ["tap0"],
		"dns": null,
		"filter": {"allow": ["10.0.1.5/32"], "deny": ["0.0.0.0/0", "::/0"]},
		"public": {"allow": ["10.0.1.5/32"], "deny": ["0.0.0.0/0", "::/0"]},
		"foreign": {"rules": 0, "addrs": 0},
		"held": {
			"allow": {"10.0.1.5": "system/svc/haproxy", "10.0.1.7": {"path": "system/svc/db", "object_id": "id-db"}},
			"deny": {"10.0.1.7": {"path": "system/svc/db", "object_id": "id-db"}}
		},
		"kept": {"deny": {"10.0.1.8": "system/svc/db", "10.0.1.9": "other/svc/gone"}}
	}`)
	assert.Equal(t, Texts{
		"deny object system/svc/db",
		"deny address 10.9.9.9",
		"allow object system/svc/haproxy",
		"allow namespace .",
		"allow network default node n1",
		"allow object ./svc/db rid ip#1 port 5432 proto tcp",
	}, rec.NetRules, "the deny rules first: they won over the allow ones")
	rules, err := rec.Rules()
	require.NoError(t, err)
	assert.Len(t, rules, 6)
	a5, a7, a8 := addrs("10.0.1.5")[0], addrs("10.0.1.7")[0], addrs("10.0.1.8")[0]
	assert.Equal(t, Memory{
		"allow object system/svc/haproxy": {a5: {Path: "system/svc/haproxy"}},
		"allow namespace .": {
			a5: {Path: "system/svc/haproxy"},
			a7: {Path: "system/svc/db", ObjectID: "id-db"},
		},
		"allow object ./svc/db rid ip#1 port 5432 proto tcp": {a7: {Path: "system/svc/db", ObjectID: "id-db"}},
		"deny object system/svc/db":                          {a7: {Path: "system/svc/db", ObjectID: "id-db"}},
	}, rec.Held, "an address held of a side is given to the rules of its verb naming its object")
	assert.Equal(t, Memory{"deny object system/svc/db": {a8: {Path: "system/svc/db"}}}, rec.Kept,
		"and one of an object no rule names is forgotten")
	allow, deny := rec.Filter.Flat()
	assert.Equal(t, []string{"10.0.1.5/32"}, Strings(allow))
	assert.Equal(t, Any(), deny)

	// As the text of the keywords, several rules on a line.
	rec = read(`{
		"net_allow": "\n   object system/svc/haproxy\n   network default node n1 localhost",
		"net_deny": "node n2 address 10.9.9.9",
		"namespace": "system", "pg": "a.slice", "dns": null, "filter": {}, "public": {}, "foreign": {},
		"held": {"allow": {"10.0.1.5": "system/svc/haproxy"}}, "kept": {}
	}`)
	assert.Equal(t, Texts{"deny node n2", "deny address 10.9.9.9", "allow object system/svc/haproxy", "allow network default node n1", "allow localhost"}, rec.NetRules)
	assert.Equal(t, Memory{"allow object system/svc/haproxy": {a5: {Path: "system/svc/haproxy"}}}, rec.Held)
	assert.True(t, rec.Kept.IsZero())
	assert.True(t, rec.HasRules("deny node n2\ndeny address 10.9.9.9\nallow object system/svc/haproxy\nallow network default node n1\nallow localhost"))

	// Deny rules alone allowed the rest: a last allow any says so.
	rec = read(`{"net_allow": [], "net_deny": ["node n2", "network lan"], "namespace": "system", "filter": {"allow": [], "deny": ["10.29.0.12/32"]}}`)
	assert.Equal(t, Texts{"deny node n2", "deny network lan", "allow any"}, rec.NetRules)
	rec = read(`{"net_allow": "", "net_deny": "node n2", "namespace": "system"}`)
	assert.Equal(t, Texts{"deny node n2", "allow any"}, rec.NetRules)
	rec = read(`{"net_allow": ["node n1"], "net_deny": [], "namespace": "system"}`)
	assert.Equal(t, Texts{"allow node n1"}, rec.NetRules, "with allow rules, the rest was denied, as the implicit rule denies it")
	rec = read(`{"net_allow": [], "net_deny": [], "namespace": "system"}`)
	assert.Nil(t, rec.NetRules)

	// What the converted rules resolve to is what the two keywords
	// did: the denied node out of the allowed cluster.
	rec = read(`{"net_allow": ["cluster"], "net_deny": ["node n2"], "namespace": "test", "pg": "a.slice"}`)
	next, _, err := rec.Refresh(testView())
	require.NoError(t, err)
	allow, deny = next.Filter.Flat()
	assert.Equal(t, []string{"10.29.0.11/32", "10.29.0.53/32", "127.0.0.0/8", "::1/128", "fd01::11/128"}, Strings(allow))
	assert.Equal(t, Any(), deny)

	// Written again, it is the record of the one keyword.
	require.NoError(t, next.Save(file))
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"rules":["deny node n2","allow cluster"]`)
	assert.NotContains(t, string(b), "net_allow")
	again, err := LoadRecord(file)
	require.NoError(t, err)
	assert.Equal(t, next.NetRules, again.NetRules)
	assert.True(t, next.Filter.Equal(again.Filter))

	// Rules that are none are kept as they are, and refused when read.
	rec = read(`{"net_allow": ["nowhere at all"], "net_deny": [], "namespace": "system"}`)
	assert.Equal(t, Texts{"allow nowhere at all"}, rec.NetRules)
	_, err = rec.Rules()
	assert.Error(t, err)

	for _, damaged := range []string{`{"net_allow": 3}`, `{"net_allow": [3]}`, `{"net_deny": {"a": "b"}}`, `{"net_rules": "allow any"}`, `{"net_rules": [3]}`} {
		require.NoError(t, os.WriteFile(file, []byte(damaged), 0o600))
		_, err = LoadRecord(file)
		assert.Error(t, err, damaged)
	}
}

// The rules of a record are compared with the ones of the keyword as
// rules: the layout of a value, its lines and its spaces, is no change of
// them, and a rule added, removed, changed or moved is one.
func TestRecordHasRules(t *testing.T) {
	rec := Record{NetRules: TextsOf("deny address 192.168.5.7\nallow object ./svc/db rid ip#1\nallow node n2\nallow network lan")}
	for _, same := range []string{
		"deny address 192.168.5.7\nallow object ./svc/db rid ip#1\nallow node n2\nallow network lan",
		"deny address 192.168.5.7 allow object ./svc/db rid ip#1 allow node n2 allow network lan",
		"\n    deny address 192.168.5.7\n    allow object ./svc/db rid ip#1\n    allow node n2\n    allow network lan\n",
		"deny   address 192.168.5.7\nallow object   ./svc/db\trid ip#1\n\n\nallow node n2 allow network lan",
	} {
		assert.True(t, rec.HasRules(same), "%q", same)
	}
	for _, changed := range []string{
		"",
		"deny address 192.168.5.7\nallow object ./svc/db rid ip#1\nallow node n2",
		"deny address 192.168.5.7\nallow object ./svc/db rid ip#1\nallow node n2\nallow network lan\nallow dns",
		"deny address 192.168.5.7\nallow object ./svc/db rid ip#2\nallow node n2\nallow network lan",
		"deny address 192.168.5.7\nallow object ./svc/db\nallow node n2\nallow network lan",
		"allow object ./svc/db rid ip#1\ndeny address 192.168.5.7\nallow node n2\nallow network lan",
		"allow address 192.168.5.7\nallow object ./svc/db rid ip#1\nallow node n2\nallow network lan",
		"deny address 192.168.5.7\nallow object ./svc/db rid ip#1\nallow network lan node n2",
		"deny address 192.168.5.7\nallow object ./svc/db rid ip#1\nallow node n2\nallow network lan\nnowhere",
	} {
		assert.False(t, rec.HasRules(changed), "%q", changed)
	}
	assert.True(t, Record{}.HasRules(" \n"))
}

// An error of the rules of a record names the entry of its list, where one
// of the rules of a configuration names the line of the keyword: a record
// tells no line.
func TestTheErrorsOfARecordNameTheRule(t *testing.T) {
	_, err := Record{NetRules: Texts{"allow node n1", "allow nowhere"}}.Rules()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `net_rules rule 2: "allow nowhere": unknown rule`)

	_, err = Record{NetRules: Texts{"deny node n1", "deny dns", "deny node n2 port 22 proto tcp"}}.Rules()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `net_rules rule 3: "deny node n2 port 22 proto tcp": port 22: `)

	_, err = ParseKeyword("allow node n1 allow dns\nallow nowhere", BackendCgroup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `net_rules line 2: "allow nowhere": unknown rule`)

	_, err = ParseKeyword("deny node n1 deny dns\n\ndeny node n2 port 22 proto tcp", BackendCgroup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `net_rules line 2: "deny node n2 port 22 proto tcp": port 22: `)

	// The rules changed under a running resource are read from its
	// configuration: the error names the line, and the record is kept.
	rec := Record{NetRules: Texts{"allow node n1"}}
	next, err := rec.WithRules("allow node n1 allow dns\nallow nowhere")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "net_rules line 2: ")
	assert.Equal(t, rec, next)

	next, err = rec.WithRules("\n   allow node n1 allow dns\n   deny network lan")
	require.NoError(t, err)
	assert.Equal(t, Texts{"allow node n1", "allow dns", "deny network lan"}, next.NetRules)
}

// A record of an om naming the list of its rules net_rules, and the action
// of the steps of its filter verb, reads as one naming them rules and
// action.
func TestARecordOfTheNamesBeforeReadsTheSame(t *testing.T) {
	now := `{"rules":["allow node n2"],"filter":{"steps":[{"action":"allow","ranges":["10.29.0.12/32"]},{"action":"deny","ranges":["0.0.0.0/0"]}]}}`
	before := `{"net_rules":["allow node n2"],"filter":{"steps":[{"verb":"allow","ranges":["10.29.0.12/32"]},{"verb":"deny","ranges":["0.0.0.0/0"]}]}}`
	var a, b Record
	require.NoError(t, json.Unmarshal([]byte(now), &a))
	require.NoError(t, json.Unmarshal([]byte(before), &b))
	assert.Equal(t, Texts{"allow node n2"}, a.NetRules)
	assert.Equal(t, a, b)
	assert.Equal(t, ActionAllow, b.Filter.Steps[0].Action)

	out, err := json.Marshal(b)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"rules":["allow node n2"]`)
	assert.Contains(t, string(out), `"action":"allow"`)
	assert.NotContains(t, string(out), `"net_rules"`)
	assert.NotContains(t, string(out), `"verb"`)
}
