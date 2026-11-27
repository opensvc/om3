//go:build linux

package daemonapi

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

// writeNet runs what a write of the administrator of the test namespace
// goes through, for the object test/svc/foo: the keyword policy over what
// the write changes, then the check of the rules of net_rules. An
// empty from is an object being created.
func writeNet(t *testing.T, from, to string) error {
	t.Helper()
	p, _ := naming.ParsePath("test/svc/foo")
	var fromCfg *xconfig.T
	if from != "" {
		fromCfg = configOf(t, from)
	}
	toCfg := configOf(t, to)
	if err := configRbacChanges(admin, p.Kind, fromCfg, toCfg); err != nil {
		return err
	}
	return errors.Join(netRuleRbac(admin, p, fromCfg, toCfg)...)
}

const netNode = "[DEFAULT]\nnodes = *\n"

// The administrator of a namespace names the objects of its namespace, and
// the namespace itself, in the rules of its containers and tasks, however
// the namespace is written.
func TestNetRulesNamingTheNamespaceOfTheObject(t *testing.T) {
	knownNodes(t)
	for _, rules := range []string{
		"net_rules = allow object ./svc/db\n",
		"net_rules = allow object ./svc/{name}-db rid ip#1\n",
		"net_rules = allow object test/svc/db\n",
		"net_rules = allow object {namespace}/svc/db\n",
		"net_rules =\n  deny object ./svc/bad\n  allow namespace .\n",
		"net_rules = deny namespace test\n",
		"net_rules = allow namespace {namespace}\n",
		"net_rules@node1 = allow object ./svc/db\nnet_rules = allow node n1\n",
	} {
		assert.NoError(t, writeNet(t, "", netNode+"[container#1]\ntype = docker\nimage = alpine\n"+rules), rules)
		assert.NoError(t, writeNet(t, "", netNode+"[task#1]\ntype = docker\nimage = alpine\n"+rules), rules)
	}
}

// A rule naming an object of another namespace, or another namespace, asks
// for a grant reading that namespace, or the root grant, as it resolves: a
// reference expanding to another namespace is one, on any node of the
// object, and so is a rule a section takes from DEFAULT.
func TestNetRulesNamingAnotherNamespace(t *testing.T) {
	knownNodes(t)
	container := netNode + "[container#1]\ntype = docker\nimage = alpine\n"
	for rules, ref := range map[string]string{
		"net_rules = allow object prod/svc/db\n":                                  "allow object prod/svc/db",
		"net_rules = deny object prod/svc/db rid ip#1\n":                          "deny object prod/svc/db rid ip#1",
		"net_rules = allow namespace prod\n":                                      "allow namespace prod",
		"net_rules = allow object svc/db\n":                                       "allow object db",
		"net_rules = allow object {namespace}2/svc/db\n":                          "allow object test2/svc/db",
		"net_rules = allow namespace {env.ns}\n[env]\nns = prod\n":                "allow namespace prod",
		"net_rules = allow node n1\nnet_rules@node1 = allow object prod/svc/db\n": "allow object prod/svc/db",
	} {
		err := writeNet(t, "", container+rules)
		require.Error(t, err, rules)
		assert.ErrorIs(t, err, ErrDenied, rules)
		assert.ErrorContains(t, err, ref+": a rule naming an object of another namespace than the one of the object, or another namespace, requires the guest, operator or admin grant on that namespace, or the root grant", rules)
	}
	// A rule refused on every node is said as written, whatever the name of
	// the node of the request sorts after or before the others.
	for _, name := range []string{"aanode", "zznode"} {
		restore := hostname.Impersonate(name)
		err := writeNet(t, "", netNode+"net_rules = allow object prod/svc/db\n[container#1]\ntype = docker\nimage = alpine\n")
		restore()
		assert.ErrorContains(t, err, "container#1.net_rules: allow object prod/svc/db", "inherited from DEFAULT, on %s", name)
	}

	err := writeNet(t, "", container+"net_rules@node1 = allow object prod/svc/db\n")
	assert.ErrorContains(t, err, "container#1.net_rules on node1: allow object prod/svc/db")

	// Reading the namespace named is enough, a guest of it included, and
	// reading another one is not.
	p, _ := naming.ParsePath("test/svc/foo")
	prod := configOf(t, container+"net_rules = allow object prod/svc/db\n")
	guest := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "test"), rbac.NewGrant(rbac.RoleGuest, "prod")}
	assert.Empty(t, netRuleRbac(guest, p, nil, prod))
	dev := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "test"), rbac.NewGrant(rbac.RoleAdmin, "dev")}
	assert.NotEmpty(t, netRuleRbac(dev, p, nil, prod))
	assert.Empty(t, netRuleRbac(rbac.Grants{rbac.GrantRoot}, p, nil, prod))
}

// Only what the write adds is refused: the rules root wrote stay, whatever
// the administrator of the namespace edits beside them.
func TestNetRulesRootWroteAreNotAskedForAgain(t *testing.T) {
	knownNodes(t)
	container := netNode + "[container#1]\ntype = docker\nimage = alpine\n"
	from := container + "net_rules =\n  allow object prod/svc/db\n  allow namespace ops\n  allow node n1\n"
	for _, to := range []string{
		from + "comment = edited\n",
		container + "net_rules =\n  allow node n2\n  allow namespace ops\n  allow object prod/svc/db\n  allow object ./svc/cache\n",
		container + "net_rules = allow object prod/svc/db\n",
		container + "net_rules = allow node n1\n",
		// Moved in the list, among rules of its own namespace.
		container + "net_rules =\n  deny cluster\n  allow namespace ops\n  deny namespace .\n  allow object prod/svc/db\n  allow any\n",
		container,
		netNode,
		from + "[container#2]\ntype = docker\nimage = alpine\nnet_rules = allow object ./svc/db\n",
	} {
		assert.NoError(t, writeNet(t, from, to), to)
	}
	for _, to := range []string{
		container + "net_rules =\n  allow object prod/svc/db\n  allow namespace ops\n  allow object prod/svc/api\n",
		container + "net_rules =\n  deny object prod/svc/db\n  allow object prod/svc/db\n",
		from + "[container#2]\ntype = docker\nimage = alpine\nnet_rules = allow object prod/svc/db\n",
		from + "net_rules@node1 = allow namespace other\n",
		// The reference an allow rule holds is not held for a deny.
		container + "net_rules =\n  allow object prod/svc/db\n  deny namespace ops\n  allow node n1\n",
	} {
		assert.Error(t, writeNet(t, from, to), to)
	}
}

// The administrator of a namespace sets the keyword on its kvm
// containers too, with the ports their filter knows, the other keywords of
// the section staying as they were, and the rules naming another namespace
// asking for a grant on it as on any section.
func TestNetRulesOfAKVMContainer(t *testing.T) {
	knownNodes(t)
	kvm := netNode + "[container#1]\ntype = kvm\nnetwork = backend\n"
	for _, rules := range []string{
		"net_rules =\n  deny any port 25 proto tcp\n  allow object ./svc/db port 5432 proto tcp\n",
		"net_rules = allow namespace .\n",
		"net_rules = deny address from ./sec/acl key denied\n",
	} {
		assert.NoError(t, writeNet(t, "", kvm+rules), rules)
		assert.NoError(t, writeNet(t, kvm, kvm+rules), rules)
		assert.NoError(t, writeNet(t, kvm+rules, kvm), "taking them away: %s", rules)
	}
	err := writeNet(t, kvm, kvm+"net_rules = allow object prod/svc/db port 5432 proto tcp\n")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDenied)
	assert.ErrorContains(t, err, "allow object prod/svc/db: a rule naming an object of another namespace")
	assert.Error(t, writeNet(t, kvm, kvm+"net_rules = allow any\nnetwork_filter = false\n"), "the guard of the addresses is still root's")
	assert.Error(t, writeNet(t, kvm, kvm+"net_rules = allow any\nrcmd = true\n"))
}

// A rule reading its addresses from a datastore of another namespace asks
// for the datastore to share its keys with the namespace of the object,
// whatever is held on its namespace, as the rule resolves on every node of
// the object, and for no grant once it does.
func TestNetRulesReadingADatastoreOfAnotherNamespace(t *testing.T) {
	knownNodes(t)
	// infra/cfg/acl shares its keys with the test namespace, as the
	// node holds it, and no other datastore shares any.
	was := netDatastoreShared
	t.Cleanup(func() { netDatastoreShared = was })
	netDatastoreShared = func(datastore naming.Path, namespace string) bool {
		return datastore.String() == "infra/cfg/acl" && namespace == "test"
	}
	container := netNode + "[container#1]\ntype = docker\nimage = alpine\n"
	for _, rules := range []string{
		"net_rules = allow address from ./cfg/acl key allowed\n",
		"net_rules = allow address from {namespace}/cfg/{name} key allowed\n",
		"net_rules = deny address from test/sec/acl key denied\n",
		"net_rules = allow address from infra/cfg/acl key allowed\n",
		"net_rules = deny address from {env.ns}/cfg/acl key denied\n[env]\nns = infra\n",
	} {
		assert.NoError(t, writeNet(t, "", container+rules), rules)
	}
	for rules, ref := range map[string]string{
		"net_rules = allow address from prod/cfg/acl key allowed\n":                          "allow address from prod/cfg/acl key allowed",
		"net_rules = deny address from cfg/acl key k\n":                                      "deny address from cfg/acl key k",
		"net_rules = allow address from {env.ns}/sec/acl key k\n[env]\nns = prod\n":          "allow address from prod/sec/acl key k",
		"net_rules = allow node n1\nnet_rules@node1 = allow address from prod/cfg/a key k\n": "allow address from prod/cfg/a key k",
		"net_rules = allow address from infra/sec/acl key allowed\n":                         "allow address from infra/sec/acl key allowed",
	} {
		err := writeNet(t, "", container+rules)
		require.Error(t, err, rules)
		assert.ErrorIs(t, err, ErrDenied, rules)
		assert.ErrorContains(t, err, ref+": a rule reading its addresses from a datastore of another namespace than the one of the object requires the datastore to share its keys with the namespace of the object, by its share keyword", rules)
	}
	p, _ := naming.ParsePath("test/svc/foo")
	prod := configOf(t, container+"net_rules = allow address from prod/sec/acl key k\n")
	both := rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "test"), rbac.NewGrant(rbac.RoleAdmin, "prod")}
	assert.NotEmpty(t, netRuleRbac(both, p, nil, prod), "the administrator of both namespaces is not the one sharing the keys")
	assert.Empty(t, netRuleRbac(rbac.Grants{rbac.GrantRoot}, p, nil, prod))

	// The sharing is one with the namespace of the object written.
	dev, _ := naming.ParsePath("dev/svc/foo")
	infra := configOf(t, container+"net_rules = allow address from infra/cfg/acl key allowed\n")
	assert.Empty(t, netRuleRbac(rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "test")}, p, nil, infra))
	assert.NotEmpty(t, netRuleRbac(rbac.Grants{rbac.NewGrant(rbac.RoleAdmin, "dev")}, dev, nil, infra))

	// A rule root wrote stays when the administrator edits the others.
	from := container + "net_rules =\n  allow address from prod/cfg/acl key allowed\n  allow node n1\n"
	assert.NoError(t, writeNet(t, from, container+"net_rules =\n  allow node n2\n  allow address from prod/cfg/acl key allowed\n"))
	assert.Error(t, writeNet(t, from, container+"net_rules =\n  allow address from prod/cfg/acl key allowed\n  allow address from prod/cfg/acl key other\n"))
}

// The sharing of a datastore is read from the configuration the node holds
// of it: its share keyword naming the namespace, or every namespace. A
// datastore the node does not hold shares nothing.
func TestNetDatastoreShared(t *testing.T) {
	testhelper.Setup(t)
	acl := naming.Path{Namespace: "infra", Kind: naming.KindCfg, Name: "acl"}
	assert.False(t, netDatastoreShared(acl, "test"), "not there")
	cfg, err := object.NewCfg(acl)
	require.NoError(t, err)
	require.NoError(t, cfg.AddKey("allowed", []byte("10.1.0.0/16\n")))
	assert.False(t, netDatastoreShared(acl, "test"), "no share keyword")
	assert.True(t, netDatastoreShared(acl, "infra"))
	share := func(value string) {
		require.NoError(t, cfg.Config().Set(keyop.T{Key: key.Parse("share"), Op: keyop.Set, Value: value}))
		require.NoError(t, cfg.Config().Commit())
	}
	share("dev test")
	assert.True(t, netDatastoreShared(acl, "test"))
	assert.False(t, netDatastoreShared(acl, "prod"))
	share("*")
	assert.True(t, netDatastoreShared(acl, "prod"))
	share("dev")
	assert.False(t, netDatastoreShared(acl, "test"), "taken back")
}
